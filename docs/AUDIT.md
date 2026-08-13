# ERP 系统（erp-xincode）代码审计总报告

> 审计日期：2026-08-13
> 审计范围：`/opt/erp-xincode` @ `9df33b1`，全栈约 1.05 万行（Go 后端 + Vue3 前端）
> 审计维度：后端安全鉴权、业务正确性、基础设施/代码质量、前端安全与逻辑
> 审计方法：4 个独立 agent 并行精读源码与配置，只读审计，未修改任何代码
> 分报告：[安全与鉴权](audit/01-security.md) · [业务逻辑](audit/02-business.md) · [基础设施](audit/03-infrastructure.md) · [前端](audit/04-frontend.md)

---

## 结论

**当前代码不可上线、甚至不能投入任何真实业务数据。**

它处于"脚手架/Demo"阶段——认证是空壳、权限未落地、库存与财务的核心核算存在确定性错误、生产/财务模块因表结构与代码不匹配一调用就 SQL 报错。架构分层清晰（handler / service / repository / model / dto）是唯一的亮点，但核心血肉没填对。

---

## P0 致命问题（任一存在都不应对外提供服务）

### P0-1 JWT 认证中间件是空壳，系统等同于无认证
`internal/pkg/middleware/middleware.go:82-92`

```go
token := c.GetHeader("Authorization")
if token == "" { ... 401 ... }
c.Next()   // 从未调用 auth.ParseToken，从未 c.Set("user_id", ...)
```

- 任意非空 `Authorization` 头（如 `Authorization: x`）即可通过**全部** `/api/v1/*` 接口（用户、财务、采购审批、OpenAPI 管理）。
- `c.GetInt64("user_id")` 恒返回 0；LLM handler 用 `if userID == 0 { userID = 1 }` 掩盖，所有 LLM 请求以 admin 身份执行；`purchase/production` 的 `CreatedBy` 硬编码为 1。
- 前端因同样原因"看起来能登录"，但任何请求都不携带有效身份。

### P0-2 零权限/角色校验，水平 + 垂直越权
- 路由组只挂了空壳 `JWTAuth()`，**没有任何 `RequirePermission/RequireRole` 中间件**。`sys_permission` / `data_scope` 表和 `GetPermissions` 接口存在但服务端从不校验。
- 前端 16 条路由全部静态定义、菜单硬编码、无 `meta.permission`、无 `v-permission` 指令；`getPermissions` API 定义了但全项目零调用。**仓库管理员可直接敲 `/iam/users` 管理用户。**
- `:id` 类接口不校验归属；多租户 `tenant_id` 字段在 `sys_user` 存在但全代码零过滤。

### P0-3 移动平均成本公式写死为 0，库存估值永久归零
`internal/warehouse/service/service.go:94-98`

```go
inv.AvgCost = (inv.AvgCost*inv.Qty + 0) / (inv.Qty + req.Qty)  // 第二项应为 unitCost*inQty
```

- 入库接口不接收单价，采购订单 `matchedItem.Price` 从未传入库存模块。
- 结果：`inv_inventory.avg_cost` 恒为 0 → 库存总价值、出库成本、财务成本全部为 0，**存货核算完全失真**。

### P0-4 核心业务跨两个独立事务，必现账实不符
`internal/purchase/service/service.go:227/232`、`internal/production/service/service.go:235/240`

- 采购入库：先调 `warehouseService.Inbound`（事务1，已提交 inv+ledger），**再开事务2** 更新 `received_qty` + 写 `pur_purchase_inbound`。事务2 因单号冲突或 DB 抖动失败时，**库存已加、订单未记**。
- 生产领料：出库扣减与工单 `issued_qty` 更新同样拆分，出库已扣后无法回滚。
- 仓库 service 内部在 `WithTx` 回调里调用的 `FindByMaterialWarehouse` 用的是 `r.db` 而不是 `tx`，读不在事务内、无 `FOR UPDATE`，叠加 P0-5 必然丢失更新。

### P0-5 库存/收货/领料读-改-写无行锁，并发必超卖/超收/超领
- 全代码 `grep "FOR UPDATE\|clauses.Locking"` **零命中**。
- 出库 `tx.Where(...).First(&inv); check; tx.Save(&inv)`：两个并发出库读到同一 `available_qty`，都通过校验、都 Save → 只扣一次或扣成负数。
- 采购超收校验在内存里做（`receivedQty+reqQty > qty`），UPDATE 是 `received_qty = received_qty + ?` 无上限守卫 → 可超订单收货；生产领料同理可超领。
- 出库还**忽略库位**（只按 material+warehouse 过滤，`First()` 取主键最小行），采购入库仓库**硬编码 `ID=1`** → 库位/仓库级库存必然错乱。

### P0-6 业务单号用秒级时间戳 + 无幂等，重试即重复入账
`IB<unix秒>`、`PO<unix秒>`、`WO<unix秒>`、`OUT<unix秒>`、`FV<unix秒>`：
- 同秒两笔 → 唯一键冲突；事务2 失败但事务1（库存）已提交（P0-4），操作员重试 → 库存翻倍。
- 扫码接口无幂等键，`inv_stock_ledger` 对 `biz_no` 只有普通索引无唯一约束；PDA 重发即重复入账。

### P0-7 生产/财务模块模型与迁移严重不一致，调用即 SQL 报错

| 问题 | 具体 |
|---|---|
| 表根本不存在 | `prod_bom`、`prod_bom_item`、`fin_budget`（但 `BomRepository`/`BudgetRepository` 会查） |
| 列名不匹配 | `prod_work_order`：代码用 `work_order_no/produced_qty/plan_start_at/...`，库里是 `wo_no/...`；`prod_work_order_bom`：代码 `plan_qty/issued_qty/unit`，库里 `req_qty/picked_qty/is_substitute` |
| 列名+类型都不匹配 | `fin_voucher`：代码 `entry_no/account_code/debit_amount/biz_type string`，库里 `voucher_no/debit/biz_type SMALLINT`；`fin_cost` 同理 |

→ `POST /production/bom`、`POST /production/work-orders`、`GET /finance/account-entries`、`GET /finance/financial-report`、`GET /finance/budgets` 等全部 `column/relation does not exist`。**这些路径显然从未在迁移后的库上跑通过，且零测试。**

### P0-8 财务核算链路缺失：凭证从不生成、预算从不校验、报表口径错误
- `accountEntryRepo.Create`、`costCardRepo.Create`、`budgetRepo.FindByTypeAndPeriod/UpdateUsedAmount` 全代码零调用。采购入库/生产领料不产生任何会计分录，预算纯展示、下单时不校验不扣减。
- `finance/service.go:147-161` 报表：资产/负债只精确匹配科目 `1000/2000` 而非按前缀汇总；收入类（5xxx，中国准则贷方发生额）却 `SUM(debit_amount)` 取借方；期间用 `created_at <= period+"-01"` 只统计到月初；错误全部 `_ =` 吞掉 → **报表恒返回零且静默**。
- 无借贷平衡校验，无凭证号生成。

---

## P1 高危

### 安全类
- **P1-1 所有密钥/口令明文入库并提交 Git**：JWT secret、DB 密码、Redis 密码、OpenAPI 默认 `client_secret` 全部硬编码。且 viper 未启用 `AutomaticEnv`，运维在 compose 里轮换 `JWT_SECRET` 静默不生效（陷阱）。
- **P1-2 默认 admin 是公开 bcrypt 哈希**（网上教程同款），未强制首登改密；登录**无防爆破/锁定/验证码/限流**。前端登录页还硬编码并明文展示 `admin/admin123`。
- **P1-3 OpenAPI OAuth 是断链**：`ValidateAccessToken` 是死代码，webhook/sync 仅凭 body 中 `client_id` 就操作；`client_secret` 明文存储、用 `!=` 非常量时间比较；scope 不校验；refresh token 无旋转失效。
- **P1-4 LLM 会话 IDOR**：`GetHistory(sessionID)` 不校验归属，可遍历 `session_id` 读取他人聊天记录；`Chat` 可向他人会话写入。
- **P1-5 审计日志是死代码**：`AuditLogRepository` 注入了但从不 `.Create()`，关键操作无追溯。
- **P1-6 Mass Assignment + IDOR**：`UpdateUser` 忽略 URL `:id`，直接 `ShouldBindJSON(&model)` 后 `db.Save()` 全字段覆盖；`UpdateMaterial/UpdateSupplier` 同样问题。
- **P1-7 所有错误返回 HTTP 200**：`errors.GetHTTP`/`AppError.HTTP` 是死代码。前端只有业务码 `10001` 才登出，HTTP 401 不清 token 不跳登录。
- **P1-8 Token 存 localStorage、无 CSP、无安全头**：可被任意 JS 读取；nginx 无 X-Frame-Options/X-Content-Type-Options/CSP/HSTS。
- **P1-9 分页/输入无边界校验**：`page_size=99999999` 可拖库。
- **P1-10 容器以 root 运行、DB `sslmode=disable`、JWT 无 jti/无法撤销。**

### 正确性/可用性类
- **P1-11 无优雅关闭、HTTP Server 零超时**：`r.Run()` 未构造 `&http.Server{}`，Slowloris 风险；SIGTERM 强杀在途请求；`server.timeout: 30s` 从未被读取。
- **P1-12 中间件写了不挂**：`TraceID/CORS/Recovery/Timeout` 全部定义但不注册。CORS 白名单是死配置；`Timeout` 中间件本身有 goroutine 泄漏 + 重复写响应 bug。
- **P1-13 配置系统断裂**：viper 只读两个键；DB/Redis 各自 `os.LookupEnv`；日志硬编码 text 忽略 `log.format: json`；token 过期硬编码 2h/24h，与 config/compose 声称的 2h/7d 不符（**refresh 实际只有 24 小时**）；gin 实际跑在 debug 模式。
- **P1-14 前端 API 失败弹"成功"并写入假数据**：几乎所有 view 的 catch 块 `ElMessage.success('创建成功（模拟）')`；dashboard catch 硬编码 KPI。ERP 场景会导致误判入库/审批已完成。
- **P1-15 所有创建/审批表单零校验**：只有登录页有 `:rules`，出入库 `:min="0"` 允许 0 数量。
- **P1-16 MDM 多个写接口是空实现**：`DeleteMaterial/DeleteSupplier/CreateWarehouse/CreateLocation` 直接 `return nil`，返回 200 但不写库。
- **P1-17 LLM 功能不可用**：main.go `NewQwenGateway("", "qwen-turbo")` 硬编码空 key；文心网关返回"暂未对接"且 `nil` error；多轮对话从不加载历史。
- **P1-18 金额/数量全部 `float64`**：库表是 `NUMERIC(18,2~4)`，浮点累加必然产生几分钱误差。

---

## P2 中危（节选）

- 列表接口普遍 N+1（20 行触发 40-60 次查询）。
- 大量 error 被 `_ =` 吞掉（dashboard 8 处、finance 报表、`strconv.Atoi` 等）。
- 状态机不完整：订单收满不自动完成、工单无完工、`approved_at` 不写；Go 常量 1/2/3 与库表 `DEFAULT 10` 语义冲突。
- SQL 迁移无外键、缺索引、无版本管理（compose 只在卷空时执行一次 up.sql，无 schema_migrations）。
- 零测试 + 构建不可复现：无 `_test.go`、无 `go.sum`、Dockerfile 里 `go mod tidy` 两次、前端 `npm install` 而非 `npm ci`、镜像 `alpine:latest` tag 漂移、无 HEALTHCHECK。
- `Makefile` 迁移 DSN 与实际完全不符，`make migrate-up` 必失败。
- MDM/Device 的 JSONB 用 `[]byte` 映射，可能报 `type bytea does not match jsonb`。
- `CreateUser` 无法设置密码（`PasswordHash` 标 `json:"-"`，bcrypt 空串，登录又要求 required）→ 新用户永远无法登录。
- OpenAPI webhook URL 用户可控，实现投递即 SSRF。
- 无可观测性：无 `/metrics`、无慢查询日志、无 `/healthz`、无 pprof。
- nginx 无 `client_max_body_size`（默认 1MB），`deploy/nginx.conf` 与 `web/nginx.conf` 重复且前者是死文件。
- 前端：`JSON.parse(localStorage.userInfo)` 无 try-catch、无全局 errorHandler、无 token 刷新、ECharts 不 dispose。

---

## P3 低危/提示

- Go 1.21 已 EOL；`gin v1.9.1`、`golang.org/x/crypto v0.17.0`、`x/net v0.22.0`、`gorilla/websocket v1.5.0` 偏旧。
- 未使用依赖：`gorilla/websocket`、`golang.org/x/time`（设备 WS/限流都未实现）。
- device 三个 adapter 字段与方法完全重复且全是空桩；`device.websocket_port` 无对应实现。
- module 名 `erp-system` 与目录 `erp-xincode` 不一致；路由无任何 DELETE 方法。
- 库存预警阈值硬编码 `<10/<5`，物料无 `min_stock` 字段。
- 错误信息直接拼接 `err.Error()` 回客户端，泄漏内部结构。
- DSN 缺 `TimeZone`/`connect_timeout`；`db.go`/`redis.go` 各自复制一份相同的 `getEnv`。

---

## 修复路线图

### 阶段 0 — 止血（1-2 天）
1. 实现真正的 `JWTAuth`：`auth.ParseToken` → `c.Set("user_id",...)`；剥 `Bearer ` 前缀；secret 从 env 注入并校验长度 ≥32B。
2. 实现 `RequirePermission(code)` 中间件并在路由组挂载；前端三级权限接上 `getPermissions`。
3. 删除登录页硬编码 `admin/admin123`；首次启动随机生成 admin 密码或强制改密；登录加 Redis 限流。
4. 所有密钥外置到环境变量/secret，轮换已泄露的 JWT/DB/Redis/OpenAPI secret；清理 git 历史。

### 阶段 1 — 让数据正确（1-2 周）
5. **对齐模型与迁移**：以 model 为准补建表与列名映射，或反向修 model；引入 golang-migrate/goose 和 `schema_migrations`。
6. **重写库存事务**：repository 接收 `tx`；库存增减改**原子条件 UPDATE** 并检查 `RowsAffected`；出库带 `location_id`，入库带 `warehouse_id`。
7. **合并跨服务事务**：采购入库/生产领料把库存、单据、台账、财务凭证放进同一个事务，或引入 Outbox/补偿。
8. **修复移动平均成本**：入库加 `unit_cost`，公式 `(avgCost*qty + unitCost*inQty)/(qty+inQty)`；金额全部改 `shopspring/decimal` 或整数分。
9. **业务单号 + 幂等**：DB sequence/雪花/UUID；扫码接口加幂等键，台账对业务单号加唯一约束。
10. **财务链路**：业务发生时同事务写借贷双分录并校验平衡；预算下单时原子校验+扣减；报表按 `period`、科目前缀、正确的借贷方向重写；删除所有 `_ = err`。

### 阶段 2 — 上生产硬门槛（1 周）
11. `&http.Server{ReadTimeout,WriteTimeout,IdleTimeout,MaxHeaderBytes}` + `signal.NotifyContext` + `Shutdown`。
12. 注册 `TraceID/CORS/SecurityHeaders/RequestLogger`；删除有 bug 的 Timeout 中间件；`gin.SetMode(release)`。
13. 统一 viper 配置：`AutomaticEnv` + `SetEnvKeyReplacer(".","_")`，db/redis/logger/token 过期全部从 viper 取。
14. 错误处理：handler 用 `errors.GetHTTP(code)` 设置真实状态码；前端 HTTP 401 清登录态；**删除前端所有 catch 里的"成功"提示和 mock 数据**。
15. 非 root 镜像、HEALTHCHECK、`/healthz`、慢查询日志、prometheus `/metrics`；提交 `go.sum`、用 `npm ci`、固定镜像 tag。
16. 前端：token 改 httpOnly cookie（或 Bearer + 严格 CSP + 短有效期 + refresh 轮换）；表单加 rules；ECharts `dispose()`；安全存储读取 try-catch。

### 阶段 3 — 质量护栏（持续）
17. testcontainers-go + Postgres 为每个 service 写集成测试，覆盖并发扫码、事务回滚、成本核算、分录平衡。
18. CI 强制 `go test ./...`、`golangci-lint run`、`govulncheck`、前端 build。
19. 补外键与索引；OpenAPI `client_secret` 改 bcrypt/argon2 + `subtle.ConstantTimeCompare` + scope 校验 + token 哈希存储 + 旋转失效。
20. 实现审计日志中间件；多租户 `tenant_id` 用 GORM Scopes 自动注入（若产品确实要多租户）。

---

## 总评

| 维度 | 评价 |
|---|---|
| 架构分层 | ✅ 分层清晰，目录组织规范——唯一亮点 |
| 安全 | ❌ 不可接受：认证空壳、权限未落地、密钥全部泄露 |
| 业务正确性 | ❌ 不可接受：成本归零、事务拆分、无并发控制、生产/财务跑不起来 |
| 数据模型 | ❌ Go 模型与 SQL 迁移大面积不一致，且无测试兜底 |
| 基础设施 | ⚠️ 有硬伤：无超时/优雅关闭、配置断裂、构建不可复现、零可观测性 |
| 前端 | ⚠️ 原型完成度：权限空白、错误处理颠倒、表单零校验；但 XSS 直接风险低、依赖较新 |

> **在修复 P0 的 8 项之前，不要连接任何包含真实业务/财务数据的数据库，也不要暴露到非可信网络。**
