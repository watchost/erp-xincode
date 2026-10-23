// Copyright 2026 zhouhouping. All Rights Reserved.

package printer

import (
	"fmt"
	"strings"
)

// code128Patterns Code 128 字符图案映射 (B 字符集子集，最通用)
var code128Patterns = []string{
	"212222", "222122", "222221", "121223", "121322", "131222", "122213", "122312", "132212", "221213",
	"221312", "231212", "112232", "122132", "122231", "113222", "123122", "123221", "223211", "221132",
	"221231", "213212", "223112", "312131", "311222", "321122", "321221", "312212", "322112", "322211",
	"212123", "212321", "232121", "111323", "131123", "131321", "112313", "132113", "132311", "211313",
	"231113", "231311", "112133", "112331", "132131", "113123", "113321", "133121", "313121", "211331",
	"231131", "213113", "213311", "213131", "311123", "311321", "331121", "312113", "312311", "332111",
	"314111", "221411", "431111", "111224", "111422", "121124", "121421", "141122", "141221", "112214",
	"112412", "122114", "122411", "142112", "142211", "241211", "221114", "413111", "241112", "134111",
	"111242", "121142", "121241", "114212", "124112", "124211", "411212", "421112", "421211", "212141",
	"214121", "412121", "111143", "111341", "131141", "114113", "114311", "411113", "411311", "113141",
	"114131", "311141", "411131", "211412", "211214", "211232", "2331112",
}

// GenerateCode128SVG 生成 Code 128 条码的纯 SVG 字符串（标准矢矢量图，高分辨率打印清晰无锯齿）。
func GenerateCode128SVG(content string, width, height int) (string, error) {
	if content == "" {
		return "", fmt.Errorf("条码内容不能为空")
	}
	if width <= 0 {
		width = 240
	}
	if height <= 0 {
		height = 60
	}

	// 起始符 B (值 104)
	codes := []int{104}
	checksum := 104
	for i, r := range content {
		val := int(r) - 32
		if val < 0 || val > 95 {
			val = 0
		}
		codes = append(codes, val)
		checksum += val * (i + 1)
	}
	checkDigit := checksum % 103
	codes = append(codes, checkDigit)
	// 终止符 (值 106)
	codes = append(codes, 106)

	var pattern strings.Builder
	for _, c := range codes {
		if c >= 0 && c < len(code128Patterns) {
			pattern.WriteString(code128Patterns[c])
		}
	}

	// 转换为模块宽度
	patStr := pattern.String()
	totalModules := 0
	for _, ch := range patStr {
		totalModules += int(ch - '0')
	}

	moduleWidth := float64(width) / float64(totalModules)
	var svg strings.Builder
	svg.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, width, height, width, height))
	svg.WriteString(fmt.Sprintf(`<rect width="%d" height="%d" fill="#ffffff"/>`, width, height))

	currentX := 0.0
	isBar := true
	for _, ch := range patStr {
		w := float64(ch-'0') * moduleWidth
		if isBar {
			svg.WriteString(fmt.Sprintf(`<rect x="%.2f" y="0" width="%.2f" height="%d" fill="#000000"/>`, currentX, w, height-15))
		}
		currentX += w
		isBar = !isBar
	}

	// 底部文字
	svg.WriteString(fmt.Sprintf(`<text x="%d" y="%d" font-family="monospace" font-size="12" text-anchor="middle" fill="#000000">%s</text>`, width/2, height-2, content))
	svg.WriteString(`</svg>`)

	return svg.String(), nil
}
