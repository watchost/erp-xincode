// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type ProWorkOrder struct {
	ID            int64           `json:"id"`
	WorkOrderNo   string          `json:"work_order_no"`
	ProductID     int64           `json:"product_id"`
	PlanQty       decimal.Decimal `json:"plan_qty"`
	ProducedQty   decimal.Decimal `json:"produced_qty"`
	Status        int             `json:"status"`
	PlanStartAt   time.Time       `json:"plan_start_at"`
	PlanEndAt     time.Time       `json:"plan_end_at"`
	CreatedBy     int64           `json:"created_by"`
	CreatedAt     time.Time       `json:"created_at"`
	ActualStartAt time.Time       `json:"actual_start_at"`
	ActualEndAt   time.Time       `json:"actual_end_at"`
}

type ProWorkOrderMaterial struct {
	ID          int64           `json:"id"`
	WorkOrderID int64           `json:"work_order_id"`
	MaterialID  int64           `json:"material_id"`
	PlanQty     decimal.Decimal `json:"plan_qty"`
	IssuedQty   decimal.Decimal `json:"issued_qty"`
	Unit        string          `json:"unit"`
}

type ProBom struct {
	ID             int64     `json:"id"`
	ProductID      int64     `json:"product_id"`
	BomVersion     string    `json:"bom_version"`
	IsActive       bool      `json:"is_active"`
	EffectiveStart time.Time `json:"effective_start"`
	EffectiveEnd   time.Time `json:"effective_end"`
	CreatedBy      int64     `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type ProBomItem struct {
	ID         int64           `json:"id"`
	BomID      int64           `json:"bom_id"`
	MaterialID int64           `json:"material_id"`
	Qty        decimal.Decimal `json:"qty"`
	Unit       string          `json:"unit"`
	ScrapRate  decimal.Decimal `json:"scrap_rate"`
	Sequence   int             `json:"sequence"`
}

// ProProductionReceipt 生产入库单（成品/半成品完工入库）。
type ProProductionReceipt struct {
	ID          int64           `json:"id"`
	ReceiptNo   string          `json:"receipt_no"`
	WorkOrderID int64           `json:"work_order_id"`
	MaterialID  int64           `json:"material_id"`
	WarehouseID int64           `json:"warehouse_id"`
	Qty         decimal.Decimal `json:"qty"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
	Status      int             `json:"status"`
	CreatedBy   int64           `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (ProWorkOrder) TableName() string         { return "prod_work_order" }
func (ProWorkOrderMaterial) TableName() string { return "prod_work_order_bom" }
func (ProBom) TableName() string               { return "prod_bom" }
func (ProBomItem) TableName() string           { return "prod_bom_item" }
func (ProProductionReceipt) TableName() string { return "prod_production_receipt" }
