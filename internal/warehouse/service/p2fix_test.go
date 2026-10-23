// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"errors"
	"testing"

	mdmModel "erp-system/internal/mdm/model"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
	warehouseModel "erp-system/internal/warehouse/model"
)

func p2AppErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// P2 缺陷修复 #7：普通出库须在同一事务内按 FEFO（先到期先出）扣减批次余量。
func TestOutbound_DeductsBatchesFEFO(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "fefo")
	svc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	if _, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("35"), UnitCost: testsupport.D("10"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	batches := []struct{ no, exp, qty string }{
		{"FEFO-NOV", "2026-11-01", "10"},
		{"FEFO-DEC", "2026-12-01", "20"},
		{"FEFO-OCT", "2026-10-15", "5"}, // 最早到期
	}
	for _, b := range batches {
		exp := b.exp
		if _, err := svc.CreateBatch(ctx, warehouseDto.CreateBatchReq{
			BatchNo: b.no, MaterialID: fx.Material.ID, ExpiredAt: &exp, QtyIn: testsupport.D(b.qty),
		}, 0); err != nil {
			t.Fatalf("创建批次 %s 失败: %v", b.no, err)
		}
	}

	if _, err := svc.Outbound(ctx, warehouseDto.OutboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("12"),
	}); err != nil {
		t.Fatalf("出库失败: %v", err)
	}

	want := map[string]string{"FEFO-OCT": "0", "FEFO-NOV": "3", "FEFO-DEC": "20"}
	for no, qty := range want {
		var b warehouseModel.InvBatch
		if err := gdb.Where("batch_no = ?", no).First(&b).Error; err != nil {
			t.Fatalf("查询批次 %s 失败: %v", no, err)
		}
		if !b.QtyRemaining.Equal(testsupport.D(qty)) {
			t.Fatalf("批次 %s 余量应为 %s，实际 %v", no, qty, b.QtyRemaining)
		}
	}
}

// P2 缺陷修复 #6：盘盈物料无任何成本数据时拒绝审批（40003），不生成 0 金额凭证，且整单回滚。
func TestStockCheck_ZeroCostGainRejected(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "zc")
	svc := testsupport.NewWarehouseService(gdb)
	svc.SetFinance(testsupport.NewFinanceService(gdb))
	ctx := testsupport.Ctx()

	if _, err := svc.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("10"), UnitCost: testsupport.D("100"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	// 一个从未入过库、没有任何成本的新物料
	newMat := &mdmModel.MdmMaterial{MaterialCode: "ZC-NEW-MAT", Name: "无成本新物料", Unit: "个", CostMethod: 1, Status: 1}
	if err := gdb.Create(newMat).Error; err != nil {
		t.Fatalf("创建物料失败: %v", err)
	}

	check, err := svc.CreateStockCheck(ctx, warehouseDto.CreateStockCheckReq{
		WarehouseID: fx.Warehouse.ID, LocationID: &fx.Location.ID,
	}, 1)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	if err := svc.FreezeStockCheck(ctx, check.ID); err != nil {
		t.Fatalf("冻结失败: %v", err)
	}
	if err := svc.EntryStockCheck(ctx, check.ID, warehouseDto.EntryStockCheckReq{
		Items: []warehouseDto.EntryStockCheckItemReq{
			{MaterialID: fx.Material.ID, LocationID: &fx.Location.ID, ActualQty: testsupport.D("10")},
			{MaterialID: newMat.ID, LocationID: &fx.Location.ID, ActualQty: testsupport.D("3")},
		},
	}); err != nil {
		t.Fatalf("录入实盘失败: %v", err)
	}

	err = svc.ApproveStockCheck(ctx, check.ID, 1)
	if p2AppErrCode(err) != 40003 {
		t.Fatalf("盘盈 0 成本应以 40003 拒绝审批，实际 err=%v", err)
	}
	var vouchers, newInv int64
	gdb.Table("fin_voucher").Where("biz_no = ?", check.CheckNo).Count(&vouchers)
	gdb.Table("inv_inventory").Where("material_id = ?", newMat.ID).Count(&newInv)
	if vouchers != 0 || newInv != 0 {
		t.Fatalf("拒绝审批时应无凭证、无新物料库存：凭证 %d，库存行 %d", vouchers, newInv)
	}
}
