// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type OverviewVO struct {
	TotalInventory        decimal.Decimal `json:"total_inventory"`
	InventoryValue        decimal.Decimal `json:"inventory_value"`
	TodayInboundQty       decimal.Decimal `json:"today_inbound_qty"`
	TodayOutboundQty      decimal.Decimal `json:"today_outbound_qty"`
	PendingPurchaseOrders int64           `json:"pending_purchase_orders"`
	InProgressWorkOrders  int64           `json:"in_progress_work_orders"`
	StockAlertCount       int64           `json:"stock_alert_count"`
}

// StockAlertVO 库存预警行：可用量低于安全库存（未设置安全库存时按默认阈值）。
type StockAlertVO struct {
	MaterialCode  string          `json:"material_code"`
	MaterialName  string          `json:"material_name"`
	WarehouseName string          `json:"warehouse_name"`
	CurrentQty    decimal.Decimal `json:"current_qty"`
	AvailableQty  decimal.Decimal `json:"available_qty"`
	MinStock      decimal.Decimal `json:"min_stock"`
	Level         string          `json:"level"` // warning / danger
}

// RecentOrderVO 最近采购订单（仪表盘最近订单列表）。
type RecentOrderVO struct {
	OrderNo      string          `json:"order_no"`
	SupplierName string          `json:"supplier_name"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	StatusDesc   string          `json:"status_desc"`
	CreatedAt    string          `json:"created_at"`
}

type LLMAnalysisVO struct {
	Question     string `json:"question"`
	Answer       string `json:"answer"`
	AnalysisType string `json:"analysis_type"`
}
