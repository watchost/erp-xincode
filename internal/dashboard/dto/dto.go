// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type OverviewVO struct {
	TotalInventory       float64 `json:"total_inventory"`
	InventoryValue      float64 `json:"inventory_value"`
	TodayInboundQty     float64 `json:"today_inbound_qty"`
	TodayOutboundQty    float64 `json:"today_outbound_qty"`
	PendingPurchaseOrders int64 `json:"pending_purchase_orders"`
	InProgressWorkOrders int64 `json:"in_progress_work_orders"`
	StockAlertCount     int64 `json:"stock_alert_count"`
}

type LLMAnalysisVO struct {
	Question    string `json:"question"`
	Answer      string `json:"answer"`
	AnalysisType string `json:"analysis_type"`
}
