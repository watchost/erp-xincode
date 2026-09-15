// Copyright 2026 zhouhouping. All Rights Reserved.

package decimalx_test

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"erp-system/internal/pkg/decimalx"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestRounders(t *testing.T) {
	cases := []struct {
		name string
		fn   func(decimal.Decimal) decimal.Decimal

		in, want string
	}{
		{"金额 2.345 四舍五入", decimalx.Money, "2.345", "2.35"},
		{"金额 2.344", decimalx.Money, "2.344", "2.34"},
		{"金额负数 half away from zero", decimalx.Money, "-2.345", "-2.35"},
		{"数量保留 3 位", decimalx.Qty, "0.005", "0.005"},
		{"数量 1.2345→1.235", decimalx.Qty, "1.2345", "1.235"},
		{"成本保留 4 位", decimalx.Cost, "100.090909", "100.0909"},
		{"成本第 5 位进位", decimalx.Cost, "1.23456", "1.2346"},
		{"比率保留 4 位", decimalx.Rate, "0.05001", "0.05"},
		{"零值", decimalx.Money, "0", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.fn(d(c.in)); !got.Equal(d(c.want)) {
				t.Fatalf("%s(%s) = %s, want %s", c.name, c.in, got, c.want)
			}
		})
	}
}

// decimal 必须以裸 JSON number 出线（前端零改动契约）。
func TestMarshalJSONEmitsBareNumber(t *testing.T) {
	type vo struct {
		Amount decimal.Decimal  `json:"amount"`
		Min    *decimal.Decimal `json:"min_stock"`
	}
	b, err := json.Marshal(vo{Amount: decimal.RequireFromString("3.5")})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"amount":3.5,"min_stock":null}` {
		t.Fatalf("decimal 应以裸 number 出线，实际 %s", b)
	}

	// 反序列化同时接受 number 与数字字符串
	for _, payload := range []string{`{"amount":3.5}`, `{"amount":"3.5"}`} {
		var got vo
		if err := json.Unmarshal([]byte(payload), &got); err != nil {
			t.Fatalf("payload %s 解析失败: %v", payload, err)
		}
		if !got.Amount.Equal(decimal.RequireFromString("3.5")) {
			t.Fatalf("payload %s 回读错误: %v", payload, got.Amount)
		}
	}
}
