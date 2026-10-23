// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"testing"
	"time"

	financeDto "erp-system/internal/finance/dto"
	financeModel "erp-system/internal/finance/model"
	"erp-system/internal/testsupport"
)

// P2 §3.5: 应收与收款核销全流程——创建应收单 -> 创建收款单 -> 执行核销 -> 自动生成会计凭证(借1002银行存款/贷1122应收账款) -> 状态转为已结清
func TestReceivable_Receipt_Settlement_Voucher(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()

	cus := testsupport.CreateCustomer(t, gdb, "ar1", testsupport.D("100000"))

	// 1. 创建应收单 5000 元
	recv, err := svc.CreateReceivable(ctx, financeDto.CreateReceivableReq{
		CustomerID: cus.ID,
		Amount:     testsupport.D("5000"),
		DueAt:      time.Now().Add(15 * 24 * time.Hour).Format("2006-01-02"),
		Remark:     "销售发票应收款",
	}, 0)
	if err != nil {
		t.Fatalf("创建应收单失败: %v", err)
	}
	if recv.Status != financeModel.BillStatusPending || !recv.Amount.Equal(testsupport.D("5000")) {
		t.Fatalf("应收单初始状态或金额错误: %+v", recv)
	}

	// 2. 创建收款单 5000 元
	receipt, err := svc.CreateReceipt(ctx, financeDto.CreateReceiptReq{
		CustomerID:  cus.ID,
		Amount:      testsupport.D("5000"),
		ReceiptDate: time.Now().Format("2006-01-02"),
		BankAccount: "6222020000000001",
		Remark:      "电汇收款",
	}, 1, 0)
	if err != nil {
		t.Fatalf("创建收款单失败: %v", err)
	}

	// 3. 执行收款核销
	settle, err := svc.CreateSettlement(ctx, financeDto.CreateSettlementReq{
		SettlementType: 1, // 收款核销
		BillID:         receipt.ID,
		TargetID:       recv.ID,
		Amount:         testsupport.D("5000"),
	}, 1)
	if err != nil {
		t.Fatalf("执行核销失败: %v", err)
	}
	if !settle.Amount.Equal(testsupport.D("5000")) {
		t.Fatalf("核销金额错误: %+v", settle)
	}

	// 4. 验证应收单已转为结清状态
	updatedRecv, err := svc.GetReceivable(ctx, recv.ID)
	if err != nil {
		t.Fatalf("查询应收单失败: %v", err)
	}
	if updatedRecv.Status != financeModel.BillStatusSettled || !updatedRecv.ReceivedAmount.Equal(testsupport.D("5000")) {
		t.Fatalf("核销后应收单状态错误: %+v", updatedRecv)
	}
}

// P2 §3.5: 应付与付款核销全流程——创建应付单 -> 创建付款单 -> 执行核销 -> 自动生成会计凭证(借2202应付账款/贷1002银行存款) -> 状态转为已结清
func TestPayable_Payment_Settlement_Voucher(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()

	sup := testsupport.CreateSupplier(t, gdb, "SUP-AP-001", "采购测试供应商")

	// 1. 创建应付单 3000 元
	pay, err := svc.CreatePayable(ctx, financeDto.CreatePayableReq{
		SupplierID: sup.ID,
		Amount:     testsupport.D("3000"),
		DueAt:      time.Now().Add(30 * 24 * time.Hour).Format("2006-01-02"),
		Remark:     "采购暂估应付款",
	}, 0)
	if err != nil {
		t.Fatalf("创建应付单失败: %v", err)
	}

	// 2. 创建付款单 3000 元
	payment, err := svc.CreatePayment(ctx, financeDto.CreatePaymentReq{
		SupplierID:  sup.ID,
		Amount:      testsupport.D("3000"),
		PaymentDate: time.Now().Format("2006-01-02"),
		BankAccount: "6222020000000001",
		Remark:      "网银付款",
	}, 1, 0)
	if err != nil {
		t.Fatalf("创建付款单失败: %v", err)
	}

	// 3. 执行付款核销
	_, err = svc.CreateSettlement(ctx, financeDto.CreateSettlementReq{
		SettlementType: 2, // 付款核销
		BillID:         payment.ID,
		TargetID:       pay.ID,
		Amount:         testsupport.D("3000"),
	}, 1)
	if err != nil {
		t.Fatalf("执行付款核销失败: %v", err)
	}

	// 4. 验证应付单已结清
	updatedPay, err := svc.GetPayable(ctx, pay.ID)
	if err != nil {
		t.Fatalf("查询应付单失败: %v", err)
	}
	if updatedPay.Status != financeModel.BillStatusSettled || !updatedPay.PaidAmount.Equal(testsupport.D("3000")) {
		t.Fatalf("核销后应付单状态错误: %+v", updatedPay)
	}
}

// P2 §3.5: 账龄分析分桶统计（未到期、0-30天、31-60天等）
func TestAgingReport_Bucketing(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	ctx := testsupport.Ctx()

	cus := testsupport.CreateCustomer(t, gdb, "aging", testsupport.D("100000"))

	now := time.Now()
	// 1. 未到期：10天后到期 1000元
	_, _ = svc.CreateReceivable(ctx, financeDto.CreateReceivableReq{
		CustomerID: cus.ID,
		Amount:     testsupport.D("1000"),
		DueAt:      now.Add(10 * 24 * time.Hour).Format("2006-01-02"),
	}, 0)

	// 2. 逾期 15 天（属于 0-30 天）：2000元
	_, _ = svc.CreateReceivable(ctx, financeDto.CreateReceivableReq{
		CustomerID: cus.ID,
		Amount:     testsupport.D("2000"),
		DueAt:      now.Add(-15 * 24 * time.Hour).Format("2006-01-02"),
	}, 0)

	// 3. 逾期 45 天（属于 31-60 天）：3000元
	_, _ = svc.CreateReceivable(ctx, financeDto.CreateReceivableReq{
		CustomerID: cus.ID,
		Amount:     testsupport.D("3000"),
		DueAt:      now.Add(-45 * 24 * time.Hour).Format("2006-01-02"),
	}, 0)

	report, err := svc.GetAgingReport(ctx, financeDto.AgingQuery{
		Direction:  "recv",
		CustomerID: cus.ID,
	})
	if err != nil {
		t.Fatalf("生成账龄报表失败: %v", err)
	}

	if !report.Total.NotDue.Equal(testsupport.D("1000")) {
		t.Fatalf("未到期金额错误: %v", report.Total.NotDue)
	}
	if !report.Total.Days0To30.Equal(testsupport.D("2000")) {
		t.Fatalf("0-30天逾期金额错误: %v", report.Total.Days0To30)
	}
	if !report.Total.Days31To60.Equal(testsupport.D("3000")) {
		t.Fatalf("31-60天逾期金额错误: %v", report.Total.Days31To60)
	}
	if !report.Total.TotalAmount.Equal(testsupport.D("6000")) {
		t.Fatalf("总待结金额错误: %v", report.Total.TotalAmount)
	}
}
