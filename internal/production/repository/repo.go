// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/production/model"
)

type WorkOrderRepository interface {
	Create(tx *gorm.DB, order *model.ProWorkOrder) error
	CreateMaterials(tx *gorm.DB, materials []model.ProWorkOrderMaterial) error
	FindByWorkOrderNo(workOrderNo string) (*model.ProWorkOrder, error)
	List(workOrderNo string, productID int64, status int, page, pageSize int) ([]model.ProWorkOrder, int64, error)
	FindMaterials(workOrderID int64) ([]model.ProWorkOrderMaterial, error)
	UpdateMaterialIssuedQty(tx *gorm.DB, materialID int64, qty float64) error
	UpdateWorkOrderStatus(tx *gorm.DB, workOrderID int64, status int) error
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

func (r *workOrderRepo) List(workOrderNo string, productID int64, status int, page, pageSize int) ([]model.ProWorkOrder, int64, error) {
	var list []model.ProWorkOrder
	var total int64
	query := r.db.Model(&model.ProWorkOrder{})
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

func (r *workOrderRepo) UpdateMaterialIssuedQty(tx *gorm.DB, materialID int64, qty float64) error {
	return tx.Model(&model.ProWorkOrderMaterial{}).
		Where("id = ?", materialID).
		Update("issued_qty", gorm.Expr("issued_qty + ?", qty)).Error
}

func (r *workOrderRepo) UpdateWorkOrderStatus(tx *gorm.DB, workOrderID int64, status int) error {
	return tx.Model(&model.ProWorkOrder{}).
		Where("id = ?", workOrderID).
		Update("status", status).Error
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
