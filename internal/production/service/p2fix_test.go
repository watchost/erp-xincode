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

// P2 缺陷修复 #2：末级物料缺成本数据时标记 cost_pending，并列出缺成本物料；
// 有成本的物料不受影响（区分“成本为 0”和“缺少成本数据”）。
func TestBomRollup_MissingLeafCostMarkedPending(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc, _, _ := newProductionService(t, gdb)
	ctx := testsupport.CtxWithUser(1)
	fx := testsupport.CreateFixture(t, gdb, "bompend")

	// fx.Material 有库存成本 8；noCost 从未入库
	if _, err := testsupport.NewWarehouseService(gdb).Inbound(testsupport.Ctx(), warehouseDto.InboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("5"), UnitCost: testsupport.D("8"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	noCost := createProduct(t, gdb, "BOMPEND-NOCOST")
	product := createProduct(t, gdb, "BOMPEND-P")
	if _, err := svc.CreateBom(ctx, productionDto.CreateBomReq{
		ProductID: product.ID,
		Items: []productionDto.BomItemReq{
			{MaterialID: fx.Material.ID, Qty: testsupport.D("2"), Unit: "个", ScrapRate: testsupport.D("0")},
			{MaterialID: noCost.ID, Qty: testsupport.D("1"), Unit: "个", ScrapRate: testsupport.D("0")},
		},
	}); err != nil {
		t.Fatalf("创建 BOM 失败: %v", err)
	}

	rollup, err := svc.CalculateBomCostRollup(ctx, product.ID, testsupport.D("0"), testsupport.D("0"))
	if err != nil {
		t.Fatalf("成本卷积失败: %v", err)
	}
	if !rollup.CostPending {
		t.Fatal("存在缺成本末级物料时 cost_pending 应为 true")
	}
	if len(rollup.MissingCostMaterials) != 1 || rollup.MissingCostMaterials[0].MaterialID != noCost.ID {
		t.Fatalf("缺成本物料清单应只含 %d，实际 %+v", noCost.ID, rollup.MissingCostMaterials)
	}
	// 已知部分 = 8 × 2 = 16，仅作为下限展示
	if !rollup.MaterialCost.Equal(testsupport.D("16")) {
		t.Fatalf("已知材料成本应为 16，实际 %v", rollup.MaterialCost)
	}
	for _, c := range rollup.ExplosionTree.Children {
		if c.MaterialID == fx.Material.ID && c.CostPending {
			t.Fatal("有成本的物料不应被标记为待定")
		}
	}
}
