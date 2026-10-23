// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type CreateWorkOrderReq struct {
	ProductID   int64           `json:"product_id" binding:"required"`
	PlanQty     decimal.Decimal `json:"plan_qty" binding:"decgt=0"`
	PlanStartAt string          `json:"plan_start_at"`
	PlanEndAt   string          `json:"plan_end_at"`
}

type WorkOrderVO struct {
	ID          int64                 `json:"id"`
	WorkOrderNo string                `json:"work_order_no"`
	ProductID   int64                 `json:"product_id"`
	ProductCode string                `json:"product_code"`
	ProductName string                `json:"product_name"`
	PlanQty     decimal.Decimal       `json:"plan_qty"`
	ProducedQty decimal.Decimal       `json:"produced_qty"`
	Status      int                   `json:"status"`
	StatusDesc  string                `json:"status_desc"`
	PlanStartAt string                `json:"plan_start_at"`
	PlanEndAt   string                `json:"plan_end_at"`
	CreatedBy   int64                 `json:"created_by"`
	CreatedAt   string                `json:"created_at"`
	Materials   []WorkOrderMaterialVO `json:"materials"`
}

type WorkOrderMaterialVO struct {
	ID           int64           `json:"id"`
	MaterialID   int64           `json:"material_id"`
	MaterialCode string          `json:"material_code"`
	MaterialName string          `json:"material_name"`
	PlanQty      decimal.Decimal `json:"plan_qty"`
	IssuedQty    decimal.Decimal `json:"issued_qty"`
	Unit         string          `json:"unit"`
}

type WorkOrderListReq struct {
	WorkOrderNo string `form:"work_order_no" json:"work_order_no"`
	ProductID   int64  `form:"product_id" json:"product_id"`
	Status      int    `form:"status" json:"status"`
	Page        int    `form:"page" json:"page" binding:"min=1"`
	PageSize    int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type MaterialIssueScanReq struct {
	WorkOrderNo    string          `json:"work_order_no" binding:"required"`
	MaterialCode   string          `json:"material_code" binding:"required"`
	WarehouseCode  string          `json:"warehouse_code" binding:"required"`
	LocationCode   string          `json:"location_code" binding:"required"`
	Qty            decimal.Decimal `json:"qty" binding:"decgt=0"`
	DeviceCode     string          `json:"device_code"`
	IdempotencyKey string          `json:"-"`
}

type MaterialIssueScanRes struct {
	OutboundNo string          `json:"outbound_no"`
	Matched    bool            `json:"matched"`
	DiffQty    decimal.Decimal `json:"diff_qty"`
	AfterQty   decimal.Decimal `json:"after_qty"`
}

// ProductionReceiptReq 生产入库/完工报工扫码。
// 成品取工单 ProductID，无需扫码物料；WarehouseCode/LocationCode 为成品入仓位。
// 入库成本不接受请求传入（P1.5 S1）：由后端从工单成本池（累计领料−累计已结转）
// 按比例结转，完工批次全额收尾，保证库存计价、财务凭证与 5001 轧平一致。
type ProductionReceiptReq struct {
	WorkOrderNo    string          `json:"work_order_no" binding:"required"`
	WarehouseCode  string          `json:"warehouse_code" binding:"required"`
	LocationCode   string          `json:"location_code" binding:"required"`
	Qty            decimal.Decimal `json:"qty" binding:"decgt=0"`
	DeviceCode     string          `json:"device_code"`
	IdempotencyKey string          `json:"-"`
}

type ProductionReceiptRes struct {
	ReceiptNo       string          `json:"receipt_no"`
	Matched         bool            `json:"matched"`
	AfterQty        decimal.Decimal `json:"after_qty"`
	WorkOrderStatus int             `json:"work_order_status"`
	Completed       bool            `json:"completed"`
}

type CreateBomReq struct {
	ProductID      int64        `json:"product_id" binding:"required"`
	BomVersion     string       `json:"bom_version"`
	EffectiveStart string       `json:"effective_start"`
	Items          []BomItemReq `json:"items" binding:"required,min=1,dive"`
}

type BomItemReq struct {
	MaterialID int64           `json:"material_id" binding:"required"`
	Qty        decimal.Decimal `json:"qty" binding:"decgt=0"`
	Unit       string          `json:"unit" binding:"required"`
	ScrapRate  decimal.Decimal `json:"scrap_rate" binding:"decgte=0"`
	Sequence   int             `json:"sequence"`
}

type BomVO struct {
	ID             int64       `json:"id"`
	ProductID      int64       `json:"product_id"`
	ProductCode    string      `json:"product_code"`
	ProductName    string      `json:"product_name"`
	BomVersion     string      `json:"bom_version"`
	IsActive       bool        `json:"is_active"`
	EffectiveStart string      `json:"effective_start"`
	EffectiveEnd   string      `json:"effective_end"`
	CreatedAt      string      `json:"created_at"`
	Items          []BomItemVO `json:"items"`
}

type BomItemVO struct {
	ID           int64           `json:"id"`
	MaterialID   int64           `json:"material_id"`
	MaterialCode string          `json:"material_code"`
	MaterialName string          `json:"material_name"`
	Qty          decimal.Decimal `json:"qty"`
	Unit         string          `json:"unit"`
	ScrapRate    decimal.Decimal `json:"scrap_rate" binding:"decgte=0"`
	Sequence     int             `json:"sequence"`
}

// ---- P2 §3.6 BOM 成本卷积与标准成本 DTO ----

type BomExplosionNodeVO struct {
	MaterialID   int64                `json:"material_id"`
	MaterialCode string               `json:"material_code"`
	MaterialName string               `json:"material_name"`
	Unit         string               `json:"unit"`
	BaseQty      decimal.Decimal      `json:"base_qty"`
	ScrapRate    decimal.Decimal      `json:"scrap_rate"`
	EffectiveQty decimal.Decimal      `json:"effective_qty"`
	StandardCost decimal.Decimal      `json:"standard_cost"`
	SubtotalCost decimal.Decimal      `json:"subtotal_cost"`
	Level        int                  `json:"level"`
	// CostPending=true 表示本节点或其下级存在缺少成本数据的末级物料，
	// StandardCost/SubtotalCost 只是已知部分之和，不是确定值。
	CostPending bool                 `json:"cost_pending"`
	Children    []BomExplosionNodeVO `json:"children,omitempty"`
}

// MissingCostMaterialVO 缺少成本数据的末级物料。
type MissingCostMaterialVO struct {
	MaterialID   int64  `json:"material_id"`
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
}

type BomCostRollupVO struct {
	ProductID     int64               `json:"product_id"`
	ProductCode   string              `json:"product_code"`
	ProductName   string              `json:"product_name"`
	MaterialCost  decimal.Decimal     `json:"material_cost"`
	LaborCost     decimal.Decimal     `json:"labor_cost"`
	OverheadCost  decimal.Decimal     `json:"overhead_cost"`
	StandardCost  decimal.Decimal     `json:"standard_cost"`
	// CostPending=true 时 MaterialCost/StandardCost 为不完整的下限值，不可作为确定标准成本使用。
	CostPending          bool                    `json:"cost_pending"`
	MissingCostMaterials []MissingCostMaterialVO `json:"missing_cost_materials"`
	ExplosionTree        *BomExplosionNodeVO     `json:"explosion_tree"`
}

type CostVarianceReq struct {
	ProductID int64  `form:"product_id" binding:"required"`
	Period    string `form:"period"`
}

type CostVarianceVO struct {
	ProductID      int64           `json:"product_id"`
	ProductCode    string          `json:"product_code"`
	ProductName    string          `json:"product_name"`
	StandardCost   decimal.Decimal `json:"standard_cost"`
	ActualAvgCost  decimal.Decimal `json:"actual_avg_cost"`
	VarianceAmount decimal.Decimal `json:"variance_amount"` // 实际 - 标准
	VarianceRate   decimal.Decimal `json:"variance_rate"`   // (实际 - 标准) / 标准 * 100%
}

