// Copyright 2026 zhouhouping. All Rights Reserved.

package printer

import (
	"bytes"
	"fmt"
	"html/template"
)

// DocumentPrintData 单据打印数据结构
type DocumentPrintData struct {
	Title       string                 `json:"title"`
	DocNo       string                 `json:"doc_no"`
	DocDate     string                 `json:"doc_date"`
	PartnerName string                 `json:"partner_name"`
	Warehouse   string                 `json:"warehouse"`
	TotalAmount string                 `json:"total_amount"`
	Remark      string                 `json:"remark"`
	Items       []PrintItem            `json:"items"`
	Extra       map[string]interface{} `json:"extra,omitempty"`
}

type PrintItem struct {
	Index        int    `json:"index"`
	MaterialCode string `json:"material_code"`
	MaterialName string `json:"material_name"`
	Spec         string `json:"spec"`
	Unit         string `json:"unit"`
	Qty          string `json:"qty"`
	UnitPrice    string `json:"unit_price"`
	Amount       string `json:"amount"`
}

const docTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>{{.Title}} - {{.DocNo}}</title>
<style>
  body { font-family: "PingFang SC", "Microsoft YaHei", sans-serif; margin: 20px; color: #333; }
  .header { text-align: center; margin-bottom: 20px; border-bottom: 2px solid #333; padding-bottom: 10px; }
  .header h2 { margin: 0 0 8px 0; }
  .meta-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 15px; font-size: 14px; }
  table { width: 100%; border-collapse: collapse; margin-top: 10px; font-size: 13px; }
  th, td { border: 1px solid #666; padding: 6px 8px; text-align: left; }
  th { background-color: #f2f2f2; }
  .text-right { text-align: right; }
  .footer { margin-top: 25px; display: flex; justify-content: space-between; font-size: 13px; }
  @media print {
    body { margin: 0; }
    .no-print { display: none; }
  }
</style>
</head>
<body>
  <div class="header">
    <h2>{{.Title}}</h2>
    <div>单号：{{.DocNo}}</div>
  </div>
  <div class="meta-grid">
    <div><strong>往来单位/客户/供应商：</strong>{{.PartnerName}}</div>
    <div><strong>单据日期：</strong>{{.DocDate}}</div>
    <div><strong>关联仓库：</strong>{{.Warehouse}}</div>
    <div><strong>总金额：</strong>{{.TotalAmount}}</div>
  </div>
  <table>
    <thead>
      <tr>
        <th style="width: 40px;">#</th>
        <th>物料编码</th>
        <th>物料名称</th>
        <th>规格</th>
        <th>单位</th>
        <th class="text-right">数量</th>
        <th class="text-right">单价</th>
        <th class="text-right">金额</th>
      </tr>
    </thead>
    <tbody>
      {{range .Items}}
      <tr>
        <td>{{.Index}}</td>
        <td>{{.MaterialCode}}</td>
        <td>{{.MaterialName}}</td>
        <td>{{.Spec}}</td>
        <td>{{.Unit}}</td>
        <td class="text-right">{{.Qty}}</td>
        <td class="text-right">{{.UnitPrice}}</td>
        <td class="text-right">{{.Amount}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  <div class="footer">
    <div>制单人：_________</div>
    <div>经办人：_________</div>
    <div>审核人：_________</div>
    <div>打印时间：{{.DocDate}}</div>
  </div>
</body>
</html>`

// RenderDocumentHTML 渲染单据打印 HTML
func RenderDocumentHTML(data DocumentPrintData) (string, error) {
	tmpl, err := template.New("doc").Parse(docTemplate)
	if err != nil {
		return "", fmt.Errorf("模板解析失败: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("渲染失败: %w", err)
	}
	return buf.String(), nil
}
