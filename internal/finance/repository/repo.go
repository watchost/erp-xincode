// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
)

type CostCardRepository interface {
	Create(tx *gorm.DB, costCard *model.FinCostCard) error
	List(productID int64, costType string, startDate, endDate string, page, pageSize int) ([]model.FinCostCard, int64, error)
	SummaryByProduct(productID int64, startDate, endDate string) ([]*CostSummaryRow, error)
}

type CostSummaryRow struct {
	ProductID    int64           `json:"product_id"`
	MaterialCost decimal.Decimal `json:"material_cost"`
	LaborCost    decimal.Decimal `json:"labor_cost"`
	OverheadCost decimal.Decimal `json:"overhead_cost"`
}

type AccountEntryRepository interface {
	Create(tx *gorm.DB, entry *model.FinAccountEntry) error
	List(accountCode, bizType, bizNo, startDate, endDate string, page, pageSize int) ([]model.FinAccountEntry, int64, error)
	ListByVoucher(voucherID int64) ([]model.FinAccountEntry, error)
	GetBalance(accountCode string, endDate string) (decimal.Decimal, error)
	// SumBalance 返回 account_code LIKE prefix+'%' 的分录在期间区间 [from, to) 内
	// 「借方合计 − 贷方合计」。期间取凭证主表 period（YYYY-MM，字典序可比），
	// 使期末结转凭证能归属到被结转的期间而非录入当天。from/to 为 nil 不限制。
	SumBalance(prefix string, periodFrom, periodTo *string) (decimal.Decimal, error)
	// SumByAccount 按科目分组汇总期间区间内的借贷净额（借−贷），利润表取数用。
	SumByAccount(prefix string, periodFrom, periodTo *string) ([]AccountBalanceRow, error)
	// CashFlowByBizType 按业务类型汇总现金类科目（1001/1002）的借贷发生额。
	CashFlowByBizType(periodFrom, periodTo *string) ([]CashFlowRow, error)
	ExistsByAccount(accountCode string) (bool, error)
}

// AccountBalanceRow 科目级借贷净额（借−贷）。
type AccountBalanceRow struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Net         decimal.Decimal `json:"net"`
}

// CashFlowRow 某业务类型的现金流入/流出。
type CashFlowRow struct {
	BizType string          `json:"biz_type"`
	Inflow  decimal.Decimal `json:"inflow"`
	Outflow decimal.Decimal `json:"outflow"`
}

// VoucherRepository 凭证主表。
type VoucherRepository interface {
	Create(tx *gorm.DB, voucher *model.FinVoucher) error
	FindByID(id int64) (*model.FinVoucher, error)
	List(period, bizType string, page, pageSize int) ([]model.FinVoucher, int64, error)
	ExistsPeriodClose(period string) (bool, error)
}

// AccountRepository 会计科目。
type AccountRepository interface {
	FindByID(id int64) (*model.FinAccount, error)
	FindByCode(code string) (*model.FinAccount, error)
	List() ([]model.FinAccount, error)
	Create(a *model.FinAccount) error
	Update(a *model.FinAccount) error
	Delete(id int64) error
	CountChildren(parentID int64) (int64, error)
}

type BudgetRepository interface {
	Create(tx *gorm.DB, budget *model.FinBudget) error
	List(budgetType string, year, month int, page, pageSize int) ([]model.FinBudget, int64, error)
	FindByTypeAndPeriod(budgetType string, year, month int) (*model.FinBudget, error)
	UpdateUsedAmount(tx *gorm.DB, budgetID int64, amount decimal.Decimal) error
	// ReserveAtomic 在调用方事务内原子占用预算：仅当 used_amount+amount<=amount 时
	// 累加 used_amount 并返回 (exists=true, ok=true)。预算行不存在返回
	// (exists=false, ok=false)（表示本期未设预算，调用方可选择不强制）；
	// 预算存在但余额不足返回 (exists=true, ok=false)。单条条件 UPDATE，并发安全。
	ReserveAtomic(tx *gorm.DB, budgetType string, year, month int, amount decimal.Decimal) (exists bool, ok bool, err error)
	// ReleaseAtomic 在调用方事务内原子释放预算：used_amount 扣减 amount，
	// 用 GREATEST(...,0) 兜底防止跨期/重复释放导致占用为负。命中预算行返回 true；
	// 本期未设预算返回 false（调用方视为不启用预算控制，无需释放）。
	ReleaseAtomic(tx *gorm.DB, budgetType string, year, month int, amount decimal.Decimal) (exists bool, err error)
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

func (r *accountEntryRepo) GetBalance(accountCode string, endDate string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	err := r.db.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(debit_amount), 0) - COALESCE(SUM(credit_amount), 0)").
		Where("account_code = ?", accountCode).
		Where("created_at <= ?", endDate).
		Scan(&balance).Error
	return balance, err
}

// withPeriodRange 把查询挂到凭证主表上按 period 过滤（YYYY-MM 字典序即时间序）。
func withPeriodRange(q *gorm.DB, periodFrom, periodTo *string) *gorm.DB {
	q = q.Joins("JOIN fin_voucher v ON v.id = fin_voucher_entry.voucher_id")
	if periodFrom != nil {
		q = q.Where("v.period >= ?", *periodFrom)
	}
	if periodTo != nil {
		q = q.Where("v.period < ?", *periodTo)
	}
	return q
}

func (r *accountEntryRepo) SumBalance(prefix string, periodFrom, periodTo *string) (decimal.Decimal, error) {
	var balance decimal.Decimal
	q := r.db.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(debit_amount), 0) - COALESCE(SUM(credit_amount), 0)").
		Where("account_code LIKE ?", prefix+"%")
	err := withPeriodRange(q, periodFrom, periodTo).Scan(&balance).Error
	return balance, err
}

func (r *accountEntryRepo) SumByAccount(prefix string, periodFrom, periodTo *string) ([]AccountBalanceRow, error) {
	var rows []AccountBalanceRow
	q := r.db.Model(&model.FinAccountEntry{}).
		Select("account_code, MAX(account_name) AS account_name, COALESCE(SUM(debit_amount),0) - COALESCE(SUM(credit_amount),0) AS net").
		Where("account_code LIKE ?", prefix+"%").
		Group("account_code").
		Order("account_code")
	err := withPeriodRange(q, periodFrom, periodTo).Scan(&rows).Error
	return rows, err
}

func (r *accountEntryRepo) CashFlowByBizType(periodFrom, periodTo *string) ([]CashFlowRow, error) {
	var rows []CashFlowRow
	q := r.db.Model(&model.FinAccountEntry{}).
		Select("COALESCE(fin_voucher_entry.biz_type, '') AS biz_type, COALESCE(SUM(debit_amount),0) AS inflow, COALESCE(SUM(credit_amount),0) AS outflow").
		Where("account_code IN ('1001','1002')").
		Group("fin_voucher_entry.biz_type")
	err := withPeriodRange(q, periodFrom, periodTo).Scan(&rows).Error
	return rows, err
}

func (r *accountEntryRepo) ListByVoucher(voucherID int64) ([]model.FinAccountEntry, error) {
	var list []model.FinAccountEntry
	err := r.db.Where("voucher_id = ?", voucherID).Order("entry_no").Find(&list).Error
	return list, err
}

func (r *accountEntryRepo) ExistsByAccount(accountCode string) (bool, error) {
	var count int64
	err := r.db.Model(&model.FinAccountEntry{}).Where("account_code = ?", accountCode).Count(&count).Error
	return count > 0, err
}

type voucherRepo struct {
	db *gorm.DB
}

func NewVoucherRepository(db *gorm.DB) VoucherRepository {
	return &voucherRepo{db: db}
}

func (r *voucherRepo) Create(tx *gorm.DB, voucher *model.FinVoucher) error {
	return tx.Create(voucher).Error
}

func (r *voucherRepo) FindByID(id int64) (*model.FinVoucher, error) {
	var v model.FinVoucher
	if err := r.db.Where("id = ?", id).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *voucherRepo) List(period, bizType string, page, pageSize int) ([]model.FinVoucher, int64, error) {
	var list []model.FinVoucher
	var total int64
	query := r.db.Model(&model.FinVoucher{})
	if period != "" {
		query = query.Where("period = ?", period)
	}
	if bizType != "" {
		query = query.Where("biz_type = ?", bizType)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("voucher_no DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *voucherRepo) ExistsPeriodClose(period string) (bool, error) {
	var count int64
	err := r.db.Model(&model.FinVoucher{}).
		Where("biz_type = ? AND biz_no = ?", "period_close", period).
		Count(&count).Error
	return count > 0, err
}

type accountRepo struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepo{db: db}
}

func (r *accountRepo) FindByID(id int64) (*model.FinAccount, error) {
	var a model.FinAccount
	if err := r.db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) FindByCode(code string) (*model.FinAccount, error) {
	var a model.FinAccount
	if err := r.db.Where("code = ?", code).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *accountRepo) List() ([]model.FinAccount, error) {
	var list []model.FinAccount
	err := r.db.Order("code").Find(&list).Error
	return list, err
}

func (r *accountRepo) Create(a *model.FinAccount) error {
	return r.db.Create(a).Error
}

func (r *accountRepo) Update(a *model.FinAccount) error {
	return r.db.Save(a).Error
}

func (r *accountRepo) Delete(id int64) error {
	return r.db.Delete(&model.FinAccount{}, id).Error
}

func (r *accountRepo) CountChildren(parentID int64) (int64, error) {
	var count int64
	err := r.db.Model(&model.FinAccount{}).Where("parent_id = ?", parentID).Count(&count).Error
	return count, err
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

func (r *budgetRepo) UpdateUsedAmount(tx *gorm.DB, budgetID int64, amount decimal.Decimal) error {
	return tx.Model(&model.FinBudget{}).
		Where("id = ?", budgetID).
		Update("used_amount", gorm.Expr("used_amount + ?", amount)).Error
}

func (r *budgetRepo) ReserveAtomic(tx *gorm.DB, budgetType string, year, month int, amount decimal.Decimal) (bool, bool, error) {
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

func (r *budgetRepo) ReleaseAtomic(tx *gorm.DB, budgetType string, year, month int, amount decimal.Decimal) (bool, error) {
	res := tx.Model(&model.FinBudget{}).
		Where("budget_type = ? AND year = ? AND month = ?", budgetType, year, month).
		Update("used_amount", gorm.Expr("GREATEST(used_amount - ?, 0)", amount))
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
