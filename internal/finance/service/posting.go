// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	"erp-system/internal/pkg/errors"
)

// 最小会计科目表（Chinese GAAP 常用编码）。
// 财务最小闭环只覆盖采购入库与生产领料两个业务事件，用到的科目在此集中维护；
// 完整的 fin_account 科目主数据在 P1 阶段补齐。
const (
	AccountRawMaterials   = "1403" // 原材料（资产类，借增贷减）
	AccountFinishedGoods  = "1405" // 库存商品（资产类，借增贷减）
	AccountsPayable       = "2202" // 应付账款（负债类，贷增借减）
	AccountProductionCost = "5001" // 生产成本（成本类，借增贷减；余额为在产品）
)

// 业务事件类型，写入 fin_voucher.biz_type，便于按事件溯源与对账。
const (
	BizTypePurchaseInbound = "purchase_inbound" // 采购入库
	BizTypeProductionIssue = "production_issue" // 生产领料
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
	Debit       float64
	Credit      float64
}

// CostCardInput 描述一张成本卡（按产品/物料归集的成本）。
type CostCardInput struct {
	ProductID  int64
	CostType   string
	Amount     float64
	SourceType string
	SourceID   int64
}

// PostingInput 描述一张需要在业务事务内过账的记账凭证。
type PostingInput struct {
	BizType     string         // 业务事件类型，见 BizType* 常量
	BizNo       string         // 业务单号（入库单号/出库单号），用于溯源
	Description string         // 摘要
	Lines       []PostingLine  // 借贷分录行，必须借贷平衡
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

	var debitSum, creditSum float64
	for _, line := range in.Lines {
		if line.Debit < 0 || line.Credit < 0 {
			return errors.New(60002, 500, "分录金额不能为负")
		}
		if (line.Debit > 0) == (line.Credit > 0) {
			return errors.New(60003, 500, "每条分录必须且只能有一方金额（借或贷）")
		}
		debitSum += line.Debit
		creditSum += line.Credit
	}
	// 浮点容差比较：借贷差额超过 1 分即视为不平衡。
	if math.Abs(debitSum-creditSum) > 0.01 {
		return errors.New(60004, 500, "记账凭证借贷不平衡，已拒绝过账")
	}
	if debitSum <= 0 {
		return errors.New(60005, 500, "记账凭证金额必须大于零")
	}

	// 一张凭证一个凭证号，每条分录行用「凭证号-行号」作为唯一 entry_no。
	voucherNo := s.generateVoucherNo(ctx)
	now := time.Now()
	for i, line := range in.Lines {
		entry := model.FinAccountEntry{
			EntryNo:      fmt.Sprintf("%s-%02d", voucherNo, i+1),
			AccountCode:  line.AccountCode,
			AccountName:  line.AccountName,
			DebitAmount:  line.Debit,
			CreditAmount: line.Credit,
			Balance:      line.Debit - line.Credit,
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
			Amount:     card.Amount,
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
