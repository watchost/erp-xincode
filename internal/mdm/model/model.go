// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type MdmMaterial struct {
	ID            int64     `json:"id"`
	MaterialCode  string    `json:"material_code"`
	Name          string    `json:"name"`
	Spec          string    `json:"spec"`
	CategoryID    int64     `json:"category_id"`
	Unit          string    `json:"unit"`
	CostMethod    int       `json:"cost_method"`
	Attributes    []byte    `json:"attributes"`
	Status        int       `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type MdmSupplier struct {
	ID            int64     `json:"id"`
	SupplierCode  string    `json:"supplier_code"`
	Name          string    `json:"name"`
	Contact       string    `json:"contact"`
	Level         int       `json:"level"`
	Attributes    []byte    `json:"attributes"`
	CreatedAt     time.Time `json:"created_at"`
}

type MdmWarehouse struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Type      int       `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type MdmLocation struct {
	ID           int64     `json:"id"`
	WarehouseID  int64     `json:"warehouse_id"`
	LocationCode string    `json:"location_code"`
	Zone         string    `json:"zone"`
	CreatedAt    time.Time `json:"created_at"`
}

func (MdmMaterial) TableName() string { return "mdm_material" }
func (MdmSupplier) TableName() string { return "mdm_supplier" }
func (MdmWarehouse) TableName() string { return "mdm_warehouse" }
func (MdmLocation) TableName() string { return "mdm_location" }
