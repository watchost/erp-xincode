# IAM 用户/角色/权限/审计

## 0. 当前实现状态（2026-08-14）

已完成第一批 P0 后端能力：

- `Authorization: Bearer <token>` 真实解析 JWT，校验签名、过期时间和 HMAC 算法；
- claims 中的 `user_id`、`username`、`tenant_id`、`roles`、`perms`、`jti` 注入 gin context 和标准 `context.Context`；
- 所有 `/api/v1/*` 业务接口经过 `JWTAuth`，写/敏感接口经过 `RequirePermission`；
- access token 默认 30 分钟、refresh token 默认 7 天；refresh token 在 Redis 中保存并旋转；
- `POST /api/v1/auth/logout` 将当前 jti 写入 Redis 黑名单；`POST /api/v1/auth/change-password` 支持用户改密；
- 用户创建/更新改用 DTO，避免 `PasswordHash json:"-"` 导致无法设密码和全字段 mass assignment；密码使用 bcrypt cost 12。

尚未完成：中间件逐请求校验 jti 黑名单、登录失败限流/验证码、强制首登改密、角色/权限完整 CRUD、审计日志写入、数据权限 Scopes、前端路由/菜单/按钮三级权限。

## 1. 目标

完整的身份认证与访问控制：登录、登出、改密、用户/角色/权限 CRUD、数据权限、审计日志、登录防爆破。

## 2. 数据模型（补充字段）

见 [schema.md P1-1](../schema.md#p1-1-iam-与审计)。

- `sys_user` 增加 email/dept_id/must_change_pwd/pwd_changed_at/last_login_at/last_login_ip/deleted_at；
- 新增 `sys_department` 部门树；
- `sys_audit_log` 增加 trace_id/user_agent/status/duration_ms；
- 新增 `sys_login_attempt` 用于防爆破。

## 3. 接口

当前已挂载的接口：

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /login | 公开 | 登录，返回 access+refresh |
| POST | /refresh | 公开 | refresh token 换新并旋转 |
| POST | /auth/logout | 登录 | 拉黑当前 jti |
| POST | /auth/change-password | 登录 | 修改自己密码 |
| GET | /users/profile | 登录 | 当前用户信息 |
| GET | /users/permissions | 登录 | 当前用户权限码 |
| GET | /iam/users | `iam:user` | 用户列表 |
| GET | /iam/users/:id | `iam:user` | 用户详情 |
| POST | /iam/users | `iam:user` | 创建用户（含密码、角色） |
| PUT | /iam/users/:id | `iam:user` | 更新用户并替换角色 |

规划中的完整 IAM 接口：

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| DELETE | /iam/users/:id | `iam:user` | 停用（软删除） |
| POST | /iam/users/:id/reset-password | `iam:user` | 管理员重置密码 |
| GET/POST | /roles | `iam:role` | 角色列表/创建 |
| PUT/DELETE | /roles/:id | `iam:role` | 更新/删除角色 |
| GET/PUT | /roles/:id/permissions | `iam:role` | 角色权限查询/分配 |
| GET | /permissions | 登录 | 全部权限树 |
| GET | /audit-logs | `iam:audit:view` | 审计日志查询 |
| GET/POST | /departments | `iam:dept:*` | 部门树/新建部门 |

## 4. 关键逻辑

### 4.1 登录
1. 校验验证码（连续失败 5 次后要求）；
2. 检查账号是否被锁定（15 分钟内失败 5 次）；
3. 查用户、bcrypt 比对密码；
4. 成功：更新 last_login_at/ip，生成 access（30min）+ refresh（7d），写审计日志；
5. 失败：写 sys_login_attempt，计数超限锁定。

### 4.2 JWT Claims
```go
type Claims struct {
    UserID   int64
    Username string
    TenantID int64
    Roles    []string
    Perms    []string // 权限码列表；*:*:* 表示超级管理员
    JTI      string
    jwt.RegisteredClaims
}
```

`DeptID`、`DataScope` 等字段将在数据权限阶段补充。

### 4.3 权限中间件
```go
func RequirePermission(code string) gin.HandlerFunc {
    return func(c *gin.Context) {
        perms := c.GetStringSlice("perms")
        if !contains(perms, code) {
            abort(403)
            return
        }
        c.Next()
    }
}
```

### 4.4 数据权限
用户角色的 `data_scope`：
- 1=全部：不过滤；
- 2=本部门：`WHERE dept_id = ?`；
- 3=本部门及下级：`WHERE dept_id IN (子部门ID列表)`；
- 4=仅本人：`WHERE created_by = ?`；
- 5=自定义：关联 `sys_role_dept` 表。

用 GORM Scopes 注入：
```go
func DataScope(user *Claims) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        switch user.DataScope {
        case 2: return db.Where("dept_id = ?", user.DeptID)
        // ...
        }
    }
}
```

### 4.5 审计日志中间件
记录：path、method、user_id、tenant_id、ip、ua、请求参数（脱敏后 JSON）、响应状态、耗时、trace_id。写操作（POST/PUT/DELETE）必记；GET 可选。敏感字段（password/token）在 JSON 序列化前打码。

## 5. 测试要点
- 无 token/错 token → 401；
- 有权限/无权限 → 200/403；
- 水平越权：A 用户不能 GET /users/:id 拿到 B 用户详情（当 data_scope=仅本人）；
- 连续 5 次登录失败锁定 15 分钟；
- 改密后旧 token 加入黑名单；
- 审计日志正确写入。
