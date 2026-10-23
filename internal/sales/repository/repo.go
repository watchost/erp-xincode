// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/sales/model"
)

type SalesOrderRepository interface {
	Create(tx *gorm.DB, order *model.SalSalesOrder) error
	CreateItems(tx *gorm.DB, items []model.SalSalesOrderItem) error
	DeleteItems(tx *gorm.DB, orderID int64) error
	FindByOrderNo(orderNo string) (*model.SalSalesOrder, error)
	FindByID(id int64) (*model.SalSalesOrder, error)
	FindItems(orderID int64) ([]model.SalSalesOrderItem, error)
	// FindItemsTx 事务内版本：高并发扫码下，事务内另起连接读明细会在
	// 连接池耗尽时与行锁等待互相饿死（池内全部连接都在等行锁、持锁事务
	// 又在等池里空出连接），事务内读必须复用同一 tx 连接。
	FindItemsTx(tx *gorm.DB, orderID int64) ([]model.SalSalesOrderItem, error)
	List(orderNo string, customerID int64, status int, dateFrom, dateTo string, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.SalSalesOrder, int64, error)

	// 事务内方法：供出库/审批/取消在同一事务里锁定订单/明细。
	FindByOrderNoForUpdate(tx *gorm.DB, orderNo string) (*model.SalSalesOrder, error)
	FindItemForUpdate(tx *gorm.DB, itemID int64) (*model.SalSalesOrderItem, error)
	UpdateItemShippedQty(tx *gorm.DB, itemID int64, qty decimal.Decimal) error
	UpdateOrderStatus(tx *gorm.DB, orderID int64, status int, fields map[string]interface{}) error

	// SumOccupiedAmount 客户已用信用额度：已审批未完成（20/30）订单总额合计。
	SumOccupiedAmount(customerID int64) (decimal.Decimal, error)
	// SumOccupiedAmountTx 事务内版本（审批时在锁内复检，复用 tx 连接）。
	SumOccupiedAmountTx(tx *gorm.DB, customerID int64) (decimal.Decimal, error)
}

type SalesOutboundRepository interface {
	Create(tx *gorm.DB, outbound *model.SalSalesOutbound) error
	CreateItems(tx *gorm.DB, items []model.SalSalesOutboundItem) error
	FindByOutboundNo(outboundNo string) (*model.SalSalesOutbound, error)
	FindItems(outboundID int64) ([]model.SalSalesOutboundItem, error)
	// FindItemsTx 事务内版本（同 FindItemsTx 的连接饿死说明）。
	FindItemsTx(tx *gorm.DB, outboundID int64) ([]model.SalSalesOutboundItem, error)
	List(orderID int64, page, pageSize int) ([]model.SalSalesOutbound, int64, error)

	// FindItemForUpdate 退货入库前锁定原出库明细行，防止并发超退。
	FindItemForUpdate(tx *gorm.DB, itemID int64) (*model.SalSalesOutboundItem, error)
	UpdateItemReturnedQty(tx *gorm.DB, itemID int64, qty decimal.Decimal) error
}

type SalesReturnRepository interface {
	Create(tx *gorm.DB, ret *model.SalSalesReturn) error
	CreateItems(tx *gorm.DB, items []model.SalSalesReturnItem) error
	FindByReturnNo(returnNo string) (*model.SalSalesReturn, error)
	FindByReturnNoForUpdate(tx *gorm.DB, returnNo string) (*model.SalSalesReturn, error)
	FindItems(returnID int64) ([]model.SalSalesReturnItem, error)
	UpdateStatus(tx *gorm.DB, returnID int64, status int) error
}

type salesOrderRepo struct {
	db *gorm.DB
}

func NewSalesOrderRepository(db *gorm.DB) SalesOrderRepository {
	return &salesOrderRepo{db: db}
}

func (r *salesOrderRepo) Create(tx *gorm.DB, order *model.SalSalesOrder) error {
	return tx.Create(order).Error
}

func (r *salesOrderRepo) CreateItems(tx *gorm.DB, items []model.SalSalesOrderItem) error {
	return tx.Create(&items).Error
}

func (r *salesOrderRepo) DeleteItems(tx *gorm.DB, orderID int64) error {
	return tx.Where("order_id = ?", orderID).Delete(&model.SalSalesOrderItem{}).Error
}

func (r *salesOrderRepo) FindByOrderNo(orderNo string) (*model.SalSalesOrder, error) {
	var order model.SalSalesOrder
	if err := r.db.Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *salesOrderRepo) FindByID(id int64) (*model.SalSalesOrder, error) {
	var order model.SalSalesOrder
	if err := r.db.First(&order, id).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *salesOrderRepo) FindByOrderNoForUpdate(tx *gorm.DB, orderNo string) (*model.SalSalesOrder, error) {
	var order model.SalSalesOrder
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_no = ?", orderNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *salesOrderRepo) FindItemForUpdate(tx *gorm.DB, itemID int64) (*model.SalSalesOrderItem, error) {
	var item model.SalSalesOrderItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", itemID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *salesOrderRepo) FindItems(orderID int64) ([]model.SalSalesOrderItem, error) {
	return r.FindItemsTx(r.db, orderID)
}

func (r *salesOrderRepo) FindItemsTx(tx *gorm.DB, orderID int64) ([]model.SalSalesOrderItem, error) {
	var items []model.SalSalesOrderItem
	if err := tx.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *salesOrderRepo) List(orderNo string, customerID int64, status int, dateFrom, dateTo string, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.SalSalesOrder, int64, error) {
	var list []model.SalSalesOrder
	var total int64
	query := r.db.Model(&model.SalSalesOrder{}).Scopes(scopes...)
	if orderNo != "" {
		query = query.Where("order_no LIKE ?", "%"+orderNo+"%")
	}
	if customerID > 0 {
		query = query.Where("customer_id = ?", customerID)
	}
	if status > 0 {
		query = query.Where("status = ?", status)
	}
	if dateFrom != "" {
		query = query.Where("created_at >= ?::date", dateFrom)
	}
	if dateTo != "" {
		// 闭区间含当日，取次日 0 点之前
		query = query.Where("created_at < (?::date + interval '1 day')", dateTo)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *salesOrderRepo) UpdateItemShippedQty(tx *gorm.DB, itemID int64, qty decimal.Decimal) error {
	return tx.Model(&model.SalSalesOrderItem{}).
		Where("id = ?", itemID).
		Update("shipped_qty", gorm.Expr("shipped_qty + ?", qty)).Error
}

func (r *salesOrderRepo) UpdateOrderStatus(tx *gorm.DB, orderID int64, status int, fields map[string]interface{}) error {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["status"] = status
	return tx.Model(&model.SalSalesOrder{}).Where("id = ?", orderID).Updates(fields).Error
}

func (r *salesOrderRepo) SumOccupiedAmount(customerID int64) (decimal.Decimal, error) {
	return r.SumOccupiedAmountTx(r.db, customerID)
}

func (r *salesOrderRepo) SumOccupiedAmountTx(tx *gorm.DB, customerID int64) (decimal.Decimal, error) {
	var sum decimal.Decimal
	err := tx.Model(&model.SalSalesOrder{}).
		Where("customer_id = ? AND status IN ?", customerID, []int{20, 30}).
		Select("COALESCE(SUM(total_amount),0)").Scan(&sum).Error
	return sum, err
}

type salesOutboundRepo struct {
	db *gorm.DB
}

func NewSalesOutboundRepository(db *gorm.DB) SalesOutboundRepository {
	return &salesOutboundRepo{db: db}
}

func (r *salesOutboundRepo) Create(tx *gorm.DB, outbound *model.SalSalesOutbound) error {
	return tx.Create(outbound).Error
}

func (r *salesOutboundRepo) CreateItems(tx *gorm.DB, items []model.SalSalesOutboundItem) error {
	return tx.Create(&items).Error
}

func (r *salesOutboundRepo) FindByOutboundNo(outboundNo string) (*model.SalSalesOutbound, error) {
	var outbound model.SalSalesOutbound
	if err := r.db.Where("outbound_no = ?", outboundNo).First(&outbound).Error; err != nil {
		return nil, err
	}
	return &outbound, nil
}

func (r *salesOutboundRepo) FindItems(outboundID int64) ([]model.SalSalesOutboundItem, error) {
	return r.FindItemsTx(r.db, outboundID)
}

func (r *salesOutboundRepo) FindItemsTx(tx *gorm.DB, outboundID int64) ([]model.SalSalesOutboundItem, error) {
	var items []model.SalSalesOutboundItem
	if err := tx.Where("outbound_id = ?", outboundID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *salesOutboundRepo) List(orderID int64, page, pageSize int) ([]model.SalSalesOutbound, int64, error) {
	var list []model.SalSalesOutbound
	var total int64
	query := r.db.Model(&model.SalSalesOutbound{})
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

func (r *salesOutboundRepo) FindItemForUpdate(tx *gorm.DB, itemID int64) (*model.SalSalesOutboundItem, error) {
	var item model.SalSalesOutboundItem
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", itemID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *salesOutboundRepo) UpdateItemReturnedQty(tx *gorm.DB, itemID int64, qty decimal.Decimal) error {
	return tx.Model(&model.SalSalesOutboundItem{}).
		Where("id = ?", itemID).
		Update("returned_qty", gorm.Expr("returned_qty + ?", qty)).Error
}

type salesReturnRepo struct {
	db *gorm.DB
}

func NewSalesReturnRepository(db *gorm.DB) SalesReturnRepository {
	return &salesReturnRepo{db: db}
}

func (r *salesReturnRepo) Create(tx *gorm.DB, ret *model.SalSalesReturn) error {
	return tx.Create(ret).Error
}

func (r *salesReturnRepo) CreateItems(tx *gorm.DB, items []model.SalSalesReturnItem) error {
	return tx.Create(&items).Error
}

func (r *salesReturnRepo) FindByReturnNo(returnNo string) (*model.SalSalesReturn, error) {
	var ret model.SalSalesReturn
	if err := r.db.Where("return_no = ?", returnNo).First(&ret).Error; err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *salesReturnRepo) FindByReturnNoForUpdate(tx *gorm.DB, returnNo string) (*model.SalSalesReturn, error) {
	var ret model.SalSalesReturn
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("return_no = ?", returnNo).First(&ret).Error; err != nil {
		return nil, err
	}
	return &ret, nil
}

func (r *salesReturnRepo) FindItems(returnID int64) ([]model.SalSalesReturnItem, error) {
	var items []model.SalSalesReturnItem
	if err := r.db.Where("return_id = ?", returnID).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *salesReturnRepo) UpdateStatus(tx *gorm.DB, returnID int64, status int) error {
	return tx.Model(&model.SalSalesReturn{}).Where("id = ?", returnID).Update("status", status).Error
}
