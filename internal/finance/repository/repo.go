// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"time"

	"erp-system/internal/finance/model"
	"gorm.io/gorm"
)

type CostCardRepository interface {
	Create(tx *gorm.DB, costCard *model.FinCostCard) error
	List(productID int64, costType string, startDate, endDate string, page, pageSize int) ([]model.FinCostCard, int64, error)
	SummaryByProduct(productID int64, startDate, endDate string) ([]*CostSummaryRow, error)
}

type CostSummaryRow struct {
	ProductID    int64   `json:"product_id"`
	MaterialCost float64 `json:"material_cost"`
	LaborCost    float64 `json:"labor_cost"`
	OverheadCost float64 `json:"overhead_cost"`
}

type AccountEntryRepository interface {
	Create(tx *gorm.DB, entry *model.FinAccountEntry) error
	List(accountCode, bizType, bizNo, startDate, endDate string, page, pageSize int) ([]model.FinAccountEntry, int64, error)
	GetBalance(accountCode string, endDate string) (float64, error)
	// SumBalance 返回 account_code LIKE prefix+'%' 的分录在 [start, end) 区间内
	// 「借方合计 − 贷方合计」。借增科目（资产/成本/费用）结果为正，贷增科目
	// （负债/权益/收入）结果为负。start/end 为 nil 表示该方向不限制时间。
	SumBalance(prefix string, start, end *time.Time) (float64, error)
}

type BudgetRepository interface {
	Create(tx *gorm.DB, budget *model.FinBudget) error
	List(budgetType string, year, month int, page, pageSize int) ([]model.FinBudget, int64, error)
	FindByTypeAndPeriod(budgetType string, year, month int) (*model.FinBudget, error)
	UpdateUsedAmount(tx *gorm.DB, budgetID int64, amount float64) error
	// ReserveAtomic 在调用方事务内原子占用预算：仅当 used_amount+amount<=amount 时
	// 累加 used_amount 并返回 (exists=true, ok=true)。预算行不存在返回
	// (exists=false, ok=false)（表示本期未设预算，调用方可选择不强制）；
	// 预算存在但余额不足返回 (exists=true, ok=false)。单条条件 UPDATE，并发安全。
	ReserveAtomic(tx *gorm.DB, budgetType string, year, month int, amount float64) (exists bool, ok bool, err error)
	// ReleaseAtomic 在调用方事务内原子释放预算：used_amount 扣减 amount，
	// 用 GREATEST(...,0) 兜底防止跨期/重复释放导致占用为负。命中预算行返回 true；
	// 本期未设预算返回 false（调用方视为不启用预算控制，无需释放）。
	ReleaseAtomic(tx *gorm.DB, budgetType string, year, month int, amount float64) (exists bool, err error)
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

func (r *accountEntryRepo) SumBalance(prefix string, start, end *time.Time) (float64, error) {
	var balance float64
	q := r.db.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(debit_amount), 0) - COALESCE(SUM(credit_amount), 0)").
		Where("account_code LIKE ?", prefix+"%")
	if start != nil {
		q = q.Where("created_at >= ?", *start)
	}
	if end != nil {
		q = q.Where("created_at < ?", *end)
	}
	err := q.Scan(&balance).Error
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

func (r *budgetRepo) ReserveAtomic(tx *gorm.DB, budgetType string, year, month int, amount float64) (bool, bool, error) {
	res := tx.Model(&model.FinBudget{}).
		Where("budget_type = ? AND year = ? AND month = ? AND used_amount + ? <= amount",
			budgetType, year, month, amount).
		Update("used_amount", gorm.Expr("used_amount + ?", amount))
	if res.Error != nil {
		return false, false, res.Error
	}
	if res.RowsAffected > 0 {
		return true, true, nil
	}

	// 条件更新未命中：区分「本期无预算」与「预算不足」。
	var count int64
	if err := tx.Model(&model.FinBudget{}).
		Where("budget_type = ? AND year = ? AND month = ?", budgetType, year, month).
		Count(&count).Error; err != nil {
		return false, false, err
	}
	if count == 0 {
		return false, false, nil
	}
	return true, false, nil
}

func (r *budgetRepo) ReleaseAtomic(tx *gorm.DB, budgetType string, year, month int, amount float64) (bool, error) {
	res := tx.Model(&model.FinBudget{}).
		Where("budget_type = ? AND year = ? AND month = ?", budgetType, year, month).
		Update("used_amount", gorm.Expr("GREATEST(used_amount - ?, 0)", amount))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
