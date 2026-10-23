// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/dto"
	financeService "erp-system/internal/finance/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/db"
	"erp-system/internal/testsupport"
)

func accErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// postVoucher 直接在事务里过一张凭证（模拟销售收款/费用付款等无业务模块的事件）。
func postVoucher(t *testing.T, gdb *gorm.DB, svc *financeService.FinanceService, in financeService.PostingInput) {
	t.Helper()
	if err := db.NewTxManager(gdb).WithTx(testsupport.Ctx(), func(tx *gorm.DB) error {
		return svc.PostTx(testsupport.Ctx(), tx, in)
	}); err != nil {
		t.Fatalf("过账失败: %v", err)
	}
}

// 凭证主表/分录拆分：过账生成主表行 + N 条分录，GetVoucher 带回分录，期间归属正确。
func TestVoucher_MasterEntrySplit(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()

	postVoucher(t, gdb, svc, financeService.PostingInput{
		BizType:     "manual",
		BizNo:       "M-001",
		Description: "销售收款",
		Lines: []financeService.PostingLine{
			{AccountCode: "1002", Debit: testsupport.D("1000")},
			{AccountCode: "6001", Credit: testsupport.D("1000")},
		},
	})

	list, total, err := svc.ListVouchers(ctx, dto.VoucherQuery{Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("凭证列表错误: total=%d len=%d err=%v", total, len(list), err)
	}
	v := list[0]
	if !v.DebitTotal.Equal(testsupport.D("1000")) || !v.CreditTotal.Equal(testsupport.D("1000")) {
		t.Fatalf("凭证借贷合计错误: %+v", v)
	}
	if v.Period == "" {
		t.Fatal("凭证期间不能为空")
	}

	detail, err := svc.GetVoucher(ctx, v.ID)
	if err != nil {
		t.Fatalf("查询凭证详情失败: %v", err)
	}
	if len(detail.Entries) != 2 {
		t.Fatalf("凭证明细应 2 条，实际 %d", len(detail.Entries))
	}
	// 科目名称从科目表自动带出
	for _, e := range detail.Entries {
		if e.AccountName == "" {
			t.Fatalf("分录科目名称应自动带出: %+v", e)
		}
	}
}

// 科目树：种子科目存在、新建子科目联动父级 is_leaf、删除守卫。
func TestAccount_TreeAndGuards(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()

	tree, err := svc.ListAccountTree(ctx)
	if err != nil {
		t.Fatalf("查询科目树失败: %v", err)
	}
	if len(tree) < 10 {
		t.Fatalf("种子科目应存在，实际根节点 %d 个", len(tree))
	}

	// 找到 1002 银行存款，在其下建子科目
	var bankID int64
	for _, a := range tree {
		if a.Code == "1002" {
			bankID = a.ID
		}
	}
	if bankID == 0 {
		t.Fatal("种子科目 1002 不存在")
	}

	sub, err := svc.CreateAccount(ctx, dto.AccountReq{
		Code: "100201", Name: "基本户", ParentID: &bankID, Category: 1, Direction: 1,
	})
	if err != nil {
		t.Fatalf("创建子科目失败: %v", err)
	}
	if sub.Level != 2 {
		t.Fatalf("子科目层级应为 2，实际 %d", sub.Level)
	}

	// 类别不一致拒绝
	if _, err := svc.CreateAccount(ctx, dto.AccountReq{
		Code: "100202", Name: "错误类别", ParentID: &bankID, Category: 2, Direction: 2,
	}); accErrCode(err) != 60007 {
		t.Fatalf("类别不一致应返回 60007，实际 %v", err)
	}

	// 重复编码拒绝
	if _, err := svc.CreateAccount(ctx, dto.AccountReq{
		Code: "100201", Name: "重复",
	}); accErrCode(err) != 60007 {
		t.Fatalf("重复编码应返回 60007，实际 %v", err)
	}

	// 有子级的科目不能删除
	if err := svc.DeleteAccount(ctx, bankID); accErrCode(err) != 60009 {
		t.Fatalf("有子级删除应返回 60009，实际 %v", err)
	}

	// 有分录发生的科目不能删除/改编码
	postVoucher(t, gdb, svc, financeService.PostingInput{
		BizType: "manual", BizNo: "M-002", Description: "子科目收款的",
		Lines: []financeService.PostingLine{
			{AccountCode: "100201", Debit: testsupport.D("10")},
			{AccountCode: "6001", Credit: testsupport.D("10")},
		},
	})
	if err := svc.DeleteAccount(ctx, sub.ID); accErrCode(err) != 60009 {
		t.Fatalf("有分录删除应返回 60009，实际 %v", err)
	}
	if err := svc.UpdateAccount(ctx, sub.ID, dto.AccountReq{Code: "100299", Name: "改名"}); accErrCode(err) != 60009 {
		t.Fatalf("有分录改编码应返回 60009，实际 %v", err)
	}
}

// 期末结转：损益结平到 4103、凭证归属被结转期间、重复结转拒绝、无损益期间拒绝。
func TestClosePeriod(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()
	period := "2026-08"

	// 本期发生：收入 1000（贷 6001）、费用 300（借 6401）、现金对应
	postVoucher(t, gdb, svc, financeService.PostingInput{
		BizType: "manual", BizNo: "S-001", Description: "8月销售收入", Period: period,
		Lines: []financeService.PostingLine{
			{AccountCode: "1002", Debit: testsupport.D("1000")},
			{AccountCode: "6001", Credit: testsupport.D("1000")},
		},
	})
	postVoucher(t, gdb, svc, financeService.PostingInput{
		BizType: "manual", BizNo: "E-001", Description: "8月成本费用", Period: period,
		Lines: []financeService.PostingLine{
			{AccountCode: "6401", Debit: testsupport.D("300")},
			{AccountCode: "1002", Credit: testsupport.D("300")},
		},
	})

	// 结转前利润表
	is, err := svc.GetIncomeStatement(ctx, period)
	if err != nil {
		t.Fatalf("利润表查询失败: %v", err)
	}
	if !is.TotalRevenue.Equal(testsupport.D("1000")) || !is.TotalExpense.Equal(testsupport.D("300")) {
		t.Fatalf("结转前利润表错误: %+v", is)
	}
	if !is.NetProfit.Equal(testsupport.D("700")) {
		t.Fatalf("净利润应为 700，实际 %v", is.NetProfit)
	}

	// 结转
	if err := svc.ClosePeriod(ctx, period); err != nil {
		t.Fatalf("期末结转失败: %v", err)
	}

	// 结转后：收入/费用科目净额为 0，4103 贷方净额 700
	periodTo := "2026-09"
	revNet, err := svcSum(gdb, "60", period, periodTo)
	if err != nil {
		t.Fatal(err)
	}
	if !revNet.IsZero() {
		t.Fatalf("结转后收入科目应结平，实际净额 %v", revNet)
	}
	expNet, err := svcSum(gdb, "64", period, periodTo)
	if err != nil {
		t.Fatal(err)
	}
	if !expNet.IsZero() {
		t.Fatalf("结转后费用科目应结平，实际净额 %v", expNet)
	}
	plNet, err := svcSum(gdb, "4103", period, periodTo)
	if err != nil {
		t.Fatal(err)
	}
	if !plNet.Equal(testsupport.D("-700")) {
		t.Fatalf("4103 应贷方净额 700（net=-700），实际 %v", plNet)
	}

	// 重复结转 → 60010
	if err := svc.ClosePeriod(ctx, period); accErrCode(err) != 60010 {
		t.Fatalf("重复结转应返回 60010，实际 %v", err)
	}

	// P1.5 H3：结转后利润表本期数不被结转分录对冲清零（流量口径排除 period_close）
	is2, err := svc.GetIncomeStatement(ctx, period)
	if err != nil {
		t.Fatalf("结转后利润表查询失败: %v", err)
	}
	if !is2.TotalRevenue.Equal(testsupport.D("1000")) || !is2.TotalExpense.Equal(testsupport.D("300")) {
		t.Fatalf("结转后利润表应仍列示本期发生 1000/300，实际 %+v", is2)
	}
	if !is2.NetProfit.Equal(testsupport.D("700")) {
		t.Fatalf("结转后净利润应仍为 700，实际 %v", is2.NetProfit)
	}
	rep, err := svc.GetFinancialReport(ctx, period)
	if err != nil {
		t.Fatalf("财务简表查询失败: %v", err)
	}
	if !rep.TotalRevenue.Equal(testsupport.D("1000")) || !rep.TotalCost.Equal(testsupport.D("300")) {
		t.Fatalf("结转后财务简表本期数应仍为 1000/300，实际 %+v", rep)
	}
	// 资产端为时点余额口径（含结转分录）：1002 净额 700 = 收入1000 − 费用300
	if !rep.TotalAssets.IsPositive() {
		t.Fatalf("结转后资产余额不应被对冲，实际 %v", rep.TotalAssets)
	}

	// P1.5 S4：已结转期间拒绝普通业务过账 → 60012
	err = db.NewTxManager(gdb).WithTx(ctx, func(tx *gorm.DB) error {
		return svc.PostTx(ctx, tx, financeService.PostingInput{
			BizType: "manual", BizNo: "LATE-001", Description: "迟到的8月凭证", Period: period,
			Lines: []financeService.PostingLine{
				{AccountCode: "1002", Debit: testsupport.D("1")},
				{AccountCode: "6001", Credit: testsupport.D("1")},
			},
		})
	})
	if accErrCode(err) != 60012 {
		t.Fatalf("已结转期间过账应返回 60012，实际 %v", err)
	}

	// 无损益期间 → 60011
	if err := svc.ClosePeriod(ctx, "2026-07"); accErrCode(err) != 60011 {
		t.Fatalf("无损益期间应返回 60011，实际 %v", err)
	}

	// 现金流量：流入 1000、流出 300、净额 700
	cf, err := svc.GetCashFlow(ctx, period)
	if err != nil {
		t.Fatalf("现金流量查询失败: %v", err)
	}
	if !cf.TotalInflow.Equal(testsupport.D("1000")) || !cf.TotalOutflow.Equal(testsupport.D("300")) {
		t.Fatalf("现金流量错误: %+v", cf)
	}
	if !cf.NetCashFlow.Equal(testsupport.D("700")) {
		t.Fatalf("现金净流量应为 700，实际 %v", cf.NetCashFlow)
	}
}

// P1.5 S4：并发期末结转被期间 advisory lock 串行化——恰好一个成功、
// 其余在锁内重检返回 60010，且只落一张结转凭证。
func TestClosePeriod_Concurrent(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()
	period := "2026-05"

	postVoucher(t, gdb, svc, financeService.PostingInput{
		BizType: "manual", BizNo: "S-100", Description: "5月销售收入", Period: period,
		Lines: []financeService.PostingLine{
			{AccountCode: "1002", Debit: testsupport.D("500")},
			{AccountCode: "6001", Credit: testsupport.D("500")},
		},
	})

	const n = 4
	var wg sync.WaitGroup
	codes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- accErrCode(svc.ClosePeriod(ctx, period))
		}()
	}
	wg.Wait()
	close(codes)

	succeeded, dupRejected := 0, 0
	for c := range codes {
		switch c {
		case 0:
			succeeded++
		case 60010:
			dupRejected++
		default:
			t.Fatalf("并发结转出现意外错误码 %d", c)
		}
	}
	if succeeded != 1 || dupRejected != n-1 {
		t.Fatalf("并发结转应恰 1 成功、%d 个 60010，实际 成功=%d 拒绝=%d", n-1, succeeded, dupRejected)
	}

	// 只生成一张 period_close 凭证
	var closes int64
	if err := gdb.Table("fin_voucher").Where("biz_type = 'period_close' AND biz_no = ?", period).
		Count(&closes).Error; err != nil {
		t.Fatal(err)
	}
	if closes != 1 {
		t.Fatalf("结转凭证应只有 1 张，实际 %d", closes)
	}
	// 4103 贷方净额恰好 500（未被重复结转放大）
	plNet, err := svcSum(gdb, "4103", period, "2026-06")
	if err != nil {
		t.Fatal(err)
	}
	if !plNet.Equal(testsupport.D("-500")) {
		t.Fatalf("4103 贷方净额应为 500，实际 %v", plNet)
	}
}

// svcSum 直接按期间汇总科目净额（借−贷），与报表同一取数口径。
func svcSum(gdb *gorm.DB, prefix, periodFrom, periodTo string) (decimal.Decimal, error) {
	var net decimal.Decimal
	err := gdb.Table("fin_voucher_entry").
		Select("COALESCE(SUM(debit_amount),0) - COALESCE(SUM(credit_amount),0)").
		Joins("JOIN fin_voucher v ON v.id = fin_voucher_entry.voucher_id").
		Where("account_code LIKE ?", prefix+"%").
		Where("v.period >= ? AND v.period < ?", periodFrom, periodTo).
		Scan(&net).Error
	return net, err
}
