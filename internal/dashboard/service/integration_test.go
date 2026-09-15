// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"

	dashboardService "erp-system/internal/dashboard/service"
	"erp-system/internal/pkg/db"
	"erp-system/internal/testsupport"
)

// 仪表盘 SUM/阈值必须基于 decimal 精确回读：
//   - 物料 A 未设安全库存（NULL）→ 默认阈值 10；
//   - 物料 B 安全库存 3.5（小数阈值）；
//   - KPI 汇总值精确，预警分级 warning/danger 正确。
func TestDashboard_OverviewAndAlertsDecimal(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "dash")
	whSvc := testsupport.NewWarehouseService(gdb)
	ctx := testsupport.Ctx()

	// 物料 B：安全库存 3.5
	matB := createMaterialWithMinStock(t, gdb, "MAT-IT-dashb", "3.5")

	// A 入库 8@100（8 < 默认阈值 10 → warning，8 >= 5 非 danger）
	inbound(t, whSvc, fx.Material.MaterialCode, fx, "8", "100")
	// B 入库 1@100（1 < 3.5/2=1.75 → danger）
	inbound(t, whSvc, matB.MaterialCode, fx, "1", "100")

	svc := dashboardService.NewDashboardService(db.NewTxManager(gdb), gdb, nil, false)

	ov, err := svc.GetOverview(ctx)
	if err != nil {
		t.Fatalf("获取仪表盘总览失败: %v", err)
	}
	if !ov.TotalInventory.Equal(testsupport.D("9")) {
		t.Fatalf("库存总量应为精确 9，实际 %v", ov.TotalInventory)
	}
	if !ov.InventoryValue.Equal(testsupport.D("900")) {
		t.Fatalf("库存总值应为精确 900，实际 %v", ov.InventoryValue)
	}
	if !ov.TodayInboundQty.Equal(testsupport.D("9")) {
		t.Fatalf("今日入库应为精确 9，实际 %v", ov.TodayInboundQty)
	}
	if !ov.TodayOutboundQty.IsZero() {
		t.Fatalf("今日出库应为 0，实际 %v", ov.TodayOutboundQty)
	}
	if ov.StockAlertCount != 2 {
		t.Fatalf("预警数应为 2（默认阈值 + 小数安全库存），实际 %d", ov.StockAlertCount)
	}

	alerts, err := svc.GetStockAlerts(ctx)
	if err != nil {
		t.Fatalf("获取预警明细失败: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("预警明细应为 2 行，实际 %d", len(alerts))
	}
	levelByCode := map[string]string{}
	minByCode := map[string]string{}
	for _, a := range alerts {
		levelByCode[a.MaterialCode] = a.Level
		minByCode[a.MaterialCode] = a.MinStock.String()
	}
	if levelByCode[fx.Material.MaterialCode] != "warning" {
		t.Fatalf("无安全库存物料 8/10 应为 warning，实际 %q", levelByCode[fx.Material.MaterialCode])
	}
	if minByCode[fx.Material.MaterialCode] != "10" {
		t.Fatalf("无安全库存时阈值应回退为 10，实际 %s", minByCode[fx.Material.MaterialCode])
	}
	if levelByCode[matB.MaterialCode] != "danger" {
		t.Fatalf("安全库存 3.5、可用 1（低于半阈值 1.75）应为 danger，实际 %q", levelByCode[matB.MaterialCode])
	}
	if minByCode[matB.MaterialCode] != "3.5" {
		t.Fatalf("安全库存应精确回读 3.5，实际 %s", minByCode[matB.MaterialCode])
	}
}
