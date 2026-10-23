// Copyright 2026 zhouhouping. All Rights Reserved.

package excel

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ExportCSV 生成带 UTF-8 BOM 的标准 CSV 字节流（Excel 打开不乱码）。
func ExportCSV(headers []string, rows [][]string) ([]byte, error) {
	var buf bytes.Buffer
	// 写入 UTF-8 BOM (0xEF, 0xBB, 0xBF)
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, row := range rows {
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImportCSV 解析 CSV 输入流，返回去除表头后的数据行。
func ImportCSV(r io.Reader) ([]string, [][]string, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("CSV 解析失败: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("CSV 内容为空")
	}

	headers := records[0]
	// 清理表头 BOM
	if len(headers) > 0 {
		headers[0] = strings.TrimPrefix(headers[0], "\ufeff")
	}

	rows := records[1:]
	return headers, rows, nil
}
