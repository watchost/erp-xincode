// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"

	productionDto "erp-system/internal/production/dto"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
)

// P2 缺陷修复 #1：无任何实际成本数据（无完工入库、无库存成本）时返回 50007，不得伪造实际成本。
func TestCostVariance_NoActualCostReturns50007(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc, _, _ := newProductionService(t, gdb)
	ctx := testsupport.CtxWithUser(1)

	p := createProduct(t, gdb, "CV-NOCOST-P")
	_, err := svc.GetCostVariance(ctx, productionDto.CostVarianceReq{ProductID: p.ID})
	if appCode(err) != 50007 {
		t.Fatalf("无实际成本应返回 50007，实际 err=%v", err)
	}
}

// 有库存成本时，实际成本取真实加权均价（不再是标准成本×1.05）。
func TestCostVariance_UsesRealInventoryCost(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc, _, _ := newProductionService(t, gdb)
	ctx := testsupport.CtxWithUser(1)
	fx := testsupport.CreateFixture(t, gdb, "cvreal")

	if _, err := testsupport.NewWarehouseService(gdb).Inbound(testsupport.Ctx(), warehouseDto.InboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("4"), UnitCost: testsupport.D("12.5"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	vo, err := svc.GetCostVariance(ctx, productionDto.CostVarianceReq{ProductID: fx.Material.ID})
	if err != nil {
		t.Fatalf("成本差异分析失败: %v", err)
	}
	if !vo.ActualAvgCost.Equal(testsupport.D("12.5")) {
		t.Fatalf("实际成本应为库存均价 12.5，实际 %v", vo.ActualAvgCost)
	}
}
