// Copyright 2026 zhouhouping. All Rights Reserved.

package workflow

import (
	"testing"

	"github.com/shopspring/decimal"
)

// 纯函数单元测试：不依赖数据库，本机可真正执行。

func TestParseFlow_Validation(t *testing.T) {
	ok := `{"nodes":[{"code":"start","type":"start"},
		{"code":"a","type":"approval","assignee":{"type":"role","value":"manager"},"sign_mode":"all"},
		{"code":"end","type":"end"}]}`
	if _, err := ParseFlow([]byte(ok)); err != nil {
		t.Fatalf("合法定义被拒: %v", err)
	}
	bad := map[string]string{
		"非 JSON":      `{`,
		"缺审批节点":      `{"nodes":[{"code":"start","type":"start"},{"code":"end","type":"end"}]}`,
		"审批节点缺审批人":   `{"nodes":[{"code":"start","type":"start"},{"code":"a","type":"approval"},{"code":"end","type":"end"}]}`,
		"不支持的审批人类型":  `{"nodes":[{"code":"start","type":"start"},{"code":"a","type":"approval","assignee":{"type":"x"}},{"code":"end","type":"end"}]}`,
		"非法 sign_mode": `{"nodes":[{"code":"start","type":"start"},{"code":"a","type":"approval","assignee":{"type":"dept_leader"},"sign_mode":"half"},{"code":"end","type":"end"}]}`,
		"节点 code 重复":   `{"nodes":[{"code":"start","type":"start"},{"code":"a","type":"approval","assignee":{"type":"dept_leader"}},{"code":"a","type":"approval","assignee":{"type":"dept_leader"}},{"code":"end","type":"end"}]}`,
		"不以 start 开头":  `{"nodes":[{"code":"a","type":"approval","assignee":{"type":"dept_leader"}},{"code":"start","type":"start"},{"code":"end","type":"end"}]}`,
		"不支持的节点类型":   `{"nodes":[{"code":"start","type":"start"},{"code":"p","type":"parallel"},{"code":"end","type":"end"}]}`,
	}
	for name, raw := range bad {
		if _, err := ParseFlow([]byte(raw)); err == nil {
			t.Errorf("%s：应校验失败", name)
		}
	}
}

func TestConditionMatch(t *testing.T) {
	vars := map[string]decimal.Decimal{"amount": decimal.RequireFromString("10000")}
	cases := []struct {
		op   string
		val  string
		want bool
	}{
		{">", "10000", false}, {">=", "10000", true}, {"<", "10001", true},
		{"<=", "9999", false}, {"==", "10000", true}, {"!=", "10000", false},
	}
	for _, c := range cases {
		got := conditionMatch(&Condition{Field: "amount", Op: c.op, Value: decimal.RequireFromString(c.val)}, vars)
		if got != c.want {
			t.Errorf("amount %s %s：期望 %v，实际 %v", c.op, c.val, c.want, got)
		}
	}
	if !conditionMatch(nil, vars) {
		t.Error("无条件的节点应参与审批")
	}
	if !conditionMatch(&Condition{Field: "missing", Op: ">", Value: decimal.Zero}, vars) {
		t.Error("缺少条件变量时不应跳过节点（宁可多审一级）")
	}
}
