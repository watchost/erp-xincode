// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/pkg/db"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseModel "erp-system/internal/warehouse/model"
	warehouseService "erp-system/internal/warehouse/service"
)

func findInventory(t *testing.T, gdb *gorm.DB, materialID, warehouseID, locationID int64) warehouseModel.InvInventory {
	t.Helper()
	var inv warehouseModel.InvInventory
	if err := gdb.Where("material_id = ? AND warehouse_id = ? AND location_id = ?",
		materialID, warehouseID, locationID).First(&inv).Error; err != nil {
		t.Fatalf("查询库存失败: %v", err)
	}
	return inv
}

func ledgerCount(t *testing.T, gdb *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := gdb.Model(&warehouseModel.InvStockLedger{}).Count(&n).Error; err != nil {
		t.Fatalf("查询台账失败: %v", err)
	}
	return n
}

// 首次入库建立移动平均单价。
func TestInbound_FirstInSetsAvgCost(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "first")
	svc := testsupport.NewWarehouseService(gdb)

	res, err := svc.Inbound(testsupport.Ctx(), warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("10"),
		UnitCost:      testsupport.D("100"),
	})
	if err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	if !res.AfterQty.Equal(testsupport.D("10")) || !res.AvgCost.Equal(testsupport.D("100")) {
		t.Fatalf("首次入库结果错误: after=%v avg=%v", res.AfterQty, res.AvgCost)
	}
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("10")) || !inv.AvailableQty.Equal(testsupport.D("10")) || !inv.AvgCost.Equal(testsupport.D("100")) {
		t.Fatalf("库存状态错误: %+v", inv)
	}
	if ledgerCount(t, gdb) != 1 {
		t.Fatal("台账应写入 1 行")
	}
}

// 两次不同单价入库后，移动加权平均成本 = (10*100 + 10*200)/20 = 150。
// 对应 P0 验收：采购入库后 avg_cost 必须正确反映采购价。
func TestInbound_MovingWeightedAverage(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "avg")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()
	req := warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
	}

	req.Qty, req.UnitCost = testsupport.D("10"), testsupport.D("100")
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("第一次入库失败: %v", err)
	}
	req.Qty, req.UnitCost = testsupport.D("10"), testsupport.D("200")
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("第二次入库失败: %v", err)
	}

	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("20")) || !inv.AvailableQty.Equal(testsupport.D("20")) || !inv.AvgCost.Equal(testsupport.D("150")) {
		t.Fatalf("移动平均成本错误: %+v", inv)
	}

	// 单价为 0 的入库（来源单价未知）不得把平均成本冲为 0，只加数量。
	req.Qty, req.UnitCost = testsupport.D("5"), decimal.Zero
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("零单价入库失败: %v", err)
	}
	inv = findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("25")) || !inv.AvgCost.Equal(testsupport.D("150")) {
		t.Fatalf("零单价入库后平均成本不应变化: %+v", inv)
	}
}

// 出库按当前移动平均单价结转成本，台账成本金额 = avg_cost * qty。
func TestOutbound_CarriesAvgCost(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "out")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()
	req := warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("10"),
		UnitCost:      testsupport.D("100"),
	}
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("入库失败: %v", err)
	}

	res, err := svc.Outbound(ctx, warehouseDto.OutboundScanReq{
		OutboundNo:    "OB-TEST-001",
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("4"),
	})
	if err != nil {
		t.Fatalf("出库失败: %v", err)
	}
	if !res.AfterQty.Equal(testsupport.D("6")) {
		t.Fatalf("出库后数量应为 6，实际 %v", res.AfterQty)
	}
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("6")) || !inv.AvailableQty.Equal(testsupport.D("6")) || !inv.AvgCost.Equal(testsupport.D("100")) {
		t.Fatalf("出库后库存错误: %+v", inv)
	}
	var ledger warehouseModel.InvStockLedger
	if err := gdb.Where("biz_no = ?", "OB-TEST-001").First(&ledger).Error; err != nil {
		t.Fatalf("查不到出库台账: %v", err)
	}
	if !ledger.ChangeQty.Equal(testsupport.D("-4")) || !ledger.CostAmount.Equal(testsupport.D("400")) {
		t.Fatalf("出库台账金额错误: %+v", ledger)
	}
}

// 库存不足必须拒绝：返回 409，数量不能被扣成负数，台账不得新增。
func TestOutbound_InsufficientStockRejected(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "short")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()
	if _, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("10"),
		UnitCost:      testsupport.D("10"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}

	_, err := svc.Outbound(ctx, warehouseDto.OutboundScanReq{
		OutboundNo:    "OB-TEST-SHORT",
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("11"),
	})
	if err == nil {
		t.Fatal("超库存出库必须失败")
	}
	var appErr *apperrors.AppError
	if ok := asAppError(err, &appErr); !ok || appErr.HTTP != 409 {
		t.Fatalf("应返回 409 库存不足，实际 %v", err)
	}
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("10")) || !inv.AvailableQty.Equal(testsupport.D("10")) {
		t.Fatalf("拒绝出库后库存不应变化: %+v", inv)
	}
	if ledgerCount(t, gdb) != 1 {
		t.Fatal("拒绝出库不得新增台账行")
	}
}

// P0 验收压测：库存 100，120 个并发事务各扣 1。
// 必须恰好 100 个成功、20 个库存不足失败，最终 qty/available_qty 恰好为 0，
// 台账变更合计 -100，任何时刻都不允许超卖。
func TestConcurrentOutbound_NoOversell(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "conc")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	// 准备 100 库存
	if _, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("100"),
		UnitCost:      testsupport.D("10"),
	}); err != nil {
		t.Fatalf("准备库存失败: %v", err)
	}

	txMgr := db.NewTxManager(gdb)
	const goroutines = 120
	var wg sync.WaitGroup
	var success, insufficient, other int64
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			bizNo := fmt.Sprintf("OB-CONC-%04d", i)
			err := txMgr.WithTx(ctx, func(tx *gorm.DB) error {
				_, e := svc.ApplyOutboundTx(ctx, tx, warehouseDto.MoveInput{
					MaterialID:  fx.Material.ID,
					WarehouseID: fx.Warehouse.ID,
					LocationID:  fx.Location.ID,
					Qty:         testsupport.D("1"),
					BizType:     warehouseService.BizTypeProductionOutbound,
					BizNo:       bizNo,
				})
				return e
			})
			switch {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case isStockShort(err):
				atomic.AddInt64(&insufficient, 1)
			default:
				atomic.AddInt64(&other, 1)
				t.Errorf("并发出库出现非预期错误: %v", err)
			}
		}()
	}
	wg.Wait()

	if success != 100 {
		t.Errorf("成功扣减数应为 100，实际 %d", success)
	}
	if insufficient != 20 {
		t.Errorf("库存不足失败数应为 20，实际 %d", insufficient)
	}
	if other != 0 {
		t.Errorf("不应出现其他错误，实际 %d", other)
	}

	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.IsZero() || !inv.AvailableQty.IsZero() {
		t.Fatalf("最终库存必须恰好为 0，实际 qty=%v available=%v", inv.Qty, inv.AvailableQty)
	}

	var sumChange decimal.Decimal
	if err := gdb.Model(&warehouseModel.InvStockLedger{}).
		Where("biz_no LIKE ?", "OB-CONC-%").
		Select("COALESCE(SUM(change_qty),0)").Scan(&sumChange).Error; err != nil {
		t.Fatalf("汇总台账失败: %v", err)
	}
	if !sumChange.Equal(testsupport.D("-100")) {
		t.Fatalf("台账变更合计应为 -100，实际 %v", sumChange)
	}
}

// 三位小数数量必须精确落库与回读（NUMERIC(18,3)），不得出现浮点尾数。
func TestInbound_FractionalQtyExact(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "frac")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	res, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("0.005"),
		UnitCost:      testsupport.D("100"),
	})
	if err != nil {
		t.Fatalf("小数入库失败: %v", err)
	}
	if !res.AfterQty.Equal(testsupport.D("0.005")) {
		t.Fatalf("入库后数量应为精确 0.005，实际 %v", res.AfterQty)
	}
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("0.005")) || !inv.AvailableQty.Equal(testsupport.D("0.005")) {
		t.Fatalf("小数数量库存错误: %+v", inv)
	}
	var ledger warehouseModel.InvStockLedger
	if err := gdb.Last(&ledger).Error; err != nil {
		t.Fatalf("查台账失败: %v", err)
	}
	if !ledger.ChangeQty.Equal(testsupport.D("0.005")) {
		t.Fatalf("台账数量应为 0.005，实际 %v", ledger.ChangeQty)
	}
}

// 除不尽的移动加权平均：10@100 后再入 1@101，均价 = 100.0909（4 位标度，
// half-away-from-zero）；随后出库 3 个，结转成本 = 100.0909*3 = 300.2727
// → 金额 2 位标度 300.27。
func TestInbound_AvgRepeatingDecimalRoundsHalfUp(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "rep")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()
	req := warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
	}

	req.Qty, req.UnitCost = testsupport.D("10"), testsupport.D("100")
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("第一次入库失败: %v", err)
	}
	req.Qty, req.UnitCost = testsupport.D("1"), testsupport.D("101")
	if _, err := svc.Inbound(ctx, req); err != nil {
		t.Fatalf("第二次入库失败: %v", err)
	}
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("11")) || !inv.AvgCost.Equal(testsupport.D("100.0909")) {
		t.Fatalf("均价应为 100.0909，实际 qty=%v avg=%v", inv.Qty, inv.AvgCost)
	}

	res, err := svc.Outbound(ctx, warehouseDto.OutboundScanReq{
		OutboundNo:    "OB-REP-001",
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("3"),
	})
	if err != nil {
		t.Fatalf("出库失败: %v", err)
	}
	if !res.AfterQty.Equal(testsupport.D("8")) {
		t.Fatalf("出库后数量应为 8，实际 %v", res.AfterQty)
	}
	var ledger warehouseModel.InvStockLedger
	if err := gdb.Where("biz_no = ?", "OB-REP-001").First(&ledger).Error; err != nil {
		t.Fatalf("查不到出库台账: %v", err)
	}
	if !ledger.CostAmount.Equal(testsupport.D("300.27")) {
		t.Fatalf("出库结转成本应为 300.27，实际 %v", ledger.CostAmount)
	}
}

// P2 §3.2: 盘点全链路——创建草稿快照 -> 冻结仓库 -> 拦截出入库 -> 录入实盘(盘盈+盘亏) -> 审批自动调账并生成财务凭证 -> 自动解冻
func TestStockCheck_Flow_Freeze_Adjust_Voucher(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "check")
	svc := testsupport.NewWarehouseService(gdb)
	finSvc := testsupport.NewFinanceService(gdb)
	svc.SetFinance(finSvc)
	ctx := testsupport.Ctx()

	// 初始入库 10 个 @ 100
	_, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("10"),
		UnitCost:      testsupport.D("100"),
	})
	if err != nil {
		t.Fatalf("初始入库失败: %v", err)
	}

	// 1. 创建盘点单（草稿）
	check, err := svc.CreateStockCheck(ctx, warehouseDto.CreateStockCheckReq{
		WarehouseID: fx.Warehouse.ID,
		LocationID:  &fx.Location.ID,
		Remark:      "月度盘点",
	}, 1)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	if len(check.Items) != 1 || !check.Items[0].BookQty.Equal(testsupport.D("10")) {
		t.Fatalf("盘点单明细账面数量快照错误: %+v", check.Items)
	}

	// 2. 冻结仓库
	if err := svc.FreezeStockCheck(ctx, check.ID); err != nil {
		t.Fatalf("冻结仓库失败: %v", err)
	}

	// 3. 验证冻结期间常规出入库被拦截（40008 / 409）
	_, err = svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("5"),
		UnitCost:      testsupport.D("100"),
	})
	if err == nil {
		t.Fatal("冻结期间入库应被拦截")
	}

	// 4. 录入实盘数量：实际盘点到 12 个（盘盈 2 个）
	err = svc.EntryStockCheck(ctx, check.ID, warehouseDto.EntryStockCheckReq{
		Items: []warehouseDto.EntryStockCheckItemReq{
			{
				MaterialID: fx.Material.ID,
				LocationID: &fx.Location.ID,
				ActualQty:  testsupport.D("12"),
			},
		},
	})
	if err != nil {
		t.Fatalf("录入实盘失败: %v", err)
	}

	// 5. 审批盘点单（自动调账并生成财务凭证）
	if err := svc.ApproveStockCheck(ctx, check.ID, 1); err != nil {
		t.Fatalf("审批盘点单失败: %v", err)
	}

	// 校验库存已调账为 12 个
	inv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !inv.Qty.Equal(testsupport.D("12")) {
		t.Fatalf("调账后库存应为 12，实际为 %v", inv.Qty)
	}

	// 校验仓库已解冻，正常出库应放行
	_, err = svc.Outbound(ctx, warehouseDto.OutboundScanReq{
		OutboundNo:    "OB-CHK-TEST-001",
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("2"),
	})
	if err != nil {
		t.Fatalf("解冻后出库应成功: %v", err)
	}
}

// P2 §3.3: 调拨全链路——创建调拨单 -> 审批 -> 调出发货(扣调出仓库存并计入在途) -> 调入收货(入调入仓库存，成本原值传递)
func TestStockTransfer_Flow_Ship_Receive_CostConsistency(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "trf1")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	// 创建第二个仓库与库位作为调入目标
	toWh := testsupport.CreateWarehouse(t, gdb, "WH-TO-001", "调入目标仓")
	toLoc := testsupport.CreateLocation(t, gdb, toWh.ID, "LOC-TO-001", "A")

	// 调出仓初始入库 20 个 @ 150
	_, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("20"),
		UnitCost:      testsupport.D("150"),
	})
	if err != nil {
		t.Fatalf("初始入库失败: %v", err)
	}

	// 1. 创建调拨单：从 fx.Warehouse 调拨 8 个到 toWh
	tr, err := svc.CreateTransfer(ctx, warehouseDto.CreateTransferReq{
		FromWarehouseID: fx.Warehouse.ID,
		ToWarehouseID:   toWh.ID,
		Remark:          "跨仓调拨",
		Items: []warehouseDto.CreateTransferItemReq{
			{
				MaterialID:     fx.Material.ID,
				Qty:            testsupport.D("8"),
				FromLocationID: &fx.Location.ID,
				ToLocationID:   &toLoc.ID,
			},
		},
	}, 1)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	if tr.Status != warehouseModel.TransferStatusDraft || !tr.TotalAmount.Equal(testsupport.D("1200")) {
		t.Fatalf("调拨单状态或金额错误: status=%v amount=%v", tr.Status, tr.TotalAmount)
	}

	// 2. 审批调拨单
	if err := svc.ApproveTransfer(ctx, tr.ID, 1); err != nil {
		t.Fatalf("审批调拨单失败: %v", err)
	}

	// 3. 调出发货（扣调出仓库存，计入在途）
	if err := svc.ShipTransfer(ctx, tr.ID); err != nil {
		t.Fatalf("调出发货失败: %v", err)
	}

	fromInv := findInventory(t, gdb, fx.Material.ID, fx.Warehouse.ID, fx.Location.ID)
	if !fromInv.Qty.Equal(testsupport.D("12")) || !fromInv.AvailableQty.Equal(testsupport.D("12")) {
		t.Fatalf("调出后调出仓库存应为 12，实际为 %v", fromInv.Qty)
	}

	// 4. 调入收货（入调入仓库存，成本原值 150 跨仓传递）
	if err := svc.ReceiveTransfer(ctx, tr.ID); err != nil {
		t.Fatalf("调入收货失败: %v", err)
	}

	toInv := findInventory(t, gdb, fx.Material.ID, toWh.ID, toLoc.ID)
	if !toInv.Qty.Equal(testsupport.D("8")) || !toInv.AvgCost.Equal(testsupport.D("150")) {
		t.Fatalf("调入后调入仓库存或成本错误: qty=%v avg=%v", toInv.Qty, toInv.AvgCost)
	}
}

// 调拨库存不足时阻断拦截。
func TestStockTransfer_RejectsInsufficientStock(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "trf2")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	toWh := testsupport.CreateWarehouse(t, gdb, "WH-TO-002", "调入目标仓2")
	toLoc := testsupport.CreateLocation(t, gdb, toWh.ID, "LOC-TO-002", "A")

	// 调出仓只有 5 个
	_, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode:  fx.Material.MaterialCode,
		WarehouseCode: fx.Warehouse.Code,
		LocationCode:  fx.Location.LocationCode,
		Qty:           testsupport.D("5"),
		UnitCost:      testsupport.D("100"),
	})
	if err != nil {
		t.Fatalf("初始入库失败: %v", err)
	}

	// 尝试调拨 10 个（超出库存）
	_, err = svc.CreateTransfer(ctx, warehouseDto.CreateTransferReq{
		FromWarehouseID: fx.Warehouse.ID,
		ToWarehouseID:   toWh.ID,
		Items: []warehouseDto.CreateTransferItemReq{
			{
				MaterialID:     fx.Material.ID,
				Qty:            testsupport.D("10"),
				FromLocationID: &fx.Location.ID,
				ToLocationID:   &toLoc.ID,
			},
		},
	}, 1)
	if err == nil {
		t.Fatal("调拨数量超过可用库存时必须被拦截")
	}
}

