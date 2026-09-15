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

type FinAccountEntry struct {
	ID           int64           `json:"id"`
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
func (FinAccountEntry) TableName() string { return "fin_voucher" }
func (FinBudget) TableName() string       { return "fin_budget" }
