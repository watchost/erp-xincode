// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type MaterialVO struct {
	ID            int64           `json:"id"`
	MaterialCode  string          `json:"material_code"`
	Name          string          `json:"name"`
	Spec          string          `json:"spec"`
	CategoryID    int64           `json:"category_id"`
	Unit          string          `json:"unit"`
	CostMethod    int             `json:"cost_method"`
	Attributes    map[string]any  `json:"attributes"`
	Status        int             `json:"status"`
	CreatedAt     string          `json:"created_at"`
}

type MaterialListReq struct {
	MaterialCode string `json:"material_code"`
	Name         string `json:"name"`
	Page         int    `json:"page" binding:"min=1"`
	PageSize     int    `json:"page_size" binding:"min=1,max=100"`
}

type SupplierVO struct {
	ID            int64           `json:"id"`
	SupplierCode  string          `json:"supplier_code"`
	Name          string          `json:"name"`
	Contact       string          `json:"contact"`
	Level         int             `json:"level"`
	Attributes    map[string]any  `json:"attributes"`
	CreatedAt     string          `json:"created_at"`
}

type SupplierListReq struct {
	SupplierCode string `json:"supplier_code"`
	Name         string `json:"name"`
	Page         int    `json:"page" binding:"min=1"`
	PageSize     int    `json:"page_size" binding:"min=1,max=100"`
}

type WarehouseVO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	CreatedAt string `json:"created_at"`
}

type LocationVO struct {
	ID           int64  `json:"id"`
	WarehouseID  int64  `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	LocationCode string `json:"location_code"`
	Zone         string `json:"zone"`
	CreatedAt    string `json:"created_at"`
}
