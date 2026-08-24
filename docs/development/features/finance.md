# 财务模块

## 1. 目标
完整的财务核算：会计科目、凭证（自动+手工）、预算控制、应收应付、收付款核销、期末结转、三大报表。

## 2. 数据模型
见 [schema.md P0-2](../schema.md#p0-2-修正财务模块) 和 [P2-5](../schema.md#p2-5-应收应付)。

- `fin_account` 会计科目表（树形）；
- `fin_voucher` 凭证主表 + `fin_voucher_entry` 分录；
- `fin_budget` 预算；
- `fin_cost` 成本发生；
- `fin_receivable` / `fin_payable` 应收应付；
- `fin_receipt` / `fin_payment` 收付款；
- `fin_settlement` 核销。

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /accounts | 会计科目 CRUD |
| GET/POST | /vouchers | 凭证列表/手工创建 |
| POST | /vouchers/:no/post | 过账 |
| GET | /vouchers/:no | 凭证详情 |
| GET/POST | /budgets | 预算 |
| GET | /receivables | 应收列表 |
| GET | /payables | 应付列表 |
| GET/POST | /receipts | 收款单 |
| GET/POST | /payments | 付款单 |
| POST | /settlements | 核销 |
| GET | /reports/balance-sheet?period=2026-08 | 资产负债表 |
| GET | /reports/income-statement?period= | 利润表 |
| GET | /reports/cash-flow?period= | 现金流量表 |
| GET | /reports/aging?direction=recv&customer_id= | 账龄分析 |
| POST | /period/close?period= | 期末结转 |

## 4. 关键逻辑

### 4.1 会计科目
树形结构，编码规则：
- 1xxx 资产（借方余额）
- 2xxx 负债（贷方）
- 3xxx 权益
- 4xxx 成本
- 5xxx 收入（贷方）
- 6xxx 费用（借方）

`is_leaf` 标记末级科目，只有末级科目可以记账。

### 4.2 凭证规则
- 一张凭证至少 2 条分录；
- 借方合计 = 贷方合计（`decimal.Equal`），不等则拒绝；
- 凭证号：`FV` + 期间 + 流水（每月重置）；
- 状态：草稿 → 已过账。已过账不可编辑，只能红字冲销；
- 每个分录必须是末级科目。

### 4.3 自动凭证模板

| 业务 | 借方 | 贷方 |
|---|---|---|
| 采购入库（票货同到） | 库存商品、应交税费-进项 | 应付账款 |
| 销售出库 | 主营业务成本 | 库存商品 |
| 销售确认 | 应收账款 | 主营业务收入、应交税费-销项 |
| 生产领料 | 生产成本-直接材料 | 库存商品 |
| 生产入库 | 库存商品 | 生产成本-直接材料/人工/制费 |
| 盘盈 | 库存商品 | 待处理财产损溢 |
| 盘亏 | 待处理财产损溢 | 库存商品 |
| 收款 | 银行存款 | 应收账款 |
| 付款 | 应付账款 | 银行存款 |

所有自动凭证与业务单据在**同一个数据库事务**里写入。

### 4.4 预算控制
- 预算按 `account_code + period` 维度；
- 业务下单（采购下单、费用申请）时检查：`used_amount + 本次金额 <= amount`；
- 原子扣减：
  ```sql
  UPDATE fin_budget SET used_amount = used_amount + ?
  WHERE account_code = ? AND period = ? AND used_amount + ? <= amount;
  ```
  RowsAffected=0 返回"预算不足"；
- 单据取消时回退预算。

### 4.5 应收应付
- 销售出库/采购入库时自动生成应收/应付单；
- `amount` 为含税金额，`received_amount/paid_amount` 记录已核销金额；
- 账期 `due_at`，超期列入账龄；
- 收付款单通过 `fin_settlement` 与应收应付核销（支持部分核销、多对多）。

### 4.6 账龄分析
按 `now - due_at` 分桶：
- 未到期
- 0-30 天
- 31-60 天
- 61-90 天
- 91-180 天
- 180+ 天

### 4.7 期末结转
- 损益类科目（5xxx、6xxx）余额结转到 `3xxx 本年利润`；
- 生成结转凭证；
- 结转后该期间不可再录入凭证（关账）。

### 4.8 报表取数
- **资产负债表**：按科目类别（1%/2%/3%）取期末余额；
- **利润表**：收入类取贷方发生额（5xxx），费用类取借方发生额（6xxx）；
- **现金流量表**：按现金/银行存款科目的对方科目分类汇总；
- 期间按 `period`（YYYY-MM）过滤，不用 `created_at`。

## 5. 边界条件
- 借贷不平拒绝；
- 非末级科目不能记账；
- 已过账凭证不可改，只能红字冲销；
- 预算不足可拦截（可配置为仅警告）；
- 已关账期间不可新增凭证。

## 6. 测试要点
- 凭证借贷平衡校验；
- 自动凭证与业务单据同事务（业务失败回滚）；
- 预算超支拦截；
- 收付款核销后剩余应收/应付正确；
- 期末结转后损益科目余额为 0；
- 报表数字与凭证汇总一致。
