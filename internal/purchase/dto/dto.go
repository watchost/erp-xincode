// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type PurchaseOrderItemReq struct {
	MaterialID int64           `json:"material_id" binding:"required"`
	Qty        decimal.Decimal `json:"qty" binding:"decgt=0"`
	Price      decimal.Decimal `json:"price" binding:"decgt=0"`
}

type CreatePurchaseOrderReq struct {
	SupplierID   int64                  `json:"supplier_id" binding:"required"`
	Items        []PurchaseOrderItemReq `json:"items" binding:"required,min=1,dive"`
	PlanArriveAt string                 `json:"plan_arrive_at"`
}

type PurchaseOrderVO struct {
	ID           int64                 `json:"id"`
	OrderNo      string                `json:"order_no"`
	SupplierID   int64                 `json:"supplier_id"`
	SupplierName string                `json:"supplier_name"`
	Status       int                   `json:"status"`
	StatusDesc   string                `json:"status_desc"`
	TotalAmount  decimal.Decimal       `json:"total_amount"`
	PlanArriveAt string                `json:"plan_arrive_at"`
	CreatedBy    int64                 `json:"created_by"`
	CreatedAt    string                `json:"created_at"`
	Items        []PurchaseOrderItemVO `json:"items"`
}

type PurchaseOrderItemVO struct {
	ID           int64           `json:"id"`
	MaterialID   int64           `json:"material_id"`
	MaterialCode string          `json:"material_code"`
	MaterialName string          `json:"material_name"`
	Qty          decimal.Decimal `json:"qty"`
	ReceivedQty  decimal.Decimal `json:"received_qty"`
	Price        decimal.Decimal `json:"price"`
}

type PurchaseOrderListReq struct {
	OrderNo    string `form:"order_no"`
	SupplierID int64  `form:"supplier_id"`
	Status     int    `form:"status"`
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
}

// PurchaseInboundScanReq 采购入库扫码。
// WarehouseCode/LocationCode 必填（修复之前硬编码 warehouse_id=1）。
type PurchaseInboundScanReq struct {
	OrderNo        string          `json:"order_no" binding:"required"`
	MaterialCode   string          `json:"material_code" binding:"required"`
	WarehouseCode  string          `json:"warehouse_code" binding:"required"`
	LocationCode   string          `json:"location_code" binding:"required"`
	Qty            decimal.Decimal `json:"qty" binding:"decgt=0"`
	DeviceCode     string          `json:"device_code"`
	IdempotencyKey string          `json:"-"`
}

type PurchaseInboundScanRes struct {
	InboundNo string          `json:"inbound_no"`
	Matched   bool            `json:"matched"`
	DiffQty   decimal.Decimal `json:"diff_qty"`
	AfterQty  decimal.Decimal `json:"after_qty"`
}
