# 库存管理增强：盘点 / 调拨 / 批次

## 0. 当前 P0 基础库存能力（2026-08-14）

当前已完成采购、生产主链路所需的基础库存能力：

- 库存唯一维度为 `material_id + warehouse_id + location_id`；
- 入库/出库必须指定 `warehouse_code` 和 `location_code`，已删除采购入库硬编码仓库 ID=1 的逻辑；
- 库存行在事务内使用 `SELECT ... FOR UPDATE` 锁定，入库使用 `ON CONFLICT DO UPDATE` 原子 upsert；
- 出库同时校验 `qty` 和 `available_qty`，不足时返回 HTTP 409；
- 入库按移动加权平均成本计算：`new_avg = (old_avg*old_qty + unit_cost*in_qty)/(old_qty+in_qty)`；采购入库从订单明细单价带入成本；
- 出库按当前平均成本结转 `cost_amount = avg_cost * qty` 并写 `inv_stock_ledger`；
- 采购服务和生产服务通过 `ApplyInboundTx` / `ApplyOutboundTx` 在调用方事务内更新库存，避免库存已提交但单据失败的账实不符；
- 业务单号使用 Redis 日流水号生成，格式如 `IB20260814-000001`、`OB20260814-000001`；
- 采购入库、生产领料强制 `Idempotency-Key`，重复提交返回 HTTP 429。

待完成：数据库级台账唯一约束、库存冻结/盘点、调拨、批次/序列号、金额 decimal 化、库存预警阈值主数据化。

## 一、库存盘点

### 1. 目标
定期盘点库存，自动调整账实差异并生成财务凭证。

### 2. 数据模型
见 [schema.md P2-2](../schema.md#p2-2-盘点)。
- `inv_stock_check` 盘点单；
- `inv_stock_check_item` 盘点明细。

### 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /stock-checks | 创建盘点单（草稿，快照账面数量） |
| POST | /stock-checks/:no/freeze | 冻结仓库（禁止该仓库出入库） |
| POST | /stock-checks/:no/items | 录入实盘数量（可多次） |
| POST | /stock-checks/:no/submit | 提交待审批 |
| POST | /stock-checks/:no/approve | 审批通过，自动调账 |
| POST | /stock-checks/:no/cancel | 取消并解冻 |
| GET | /stock-checks | 列表 |

### 4. 关键逻辑

**创建盘点单**：按仓库/库位/物料查询当前 `inv_inventory`，把 `qty` 作为 `book_qty` 写入明细。

**冻结**：`inv_stock_check.frozen = TRUE`。所有出入库 service 检查目标仓库是否有未完成的盘点单，有则拒绝。

**审批通过（单事务）**：
```
for each item:
    diff = actual_qty - book_qty
    if diff > 0: 盘盈入库（biz_type=盘盈，unit_cost=当前 avg_cost）
    if diff < 0: 盘亏出库（biz_type=盘亏）
    更新 inv_inventory、写台账
    生成凭证：
      盘盈：借 库存商品 / 贷 待处理财产损溢
      盘亏：借 待处理财产损溢 / 贷 库存商品
```
审批后解冻。

### 5. 边界
- 盘点期间冻结仓库，禁止出入库；
- 差异金额 = diff * unit_cost；
- 支持按仓库全盘或抽盘（指定物料范围）。

---

## 二、库存调拨

### 1. 目标
仓库间或库位间调拨，支持在途。

### 2. 数据模型
见 [schema.md P2-3](../schema.md#p2-3-调拨)。
- `inv_transfer` / `inv_transfer_item`。

### 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /transfers | 创建调拨单 |
| POST | /transfers/:no/approve | 审批 |
| POST | /transfers/:no/ship | 调出出库（扣调出仓，计入在途） |
| POST | /transfers/:no/receive | 调入入库（入调入仓，冲在途） |
| GET | /transfers | 列表 |

### 4. 关键逻辑

**调出（事务）**：
- 原子扣减调出仓库存；
- 调拨单状态=在途；
- 台账记调拨出库（biz_type=调拨出库）。

**调入（事务）**：
- 原子增加调入仓库存，avg_cost 沿用调出仓的成本（跨仓成本不变）；
- 台账记调拨入库（biz_type=调拨入库）；
- 调拨单状态=已完成，写 received_at。

**同仓库内库位调拨**：不需要在途，一步完成（同时扣减/增加两个库位）。

### 5. 边界
- 调出数量不能超过库存；
- 调入时必须与调拨单明细一致；
- 支持部分调入（多次 receive）。

---

## 三、批次与序列号

### 1. 目标
对需要追溯的物料（食品、医药、电子）实现批次/序列号管理，支持 FEFO 和全链路追溯。

### 2. 数据模型
见 [schema.md P2-4](../schema.md#p2-4-批次序列号)。
- `inv_batch` 批次（生产日期、失效日期、供应商、剩余数量）；
- `inv_serial` 序列号（状态、当前位置）。

### 3. 物料主数据扩展
`mdm_material` 新增：
- `batch_managed BOOLEAN`：是否批次管理；
- `serial_managed BOOLEAN`：是否序列号管理；
- `shelf_life_days INT`：保质期（天）。

### 4. 入库
- 批次管理物料：入库时必须录入批次号、生产日期、失效日期；写 `inv_batch`，`qty_in = qty_remaining = qty`；
- 序列号管理物料：每个序列号一条 `inv_serial` 记录，状态=在库；
- 库存表（`inv_inventory`）按批次维度记录（增加 `batch_id`，唯一约束变为 `(material_id, warehouse_id, location_id, batch_id)`）。

### 5. 出库
- 批次管理：系统按 FEFO（先到期先出）推荐批次，可手工调整；扫码时校验批次归属和剩余数量；原子扣减 `inv_batch.qty_remaining`；
- 序列号：扫码指定序列号，更新 `inv_serial.status=已售`、`outbound_no`、`warehouse_id=NULL`；
- 出库后更新台账（带 batch_id）。

### 6. 追溯查询
```
GET /inventory/batch-trace?batch_no=xxx
```
返回：
- 上游：采购入库单号、供应商、生产日期；
- 中间：被哪个生产工单领用、产出到哪个成品批次；
- 下游：销售出库单号、客户、出库日期。

用 `inv_stock_ledger` 的 biz_type/biz_no 串联整条链。

### 7. 边界条件
- 批次管理物料出库必须指定批次；
- 序列号物料出库数量必须与序列号个数一致；
- 过期批次禁止出库（可配置权限绕过）；
- 批次剩余数量不能为负。

### 8. 测试要点
- FEFO 推荐：同物料两个批次，先出更早到期的；
- 批次追溯链完整；
- 序列号状态机正确；
- 过期批次出库被拦截。
