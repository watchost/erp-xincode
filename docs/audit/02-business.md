# 02 · 业务逻辑与正确性审计

> 审计范围：`internal/warehouse`、`purchase`、`production`、`finance`、`mdm`、`dashboard`、`llm`、`device`、`internal/pkg/db`
> 分级标准：严重 = 会导致资金/库存账实不符

---

## 一、严重（资金/库存账实不符）

### S1 移动平均成本公式写死为 0，库存估值永久归零
- 文件：`internal/warehouse/service/service.go:94-98`

```go
if inv.Qty == 0 {
    inv.AvgCost = 0
} else {
    inv.AvgCost = (inv.AvgCost*inv.Qty + 0) / (inv.Qty + req.Qty)
}
```

采购入库 10 个、单价 50 元，分子第二项应是 `unitCost*req.Qty = 500`，实际写死 `+0`。入库后 `avg_cost=0`；再次入库仍为 0。`TotalValue = qty*avg_cost = 0`，仪表库存价值、出库成本（`service.go:183 inv.AvgCost*req.Qty`）、财务成本全部为 0。`WarehouseService.Inbound` 根本不接收单价，`matchedItem.Price` 从未传入。

修复：入库接口加 `unitCost`；公式改 `(avgCost*qty + unitCost*inQty)/(qty+inQty)`；零库存首入 `avgCost=unitCost`；退库/冲销单独处理。

### S2 采购入库把库存写入与订单更新拆成两个独立事务
- 文件：`internal/purchase/service/service.go:227 与 232`

```go
warehouseRes, err := s.warehouseService.Inbound(ctx, inboundReq) // 事务1：已提交 inv+ledger
...
err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {          // 事务2：订单 + 入库单
    s.orderRepo.UpdateItemReceivedQty(tx, matchedItem.ID, req.Qty)
    return s.inboundRepo.Create(tx, &inbound)
})
```

事务1 提交后事务2 因单号唯一键冲突（见 S6）或 DB 抖动失败 → 库存已加、台账已记，但 `received_qty` 未加、入库单无记录。重试会再次加库存。生产领料 `MaterialIssueScan`（`production/service.go:235/240`）同样问题。

修复：库存、台账、订单数量、入库单、财务凭证进同一个 `*gorm.DB` 事务；仓库 service 接收 `tx`。

### S3 库存/订单数量更新无行锁，并发超卖/超收/丢失更新
全代码无 `FOR UPDATE` / `clauses.Locking`。

**(a) 入库读不在事务里** — `warehouse/service/service.go:78` 调 `FindByMaterialWarehouse`，repo 用的是 `r.db` 而非 `tx`（`warehouse/repository/repo.go:31`），随后 `tx.Save(inv)` 用旧值覆盖。两个并发入库后写覆盖先写。

**(b) 出库读后写无锁** — `warehouse/service/service.go:149-165`：
```go
tx.Where(...).First(&inv)
if inv.AvailableQty < req.Qty { return ... }
inv.Qty -= req.Qty; inv.AvailableQty -= req.Qty
tx.Save(&inv)
```
两个并发出库读到相同值，都通过校验 → 只扣一次或扣成负数。

**(c) 采购超收校验在内存、UPDATE 无条件守卫** — `purchase/service.go:217` 加 `purchase/repository/repo.go:78`：
```go
if matchedItem.ReceivedQty+req.Qty > matchedItem.Qty { return ... }
... Update("received_qty", gorm.Expr("received_qty + ?", qty)) // 无 qty 上限
```
生产领料对 `issued_qty/plan_qty` 同样超领。

修复：原子条件 UPDATE：
```sql
UPDATE inv_inventory SET qty=qty+?, available_qty=available_qty+?
WHERE id=? AND available_qty >= ?;  -- 检查 RowsAffected
```
收货加 `AND received_qty+? <= qty`；或事务内 `SELECT ... FOR UPDATE`。

### S4 数据库表结构与 Go 模型大面积不匹配，生产/财务模块运行即报错
main.go 无 AutoMigrate、用默认命名策略。

| 表 | DB 实际列 | Go 模型字段 | 后果 |
|---|---|---|---|
| `prod_work_order` | `wo_no, bom_id`；无 `produced_qty/plan_start_at/...` | `WorkOrderNo→work_order_no`、`ProducedQty`、`PlanStartAt`... | 工单创建/查询/下达/领料全部 SQL 报错 |
| `prod_work_order_bom` | `req_qty, picked_qty, is_substitute` | `PlanQty→plan_qty`、`IssuedQty→issued_qty`、`Unit` | 领料读写报错；替代品流程未实现 |
| `prod_bom/prod_bom_item` | **表不存在** | `ProBom/ProBomItem` | 创建 BOM、建工单前查 BOM 报 relation 不存在 |
| `fin_voucher` | `voucher_no, debit, credit, period, biz_type SMALLINT` | `entry_no, account_code, debit_amount, biz_type string`... | 凭证列表/余额/报表全部报错 |
| `fin_cost` | `cost_type SMALLINT, ref_id, period` | `product_id, cost_date, source_type, cost_type string` | 成本卡/汇总全部报错 |
| `fin_budget` | **表不存在** | `FinBudget` | 预算列表报错 |

修复：以模型为准重写迁移或反向修模型；补齐缺失表/列；加真实 PG 集成测试。

### S5 财务凭证从不生成、预算从不校验、报表口径错误
- `accountEntryRepo.Create`、`costCardRepo.Create` 在 service 层零调用；采购入库/生产领料无会计分录；无凭证号生成；无借贷平衡校验。
- `budgetRepo.FindByTypeAndPeriod/UpdateUsedAmount` 零调用——预算纯展示，下单不校验不扣减。
- 报表 `finance/service.go:147-161`：
```go
totalAssets, _ := s.accountEntryRepo.GetBalance("1000", endDate)        // 只取单一科目
... .Where("account_code LIKE '5%'").Select("SUM(debit_amount)")        // 收入类应取贷方
... .Where("account_code LIKE '6%'").Select("SUM(debit_amount)")
```
  - 中国准则下 5xxx 收入类为贷方发生额，取借方得到冲销/退款，毛利为负或 0；
  - 资产/负债只取 `1000/2000` 而非前缀 `1%/2%`；
  - 期间用 `created_at <= period+"-01"`，传 `2026-08` 只统计到 8 月 1 日；
  - 错误全部 `_ =` 吞掉，报表恒返回零。

修复：业务发生时同事务写借贷双分录并校验平衡；预算下单时原子校验+扣减；报表按 `period`、科目前缀、正确借贷方向汇总；不要吞错。

---

## 二、高

### H1 业务单号用秒级时间戳生成，并发冲突并与 S2 叠加造成重复库存
- 文件：`warehouse/service/service.go:74,173`、`purchase/service/service.go:61`、`production/service/service.go:69`、`finance/service/service.go:231`

```go
inboundNo := fmt.Sprintf("IB%v", time.Now().Unix())
```

同秒两笔入库得相同单号，唯一索引冲突；事务2 失败但事务1（库存）已提交，重试后库存翻倍。

修复：DB sequence/雪花/UUID；单号与业务写同一事务；扫码加幂等键。

### H2 扫码接口无幂等，重复提交重复入账
无幂等键、无 `biz_no` 去重；台账对 `biz_no` 只有普通索引。PDA 重发/误扫两次，不同秒生成不同单号，库存与台账各加两次。

修复：客户端幂等键（或 `biz_no+material+qty` 唯一约束）；台账对业务单号加唯一约束。

### H3 JWT 鉴权中间件未真正校验 token
- 文件：`internal/pkg/middleware/middleware.go:82-92`

任意非空 Authorization 即可访问；`c.GetInt64("user_id")` 恒为 0；`CreatedBy` 硬编码为 1（`purchase/service.go:88,244`、`production/service.go:81`），审计无法追溯到人。（详见 [01-security.md](01-security.md) S-1）

### H4 LLM 会话越权（IDOR）且功能不可用
- `GetSessionHistory(ctx, sessionID)` 不接收也不校验 userID；`Chat` 在 sessionID≠0 时不校验归属。
- `buildMessages`（`service.go:81-95`）只发 system+当前问题，从不加载历史，多轮无上下文。
- `main.go:129` 用空 API Key 初始化；`WenXinGateway.Chat` 返回"暂未对接"且 `nil` error。

### H5 出库完全忽略库位，库位级库存错乱
- 文件：`internal/warehouse/service/service.go:149`

```go
tx.Where("material_id = ? AND warehouse_id = ?", material.ID, warehouse.ID).First(&inv)
```

入库按 `location_id` 写（`UNIQUE(material,warehouse,location)`），出库不带库位，`First()` 取主键最小行。A-01 有 0、A-02 有 100 → 出库报"库存不足"；或扣错库位。`OutboundScanReq` 没有 `location_code`。

修复：出库必须带库位，查询加 `location_id` 并加锁/原子更新。

### H6 采购入库仓库硬编码 ID=1，多仓库不可用
- 文件：`purchase/service.go:241`、`warehouse/service/service.go:61,138`

`WarehouseID: 1`；默认 `warehouse := &MdmWarehouse{ID:1}`；请求无 `warehouse_code`。WH002 收货仍记到 WH001。

### H7 MDM 多个写接口是空实现
- 文件：`internal/mdm/service/service.go`

`DeleteMaterial`（:106）、`DeleteSupplier`（:173）、`CreateWarehouse`（:177）、`CreateLocation`（:245）均 `return nil`。接口返回 200 但不写库。

---

## 三、中

### M1 状态机不完整、状态值与库表默认值冲突
- `OrderStatusCompleted=3`、`WorkOrderStatusCompleted=4` 从未被赋值；无生产入库/报工端点，`BizTypeProductionInbound` 定义了无人用。
- `ApproveOrder` 只更新 status，不写 `approved_at`。
- 迁移 `DEFAULT 10`，Go 常量 1/2/3，语义不一致。

### M2 Timeout 中间件会在超时后继续执行 handler 并重复写响应
- `internal/pkg/middleware/middleware.go:62-80`：`c.Next()` 在 goroutine 中执行，超时主流程写 504 并 Abort，但 handler goroutine 仍在跑，可能再写响应（gin "superfluous call" panic），或在客户端放弃后仍提交事务。配合无幂等（H2），客户端重试导致重复入库。

### M3 列表接口普遍 N+1
- `warehouse/service/service.go:206-209`：每行库存各查物料/仓库/库位；
- `purchase/service.go:128-131`+`buildOrderVO:145`：每订单每物料查一次；
- `production/service.go:128-131`+`147` 同理；
- `finance/service.go:50,93`：按产品逐个查物料。

修复：Preload/Join 批量查询。

### M4 金额与数量全部使用 float64
所有 model 的 `Qty/Price/AvgCost/TotalAmount/DebitAmount/CostAmount` 均为 `float64`，库表为 NUMERIC。浮点累加误差导致对账几分钱差异。

修复：`shopspring/decimal` 或整数分/毫克。

### M5 Dashboard 静默吞错返回零值
- `internal/dashboard/service/service.go:32-53`：8 处 `_ = s.db...`，SQL 出错（含 S4 列不存在）KPI 全返回 0；库存价值受 S1 影响恒为 0。

### M6 GORM Save 全字段覆盖、无租户隔离
- `mdm/repository/repo.go:66,118` `db.Save(material)`，半填充结构体会把未传字段刷成零值。
- 业务表无 `tenant_id`，查询无租户条件。

### M7 财务报表期间维度错误
按 `created_at` 而非 `period` 过滤，`endDate = period+"-01"` 区间错误；`GetBalance` 按科目编码精确匹配而非级次汇总。

---

## 四、低

- **L1** `warehouse/service.go:192` 出库响应把 `InboundNo` 设为 `req.OutboundNo`，调用方未传时服务内生成的 `outboundNo`（:173）被丢弃，响应单号为空。
- **L2** `mdm/service.go:47,77,120,146` 物料/供应商 `attributes` 的 `json.Unmarshal` 错误被忽略，坏 JSON 静默变 nil。
- **L3** `middleware.go:22` TraceID 用 `context.Background()` 重建 ctx，覆盖原 ctx（丢失超时/取消）。
- **L4** `purchase/service.go:70` `_ = material` 死代码。
- **L5** `openapi/service.go:76-106` 刷新令牌只删旧 access_token，旧 refresh_token 仍可重复使用（无旋转失效）。
- **L6** 库存预警阈值硬编码 `<10/<5`，物料无 `min_stock` 字段。
- **L7** `CreateMaterial/CreateSupplier` "编码已存在"检查是事务外 TOCTOU，靠唯一索引兜底，错误信息不友好。
- **L8** `dashboard/service.go:58-66` `GetStockAlerts/GetRecentOrders` 返回空切片；`GetLLMAnalysis` 返回写死字符串。

---

## 五、整体评价与必修清单

分层结构清晰，但**业务正确性远未达可上线状态**。仓库与采购表结构与代码尚能对应，但存在成本归零、事务拆分、无并发控制、单号冲突、无幂等等资金级缺陷；**生产与财务模块因模型与迁移大面积不匹配，端到端不可用**；财务核算链路基本缺失；JWT 是空壳；LLM 越权且未真正配置。当前代码更接近脚手架/Demo，不能作为真实 ERP 记账依据。

### 必须先修（会导致账实不符/资金错误）
1. **S1** 移动平均成本写死 0，且入库不传单价 → 库存估值全错。
2. **S2** 采购入库/生产领料跨两个独立事务 → 必现账实不符。
3. **S3** 库存/收货/领料读-改-写无行锁 → 并发必超卖/超收/超领。
4. **S4** 生产/财务模型与迁移不匹配、缺表 → 模块运行即报错。
5. **S5** 凭证不生成、预算不校验、报表借贷方向/科目汇总错误。
6. **H1+H2** 秒级时间戳单号 + 无幂等 → 重复入库。
7. **H3** JWT 不校验，`CreatedBy` 硬编码为 1。
8. **H5/H6** 出库忽略库位、仓库硬编码 → 库位/仓库级库存错乱。

> 以上 8 项不修，库存数量、存货金额、应收应付、预算执行均无法保证准确，不建议投入任何真实业务数据。
