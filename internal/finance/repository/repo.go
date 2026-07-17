// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/finance/model"
)

type CostCardRepository interface {
	Create(tx *gorm.DB, costCard *model.FinCostCard) error
	List(productID int64, costType string, startDate, endDate string, page, pageSize int) ([]model.FinCostCard, int64, error)
	SummaryByProduct(productID int64, startDate, endDate string) ([]*CostSummaryRow, error)
}

type CostSummaryRow struct {
	ProductID     int64   `json:"product_id"`
	MaterialCost  float64 `json:"material_cost"`
	LaborCost     float64 `json:"labor_cost"`
	OverheadCost  float64 `json:"overhead_cost"`
}

type AccountEntryRepository interface {
	Create(tx *gorm.DB, entry *model.FinAccountEntry) error
	List(accountCode, bizType, bizNo, startDate, endDate string, page, pageSize int) ([]model.FinAccountEntry, int64, error)
	GetBalance(accountCode string, endDate string) (float64, error)
}

type BudgetRepository interface {
	Create(tx *gorm.DB, budget *model.FinBudget) error
	List(budgetType string, year, month int, page, pageSize int) ([]model.FinBudget, int64, error)
	FindByTypeAndPeriod(budgetType string, year, month int) (*model.FinBudget, error)
	UpdateUsedAmount(tx *gorm.DB, budgetID int64, amount float64) error
}

type costCardRepo struct {
	db *gorm.DB
}

func NewCostCardRepository(db *gorm.DB) CostCardRepository {
	return &costCardRepo{db: db}
}

func (r *costCardRepo) Create(tx *gorm.DB, costCard *model.FinCostCard) error {
	return tx.Create(costCard).Error
}

func (r *costCardRepo) List(productID int64, costType string, startDate, endDate string, page, pageSize int) ([]model.FinCostCard, int64, error) {
	var list []model.FinCostCard
	var total int64
	query := r.db.Model(&model.FinCostCard{})
	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if costType != "" {
		query = query.Where("cost_type = ?", costType)
	}
	if startDate != "" {
		query = query.Where("cost_date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("cost_date <= ?", endDate)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("cost_date DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *costCardRepo) SummaryByProduct(productID int64, startDate, endDate string) ([]*CostSummaryRow, error) {
	var rows []*CostSummaryRow
	query := r.db.Select(`
		product_id,
		SUM(CASE WHEN cost_type = 'material' THEN amount ELSE 0 END) as material_cost,
		SUM(CASE WHEN cost_type = 'labor' THEN amount ELSE 0 END) as labor_cost,
		SUM(CASE WHEN cost_type = 'overhead' THEN amount ELSE 0 END) as overhead_cost
	`).Model(&model.FinCostCard{})

	if productID > 0 {
		query = query.Where("product_id = ?", productID)
	}
	if startDate != "" {
		query = query.Where("cost_date >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("cost_date <= ?", endDate)
	}

	if err := query.Group("product_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

type accountEntryRepo struct {
	db *gorm.DB
}

func NewAccountEntryRepository(db *gorm.DB) AccountEntryRepository {
	return &accountEntryRepo{db: db}
}

func (r *accountEntryRepo) Create(tx *gorm.DB, entry *model.FinAccountEntry) error {
	return tx.Create(entry).Error
}

func (r *accountEntryRepo) List(accountCode, bizType, bizNo, startDate, endDate string, page, pageSize int) ([]model.FinAccountEntry, int64, error) {
	var list []model.FinAccountEntry
	var total int64
	query := r.db.Model(&model.FinAccountEntry{})
	if accountCode != "" {
		query = query.Where("account_code = ?", accountCode)
	}
	if bizType != "" {
		query = query.Where("biz_type = ?", bizType)
	}
	if bizNo != "" {
		query = query.Where("biz_no LIKE ?", "%"+bizNo+"%")
	}
	if startDate != "" {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != "" {
		query = query.Where("created_at <= ?", endDate)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *accountEntryRepo) GetBalance(accountCode string, endDate string) (float64, error) {
	var balance float64
	err := r.db.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(debit_amount), 0) - COALESCE(SUM(credit_amount), 0)").
		Where("account_code = ?", accountCode).
		Where("created_at <= ?", endDate).
		Scan(&balance).Error
	return balance, err
}

type budgetRepo struct {
	db *gorm.DB
}

func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepo{db: db}
}

func (r *budgetRepo) Create(tx *gorm.DB, budget *model.FinBudget) error {
	return tx.Create(budget).Error
}

func (r *budgetRepo) List(budgetType string, year, month int, page, pageSize int) ([]model.FinBudget, int64, error) {
	var list []model.FinBudget
	var total int64
	query := r.db.Model(&model.FinBudget{})
	if budgetType != "" {
		query = query.Where("budget_type = ?", budgetType)
	}
	if year > 0 {
		query = query.Where("year = ?", year)
	}
	if month > 0 {
		query = query.Where("month = ?", month)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("year DESC, month DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *budgetRepo) FindByTypeAndPeriod(budgetType string, year, month int) (*model.FinBudget, error) {
	var budget model.FinBudget
	if err := r.db.Where("budget_type = ? AND year = ? AND month = ?", budgetType, year, month).First(&budget).Error; err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *budgetRepo) UpdateUsedAmount(tx *gorm.DB, budgetID int64, amount float64) error {
	return tx.Model(&model.FinBudget{}).
		Where("id = ?", budgetID).
		Update("used_amount", gorm.Expr("used_amount + ?", amount)).Error
}
