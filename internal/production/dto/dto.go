// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type CreateWorkOrderReq struct {
	ProductID   int64   `json:"product_id" binding:"required"`
	PlanQty     float64 `json:"plan_qty" binding:"required,gt=0"`
	PlanStartAt string  `json:"plan_start_at"`
	PlanEndAt   string  `json:"plan_end_at"`
}

type WorkOrderVO struct {
	ID          int64                 `json:"id"`
	WorkOrderNo string                `json:"work_order_no"`
	ProductID   int64                 `json:"product_id"`
	ProductCode string                `json:"product_code"`
	ProductName string                `json:"product_name"`
	PlanQty     float64               `json:"plan_qty"`
	ProducedQty float64               `json:"produced_qty"`
	Status      int                   `json:"status"`
	StatusDesc  string                `json:"status_desc"`
	PlanStartAt string                `json:"plan_start_at"`
	PlanEndAt   string                `json:"plan_end_at"`
	CreatedBy   int64                 `json:"created_by"`
	CreatedAt   string                `json:"created_at"`
	Materials   []WorkOrderMaterialVO `json:"materials"`
}

type WorkOrderMaterialVO struct {
	ID           int64   `json:"id"`
	MaterialID   int64   `json:"material_id"`
	MaterialCode string  `json:"material_code"`
	MaterialName string  `json:"material_name"`
	PlanQty      float64 `json:"plan_qty"`
	IssuedQty    float64 `json:"issued_qty"`
	Unit         string  `json:"unit"`
}

type WorkOrderListReq struct {
	WorkOrderNo string `form:"work_order_no" json:"work_order_no"`
	ProductID   int64  `form:"product_id" json:"product_id"`
	Status      int    `form:"status" json:"status"`
	Page        int    `form:"page" json:"page" binding:"min=1"`
	PageSize    int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type MaterialIssueScanReq struct {
	WorkOrderNo    string  `json:"work_order_no" binding:"required"`
	MaterialCode   string  `json:"material_code" binding:"required"`
	WarehouseCode  string  `json:"warehouse_code" binding:"required"`
	LocationCode   string  `json:"location_code" binding:"required"`
	Qty            float64 `json:"qty" binding:"required,gt=0"`
	DeviceCode     string  `json:"device_code"`
	IdempotencyKey string  `json:"-"`
}

type MaterialIssueScanRes struct {
	OutboundNo string  `json:"outbound_no"`
	Matched    bool    `json:"matched"`
	DiffQty    float64 `json:"diff_qty"`
	AfterQty   float64 `json:"after_qty"`
}

// ProductionReceiptReq 生产入库/完工报工扫码。
// 成品取工单 ProductID，无需扫码物料；WarehouseCode/LocationCode 为成品入仓位。
// UnitCost 为本次完工单位成本（标准/估算成本，P1 成本归集后改为实际结转），
// 用于成品库存移动加权计价与「借 库存商品 / 贷 生产成本」过账，必须 > 0 以保证
// 库存台账金额与财务分录一致。
type ProductionReceiptReq struct {
	WorkOrderNo    string  `json:"work_order_no" binding:"required"`
	WarehouseCode  string  `json:"warehouse_code" binding:"required"`
	LocationCode   string  `json:"location_code" binding:"required"`
	Qty            float64 `json:"qty" binding:"required,gt=0"`
	UnitCost       float64 `json:"unit_cost" binding:"required,gt=0"`
	DeviceCode     string  `json:"device_code"`
	IdempotencyKey string  `json:"-"`
}

type ProductionReceiptRes struct {
	ReceiptNo       string  `json:"receipt_no"`
	Matched         bool    `json:"matched"`
	AfterQty        float64 `json:"after_qty"`
	WorkOrderStatus int     `json:"work_order_status"`
	Completed       bool    `json:"completed"`
}

type CreateBomReq struct {
	ProductID      int64        `json:"product_id" binding:"required"`
	BomVersion     string       `json:"bom_version"`
	EffectiveStart string       `json:"effective_start"`
	Items          []BomItemReq `json:"items" binding:"required,min=1"`
}

type BomItemReq struct {
	MaterialID int64   `json:"material_id" binding:"required"`
	Qty        float64 `json:"qty" binding:"required,gt=0"`
	Unit       string  `json:"unit" binding:"required"`
	ScrapRate  float64 `json:"scrap_rate"`
	Sequence   int     `json:"sequence"`
}

type BomVO struct {
	ID             int64       `json:"id"`
	ProductID      int64       `json:"product_id"`
	ProductCode    string      `json:"product_code"`
	ProductName    string      `json:"product_name"`
	BomVersion     string      `json:"bom_version"`
	IsActive       bool        `json:"is_active"`
	EffectiveStart string      `json:"effective_start"`
	EffectiveEnd   string      `json:"effective_end"`
	CreatedAt      string      `json:"created_at"`
	Items          []BomItemVO `json:"items"`
}

type BomItemVO struct {
	ID           int64   `json:"id"`
	MaterialID   int64   `json:"material_id"`
	MaterialCode string  `json:"material_code"`
	MaterialName string  `json:"material_name"`
	Qty          float64 `json:"qty"`
	Unit         string  `json:"unit"`
	ScrapRate    float64 `json:"scrap_rate"`
	Sequence     int     `json:"sequence"`
}
