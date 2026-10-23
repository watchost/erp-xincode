// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type MaterialVO struct {
	ID             int64            `json:"id"`
	MaterialCode   string           `json:"material_code"`
	Name           string           `json:"name"`
	Spec           string           `json:"spec"`
	CategoryID     int64            `json:"category_id"`
	Unit           string           `json:"unit"`
	PurchaseUnit   string           `json:"purchase_unit"`
	StockUnit      string           `json:"stock_unit"`
	ConversionRate *decimal.Decimal `json:"conversion_rate"`
	CostMethod     int              `json:"cost_method"`
	Attributes     map[string]any   `json:"attributes"`
	Status         int              `json:"status"`
	MinStock       *decimal.Decimal `json:"min_stock"`
	MaxStock       *decimal.Decimal `json:"max_stock"`
	BatchManaged   bool             `json:"batch_managed"`
	SerialManaged  bool             `json:"serial_managed"`
	ShelfLifeDays  int              `json:"shelf_life_days"`
	CreatedAt      string           `json:"created_at"`
}

type MaterialListReq struct {
	MaterialCode string `form:"material_code" json:"material_code"`
	Name         string `form:"name" json:"name"`
	Page         int    `form:"page" json:"page" binding:"min=1"`
	PageSize     int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type SupplierVO struct {
	ID           int64          `json:"id"`
	SupplierCode string         `json:"supplier_code"`
	Name         string         `json:"name"`
	Contact      string         `json:"contact"`
	Level        int            `json:"level"`
	Attributes   map[string]any `json:"attributes"`
	CreatedAt    string         `json:"created_at"`
}

type SupplierListReq struct {
	SupplierCode string `form:"supplier_code" json:"supplier_code"`
	Name         string `form:"name" json:"name"`
	Page         int    `form:"page" json:"page" binding:"min=1"`
	PageSize     int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type WarehouseVO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
	CreatedAt string `json:"created_at"`
}

type LocationVO struct {
	ID            int64  `json:"id"`
	WarehouseID   int64  `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	LocationCode  string `json:"location_code"`
	Zone          string `json:"zone"`
	CreatedAt     string `json:"created_at"`
}

// CategoryVO 物料分类节点，Tree 接口返回时 Children 已按 code 排序装配。
type CategoryVO struct {
	ID        int64        `json:"id"`
	Code      string       `json:"code"`
	Name      string       `json:"name"`
	ParentID  *int64       `json:"parent_id"`
	Path      string       `json:"path"`
	Children  []CategoryVO `json:"children,omitempty"`
	CreatedAt string       `json:"created_at"`
}

type CategoryReq struct {
	Code     string `json:"code" binding:"required,max=32"`
	Name     string `json:"name" binding:"required,max=64"`
	ParentID *int64 `json:"parent_id"`
}

type CustomerVO struct {
	ID           int64           `json:"id"`
	CustomerCode string          `json:"customer_code"`
	Name         string          `json:"name"`
	Contact      string          `json:"contact"`
	Phone        string          `json:"phone"`
	Address      string          `json:"address"`
	CreditLimit  decimal.Decimal `json:"credit_limit"`
	Level        int             `json:"level"`
	Attributes   map[string]any  `json:"attributes"`
	Status       int             `json:"status"`
	CreatedAt    string          `json:"created_at"`
}

type CustomerReq struct {
	CustomerCode string          `json:"customer_code" binding:"required,max=64"`
	Name         string          `json:"name" binding:"required,max=128"`
	Contact      string          `json:"contact" binding:"max=64"`
	Phone        string          `json:"phone" binding:"max=32"`
	Address      string          `json:"address" binding:"max=255"`
	CreditLimit  decimal.Decimal `json:"credit_limit" binding:"decgte=0"`
	Level        int             `json:"level" binding:"min=1,max=5"`
	Status       int             `json:"status" binding:"oneof=0 1"`
	Attributes   map[string]any  `json:"attributes"`
}

// CustomerUpdateReq 客户部分更新请求（P1.5 S5）：
// 全部字段用指针，nil 表示「未传」——service 只覆盖请求中出现的字段，
// 避免 PUT 绑 model 后 Save 全列把未传的风控字段（credit_limit/level/status）清零。
type CustomerUpdateReq struct {
	CustomerCode *string          `json:"customer_code" binding:"omitempty,max=64"`
	Name         *string          `json:"name" binding:"omitempty,max=128"`
	Contact      *string          `json:"contact" binding:"omitempty,max=64"`
	Phone        *string          `json:"phone" binding:"omitempty,max=32"`
	Address      *string          `json:"address" binding:"omitempty,max=255"`
	CreditLimit  *decimal.Decimal `json:"credit_limit" binding:"decgte=0"`
	Level        *int             `json:"level" binding:"omitempty,min=1,max=5"`
	Status       *int             `json:"status" binding:"omitempty,oneof=0 1"`
	Attributes   map[string]any   `json:"attributes"`
}

// MaterialUpdateReq 物料部分更新请求（P1.6 H6）：
// nil 字段保留原值——conversion_rate 是 NOT NULL 列，客户端不传时绝不能写 NULL。
type MaterialUpdateReq struct {
	MaterialCode   *string          `json:"material_code" binding:"omitempty,max=64"`
	Name           *string          `json:"name" binding:"omitempty,max=128"`
	Spec           *string          `json:"spec" binding:"omitempty,max=128"`
	CategoryID     *int64           `json:"category_id"`
	Unit           *string          `json:"unit" binding:"omitempty,max=16"`
	PurchaseUnit   *string          `json:"purchase_unit" binding:"omitempty,max=16"`
	StockUnit      *string          `json:"stock_unit" binding:"omitempty,max=16"`
	ConversionRate *decimal.Decimal `json:"conversion_rate" binding:"decgt=0"`
	CostMethod     *int             `json:"cost_method"`
	Attributes     map[string]any   `json:"attributes"`
	Status         *int             `json:"status"`
	MinStock       *decimal.Decimal `json:"min_stock"`
	MaxStock       *decimal.Decimal `json:"max_stock"`
	BatchManaged   *bool            `json:"batch_managed"`
	SerialManaged  *bool            `json:"serial_managed"`
	ShelfLifeDays  *int             `json:"shelf_life_days"`
}

type CustomerListReq struct {
	CustomerCode string `form:"customer_code" json:"customer_code"`
	Name         string `form:"name" json:"name"`
	Page         int    `form:"page" json:"page" binding:"min=1"`
	PageSize     int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}
