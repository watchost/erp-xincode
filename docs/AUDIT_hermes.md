# ERP-Xincode 代码审计报告

> 审计时间: 2026-08-12
> 审计对象: github.com/watchost/erp-xincode
> 提交版本: 9df33b1

---

## 一、项目概览

| 维度 | 详情 |
|------|------|
| 后端 | Go 1.21, Gin, GORM, PostgreSQL, Redis |
| 前端 | Vue 3, Vite 5, Element Plus, Pinia, Axios |
| 模块 | IAM, MDM(主数据), 仓库, 采购, 生产, 财务, 仪表盘, 设备, LLM, OpenAPI |
| 架构模式 | 分层架构 (handler -> service -> repository), 依赖注入 |
| 部署 | Docker + docker-compose, Nginx 反向代理 |
| 代码规模 | ~6,600 行 Go, ~3,900 行前端, ~387 行 SQL migrations |

---

## 二、问题清单（按严重程度分级）

### 🔴 严重 (Critical) - 7项

**C1: JWT 认证中间件形同虚设**
- 文件: `internal/pkg/middleware/middleware.go` L82-92
- `JWTAuth()` 仅检查 `Authorization` header 是否为空字符串，**完全不解析或验证 JWT token**。任何请求只要携带任意 Authorization 值即可通过认证。
- 影响: 所有受保护 API 可被未认证访问，系统完全不设防。

**C2: 中间件未将用户身份注入 Context**
- 文件: `internal/pkg/middleware/middleware.go` L82-92
- 即使修复 JWT 验证，当前中间件也从未将 `user_id` 设置到 gin.Context 中。Handler 中 `c.GetInt64("user_id")` 始终返回 0。
- LLM handler (L31-33) 对此的"修复"是 `if userID == 0 { userID = 1 }`，即所有请求都以 admin 身份执行。
- 影响: 越权访问，所有用户共享 admin 会话。

**C3: 无 RBAC 权限控制**
- 文件: `internal/routes/routes.go`
- 所有 API 路由仅使用 `JWTAuth()` 中间件，无任何权限检查中间件。系统定义了 `sys_role`、`sys_permission` 表和查询逻辑，但从未在请求链路中调用。
- 影响: 任何用户可访问用户管理、财务报表、采购审批等所有功能。

**C4: 硬编码密钥泄露（提交至 Git）**
- `configs/config.yaml` L28: JWT secret = `erp-system-jwt-secret-key-zhouhouping-copyright-2026`
- `internal/pkg/db/db.go` L50: 默认 DB 密码 = `erp_password_zhouhouping_2026`
- `docker-compose.yml` L10,27,51,56,58: PostgreSQL/Redis/JWT 密码全部明文
- `migrations/0002_openapi_device.up.sql` L58: OpenAPI client secret = `erp-system-secret-zhouhouping-copyright-2026`
- 影响: 任何能访问代码仓库的人可直接获取全部凭据。

**C5: OpenAPI 密钥明文存储与比较**
- 文件: `internal/openapi/service/service.go` L43
- `client.ClientSecret != req.ClientSecret` - 明文比较，数据库中也明文存储。
- 影响: 密钥泄露风险，时序攻击可能。

**C6: 前端暴露默认管理员凭据**
- 文件: `web/src/views/login/index.vue` L45-46, L64-65
- 登录页面显示 "默认账号：admin / admin123"，且表单预填了凭据。
- 影响: 攻击者可直接使用公开的凭据登录系统。

**C7: 中间件未注册（CORS/Recovery/TraceID/Timeout 均未生效）**
- 文件: `internal/routes/routes.go`
- `RegisterRoutes` 中仅注册了 `JWTAuth()` 中间件。CORS、Recovery、TraceID、Timeout 中间件虽有定义但从未使用。
- `gin.Default()` 自带 Recovery，但自定义的带 trace_id 的 Recovery 未生效。CORS 完全缺失，可能导致跨域问题或安全策略失效。

### 🟠 高 (High) - 8项

**H1: Viper 配置与环境变量未打通**
- `main.go` L80: `viper.GetString("jwt.secret")` 读取 config.yaml 的值
- `docker-compose.yml` 设置 `JWT_SECRET` 环境变量，但 Viper 未配置 `AutomaticEnv()` 或 env key mapping
- `db.Init()` 和 `redis.Init()` 完全绕过 Viper，直接用 `os.LookupEnv`
- 影响: Docker 部署时环境变量不生效，config.yaml 的配置也不被 DB/Redis 使用。

**H2: 无登录限流/暴力破解防护**
- 登录端点 `/api/v1/login` 无任何速率限制。
- `golang.org/x/time` 已在依赖中但未使用。

**H3: 数据库服务端口暴露宿主机**
- `docker-compose.yml`: PostgreSQL `5432:5432`，Redis `6380:6379` 均映射到宿主机
- 生产环境不应暴露数据库端口。

**H4: OpenAPI 端点错误地置于 JWT 认证之后**
- `routes.go` L124-129: `/api/v1/openapi/*` 路由在 `api.Use(middleware.JWTAuth())` 保护范围内
- OpenAPI 应使用 OAuth token 验证，而非 JWT。第三方客户端无法使用 JWT 访问。

**H5: CreateUser 直接绑定 Model，可篡改字段**
- `internal/iam/handler/handler.go` L117: `c.ShouldBindJSON(&user)` 直接绑定到 `model.SysUser`
- 攻击者可通过 JSON 注入 `id`、`status`、`tenant_id` 等字段
- 应使用 DTO 并做字段白名单。

**H6: 无 HTTPS/TLS 配置**
- Nginx 和后端均未配置 TLS，所有通信（含密码、token）明文传输。

**H7: LLM Gateway 创建时 API Key 为空**
- `main.go` L129: `gateway.NewQwenGateway("", "qwen-turbo")` - 硬编码空字符串
- config.yaml 中 `llm.api_key` 也为空，且 Viper 未打通环境变量
- 影响: LLM 功能完全不可用。

**H8: Webhook URL 无 SSRF 防护**
- `internal/openapi/service/service.go` L127-133: Webhook URL 直接存储，无 URL 校验
- 可被用于 SSRF 攻击内部网络。

### 🟡 中 (Medium) - 12项

**M1: N+1 查询问题严重**
- `warehouse/service/service.go` L206-209: ListInventory 中每条库存记录查询 3 次 (material, warehouse, location)
- `purchase/service/service.go` L128-131: ListOrders 中每笔订单查询 supplier + items，items 中每个 item 再查 material
- `production/service/service.go` L128-131: ListWorkOrders 同样模式
- 影响: 列表接口性能差，100 条记录可能产生数百次查询。

**M2: 零测试覆盖**
- 无任何 `_test.go` 文件存在
- Makefile 有 `test` 目标但无测试可执行

**M3: 大量错误被忽略 (`_ =` 模式)**
- `dashboard/service/service.go` L32-53: 6 次数据库查询全部忽略错误
- `finance/service/service.go` L151-161: 财务报表查询忽略错误
- 影响: 数据库故障时返回零值，用户无感知。

**M4: 无优雅关闭**
- `main.go` L151: `log.Fatal(r.Run(...))` 无 signal handling
- 进程终止时进行中的请求和事务会被中断

**M5: Logger 忽略配置**
- `logger/logger.go` L48-50: `Init()` 硬编码 `"info"` 和 `"text"`
- config.yaml 中的 `log.level: info` 和 `log.format: json` 未被使用

**M6: Timeout 中间件存在并发安全问题**
- `middleware.go` L62-80: goroutine 中调用 `c.Next()` 操作同一个 `*gin.Context`
- gin.Context 不是并发安全的，可能导致数据竞争

**M7: 无外键约束**
- migrations SQL 中所有表无 FOREIGN KEY 约束
- 数据完整性依赖应用层，存在孤儿记录风险

**M8: 无数据库 updated_at 自动更新**
- `inv_inventory` 有 `updated_at` 但无触发器，依赖应用代码 `inv.UpdatedAt = time.Now()` 设置
- 部分更新路径可能遗漏

**M9: 采购入库非原子操作**
- `purchase/service/service.go` L227-247: 仓库入库和采购记录更新分属不同事务
- 仓库入库成功但采购记录更新失败时，数据不一致

**M10: 无安全响应头**
- Nginx 和后端均未设置 `X-Content-Type-Options`、`X-Frame-Options`、`Strict-Transport-Security` 等头

**M11: 无健康检查端点**
- 无 `/health` 或 `/readiness` 端点，Docker 无 healthcheck for erp-server

**M12: 模块名不匹配**
- `go.mod` 声明 `module erp-system`，但项目路径为 `github.com/watchost/erp-xincode`

### 🟢 低 (Low) - 10项

**L1: 大量硬编码值**
- `warehouse/service.go` L61: `warehouse := &mdmModel.MdmWarehouse{ID: 1}` 默认仓库
- 多处 `CreatedBy: 1` 硬编码用户 ID
- `warehouse/service.go` L249: `available_qty < 10` 硬编码阈值

**L2: 设备适配器代码完全重复**
- `device/adapter/adapter.go`: ScannerAdapter、RFIDAdapter、PDAAdapter 三个结构体代码完全相同（均为空实现 stub）

**L3: 未使用的依赖**
- `gorilla/websocket` 在 go.mod 中但代码中未使用（设备适配器是 stub）

**L4: 错误码映射不完整**
- `errors.go` 中 `codeHTTP` map 缺少部分代码（如 40006、50005、50006、70006）

**L5: 缺少 go.sum**
- Dockerfile 中 `COPY go.mod go.sum*` 的通配符暗示 go.sum 可能不存在
- 应提交 go.sum 确保依赖完整性

**L6: 前端路由无权限守卫**
- `router/index.js` L122-137: 仅检查 token 是否存在，不验证用户权限
- 所有已登录用户可访问所有页面

**L7: 前端 token 存储在 localStorage**
- `store/user.js` L19: `localStorage.setItem('token', ...)`
- 易受 XSS 攻击，建议使用 httpOnly cookie

**L8: Makefile 中数据库连接串含密码**
- `Makefile` L13: `postgres://erp:***@localhost:5432/erp_dev` - 密码被掩码但格式不正确

**L9: Dockerfile 未优化镜像大小**
- 使用 `golang:1.21` 而非 `golang:1.21-alpine` 作为构建阶段
- 未使用多阶段构建的 `scratch` 或 `distroless` 最终镜像

**L10: 无 .dockerignore**
- 构建上下文会包含 .git、web/node_modules 等不必要文件，增加构建时间

---

## 三、改进建议

### 优先级 P0（上线前必须修复）

1. **实现完整的 JWT 验证** (C1, C2): 使用 `golang-jwt/jwt` 库解析和验证 token，将 user_id 注入 gin.Context
2. **实现 RBAC 中间件** (C3): 在路由注册时添加权限检查中间件，基于 `sys_role`/`sys_permission` 表
3. **移除所有硬编码密钥** (C4): 使用环境变量或密钥管理服务，从代码和配置文件中清除所有明文密码
4. **移除前端默认凭据** (C6): 登录页面不显示任何默认账号密码
5. **注册所有中间件** (C7): CORS、Recovery(带TraceID)、Timeout 均需在路由中注册
6. **修复 Viper 环境变量** (H1): 配置 `viper.AutomaticEnv()` 和 key mapping，确保 Docker 环境变量生效
7. **数据库端口不暴露** (H3): docker-compose 中改为 `expose:` 而非 `ports:`

### 优先级 P1（上线后尽快修复）

8. **添加登录限流** (H2): 使用 `golang.org/x/time/rate` 或 Redis 令牌桶
9. **OpenAPI 路由移出 JWT 保护** (H4): 使用独立的 API Key 认证中间件
10. **使用 DTO 绑定请求** (H5): 创建 CreateUserDTO，仅允许合法字段
11. **配置 HTTPS/TLS** (H6): Nginx 配置 SSL 证书，后端仅监听 localhost
12. **修复 OpenAPI 密钥比较** (C5): 使用 `subtle.ConstantTimeCompare`，数据库中存储 hash
13. **添加 Webhook URL 校验** (H8): 校验 URL scheme、拒绝内网地址

### 优先级 P2（中期改进）

14. **修复 N+1 查询** (M1): 使用 GORM `Preload` 或 `Joins`
15. **添加单元测试** (M2): 至少覆盖 service 层核心逻辑
16. **修复错误处理** (M3): 不忽略数据库查询错误
17. **实现优雅关闭** (M4): 使用 `http.Server` + signal handling
18. **添加外键约束** (M7): 在 migrations 中定义 FK
19. **添加健康检查** (M11): `/health` 端点 + Docker healthcheck
20. **修复模块名** (M12): go.mod module 改为 `github.com/watchost/erp-xincode`

### 优先级 P3（长期优化）

21. **前端权限守卫** (L6): 路由守卫中校验用户角色
22. **优化 Docker 镜像** (L9, L10): 使用 alpine + 多阶段构建 + .dockerignore
23. **提交 go.sum** (L5): 确保依赖版本锁定
24. **消除硬编码** (L1): 使用配置或常量管理

---

## 四、总结评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 代码结构 | 7/10 | 分层清晰，模块划分合理，命名规范 |
| 安全性 | 2/10 | 认证形同虚设，密钥泄露，无权限控制 |
| 代码质量 | 4/10 | 零测试，错误处理差，N+1 查询 |
| 依赖管理 | 5/10 | 依赖版本合理，但缺 go.sum |
| 部署配置 | 3/10 | 密码明文，端口暴露，无 HTTPS |
| 数据库设计 | 5/10 | 表结构完整，但无外键、无触发器 |
| 前端 | 5/10 | 结构清晰，但无权限守卫，token 存 localStorage |
| **综合评分** | **4/10** | **架构基础良好，但安全几乎为零，上线前需大量修复** |

---

*审计人: Hermes Agent*
