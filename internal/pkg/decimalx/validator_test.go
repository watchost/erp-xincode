// Copyright 2026 zhouhouping. All Rights Reserved.

package decimalx_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"

	"erp-system/internal/pkg/decimalx"
)

type decForm struct {
	V decimal.Decimal `binding:"decgt=0"`
}

type decGteForm struct {
	V decimal.Decimal `binding:"decgte=0"`
}

type decPtrForm struct {
	V *decimal.Decimal `binding:"decgt=0"`
}

func newGinValidator() *validator.Validate {
	v := validator.New()
	v.SetTagName("binding") // 与 gin 默认 validator 一致
	decimalx.RegisterValidators(v)
	return v
}

func TestDecGT(t *testing.T) {
	v := newGinValidator()

	if errs := v.Struct(decForm{V: d("0.01")}); errs != nil {
		t.Fatalf("0.01 应通过 decgt=0: %v", errs)
	}
	if errs := v.Struct(decForm{V: decimal.Zero}); errs == nil {
		t.Fatal("0 不应通过 decgt=0")
	}
	if errs := v.Struct(decForm{V: d("-1")}); errs == nil {
		t.Fatal("-1 不应通过 decgt=0")
	}
}

func TestDecGTE(t *testing.T) {
	v := newGinValidator()

	if errs := v.Struct(decGteForm{V: decimal.Zero}); errs != nil {
		t.Fatalf("0 应通过 decgte=0: %v", errs)
	}
	if errs := v.Struct(decGteForm{V: d("-0.01")}); errs == nil {
		t.Fatal("-0.01 不应通过 decgte=0")
	}
}

func TestNilPointerPasses(t *testing.T) {
	v := newGinValidator()

	if errs := v.Struct(decPtrForm{V: nil}); errs != nil {
		t.Fatalf("nil 指针应放行: %v", errs)
	}
	pos := d("1")
	if errs := v.Struct(decPtrForm{V: &pos}); errs != nil {
		t.Fatalf("非 nil 正值应通过: %v", errs)
	}
	zero := decimal.Zero
	if errs := v.Struct(decPtrForm{V: &zero}); errs == nil {
		t.Fatal("非 nil 零值不应通过 decgt=0")
	}
}

type badTagForm struct {
	V int `binding:"decgt=0"`
}

func TestWrongTypePanics(t *testing.T) {
	v := newGinValidator()

	defer func() {
		if recover() == nil {
			t.Fatal("decgt 挂在 int 字段上应 panic")
		}
	}()
	_ = v.Struct(badTagForm{V: 1})
}
