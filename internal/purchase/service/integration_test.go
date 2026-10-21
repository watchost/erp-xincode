// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	financeService "erp-system/internal/finance/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/idemp"
	purchaseDto "erp-system/internal/purchase/dto"
	purchaseModel "erp-system/internal/purchase/model"
	purchaseService "erp-system/internal/purchase/service"
	"erp-system/internal/testsupport"
	warehouseModel "erp-system/internal/warehouse/model"
)

func appErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func createApprovedOrder(t *testing.T, deps testsupport.PurchaseDeps, fx testsupport.Fixture, qty, price decimal.Decimal) string {
	t.Helper()
	ctx := testsupport.CtxWithUser(1)
	vo, err := deps.Service.CreateOrder(ctx, purchaseDto.CreatePurchaseOrderReq{
		SupplierID: fx.Supplier.ID,
		Items:      []purchaseDto.PurchaseOrderItemReq{{MaterialID: fx.Material.ID, Qty: qty, Price: price}},
	})
	if err != nil {
		t.Fatalf("创建采购订单失败: %v", err)
	}
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	return vo.OrderNo
}

func inboundReq(orderNo string, fx testsupport.Fixture, qty decimal.Decimal) purchaseDto.PurchaseInboundScanReq {
	return purchaseDto.PurchaseInboundScanReq{
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

// P0 主链路：订单 → 审批 → 部分收货 → 收齐完成。
// 同时验证库存移动平均价取采购单价、台账/入库单/财务分录/成本卡同事务落库。
func TestPurchaseFlow_PartialThenComplete(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po1")
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))

	// 第一次收货 6 件 → 部分收货
	res, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("6")))
	if err != nil {
		t.Fatalf("第一次入库失败: %v", err)
	}
	if !res.Matched || !res.AfterQty.Equal(testsupport.D("6")) || !res.DiffQty.Equal(testsupport.D("4")) {
		t.Fatalf("部分收货结果错误: %+v", res)
	}
	order, err := deps.OrderRepo.FindByOrderNo(orderNo)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != purchaseService.OrderStatusPartiallyReceived {
		t.Fatalf("订单应为部分收货(5)，实际 %d", order.Status)
	}

	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ? AND warehouse_id = ?", fx.Material.ID, fx.Warehouse.ID).First(&inv).Error; err != nil {
		t.Fatalf("查询库存失败: %v", err)
	}
	if !inv.Qty.Equal(testsupport.D("6")) || !inv.AvailableQty.Equal(testsupport.D("6")) || !inv.AvgCost.Equal(testsupport.D("25")) {
		t.Fatalf("库存/平均成本错误: %+v", inv)
	}

	var entries int64
	countWhere(t, gdb, "fin_voucher", &entries, "biz_type = ? AND biz_no = ?", "purchase_inbound", res.InboundNo)
	if entries != 2 {
		t.Fatalf("每次入库应生成借贷 2 条分录，实际 %d", entries)
	}
	var inboundRows int64
	countWhere(t, gdb, "pur_purchase_inbound", &inboundRows, "order_id = ?", order.ID)
	if inboundRows != 1 {
		t.Fatalf("入库单应 1 张，实际 %d", inboundRows)
	}

	// 第二次收齐 4 件 → 已完成
	if _, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("4"))); err != nil {
		t.Fatalf("第二次入库失败: %v", err)
	}
	order, _ = deps.OrderRepo.FindByOrderNo(orderNo)
	if order.Status != purchaseService.OrderStatusCompleted {
		t.Fatalf("订单应为已完成(3)，实际 %d", order.Status)
	}
	items, err := deps.OrderRepo.FindItems(order.ID)
	if err != nil || len(items) != 1 || !items[0].ReceivedQty.Equal(testsupport.D("10")) {
		t.Fatalf("收货数量错误: %+v err=%v", items, err)
	}
	if err := gdb.Where("material_id = ?", fx.Material.ID).First(&inv).Error; err != nil {
		t.Fatal(err)
	}
	if !inv.Qty.Equal(testsupport.D("10")) || !inv.AvgCost.Equal(testsupport.D("25")) {
		t.Fatalf("收齐后库存错误: %+v", inv)
	}

	// 全链路借贷平衡：借原材料合计 = 贷应付合计 = 250
	var debit, credit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("account_code = ?", "1403").Select("COALESCE(SUM(debit_amount),0)").Scan(&debit)
	gdb.Model(&model.FinAccountEntry{}).Where("account_code = ?", "2202").Select("COALESCE(SUM(credit_amount),0)").Scan(&credit)
	if !debit.Equal(testsupport.D("250")) || !credit.Equal(testsupport.D("250")) {
		t.Fatalf("借贷合计应各为 250：借=%v 贷=%v", debit, credit)
	}
	var costTotal decimal.Decimal
	gdb.Model(&model.FinCostCard{}).Where("product_id = ?", fx.Material.ID).Select("COALESCE(SUM(amount),0)").Scan(&costTotal)
	if !costTotal.Equal(testsupport.D("250")) {
		t.Fatalf("材料成本卡合计应为 250，实际 %v", costTotal)
	}

	// 已完成订单再入库必须拒绝
	if _, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("1"))); appErrCode(err) == 0 {
		t.Fatal("已完成订单入库应报错")
	}
}

// 超收必须在锁内拒绝，且库存/单据/财务都不得变化。
func TestPurchaseInbound_OverReceiptRollsBack(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po2")
	deps := testsupport.NewPurchaseService(gdb, testsupport.NewWarehouseService(gdb), testsupport.NewFinanceService(gdb))
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("1"), testsupport.D("25"))
	_, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("2")))
	if appErrCode(err) != 40006 {
		t.Fatalf("超收应返回 40006，实际 %v", err)
	}

	var invCount int64
	countWhere(t, gdb, "inv_inventory", &invCount, "material_id = ?", fx.Material.ID)
	if invCount != 0 {
		t.Fatal("超收回滚后不得产生库存行")
	}
	order, _ := deps.OrderRepo.FindByOrderNo(orderNo)
	if order.Status != purchaseService.OrderStatusApproved {
		t.Fatalf("超收后订单状态不应变化: %d", order.Status)
	}
	items, _ := deps.OrderRepo.FindItems(order.ID)
	if !items[0].ReceivedQty.IsZero() {
		t.Fatalf("超收后收货数量应为 0: %v", items[0].ReceivedQty)
	}
	var n int64
	countWhere(t, gdb, "fin_voucher", &n, "1=1")
	if n != 0 {
		t.Fatal("超收回滚后不得有财务分录")
	}
}

// 预算：审批时占用，超额拒批；零收货取消时释放，已有收货不允许取消。
func TestPurchaseBudget_ReserveReleaseAndBlock(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po3")
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, testsupport.NewWarehouseService(gdb), fin)
	ctx := testsupport.CtxWithUser(1)
	now := time.Now()

	// 预算仅 100，订单金额 250 → 审批拒绝，订单停留草稿，预算零占用
	if _, err := fin.CreateBudget(ctx, "purchase", testsupport.D("100"), now.Year(), int(now.Month())); err != nil {
		t.Fatal(err)
	}
	vo, err := deps.Service.CreateOrder(ctx, purchaseDto.CreatePurchaseOrderReq{
		SupplierID: fx.Supplier.ID,
		Items:      []purchaseDto.PurchaseOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("10"), Price: testsupport.D("25")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); appErrCode(err) != 60010 {
		t.Fatalf("预算不足应返回 60010，实际 %v", err)
	}
	order, _ := deps.OrderRepo.FindByOrderNo(vo.OrderNo)
	if order.Status != purchaseService.OrderStatusDraft {
		t.Fatalf("拒批后订单应为草稿，实际 %d", order.Status)
	}
	var b model.FinBudget
	gdb.Where("budget_type = ? AND year = ? AND month = ?", "purchase", now.Year(), int(now.Month())).First(&b)
	if !b.UsedAmount.IsZero() {
		t.Fatalf("拒批不得占用预算，used=%v", b.UsedAmount)
	}
	// 草稿单可直接取消
	if err := deps.Service.CancelOrder(ctx, vo.OrderNo); err != nil {
		t.Fatalf("取消草稿单失败: %v", err)
	}

	// 预算提高到 1000：审批成功占用 250；零收货取消后释放
	gdb.Model(&b).Where("id = ?", b.ID).Update("amount", testsupport.D("1000"))
	orderNo2 := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	gdb.First(&b, b.ID)
	if !b.UsedAmount.Equal(testsupport.D("250")) {
		t.Fatalf("审批后应占用 250，实际 %v", b.UsedAmount)
	}
	if err := deps.Service.CancelOrder(ctx, orderNo2); err != nil {
		t.Fatalf("取消已审批零收货订单失败: %v", err)
	}
	gdb.First(&b, b.ID)
	if !b.UsedAmount.IsZero() {
		t.Fatalf("取消后应释放预算，used=%v", b.UsedAmount)
	}
	order2, _ := deps.OrderRepo.FindByOrderNo(orderNo2)
	if order2.Status != purchaseService.OrderStatusCanceled {
		t.Fatalf("订单应为已取消(4)，实际 %d", order2.Status)
	}

	// 已有入库的订单不允许取消（应付与库存已生成）：部分收货后停留在状态 5
	orderNo3 := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	if _, err := deps.Service.InboundScan(ctx, inboundReq(orderNo3, fx, testsupport.D("5"))); err != nil {
		t.Fatal(err)
	}
	if err := deps.Service.CancelOrder(ctx, orderNo3); appErrCode(err) != 40007 {
		t.Fatalf("有收货记录取消应返回 40007，实际 %v", err)
	}
}

// 故障注入：库存写入后财务过账失败，库存/台账/收货数量必须随外层事务全部回滚。
// 对应 P0 验收「单据写入失败时库存回滚」。
func TestInbound_PostingFailureRollsBackInventory(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po4")
	stub := &failingFinance{}
	deps := testsupport.NewPurchaseServiceWith(gdb, testsupport.NewWarehouseService(gdb), stub, nil)
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	_, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("6")))
	if err == nil {
		t.Fatal("财务过账故障必须导致入库失败")
	}

	var invCount, ledgerCount, inboundCount int64
	countWhere(t, gdb, "inv_inventory", &invCount, "material_id = ?", fx.Material.ID)
	countWhere(t, gdb, "inv_stock_ledger", &ledgerCount, "material_id = ?", fx.Material.ID)
	countWhere(t, gdb, "pur_purchase_inbound", &inboundCount, "1=1")
	if invCount != 0 || ledgerCount != 0 || inboundCount != 0 {
		t.Fatalf("事务回滚不彻底: inv=%d ledger=%d inbound=%d", invCount, ledgerCount, inboundCount)
	}
	var order purchaseModel.PurPurchaseOrder
	if err := gdb.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		t.Fatal(err)
	}
	var item purchaseModel.PurPurchaseOrderItem
	if err := gdb.Where("order_id = ?", order.ID).First(&item).Error; err != nil {
		t.Fatal(err)
	}
	if !item.ReceivedQty.IsZero() || order.Status != purchaseService.OrderStatusApproved {
		t.Fatalf("回滚后收货数应为 0、订单仍为已审批: qty=%v status=%d", item.ReceivedQty, order.Status)
	}
}

// 幂等：同一 Idempotency-Key 重复扫码第二次必须被拒绝。
func TestInbound_IdempotencyKeyRejectsDuplicate(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po5")
	guard := idemp.NewGuard(&memStore{}, time.Hour)
	deps := testsupport.NewPurchaseServiceWith(gdb, testsupport.NewWarehouseService(gdb),
		testsupport.NewFinanceService(gdb), guard)
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	req := inboundReq(orderNo, fx, testsupport.D("3"))
	req.IdempotencyKey = "idem-key-001"
	if _, err := deps.Service.InboundScan(ctx, req); err != nil {
		t.Fatalf("首次扫码应成功: %v", err)
	}
	if _, err := deps.Service.InboundScan(ctx, req); appErrCode(err) != 10200 {
		t.Fatalf("重复扫码应返回 10200，实际 %v", err)
	}
	var invCount int64
	countWhere(t, gdb, "inv_inventory", &invCount, "material_id = ? AND qty = ?", fx.Material.ID, testsupport.D("3"))
	if invCount != 1 {
		t.Fatal("重复扫码不得重复入库")
	}
}

// failingFinance 是财务网关故障桩：预算检查放行，过账一律失败。
type failingFinance struct{}

func (failingFinance) PostTx(ctx context.Context, tx *gorm.DB, in financeService.PostingInput) error {
	_ = ctx
	_ = tx
	_ = in
	return errors.New("finance posting unavailable（故障注入）")
}
func (failingFinance) ReserveBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error {
	return nil
}
func (failingFinance) ReleaseBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error {
	return nil
}

// memStore 是 idemp.Store 的进程内实现，避免集成测试依赖 Redis。
type memStore struct {
	mu   sync.Mutex
	keys map[string]bool
}

func (m *memStore) SetNX(ctx context.Context, key string, value interface{}, expiration int) (bool, error) {
	_ = ctx
	_ = value
	_ = expiration
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.keys == nil {
		m.keys = map[string]bool{}
	}
	if m.keys[key] {
		return false, nil
	}
	m.keys[key] = true
	return true, nil
}

// 精度回归：0.1 + 0.2 必须精确等于 0.30（订单总额、入库后 1403 借方合计、成本卡合计）。
func TestPurchaseFlow_DecimalPrecisionOneTenthPlusTwoTenths(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "po6")
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	// 订单 0.300 件 × 单价 1：总额必须精确为 0.30
	vo, err := deps.Service.CreateOrder(ctx, purchaseDto.CreatePurchaseOrderReq{
		SupplierID: fx.Supplier.ID,
		Items:      []purchaseDto.PurchaseOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("0.300"), Price: testsupport.D("1")}},
	})
	if err != nil {
		t.Fatalf("创建订单失败: %v", err)
	}
	if !vo.TotalAmount.Equal(testsupport.D("0.30")) {
		t.Fatalf("订单总额应精确为 0.30，实际 %v", vo.TotalAmount)
	}
	if err := deps.Service.ApproveOrder(ctx, vo.OrderNo); err != nil {
		t.Fatalf("审批失败: %v", err)
	}

	if _, err := deps.Service.InboundScan(ctx, inboundReq(vo.OrderNo, fx, testsupport.D("0.100"))); err != nil {
		t.Fatalf("第一次入库 0.100 失败: %v", err)
	}
	if _, err := deps.Service.InboundScan(ctx, inboundReq(vo.OrderNo, fx, testsupport.D("0.200"))); err != nil {
		t.Fatalf("第二次入库 0.200 失败: %v", err)
	}

	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ?", fx.Material.ID).First(&inv).Error; err != nil {
		t.Fatal(err)
	}
	if !inv.Qty.Equal(testsupport.D("0.300")) {
		t.Fatalf("库存数量应精确为 0.300，实际 %v", inv.Qty)
	}

	var debit decimal.Decimal
	gdb.Model(&model.FinAccountEntry{}).Where("account_code = ?", "1403").
		Select("COALESCE(SUM(debit_amount),0)").Scan(&debit)
	if !debit.Equal(testsupport.D("0.30")) {
		t.Fatalf("1403 借方合计应精确为 0.30，实际 %v", debit)
	}
	var costTotal decimal.Decimal
	gdb.Model(&model.FinCostCard{}).Where("product_id = ?", fx.Material.ID).
		Select("COALESCE(SUM(amount),0)").Scan(&costTotal)
	if !costTotal.Equal(testsupport.D("0.30")) {
		t.Fatalf("成本卡合计应精确为 0.30（18,4 列回读），实际 %v", costTotal)
	}
}

// 多条件查询：单号/供应商/状态/创建日期范围组合过滤。
func TestListOrders_MultiCondition(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "qry")
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	// 一张草稿单 + 一张已审批单
	draftVO, err := deps.Service.CreateOrder(ctx, purchaseDto.CreatePurchaseOrderReq{
		SupplierID: fx.Supplier.ID,
		Items:      []purchaseDto.PurchaseOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("1"), Price: testsupport.D("10")}},
	})
	if err != nil {
		t.Fatalf("创建草稿单失败: %v", err)
	}
	approvedNo := createApprovedOrder(t, deps, fx, testsupport.D("2"), testsupport.D("20"))

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	list := func(req purchaseDto.PurchaseOrderListReq) int64 {
		_, total, err := deps.Service.ListOrders(ctx, req)
		if err != nil {
			t.Fatalf("查询订单失败: %v", err)
		}
		return total
	}

	page := func(req purchaseDto.PurchaseOrderListReq) purchaseDto.PurchaseOrderListReq {
		req.Page, req.PageSize = 1, 20
		return req
	}

	if n := list(page(purchaseDto.PurchaseOrderListReq{OrderNo: draftVO.OrderNo})); n != 1 {
		t.Fatalf("按单号查询应命中 1，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{SupplierID: fx.Supplier.ID})); n != 2 {
		t.Fatalf("按供应商查询应命中 2，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{Status: purchaseService.OrderStatusDraft})); n != 1 {
		t.Fatalf("按状态(草稿)查询应命中 1，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{Status: purchaseService.OrderStatusApproved})); n != 1 {
		t.Fatalf("按状态(已审批)查询应命中 1，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{DateFrom: yesterday, DateTo: today})); n != 2 {
		t.Fatalf("日期范围含今天应命中 2，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{DateFrom: tomorrow})); n != 0 {
		t.Fatalf("起始日期为明天应命中 0，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{DateTo: yesterday})); n != 0 {
		t.Fatalf("截止日期为昨天应命中 0，实际 %d", n)
	}
	if n := list(page(purchaseDto.PurchaseOrderListReq{OrderNo: approvedNo, Status: purchaseService.OrderStatusApproved, DateFrom: today, DateTo: today})); n != 1 {
		t.Fatalf("组合条件应命中 1，实际 %d", n)
	}
}
