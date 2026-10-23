// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/production/model"
)

type WorkOrderRepository interface {
	Create(tx *gorm.DB, order *model.ProWorkOrder) error
	CreateMaterials(tx *gorm.DB, materials []model.ProWorkOrderMaterial) error
	FindByWorkOrderNo(workOrderNo string) (*model.ProWorkOrder, error)
	List(workOrderNo string, productID int64, status int, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.ProWorkOrder, int64, error)
	FindMaterials(workOrderID int64) ([]model.ProWorkOrderMaterial, error)

	// 事务内加锁读，防止并发领料超发。
	FindByWorkOrderNoForUpdate(tx *gorm.DB, workOrderNo string) (*model.ProWorkOrder, error)
	FindMaterialForUpdate(tx *gorm.DB, id int64) (*model.ProWorkOrderMaterial, error)

	UpdateMaterialIssuedQty(tx *gorm.DB, materialID int64, qty decimal.Decimal) error
	UpdateWorkOrderStatus(tx *gorm.DB, workOrderID int64, status int, fields map[string]interface{}) error
	// AddOrderCosts 累加工单成本池（领料成本/已结转成本），须在持工单行锁的事务内调用。
	AddOrderCosts(tx *gorm.DB, workOrderID int64, issuedDelta, receivedDelta decimal.Decimal) error
}

type ProductionReceiptRepository interface {
	Create(tx *gorm.DB, receipt *model.ProProductionReceipt) error
	List(workOrderID int64, page, pageSize int) ([]model.ProProductionReceipt, int64, error)
}

type BomRepository interface {
	Create(tx *gorm.DB, bom *model.ProBom) error
	CreateItems(tx *gorm.DB, items []model.ProBomItem) error
	FindActiveByProductID(productID int64) (*model.ProBom, error)
	FindByID(bomID int64) (*model.ProBom, error)
	FindItems(bomID int64) ([]model.ProBomItem, error)
	List(productID int64, page, pageSize int) ([]model.ProBom, int64, error)
	DeactivateByProductID(tx *gorm.DB, productID int64) error
}

type workOrderRepo struct {
	db *gorm.DB
}

func NewWorkOrderRepository(db *gorm.DB) WorkOrderRepository {
	return &workOrderRepo{db: db}
}

func (r *workOrderRepo) Create(tx *gorm.DB, order *model.ProWorkOrder) error {
	return tx.Create(order).Error
}

func (r *workOrderRepo) CreateMaterials(tx *gorm.DB, materials []model.ProWorkOrderMaterial) error {
	return tx.Create(&materials).Error
}

func (r *workOrderRepo) FindByWorkOrderNo(workOrderNo string) (*model.ProWorkOrder, error) {
	var order model.ProWorkOrder
	if err := r.db.Where("work_order_no = ?", workOrderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *workOrderRepo) FindByWorkOrderNoForUpdate(tx *gorm.DB, workOrderNo string) (*model.ProWorkOrder, error) {
	var order model.ProWorkOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("work_order_no = ?", workOrderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *workOrderRepo) FindMaterialForUpdate(tx *gorm.DB, id int64) (*model.ProWorkOrderMaterial, error) {
	var m model.ProWorkOrderMaterial
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *workOrderRepo) List(workOrderNo string, productID int64, status int, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.ProWorkOrder, int64, error) {
	var list []model.ProWorkOrder
	var total int64
	query := r.db.Model(&model.ProWorkOrder{}).Scopes(scopes...)
	if workOrderNo != "" {
		query = query.Where("work_order_no LIKE ?", "%"+workOrderNo+"%")
	}
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *workOrderRepo) FindMaterials(workOrderID int64) ([]model.ProWorkOrderMaterial, error) {
	var materials []model.ProWorkOrderMaterial
	if err := r.db.Where("work_order_id = ?", workOrderID).Find(&materials).Error; err != nil {
		return nil, err
	}
	return materials, nil
}

func (r *workOrderRepo) UpdateMaterialIssuedQty(tx *gorm.DB, materialID int64, qty decimal.Decimal) error {
	return tx.Model(&model.ProWorkOrderMaterial{}).
		Where("id = ?", materialID).
		Update("issued_qty", gorm.Expr("issued_qty + ?", qty)).Error
}

// AddOrderCosts 累加工单成本池：issuedDelta 计入领料成本、receivedDelta 计入已结转成本（可为零值跳过对应更新）。
func (r *workOrderRepo) AddOrderCosts(tx *gorm.DB, workOrderID int64, issuedDelta, receivedDelta decimal.Decimal) error {
	fields := map[string]interface{}{}
	if !issuedDelta.IsZero() {
		fields["issued_cost"] = gorm.Expr("issued_cost + ?", issuedDelta)
	}
	if !receivedDelta.IsZero() {
		fields["received_cost"] = gorm.Expr("received_cost + ?", receivedDelta)
	}
	if len(fields) == 0 {
		return nil
	}
	return tx.Model(&model.ProWorkOrder{}).Where("id = ?", workOrderID).Updates(fields).Error
}

func (r *workOrderRepo) UpdateWorkOrderStatus(tx *gorm.DB, workOrderID int64, status int, fields map[string]interface{}) error {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["status"] = status
	return tx.Model(&model.ProWorkOrder{}).Where("id = ?", workOrderID).Updates(fields).Error
}

type bomRepo struct {
	db *gorm.DB
}

func NewBomRepository(db *gorm.DB) BomRepository {
	return &bomRepo{db: db}
}

func (r *bomRepo) Create(tx *gorm.DB, bom *model.ProBom) error {
	return tx.Create(bom).Error
}

func (r *bomRepo) CreateItems(tx *gorm.DB, items []model.ProBomItem) error {
	return tx.Create(&items).Error
}

func (r *bomRepo) FindActiveByProductID(productID int64) (*model.ProBom, error) {
	var bom model.ProBom
	if err := r.db.Where("product_id = ? AND is_active = ?", productID, true).First(&bom).Error; err != nil {
		return nil, err
	}
	return &bom, nil
}

func (r *bomRepo) FindByID(bomID int64) (*model.ProBom, error) {
	var bom model.ProBom
	if err := r.db.Where("id = ?", bomID).First(&bom).Error; err != nil {
		return nil, err
	}
	return &bom, nil
}

func (r *bomRepo) FindItems(bomID int64) ([]model.ProBomItem, error) {
	var items []model.ProBomItem
	if err := r.db.Where("bom_id = ?", bomID).Order("sequence").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *bomRepo) List(productID int64, page, pageSize int) ([]model.ProBom, int64, error) {
	var list []model.ProBom
	var total int64
	query := r.db.Model(&model.ProBom{})
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *bomRepo) DeactivateByProductID(tx *gorm.DB, productID int64) error {
	return tx.Model(&model.ProBom{}).
		Where("product_id = ? AND is_active = ?", productID, true).
		Update("is_active", false).Error
}

type productionReceiptRepo struct {
	db *gorm.DB
}

func NewProductionReceiptRepository(db *gorm.DB) ProductionReceiptRepository {
	return &productionReceiptRepo{db: db}
}

func (r *productionReceiptRepo) Create(tx *gorm.DB, receipt *model.ProProductionReceipt) error {
	return tx.Create(receipt).Error
}

func (r *productionReceiptRepo) List(workOrderID int64, page, pageSize int) ([]model.ProProductionReceipt, int64, error) {
	var list []model.ProProductionReceipt
	var total int64
	query := r.db.Model(&model.ProProductionReceipt{})
	if workOrderID > 0 {
		query = query.Where("work_order_id = ?", workOrderID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
