// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type SalesOrderItemReq struct {
	MaterialID int64           `json:"material_id" binding:"required"`
	Qty        decimal.Decimal `json:"qty" binding:"decgt=0"`
	Price      decimal.Decimal `json:"price" binding:"decgte=0"`
	// Discount 折扣率，0 表示无折扣，0.1 表示九折；必须落在 [0,1)。
	Discount decimal.Decimal `json:"discount" binding:"decgte=0,declt=1"`
}

type CreateSalesOrderReq struct {
	CustomerID     int64               `json:"customer_id" binding:"required"`
	Items          []SalesOrderItemReq `json:"items" binding:"required,min=1,dive"`
	PlanDeliveryAt string              `json:"plan_delivery_at"`
	Remark         string              `json:"remark" binding:"max=500"`
}

// UpdateSalesOrderReq 仅草稿状态可整体替换明细。
type UpdateSalesOrderReq struct {
	Items          []SalesOrderItemReq `json:"items" binding:"required,min=1,dive"`
	PlanDeliveryAt string              `json:"plan_delivery_at"`
	Remark         string              `json:"remark" binding:"max=500"`
}

type SalesOrderVO struct {
	ID             int64                `json:"id"`
	OrderNo        string               `json:"order_no"`
	CustomerID     int64                `json:"customer_id"`
	CustomerName   string               `json:"customer_name"`
	Status         int                  `json:"status"`
	StatusDesc     string               `json:"status_desc"`
	TotalAmount    decimal.Decimal      `json:"total_amount"`
	DiscountAmount decimal.Decimal      `json:"discount_amount"`
	PlanDeliveryAt string               `json:"plan_delivery_at"`
	Remark         string               `json:"remark"`
	CreatedBy      int64                `json:"created_by"`
	CreatedAt      string               `json:"created_at"`
	Items          []SalesOrderItemVO   `json:"items"`
}

type SalesOrderItemVO struct {
	ID           int64           `json:"id"`
	MaterialID   int64           `json:"material_id"`
	MaterialCode string          `json:"material_code"`
	MaterialName string          `json:"material_name"`
	Qty          decimal.Decimal `json:"qty"`
	ShippedQty   decimal.Decimal `json:"shipped_qty"`
	Price        decimal.Decimal `json:"price"`
	Discount     decimal.Decimal `json:"discount"`
	Amount       decimal.Decimal `json:"amount"`
}

// SalesOrderListReq 多条件查询：单号/客户/状态/创建日期范围（YYYY-MM-DD，闭区间）。
type SalesOrderListReq struct {
	OrderNo    string `form:"order_no"`
	CustomerID int64  `form:"customer_id"`
	Status     int    `form:"status"`
	DateFrom   string `form:"date_from"`
	DateTo     string `form:"date_to"`
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
}

// SalesOutboundScanReq 销售出库扫码。
type SalesOutboundScanReq struct {
	OrderNo        string          `json:"order_no" binding:"required"`
	MaterialCode   string          `json:"material_code" binding:"required"`
	WarehouseCode  string          `json:"warehouse_code" binding:"required"`
	LocationCode   string          `json:"location_code" binding:"required"`
	Qty            decimal.Decimal `json:"qty" binding:"decgt=0"`
	DeviceCode     string          `json:"device_code"`
	IdempotencyKey string          `json:"-"`
}

type SalesOutboundScanRes struct {
	OutboundNo string          `json:"outbound_no"`
	Matched    bool            `json:"matched"`
	DiffQty    decimal.Decimal `json:"diff_qty"`
	AfterQty   decimal.Decimal `json:"after_qty"`
	CostAmount decimal.Decimal `json:"cost_amount"`
}

type SalesOutboundVO struct {
	ID           int64           `json:"id"`
	OutboundNo   string          `json:"outbound_no"`
	OrderID      int64           `json:"order_id"`
	OrderNo      string          `json:"order_no"`
	CustomerID   int64           `json:"customer_id"`
	CustomerName string          `json:"customer_name"`
	WarehouseID  int64           `json:"warehouse_id"`
	CostAmount   decimal.Decimal `json:"cost_amount"`
	CreatedBy    int64           `json:"created_by"`
	CreatedAt    string          `json:"created_at"`
}

type SalesOutboundListReq struct {
	OrderNo  string `form:"order_no"`
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
}

// CreateSalesReturnReq 退货申请：引用原出库单，明细按出库行退货。
type CreateSalesReturnReq struct {
	OutboundNo   string               `json:"outbound_no" binding:"required"`
	WarehouseCode string              `json:"warehouse_code" binding:"required"`
	LocationCode string               `json:"location_code" binding:"required"`
	Reason       string               `json:"reason" binding:"max=255"`
	Items        []SalesReturnItemReq `json:"items" binding:"required,min=1,dive"`
}

type SalesReturnItemReq struct {
	MaterialID int64           `json:"material_id" binding:"required"`
	Qty        decimal.Decimal `json:"qty" binding:"decgt=0"`
}

type SalesReturnVO struct {
	ID           int64           `json:"id"`
	ReturnNo     string          `json:"return_no"`
	OutboundID   int64           `json:"outbound_id"`
	CustomerID   int64           `json:"customer_id"`
	Status       int             `json:"status"`
	StatusDesc   string          `json:"status_desc"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	Reason       string          `json:"reason"`
	CreatedAt    string          `json:"created_at"`
}
