// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type PurchaseOrderItemReq struct {
	MaterialID int64   `json:"material_id" binding:"required"`
	Qty        float64 `json:"qty" binding:"required,gt=0"`
	Price      float64 `json:"price" binding:"required,gt=0"`
}

type CreatePurchaseOrderReq struct {
	SupplierID    int64                    `json:"supplier_id" binding:"required"`
	Items         []PurchaseOrderItemReq   `json:"items" binding:"required,min=1"`
	PlanArriveAt  string                   `json:"plan_arrive_at"`
}

type PurchaseOrderVO struct {
	ID            int64       `json:"id"`
	OrderNo       string      `json:"order_no"`
	SupplierID    int64       `json:"supplier_id"`
	SupplierName  string      `json:"supplier_name"`
	Status        int         `json:"status"`
	StatusDesc    string      `json:"status_desc"`
	TotalAmount   float64     `json:"total_amount"`
	PlanArriveAt  string      `json:"plan_arrive_at"`
	CreatedBy     int64       `json:"created_by"`
	CreatedAt     string      `json:"created_at"`
	Items         []PurchaseOrderItemVO `json:"items"`
}

type PurchaseOrderItemVO struct {
	ID          int64   `json:"id"`
	MaterialID  int64   `json:"material_id"`
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
	Qty         float64 `json:"qty"`
	ReceivedQty float64 `json:"received_qty"`
	Price       float64 `json:"price"`
}

type PurchaseOrderListReq struct {
	OrderNo     string `json:"order_no"`
	SupplierID  int64  `json:"supplier_id"`
	Status      int    `json:"status"`
	Page        int    `json:"page" binding:"min=1"`
	PageSize    int    `json:"page_size" binding:"min=1,max=100"`
}

type PurchaseInboundScanReq struct {
	OrderNo        string  `json:"order_no" binding:"required"`
	MaterialCode   string  `json:"material_code" binding:"required"`
	LocationCode   string  `json:"location_code" binding:"required"`
	Qty            float64 `json:"qty" binding:"required,gt=0"`
	DeviceCode     string  `json:"device_code"`
}

type PurchaseInboundScanRes struct {
	InboundNo  string  `json:"inbound_no"`
	Matched    bool    `json:"matched"`
	DiffQty    float64 `json:"diff_qty"`
	AfterQty   float64 `json:"after_qty"`
}
