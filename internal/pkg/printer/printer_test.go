// Copyright 2026 zhouhouping. All Rights Reserved.

package printer_test

import (
	"strings"
	"testing"

	"erp-system/internal/pkg/printer"
)

func TestRenderDocumentHTML(t *testing.T) {
	data := printer.DocumentPrintData{
		Title:       "销售出库单",
		DocNo:       "OB-20261023-0001",
		DocDate:     "2026-10-23",
		PartnerName: "华为技术有限公司",
		Warehouse:   "深圳总仓",
		TotalAmount: "¥ 12,800.00",
		Items: []printer.PrintItem{
			{
				Index:        1,
				MaterialCode: "MAT-PROD-001",
				MaterialName: "智能网关控制器",
				Spec:         "V2.0",
				Unit:         "台",
				Qty:          "10",
				UnitPrice:    "1280.00",
				Amount:       "12800.00",
			},
		},
	}

	html, err := printer.RenderDocumentHTML(data)
	if err != nil {
		t.Fatalf("渲染打印 HTML 失败: %v", err)
	}
	if !strings.Contains(html, "OB-20261023-0001") || !strings.Contains(html, "华为技术有限公司") {
		t.Fatalf("打印内容缺失关键单据数据: %s", html)
	}
}
