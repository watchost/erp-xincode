// Copyright 2026 zhouhouping. All Rights Reserved.

package excel_test

import (
	"bytes"
	"testing"

	"erp-system/internal/pkg/excel"
)

func TestExportAndImportCSV(t *testing.T) {
	headers := []string{"物料编码", "物料名称", "单位", "数量"}
	rows := [][]string{
		{"MAT-001", "电阻 10k", "个", "100"},
		{"MAT-002", "电容 100uF", "个", "50"},
	}

	csvBytes, err := excel.ExportCSV(headers, rows)
	if err != nil {
		t.Fatalf("导出 CSV 失败: %v", err)
	}

	readHeaders, readRows, err := excel.ImportCSV(bytes.NewReader(csvBytes))
	if err != nil {
		t.Fatalf("导入 CSV 失败: %v", err)
	}

	if len(readHeaders) != 4 || readHeaders[0] != "物料编码" {
		t.Fatalf("表头解析错误: %v", readHeaders)
	}
	if len(readRows) != 2 || readRows[0][0] != "MAT-001" {
		t.Fatalf("数据行解析错误: %v", readRows)
	}
}
