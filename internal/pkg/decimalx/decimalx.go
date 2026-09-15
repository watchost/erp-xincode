// Copyright 2026 zhouhouping. All Rights Reserved.

// Package decimalx 提供金额/数量字段的统一舍入助手。
//
// 舍入纪律（见 docs/development/00-conventions.md §5）：
//   - 只在两个边界做 Round：①入参 DTO 转落库 model 时（按目标列标度）；
//     ②跨精度运算结果落库前（如移动平均价、金额乘积）。
//   - 中间运算一律保留 decimal 全精度，禁止反复 Round。
//   - 除法必须使用 DivRound（或先 DivRound 到保护精度再 Round），
//     shopspring/decimal 的裸 Div 遇到除不尽会 panic。
//
// 对应库表标度：金额 NUMERIC(18,2)、数量 NUMERIC(18,3)、
// 单价/成本/BOM 用量 NUMERIC(18,4)、比率 NUMERIC(8,4)。
package decimalx

import "github.com/shopspring/decimal"

func init() {
	// 全局策略：decimal 序列化为不带引号的 JSON number，保证前端 wire 契约
	// （前端按数值字段消费，本期前端零改动）。shopspring 默认输出 "1.23" 字符串。
	// 反序列化同时接受 number 与数字字符串，旧报文不受影响。
	decimal.MarshalJSONWithoutQuotes = true
}

const (
	PlacesMoney = 2 // 金额：凭证/预算/订单/入库单/台账金额
	PlacesQty   = 3 // 数量
	PlacesCost  = 4 // 单价/成本/BOM 用量/fin_cost.amount
	PlacesRate  = 4 // 比率（scrap_rate 等）
)

// Money 四舍五入到 2 位小数（金额）。
func Money(d decimal.Decimal) decimal.Decimal { return d.Round(PlacesMoney) }

// Qty 四舍五入到 3 位小数（数量）。
func Qty(d decimal.Decimal) decimal.Decimal { return d.Round(PlacesQty) }

// Cost 四舍五入到 4 位小数（单价/成本）。
func Cost(d decimal.Decimal) decimal.Decimal { return d.Round(PlacesCost) }

// Rate 四舍五入到 4 位小数（比率）。
func Rate(d decimal.Decimal) decimal.Decimal { return d.Round(PlacesRate) }
