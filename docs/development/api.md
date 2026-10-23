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

## 12. 审批工作流 `/workflow`（P3 §4.1，2026-10-23 后端已实现）

> 前端任务 6（流程审批、待办、通知铃铛）以本节为准。统一响应 `{code, data, message}`；列表接口返回分页结构 `{list, total, page, page_size}`（与现有 `response.Page` 一致）。

### 12.1 接口

| 方法 | 路径 | 权限 | 请求 | 说明 |
|---|---|---|---|---|
| GET | /workflow/definitions?code= | `workflow:definition` | — | 流程定义列表（按 code、版本倒序） |
| POST | /workflow/definitions | `workflow:definition` | `{code, name, definition}` | 新建定义，自动生成下一版本号，状态=草稿(0)；定义 JSON 会被校验 |
| POST | /workflow/definitions/:id/deploy | `workflow:definition` | — | 部署该版本（状态=1），同 code 其他已部署版本自动停用(2)；运行中的实例继续按原版本流转 |
| POST | /workflow/instances/start | `workflow:task` | `{biz_type, biz_no, variables?}` | 通用启动；返回 `{instance_no, status, current_node}`。采购订单请用 `/purchase/orders/:order_no/submit` |
| GET | /workflow/instances/by-biz?biz_type=&biz_no= | `workflow:task` | — | 单据最近一次实例号 `{instance_no}`（没有时为空串），用于单据详情页跳转审批历史 |
| GET | /workflow/instances/:no | `workflow:task` | — | 审批历史：`{instance, tasks[], approvals[]}` |
| POST | /workflow/instances/:no/cancel | `workflow:task` | `{comment?}` | 发起人撤销（非发起人返回 80011） |
| GET | /workflow/tasks/todo?page=&page_size= | `workflow:task` | — | 我的待办（仅运行中实例的待办） |
| GET | /workflow/tasks/done?page=&page_size= | `workflow:task` | — | 我的已办（已通过/已转办/已驳回） |
| POST | /workflow/tasks/:id/approve | `workflow:task` | `{comment?}` | 通过 |
| POST | /workflow/tasks/:id/reject | `workflow:task` | `{comment}` | 驳回，意见必填（80008） |
| POST | /workflow/tasks/:id/transfer | `workflow:task` | `{target_user_id, comment?}` | 转办 |
| POST | /workflow/tasks/:id/add-sign | `workflow:task` | `{target_user_id, comment?}` | 加签（加签后本节点变为会签） |
| POST | /purchase/orders/:order_no/submit | `purchase:order:submit` | — | 采购草稿单提交审批：订单 10→15（审批中），同事务启动流程，返回 `{instance_no}` |

待办列表项字段（`TaskVO`）：`task_id, instance_no, biz_type, biz_no, node_code, node_name, sign_mode, status, starter_id, instance_status, created_at, finished_at`。

### 12.2 状态与错误码

- 实例 `status`：1 审批中，2 通过，3 驳回，4 撤销。
- 任务 `status`：1 待办，2 已通过，3 已转办，4 已驳回，5 已取消（或签他人已处理/流程结束）。
- 采购订单新增状态 **15 审批中**；驳回或撤销退回 10 草稿；全部通过后在同一事务执行原审批逻辑（占用预算）→ 20 已审批。
- 已部署 `purchase_order` 流程时，`POST /purchase/orders/:order_no/approve` 直接审批返回 **40010**。
- 错误码：80001 定义不存在｜80002 无已部署定义｜80003 实例不存在｜80004 任务不存在｜80005 非审批人｜80006 任务已处理｜80007 单据已有运行中流程｜80008 驳回意见必填｜80009 找不到可用审批人｜80010 定义不合法｜80011 非发起人撤销｜80012 转办/加签目标无效。

### 12.3 定义 JSON 与审批人规则

当前版本支持 `start` / `approval` / `end` 三种节点，按数组顺序流转；审批节点可带 `condition`（`field` 取启动变量，如采购单的 `amount`；不满足则跳过本节点；缺少变量时不跳过）。`cc`、`parallel`、独立 `condition` 分支节点**尚未实现**，定义中出现会被拒绝。

| `assignee.type` | `value` | 解析口径 |
|---|---|---|
| `user` | 用户 ID 或 ID 数组 | 指定人 |
| `role` | 角色 code | 该角色全部用户 |
| `dept_leader` | — | 发起人所在部门负责人；发起人本人是负责人时向上取上级部门负责人 |
| `initiator_leader` | — | 发起人上级。`sys_user` 没有直属上级字段，按部门负责人口径实现（同 `dept_leader`） |
| `dept_leader_up` | — | 上级部门负责人 |

只会分配给存在、启用、未删除的用户；解析结果为空时启动或流转失败（80009）并整体回滚，不会静默跳过节点。

默认部署：`purchase_order` v1 = 部门负责人（或签）→ 管理层角色 `manager`（或签）。

## 13. 多租户、审计日志、站内消息（P3 §4.2–§4.4，2026-10-23 后端已实现）

> 前端任务 6（通知铃铛、消息中心）以 §13.3 为准。迁移：`0022_tenant_audit_message`。

### 13.1 多租户 `/admin/tenants`、`/tenant/current`

| 方法 | 路径 | 权限 | 请求 | 说明 |
|---|---|---|---|---|
| GET | /tenant/current | 登录即可 | — | 当前租户 `{id, code, name, status, expire_at, config}` |
| PUT | /tenant/current | `tenant:config` | `{name?, config?}` | 租户管理员改本租户名称/配置（不能改状态、过期时间） |
| GET | /admin/tenants | `platform:tenant` | — | 租户列表 |
| POST | /admin/tenants | `platform:tenant` | `{code, name, expire_at?, config?, admin_username, admin_password}` | 新建租户：复制默认租户的科目表（parent_id 按编码重映射）与已部署流程定义，并创建该租户的 admin 用户 |
| PUT | /admin/tenants/:id | `platform:tenant` | `{name?, expire_at?, config?}` | 修改租户 |
| POST | /admin/tenants/:id/disable · /enable | `platform:tenant` | — | 停用/启用（默认租户 0 不能停用） |
| GET | /admin/tenants/:id/stats | `platform:tenant` | — | `{users, materials, purchase_orders, sales_orders}` |

- `platform:tenant` 接口额外要求调用者属于默认租户 0（`sys_role` 是全局表，其他租户的 admin 也有角色 1，所以单靠权限码不够），否则 81004。
- `config` 为自由 JSON，约定键：`logo`、`currency`、`period_rule`、`plan`。
- 停用租户：所有接口 403（81002），且禁止登录/刷新 token；过期租户：GET 放行，写操作 403（81003），禁止登录。
- 用户名保持**全局唯一**（登录不带租户），新建租户时管理员用户名冲突返回 81006。
- 错误码：81001 租户不存在｜81002 已停用｜81003 已过期｜81004 仅平台可操作｜81005 编码重复｜81006 用户名已被使用｜81007 默认租户不能停用。

### 13.2 审计日志 `GET /iam/audit-logs`

权限 `iam:audit:view`。新增查询参数 `trace_id`（按一次请求追溯）；结果只包含当前租户的记录。
审计中间件对 `/api/v1` 下全部 POST/PUT/PATCH/DELETE 异步落库，记录用户、租户、IP、UA、模块、动作、状态码、耗时、trace_id、参数摘要：

- JSON 请求体（≤4KB）递归脱敏后保存，并合并路径参数到 `_path_params`；
- 非 JSON / 超过 4KB / 空体：保存摘要 `{_body: not_json|too_large_or_chunked|empty|unparsable, _content_type, _size, _path_params, _query}`，保证每次写操作都有可定位到单据的记录；
- 脱敏键（大小写不敏感，任意层级）：password、old_password、new_password、admin_password、password_hash、token、access_token、refresh_token、captcha_code、secret、client_secret、api_key、apikey、authorization、private_key、bank_account、card_no、id_card。
- 限制：写库是旁路异步，进程被强杀时可能丢失最后几条；登录/刷新在 JWT 组外，由 IAM service 自行写审计（原有行为）。

### 13.3 站内消息 `/messages`（权限 `message:view`，所有角色已授予）

| 方法 | 路径 | 请求 | 说明 |
|---|---|---|---|
| GET | /messages?category=&unread=1&page=&page_size= | — | 我的消息，按时间倒序，分页结构同 `response.Page` |
| GET | /messages/unread-count | — | `{total, by_category: {approval_todo: n, stock_low: n, ...}}`（通知铃铛） |
| POST | /messages/:id/read | — | 单条已读（不是自己的或其他租户的消息返回 82001） |
| POST | /messages/read-all | — | 全部已读，返回 `{updated}` |
| GET | /messages/subscriptions | — | 我的外部渠道订阅 |
| PUT | /messages/subscriptions | `{category, channel, target, enabled}` | 按 (类别, 渠道) 新增或更新订阅 |

消息字段：`id, category, title, content, biz_type, biz_no, link, is_read, read_at, created_at`。`link` 是前端跳转路径（审批待办为 `/workflow/instances/:no`，库存预警为 `/warehouse/inventory?material_id=`）。

**类别与触发事件：**

| category | 触发 | 接收人 |
|---|---|---|
| `approval_todo` | 审批流新建待办：提交、进入下一级、转办、加签 | 新待办的审批人 |
| `stock_low` | 出库后物料跨仓总库存从 ≥ 安全库存跌破到 < 安全库存（只在跌破那一次提醒） | 本租户 warehouse、purchase 角色用户；都没有时发给 admin |
| `system` | 预留 | — |

- 安全库存取物料已有字段 `mdm_material.min_stock`，为空不预警。
- 站内信与业务在同一事务写入：业务回滚，消息一起回滚。

**外部渠道**（`channel`）：`email`、`dingtalk`、`wecom`、`webhook`。订阅了外部渠道的消息由后台派发器（每 10 秒，`FOR UPDATE SKIP LOCKED`）在事务提交后发送，结果写入 `channel_result`：

- `webhook` / `dingtalk` / `wecom`：向订阅的 https 地址 POST JSON（钉钉/企业微信为群机器人 text 消息格式），超时 5 秒；**未实现钉钉加签**；未在真实钉钉/企业微信环境验证；
- `email`：**未接入 SMTP**，始终记为 `skipped: 未配置 SMTP 发信服务`，不会伪造发送成功；
- 订阅地址必须是 `https://`，邮箱必须含 `@`，否则 82004。
- 错误码：82001 消息不存在｜82002 未知类别｜82003 不支持的渠道｜82004 订阅目标不合法。

### 13.4 默认采购审批流的前置条件（任务 2 遗留核实）

- `manager` 角色在迁移 `0001_init` 中已存在（`('manager', '管理层', 1)`），可以通过 `PUT /iam/users/:id/roles` 分配给用户；**但种子数据里没有任何用户拥有该角色**。
- 第一级"部门负责人"要求发起人所在部门（或上级部门）设置了 `leader_id`。
- 两者任一不满足，提交采购审批会返回 80009（找不到审批人）并整体回滚，订单保持草稿。部署后需先：给至少一名用户分配 `manager` 角色，并为部门设置负责人。没有为此添加虚构的种子用户。
- 新建租户会复制该流程定义，新租户同样需要完成上述配置。

### 13.5 租户隔离的实现与已知缺口（如实记录）

**已实现：**
- `sys_tenant`；29 张业务表补 `tenant_id`（现有数据归属租户 0）；业务编码/单号唯一约束改为按租户唯一（索引名不变）。
- GORM 插件 `internal/pkg/tenant`：context 中带租户时，对含 `tenant_id` 的表自动追加过滤（Query/Row/Update/Delete，支持表别名），创建时强制写入上下文租户（忽略调用方传入值）；表有 `tenant_id` 但模型缺 `TenantID` 字段时，非 0 租户写入直接报错（fail-closed）。
- JWT → context 已带 `tenant_id`；`TenantGuard` 中间件；登录/刷新检查租户状态。
- 审批流（定义/实例/审批人解析）、审计查询、站内消息全部显式按租户过滤，不依赖插件。

**未达成（§4.8"两个租户数据完全隔离"目前不满足）：**
- 插件依赖语句的 context。现有 repository 有约 **281 个方法不接收 ctx**，直接用 `r.db` 查询（统计于 2026-10-23），这些查询**不会被过滤**。因此：事务内（`TxManager.WithTx` 会传 ctx）的读写、以及新写的 `db.WithContext(ctx)` 查询是隔离的；**大部分列表/详情读接口仍可跨租户读取**。测试 `TestTenantIsolation_KnownGap_RepoWithoutCtx` 专门断言这个缺口存在。
- `db.Raw` / `db.Exec` 手写 SQL 不会被插件改写，需要手工带 `tenant_id`。
- 在完成下面的改造之前，**不要在生产环境开通第二个租户**。

**补齐方案（建议作为独立任务）：** repository 方法统一增加 `ctx context.Context` 首参并使用 `r.db.WithContext(ctx)`，按模块分批（mdm → warehouse → purchase/sales → production → finance → 其余），每批补一组跨租户读取的集成测试；全部完成后把上面的缺口测试改为断言查不到。

## 14. 后续模块

- 销售 `/sales`：P2，详见 [features/sales.md](features/sales.md#3-接口)。
- 消息 `/messages`：P3，支持未读数、单条已读、全部已读。
- 附件 `/attachments`：P3，支持上传、下载和业务关联查询。
