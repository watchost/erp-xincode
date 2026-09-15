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
	if err := svc.ReleaseWorkOrder(ctx, wo.WorkOrderNo); err != nil {
		t.Fatalf("下达工单失败: %v", err)
	}

	// 组件备料：入库 3@100
	if _, err := whSvc.Inbound(ctx, inboundFor(fx, testsupport.D("3"), testsupport.D("100"))); err != nil {
		t.Fatalf("组件备料失败: %v", err)
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
	if _, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1.1"), testsupport.D("10"))); appCode(err) != 50006 {
		t.Fatalf("超产应返回 50006，实际 %v", err)
	}

	// 完工入库 1@10：借库存商品/贷生产成本，工单置完成
	res, err := svc.ProductionReceipt(ctx, receiptReq(wo.WorkOrderNo, fx, testsupport.D("1"), testsupport.D("10")))
	if err != nil {
		t.Fatalf("完工入库失败: %v", err)
	}
	if !res.Completed || res.WorkOrderStatus != productionService.WorkOrderStatusCompleted {
		t.Fatalf("应收工完工，实际 completed=%v status=%d", res.Completed, res.WorkOrderStatus)
	}
	if !balanced(t, gdb, "production_receipt") {
		t.Fatal("完工凭证借贷不平衡")
	}

	// 成品库存 1@10、台账金额 10.00
	var fgInv struct {
		Qty     decimal.Decimal
		AvgCost decimal.Decimal
	}
	if err := gdb.Table("inv_inventory").Where("material_id = ?", product.ID).Take(&fgInv).Error; err != nil {
		t.Fatal(err)
	}
	if !fgInv.Qty.Equal(testsupport.D("1")) || !fgInv.AvgCost.Equal(testsupport.D("10")) {
		t.Fatalf("成品库存错误: %+v", fgInv)
	}
}

func balanced(t *testing.T, gdb *gorm.DB, bizType string) bool {
	t.Helper()
	var debit, credit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ?", bizType).Select("COALESCE(SUM(debit_amount),0)").Scan(&debit)
	gdb.Model(&model.FinAccountEntry{}).Where("biz_type = ?", bizType).Select("COALESCE(SUM(credit_amount),0)").Scan(&credit)
	return debit.Equal(credit)
}
