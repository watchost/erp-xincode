// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// P1.5 H12：会话标题必须按 rune 截断且产物恒为合法 UTF-8——
// 旧实现 s[:maxLen] 按字节切割会切断 3 字节中文字，非法序列写入 PG
// 报 invalid byte sequence，长中文提问建会话直接 500。
func TestTruncateTitle(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		maxLen    int
		want      string
		wantRunes int // 期望 rune 数（不含追加的 "..."）；-1 表示不校验
	}{
		{
			name:      "短标题原样返回",
			input:     "库存情况如何",
			maxLen:    50,
			want:      "库存情况如何",
			wantRunes: 6,
		},
		{
			name:      "恰好 50 个汉字不追加省略号",
			input:     strings.Repeat("汉", 50),
			maxLen:    50,
			want:      strings.Repeat("汉", 50),
			wantRunes: 50,
		},
		{
			name:      "超长中文按 rune 截断为 50 字加省略号",
			input:     strings.Repeat("汉", 60),
			maxLen:    50,
			want:      strings.Repeat("汉", 50) + "...",
			wantRunes: 50,
		},
		{
			name:      "中英混排按字符数截断而非字节数",
			input:     strings.Repeat("a汉", 30), // 60 rune / 120 字节
			maxLen:    50,
			want:      strings.Repeat("a汉", 25) + "...",
			wantRunes: 50,
		},
		{
			name:      "输入本身含非法 UTF-8 字节时先兜底清洗",
			input:     "正常标题" + string([]byte{0xff, 0xfe}) + strings.Repeat("字", 60),
			maxLen:    50,
			wantRunes: 50,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateTitle(tc.input, tc.maxLen)
			if !utf8.ValidString(got) {
				t.Fatalf("产物必须是合法 UTF-8，实际 %q", got)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("期望 %q，实际 %q", tc.want, got)
			}
			if tc.wantRunes >= 0 {
				body := strings.TrimSuffix(got, "...")
				if n := utf8.RuneCountInString(body); n != tc.wantRunes {
					t.Fatalf("截断后应为 %d 个字符，实际 %d（%q）", tc.wantRunes, n, got)
				}
			}
			// 长度不变式：只有输入超过 maxLen 个字符才追加省略号
			if utf8.RuneCountInString(strings.ToValidUTF8(tc.input, "")) <= tc.maxLen && strings.HasSuffix(got, "...") {
				t.Fatalf("未超长的输入不应追加省略号，实际 %q", got)
			}
		})
	}
}
