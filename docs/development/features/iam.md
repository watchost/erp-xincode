# IAM 用户/角色/权限/审计

## 1. 目标

完整的身份认证与访问控制：登录、登出、改密、用户/角色/权限 CRUD、数据权限、审计日志、登录防爆破。

## 2. 数据模型（补充字段）

见 [schema.md P1-1](../schema.md#p1-1-iam-与审计)。

- `sys_user` 增加 email/dept_id/must_change_pwd/pwd_changed_at/last_login_at/last_login_ip/deleted_at；
- 新增 `sys_department` 部门树；
- `sys_audit_log` 增加 trace_id/user_agent/status/duration_ms；
- 新增 `sys_login_attempt` 用于防爆破。

## 3. 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /auth/login | 公开 | 登录，返回 access+refresh |
| POST | /auth/logout | 登录 | 拉黑当前 jti |
| POST | /auth/refresh | 公开 | refresh token 换新 |
| POST | /auth/change-password | 登录 | 修改自己密码 |
| GET | /users/profile | 登录 | 当前用户信息+权限 |
| GET | /users | iam:user:view | 用户列表（带数据权限过滤） |
| POST | /users | iam:user:create | 创建用户（含密码、角色） |
| GET | /users/:id | iam:user:view | 用户详情 |
| PUT | /users/:id | iam:user:update | 更新用户 |
| DELETE | /users/:id | iam:user:delete | 停用（软删除） |
| PUT | /users/:id/roles | iam:user:assign | 分配角色 |
| POST | /users/:id/reset-password | iam:user:reset | 管理员重置密码 |
| GET | /roles | iam:role:view | 角色列表 |
| POST | /roles | iam:role:create | 创建角色 |
| PUT | /roles/:id | iam:role:update | 更新角色 |
| DELETE | /roles/:id | iam:role:delete | 删除角色 |
| GET | /roles/:id/permissions | iam:role:view | 角色权限 |
| PUT | /roles/:id/permissions | iam:role:assign | 分配权限 |
| GET | /permissions | 登录 | 全部权限树 |
| GET | /audit-logs | iam:audit:view | 审计日志查询 |
| GET | /departments | iam:dept:view | 部门树 |
| POST | /departments | iam:dept:manage | 新建部门 |

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
    DeptID   int64
    Roles    []string
    Perms    []string    // 权限码列表
    jti      string
    jwt.RegisteredClaims
}
```

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
