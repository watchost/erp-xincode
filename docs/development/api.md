# API 总览

> 本文汇总 P0-P3 新增/修改的接口。所有接口前缀 `/api/v1`，OpenAPI 第三方接口前缀 `/openapi/v1`。
> 所有写接口需带 `Idempotency-Key` header；所有受保护接口需 `Authorization: Bearer <token>`。

## 认证 `/auth`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| POST | /auth/login | P0 | 登录 |
| POST | /auth/logout | P1 | 登出（拉黑 jti） |
| POST | /auth/refresh | P0 | 刷新 token |
| POST | /auth/change-password | P1 | 改密 |

## IAM `/users` `/roles` `/permissions` `/departments` `/audit-logs`

详见 [features/iam.md](features/iam.md#3-接口)。

## 主数据 `/mdm`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET/POST | /mdm/materials | P0/P1 | 物料（补 min_stock、批次标志、单位换算） |
| GET/PUT/DELETE | /mdm/materials/:id | P0/P1 | |
| GET/POST | /mdm/material-categories | P1 | 物料分类树（新增） |
| GET/POST | /mdm/suppliers | P0/P1 | |
| GET/PUT/DELETE | /mdm/suppliers/:id | P1 | Delete 由空实现改为真实软删 |
| GET/POST | /mdm/warehouses | P1 | Create 由空实现改为真实写入 |
| GET/POST | /mdm/locations | P1 | 同上 |
| GET/POST | /mdm/customers | P2 | 客户主数据（新增） |
| GET | /mdm/accounts | P3 | 结算账户 |

## 仓库 `/warehouse`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| POST | /warehouse/inbound/scan | P0 | 入库扫码（修成本、事务、幂等、传仓库/库位） |
| POST | /warehouse/outbound/scan | P0 | 出库扫码（修锁、库位必填） |
| GET | /warehouse/inventory | P0 | 库存列表 |
| GET | /warehouse/stock-alerts | P0 | 库存预警 |
| GET/POST | /warehouse/stock-checks | P2 | 盘点单 |
| POST | /warehouse/stock-checks/:no/freeze | P2 | 冻结 |
| POST | /warehouse/stock-checks/:no/approve | P2 | 审批调账 |
| GET/POST | /warehouse/transfers | P2 | 调拨 |
| POST | /warehouse/transfers/:no/ship | P2 | 调出 |
| POST | /warehouse/transfers/:no/receive | P2 | 调入 |
| GET | /warehouse/batches | P2 | 批次查询 |
| GET | /warehouse/serials | P2 | 序列号查询 |
| GET | /warehouse/batch-trace | P2 | 批次追溯 |

## 采购 `/purchase`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET/POST | /purchase/orders | P0 | 采购订单 |
| GET | /purchase/orders/:no | P0 | 详情 |
| PUT | /purchase/orders/:no | P1 | 修改草稿 |
| POST | /purchase/orders/:no/approve | P0 | 审批（写 approved_at） |
| POST | /purchase/orders/:no/cancel | P1 | 取消 |
| POST | /purchase/inbound/scan | P0 | 采购入库（合并事务） |
| GET/POST | /purchase/returns | P2 | 采购退货 |
| GET | /purchase/inbound | P1 | 入库单列表 |

## 生产 `/production`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET/POST | /production/boms | P0/P1 | BOM（修表结构） |
| GET | /production/boms/:id/tree | P1 | BOM 树展开 |
| GET/POST | /production/work-orders | P0 | 工单（修表结构） |
| POST | /production/work-orders/:no/release | P0 | 下达（齐套检查） |
| POST | /production/work-orders/:no/material-issue/scan | P0 | 领料（修锁/超领） |
| POST | /production/work-orders/:no/receipt | P1 | 生产入库（新增） |
| POST | /production/work-orders/:no/close | P1 | 完工关闭 |
| GET | /production/work-orders/:no/cost | P1 | 工单成本 |

## 销售 `/sales`（P2 新增）

详见 [features/sales.md](features/sales.md#3-接口)。

## 财务 `/finance`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET/POST | /finance/accounts | P1 | 会计科目 |
| GET/POST | /finance/vouchers | P1 | 凭证 |
| POST | /finance/vouchers/:no/post | P1 | 过账 |
| GET/POST | /finance/budgets | P0/P1 | 预算（修表+校验） |
| GET | /finance/cost-cards | P0 | 成本卡（修表） |
| GET | /finance/account-entries | P0 | 凭证分录（修表） |
| GET | /finance/receivables | P2 | 应收 |
| GET | /finance/payables | P2 | 应付 |
| GET/POST | /finance/receipts | P2 | 收款 |
| GET/POST | /finance/payments | P2 | 付款 |
| POST | /finance/settlements | P2 | 核销 |
| GET | /finance/reports/balance-sheet | P1 | 资产负债表 |
| GET | /finance/reports/income-statement | P1 | 利润表 |
| GET | /finance/reports/cash-flow | P1 | 现金流量表 |
| GET | /finance/reports/aging | P2 | 账龄分析 |
| POST | /finance/period/close | P1 | 期末结转 |

## 仪表 `/dashboard`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET | /dashboard/overview | P0 | 真实 KPI（删 mock） |
| GET | /dashboard/stock-alerts | P1 | 真实预警 |
| GET | /dashboard/recent-orders | P1 | 真实最近订单 |
| POST | /dashboard/llm/analysis | P1 | 真实 LLM 分析 |

## 设备 `/device` 与 WebSocket

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| GET/POST | /devices | P1 | 设备 CRUD |
| GET | /devices/:code | P1 | |
| POST | /devices/:code/command | P1 | 下发指令 |
| GET | /devices/:code/messages | P1 | 消息流水 |
| WS | /ws (8081) | P1 | 设备 WebSocket |

## LLM `/llm`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| POST | /llm/chat | P1 | 对话（流式） |
| GET | /llm/sessions | P1 | 我的会话 |
| DELETE | /llm/sessions/:id | P1 | 删除（校验归属） |
| GET | /llm/sessions/:id/history | P1 | 历史（校验归属） |

## OpenAPI `/openapi/v1`

| 方法 | 路径 | 阶段 | 说明 |
|---|---|---|---|
| POST | /oauth/token | P1 | OAuth2 取 token |
| GET | /materials 等 | P1 | 资源接口 |
| POST | /webhooks | P1 | 订阅 |
| POST | /sync | P1 | 数据同步 |

## 工作流 `/workflow`（P3）

详见 [features/workflow.md](features/workflow.md#4-接口)。

## 消息 `/messages`（P3）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /messages | 我的消息 |
| GET | /messages/unread-count | 未读数 |
| POST | /messages/:id/read | 标记已读 |
| POST | /messages/read-all | 全部已读 |

## 文件 `/attachments`（P3）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /attachments/upload | 上传 |
| GET | /attachments/:id/download | 下载 |
| GET | /attachments?biz_type=&biz_no= | 业务附件列表 |
