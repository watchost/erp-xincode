// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type ProWorkOrder struct {
	ID             int64     `json:"id"`
	WorkOrderNo    string    `json:"work_order_no"`
	ProductID      int64     `json:"product_id"`
	PlanQty        float64   `json:"plan_qty"`
	ProducedQty    float64   `json:"produced_qty"`
	Status         int       `json:"status"`
	PlanStartAt    time.Time `json:"plan_start_at"`
	PlanEndAt      time.Time `json:"plan_end_at"`
	CreatedBy      int64     `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	ActualStartAt  time.Time `json:"actual_start_at"`
	ActualEndAt    time.Time `json:"actual_end_at"`
}

type ProWorkOrderMaterial struct {
	ID          int64   `json:"id"`
	WorkOrderID int64   `json:"work_order_id"`
	MaterialID  int64   `json:"material_id"`
	PlanQty     float64 `json:"plan_qty"`
	IssuedQty   float64 `json:"issued_qty"`
	Unit        string  `json:"unit"`
}

type ProBom struct {
	ID              int64       `json:"id"`
	ProductID       int64       `json:"product_id"`
	BomVersion      string      `json:"bom_version"`
	IsActive        bool        `json:"is_active"`
	EffectiveStart  time.Time   `json:"effective_start"`
	EffectiveEnd    time.Time   `json:"effective_end"`
	CreatedBy       int64       `json:"created_by"`
	CreatedAt       time.Time   `json:"created_at"`
}

type ProBomItem struct {
	ID          int64   `json:"id"`
	BomID       int64   `json:"bom_id"`
	MaterialID  int64   `json:"material_id"`
	Qty         float64 `json:"qty"`
	Unit        string  `json:"unit"`
	ScrapRate   float64 `json:"scrap_rate"`
	Sequence    int     `json:"sequence"`
}

func (ProWorkOrder) TableName() string { return "prod_work_order" }
func (ProWorkOrderMaterial) TableName() string { return "prod_work_order_bom" }
func (ProBom) TableName() string { return "prod_bom" }
func (ProBomItem) TableName() string { return "prod_bom_item" }
