// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"

	dashboardService "erp-system/internal/dashboard/service"
	"erp-system/internal/pkg/db"
	productionModel "erp-system/internal/production/model"
	productionService "erp-system/internal/production/service"
	purchaseModel "erp-system/internal/purchase/model"
	purchaseService "erp-system/internal/purchase/service"
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

// P1.5 H7：迁移 0010 状态码改版（旧 1/2/5/3/4 → 新 10/20/30/40/50）后，
// 仪表盘统计必须按新码口径计数、状态描述不得回退为「未知」。
// 夹具直接落库新状态码的采购单/工单，断言：
//   - 待审批采购订单只统计草稿（10）；
//   - 进行中工单只统计已下达（20）+ 生产中（30）；
//   - 最近采购订单五种状态的描述全部命中新码文案。
func TestDashboard_StatusCodesAfterMigration0010(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "dashst")
	ctx := testsupport.Ctx()

	// 采购订单：五种新状态码各一张
	purStatuses := []int{
		purchaseService.OrderStatusDraft,             // 10 草稿 → 待审批
		purchaseService.OrderStatusApproved,          // 20 已审批
		purchaseService.OrderStatusPartiallyReceived, // 30 部分收货
		purchaseService.OrderStatusCompleted,         // 40 已完成
		purchaseService.OrderStatusCanceled,          // 50 已取消
	}
	for i, st := range purStatuses {
		order := &purchaseModel.PurPurchaseOrder{
			OrderNo:     "PO-IT-ST-" + string(rune('A'+i)),
			SupplierID:  fx.Supplier.ID,
			Status:      st,
			TotalAmount: testsupport.D("100"),
			CreatedBy:   1,
		}
		if err := gdb.Create(order).Error; err != nil {
			t.Fatalf("创建采购订单失败(status=%d): %v", st, err)
		}
	}

	// 生产工单：已下达/生产中/已完成/草稿 各一张
	woStatuses := []int{
		productionService.WorkOrderStatusReleased,   // 20 已下达 → 进行中
		productionService.WorkOrderStatusInProgress, // 30 生产中 → 进行中
		productionService.WorkOrderStatusCompleted,  // 40 已完成 → 不计
		productionService.WorkOrderStatusDraft,      // 10 草稿 → 不计
	}
	for i, st := range woStatuses {
		wo := &productionModel.ProWorkOrder{
			WorkOrderNo: "WO-IT-ST-" + string(rune('A'+i)),
			ProductID:   fx.Material.ID,
			PlanQty:     testsupport.D("10"),
			Status:      st,
			CreatedBy:   1,
		}
		if err := gdb.Create(wo).Error; err != nil {
			t.Fatalf("创建工单失败(status=%d): %v", st, err)
		}
	}

	svc := dashboardService.NewDashboardService(db.NewTxManager(gdb), gdb, nil, false)

	ov, err := svc.GetOverview(ctx)
	if err != nil {
		t.Fatalf("获取仪表盘总览失败: %v", err)
	}
	if ov.PendingPurchaseOrders != 1 {
		t.Fatalf("待审批采购订单应为 1（仅草稿 10），实际 %d", ov.PendingPurchaseOrders)
	}
	if ov.InProgressWorkOrders != 2 {
		t.Fatalf("进行中工单应为 2（已下达 20 + 生产中 30），实际 %d", ov.InProgressWorkOrders)
	}

	orders, err := svc.GetRecentOrders(ctx)
	if err != nil {
		t.Fatalf("获取最近采购订单失败: %v", err)
	}
	if len(orders) != len(purStatuses) {
		t.Fatalf("最近采购订单应为 %d 条，实际 %d", len(purStatuses), len(orders))
	}
	descSeen := map[string]bool{}
	for _, o := range orders {
		if o.StatusDesc == "未知" {
			t.Fatalf("订单 %s 状态描述不应为「未知」（新状态码未命中）", o.OrderNo)
		}
		descSeen[o.StatusDesc] = true
	}
	for _, want := range []string{"草稿", "已审批", "部分收货", "已完成", "已取消"} {
		if !descSeen[want] {
			t.Fatalf("状态描述 %q 未出现，实际集合 %v", want, descSeen)
		}
	}
}
