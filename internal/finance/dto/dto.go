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

// ---- 会计科目 ----

type AccountVO struct {
	ID        int64       `json:"id"`
	Code      string      `json:"code"`
	Name      string      `json:"name"`
	ParentID  *int64      `json:"parent_id"`
	Level     int         `json:"level"`
	Category  int         `json:"category"`
	Direction int         `json:"direction"`
	IsLeaf    bool        `json:"is_leaf"`
	Children  []AccountVO `json:"children,omitempty"`
}

// AccountReq 新建/编辑科目。category：1资产 2负债 3权益 4成本 5收入 6费用；direction：1借 2贷。
type AccountReq struct {
	Code      string `json:"code" binding:"required,max=32"`
	Name      string `json:"name" binding:"required,max=64"`
	ParentID  *int64 `json:"parent_id"`
	Category  int    `json:"category" binding:"min=1,max=6"`
	Direction int    `json:"direction" binding:"min=1,max=2"`
}

// ---- 凭证 ----

type VoucherQuery struct {
	Period   string `form:"period" json:"period"`
	BizType  string `form:"biz_type" json:"biz_type"`
	Page     int    `form:"page" json:"page" binding:"min=1"`
	PageSize int    `form:"page_size" json:"page_size" binding:"min=1,max=100"`
}

type VoucherVO struct {
	ID          int64            `json:"id"`
	VoucherNo   string           `json:"voucher_no"`
	Period      string           `json:"period"`
	Summary     string           `json:"summary"`
	BizType     string           `json:"biz_type"`
	BizNo       string           `json:"biz_no"`
	DebitTotal  decimal.Decimal  `json:"debit_total"`
	CreditTotal decimal.Decimal  `json:"credit_total"`
	PostedAt    string           `json:"posted_at"`
	CreatedBy   int64            `json:"created_by"`
	Entries     []AccountEntryVO `json:"entries,omitempty"`
}

// PeriodCloseReq 期末结转请求：把该期间损益类科目（5收入/6费用）结转到 4103 本年利润。
type PeriodCloseReq struct {
	Period string `json:"period" binding:"required,len=7"`
}

// ---- 利润表 / 现金流量 ----

type IncomeStatementLineVO struct {
	AccountCode string          `json:"account_code"`
	AccountName string          `json:"account_name"`
	Amount      decimal.Decimal `json:"amount"` // 收入为贷方净额（正=收入），费用为借方净额（正=费用）
}

type IncomeStatementVO struct {
	Period      string                   `json:"period"`
	Revenue     []IncomeStatementLineVO  `json:"revenue"`
	Expense     []IncomeStatementLineVO  `json:"expense"`
	TotalRevenue decimal.Decimal         `json:"total_revenue"`
	TotalExpense decimal.Decimal         `json:"total_expense"`
	NetProfit    decimal.Decimal          `json:"net_profit"`
}

type CashFlowLineVO struct {
	BizType string          `json:"biz_type"`
	Inflow  decimal.Decimal `json:"inflow"`
	Outflow decimal.Decimal `json:"outflow"`
	Net     decimal.Decimal `json:"net"`
}

type CashFlowVO struct {
	Period      string           `json:"period"`
	Lines       []CashFlowLineVO `json:"lines"`
	TotalInflow  decimal.Decimal `json:"total_inflow"`
	TotalOutflow decimal.Decimal `json:"total_outflow"`
	NetCashFlow  decimal.Decimal `json:"net_cash_flow"`
}

// ---- P2 §3.5 应收应付与对账 DTO ----

type CreateReceivableReq struct {
	CustomerID int64           `json:"customer_id" binding:"required"`
	SourceType int16           `json:"source_type"`
	SourceNo   string          `json:"source_no"`
	Amount     decimal.Decimal `json:"amount" binding:"decgt=0"`
	DueAt      string          `json:"due_at" binding:"required"`
	Period     string          `json:"period"`
	Remark     string          `json:"remark"`
}

type ReceivableQuery struct {
	CustomerID   int64  `form:"customer_id"`
	ReceivableNo string `form:"receivable_no"`
	Status       int16  `form:"status"`
	Period       string `form:"period"`
	Page         int    `form:"page" binding:"required,min=1"`
	PageSize     int    `form:"page_size" binding:"required,min=1,max=100"`
}

type CreatePayableReq struct {
	SupplierID int64           `json:"supplier_id" binding:"required"`
	SourceType int16           `json:"source_type"`
	SourceNo   string          `json:"source_no"`
	Amount     decimal.Decimal `json:"amount" binding:"decgt=0"`
	DueAt      string          `json:"due_at" binding:"required"`
	Period     string          `json:"period"`
	Remark     string          `json:"remark"`
}

type PayableQuery struct {
	SupplierID int64  `form:"supplier_id"`
	PayableNo  string `form:"payable_no"`
	Status     int16  `form:"status"`
	Period     string `form:"period"`
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
}

type CreateReceiptReq struct {
	CustomerID    int64           `json:"customer_id" binding:"required"`
	Amount        decimal.Decimal `json:"amount" binding:"decgt=0"`
	ReceiptDate   string          `json:"receipt_date" binding:"required"`
	PaymentMethod int16           `json:"payment_method"`
	BankAccount   string          `json:"bank_account"`
	Remark        string          `json:"remark"`
}

type ReceiptQuery struct {
	CustomerID int64  `form:"customer_id"`
	ReceiptNo  string `form:"receipt_no"`
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
}

type CreatePaymentReq struct {
	SupplierID    int64           `json:"supplier_id" binding:"required"`
	Amount        decimal.Decimal `json:"amount" binding:"decgt=0"`
	PaymentDate   string          `json:"payment_date" binding:"required"`
	PaymentMethod int16           `json:"payment_method"`
	BankAccount   string          `json:"bank_account"`
	Remark        string          `json:"remark"`
}

type PaymentQuery struct {
	SupplierID int64  `form:"supplier_id"`
	PaymentNo  string `form:"payment_no"`
	Page       int    `form:"page" binding:"required,min=1"`
	PageSize   int    `form:"page_size" binding:"required,min=1,max=100"`
}

type CreateSettlementReq struct {
	SettlementType int16           `json:"settlement_type" binding:"required,oneof=1 2"` // 1=收款核销 2=付款核销
	BillID         int64           `json:"bill_id" binding:"required"`
	TargetID       int64           `json:"target_id" binding:"required"`
	Amount         decimal.Decimal `json:"amount" binding:"decgt=0"`
}

type AgingQuery struct {
	Direction  string `form:"direction" binding:"required,oneof=recv pay"` // recv=应收账龄 pay=应付账龄
	CustomerID int64  `form:"customer_id"`
	SupplierID int64  `form:"supplier_id"`
	AsOfDate   string `form:"as_of_date"` // 基准日期，默认今天
}

type AgingBucketVO struct {
	NotDue      decimal.Decimal `json:"not_due"`      // 未到期
	Days0To30   decimal.Decimal `json:"days_0_30"`    // 0-30天
	Days31To60  decimal.Decimal `json:"days_31_60"`   // 31-60天
	Days61To90  decimal.Decimal `json:"days_61_90"`   // 61-90天
	Days91To180 decimal.Decimal `json:"days_91_180"`  // 91-180天
	Days180Plus decimal.Decimal `json:"days_180_plus"` // 180天以上
	TotalAmount decimal.Decimal `json:"total_amount"` // 合计待结清金额
}

type AgingPartnerItemVO struct {
	PartnerID   int64           `json:"partner_id"`
	PartnerCode string          `json:"partner_code"`
	PartnerName string          `json:"partner_name"`
	Buckets     AgingBucketVO   `json:"buckets"`
}

type AgingReportVO struct {
	Direction string               `json:"direction"`
	AsOfDate  string               `json:"as_of_date"`
	Total     AgingBucketVO        `json:"total"`
	Items     []AgingPartnerItemVO `json:"items"`
}

