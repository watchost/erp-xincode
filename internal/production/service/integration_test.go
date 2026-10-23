// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	apperrors "erp-system/internal/pkg/errors"
	productionDto "erp-system/internal/production/dto"
	productionRepo "erp-system/internal/production/repository"
	productionService "erp-system/internal/production/service"
	"erp-system/internal/testsupport"
)

// newProductionService 在测试库内装配完整生产服务（真实仓储 + 真实仓库/财务服务）。
func newProductionService(t *testing.T, gdb *gorm.DB) (*productionService.ProductionService,
	productionRepo.WorkOrderRepository, productionRepo.BomRepository) {
	t.Helper()
	woRepo := productionRepo.NewWorkOrderRepository(gdb)
	bomRepo := productionRepo.NewBomRepository(gdb)
	svc := productionService.NewProductionService(
		db.NewTxManager(gdb),
		woRepo,
		productionRepo.NewProductionReceiptRepository(gdb),
		bomRepo,
		mdmRepo.NewMaterialRepository(gdb),
		mdmRepo.NewWarehouseRepository(gdb),
		mdmRepo.NewLocationRepository(gdb),
		testsupport.NewWarehouseService(gdb),
		testsupport.NewFinanceService(gdb),
		bizno.NewGenerator(nil),
		nil,
	)
	return svc, woRepo, bomRepo
}

func createProduct(t *testing.T, gdb *gorm.DB, code string) *mdmModel.MdmMaterial {
	t.Helper()
	p := &mdmModel.MdmMaterial{MaterialCode: code, Name: "成品-" + code, Unit: "个", CostMethod: 1, Status: 1}
	if err := gdb.Create(p).Error; err != nil {
		t.Fatalf("创建成品失败: %v", err)
	}
	return p
}

func appCode(err error) int {
	var ae *apperrors.AppError
	if err == nil {
		return 0
	}
	if !asAppErr(err, &ae) {
		return -1
	}
	return ae.Code
}

// 全链路 decimal：BOM 损耗率展开（2×1×1.1=2.2）→ 超领/超产 50006 →
// 正常领料（借5001/贷1403）→ 完工入库（借1405/贷5001）→ 工单完工；
// 两类凭证各自借贷精确平衡。
func TestProductionFlow_DecimalBomIssueReceipt(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "prod")
	svc, _, _ := newProductionService(t, gdb)
	whSvc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.CtxWithUser(1)
	product := createProduct(t, gdb, "PROD-DEC-1")

	// BOM：每件产品耗 2 个组件，损耗率 10%
	if _, err := svc.CreateBom(ctx, productionDto.CreateBomReq{
		ProductID: product.ID,
		Items: []productionDto.BomItemReq{{
			MaterialID: fx.Material.ID,
			Qty:        testsupport.D("2"),
			Unit:       "个",
			ScrapRate:  testsupport.D("0.1"),
		}},
	}); err != nil {
		t.Fatalf("创建 BOM 失败: %v", err)
	}

	// 工单 1 件 → 组件需求精确为 2.2
	wo, err := svc.CreateWorkOrder(ctx, productionDto.CreateWorkOrderReq{
		ProductID: product.ID,
		PlanQty:   testsupport.D("1"),
	})
	if err != nil {
		t.Fatalf("创建工单失败: %v", err)
	}
	// 组件备料：入库 3@100（齐套检查要求下达前可用库存 ≥ 需求 2.2）
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("3"), testsupport.D("100"))); err != nil {
		t.Fatalf("组件备料失败: %v", err)
	}

	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); err != nil {
		t.Fatalf("下达工单失败: %v", err)
	}

	// 超领 3 > 计划 2.2 → 50006，库存不得扣减
	if _, err := svc.MaterialIssueScan(ctx, issueReq(wo.WorkOrderNo, fx, testsupport.D("3"))); appCode(err) != 50006 {
		t.Fatalf("超领应返回 50006，实际 %v", err)
	}
	var compInv struct{ Qty decimal.Decimal }
	if err := gdb.Table("inv_inventory").Where("material_id = ?", fx.Material.ID).Take(&compInv).Error; err != nil {
		t.Fatal(err)
	}
	if !compInv.Qty.Equal(testsupport.D("3")) {
		t.Fatalf("超领回滚后组件库存应仍为 3，实际 %v", compInv.Qty)
	}

	// 正常领料 2.2：借生产成本/贷原材料，金额 = 100 × 2.2 = 220.00
	if _, err := svc.MaterialIssueScan(ctx, issueReq(wo.WorkOrderNo, fx, testsupport.D("2.2"))); err != nil {
		t.Fatalf("正常领料失败: %v", err)
	}
	if !balanced(t, gdb, "production_issue") {
		t.Fatal("领料凭证借贷不平衡")
	}
	var issueDebit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ? AND account_code = ?", "production_issue", "5001").
		Select("COALESCE(SUM(debit_amount),0)").Scan(&issueDebit)
	if !issueDebit.Equal(testsupport.D("220")) {
		t.Fatalf("领料结转成本应为 220.00，实际 %v", issueDebit)
	}

	// 超产 1.1 > 计划 1 → 50006
	if _, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1.1"))); appCode(err) != 50006 {
		t.Fatalf("超产应返回 50006，实际 %v", err)
	}

	// 完工入库 1 件：成本从工单成本池全额结转（领料 220.00），不接受请求传价（S1）。
	res, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1")))
	if err != nil {
		t.Fatalf("完工入库失败: %v", err)
	}
	if !res.Completed || res.WorkOrderStatus != productionService.WorkOrderStatusCompleted {
		t.Fatalf("应收工完工，实际 completed=%v status=%d", res.Completed, res.WorkOrderStatus)
	}
	if !balanced(t, gdb, "production_receipt") {
		t.Fatal("完工凭证借贷不平衡")
	}

	// 成品库存 1@220（=领料成本 220.00 全额结转）、台账金额 220.00
	var fgInv struct {
		Qty     decimal.Decimal
		AvgCost decimal.Decimal
	}
	if err := gdb.Table("inv_inventory").Where("material_id = ?", product.ID).Take(&fgInv).Error; err != nil {
		t.Fatal(err)
	}
	if !fgInv.Qty.Equal(testsupport.D("1")) || !fgInv.AvgCost.Equal(testsupport.D("220")) {
		t.Fatalf("成品库存错误: %+v", fgInv)
	}

	// 完工后 5001 轧平：借方 220（领料）= 贷方 220（入库结转），成本池清零。
	var costDebit, costCredit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("account_code = ?", "5001").
		Select("COALESCE(SUM(debit_amount),0)").Scan(&costDebit)
	gdb.Model(&model.FinAccountEntry{}).Where("account_code = ?", "5001").
		Select("COALESCE(SUM(credit_amount),0)").Scan(&costCredit)
	if !costDebit.Equal(costCredit) || !costDebit.Equal(testsupport.D("220")) {
		t.Fatalf("完工后 5001 应轧平于 220.00，实际 借%v 贷%v", costDebit, costCredit)
	}
	var woCosts struct {
		IssuedCost   decimal.Decimal
		ReceivedCost decimal.Decimal
	}
	if err := gdb.Table("prod_work_order").Where("work_order_no = ?", wo.WorkOrderNo).
		Select("issued_cost, received_cost").Take(&woCosts).Error; err != nil {
		t.Fatal(err)
	}
	if !woCosts.IssuedCost.Equal(woCosts.ReceivedCost) {
		t.Fatalf("完工后成本池应清零，实际 issued=%v received=%v", woCosts.IssuedCost, woCosts.ReceivedCost)
	}
}

// P1.5 S1：入库成本从工单成本池按比例结转——分批入库按比例、完工批全额收尾；
// 未领料（成本池为空）拒绝入库 50007。
func TestProductionReceipt_CostPoolAllocation(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "pool")
	svc, _, _ := newProductionService(t, gdb)
	whSvc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.CtxWithUser(1)
	product := createProduct(t, gdb, "PROD-POOL-1")

	if _, err := svc.CreateBom(ctx, productionDto.CreateBomReq{
		ProductID: product.ID,
		Items: []productionDto.BomItemReq{{
			MaterialID: fx.Material.ID,
			Qty:        testsupport.D("1"),
			Unit:       "个",
			ScrapRate:  decimal.Zero,
		}},
	}); err != nil {
		t.Fatalf("创建 BOM 失败: %v", err)
	}
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("10"), testsupport.D("30"))); err != nil {
		t.Fatalf("备料失败: %v", err)
	}
	wo, err := svc.CreateWorkOrder(ctx, productionDto.CreateWorkOrderReq{
		ProductID: product.ID,
		PlanQty:   testsupport.D("4"),
	})
	if err != nil {
		t.Fatalf("创建工单失败: %v", err)
	}

	// 未领料即入库 → 50007（成本池为空）
	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); err != nil {
		t.Fatalf("下达失败: %v", err)
	}
	if _, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1"))); appCode(err) != 50007 {
		t.Fatalf("无成本池入库应返回 50007，实际 %v", err)
	}

	// 领料 4@30 → 成本池 120.00
	if _, err := svc.MaterialIssueScan(ctx, issueReq(wo.WorkOrderNo, fx, testsupport.D("4"))); err != nil {
		t.Fatalf("领料失败: %v", err)
	}

	// 第一批入库 1 件：按比例结转 120×1/4=30.00，成品均价 30
	if _, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1"))); err != nil {
		t.Fatalf("第一批入库失败: %v", err)
	}
	var fgInv struct{ AvgCost decimal.Decimal }
	if err := gdb.Table("inv_inventory").Where("material_id = ?", product.ID).Take(&fgInv).Error; err != nil {
		t.Fatal(err)
	}
	if !fgInv.AvgCost.Equal(testsupport.D("30")) {
		t.Fatalf("第一批应按比例结转 30.00，实际 %v", fgInv.AvgCost)
	}

	// 完工批入库 3 件：全额收尾剩余 90.00 → 成本池清零
	res, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("3")))
	if err != nil {
		t.Fatalf("完工批入库失败: %v", err)
	}
	if !res.Completed {
		t.Fatal("应完工")
	}
	var receiptCredit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ? AND account_code = ?", "production_receipt", "5001").
		Select("COALESCE(SUM(credit_amount),0)").Scan(&receiptCredit)
	if !receiptCredit.Equal(testsupport.D("120")) {
		t.Fatalf("入库累计结转应等于领料成本 120.00（5001 轧平），实际 %v", receiptCredit)
	}
}

func balanced(t *testing.T, gdb *gorm.DB, bizType string) bool {
	t.Helper()
	var debit, credit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ?", bizType).Select("COALESCE(SUM(debit_amount),0)").Scan(&debit)
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ?", bizType).Select("COALESCE(SUM(credit_amount),0)").Scan(&credit)
	return debit.Equal(credit)
}

// 齐套检查：无可用库存时下达返回 50005；补足库存后可下达。
func TestReleaseWorkOrder_KittingCheck(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "kit")
	svc, _, _ := newProductionService(t, gdb)
	whSvc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.CtxWithUser(1)
	product := createProduct(t, gdb, "PROD-KIT-1")

	if _, err := svc.CreateBom(ctx, productionDto.CreateBomReq{
		ProductID: product.ID,
		Items: []productionDto.BomItemReq{{
			MaterialID: fx.Material.ID,
			Qty:        testsupport.D("2"),
			Unit:       "个",
			ScrapRate:  decimal.Zero,
		}},
	}); err != nil {
		t.Fatalf("创建 BOM 失败: %v", err)
	}

	wo, err := svc.CreateWorkOrder(ctx, productionDto.CreateWorkOrderReq{
		ProductID: product.ID,
		PlanQty:   testsupport.D("5"),
	})
	if err != nil {
		t.Fatalf("创建工单失败: %v", err)
	}

	// 无库存：需求 10，可用 0 → 50005
	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); appCode(err) != 50005 {
		t.Fatalf("缺料下达应返回 50005，实际 %v", err)
	}

	// 部分库存：仍缺料 → 50005
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("6"), testsupport.D("10"))); err != nil {
		t.Fatalf("备料失败: %v", err)
	}
	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); appCode(err) != 50005 {
		t.Fatalf("部分缺料下达应返回 50005，实际 %v", err)
	}

	// 补足：可用 12 ≥ 10 → 下达成功
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("6"), testsupport.D("10"))); err != nil {
		t.Fatalf("补料失败: %v", err)
	}
	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); err != nil {
		t.Fatalf("齐套后下达失败: %v", err)
	}
}

// 工单关闭：草稿不可关(50004)；已下达零领料可关；已领料不可关(50006)；完工可关。
func TestCloseWorkOrder(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "close")
	svc, _, _ := newProductionService(t, gdb)
	whSvc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.CtxWithUser(1)
	product := createProduct(t, gdb, "PROD-CLOSE-1")

	if _, err := svc.CreateBom(ctx, productionDto.CreateBomReq{
		ProductID: product.ID,
		Items: []productionDto.BomItemReq{{
			MaterialID: fx.Material.ID,
			Qty:        testsupport.D("1"),
			Unit:       "个",
			ScrapRate:  decimal.Zero,
		}},
	}); err != nil {
		t.Fatalf("创建 BOM 失败: %v", err)
	}
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("100"), testsupport.D("10"))); err != nil {
		t.Fatalf("备料失败: %v", err)
	}

	newWO := func(planQty string) string {
		wo, err := svc.CreateWorkOrder(ctx, productionDto.CreateWorkOrderReq{
			ProductID: product.ID,
			PlanQty:   testsupport.D(planQty),
		})
		if err != nil {
			t.Fatalf("创建工单失败: %v", err)
		}
		return wo.WorkOrderNo
	}

	// 草稿不可关闭
	draftNo := newWO("1")
	if err := svc.CloseWorkOrder(ctx, draftNo); appCode(err) != 50004 {
		t.Fatalf("草稿关闭应返回 50004，实际 %v", err)
	}

	// 已下达零领料：可关闭
	if err := svc.ReleaseWorkOrder(ctx, draftNo); err != nil {
		t.Fatalf("下达失败: %v", err)
	}
	if err := svc.CloseWorkOrder(ctx, draftNo); err != nil {
		t.Fatalf("零领料关闭失败: %v", err)
	}
	// 重复关闭 → 50004
	if err := svc.CloseWorkOrder(ctx, draftNo); appCode(err) != 50004 {
		t.Fatalf("重复关闭应返回 50004，实际 %v", err)
	}

	// 已领料：不可关闭（50006）
	issuedNo := newWO("2")
	if err := svc.ReleaseWorkOrder(ctx, issuedNo); err != nil {
		t.Fatalf("下达失败: %v", err)
	}
	if _, err := svc.MaterialIssueScan(ctx, issueReq(issuedNo, fx, testsupport.D("1"))); err != nil {
		t.Fatalf("领料失败: %v", err)
	}
	if err := svc.CloseWorkOrder(ctx, issuedNo); appCode(err) != 50006 {
		t.Fatalf("已领料关闭应返回 50006，实际 %v", err)
	}

	// 完工：可关闭
	if _, err := svc.ProductionReceipt(ctx, receiptReq(issuedNo, fx, testsupport.D("2"))); err != nil {
		t.Fatalf("完工入库失败: %v", err)
	}
	if err := svc.CloseWorkOrder(ctx, issuedNo); err != nil {
		t.Fatalf("完工关闭失败: %v", err)
	}
}
