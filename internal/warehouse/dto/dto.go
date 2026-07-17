// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type ScanEvent struct {
	OrderNo        string  `json:"order_no"`
	MaterialCode   string  `json:"material_code"`
	LocationCode   string  `json:"location_code"`
	Qty            float64 `json:"qty"`
	DeviceCode     string  `json:"device_code"`
	WarehouseCode  string  `json:"warehouse_code"`
}

type InboundScanReq struct {
	MaterialCode  string  `json:"material_code" binding:"required"`
	LocationCode  string  `json:"location_code" binding:"required"`
	Qty           float64 `json:"qty" binding:"required,gt=0"`
	DeviceCode    string  `json:"device_code"`
	WarehouseCode string  `json:"warehouse_code"`
}

type InboundScanRes struct {
	InboundNo  string  `json:"inbound_no"`
	Matched    bool    `json:"matched"`
	DiffQty    float64 `json:"diff_qty"`
	AfterQty   float64 `json:"after_qty"`
}

type OutboundScanReq struct {
	OutboundNo    string  `json:"outbound_no"`
	MaterialCode  string  `json:"material_code" binding:"required"`
	Qty           float64 `json:"qty" binding:"required,gt=0"`
	DeviceCode    string  `json:"device_code"`
	WarehouseCode string  `json:"warehouse_code"`
}

type InventoryVO struct {
	ID            int64   `json:"id"`
	MaterialID    int64   `json:"material_id"`
	MaterialCode  string  `json:"material_code"`
	MaterialName  string  `json:"material_name"`
	WarehouseID   int64   `json:"warehouse_id"`
	WarehouseName string  `json:"warehouse_name"`
	LocationID    int64   `json:"location_id"`
	LocationCode  string  `json:"location_code"`
	Qty           float64 `json:"qty"`
	AvailableQty  float64 `json:"available_qty"`
	AvgCost       float64 `json:"avg_cost"`
	TotalValue    float64 `json:"total_value"`
}

type InventoryQuery struct {
	WarehouseID   int64  `json:"warehouse_id"`
	MaterialCode  string `json:"material_code"`
	MaterialName  string `json:"material_name"`
	Page          int    `json:"page" binding:"min=1"`
	PageSize      int    `json:"page_size" binding:"min=1,max=100"`
}

type StockAlertVO struct {
	ID            int64   `json:"id"`
	MaterialCode  string  `json:"material_code"`
	MaterialName  string  `json:"material_name"`
	WarehouseName string  `json:"warehouse_name"`
	CurrentQty    float64 `json:"current_qty"`
	MinQty        float64 `json:"min_qty"`
	Level         string  `json:"level"`
}

type StockLedgerVO struct {
	ID          int64   `json:"id"`
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
	BizType     int     `json:"biz_type"`
	BizTypeDesc string `json:"biz_type_desc"`
	BizNo       string  `json:"biz_no"`
	ChangeQty   float64 `json:"change_qty"`
	AfterQty    float64 `json:"after_qty"`
	CostAmount  float64 `json:"cost_amount"`
	CreatedAt   string  `json:"created_at"`
}
