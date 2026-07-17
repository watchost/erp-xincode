// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type CostCardQuery struct {
	ProductID   int64  `json:"product_id"`
	CostType    string `json:"cost_type"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Page        int    `json:"page" binding:"min=1"`
	PageSize    int    `json:"page_size" binding:"min=1,max=100"`
}

type CostCardVO struct {
	ID              int64   `json:"id"`
	ProductID       int64   `json:"product_id"`
	ProductCode     string  `json:"product_code"`
	ProductName     string  `json:"product_name"`
	CostType        string  `json:"cost_type"`
	CostTypeName    string  `json:"cost_type_name"`
	Amount          float64 `json:"amount"`
	CostDate        string  `json:"cost_date"`
	SourceType      string  `json:"source_type"`
	SourceID        int64   `json:"source_id"`
}

type CostSummaryVO struct {
	ProductID       int64   `json:"product_id"`
	ProductCode     string  `json:"product_code"`
	ProductName     string  `json:"product_name"`
	MaterialCost    float64 `json:"material_cost"`
	LaborCost       float64 `json:"labor_cost"`
	OverheadCost    float64 `json:"overhead_cost"`
	TotalCost       float64 `json:"total_cost"`
}

type AccountEntryQuery struct {
	AccountCode   string `json:"account_code"`
	BizType       string `json:"biz_type"`
	BizNo         string `json:"biz_no"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Page          int    `json:"page" binding:"min=1"`
	PageSize      int    `json:"page_size" binding:"min=1,max=100"`
}

type AccountEntryVO struct {
	ID              int64   `json:"id"`
	EntryNo         string  `json:"entry_no"`
	AccountCode     string  `json:"account_code"`
	AccountName     string  `json:"account_name"`
	DebitAmount     float64 `json:"debit_amount"`
	CreditAmount    float64 `json:"credit_amount"`
	Balance         float64 `json:"balance"`
	BizType         string  `json:"biz_type"`
	BizNo           string  `json:"biz_no"`
	Description     string  `json:"description"`
	CreatedAt       string  `json:"created_at"`
}

type FinancialReportVO struct {
	Period          string  `json:"period"`
	TotalRevenue    float64 `json:"total_revenue"`
	TotalCost       float64 `json:"total_cost"`
	GrossProfit     float64 `json:"gross_profit"`
	GrossMargin     float64 `json:"gross_margin"`
	TotalAssets     float64 `json:"total_assets"`
	TotalLiabilities float64 `json:"total_liabilities"`
	NetAssets       float64 `json:"net_assets"`
}

type BudgetVO struct {
	ID              int64   `json:"id"`
	BudgetNo        string  `json:"budget_no"`
	BudgetType      string  `json:"budget_type"`
	BudgetTypeName  string  `json:"budget_type_name"`
	Amount          float64 `json:"amount"`
	UsedAmount      float64 `json:"used_amount"`
	RemainingAmount float64 `json:"remaining_amount"`
	Year            int     `json:"year"`
	Month           int     `json:"month"`
	Status          int     `json:"status"`
	StatusDesc      string  `json:"status_desc"`
	CreatedAt       string  `json:"created_at"`
}
