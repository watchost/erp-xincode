# API 总览

> 本文按当前代码和后续规划汇总接口。所有内部接口前缀 `/api/v1`，OpenAPI 第三方接口前缀 `/openapi/v1`。
> 所有受保护接口都需要 `Authorization: Bearer <token>`。当前采购入库、生产领料等扫码写接口必须带 `Idempotency-Key`；仓库扫码接口已支持该头，后续会统一强制。

## 1. 认证 `/auth` 与当前用户

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /login | P0 已完成 | 公开 | 登录，返回 access/refresh token |
| POST | /refresh | P0 已完成 | 公开 | refresh token 轮换 |
| POST | /auth/logout | P0 已完成 | 登录 | 拉黑当前 access token 的 jti |
| POST | /auth/change-password | P0 已完成 | 登录 | 修改自己的密码 |
| GET | /users/profile | P0 已完成 | 登录 | 当前用户信息 |
| GET | /users/permissions | P0 已完成 | 登录 | 当前用户权限码 |

说明：审计阶段文档里的 `/auth/login` 当前实际挂载为 `/api/v1/login`；后续如需统一前缀，可在兼容期内同时保留两个路径。

## 2. IAM `/iam`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| GET | /iam/users | P0 已完成 | `iam:user` | 用户列表 |
| GET | /iam/users/:id | P0 已完成 | `iam:user` | 用户详情 |
| POST | /iam/users | P0 已完成 | `iam:user` | 创建用户，DTO 包含明文 password 与 role_ids |
| PUT | /iam/users/:id | P0 已完成 | `iam:user` | 白名单字段更新并替换角色 |
| GET | /roles | P1 | `iam:role` | 角色列表（待补） |
| POST | /roles | P1 | `iam:role` | 创建角色（待补） |
| PUT | /roles/:id/permissions | P1 | `iam:role` | 分配权限（待补） |
| GET | /audit-logs | P3 | `iam:audit:view` | 审计日志查询（待补） |

`0003_iam_permissions.up.sql` 已补充 `dashboard:llm`、`finance:budget`、`mdm:location`、`device:manage`、`llm:chat`、`openapi:admin` 等权限码，并授予管理员角色。

## 3. 主数据 `/mdm`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| GET/POST | /mdm/materials | P0/P1 | `mdm:material` | 物料查询/创建 |
| GET/PUT | /mdm/materials/:id | P0/P1 | `mdm:material` | 物料详情/更新 |
| GET/POST | /mdm/suppliers | P0/P1 | `mdm:supplier` | 供应商查询/创建 |
| GET/PUT | /mdm/suppliers/:id | P0/P1 | `mdm:supplier` | 供应商详情/更新 |
| GET/POST | /mdm/warehouses | P0/P1 | `mdm:warehouse` | 仓库查询/创建 |
| GET/POST | /mdm/locations | P0/P1 | `mdm:location` | 库位查询/创建 |
| GET/POST | /mdm/material-categories | P1 | `mdm:material` | 物料分类树（待补） |
| DELETE | /mdm/materials/:id | P1 | `mdm:material` | 软删除（待补） |
| GET/POST | /mdm/customers | P2 | `mdm:customer` | 客户主数据（待补） |

## 4. 仓库 `/warehouse`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /warehouse/inbound/scan | P0 已完成 | `warehouse:inbound` | 入库扫码，必填 `material_code`、`warehouse_code`、`location_code`、`qty`，可传 `unit_cost` |
| POST | /warehouse/outbound/scan | P0 已完成 | `warehouse:outbound` | 出库扫码，必填物料、仓库、库位、数量；库存不足返回 409 |
| GET | /warehouse/inventory | P0 已完成 | `warehouse:inventory` | 库存列表 |
| GET | /warehouse/stock-alerts | P0 已完成 | `warehouse:inventory` | 库存预警 |
| GET/POST | /warehouse/stock-checks | P2 | `warehouse:stock-check` | 盘点单 |
| POST | /warehouse/stock-checks/:no/freeze | P2 | `warehouse:stock-check` | 冻结 |
| POST | /warehouse/stock-checks/:no/approve | P2 | `warehouse:stock-check` | 审批调账 |
| GET/POST | /warehouse/transfers | P2 | `warehouse:transfer` | 调拨 |
| POST | /warehouse/transfers/:no/ship | P2 | `warehouse:transfer` | 调出 |
| POST | /warehouse/transfers/:no/receive | P2 | `warehouse:transfer` | 调入 |
| GET | /warehouse/batches | P2 | `warehouse:batch` | 批次查询 |
| GET | /warehouse/serials | P2 | `warehouse:serial` | 序列号查询 |
| GET | /warehouse/batch-trace | P2 | `warehouse:batch` | 批次追溯 |

### 4.1 扫码与幂等

采购入库和生产领料强制要求：

```http
Idempotency-Key: <uuid>
```

重复提交返回业务码 `10200` / HTTP 429。当前实现会拒绝重复请求，后续可扩展为缓存并回放首次响应。仓库普通扫码接口已读取该头，建议前端所有写接口统一生成 UUID。

### 4.2 入库成本

入库移动加权平均公式：

```text
new_avg = (old_avg * old_qty + unit_cost * in_qty) / (old_qty + in_qty)
```

采购入库自动从采购订单明细单价带入 `unit_cost`；其他入库由请求传入。未知单价时保持原平均成本，不再把成本重置为 0。

## 5. 采购 `/purchase`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| GET/POST | /purchase/orders | P0 已完成 | `purchase:order:view/create` | 采购订单查询/创建 |
| POST | /purchase/orders/:order_no/approve | P0 已完成 | `purchase:order:approve` | 审批订单，写 `approved_at` |
| POST | /purchase/inbound/scan | P0 已完成 | `purchase:inbound` | 采购入库扫码，必须带 `Idempotency-Key`、仓库、库位 |
| PUT | /purchase/orders/:order_no | P1 | `purchase:order:update` | 修改草稿（待补） |
| POST | /purchase/orders/:order_no/cancel | P1 | `purchase:order:approve` | 取消（待补） |
| GET | /purchase/inbound | P1 | `purchase:inbound` | 入库单列表（待补） |
| GET/POST | /purchase/returns | P2 | `purchase:return` | 采购退货（待补） |

采购入库在同一数据库事务内完成：锁订单 → 校验已审批 → 锁订单明细 → 防超收 → 库存入库与台账 → 累加 `received_qty` → 写采购入库单 → 全部收满时自动完成订单。

## 6. 生产 `/production`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /production/bom | P0 已完成 | `production:wo:create` | 创建 BOM 版本 |
| GET/POST | /production/work-orders | P0 已完成 | `production:wo:view/create` | 工单查询/创建 |
| POST | /production/work-orders/:work_order_no/release | P0 已完成 | `production:wo:create` | 下达工单，写 `actual_start_at` |
| POST | /production/material-issue/scan | P0 已完成 | `production:outbound` | 生产领料扫码，必须带 `Idempotency-Key`、仓库、库位 |
| GET | /production/boms/:id/tree | P1 | `production:wo:view` | BOM 树展开（待补） |
| POST | /production/work-orders/:work_order_no/receipt | P1 | `production:receipt` | 生产入库/报工（待补） |
| POST | /production/work-orders/:work_order_no/close | P1 | `production:wo:close` | 完工关闭（待补） |
| GET | /production/work-orders/:work_order_no/cost | P1 | `production:cost` | 工单成本（待补） |

生产领料在同一数据库事务内完成：锁工单 → 校验已下达/生产中 → 匹配工单物料 → 锁物料行 → 防超领 → 库存出库与台账 → 累加 `issued_qty` → 首次领料把工单从“已下达”推进到“生产中”。

## 7. 财务 `/finance`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| GET | /finance/cost-cards | P0 现有，P1 重构 | `finance:cost` | 成本卡查询 |
| GET | /finance/cost-summary | P0 现有，P1 重构 | `finance:cost` | 成本汇总 |
| GET | /finance/account-entries | P0 现有，P1 重构 | `finance:reports` | 分录查询 |
| GET | /finance/financial-report | P0 现有，P1 重构 | `finance:reports` | 财务报表 |
| GET | /finance/budgets | P0 现有，P1 重构 | `finance:budget` | 预算查询 |
| GET/POST | /finance/accounts | P1 | `finance:account` | 会计科目 |
| GET/POST | /finance/vouchers | P1 | `finance:voucher` | 凭证 |
| POST | /finance/vouchers/:no/post | P1 | `finance:voucher:post` | 过账 |
| GET | /finance/receivables | P2 | `finance:receivable` | 应收 |
| GET | /finance/payables | P2 | `finance:payable` | 应付 |
| GET/POST | /finance/receipts | P2 | `finance:receipt` | 收款 |
| GET/POST | /finance/payments | P2 | `finance:payment` | 付款 |
| POST | /finance/settlements | P2 | `finance:settle` | 核销 |
| GET | /finance/reports/balance-sheet | P1 | `finance:reports` | 资产负债表 |
| GET | /finance/reports/income-statement | P1 | `finance:reports` | 利润表 |
| GET | /finance/reports/cash-flow | P1 | `finance:reports` | 现金流量表 |
| GET | /finance/reports/aging | P2 | `finance:reports` | 账龄分析 |
| POST | /finance/period/close | P1 | `finance:period:close` | 期末结转 |

财务最小闭环尚未完成：采购入库/生产领料当前不生成凭证，预算未在下单时扣减，报表取数仍需按会计准则重写。

## 8. 仪表 `/dashboard`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| GET | /dashboard/overview | P0 现有 | `dashboard:view` | KPI 概览 |
| GET | /dashboard/stock-alerts | P0 现有 | `dashboard:view` | 库存预警 |
| GET | /dashboard/recent-orders | P0 现有 | `dashboard:view` | 最近订单 |
| POST | /dashboard/llm/analysis | P1 | `dashboard:llm` | LLM 分析（待接真实数据和 key） |

## 9. 设备 `/device` 与 WebSocket

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /device/register | P1 | `device:manage` | 设备注册 |
| POST | /device/heartbeat | P1 | `device:manage` | 心跳 |
| GET | /device/list | P1 | `device:manage` | 设备列表 |
| GET | /device/:device_code | P1 | `device:manage` | 设备详情 |
| POST | /devices/:code/command | P1 | `device:manage` | 下发指令（待补） |
| GET | /devices/:code/messages | P1 | `device:manage` | 消息流水（待补） |
| WS | /ws (8081) | P1 | 设备鉴权 | 真实 WebSocket（待补） |

## 10. LLM `/llm`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /llm/chat | P1 | `llm:chat` | 对话；已去掉 user_id=1 后门，需真实登录态 |
| GET | /llm/sessions | P1 | `llm:chat` | 我的会话 |
| GET | /llm/sessions/:session_id/history | P1 | `llm:chat` | 历史（仍需补归属校验） |
| DELETE | /llm/sessions/:id | P1 | `llm:chat` | 删除会话（待补） |

## 11. OpenAPI `/openapi`

| 方法 | 路径 | 阶段 | 权限 | 说明 |
|---|---|---|---|---|
| POST | /oauth/token | P1 | 公开 | OAuth2 取 token |
| POST | /oauth/refresh | P1 | 公开 | 刷新 token |
| POST | /openapi/webhooks | P1 | `openapi:admin` | 订阅 webhook |
| POST | /openapi/sync | P1 | `openapi:admin` | 数据同步 |
| GET | /openapi/clients | P1 | `openapi:admin` | 客户端列表 |

OpenAPI 独立 Bearer token 校验、scope、client_secret 哈希、SSRF 防护仍待补。

## 12. 后续模块

- 销售 `/sales`：P2，详见 [features/sales.md](features/sales.md#3-接口)。
- 审批工作流 `/workflow`：P3，详见 [features/workflow.md](features/workflow.md#4-接口)。
- 消息 `/messages`：P3，支持未读数、单条已读、全部已读。
- 附件 `/attachments`：P3，支持上传、下载和业务关联查询。
