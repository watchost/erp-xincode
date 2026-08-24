# 生产管理

## 0. 当前实现状态（2026-08-14）

已完成第一批 P0 后端能力：

- `0004_production_schema.up.sql` 已对齐当前 Go 模型：新增 `prod_bom`、`prod_bom_item`、`prod_production_receipt`，修正 `prod_work_order` 的 `work_order_no`、`produced_qty`、计划/实际时间、`created_by` 等字段；
- `POST /production/bom` 可创建启用版本的 BOM，并复制 BOM 明细；
- `POST /production/work-orders` 基于当前启用 BOM 创建工单和工单物料快照，工单号为 `WO<yyyyMMdd>-<seq>`；
- `POST /production/work-orders/:work_order_no/release` 在事务内锁工单、校验草稿状态、写入 `actual_start_at` 并推进到“已下达”；
- `POST /production/material-issue/scan` 在同一事务内锁工单和工单物料行，防超领，调用库存服务扣减库存和台账，再累加 `issued_qty`，首次领料自动推进到“生产中”；
- 生产领料单号使用 `OB<yyyyMMdd>-<seq>`，并强制 `Idempotency-Key`。

当前状态常量为：1=草稿、2=已下达、3=生产中、4=已完成。后续若要统一审计文档中的 10/20/30 编码，需要同时调整 Go 常量、数据库默认值和前端字典。

尚未完成：齐套检查、替代料、生产入库/报工、完工关闭、实际成本归集、成本卷积、多层树形 BOM 接口、凭证生成。

## 1. 目标
完整的生产管理：BOM 树形结构、工单全生命周期、按 BOM 领料、生产入库报工、成本卷积。

## 2. 数据模型
见 [schema.md P0-1](../schema.md#p0-1-修正生产模型与建表)。

- `prod_bom` BOM 主表（版本、启用状态）；
- `prod_bom_item` BOM 明细（树形、损耗率、替代料）；
- `prod_work_order` 工单（补齐字段）；
- `prod_work_order_bom` 工单 BOM 快照；
- `prod_production_outbound` 生产领料出库；
- `prod_production_receipt` 生产入库（新增）。

## 3. 接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /boms | BOM CRUD |
| GET | /boms/:id/tree | BOM 树形展开 |
| POST | /boms/:id/activate | 启用 BOM |
| GET/POST | /work-orders | 工单 CRUD |
| POST | /work-orders/:no/release | 下达工单 |
| POST | /work-orders/:no/material-issue/scan | 生产领料扫码 |
| POST | /work-orders/:no/receipt | 生产入库（报工） |
| POST | /work-orders/:no/close | 完工关闭 |
| GET | /work-orders/:no/cost | 工单实际成本 |

## 4. 关键逻辑

### 4.1 BOM 树形结构
- `prod_bom_item.parent_id` 自引用，支持多层；
- `qty` 是生产 1 个父件所需的子件数量；
- `loss_rate` 损耗率，实际领料量 = qty × 工单数量 × (1+loss_rate)；
- 替代品：`is_substitute=true` + `substitute_group`，同组内任选其一；
- 树形展开：递归查询或预计算 `path`。

### 4.2 工单创建
- 选择产品和 BOM，复制 BOM 到 `prod_work_order_bom`（快照，BOM 后改不影响已建工单）；
- 计算应领数量 `plan_qty = bom.qty * plan_qty * (1+loss_rate)`；
- 当前实现状态：1 草稿 → 2 已下达 → 3 生产中（首次领料后）→ 4 已完成；目标状态码是否改为 10/20/30/40/50 需在 P1 统一。

### 4.3 工单下达
- 当前实现：锁工单，校验草稿状态，写 `actual_start_at`，状态→已下达；
- 待补齐套检查：每个子件的可用库存 ≥ 应领数量，否则警告（可配置强制）。

### 4.4 生产领料扫码（核心事务）
当前已在一个数据库事务内完成：
1. 锁定工单并校验状态为已下达/生产中；
2. 校验扫码物料在工单物料快照中；
3. 锁定工单物料行并校验 `issued_qty + req_qty <= plan_qty`；
4. 调用库存服务在同事务内出库，库存服务内部锁定库存行并校验 `available_qty`；
5. 写库存台账（`biz_type=生产领料`）；
6. 累加 `issued_qty`；
7. 首次领料时把工单从“已下达”推进到“生产中”。

待补：写独立的 `prod_production_outbound` 领料单表、生成凭证（借 生产成本-直接材料 / 贷 库存商品）、替代料记录。

### 4.5 生产入库（报工，新增）
```
POST /production/receipt
{ work_order_no, qty, warehouse_id, location_id }
```
事务：
1. 校验工单状态；
2. 库存增加（成品入库），成本 = 工单实际成本 / 已入库数量（移动平均或按批）；
3. 写 `prod_production_receipt`；
4. 更新 `produced_qty`，达到 `plan_qty` 时状态→已完工；
5. 写台账（biz_type=生产入库）；
6. 凭证：借 库存商品 / 贷 生产成本-直接材料/人工/制造费用。

### 4.6 成本卷积
- **标准成本**：按 BOM 递归，标准成本 = Σ(子件标准成本 × 用量) + 人工 + 制造费用；
- **实际成本**：工单归集实际领料成本（移动平均出库成本）+ 人工工时 + 制费分摊；
- 入库时按实际成本结转；
- 差异分析：标准 vs 实际。

## 5. 边界条件
- 领料不能超应领（`issued_qty + ? <= plan_qty`）；
- 入库不能超计划数量（`produced_qty + ? <= plan_qty`，可配置允许超产）；
- 工单完工后不能再领料/入库；
- BOM 启用后不能修改，只能新建版本。

## 6. 测试要点
- 多层 BOM 递归展开数量正确（含损耗率）；
- 并发领料不超领；
- 领料事务中凭证写入失败时库存回滚；
- 生产成本归集与结转金额正确；
- 工单完工状态自动流转。
