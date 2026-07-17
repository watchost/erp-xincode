// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/purchase/model"
)

type PurchaseOrderRepository interface {
	Create(tx *gorm.DB, order *model.PurPurchaseOrder) error
	CreateItems(tx *gorm.DB, items []model.PurPurchaseOrderItem) error
	FindByOrderNo(orderNo string) (*model.PurPurchaseOrder, error)
	List(orderNo string, supplierID int64, status int, page, pageSize int) ([]model.PurPurchaseOrder, int64, error)
	FindItems(orderID int64) ([]model.PurPurchaseOrderItem, error)
	UpdateItemReceivedQty(tx *gorm.DB, itemID int64, qty float64) error
}

type PurchaseInboundRepository interface {
	Create(tx *gorm.DB, inbound *model.PurPurchaseInbound) error
	List(orderID int64, page, pageSize int) ([]model.PurPurchaseInbound, int64, error)
}

type purchaseOrderRepo struct {
	db *gorm.DB
}

func NewPurchaseOrderRepository(db *gorm.DB) PurchaseOrderRepository {
	return &purchaseOrderRepo{db: db}
}

func (r *purchaseOrderRepo) Create(tx *gorm.DB, order *model.PurPurchaseOrder) error {
	return tx.Create(order).Error
}

func (r *purchaseOrderRepo) CreateItems(tx *gorm.DB, items []model.PurPurchaseOrderItem) error {
	return tx.Create(&items).Error
}

func (r *purchaseOrderRepo) FindByOrderNo(orderNo string) (*model.PurPurchaseOrder, error) {
	var order model.PurPurchaseOrder
	if err := r.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *purchaseOrderRepo) List(orderNo string, supplierID int64, status int, page, pageSize int) ([]model.PurPurchaseOrder, int64, error) {
	var list []model.PurPurchaseOrder
	var total int64
	query := r.db.Model(&model.PurPurchaseOrder{})
	if orderNo != "" {
		query = query.Where("order_no LIKE ?", "%"+orderNo+"%")
	}
	if supplierID > 0 {
		query = query.Where("supplier_id = ?", supplierID)
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

func (r *purchaseOrderRepo) FindItems(orderID int64) ([]model.PurPurchaseOrderItem, error) {
	var items []model.PurPurchaseOrderItem
	if err := r.db.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *purchaseOrderRepo) UpdateItemReceivedQty(tx *gorm.DB, itemID int64, qty float64) error {
	return tx.Model(&model.PurPurchaseOrderItem{}).
		Where("id = ?", itemID).
		Update("received_qty", gorm.Expr("received_qty + ?", qty)).Error
}

type purchaseInboundRepo struct {
	db *gorm.DB
}

func NewPurchaseInboundRepository(db *gorm.DB) PurchaseInboundRepository {
	return &purchaseInboundRepo{db: db}
}

func (r *purchaseInboundRepo) Create(tx *gorm.DB, inbound *model.PurPurchaseInbound) error {
	return tx.Create(inbound).Error
}

func (r *purchaseInboundRepo) List(orderID int64, page, pageSize int) ([]model.PurPurchaseInbound, int64, error) {
	var list []model.PurPurchaseInbound
	var total int64
	query := r.db.Model(&model.PurPurchaseInbound{})
	if orderID > 0 {
		query = query.Where("order_id = ?", orderID)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
