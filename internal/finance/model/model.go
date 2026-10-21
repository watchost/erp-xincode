// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type FinCostCard struct {
	ID         int64           `json:"id"`
	ProductID  int64           `json:"product_id"`
	CostType   string          `json:"cost_type"`
	Amount     decimal.Decimal `json:"amount"`
	CostDate   time.Time       `json:"cost_date"`
	SourceType string          `json:"source_type"`
	SourceID   int64           `json:"source_id"`
	CreatedAt  time.Time       `json:"created_at"`
}

// FinAccount 会计科目（树形，category：1资产 2负债 3权益 4成本 5收入 6费用；direction：1借 2贷）。
type FinAccount struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	ParentID  *int64    `json:"parent_id"`
	Level     int       `json:"level"`
	Category  int       `json:"category"`
	Direction int       `json:"direction"`
	IsLeaf    bool      `json:"is_leaf"`
	CreatedAt time.Time `json:"created_at"`
}

// FinVoucher 记账凭证主表：一张凭证一个凭证号，期间归属用于报表与期末结转。
type FinVoucher struct {
	ID          int64           `json:"id"`
	VoucherNo   string          `json:"voucher_no"`
	Period      string          `json:"period"` // YYYY-MM
	Summary     string          `json:"summary"`
	BizType     string          `json:"biz_type"`
	BizNo       string          `json:"biz_no"`
	DebitTotal  decimal.Decimal `json:"debit_total"`
	CreditTotal decimal.Decimal `json:"credit_total"`
	PostedAt    time.Time       `json:"posted_at"`
	CreatedBy   int64           `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

// FinAccountEntry 凭证明细行（分录），挂在 FinVoucher 主表下。
type FinAccountEntry struct {
	ID           int64           `json:"id"`
	VoucherID    int64           `json:"voucher_id"`
	EntryNo      string          `json:"entry_no"`
	AccountCode  string          `json:"account_code"`
	AccountName  string          `json:"account_name"`
	DebitAmount  decimal.Decimal `json:"debit_amount"`
	CreditAmount decimal.Decimal `json:"credit_amount"`
	Balance      decimal.Decimal `json:"balance"`
	BizType      string          `json:"biz_type"`
	BizNo        string          `json:"biz_no"`
	Description  string          `json:"description"`
	CreatedAt    time.Time       `json:"created_at"`
}

type FinBudget struct {
	ID         int64           `json:"id"`
	BudgetNo   string          `json:"budget_no"`
	BudgetType string          `json:"budget_type"`
	Amount     decimal.Decimal `json:"amount"`
	UsedAmount decimal.Decimal `json:"used_amount"`
	Year       int             `json:"year"`
	Month      int             `json:"month"`
	Status     int             `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (FinCostCard) TableName() string     { return "fin_cost" }
func (FinAccount) TableName() string      { return "fin_account" }
func (FinVoucher) TableName() string      { return "fin_voucher" }
func (FinAccountEntry) TableName() string { return "fin_voucher_entry" }
func (FinBudget) TableName() string       { return "fin_budget" }
