# 03 · 基础设施、配置、可观测性与代码质量审计

> 审计范围：`cmd/server/main.go`、`internal/pkg/{db,redis,logger,middleware,response,errors}`、`internal/routes`、`configs/`、`Dockerfile`、`web/Dockerfile`、`deploy/nginx.conf`、`web/nginx.conf`、`docker-compose.yml`、`Makefile`、`.golangci.yml`、`go.mod`、`migrations/`
> 审计版本：`9df33b1`，只读审计

---

## 一、严重（Critical）

### C1 JWT 中间件是空壳——所有"受保护"接口实际无认证
- 文件：`internal/pkg/middleware/middleware.go:82-92`、`internal/routes/routes.go:39`

```go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" { c.JSON(401, ...); c.Abort(); return }
        c.Next()   // 从未调用 auth.ParseToken
    }
}
```

全仓库 `grep 'c.Set("user_id"'` **零命中**。`c.GetInt64("user_id")` 在 `iam/handler/handler.go:58,76`、`llm/handler/handler.go:33,56` 永远返回 0；`GetUserInfo` 必然返回"用户不存在"；LLM 用 `if userID==0 { userID=1 }` 掩盖 bug。无 RBAC 中间件，越权严重。

修复：`JWTAuth` 调用 `auth.ParseToken(jwtSecret, token)`，剥 `Bearer `，`c.Set("user_id"/"username", ...)`；secret 通过中间件构造函数注入；实现 `RequirePermission` 并挂载。

### C2 生产/财务模块数据模型与迁移严重不一致，接口运行即报错
- 文件：`migrations/0001_init.up.sql` vs `internal/production/model`、`internal/finance/model`

**表缺失：** `fin_budget`、`prod_bom`、`prod_bom_item` 在迁移中零命中，但 repository 会查询。

**列名/类型不匹配：**

| 表 | 迁移列 | 模型字段（推断列名） |
|---|---|---|
| `prod_work_order` | `wo_no, product_id, plan_qty, status, bom_id` | `work_order_no, produced_qty, plan_start_at, plan_end_at, created_by, actual_start_at, actual_end_at` |
| `prod_work_order_bom` | `req_qty, picked_qty, is_substitute` | `plan_qty, issued_qty, unit` |
| `fin_cost` | `cost_type SMALLINT, ref_id, period, amount` | `product_id, cost_date, source_type, source_id, cost_type string` |
| `fin_voucher` | `voucher_no, biz_type SMALLINT, debit, credit, period` | `entry_no, account_code, account_name, debit_amount, credit_amount, balance, biz_type string` |

→ `GET /finance/budgets`、`POST /production/bom`、`POST /production/work-orders` 等全部 `column/relation does not exist`。代码路径从未在迁移后的库上跑通过。

修复：以模型为准重写迁移（或反向修模型）；补建缺失表；建立迁移/模型 CI 校验。

### C3 无优雅关闭，HTTP Server 零超时配置
- 文件：`cmd/server/main.go:142-151`

```go
r := gin.Default()
routes.RegisterRoutes(...)
log.Fatal(r.Run(":" + port))
```

全仓库 `grep "signal.Notify\|Shutdown\|http.Server{"` 零命中。Read/Write/IdleTimeout、MaxHeaderBytes 全为零值。

影响：Slowloris 慢连接耗尽 goroutine/FD；`docker stop`（SIGTERM）直接强杀在途请求，DB/Redis 连接不释放，可能中断事务；`config.yaml server.timeout: 30s` 从未被读取。

修复：`&http.Server{ReadTimeout, ReadHeaderTimeout, WriteTimeout, IdleTimeout, MaxHeaderBytes}`；`signal.NotifyContext` 监听 SIGINT/SIGTERM，`srv.Shutdown(ctx)` 并关闭 DB/Redis。

---

## 二、高（High）

### H1 TraceID/CORS/Timeout/Recovery 中间件定义了但从未注册
- 文件：`internal/routes/routes.go:39`

```go
api.Use(middleware.JWTAuth())   // 仅此一个
```

`middleware.TraceID/Recovery/CORS/Timeout` 全仓库无 `Use()` 调用，只靠 `gin.Default()` 的 Logger+Recovery。
- 无 `X-Trace-ID`，日志无法串联；
- 无 CORS 头，前端 dev（5173）直连被浏览器拦；`config.yaml cors.allow_origins` 是死配置；
- 无请求级超时，慢查询/慢 LLM 长期占用 goroutine。

修复：`r.Use(TraceID(), CORS(origins), RequestLogger(), SecurityHeaders())`；CORS origins 从配置读。

### H2 事务边界与并发控制缺陷
详见 [02-business.md](02-business.md) S2/S3。要点：
1. 事务内读未走 tx（`FindByMaterialWarehouse` 用 `r.db`）；
2. 采购入库先提交库存事务，再开第二个事务更新单据，第二步失败无法回滚；
3. 出库读后写无行锁，并发丢失更新。

修复：repository 接收 tx；`clause.Locking{Strength:"UPDATE"}`；库存+单据进同一事务或 Outbox。

### H3 配置系统断裂：viper 不绑定环境变量，config.yaml 大半是死配置
- 文件：`cmd/server/main.go`、`internal/pkg/db/db.go:46-68`、`internal/pkg/redis/redis.go:75-94`、`internal/pkg/logger/logger.go:48-50`

- `initConfig` 无 `AutomaticEnv/SetEnvKeyReplacer/BindEnv`；
- viper 全仓只读 `jwt.secret`、`server.port` 两个键；
- DB/Redis 完全绕过 viper，自己 `os.LookupEnv`；
- `logger.Init()` 硬编码 `InitLogger("info","text")`，忽略 `log.format: json`；
- main.go:81 硬编码 `2h, 24h`，忽略 `jwt.access_token_expire/refresh_token_expire`（声称 2h/7d，实际 refresh 只有 **24 小时**）。

**用当前 docker-compose 启动会怎样：**
- DB/Redis 连接正常（db.go/redis.go 直接读 env）；
- `JWT_SECRET`、`SERVER_PORT`、`JWT_ACCESS_EXPIRE`、`JWT_REFRESH_EXPIRE` **被静默忽略**：轮换 JWT_SECRET 不生效；SERVER_PORT 改了不生效；refresh 实际 24h；
- 日志输出 text 而非 json；
- 本地 `go run`（无 env）时，db.go 默认密码 `erp_password_zhouhouping_2026`，而 config.yaml 是 `erp_password`——按 config 建本地库必连不上。

修复：`viper.AutomaticEnv()` + `SetEnvKeyReplacer(".","_")`；db/redis/logger/token 过期全部从 viper 取；删私有 `getEnv`。

### H4 所有密钥/口令明文入库并提交 Git
- 文件：`docker-compose.yml:10,27,34,56,58`、`configs/config.yaml:14,23,28`、`migrations/0002_openapi_device.up.sql:57-58`

DB/Redis 密码、JWT secret、OpenAPI 默认 `client_secret` 全部明文。`.gitignore` 忽略 `.env` 但 compose 不用 env_file。admin 是公开 bcrypt 哈希。

修复：机密走 env/secret；config.yaml 只留非敏感默认；强制改 admin；轮换并清理 git 历史。

### H5 零测试，lint 配置形同虚设
- 全仓 `find . -name "*_test.go"` 零结果；`.golangci.yml` 启用 errcheck/gosec/staticcheck/wrapcheck，但代码里 `_ = err` 遍布、errors 不 wrap——lint 显然从未在 CI 跑过。

修复：优先 testcontainers-go + Postgres 集成测试；CI 强制 `go test` + `golangci-lint run`；清理 `_ = err`。

### H6 所有业务错误返回 HTTP 200
- 文件：`internal/pkg/response/response.go:53-59`、各 handler

```go
func FromError(err error) Response {
    var ae *appErrors.AppError
    if errors.As(err, &ae) { return Err(ae.Code, ae.Message) } // 忽略 ae.HTTP
    return Err(10300, "系统内部错误")
}
```

`errors.GetHTTP` 零调用；401/403/404/500 在 HTTP 层都是 200，WAF/监控/重试/熔断全部失效。

修复：handler 用 `errors.GetHTTP(ae.Code)` 设置真实状态码，或在统一中间件里根据 `AppError.HTTP` 设置。

---

## 三、中（Medium）

### M1 OpenAPI token 体系未接入中间件，secret 非常量时间比较
- `ValidateAccessToken` 零调用；`/openapi/*` 仅靠空壳 JWTAuth；
- `client.ClientSecret != req.ClientSecret` 有时序侧信道；
- refresh 只删 access token，旧 refresh token 可重用，无旋转失效。

### M2 容器以 root 运行、无 HEALTHCHECK、构建不可复现
- 两个 Dockerfile 均无 `USER`；
- `erp-server` 无 HEALTHCHECK；
- **仓库无 `go.sum`**，Dockerfile 在构建时 `go mod tidy` 两次，拉到的版本可能不同；
- `web/Dockerfile` 只 COPY package.json 未 COPY package-lock.json，用 `npm install` 而非 `npm ci`；
- 基础镜像 `alpine:latest` tag 漂移。

### M3 nginx 缺上传大小限制/安全头，两份配置重复
- `web/nginx.conf` 与 `deploy/nginx.conf` `diff` 完全相同，后者未被任何文件引用；
- 无 `client_max_body_size`（默认 1MB，批量扫码会 413）；
- 无 X-Frame-Options/X-Content-Type-Options/Referrer-Policy；
- `proxy_*_timeout 300s` 过长；无 WebSocket Upgrade 头（虽然 WS 未实现）。

### M4 Makefile migrate DSN 与实际完全不符
```
-database "postgres://erp:erp123@localhost:5432/erp_dev?sslmode=disable"
```
实际是 `erp_user / erp_password_zhouhouping_2026 / erp_system`。`make migrate-up` 必失败。

### M5 docker-compose 迁移机制无版本管理
把 up.sql 挂到 `/docker-entrypoint-initdb.d/`，只在卷空时执行一次；新增迁移不会自动跑；无 `schema_migrations` 表；down 迁移从不使用。

### M6 N+1 查询遍布列表接口
- `warehouse/service.go:206-209`：每行库存 3 次 FindByID；
- `purchase/service.go:128-131`+`145`：每订单每物料一次；
- `production/service.go:128-131`+`147`；
- `finance/service.go:49-56,92-99,254-256`；
- `warehouse/service.go:254-256`：库存预警每行 2 次查询。

20 行数据触发 40-60 次查询。

### M7 大量 error 被静默吞掉
- `dashboard/service.go:32-53`：8 处 `_ = s.db...`，出错返回零值；
- `finance/service.go:147-148,151,157`：报表错误被忽略；
- `openapi/service.go:98`：`_ = s.tokenRepo.DeleteByAccessToken(...)`；
- `iam/service.go:119`：`fmt.Sscanf` 忽略返回，Redis 中 userID 损坏静默变 0；
- 各 handler `strconv.Atoi(...)` 忽略错误，非法分页参数变 0 → `OFFSET -N` 异常。

### M8 库存加权平均成本计算错误
见 [02-business.md](02-business.md) S1。入库按 0 计成本，avg_cost 随入库稀释趋近 0；`pur_purchase_inbound.cost_amount` 正确记录但未回写库存。

### M9 LLM API Key 硬编码为空，LLM 功能不可用
`cmd/server/main.go:129` `gateway.NewQwenGateway("", "qwen-turbo")`；`config.llm.api_key` 从未被读；`dashboard.GetLLMAnalysis` 是桩实现。

### M10 无可观测性
无 prometheus `/metrics`（go.mod 有间接依赖但未用）；GORM 用零值 `gorm.Config{}` 无 Logger；无 `/healthz`、`/readyz`；无 pprof。

### M11 SQL 迁移无外键、缺索引、缺唯一约束
- 所有关联表（`sys_user_role`、`pur_purchase_order_item.order_id`、`prod_work_order_bom.work_order_id`、`llm_message.session_id`、`open_token.client_id` 等）无 FOREIGN KEY；
- 缺索引：`pur_purchase_inbound(order_id/supplier_id/warehouse_id)`、`prod_production_outbound(work_order_id)`、`fin_voucher(period/biz_type/biz_no)`、`open_token(client_id/expires_at)`、`llm_session(user_id)`、`inv_stock_ledger(warehouse_id)`；
- `mdm_warehouse.code` 无唯一索引（其他主数据都有 `uk_*_code`），可插重复仓库编码。

### M12 gin 运行模式未生效
`config.yaml mode: release` 但从未 `gin.SetMode`，compose 也未设 `GIN_MODE`，实际跑 debug。

### M13 Go 版本与依赖偏旧
Go 1.21 已 EOL；`x/crypto v0.17.0`、`x/net v0.22.0`、`gin v1.9.1`、`gorm v1.25.5` 均为 2023 年末版本。建议升级并 `govulncheck ./...`。

---

## 四、低（Low）

### L1 Timeout 中间件 goroutine 泄漏
`middleware/middleware.go:67-78` 超时后 `c.Abort()` 但执行 `c.Next()` 的 goroutine 仍在跑，可能向已结束 response 写入并触发 gin panic。当前未注册，启用前必须修复。

### L2 JSONB 字段用 `[]byte` 映射
`mdm/model.go:17 Attributes []byte`、`device/model.go:17,25 Config/Content []byte` 映射 JSONB，可能 `type bytea does not match jsonb`。IAM 模块实现了 `JSON` 类型，应复用。

### L3 设备适配器/dashboard/openapi 为桩实现
- `device/adapter/adapter.go` Scanner/RFID/PDA 三个结构体字段与方法完全重复，`Connect` 仅置 connected，`Read` 返回 `nil,nil`；
- `dashboard/service.go:58-66` GetStockAlerts/GetRecentOrders 永远空数组；
- `openapi/handler.go:86-91` ListClients 永远空数组；
- `llm/gateway/gateway.go:117` WenXin 返回"暂未对接"。

### L4 硬编码 ID 掩盖鉴权缺陷
- `purchase/service.go:88`、`production/service.go:81,296` `CreatedBy: 1`；
- `warehouse/service.go:61,138` 默认仓库 `&MdmWarehouse{ID:1}`；
- `llm/handler.go:34-36,57-59` userID 回退 1。

### L5 未使用的依赖
`gorilla/websocket v1.5.0`、`golang.org/x/time v0.5.0` 代码中零引用（WS 和限流都未实现）；`errors.Errorf`（errors.go:81）是死代码。

### L6 错误信息直接拼接返回客户端
`response.Err(10005, "参数错误: "+err.Error())` 泄漏 validator/底层错误细节。

### L7 DSN 缺时区与连接超时
`db.go:54` DSN 无 `TimeZone`、无 `connect_timeout`；Redis Options 未显式设 `MinIdleConns/DialTimeout/ReadTimeout`。

### L8 重复代码
- `deploy/nginx.conf` 与 `web/nginx.conf` 完全相同；
- `db.go`/`redis.go` 各复制一份 `getEnv`；
- device 三个 adapter 字段方法完全相同；
- 分页参数解析 `strconv.Atoi(c.DefaultQuery(...))` 重复十余处，应抽 helper。

---

## 五、提示（Info）

- `config.device.websocket_port: 8081` 无对应代码实现；
- module 名 `erp-system` 与目录 `erp-xincode` 不一致；
- `finance/service.go:142-145` period 解析语义不明（非空时用 `period+"-01"` 作结束日期）；
- `warehouse/repository/repo.go:55-61` 同时按 materialCode 和 materialName 过滤会 JOIN `mdm_material` 两次；
- 路由无任何 DELETE 方法。

---

## 六、Top 5 修复优先级

1. **C1 修复 JWT 中间件并接入 RBAC**——当前系统等同裸奔。
2. **C2 对齐生产/财务模型与迁移**——大半接口一调就 SQL 报错，且零测试。
3. **C3 + H1 补齐 HTTP Server 超时/优雅关闭与核心中间件**——上生产硬门槛。
4. **H3 统一配置加载（viper 绑定环境变量）+ H4 机密外置 + M12 gin release 模式**——消除配置陷阱。
5. **H2 修复事务边界与库存并发控制 + H7 处理被吞 error**——否则财务数据出错无人知晓。
