// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type PurPurchaseOrder struct {
	ID            int64     `json:"id"`
	OrderNo       string    `json:"order_no"`
	SupplierID    int64     `json:"supplier_id"`
	Status        int       `json:"status"`
	TotalAmount   float64   `json:"total_amount"`
	PlanArriveAt  time.Time `json:"plan_arrive_at"`
	CreatedBy     int64     `json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	ApprovedAt    time.Time `json:"approved_at"`
}

type PurPurchaseOrderItem struct {
	ID           int64   `json:"id"`
	OrderID      int64   `json:"order_id"`
	MaterialID   int64   `json:"material_id"`
	Qty          float64 `json:"qty"`
	ReceivedQty  float64 `json:"received_qty"`
	Price        float64 `json:"price"`
	ReceivedJSON []byte  `json:"received_json"`
}

type PurPurchaseInbound struct {
	ID          int64     `json:"id"`
	InboundNo   string    `json:"inbound_no"`
	OrderID     int64     `json:"order_id"`
	SupplierID  int64     `json:"supplier_id"`
	WarehouseID int64     `json:"warehouse_id"`
	Status      int       `json:"status"`
	CostAmount  float64   `json:"cost_amount"`
	CreatedBy   int64     `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func (PurPurchaseOrder) TableName() string { return "pur_purchase_order" }
func (PurPurchaseOrderItem) TableName() string { return "pur_purchase_order_item" }
func (PurPurchaseInbound) TableName() string { return "pur_purchase_inbound" }
