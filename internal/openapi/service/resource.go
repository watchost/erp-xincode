// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/openapi/dto"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	purchaseModel "erp-system/internal/purchase/model"
	purchaseRepo "erp-system/internal/purchase/repository"
)

// ResourceService 标准 ERP 资源 API（/openapi/v1/*），Bearer token + scope 校验后调用。
type ResourceService struct {
	db           *gorm.DB
	txManager    *db.TxManager
	orderRepo    purchaseRepo.PurchaseOrderRepository
	supplierRepo mdmRepo.SupplierRepository
	materialRepo mdmRepo.MaterialRepository
	numbers      *bizno.Generator
}

func NewResourceService(
	gdb *gorm.DB,
	txManager *db.TxManager,
	orderRepo purchaseRepo.PurchaseOrderRepository,
	supplierRepo mdmRepo.SupplierRepository,
	materialRepo mdmRepo.MaterialRepository,
	numbers *bizno.Generator,
) *ResourceService {
	return &ResourceService{
		db:           gdb,
		txManager:    txManager,
		orderRepo:    orderRepo,
		supplierRepo: supplierRepo,
		materialRepo: materialRepo,
		numbers:      numbers,
	}
}

// ListMaterials 物料分页（scope: read）。
func (s *ResourceService) ListMaterials(ctx context.Context, materialCode string, page, pageSize int) ([]dto.MaterialVO, int64, error) {
	type row struct {
		MaterialCode string `gorm:"column:material_code"`
		Name         string `gorm:"column:name"`
		Spec         string `gorm:"column:spec"`
		Unit         string `gorm:"column:unit"`
		Status       int    `gorm:"column:status"`
	}
	var rows []row
	var total int64
	query := s.db.Table("mdm_material").Where("deleted_at IS NULL")
	if materialCode != "" {
		query = query.Where("material_code LIKE ?", "%"+materialCode+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Select("material_code, name, spec, unit, status").
		Order("material_code").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	list := make([]dto.MaterialVO, 0, len(rows))
	for _, r := range rows {
		list = append(list, dto.MaterialVO{
			MaterialCode: r.MaterialCode,
			Name:         r.Name,
			Spec:         r.Spec,
			Unit:         r.Unit,
			Status:       r.Status,
		})
	}
	return list, total, nil
}

// ListInventory 库存分页（scope: read），可按物料/仓库编码过滤。
func (s *ResourceService) ListInventory(ctx context.Context, materialCode, warehouseCode string, page, pageSize int) ([]dto.InventoryVO, int64, error) {
	type row struct {
		MaterialCode  string          `gorm:"column:material_code"`
		MaterialName  string          `gorm:"column:material_name"`
		WarehouseCode string          `gorm:"column:warehouse_code"`
		WarehouseName string          `gorm:"column:warehouse_name"`
		Qty           decimal.Decimal `gorm:"column:qty"`
		AvailableQty  decimal.Decimal `gorm:"column:available_qty"`
	}
	base := func() *gorm.DB {
		q := s.db.Table("inv_inventory AS i").
			Joins("JOIN mdm_material m ON m.id = i.material_id AND m.deleted_at IS NULL").
			Joins("JOIN mdm_warehouse w ON w.id = i.warehouse_id")
		if materialCode != "" {
			q = q.Where("m.material_code = ?", materialCode)
		}
		if warehouseCode != "" {
			q = q.Where("w.code = ?", warehouseCode)
		}
		return q
	}
	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []row
	if err := base().
		Select("m.material_code, m.name AS material_name, w.code AS warehouse_code, w.name AS warehouse_name, i.qty, i.available_qty").
		Order("m.material_code, w.code").Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	list := make([]dto.InventoryVO, 0, len(rows))
	for _, r := range rows {
		list = append(list, dto.InventoryVO{
			MaterialCode:  r.MaterialCode,
			MaterialName:  r.MaterialName,
			WarehouseCode: r.WarehouseCode,
			WarehouseName: r.WarehouseName,
			Qty:           r.Qty,
			AvailableQty:  r.AvailableQty,
		})
	}
	return list, total, nil
}

// CreateOrder 创建采购订单（scope: write），初始为草稿（10），内部流程再行审批。
func (s *ResourceService) CreateOrder(ctx context.Context, req dto.OrderCreateReq) (*dto.OrderVO, error) {
	supplier, err := s.supplierRepo.FindByCode(req.SupplierCode)
	if err != nil {
		return nil, errors.New(70009, 404, "供应商不存在: "+req.SupplierCode)
	}
	if len(req.Items) == 0 {
		return nil, errors.New(10005, 400, "订单明细不能为空")
	}

	type resolvedItem struct {
		materialID int64
		qty        decimal.Decimal
		price      decimal.Decimal
	}
	items := make([]resolvedItem, 0, len(req.Items))
	total := decimal.Zero
	for _, it := range req.Items {
		mat, err := s.materialRepo.FindByCode(it.MaterialCode)
		if err != nil {
			return nil, errors.New(70009, 404, "物料不存在: "+it.MaterialCode)
		}
		if !it.Qty.IsPositive() {
			return nil, errors.New(10005, 400, "数量必须大于零: "+it.MaterialCode)
		}
		if it.Price.IsNegative() {
			return nil, errors.New(10005, 400, "单价不能为负: "+it.MaterialCode)
		}
		items = append(items, resolvedItem{materialID: mat.ID, qty: it.Qty, price: it.Price})
		total = total.Add(it.Qty.Mul(it.Price))
	}

	orderNo := s.numbers.Next(ctx, "PO")
	order := &purchaseModel.PurPurchaseOrder{
		OrderNo:     orderNo,
		SupplierID:  supplier.ID,
		Status:      10, // 草稿
		TotalAmount: total,
	}
	if req.PlanArriveAt != "" {
		if t, err := time.Parse("2006-01-02", req.PlanArriveAt); err == nil {
			order.PlanArriveAt = t
		}
	}

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.orderRepo.Create(tx, order); err != nil {
			return err
		}
		orderItems := make([]purchaseModel.PurPurchaseOrderItem, 0, len(items))
		for _, it := range items {
			orderItems = append(orderItems, purchaseModel.PurPurchaseOrderItem{
				OrderID:    order.ID,
				MaterialID: it.materialID,
				Qty:        it.qty,
				Price:      it.price,
			})
		}
		return s.orderRepo.CreateItems(tx, orderItems)
	})
	if err != nil {
		return nil, err
	}

	return s.GetOrder(ctx, orderNo)
}

// GetOrder 按单号查询订单及明细（scope: read）。
func (s *ResourceService) GetOrder(ctx context.Context, orderNo string) (*dto.OrderVO, error) {
	order, err := s.orderRepo.FindByOrderNo(orderNo)
	if err != nil {
		return nil, errors.New(70009, 404, "订单不存在: "+orderNo)
	}
	items, err := s.orderRepo.FindItems(order.ID)
	if err != nil {
		return nil, err
	}

	// 明细带出物料/供应商编码（开放 API 只暴露编码不暴露内部 ID）。
	supplierCode := ""
	if sup, err := s.supplierRepo.FindByID(order.SupplierID); err == nil {
		supplierCode = sup.SupplierCode
	}
	vo := &dto.OrderVO{
		OrderNo:      order.OrderNo,
		SupplierCode: supplierCode,
		Status:       order.Status,
		TotalAmount:  order.TotalAmount,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
	}
	for _, it := range items {
		matCode := ""
		if mat, err := s.materialRepo.FindByID(it.MaterialID); err == nil {
			matCode = mat.MaterialCode
		}
		vo.Items = append(vo.Items, dto.OrderItemVO{
			MaterialCode: matCode,
			Qty:          it.Qty,
			ReceivedQty:  it.ReceivedQty,
			Price:        it.Price,
		})
	}
	return vo, nil
}
