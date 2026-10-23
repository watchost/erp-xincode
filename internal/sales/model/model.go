// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// SalSalesOrder 销售订单主表（迁移 0016）。
type SalSalesOrder struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	OrderNo        string          `json:"order_no"`
	CustomerID     int64           `json:"customer_id"`
	Status         int             `json:"status"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	PlanDeliveryAt *time.Time      `json:"plan_delivery_at"`
	ApprovedAt     *time.Time      `json:"approved_at"`
	ApprovedBy     *int64          `json:"approved_by"`
	Remark         string          `json:"remark"`
	CreatedBy      int64           `json:"created_by"`
	DeptID         int64           `json:"dept_id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      *time.Time      `json:"deleted_at"`
}

// SalSalesOrderItem 销售订单明细：amount = qty*price*(1-discount)（服务端计算）。
type SalSalesOrderItem struct {
	ID         int64           `json:"id"`
	TenantID   int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	OrderID    int64           `json:"order_id"`
	MaterialID int64           `json:"material_id"`
	Qty        decimal.Decimal `json:"qty"`
	ShippedQty decimal.Decimal `json:"shipped_qty"`
	Price      decimal.Decimal `json:"price"`
	Discount   decimal.Decimal `json:"discount"`
	Amount     decimal.Decimal `json:"amount"`
}

// SalSalesOutbound 销售出库单（扫码即完成，status 预留）。
// IdempotencyKey 为 *string：空键必须落 NULL——uk_sal_outbound_idem 部分唯一索引
// 只豁免 NULL，空串会被当作相同键拒绝第二次出库。
type SalSalesOutbound struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	OutboundNo     string          `json:"outbound_no"`
	OrderID        int64           `json:"order_id"`
	CustomerID     int64           `json:"customer_id"`
	WarehouseID    int64           `json:"warehouse_id"`
	Status         int             `json:"status"`
	CostAmount     decimal.Decimal `json:"cost_amount"`
	IdempotencyKey *string         `json:"idempotency_key"`
	CreatedBy      int64           `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
}

// SalSalesOutboundItem 出库明细：unit_cost 为发货时点移动加权平均成本，
// returned_qty 累计已退货数量（退货入库按 unit_cost 回填库存）。
type SalSalesOutboundItem struct {
	ID          int64           `json:"id"`
	TenantID    int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	OutboundID  int64           `json:"outbound_id"`
	MaterialID  int64           `json:"material_id"`
	Qty         decimal.Decimal `json:"qty"`
	ReturnedQty decimal.Decimal `json:"returned_qty"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
	CostAmount  decimal.Decimal `json:"cost_amount"`
}

// SalSalesReturn 销售退货单：10 待入库 → 20 已入库 → 50 已取消。
type SalSalesReturn struct {
	ID          int64           `json:"id"`
	TenantID    int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	ReturnNo    string          `json:"return_no"`
	OutboundID  int64           `json:"outbound_id"`
	CustomerID  int64           `json:"customer_id"`
	WarehouseID int64           `json:"warehouse_id"`
	Status      int             `json:"status"`
	TotalAmount decimal.Decimal `json:"total_amount"`
	Reason      string          `json:"reason"`
	CreatedBy   int64           `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SalSalesReturnItem 退货明细：amount = qty*price（原销售单价，冲减应收口径）。
type SalSalesReturnItem struct {
	ID             int64           `json:"id"`
	TenantID       int64           `json:"tenant_id"` // P3 §4.2 由租户插件在创建时写入
	ReturnID       int64           `json:"return_id"`
	OutboundItemID int64           `json:"outbound_item_id"`
	MaterialID     int64           `json:"material_id"`
	Qty            decimal.Decimal `json:"qty"`
	Price          decimal.Decimal `json:"price"`
	Amount         decimal.Decimal `json:"amount"`
}

func (SalSalesOrder) TableName() string        { return "sal_sales_order" }
func (SalSalesOrderItem) TableName() string    { return "sal_sales_order_item" }
func (SalSalesOutbound) TableName() string     { return "sal_sales_outbound" }
func (SalSalesOutboundItem) TableName() string { return "sal_sales_outbound_item" }
func (SalSalesReturn) TableName() string       { return "sal_sales_return" }
func (SalSalesReturnItem) TableName() string   { return "sal_sales_return_item" }
