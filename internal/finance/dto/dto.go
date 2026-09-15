// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

import "github.com/shopspring/decimal"

type CostCardQuery struct {
	ProductID int64  `form:"product_id" json:"product_id"`
	CostType  string `form:"cost_type" json:"cost_type"`
	StartDate string `form:"start_date" json:"start_date"`
	EndDate   string `form:"end_date" json:"end_date"`
	Page      int    `form:"page" json:"page" binding:"min=1"`
	PageSize  int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type CostCardVO struct {
	ID           int64           `json:"id"`
	ProductID    int64           `json:"product_id"`
	ProductCode  string          `json:"product_code"`
	ProductName  string          `json:"product_name"`
	CostType     string          `json:"cost_type"`
	CostTypeName string          `json:"cost_type_name"`
	Amount       decimal.Decimal `json:"amount"`
	CostDate     string          `json:"cost_date"`
	SourceType   string          `json:"source_type"`
	SourceID     int64           `json:"source_id"`
}

type CostSummaryVO struct {
	ProductID    int64           `json:"product_id"`
	ProductCode  string          `json:"product_code"`
	ProductName  string          `json:"product_name"`
	MaterialCost decimal.Decimal `json:"material_cost"`
	LaborCost    decimal.Decimal `json:"labor_cost"`
	OverheadCost decimal.Decimal `json:"overhead_cost"`
	TotalCost    decimal.Decimal `json:"total_cost"`
}

type AccountEntryQuery struct {
	AccountCode string `form:"account_code" json:"account_code"`
	BizType     string `form:"biz_type" json:"biz_type"`
	BizNo       string `form:"biz_no" json:"biz_no"`
	StartDate   string `form:"start_date" json:"start_date"`
	EndDate     string `form:"end_date" json:"end_date"`
	Page        int    `form:"page" json:"page" binding:"min=1"`
	PageSize    int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type AccountEntryVO struct {
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
	CreatedAt    string          `json:"created_at"`
}

type FinancialReportVO struct {
	Period           string          `json:"period"`
	TotalRevenue     decimal.Decimal `json:"total_revenue"`
	TotalCost        decimal.Decimal `json:"total_cost"`
	GrossProfit      decimal.Decimal `json:"gross_profit"`
	GrossMargin      decimal.Decimal `json:"gross_margin"`
	TotalAssets      decimal.Decimal `json:"total_assets"`
	TotalLiabilities decimal.Decimal `json:"total_liabilities"`
	NetAssets        decimal.Decimal `json:"net_assets"`
}

type BudgetVO struct {
	ID              int64           `json:"id"`
	BudgetNo        string          `json:"budget_no"`
	BudgetType      string          `json:"budget_type"`
	BudgetTypeName  string          `json:"budget_type_name"`
	Amount          decimal.Decimal `json:"amount"`
	UsedAmount      decimal.Decimal `json:"used_amount"`
	RemainingAmount decimal.Decimal `json:"remaining_amount"`
	Year            int             `json:"year"`
	Month           int             `json:"month"`
	Status          int             `json:"status"`
	StatusDesc      string          `json:"status_desc"`
	CreatedAt       string          `json:"created_at"`
}

// CreateBudgetReq 创建预算。budget_type 限定在四类；同一类型+年月唯一。
type CreateBudgetReq struct {
	BudgetType string          `json:"budget_type" binding:"required,oneof=purchase production marketing admin"`
	Amount     decimal.Decimal `json:"amount" binding:"decgt=0"`
	Year       int             `json:"year" binding:"required,min=2000,max=2100"`
	Month      int             `json:"month" binding:"required,min=1,max=12"`
}
