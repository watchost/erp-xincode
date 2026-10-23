// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	financeService "erp-system/internal/finance/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

// P2 缺陷修复 #5：采购入库须在同一事务内生成 fin_payable 应付单，金额 = 入库数量 × 采购单价。
func TestPurchaseInbound_CreatesPayableInSameTx(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "poap")
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, wh, fin)
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	res, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("6")))
	if err != nil {
		t.Fatalf("入库失败: %v", err)
	}

	var row struct {
		SupplierID int64
		Amount     decimal.Decimal
	}
	if err := gdb.Table("fin_payable").Select("supplier_id, amount").
		Where("source_no = ?", res.InboundNo).Scan(&row).Error; err != nil {
		t.Fatalf("查询应付单失败: %v", err)
	}
	if row.SupplierID != fx.Supplier.ID || !row.Amount.Equal(testsupport.D("150")) {
		t.Fatalf("应付单应为供应商 %d 金额 150，实际 %+v", fx.Supplier.ID, row)
	}
}

// 应付单生成失败 → 入库、库存、凭证整事务回滚。
type failingPayableGateway struct{ testsupport.FinanceGateway }

func (failingPayableGateway) CreatePayableTx(ctx context.Context, tx *gorm.DB, in financeService.PayableInput) error {
	return apperrors.New(60006, 500, "注入的应付单生成失败")
}

func TestPurchaseInbound_PayableFailureRollsBack(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "poapf")
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseServiceWith(gdb, wh, failingPayableGateway{fin}, nil)
	ctx := testsupport.CtxWithUser(1)

	orderNo := createApprovedOrder(t, deps, fx, testsupport.D("10"), testsupport.D("25"))
	if _, err := deps.Service.InboundScan(ctx, inboundReq(orderNo, fx, testsupport.D("6"))); err == nil {
		t.Fatal("应付单生成失败时入库应报错")
	}
	var inv, vouchers, pay int64
	countWhere(t, gdb, "inv_inventory", &inv, "material_id = ?", fx.Material.ID)
	countWhere(t, gdb, "fin_voucher", &vouchers, "biz_type = ?", "purchase_inbound")
	countWhere(t, gdb, "fin_payable", &pay, "supplier_id = ?", fx.Supplier.ID)
	if inv != 0 || vouchers != 0 || pay != 0 {
		t.Fatalf("应整事务回滚：库存 %d、凭证 %d、应付单 %d 均应为 0", inv, vouchers, pay)
	}
}
