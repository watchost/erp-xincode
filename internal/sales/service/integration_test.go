// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	financeModel "erp-system/internal/finance/model"
	financeService "erp-system/internal/finance/service"
	apperrors "erp-system/internal/pkg/errors"
	salesDto "erp-system/internal/sales/dto"
	salesService "erp-system/internal/sales/service"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseModel "erp-system/internal/warehouse/model"
	warehouseService "erp-system/internal/warehouse/service"
)

func appErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// seedStock 直接入库备货（销售测试的前置库存）。
func seedStock(t *testing.T, wh *warehouseService.WarehouseService, fx testsupport.Fixture, qty, cost decimal.Decimal) {
	t.Helper()
	_, err := wh.Inbound(testsupport.Ctx(), warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           qty,
		UnitCost:      cost,
	})
	if err != nil {
		t.Fatalf("备货入库失败: %v", err)
	}
}

func createApprovedOrder(t *testing.T, deps testsupport.SalesDeps, customerID, materialID int64, qty, price, discount decimal.Decimal) string {
	t.Helper()
	ctx := testsupport.CtxWithUser(1)
	vo, err := deps.Service.CreateOrder(ctx, salesDto.CreateSalesOrderReq{
		CustomerID: customerID,
		Items:      []salesDto.SalesOrderItemReq{{MaterialID: materialID, Qty: qty, Price: price, Discount: discount}},
	})
	if err != nil {
		t.Fatalf("创建销售订单失败: %v", err)
	}
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	return vo.OrderNo
}

func outboundReq(orderNo string, fx testsupport.Fixture, qty decimal.Decimal) salesDto.SalesOutboundScanReq {
	return salesDto.SalesOutboundScanReq{
		OrderNo:       orderNo,
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           qty,
	}
}

func countWhere(t *testing.T, gdb *gorm.DB, table string, count *int64, where string, args ...any) {
	t.Helper()
	if err := gdb.Table(table).Where(where, args...).Count(count).Error; err != nil {
		t.Fatalf("统计 %s 失败: %v", table, err)
	}
}

func sumEntry(t *testing.T, gdb *gorm.DB, accountCode, column string) decimal.Decimal {
	t.Helper()
	var sum decimal.Decimal
	if err := gdb.Model(&financeModel.FinAccountEntry{}).
		Where("account_code = ?", accountCode).
		Select(fmt.Sprintf("COALESCE(SUM(%s),0)", column)).Scan(&sum).Error; err != nil {
		t.Fatalf("汇总 %s 失败: %v", accountCode, err)
	}
	return sum
}

// P2 §3.1 主链路：订单 → 审批 → 部分发货 → 发齐完成。
// 验证库存扣减（移动加权成本）、台账、出库单、成本/应收两张凭证同事务落库且借贷平衡。
func TestSalesFlow_PartialThenComplete(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so1")
	customer := testsupport.CreateCustomer(t, gdb, "so1", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("20"), testsupport.D("10"))

	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("10"), testsupport.D("25"), testsupport.D("0.1")) // 九折：行金额 225

	// 第一次发货 6 件 → 部分发货
	res, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("6")))
	if err != nil {
		t.Fatalf("第一次出库失败: %v", err)
	}
	if !res.Matched || !res.AfterQty.Equal(testsupport.D("14")) || !res.DiffQty.Equal(testsupport.D("4")) {
		t.Fatalf("部分发货结果错误: %+v", res)
	}
	// 成本 = 移动加权 10 × 6 = 60
	if !res.CostAmount.Equal(testsupport.D("60")) {
		t.Fatalf("出库成本应为 60，实际 %v", res.CostAmount)
	}
	order, err := deps.OrderRepo.FindByOrderNo(orderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != salesService.OrderStatusPartiallyShipped {
		t.Fatalf("订单应为部分发货(30)，实际 %d", order.Status)
	}

	// 凭证：成本（借6401/贷1405=60）+ 应收（借1122/贷6001=6*25*0.9=135）
	var costVouchers, arEntries int64
	countWhere(t, gdb, "fin_voucher", &costVouchers, "biz_type = ? AND biz_no = ?", "sales_outbound", res.OutboundNo)
	if costVouchers != 2 {
		t.Fatalf("每次出库应生成 2 张凭证（成本+应收），实际 %d", costVouchers)
	}
	countWhere(t, gdb, "fin_voucher_entry", &arEntries, "biz_type = ? AND biz_no = ? AND account_code = ?", "sales_outbound", res.OutboundNo, "1122")
	if arEntries != 1 {
		t.Fatalf("应收分录应 1 条，实际 %d", arEntries)
	}

	// 第二次发齐 4 件 → 已完成
	if _, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("4"))); err != nil {
		t.Fatalf("第二次出库失败: %v", err)
	}
	order, _ = deps.OrderRepo.FindByOrderNo(orderNo)
	if order.Status != salesService.OrderStatusCompleted {
		t.Fatalf("订单应为已完成(40)，实际 %d", order.Status)
	}
	items, err := deps.OrderRepo.FindItems(order.ID)
	if err != nil || len(items) != 1 || !items[0].ShippedQty.Equal(testsupport.D("10")) {
		t.Fatalf("发货数量错误: %+v err=%v", items, err)
	}

	// 全链路借贷平衡：成本 100 = 10 件 × 10；收入 225 = 10 × 25 × 0.9
	if d := sumEntry(t, gdb, "6401", "debit_amount"); !d.Equal(testsupport.D("100")) {
		t.Fatalf("主营业务成本借方应为 100，实际 %v", d)
	}
	if c := sumEntry(t, gdb, "1405", "credit_amount"); !c.Equal(testsupport.D("100")) {
		t.Fatalf("库存商品贷方应为 100，实际 %v", c)
	}
	if d := sumEntry(t, gdb, "1122", "debit_amount"); !d.Equal(testsupport.D("225")) {
		t.Fatalf("应收账款借方应为 225，实际 %v", d)
	}
	if c := sumEntry(t, gdb, "6001", "credit_amount"); !c.Equal(testsupport.D("225")) {
		t.Fatalf("主营业务收入贷方应为 225，实际 %v", c)
	}

	// 已完成订单再出库必须拒绝
	if _, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("1"))); appErrCode(err) == 0 {
		t.Fatal("已完成订单出库应报错")
	}
}

// 超发拒绝 + 并发不超卖：库存 100，120 并发各发 1 → 恰好 100 成功。
func TestSalesOutbound_ConcurrentNoOversell(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so2")
	customer := testsupport.CreateCustomer(t, gdb, "so2", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("100"), testsupport.D("5"))

	deps := testsupport.NewSalesService(gdb, wh, fin)
	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("1000"), testsupport.D("9"), decimal.Zero) // 订单量足够大，瓶颈在库存

	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < 120; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("1")))
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if succeeded != 100 {
		t.Fatalf("恰好 100 并发应成功，实际 %d", succeeded)
	}

	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ? AND warehouse_id = ?", fx.Material.ID, fx.Warehouse.ID).First(&inv).Error; err != nil {
		t.Fatal(err)
	}
	if !inv.Qty.IsZero() || !inv.AvailableQty.IsZero() {
		t.Fatalf("库存应扣减到 0：%+v", inv)
	}
	var ledgerSum decimal.Decimal
	gdb.Model(&warehouseModel.InvStockLedger{}).
		Where("material_id = ? AND biz_type = ?", fx.Material.ID, warehouseService.BizTypeSalesOutbound).
		Select("COALESCE(SUM(change_qty),0)").Scan(&ledgerSum)
	if !ledgerSum.Equal(testsupport.D("-100")) {
		t.Fatalf("台账合计应为 -100，实际 %v", ledgerSum)
	}
}

// 信用额度：已占用 + 本单 > credit_limit 时下单/审批被拒。
func TestSalesOrder_CreditLimit(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so3")
	customer := testsupport.CreateCustomer(t, gdb, "so3", testsupport.D("100"))
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	// 第一单 80：创建+审批通过（占用 80）
	createApprovedOrder(t, deps, customer.ID, fx.Material.ID, testsupport.D("8"), testsupport.D("10"), decimal.Zero)

	// 第二单 30：创建即被拒（80+30 > 100）
	_, err := deps.Service.CreateOrder(ctx, salesDto.CreateSalesOrderReq{
		CustomerID: customer.ID,
		Items:      []salesDto.SalesOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("3"), Price: testsupport.D("10")}},
	})
	if appErrCode(err) != 70008 {
		t.Fatalf("应报信用额度不足(70008)，实际 %v", err)
	}

	// 第二单 20：恰好顶满额度，创建成功；但模拟「创建后额度被占满」——
	// 直接落库一张已审批的 1 元订单占掉余量后，审批必须复检拒绝。
	vo, err := deps.Service.CreateOrder(ctx, salesDto.CreateSalesOrderReq{
		CustomerID: customer.ID,
		Items:      []salesDto.SalesOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("2"), Price: testsupport.D("10")}},
	})
	if err != nil {
		t.Fatalf("额度内下单应成功: %v", err)
	}
	// 占掉剩余 20 额度
	createApprovedOrder(t, deps, customer.ID, fx.Material.ID, testsupport.D("2"), testsupport.D("10"), decimal.Zero)
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); appErrCode(err) != 70008 {
		t.Fatalf("审批时应复检信用额度并拒绝(70008)，实际 %v", err)
	}
}

// 退货全链路：出库 10 → 退 4 → 入库后库存回填（按原出库成本）、应收/成本红字冲销、超退拒绝。
func TestSalesReturn_Flow(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so4")
	customer := testsupport.CreateCustomer(t, gdb, "so4", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("20"), testsupport.D("10"))
	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("10"), testsupport.D("25"), decimal.Zero)
	outRes, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("10")))
	if err != nil {
		t.Fatalf("出库失败: %v", err)
	}

	// 退货申请 4 件
	retVO, err := deps.Service.CreateReturn(ctx, salesDto.CreateSalesReturnReq{
		OutboundNo:    outRes.OutboundNo,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Reason:        "质量问题",
		Items:         []salesDto.SalesReturnItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("4")}},
	})
	if err != nil {
		t.Fatalf("退货申请失败: %v", err)
	}
	if !retVO.TotalAmount.Equal(testsupport.D("100")) { // 4 × 25
		t.Fatalf("退货金额应为 100，实际 %v", retVO.TotalAmount)
	}

	// 重复申请超额部分应被拒（已申请 4，再申请 7 > 10）
	if _, err := deps.Service.CreateReturn(ctx, salesDto.CreateSalesReturnReq{
		OutboundNo:    outRes.OutboundNo,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Items:         []salesDto.SalesReturnItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("7")}},
	}); appErrCode(err) != 70010 {
		t.Fatalf("超量退货应拒绝(70010)，实际 %v", err)
	}

	// 退货入库
	if err := deps.Service.ReceiveReturn(ctx, retVO.ReturnNo, fx.Warehouse.Code, fx.Location.LocationCode); err != nil {
		t.Fatalf("退货入库失败: %v", err)
	}

	// 库存回填：20-10+4=14，成本按原出库均价 10 回填
	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ? AND warehouse_id = ?", fx.Material.ID, fx.Warehouse.ID).First(&inv).Error; err != nil {
		t.Fatal(err)
	}
	if !inv.Qty.Equal(testsupport.D("14")) || !inv.AvgCost.Equal(testsupport.D("10")) {
		t.Fatalf("退货后库存错误: %+v", inv)
	}

	// 出库行 returned_qty=4
	obItems, err := deps.OutboundRepo.FindItems(retVO.OutboundID)
	if err != nil || len(obItems) != 1 || !obItems[0].ReturnedQty.Equal(testsupport.D("4")) {
		t.Fatalf("退货累计数量错误: %+v err=%v", obItems, err)
	}

	// 红字冲销：借 1405/贷 6401 = 40（4×10）；借 6001/贷 1122 = 100（4×25）
	var costReverse, arReverse int64
	countWhere(t, gdb, "fin_voucher", &costReverse, "biz_type = ? AND biz_no = ?", "sales_return", retVO.ReturnNo)
	if costReverse != 2 {
		t.Fatalf("退货应生成 2 张红字凭证，实际 %d", costReverse)
	}
	_ = arReverse
	// 净应收：出库 250 − 退货 100 = 150
	arDebit := sumEntry(t, gdb, "1122", "debit_amount")
	arCredit := sumEntry(t, gdb, "1122", "credit_amount")
	if !arDebit.Sub(arCredit).Equal(testsupport.D("150")) {
		t.Fatalf("净应收应为 150，实际 借%v 贷%v", arDebit, arCredit)
	}

	// 退货单状态已入库，重复入库拒绝
	ret, err := deps.ReturnRepo.FindByReturnNo(retVO.ReturnNo)
	if err != nil || ret.Status != salesService.ReturnStatusReceived {
		t.Fatalf("退货单应为已入库(20)，实际 %+v err=%v", ret, err)
	}
	if err := deps.Service.ReceiveReturn(ctx, retVO.ReturnNo, fx.Warehouse.Code, fx.Location.LocationCode); appErrCode(err) == 0 {
		t.Fatal("重复退货入库应报错")
	}
}

// 故障注入：过账失败时库存/台账/发货数/出库单整事务回滚。
type failingPoster struct{}

func (failingPoster) PostTx(ctx context.Context, tx *gorm.DB, in financeService.PostingInput) error {
	return apperrors.New(60004, 500, "注入的过账失败")
}

func TestSalesOutbound_PostingFailureRollsBack(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so5")
	customer := testsupport.CreateCustomer(t, gdb, "so5", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("10"), testsupport.D("10"))

	// 先用真实财务建单审批（避免占用额度口径干扰）
	deps := testsupport.NewSalesService(gdb, wh, fin)
	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("5"), testsupport.D("20"), decimal.Zero)

	// 换故障过账器执行出库
	badDeps := testsupport.NewSalesServiceWith(gdb, wh, failingPoster{}, nil)
	if _, err := badDeps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("5"))); err == nil {
		t.Fatal("过账失败应返回错误")
	}

	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ? AND warehouse_id = ?", fx.Material.ID, fx.Warehouse.ID).First(&inv).Error; err != nil {
		t.Fatal(err)
	}
	if !inv.Qty.Equal(testsupport.D("10")) || !inv.AvailableQty.Equal(testsupport.D("10")) {
		t.Fatalf("过账失败库存应回滚：%+v", inv)
	}
	var outbounds, ledgers int64
	countWhere(t, gdb, "sal_sales_outbound", &outbounds, "1=1")
	countWhere(t, gdb, "inv_stock_ledger", &ledgers, "biz_type = ?", warehouseService.BizTypeSalesOutbound)
	if outbounds != 0 || ledgers != 0 {
		t.Fatalf("出库单/台账应回滚：outbound=%d ledger=%d", outbounds, ledgers)
	}
	items, _ := deps.OrderRepo.FindItems(1)
	_ = items
	order, _ := deps.OrderRepo.FindByOrderNo(orderNo)
	if order.Status != salesService.OrderStatusApproved {
		t.Fatalf("订单状态应回滚为已审批(20)，实际 %d", order.Status)
	}
}

// 状态机边界：草稿可改可取消；审批后明细冻结；零发货可取消；有发货拒绝取消。
func TestSalesOrder_StatusMachine(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "so6")
	customer := testsupport.CreateCustomer(t, gdb, "so6", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("10"), testsupport.D("10"))

	// 草稿修改明细
	vo, err := deps.Service.CreateOrder(ctx, salesDto.CreateSalesOrderReq{
		CustomerID: customer.ID,
		Items:      []salesDto.SalesOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("5"), Price: testsupport.D("20")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := deps.Service.UpdateOrder(ctx, vo.OrderNo, salesDto.UpdateSalesOrderReq{
		Items: []salesDto.SalesOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("3"), Price: testsupport.D("30")}},
	})
	if err != nil {
		t.Fatalf("草稿修改失败: %v", err)
	}
	if !updated.TotalAmount.Equal(testsupport.D("90")) {
		t.Fatalf("修改后总额应为 90，实际 %v", updated.TotalAmount)
	}

	// 重复审批拒绝
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); appErrCode(err) == 0 {
		t.Fatal("重复审批应报错")
	}
	// 审批后修改明细拒绝
	if _, err := deps.Service.UpdateOrder(ctx, vo.OrderNo, salesDto.UpdateSalesOrderReq{
		Items: []salesDto.SalesOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("1"), Price: testsupport.D("1")}},
	}); appErrCode(err) == 0 {
		t.Fatal("审批后修改明细应报错")
	}

	// 发货后取消拒绝
	if _, err := deps.Service.OutboundScan(ctx, outboundReq(vo.OrderNo, fx, testsupport.D("1"))); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.CancelOrder(ctx, vo.OrderNo); appErrCode(err) != 70007 {
		t.Fatalf("有发货取消应报 70007，实际 %v", err)
	}

	// 零发货订单可取消
	orderNo2 := createApprovedOrder(t, deps, customer.ID, fx.Material.ID, testsupport.D("1"), testsupport.D("1"), decimal.Zero)
	if err := deps.Service.CancelOrder(ctx, orderNo2); err != nil {
		t.Fatalf("零发货取消应成功: %v", err)
	}
	if _, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo2, fx, testsupport.D("1"))); appErrCode(err) == 0 {
		t.Fatal("已取消订单出库应报错")
	}
}
