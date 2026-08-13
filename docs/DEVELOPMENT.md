# 后续开发文档（Roadmap）

> 版本：v1.0-draft
> 日期：2026-08-13
> 作者：zhouhouping
> 适用仓库：`watchost/erp-xincode`
> 关联：[代码审计报告](AUDIT.md)、[架构约定](development/00-conventions.md)

本文档定义 erp-xincode 从当前脚手架状态到可上线 ERP 系统的完整开发计划，覆盖：审计整改、半成品补全、新业务模块、平台能力、集成与运维。所有功能设计遵循 [development/00-conventions.md](development/00-conventions.md) 中的编码、API、数据库、事务与安全约定。

---

## 0. 当前状态判断

经 [AUDIT.md](AUDIT.md) 审计，系统存在 8 项 P0 致命问题，生产/财务模块因模型与迁移不一致**调用即 SQL 报错**，库存成本核算确定性错误，认证/权限为空壳。当前代码**不能投入真实业务数据**。

因此后续开发**严格按顺序分四个阶段**，前一阶段未验收不得进入下一阶段：

| 阶段 | 主题 | 目标 | 预估 |
|---|---|---|---|
| **P0 整改** | 止血 | 修复全部 P0/P1，系统能安全跑通采购→入库→库存→出库主链路 | 2 周 |
| **P1 补全** | 让现有模块完整 | 补完半成品（设备/LLM/MDM/生产/财务/OpenAPI/IAM），实现 ERP 内部闭环 | 4 周 |
| **P2 扩展** | 核心业务模块 | 销售、盘点、调拨、批次/序列号、应收应付、BOM 卷积、导入导出、打印 | 6 周 |
| **P3 平台化** | 企业级能力 | 审批流、多租户、数据权限、消息通知、监控、CI/CD、移动端 | 6 周 |

总计约 **18 周（4.5 个月）** 达到可上线的中型制造/贸易 ERP 标准。每个阶段结束需通过：集成测试全绿、`govulncheck`/lint 无高危、手动验收清单。

---

## 1. P0 整改阶段（2 周）

目标：**系统能安全地放数据**。不做新功能，只修审计报告中的致命/高危问题。

### 1.1 认证与权限（阻断性，第 1 周）

依据 [AUDIT P0-1/P0-2/P1-1~P1-6](AUDIT.md)：

1. **重写 JWT 中间件** `internal/pkg/middleware/middleware.go`
   - 从 `Authorization: Bearer <token>` 提取 token；
   - 调用 `auth.ParseToken(jwtSecret, token)` 校验签名、`exp`、`nbf`；
   - `c.Set("user_id", claims.UserID)`、`c.Set("username", claims.Username)`、`c.Set("tenant_id", claims.TenantID)`、`c.Set("roles", ...)`、`c.Set("perms", ...)`；
   - jwtSecret 从 viper 读取（不再硬编码），启动校验长度 ≥32 字节。
2. **实现 `RequirePermission(code ...string)` 中间件**
   - 从 context 取用户权限集合，校验路由所需权限码；
   - 支持 `RequireAnyPermission`/`RequireAllPermission`；
   - 数据权限通过 `context` 注入 `data_scope`，repository 层用 GORM Scopes 自动过滤。
3. **路由组挂载权限码**
   - 对照 `sys_permission` 表中已有的 17 个权限码，在 `routes.go` 每个接口加 `RequirePermission`。
4. **修复 JWT Claims**
   - 增加 `iss`、`aud`、`jti`、`tenant_id`；
   - access token 短有效期（30 分钟），refresh token 7 天；
   - Redis 维护 refresh token 与 jti 黑名单（登出/改密时拉黑）。
5. **前端三级权限**
   - 路由 `meta.permissions`；
   - 守卫调用 `getPermissions()` 并校验；
   - 菜单按权限动态渲染；
   - 实现 `v-permission` 自定义指令做按钮级控制。

### 1.2 密钥与默认凭据（第 1 周）

- 删除 `configs/config.yaml` 中所有明文密钥，改为 `${JWT_SECRET}` 占位或从环境变量读取；
- `docker-compose.yml` 用 `.env` 文件（已在 `.gitignore`）注入机密，compose 文件里只留变量引用；
- 首次启动时若 admin 密码为默认值，强制改密；默认密码从镜像/环境变量注入一次性随机值并打印到启动日志；
- bcrypt cost 提到 12；
- 轮换已泄露的 JWT/DB/Redis/OpenAPI secret；
- 清理 git 历史中的密钥（`git filter-repo` 或 BFG）。

### 1.3 库存与成本正确性（第 1-2 周）

依据 [AUDIT P0-3~P0-6](AUDIT.md)、[02-business.md](audit/02-business.md) S1-S3：

1. **重写库存事务**：
   - repository 方法接收 `tx *gorm.DB`，禁止在 `WithTx` 回调里用 `r.db`；
   - 库存增减改为**原子条件 UPDATE**：
     ```sql
     -- 出库
     UPDATE inv_inventory
     SET qty = qty - ?, available_qty = available_qty - ?, updated_at = now()
     WHERE id = ? AND available_qty >= ?;
     ```
     检查 `RowsAffected`，为 0 即库存不足；
   - 入库用 `INSERT ... ON CONFLICT (material_id,warehouse_id,location_id) DO UPDATE SET qty=qty+?, available_qty=available_qty+?`；
   - 或统一在事务内 `SELECT ... FOR UPDATE`。
2. **修复移动平均成本**：
   - 入库 DTO 增加 `unit_cost`（采购入库从采购订单明细 price 带入，其他入库由请求传入）；
   - 公式：零库存首入 `avg_cost = unit_cost`，否则 `(avg_cost*qty + unit_cost*inQty)/(qty+inQty)`；
   - 退库/冲销单独处理（见 1.4）。
3. **合并跨服务事务**：
   - 采购入库、生产领料把库存变动、台账、单据状态、财务凭证放进**同一个 `*gorm.DB` 事务**；
   - `WarehouseService` 暴露 `InboundTx(tx, req)`/`OutboundTx(tx, req)` 供上层 service 在自己的事务里调用；
   - 长期看引入 Outbox/Event 模式解耦，但短期必须单事务。
4. **单号与幂等**：
   - 单号改用 DB sequence 或雪花算法（`IB20260813153012-0001`），禁止秒级时间戳；
   - 扫码请求必须带 `idempotency_key`（客户端生成 UUID），服务端用唯一约束去重；
   - `inv_stock_ledger` 对 `(biz_type, biz_no, material_id)` 加唯一约束。
5. **库位/仓库参数**：
   - 出库请求必须带 `location_code`；
   - 采购入库必须带 `warehouse_code`（删除硬编码 `ID:1`）；
   - 查询库存带 `location_id`。

### 1.4 数据模型与迁移对齐（第 2 周）

依据 [AUDIT P0-7](AUDIT.md)：

- 以 model 为准**重写迁移**，统一列名与类型；
- 补建缺失表：`prod_bom`、`prod_bom_item`、`fin_budget`（具体 DDL 见 [development/schema.md](development/schema.md)）；
- 统一状态码：库表 `DEFAULT 10` 改为与 Go 常量一致（10=草稿、20=已审批/已下达、30=进行中、40=已完成、50=已取消）；
- 引入 `golang-migrate` 管理迁移，废弃 compose 挂载 `docker-entrypoint-initdb.d` 的方式；
- 加 `schema_migrations` 版本表；
- 金额/数量字段 Go 端从 `float64` 改为 `shopspring/decimal`，库表保持 `NUMERIC(18,2~4)`；
- JSONB 字段统一用 IAM 模块已有的 `JSON` 类型或 `datatypes.JSON`。

### 1.5 财务最小闭环（第 2 周）

依据 [02-business.md S5](audit/02-business.md)：

- 业务发生时（采购入库、生产领料、销售出库——销售模块在 P2）在同事务内写**借贷双分录**并校验平衡；
- 凭证号用 sequence；
- 预算在下单时校验并原子扣减 `used_amount`；
- 报表按 `period`（会计期间）、按科目编码前缀（`1%` 资产、`2%` 负债、`5%` 收入贷方、`6%` 成本费用借方）汇总；
- 禁止 `_ = err`，所有错误必须返回或日志记录。

### 1.6 基础设施（第 2 周）

- `&http.Server{ReadTimeout,ReadHeaderTimeout,WriteTimeout,IdleTimeout,MaxHeaderBytes}` + `signal.NotifyContext` + `Shutdown`；
- 注册 `TraceID/CORS/RequestLogger/SecurityHeaders` 中间件，删除有 goroutine 泄漏 bug 的 `Timeout` 中间件或改用 `http.TimeoutHandler`；
- 统一 viper 配置：`AutomaticEnv()` + `SetEnvKeyReplacer(".","_")`，db/redis/logger/token 过期全部从 viper 取，删私有 `getEnv`；
- `gin.SetMode(release)`；
- 错误处理用真实 HTTP 状态码（`errors.GetHTTP`），前端 axios 拦截 HTTP 401 清登录态；
- 非 root Dockerfile、HEALTHCHECK、`/healthz`、`/readyz`；
- 提交 `go.sum`，前端 `npm ci`，固定镜像 tag（不用 `alpine:latest`）；
- **删除前端所有 catch 里的"成功"提示和 mock 数据**。

### 1.7 P0 验收清单

- [ ] 无 `Authorization: x` 不能访问任何 `/api/v1/*`；
- [ ] 低权限用户访问未授权接口返回 403；
- [ ] 并发出库不会超卖（压测：100 并发扣 100 库存，最终 qty=0）；
- [ ] 采购入库后 `avg_cost` 正确反映采购价；
- [ ] 事务2 失败时库存回滚（故障注入测试）；
- [ ] `POST /production/bom`、`GET /finance/account-entries` 不再 SQL 报错；
- [ ] 财务报表借贷平衡，收入取贷方；
- [ ] `go test ./...` 至少覆盖库存/采购/财务 service 核心路径；
- [ ] `govulncheck ./...` 无高危；
- [ ] 镜像以非 root 运行，`/healthz` 返回 200。

---

## 2. P1 补全阶段（4 周）

目标：把仓库里已经声明但没做完的功能补完，让现有模块真正可用。

### 2.1 IAM 完整化（第 3 周）

详见 [development/features/iam.md](development/features/iam.md)。新增接口：

- `POST /auth/logout`（拉黑当前 jti）
- `POST /auth/change-password`
- `POST /auth/reset-password/:id`（管理员）
- `GET /roles`、`POST /roles`、`PUT /roles/:id`、`DELETE /roles/:id`
- `GET /roles/:id/permissions`、`PUT /roles/:id/permissions`（分配权限）
- `PUT /users/:id/roles`（分配角色）
- `GET /users/:id` 补数据权限校验
- `GET /audit-logs`（审计日志查询页）
- `DELETE /users/:id`（停用，软删除）

修复 `CreateUser` 无法设密码的问题：新增 `CreateUserReq` DTO 含 `password` 字段（bcrypt 哈希）。

实现登录防爆破：Redis 记录 `login:fail:<ip>` 和 `login:fail:<username>`，5 次失败锁定 15 分钟，连续失败要求图形验证码。

### 2.2 MDM 主数据（第 3 周）

补全空实现：
- `CreateWarehouse`/`CreateLocation` 真正写库；
- `DeleteMaterial`/`DeleteSupplier` 改为软删除（`deleted_at`）+ 被引用时禁止删除；
- 增加 `UpdateWarehouse`/`UpdateLocation`/`DeleteWarehouse`/`DeleteLocation`；
- 物料增加字段：`min_stock`、`max_stock`、`purchase_unit`、`stock_unit`、`conversion_rate`、`batch_managed`、`serial_managed`、`shelf_life_days`、`category_id`（完整字段见 [schema.md](development/schema.md)）；
- 物料分类树：`mdm_material_category`（parent_id 自引用）；
- 客户主数据：`mdm_customer`（P2 销售模块使用，但表可以先建）。

### 2.3 采购模块补全（第 4 周）

- 采购订单状态机：`10 草稿 → 20 已审批 → 30 部分收货 → 40 已完成 → 50 已取消`，禁止跳跃/回退；
- `ApproveOrder` 写 `approved_at`、`approved_by`；
- 收满货自动转"已完成"；
- 采购退货单 `pur_purchase_return`（P2 与库存模块一起做）；
- 采购订单支持多条件查询（单号、供应商、状态、日期范围）；
- 订单明细支持按码明细（批次/序列号在 P2）。

### 2.4 生产模块补全（第 4-5 周）

- **BOM 管理**：树形 BOM，`prod_bom` + `prod_bom_item`，支持子件递归展开、替代品、损耗率；
- **工单完整状态机**：`10 创建 → 20 已下达 → 30 生产中 → 40 已完工 → 50 已关闭`；
- **生产领料**按 BOM 展开，校验 `picked_qty <= req_qty`，支持替代料；
- **生产入库/报工端点**（当前缺失）：`POST /production/receipt`，写入成品库存、台账、成本；
- **生产成本卷积**：按 BOM 滚算标准成本（材料+人工+制造费用），实际成本在入库时用移动平均；
- **工单物料齐套检查**：下达前校验库存是否够领料。

### 2.5 财务模块补全（第 5 周）

- 会计科目表 `fin_account`（树形，支持 1xxx-6xxx）；
- 凭证管理：`fin_voucher` 主表 + `fin_voucher_entry` 分录表，借贷必平衡；
- 自动凭证模板：采购入库、生产领料、生产入库、销售出库自动生成凭证；
- 预算：`fin_budget` 按科目+期间，业务下单时校验并原子扣减；
- 期末结转：损益类科目结转到本年利润；
- 财务报表：资产负债表、利润表、现金流量表（按会计准则取数）。

### 2.6 设备模块真实实现（第 6 周）

详见 [development/features/device.md](development/features/device.md)。

- WebSocket 服务监听 `device.websocket_port`（8081），设备通过 API Key + 签名鉴权连接；
- 真实 adapter 实现：Scanner（扫码枪）、RFID、PDA、打印机；
- 设备注册/心跳/下线状态机；
- 设备指令下发（通过 WebSocket 推送到设备）；
- 设备消息流水表 `dev_device_message`；
- 前端设备管理页：在线状态、远程指令、消息日志。

### 2.7 LLM 模块真实实现（第 6 周）

- `config.llm.api_key` 从 viper 读取，删除 `main.go` 硬编码空 key；
- 多轮对话：`buildMessages` 加载最近 N 条历史；
- 会话归属校验（IDOR 修复）；
- 文心一言网关要么真正对接要么删除；
- Dashboard LLM 分析：传入真实 KPI 数据 prompt，返回结构化分析（趋势、异常、建议）；
- 支持流式响应（SSE）；
- Prompt 模板管理（`llm_prompt_template` 表）。

### 2.8 OpenAPI 补全（第 6 周）

- 独立 Bearer token 校验中间件，调用 `ValidateAccessToken`；
- `client_id` 从 token 中提取，不接受 body 传入；
- `client_secret` bcrypt 存储，`subtle.ConstantTimeCompare` 比较；
- scope 校验；
- refresh token 旋转失效；
- webhook 实际投递（异步 worker，重试 + 死信）；
- webhook URL 校验 scheme、禁止内网地址（防 SSRF）；
- 标准 ERP 资源 API：`GET /openapi/v1/materials`、`GET /openapi/v1/inventory`、`POST /openapi/v1/orders`、`GET /openapi/v1/orders/:no` 等。

### 2.9 P1 验收清单

- [ ] 采购订单从创建到收货完成状态流转正确；
- [ ] 工单按 BOM 领料、生产入库、完工全链路跑通；
- [ ] 财务凭证自动生成且借贷平衡；
- [ ] 设备能通过 WebSocket 连接并发送心跳，断连检测；
- [ ] LLM 多轮对话有上下文，会话不能越权；
- [ ] OpenAPI 用 token 访问受保护资源，无 token 返回 401；
- [ ] 审计日志记录所有写操作；
- [ ] 用户/角色/权限 CRUD 完整。

---

## 3. P2 扩展阶段（6 周）

目标：补齐 ERP 核心业务模块，形成采购→销售→库存→生产→财务完整闭环。

### 3.1 销售管理（第 7-8 周）

详见 [development/features/sales.md](development/features/sales.md)。

新增表：
- `sal_sales_order` / `sal_sales_order_item`
- `sal_sales_outbound` / `sal_sales_outbound_item`
- `sal_sales_return` / `sal_sales_return_item`
- `mdm_customer`

功能：
- 销售订单 CRUD、审批、价格管理（销售单价、折扣）；
- 销售出库扫码（扣库存、写台账、生成成本凭证、确认应收）；
- 销售退货（入库、冲销应收）；
- 客户信用额度校验；
- 销售订单状态机与采购对称。

API 前缀：`/api/v1/sales/*`。

### 3.2 库存盘点（第 9 周）

新增表：
- `inv_stock_check`（盘点单主表：单号、仓库、状态、盘点人、差异金额）
- `inv_stock_check_item`（明细：物料、库位、账面数量、实盘数量、差异数量、差异成本）

功能：
- 创建盘点单（冻结库存：盘点期间该仓库/库位禁止出入库）；
- 录入实盘数量；
- 差异确认后自动生成盘盈入库/盘亏出库单（走库存事务、移动平均成本、财务凭证）；
- 盘点审批流；
- 支持按仓库/库位/物料分类盘点。

### 3.3 库存调拨（第 9 周）

新增表：
- `inv_transfer`（调拨单：调出仓库、调入仓库、状态、在途数量）
- `inv_transfer_item`

功能：
- 调拨申请、审批；
- 调出出库（扣调出仓库存，计入在途）；
- 调入入库（入调入仓库存，冲在途）；
- 移动平均成本在调入仓按新成本计算（跨仓成本不变，用调出仓的 avg_cost）；
- 支持跨库位调拨（同仓库内移动）。

### 3.4 批次与序列号（第 10 周）

- 物料标记 `batch_managed`/`serial_managed`；
- 新增 `inv_batch`（批次号、生产日期、失效日期、供应商、入库单号）；
- 新增 `inv_serial`（序列号、物料、当前库位、状态：在库/已售/在修）；
- 库存表增加 `batch_id`/`serial_id`（或拆分为库存与批次库存）；
- 入库时录入批次/序列号；
- 出库时按 FEFO（先到期先出）或 FIFO 自动推荐批次，扫码校验；
- 批次追溯：从原材料批次→生产工单→成品批次→销售客户的全链路追溯。

### 3.5 应收应付（第 11 周）

- `fin_receivable`（应收单：客户、来源单号、金额、已收金额、账期）
- `fin_payable`（应付单：供应商、来源单号、金额、已付金额、账期）
- `fin_receipt`（收款单）
- `fin_payment`（付款单）
- 销售出库/采购入库自动生成应收/应付；
- 收付款核销（支持部分核销、预收预付）；
- 账龄分析表（30/60/90/180/180+）；
- 对账单导出。

### 3.6 BOM 成本卷积与标准成本（第 11 周）

- BOM 多层递归展开；
- 标准成本 = 材料成本（子件标准成本 × 用量）+ 人工成本 + 制造费用；
- 实际成本用移动平均；
- 成本差异分析（标准 vs 实际）；
- 成本卡 `fin_cost_card` 按产品+期间。

### 3.7 导入导出与打印（第 12 周）

- 物料/供应商/客户/库存 Excel 导入（excelize）；
- 所有列表 Excel 导出（带筛选条件）；
- 单据打印模板（采购单、入库单、出库单、销售单、工单），用 PDF（gofpdf 或 wkhtmltopdf）；
- 条码/二维码生成与标签打印（boombuler/barcode）。

### 3.8 P2 验收清单

- [ ] 销售订单→出库→应收→收款全链路；
- [ ] 盘点差异自动调账，财务凭证正确；
- [ ] 调拨调出/调入库存一致，无在途悬挂；
- [ ] 批次管理物料出入库必须指定批次，FEFO 推荐正确；
- [ ] 序列号全链路可追溯；
- [ ] 应收应付账龄表准确；
- [ ] Excel 导入 1000 条物料 < 5s。

---

## 4. P3 平台化阶段（6 周）

### 4.1 审批工作流（第 13-14 周）

- `wf_definition`（流程定义：单据类型、节点、审批人规则）
- `wf_instance`（流程实例）
- `wf_task`（待办任务）
- `wf_approval`（审批记录）
- 支持多级审批、会签、或签、加签、转办、驳回；
- 审批人规则：指定人、角色、部门负责人、发起人上级；
- 前端流程设计器（拖拽节点，可用 bpmn-js 或简化版）；
- 待办列表、已办列表、审批历史。

### 4.2 多租户与组织架构（第 14-15 周）

- `sys_tenant`（租户表）
- `sys_department`（部门树）
- 用户归属部门；
- 所有业务表加 `tenant_id`，GORM Scopes 自动注入过滤；
- 租户管理员后台；
- 租户级配置（LOGO、名称、币种、会计期间规则）；
- 数据权限 `data_scope` 落地：全部/本部门/本部门及下级/本人/自定义部门。

### 4.3 审计日志与操作留痕（第 15 周）

- 审计日志中间件真正写入（登录、写操作、权限变更、审批、金额相关操作）；
- `sys_audit_log` 记录：用户、租户、IP、UA、模块、动作、请求参数摘要、响应状态、耗时、trace_id；
- 审计日志查询页（按用户/模块/时间/动作筛选）；
- 敏感字段脱敏（密码、token、手机、身份证）；
- 日志防篡改（可选：链式哈希或写入 WORM 存储）。

### 4.4 消息通知（第 16 周）

- `msg_message`（站内信）
- `msg_subscription`（用户订阅规则）
- 通知渠道：站内信、邮件、钉钉、企业微信、Webhook；
- 事件：审批待办、库存预警、到货提醒、设备离线、预算超限；
- 通知模板管理；
- 前端通知铃铛 + 未读数。

### 4.5 监控与可观测性（第 16 周）

- Prometheus `/metrics`（HTTP QPS/延迟、DB 连接池、Redis、Go runtime）；
- GORM 慢查询日志（阈值 200ms）；
- `/healthz`、`/readyz`；
- pprof（仅内网）；
- 结构化日志（zap 或 logrus JSON），带 trace_id/user_id；
- OpenTelemetry 分布式追踪（可选）；
- Grafana dashboard 模板。

### 4.6 CI/CD 与测试（第 17 周）

- GitHub Actions：`go test`、`golangci-lint`、`govulncheck`、前端 build、Docker 构建；
- 集成测试：testcontainers-go + Postgres + Redis，每个 service 至少覆盖主链路；
- 覆盖率门槛（service 层 ≥ 60%）；
- 数据库迁移 CI 校验（model 与迁移一致性）；
- 多阶段 Dockerfile、镜像扫描（trivy）；
- 预发布环境自动部署；
- 前端 ESLint + Prettier。

### 4.7 移动端/PWA（第 18 周）

- PWA 支持（manifest + service worker）；
- 仓库 PDA 专用页面（大按钮、扫码调用设备摄像头、离线缓存）；
- 移动端审批、消息查看；
- 响应式布局适配。

### 4.8 P3 验收清单

- [ ] 采购订单可配置 2 级审批流，驳回/通过/加签正常；
- [ ] 两个租户数据完全隔离；
- [ ] 审计日志可追溯任意写操作；
- [ ] 库存低于安全库存自动发站内信；
- [ ] Prometheus 抓取到业务指标；
- [ ] CI 全绿才能合并 PR；
- [ ] PWA 在手机浏览器可添加到主屏幕。

---

## 5. 非功能需求

### 5.1 性能目标
- 列表接口 P95 < 300ms（10 万行数据）；
- 扫码出入库 P95 < 200ms；
- 支持 100 并发扫码；
- 单库支持 100 万库存记录、1000 万台账记录。

### 5.2 安全
- 所有写接口幂等；
- 所有查询带租户过滤；
- 敏感字段加密存储（身份证、银行账号）；
- 密码 bcrypt cost 12；
- HTTPS 强制；
- 定期依赖扫描（每周）。

### 5.3 可用性
- 单机部署 99.5%；
- 数据库每日全备 + WAL 归档；
- RPO < 5 分钟，RTO < 30 分钟。

### 5.4 兼容性
- Chrome/Edge/Safari 最新两个大版本；
- 移动端 iOS 15+ / Android 10+；
- 屏幕分辨率 1280×720 起。

---

## 6. 技术栈演进

| 领域 | 当前 | 计划 |
|---|---|---|
| Web 框架 | gin v1.9.1 | 升级到最新稳定版 |
| ORM | gorm v1.25.5 | 保持 |
| 数据库 | PostgreSQL 15 | 升级到 16 |
| 迁移 | SQL 文件挂载 | golang-migrate |
| 金额 | float64 | shopspring/decimal |
| 日志 | logrus | zap（或保持 logrus JSON） |
| 测试 | 无 | testcontainers-go + testify |
| 任务 | 无 | asynq（基于 Redis 的异步任务/worker） |
| WebSocket | gorilla/websocket（未用） | 实际接入 |
| 前端 | Vue3 + Element Plus | 保持 |
| 图表 | echarts | 保持 |
| Excel | 无 | excelize |
| PDF | 无 | gofpdf |
| 条码 | 无 | boombuler/barcode |
| 监控 | 无 | prometheus client + grafana |

---

## 7. 风险与依赖

1. **审计整改必须先做**——在 P0 未完成前写新功能是在沙地上盖楼。
2. **生产/财务模块需要领域专家**——成本卷积、会计准则、BOM 展开最好有会计/生产顾问 review。
3. **数据迁移**——P0 阶段重写迁移会破坏现有数据，系统尚未上线可直接重建；若已有测试数据需写迁移脚本。
4. **设备对接**——真实硬件协议（Modbus、OPC UA、PLC）需要具体硬件规格，当前 adapter 只是抽象层。
5. **LLM 选型**——通义千问 API Key、配额、数据合规需确认；敏感财务数据是否适合发第三方 LLM 需评估。
6. **工作量**——18 周是单工程师全职估算，多人并行可缩短但模块间有依赖（财务依赖库存、销售依赖库存）。

---

## 8. 文档索引

- [开发约定](development/00-conventions.md)——目录分层、API 规范、事务、错误处理、前端规范
- [数据库变更](development/schema.md)——所有新表/新字段 DDL
- [功能设计](development/features/)
  - [iam.md](development/features/iam.md)——用户/角色/权限/审计
  - [sales.md](development/features/sales.md)——销售管理
  - [inventory.md](development/features/inventory.md)——盘点/调拨/批次
  - [finance.md](development/features/finance.md)——凭证/科目/应收应付/预算
  - [production.md](development/features/production.md)——BOM/工单/成本卷积
  - [device.md](development/features/device.md)——设备 WebSocket/适配器
  - [openapi.md](development/features/openapi.md)——第三方对接
  - [workflow.md](development/features/workflow.md)——审批流
  - [tenant.md](development/features/tenant.md)——多租户
- [API 总览](development/api.md)——新增接口清单
- [前端任务](development/frontend.md)——前端改造点

每个功能文档包含：需求描述、数据模型、接口设计、关键业务逻辑、边界条件、测试要点。
