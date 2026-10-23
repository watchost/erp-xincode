// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"testing"

	"gorm.io/gorm"

	financeService "erp-system/internal/finance/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
	"github.com/shopspring/decimal"
)

// P2 缺陷修复 #5：销售出库须在同一事务内生成 fin_receivable 应收单，金额 = 本次确认收入。
func TestSalesOutbound_CreatesReceivableInSameTx(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "soar")
	customer := testsupport.CreateCustomer(t, gdb, "soar", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("10"), testsupport.D("10"))
	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("4"), testsupport.D("25"), testsupport.D("0")) // 收入 = 4*25 = 100

	res, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("4")))
	if err != nil {
		t.Fatalf("出库失败: %v", err)
	}

	var row struct {
		CustomerID int64
		Amount     decimal.Decimal
	}
	if err := gdb.Table("fin_receivable").Select("customer_id, amount").
		Where("source_no = ?", res.OutboundNo).Scan(&row).Error; err != nil {
		t.Fatalf("查询应收单失败: %v", err)
	}
	if row.CustomerID != customer.ID || !row.Amount.Equal(testsupport.D("100")) {
		t.Fatalf("应收单应为客户 %d 金额 100，实际 %+v", customer.ID, row)
	}
}

// 应收单生成失败 → 出库、库存、凭证整事务回滚（证明“同一事务”）。
type failingReceivablePoster struct{ inner financeService.Poster }

func (p failingReceivablePoster) PostTx(ctx context.Context, tx *gorm.DB, in financeService.PostingInput) error {
	return p.inner.PostTx(ctx, tx, in)
}
func (failingReceivablePoster) CreateReceivableTx(ctx context.Context, tx *gorm.DB, in financeService.ReceivableInput) error {
	return apperrors.New(60005, 500, "注入的应收单生成失败")
}
func (failingReceivablePoster) CreatePayableTx(ctx context.Context, tx *gorm.DB, in financeService.PayableInput) error {
	return nil
}

func TestSalesOutbound_ReceivableFailureRollsBack(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "soarf")
	customer := testsupport.CreateCustomer(t, gdb, "soarf", decimal.Zero)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewSalesServiceWith(gdb, wh, failingReceivablePoster{inner: fin}, nil)
	ctx := testsupport.CtxWithUser(1)

	seedStock(t, wh, fx, testsupport.D("10"), testsupport.D("10"))
	orderNo := createApprovedOrder(t, deps, customer.ID, fx.Material.ID,
		testsupport.D("4"), testsupport.D("25"), testsupport.D("0"))

	if _, err := deps.Service.OutboundScan(ctx, outboundReq(orderNo, fx, testsupport.D("4"))); err == nil {
		t.Fatal("应收单生成失败时出库应报错")
	}
	var vouchers, recv int64
	countWhere(t, gdb, "fin_voucher", &vouchers, "biz_type = ?", "sales_outbound")
	countWhere(t, gdb, "fin_receivable", &recv, "customer_id = ?", customer.ID)
	if vouchers != 0 || recv != 0 {
		t.Fatalf("应整事务回滚：凭证 %d、应收单 %d 均应为 0", vouchers, recv)
	}
	var invCount int64
	countWhere(t, gdb, "inv_inventory", &invCount, "material_id = ? AND qty = ?", fx.Material.ID, testsupport.D("10"))
	if invCount != 1 {
		t.Fatal("库存应回滚为 10")
	}
}
