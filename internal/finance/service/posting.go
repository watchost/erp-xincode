// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	"erp-system/internal/pkg/decimalx"
	"erp-system/internal/pkg/errors"
)

// 最小会计科目表（Chinese GAAP 常用编码）。
// 财务最小闭环只覆盖采购入库与生产领料两个业务事件，用到的科目在此集中维护；
// 完整的 fin_account 科目主数据在 P1 阶段补齐。
const (
	AccountBankDeposit    = "1002" // 银行存款（资产类，借增贷减）
	AccountRawMaterials   = "1403" // 原材料（资产类，借增贷减）
	AccountFinishedGoods  = "1405" // 库存商品（资产类，借增贷减）
	AccountReceivable     = "1122" // 应收账款（资产类，借增贷减）
	AccountsPayable       = "2202" // 应付账款（负债类，贷增借减）
	AccountProductionCost = "5001" // 生产成本（成本类，借增贷减；余额为在产品）
	AccountMainRevenue    = "6001" // 主营业务收入（损益类，贷增借减）
	AccountMainCost       = "6401" // 主营业务成本（损益类，借增贷减）
	AccountPendingLoss    = "1901" // 待处理财产损溢（资产类/过渡科目，借方盘亏/贷方盘盈）
)

// 业务事件类型，写入 fin_voucher.biz_type，便于按事件溯源与对账。
const (
	BizTypePurchaseInbound    = "purchase_inbound"    // 采购入库
	BizTypeProductionIssue    = "production_issue"    // 生产领料
	BizTypeProductionReceipt  = "production_receipt"  // 生产入库/完工报工
	BizTypeSalesOutbound      = "sales_outbound"      // 销售出库（P2 §3.1）
	BizTypeSalesReturn        = "sales_return"        // 销售退货入库（P2 §3.1）
	BizTypeInventoryCheckGain = "inventory_check_gain" // 盘盈入库（P2 §3.2）
	BizTypeInventoryCheckLoss = "inventory_check_loss" // 盘亏出库（P2 §3.2）
	BizTypeReceiptSettlement  = "receipt_settlement"  // 收款核销（P2 §3.5）
	BizTypePaymentSettlement  = "payment_settlement"  // 付款核销（P2 §3.5）
)

// 成本卡来源类型，写入 fin_cost.source_type。
const (
	CostSourcePurchaseInbound = "purchase_inbound"
)

// 成本卡类型，与 fin_cost.cost_type 及报表汇总一致。
const (
	CostTypeMaterial = "material" // 材料成本
	CostTypeLabor    = "labor"    // 人工成本
	CostTypeOverhead = "overhead" // 制造费用
)

// 预算类型，与 fin_budget.budget_type 一致。
const (
	BudgetTypePurchase   = "purchase"   // 采购预算
	BudgetTypeProduction = "production" // 生产预算
)

// PostingLine 是一条会计分录行：借贷必须恰有一方 > 0。
type PostingLine struct {
	AccountCode string
	AccountName string
	Debit       decimal.Decimal
	Credit      decimal.Decimal
}

// CostCardInput 描述一张成本卡（按产品/物料归集的成本）。
type CostCardInput struct {
	ProductID  int64
	CostType   string
	Amount     decimal.Decimal
	SourceType string
	SourceID   int64
}

// PostingInput 描述一张需要在业务事务内过账的记账凭证。
type PostingInput struct {
	BizType     string          // 业务事件类型，见 BizType* 常量
	BizNo       string          // 业务单号（入库单号/出库单号），用于溯源
	Description string          // 摘要
	Period      string          // 会计期间 YYYY-MM；空 = 过账当天所属期间（期末结转时传入被结转期间）
	CreatedBy   int64           // 过账人 user_id（可为 0）
	Lines       []PostingLine   // 借贷分录行，必须借贷平衡
	CostCards   []CostCardInput // 同事务写入的成本卡（可为空）
}

// Poster 由业务服务（采购/生产）依赖，在各自的事务内调用以完成财务过账。
// 抽成接口避免业务服务直接依赖 FinanceService 的具体实现。
type Poster interface {
	PostTx(ctx context.Context, tx *gorm.DB, in PostingInput) error
}

// PostTx 在调用方提供的事务内过账：校验借贷平衡 → 生成凭证号 → 写双分录 → 写成本卡。
// 任何一步失败都返回错误，由调用方随业务事务一并回滚，保证业务单据与财务账严格一致。
func (s *FinanceService) PostTx(ctx context.Context, tx *gorm.DB, in PostingInput) error {
	if len(in.Lines) < 2 {
		return errors.New(60001, 500, "记账凭证至少需要两条分录")
	}

	// 落库前唯一的金额归一关口：分录金额按金额列标度 NUMERIC(18,2) 归整，
	// 后续校验与落库都使用归整后的值——避免按未归整值校验平衡、归整后反而失衡。
	lines := make([]PostingLine, len(in.Lines))
	var debitSum, creditSum decimal.Decimal
	for i, line := range in.Lines {
		line.Debit = decimalx.Money(line.Debit)
		line.Credit = decimalx.Money(line.Credit)
		lines[i] = line
		if line.Debit.IsNegative() || line.Credit.IsNegative() {
			return errors.New(60002, 500, "分录金额不能为负")
		}
		if line.Debit.IsPositive() == line.Credit.IsPositive() {
			return errors.New(60003, 500, "每条分录必须且只能有一方金额（借或贷）")
		}
		debitSum = debitSum.Add(line.Debit)
		creditSum = creditSum.Add(line.Credit)
	}
	// 精确平衡：借/贷合计必须分毫不差（不再使用浮点容差）。
	if !debitSum.Equal(creditSum) {
		return errors.New(60004, 500, "记账凭证借贷不平衡，已拒绝过账")
	}
	if !debitSum.IsPositive() {
		return errors.New(60005, 500, "记账凭证金额必须大于零")
	}

	// 凭证主表：一张凭证一个凭证号，期间归属决定报表取数窗口；
	// 分录行用「凭证号-行号」作为唯一 entry_no 并回挂 voucher_id。
	voucherNo := s.generateVoucherNo(ctx)
	now := time.Now()
	period := in.Period
	if period == "" {
		period = now.Format("2006-01")
	}

	// P1.5 S4：期间级 advisory lock 与 ClosePeriod 互斥——任何向某期间过账
	// 都必须等该期间的结转事务结束，堵死「结转取数后新凭证挤进被结转期间」
	// 的漏结窗口；同一事务内可重入（ClosePeriod 先取锁再调本方法）。
	if err := lockPeriod(tx, period); err != nil {
		return err
	}
	// 已结转期间拒绝普通业务过账（期末结转凭证自身豁免），
	// 保证「结转取数 = 该期间最终损益」不被事后追加打破。
	if in.BizType != BizTypePeriodClose {
		if closed, err := s.voucherRepo.ExistsPeriodCloseTx(tx, period); err != nil {
			return err
		} else if closed {
			return errors.New(60012, 409, "该会计期间已结转，禁止过账")
		}
	}
	voucher := model.FinVoucher{
		VoucherNo:   voucherNo,
		Period:      period,
		Summary:     in.Description,
		BizType:     in.BizType,
		BizNo:       in.BizNo,
		DebitTotal:  debitSum,
		CreditTotal: creditSum,
		PostedAt:    now,
		CreatedBy:   in.CreatedBy,
		CreatedAt:   now,
	}
	if err := s.voucherRepo.Create(tx, &voucher); err != nil {
		return err
	}

	for i, line := range lines {
		accountName := line.AccountName
		if accountName == "" {
			// 科目名称缺省时从科目表带出，保证分录可读性。
			if acc, err := s.accountRepo.FindByCode(line.AccountCode); err == nil && acc != nil {
				accountName = acc.Name
			}
		}
		entry := model.FinAccountEntry{
			VoucherID:    voucher.ID,
			EntryNo:      fmt.Sprintf("%s-%02d", voucherNo, i+1),
			AccountCode:  line.AccountCode,
			AccountName:  accountName,
			DebitAmount:  line.Debit,
			CreditAmount: line.Credit,
			Balance:      line.Debit.Sub(line.Credit),
			BizType:      in.BizType,
			BizNo:        in.BizNo,
			Description:  in.Description,
			CreatedAt:    now,
		}
		if err := s.accountEntryRepo.Create(tx, &entry); err != nil {
			return err
		}
	}

	for _, card := range in.CostCards {
		cc := model.FinCostCard{
			ProductID:  card.ProductID,
			CostType:   card.CostType,
			Amount:     decimalx.Cost(card.Amount), // fin_cost.amount NUMERIC(18,4)
			CostDate:   now,
			SourceType: card.SourceType,
			SourceID:   card.SourceID,
			CreatedAt:  now,
		}
		if err := s.costCardRepo.Create(tx, &cc); err != nil {
			return err
		}
	}

	return nil
}

func (s *FinanceService) generateVoucherNo(ctx context.Context) string {
	if s.numbers == nil {
		// 退化方案：main 必须注入 generator；到达这里说明装配有误。
		return "FV" + time.Now().Format("20060102150405")
	}
	return s.numbers.Next(ctx, "FV")
}
