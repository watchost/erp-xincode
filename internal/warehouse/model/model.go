// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type InvInventory struct {
	ID            int64     `json:"id"`
	MaterialID    int64     `json:"material_id"`
	WarehouseID   int64     `json:"warehouse_id"`
	LocationID    int64     `json:"location_id"`
	Qty           float64   `json:"qty"`
	AvailableQty  float64   `json:"available_qty"`
	AvgCost       float64   `json:"avg_cost"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type InvStockLedger struct {
	ID          int64     `json:"id"`
	MaterialID  int64     `json:"material_id"`
	WarehouseID int64     `json:"warehouse_id"`
	BizType     int       `json:"biz_type"`
	BizNo       string    `json:"biz_no"`
	ChangeQty   float64   `json:"change_qty"`
	AfterQty    float64   `json:"after_qty"`
	CostAmount  float64   `json:"cost_amount"`
	CreatedAt   time.Time `json:"created_at"`
}

func (InvInventory) TableName() string { return "inv_inventory" }
func (InvStockLedger) TableName() string { return "inv_stock_ledger" }
