// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type MdmMaterial struct {
	ID             int64            `json:"id"`
	TenantID       int64            `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	MaterialCode   string           `json:"material_code"`
	Name           string           `json:"name"`
	Spec           string           `json:"spec"`
	CategoryID     int64            `json:"category_id"`
	Unit           string           `json:"unit"`
	PurchaseUnit   string           `json:"purchase_unit"`
	StockUnit      string           `json:"stock_unit"`
	ConversionRate *decimal.Decimal `gorm:"default:1" json:"conversion_rate"`
	CostMethod     int              `json:"cost_method"`
	Attributes     []byte           `json:"attributes"`
	Status         int              `json:"status"`
	MinStock       *decimal.Decimal `json:"min_stock"`
	MaxStock       *decimal.Decimal `json:"max_stock"`
	BatchManaged   bool             `json:"batch_managed"`
	SerialManaged  bool             `json:"serial_managed"`
	ShelfLifeDays  int              `json:"shelf_life_days"`
	CreatedAt      time.Time        `json:"created_at"`
	DeletedAt      gorm.DeletedAt   `gorm:"index" json:"-"`
}

// MdmMaterialCategory 物料分类树，Path 为物化路径（/1/5/），与 sys_department 同套写法。
type MdmMaterialCategory struct {
	ID        int64          `json:"id"`
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	ParentID  *int64         `json:"parent_id"`
	Path      string         `json:"path"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// MdmCustomer 客户主数据（P2 销售模块使用，P1 先建表与 CRUD）。
type MdmCustomer struct {
	ID           int64           `json:"id"`
	CustomerCode string          `json:"customer_code"`
	Name         string          `json:"name"`
	Contact      string          `json:"contact"`
	Phone        string          `json:"phone"`
	Address      string          `json:"address"`
	CreditLimit  decimal.Decimal `json:"credit_limit"`
	Level        int             `gorm:"default:3" json:"level"`
	Attributes   []byte          `json:"attributes"`
	Status       int             `gorm:"default:1" json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    gorm.DeletedAt  `gorm:"index" json:"-"`
}

type MdmSupplier struct {
	ID           int64          `json:"id"`
	TenantID     int64          `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	SupplierCode string         `json:"supplier_code"`
	Name         string         `json:"name"`
	Contact      string         `json:"contact"`
	Level        int            `json:"level"`
	Attributes   []byte         `json:"attributes"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type MdmWarehouse struct {
	ID        int64          `json:"id"`
	TenantID  int64          `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	Code      string         `json:"code"`
	Name      string         `json:"name"`
	Type      int            `json:"type"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type MdmLocation struct {
	ID           int64          `json:"id"`
	TenantID     int64          `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	WarehouseID  int64          `json:"warehouse_id"`
	LocationCode string         `json:"location_code"`
	Zone         string         `json:"zone"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MdmMaterial) TableName() string         { return "mdm_material" }
func (MdmSupplier) TableName() string         { return "mdm_supplier" }
func (MdmWarehouse) TableName() string        { return "mdm_warehouse" }
func (MdmLocation) TableName() string         { return "mdm_location" }
func (MdmMaterialCategory) TableName() string { return "mdm_material_category" }
func (MdmCustomer) TableName() string         { return "mdm_customer" }
