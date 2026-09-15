// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"erp-system/internal/finance/model"
	"erp-system/internal/finance/service"
	"erp-system/internal/pkg/db"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

func createBudget(t *testing.T, gdb *gorm.DB, amount decimal.Decimal) model.FinBudget {
	t.Helper()
	now := time.Now()
	b := model.FinBudget{
		BudgetNo:   fmt.Sprintf("BG-TEST-%d", now.UnixNano()),
		BudgetType: service.BudgetTypePurchase,
		Amount:     amount,
		Year:       now.Year(),
		Month:      int(now.Month()),
		Status:     1,
	}
	if err := gdb.Create(&b).Error; err != nil {
		t.Fatalf("创建预算失败: %v", err)
	}
	return b
}

func budgetUsed(t *testing.T, gdb *gorm.DB, id int64) decimal.Decimal {
	t.Helper()
	var b model.FinBudget
	if err := gdb.First(&b, id).Error; err != nil {
		t.Fatalf("查询预算失败: %v", err)
	}
	return b.UsedAmount
}

// 条件 UPDATE 保证并发/连续占用不会超额：余额不足拒绝且占用额不变化。
func TestReserveBudget_AtomicGuard(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()
	b := createBudget(t, gdb, testsupport.D("1000"))

	run := func(fn func(tx *gorm.DB) error) error {
		return txMgr.WithTx(ctx, fn)
	}

	// 占用 600 → 成功，used=600
	if err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("600"))
	}); err != nil {
		t.Fatalf("占用 600 应成功: %v", err)
	}
	if got := budgetUsed(t, gdb, b.ID); !got.Equal(testsupport.D("600")) {
		t.Fatalf("used 应为 600，实际 %v", got)
	}

	// 再占用 500 → 余额不足，随事务回滚，used 保持 600
	err := run(func(tx *gorm.DB) error {
		if e := svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("500")); e != nil {
			return e
		}
		t.Fatal("占用 500 必须失败")
		return nil
	})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != 60010 {
		t.Fatalf("应为预算不足错误 60010，实际 %v", err)
	}
	if got := budgetUsed(t, gdb, b.ID); !got.Equal(testsupport.D("600")) {
		t.Fatalf("拒绝占用后 used 应保持 600，实际 %v", got)
	}

	// 占用 400 恰好用完
	if err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("400"))
	}); err != nil {
		t.Fatalf("占用至 1000 应成功: %v", err)
	}
	if got := budgetUsed(t, gdb, b.ID); !got.Equal(testsupport.D("1000")) {
		t.Fatalf("used 应为 1000，实际 %v", got)
	}

	// 释放 200 → used=800；GREATEST 兜底：再释放 9999 不会变成负数
	if err := run(func(tx *gorm.DB) error {
		return svc.ReleaseBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("200"))
	}); err != nil {
		t.Fatalf("释放失败: %v", err)
	}
	if err := run(func(tx *gorm.DB) error {
		return svc.ReleaseBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("9999"))
	}); err != nil {
		t.Fatalf("超额释放失败: %v", err)
	}
	if got := budgetUsed(t, gdb, b.ID); !got.IsZero() {
		t.Fatalf("used 兜底应为 0，实际 %v", got)
	}
}

// 本期未设预算时不强制控制（放行），避免未上线预算模块时阻断业务。
func TestReserveBudget_NoBudgetPasses(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()

	err := txMgr.WithTx(ctx, func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("999999"))
	})
	if err != nil {
		t.Fatalf("未设预算时应放行，实际: %v", err)
	}
}

// 金额必须按 decimal 精确累加：0.10 + 0.20 占用后 used 必须精确等于 0.30，
// 条件 UPDATE 的表达式参数必须接受 decimal（验证无浮点 0.30000000000000004 尾差）。
func TestReserveBudget_FractionalExact(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := testsupport.NewFinanceService(gdb)
	txMgr := db.NewTxManager(gdb)
	ctx := context.Background()
	b := createBudget(t, gdb, testsupport.D("1000"))

	run := func(fn func(tx *gorm.DB) error) error { return txMgr.WithTx(ctx, fn) }

	if err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("0.10"))
	}); err != nil {
		t.Fatalf("占用 0.10 应成功: %v", err)
	}
	if err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("0.20"))
	}); err != nil {
		t.Fatalf("占用 0.20 应成功: %v", err)
	}
	if got := budgetUsed(t, gdb, b.ID); !got.Equal(testsupport.D("0.30")) {
		t.Fatalf("used 应精确为 0.30，实际 %v", got)
	}

	// 恰好用完剩余 999.70 应成功；再多占 0.01 必须被原子条件拒绝。
	if err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("999.70"))
	}); err != nil {
		t.Fatalf("占用至 1000.00 应成功: %v", err)
	}
	err := run(func(tx *gorm.DB) error {
		return svc.ReserveBudgetTx(ctx, tx, service.BudgetTypePurchase, testsupport.D("0.01"))
	})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != 60010 {
		t.Fatalf("超额 1 分应返回 60010，实际 %v", err)
	}
}
