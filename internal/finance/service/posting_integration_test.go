// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	"erp-system/internal/finance/service"
	"erp-system/internal/pkg/db"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

func entryCount(t *testing.T, gdb *gorm.DB) int64 {
	t.Helper()
	var n int64
	if err := gdb.Model(&model.FinAccountEntry{}).Count(&n).Error; err != nil {
		t.Fatalf("查询分录失败: %v", err)
	}
	return n
}

// 借贷不平必须拒绝且不落任何分录。
func TestPostTx_RejectsUnbalanced(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)

	err := txMgr.WithTx(context.Background(), func(tx *gorm.DB) error {
		return svc.PostTx(context.Background(), tx, service.PostingInput{
			BizType: service.BizTypePurchaseInbound,
			BizNo:   "IB-T-UNBAL",
			Lines: []service.PostingLine{
				{AccountCode: service.AccountRawMaterials, Debit: testsupport.D("100")},
				{AccountCode: service.AccountsPayable, Credit: testsupport.D("99.99")},
			},
		})
	})
	if err == nil {
		t.Fatal("借贷不平必须报错")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != 60004 {
		t.Fatalf("应为借贷不平衡错误 60004，实际 %v", err)
	}
	if entryCount(t, gdb) != 0 {
		t.Fatal("拒绝过账时不得写入分录")
	}
}

// 凭证规则：至少 2 条分录、金额非负、一条分录不能借贷双方都有金额。
func TestPostTx_RejectsInvalidLines(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()

	cases := []struct {
		name  string
		lines []service.PostingLine
		code  int
	}{
		{"单分录", []service.PostingLine{{AccountCode: "1403", Debit: testsupport.D("100")}}, 60001},
		{"负金额", []service.PostingLine{{AccountCode: "1403", Debit: testsupport.D("-1")}, {AccountCode: "2202", Credit: testsupport.D("1")}}, 60002},
		{"借贷同列", []service.PostingLine{{AccountCode: "1403", Debit: testsupport.D("100"), Credit: testsupport.D("100")}, {AccountCode: "2202", Credit: testsupport.D("100")}}, 60003},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := txMgr.WithTx(ctx, func(tx *gorm.DB) error {
				return svc.PostTx(ctx, tx, service.PostingInput{BizType: "x", BizNo: "IB-X", Lines: tc.lines})
			})
			var ae *apperrors.AppError
			if !errors.As(err, &ae) || ae.Code != tc.code {
				t.Fatalf("期望错误码 %d，实际 %v", tc.code, err)
			}
		})
	}
	if entryCount(t, gdb) != 0 {
		t.Fatal("非法凭证全部不得落库")
	}
}

// 平衡凭证正常落双分录，且与调用方事务同生共死。
func TestPostTx_BalancedAndRollback(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()

	post := func(bizNo string, failAfter bool) error {
		return txMgr.WithTx(ctx, func(tx *gorm.DB) error {
			if err := svc.PostTx(ctx, tx, service.PostingInput{
				BizType:     service.BizTypePurchaseInbound,
				BizNo:       bizNo,
				Description: "测试过账",
				Lines: []service.PostingLine{
					{AccountCode: service.AccountRawMaterials, AccountName: "原材料", Debit: testsupport.D("250.5")},
					{AccountCode: service.AccountsPayable, AccountName: "应付账款", Credit: testsupport.D("250.5")},
				},
			}); err != nil {
				return err
			}
			if failAfter {
				return errors.New("业务单据写入失败（故障注入）")
			}
			return nil
		})
	}

	if err := post("IB-T-OK", false); err != nil {
		t.Fatalf("正常过账失败: %v", err)
	}
	if entryCount(t, gdb) != 2 {
		t.Fatal("平衡凭证应写入 2 条分录")
	}
	var debit, credit decimal.Decimal
	if err := gdb.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(debit_amount),0)").Scan(&debit).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Model(&model.FinAccountEntry{}).
		Select("COALESCE(SUM(credit_amount),0)").Scan(&credit).Error; err != nil {
		t.Fatal(err)
	}
	if !debit.Equal(testsupport.D("250.5")) || !credit.Equal(testsupport.D("250.5")) {
		t.Fatalf("借贷合计应各为 250.5，实际 借=%v 贷=%v", debit, credit)
	}

	// P0 验收：业务后续步骤失败时，已写分录必须随外层事务整体回滚。
	if err := post("IB-T-FAIL", true); err == nil {
		t.Fatal("故障注入应当导致事务失败")
	}
	if entryCount(t, gdb) != 2 {
		t.Fatal("调用方事务回滚后分录必须一并回滚")
	}
}

// 采购过账后财务简表：资产=负债、净资产为 0（业务未产生损益时报表必须平衡）。
func TestFinancialReport_BalanceSheetBalanced(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()

	err := txMgr.WithTx(ctx, func(tx *gorm.DB) error {
		return svc.PostTx(ctx, tx, service.PostingInput{
			BizType: service.BizTypePurchaseInbound,
			BizNo:   "IB-T-RPT",
			Lines: []service.PostingLine{
				{AccountCode: service.AccountRawMaterials, AccountName: "原材料", Debit: testsupport.D("500")},
				{AccountCode: service.AccountsPayable, AccountName: "应付账款", Credit: testsupport.D("500")},
			},
		})
	})
	if err != nil {
		t.Fatalf("过账失败: %v", err)
	}

	rpt, err := svc.GetFinancialReport(ctx, "")
	if err != nil {
		t.Fatalf("生成报表失败: %v", err)
	}
	if !rpt.TotalAssets.Equal(testsupport.D("500")) || !rpt.TotalLiabilities.Equal(testsupport.D("500")) || !rpt.NetAssets.IsZero() {
		t.Fatalf("资产负债表不平衡: assets=%v liab=%v net=%v",
			rpt.TotalAssets, rpt.TotalLiabilities, rpt.NetAssets)
	}
	if !rpt.TotalRevenue.IsZero() || !rpt.TotalCost.IsZero() {
		t.Fatalf("采购入库不产生损益，收入/成本应为 0： rev=%v cost=%v", rpt.TotalRevenue, rpt.TotalCost)
	}
	// 无收入期间不允许触发除零，毛利率必须为精确 0。
	if !rpt.GrossMargin.IsZero() {
		t.Fatalf("无收入时毛利率应为 0，实际 %v", rpt.GrossMargin)
	}
}

// fin_cost.amount 为 NUMERIC(18,4)：4 位以内原值回读，第 5 位 half-away-from-zero 归整。
func TestPostTx_CostCardFourScale(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()

	err := txMgr.WithTx(ctx, func(tx *gorm.DB) error {
		return svc.PostTx(ctx, tx, service.PostingInput{
			BizType: service.BizTypePurchaseInbound,
			BizNo:   "IB-T-COST",
			Lines: []service.PostingLine{
				{AccountCode: service.AccountRawMaterials, AccountName: "原材料", Debit: testsupport.D("12.35")},
				{AccountCode: service.AccountsPayable, AccountName: "应付账款", Credit: testsupport.D("12.35")},
			},
			CostCards: []service.CostCardInput{{
				ProductID:  1,
				CostType:   service.CostTypeMaterial,
				Amount:     testsupport.D("12.34565"), // 第 5 位四舍五入 → 12.3457
				SourceType: service.CostSourcePurchaseInbound,
			}},
		})
	})
	if err != nil {
		t.Fatalf("过账失败: %v", err)
	}
	var card model.FinCostCard
	if err := gdb.Last(&card).Error; err != nil {
		t.Fatalf("查询成本卡失败: %v", err)
	}
	if !card.Amount.Equal(testsupport.D("12.3457")) {
		t.Fatalf("成本卡金额应按 4 位标度归整为 12.3457，实际 %v", card.Amount)
	}
}
