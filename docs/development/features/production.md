# 生产管理

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
- 状态：10 创建 → 20 已下达 → 30 生产中（领料后）→ 40 已完工 → 50 已关闭。

### 4.3 工单下达
- 齐套检查：每个子件的可用库存 ≥ 应领数量，否则警告（可配置强制）；
- 写 `actual_start_at`，状态→已下达。

### 4.4 生产领料扫码（核心事务）
一个事务内完成：
1. 校验工单状态为已下达/生产中；
2. 校验扫码物料在工单 BOM 中；
3. 原子扣减库存（含 available_qty 守卫）；
4. 原子更新 `issued_qty = issued_qty + ?`，守卫 `issued_qty + ? <= plan_qty`；
5. 写 `prod_production_outbound`；
6. 写台账（biz_type=生产领料）；
7. 生成凭证：借 生产成本-直接材料 / 贷 库存商品（成本用库存移动平均）。

替代料：允许领同组其他物料，但需记录原物料和替代物料。

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
