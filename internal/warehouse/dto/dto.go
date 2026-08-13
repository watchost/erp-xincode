// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type ScanEvent struct {
	OrderNo       string  `json:"order_no"`
	MaterialCode  string  `json:"material_code"`
	LocationCode  string  `json:"location_code"`
	Qty           float64 `json:"qty"`
	DeviceCode    string  `json:"device_code"`
	WarehouseCode string  `json:"warehouse_code"`
}

// InboundScanReq 入库扫码请求。
// WarehouseCode 必填（修复之前默认 ID=1 的硬编码）。
// UnitCost 可选：采购入库时由采购单价传入；其它来源可为 0。
// IdempotencyKey 由 handler 从 header 写入，不接受 body 覆盖。
type InboundScanReq struct {
	MaterialCode   string  `json:"material_code" binding:"required"`
	WarehouseCode  string  `json:"warehouse_code" binding:"required"`
	LocationCode   string  `json:"location_code" binding:"required"`
	Qty            float64 `json:"qty" binding:"required,gt=0"`
	UnitCost       float64 `json:"unit_cost" binding:"omitempty,gte=0"`
	DeviceCode     string  `json:"device_code"`
	IdempotencyKey string  `json:"-"`
}

type InboundScanRes struct {
	InboundNo string  `json:"inbound_no"`
	Matched   bool    `json:"matched"`
	DiffQty   float64 `json:"diff_qty"`
	AfterQty  float64 `json:"after_qty"`
	AvgCost   float64 `json:"avg_cost"`
}

// OutboundScanReq 出库扫码请求。WarehouseCode/LocationCode 必填。
type OutboundScanReq struct {
	OutboundNo     string  `json:"outbound_no"`
	MaterialCode   string  `json:"material_code" binding:"required"`
	WarehouseCode  string  `json:"warehouse_code" binding:"required"`
	LocationCode   string  `json:"location_code" binding:"required"`
	Qty            float64 `json:"qty" binding:"required,gt=0"`
	DeviceCode     string  `json:"device_code"`
	IdempotencyKey string  `json:"-"`
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
	WarehouseID  int64  `form:"warehouse_id"`
	MaterialCode string `form:"material_code"`
	MaterialName string `form:"material_name"`
	Page         int    `form:"page" binding:"required,min=1"`
	PageSize     int    `form:"page_size" binding:"required,min=1,max=100"`
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
	ID           int64   `json:"id"`
	MaterialCode string  `json:"material_code"`
	MaterialName string  `json:"material_name"`
	BizType      int     `json:"biz_type"`
	BizTypeDesc  string  `json:"biz_type_desc"`
	BizNo        string  `json:"biz_no"`
	ChangeQty    float64 `json:"change_qty"`
	AfterQty     float64 `json:"after_qty"`
	CostAmount   float64 `json:"cost_amount"`
	CreatedAt    string  `json:"created_at"`
}

// MoveInput 用于"在外部事务里执行库存移动"，被采购入库/生产领料等调用。
// 调用方已经解析好 material/warehouse/location 的主键，并负责外层事务。
type MoveInput struct {
	MaterialID  int64
	WarehouseID int64
	LocationID  int64
	Qty         float64 // 入库为正、出库为负
	UnitCost    float64 // 入库单价；出库时忽略
	BizType     int
	BizNo       string
}

// MoveResult 返回更新后的库存状态。
type MoveResult struct {
	AfterQty float64
	AvgCost  float64
}
