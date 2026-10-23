// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type InvInventory struct {
	ID           int64           `json:"id"`
	TenantID     int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	MaterialID   int64           `json:"material_id"`
	WarehouseID  int64           `json:"warehouse_id"`
	LocationID   int64           `json:"location_id"`
	Qty          decimal.Decimal `json:"qty"`
	AvailableQty decimal.Decimal `json:"available_qty"`
	AvgCost      decimal.Decimal `json:"avg_cost"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type InvStockLedger struct {
	ID          int64           `json:"id"`
	TenantID    int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	MaterialID  int64           `json:"material_id"`
	WarehouseID int64           `json:"warehouse_id"`
	BizType     int             `json:"biz_type"`
	BizNo       string          `json:"biz_no"`
	ChangeQty   decimal.Decimal `json:"change_qty"`
	AfterQty    decimal.Decimal `json:"after_qty"`
	CostAmount  decimal.Decimal `json:"cost_amount"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (InvInventory) TableName() string   { return "inv_inventory" }
func (InvStockLedger) TableName() string { return "inv_stock_ledger" }

// 盘点单状态常量
const (
	StockCheckStatusDraft     int16 = 10 // 草稿
	StockCheckStatusFrozen    int16 = 20 // 盘点中/已冻结
	StockCheckStatusCounted   int16 = 30 // 已录入实盘待审批
	StockCheckStatusCompleted int16 = 40 // 已审批完成(已调账)
	StockCheckStatusCancelled int16 = 50 // 已取消(已解冻)
)

// 调拨单状态常量
const (
	TransferStatusDraft     int16 = 10 // 草稿
	TransferStatusApproved  int16 = 20 // 已审批待发货
	TransferStatusShipped   int16 = 30 // 已调出发货(在途)
	TransferStatusCompleted int16 = 40 // 已接收完成
	TransferStatusCancelled int16 = 50 // 已取消
)

type InvStockCheck struct {
	ID              int64                `json:"id"`
	CheckNo         string               `json:"check_no"`
	WarehouseID     int64                `json:"warehouse_id"`
	LocationID      *int64               `json:"location_id"`
	Status          int16                `json:"status"`
	Frozen          bool                 `json:"frozen"`
	TotalDiffAmount decimal.Decimal      `json:"total_diff_amount"`
	Remark          string               `json:"remark"`
	ApprovedBy      *int64               `json:"approved_by"`
	ApprovedAt      *time.Time           `json:"approved_at"`
	CreatedBy       int64                `json:"created_by"`
	TenantID        int64                `json:"tenant_id"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	DeletedAt       *time.Time           `json:"-" gorm:"index"`
	Items           []*InvStockCheckItem `json:"items,omitempty" gorm:"foreignKey:CheckID"`
}

type InvStockCheckItem struct {
	ID         int64           `json:"id"`
	TenantID   int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	CheckID    int64           `json:"check_id"`
	MaterialID int64           `json:"material_id"`
	LocationID *int64          `json:"location_id"`
	BookQty    decimal.Decimal `json:"book_qty"`
	ActualQty  decimal.Decimal `json:"actual_qty"`
	DiffQty    decimal.Decimal `json:"diff_qty"`
	UnitCost   decimal.Decimal `json:"unit_cost"`
	DiffAmount decimal.Decimal `json:"diff_amount"`
}

type InvTransfer struct {
	ID              int64              `json:"id"`
	TransferNo      string             `json:"transfer_no"`
	FromWarehouseID int64              `json:"from_warehouse_id"`
	ToWarehouseID   int64              `json:"to_warehouse_id"`
	Status          int16              `json:"status"`
	TotalAmount     decimal.Decimal    `json:"total_amount"`
	ShippedAt       *time.Time         `json:"shipped_at"`
	ReceivedAt      *time.Time         `json:"received_at"`
	Remark          string             `json:"remark"`
	CreatedBy       int64              `json:"created_by"`
	TenantID        int64              `json:"tenant_id"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
	DeletedAt       *time.Time         `json:"-" gorm:"index"`
	Items           []*InvTransferItem `json:"items,omitempty" gorm:"foreignKey:TransferID"`
}

type InvTransferItem struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	TransferID     int64           `json:"transfer_id"`
	MaterialID     int64           `json:"material_id"`
	Qty            decimal.Decimal `json:"qty"`
	ShippedQty     decimal.Decimal `json:"shipped_qty"`
	ReceivedQty    decimal.Decimal `json:"received_qty"`
	UnitCost       decimal.Decimal `json:"unit_cost"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	FromLocationID *int64          `json:"from_location_id"`
	ToLocationID   *int64          `json:"to_location_id"`
}

func (InvStockCheck) TableName() string     { return "inv_stock_check" }
func (InvStockCheckItem) TableName() string { return "inv_stock_check_item" }
func (InvTransfer) TableName() string       { return "inv_transfer" }
func (InvTransferItem) TableName() string   { return "inv_transfer_item" }

// 序列号状态常量
const (
	SerialStatusInStock  int16 = 1 // 在库
	SerialStatusSold     int16 = 2 // 已售
	SerialStatusRepair   int16 = 3 // 在修
	SerialStatusScrapped int16 = 4 // 报废
)

type InvBatch struct {
	ID           int64           `json:"id"`
	BatchNo      string          `json:"batch_no"`
	MaterialID   int64           `json:"material_id"`
	ProducedAt   *time.Time      `json:"produced_at"`
	ExpiredAt    *time.Time      `json:"expired_at"`
	SupplierID   *int64          `json:"supplier_id"`
	InboundNo    string          `json:"inbound_no"`
	QtyIn        decimal.Decimal `json:"qty_in"`
	QtyRemaining decimal.Decimal `json:"qty_remaining"`
	TenantID     int64           `json:"tenant_id"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	DeletedAt    *time.Time      `json:"-" gorm:"index"`
}

type InvSerial struct {
	ID          int64      `json:"id"`
	SerialNo    string     `json:"serial_no"`
	MaterialID  int64      `json:"material_id"`
	BatchID     *int64     `json:"batch_id"`
	Status      int16      `json:"status"`
	WarehouseID *int64     `json:"warehouse_id"`
	LocationID  *int64     `json:"location_id"`
	InboundNo   string     `json:"inbound_no"`
	OutboundNo  string     `json:"outbound_no"`
	TenantID    int64      `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-" gorm:"index"`
}

func (InvBatch) TableName() string  { return "inv_batch" }
func (InvSerial) TableName() string { return "inv_serial" }
