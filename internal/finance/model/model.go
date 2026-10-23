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

// 应收应付与结算状态常量
const (
	BillStatusPending   int16 = 1 // 待结算
	BillStatusPartial   int16 = 2 // 部分结算
	BillStatusSettled   int16 = 3 // 已结清
	BillStatusCancelled int16 = 5 // 已取消
)

type FinReceivable struct {
	ID             int64           `json:"id"`
	ReceivableNo   string          `json:"receivable_no"`
	CustomerID     int64           `json:"customer_id"`
	SourceType     int16           `json:"source_type"`
	SourceNo       string          `json:"source_no"`
	Amount         decimal.Decimal `json:"amount"`
	ReceivedAmount decimal.Decimal `json:"received_amount"`
	DueAt          time.Time       `json:"due_at"`
	Period         string          `json:"period"`
	Status         int16           `json:"status"`
	Remark         string          `json:"remark"`
	TenantID       int64           `json:"tenant_id"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	DeletedAt      *time.Time      `json:"-" gorm:"index"`
}

type FinPayable struct {
	ID          int64           `json:"id"`
	PayableNo   string          `json:"payable_no"`
	SupplierID  int64           `json:"supplier_id"`
	SourceType  int16           `json:"source_type"`
	SourceNo    string          `json:"source_no"`
	Amount      decimal.Decimal `json:"amount"`
	PaidAmount  decimal.Decimal `json:"paid_amount"`
	DueAt       time.Time       `json:"due_at"`
	Period      string          `json:"period"`
	Status      int16           `json:"status"`
	Remark      string          `json:"remark"`
	TenantID    int64           `json:"tenant_id"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"-" gorm:"index"`
}

type FinReceipt struct {
	ID            int64           `json:"id"`
	ReceiptNo     string          `json:"receipt_no"`
	CustomerID    int64           `json:"customer_id"`
	Amount        decimal.Decimal `json:"amount"`
	ReceiptDate   time.Time       `json:"receipt_date"`
	PaymentMethod int16           `json:"payment_method"`
	BankAccount   string          `json:"bank_account"`
	Remark        string          `json:"remark"`
	CreatedBy     int64           `json:"created_by"`
	TenantID      int64           `json:"tenant_id"`
	CreatedAt     time.Time       `json:"created_at"`
}

type FinPayment struct {
	ID            int64           `json:"id"`
	PaymentNo     string          `json:"payment_no"`
	SupplierID    int64           `json:"supplier_id"`
	Amount        decimal.Decimal `json:"amount"`
	PaymentDate   time.Time       `json:"payment_date"`
	PaymentMethod int16           `json:"payment_method"`
	BankAccount   string          `json:"bank_account"`
	Remark        string          `json:"remark"`
	CreatedBy     int64           `json:"created_by"`
	TenantID      int64           `json:"tenant_id"`
	CreatedAt     time.Time       `json:"created_at"`
}

type FinSettlement struct {
	ID             int64           `json:"id"`
	SettlementNo   string          `json:"settlement_no"`
	SettlementType int16           `json:"settlement_type"` // 1=收款核销 2=付款核销
	BillID         int64           `json:"bill_id"`         // receipt_id 或 payment_id
	TargetID       int64           `json:"target_id"`       // receivable_id 或 payable_id
	Amount         decimal.Decimal `json:"amount"`
	CreatedBy      int64           `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
}

func (FinReceivable) TableName() string { return "fin_receivable" }
func (FinPayable) TableName() string    { return "fin_payable" }
func (FinReceipt) TableName() string    { return "fin_receipt" }
func (FinPayment) TableName() string    { return "fin_payment" }
func (FinSettlement) TableName() string { return "fin_settlement" }

