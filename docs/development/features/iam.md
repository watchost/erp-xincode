# IAM 用户/角色/权限/审计

## 0. 当前实现状态（2026-09-15，P1 §2.1 后端完整化）

已完成（接口先行，前端 `web/` 不在本期范围）：

- **认证**：JWT HS256 真实校验（签名/过期/算法）；claims 注入 gin context 与标准 ctx；
  access 2h（可配）、refresh 7d（Redis 保存并旋转）；登出 jti 黑名单**逐请求校验**；
  bcrypt cost 12。
- **JWT 演进**：claims 增加 `dept_id`、`data_scope`（旧 token 缺省按「全部」兼容）；
  `BlacklistChecker` 改为 `func(ctx, *Claims) bool`，同时校验 jti 黑名单与改密时间戳
  （`pwd:chg:<uid>`）——改密/重置后旧 token 立即失效。
- **用户管理**：列表/详情（套数据权限）、创建/更新（DTO 白名单）、**软删除**
  （`deleted_at` + 部分唯一索引，删除后同名可重建）、**管理员重置密码**
  （空密码→服务端生成 12 位随机密码仅返回一次）、独立角色分配。
- **角色/权限**：角色 CRUD、角色权限全量替换（存在性校验）、角色自定义部门配置、
  全量权限树；内置 `admin` 角色不可删且锁定 data_scope=1。
- **部门**：部门树（物化路径 `/1/3/`），创建/改名/删除；删除守卫
  （有子部门/有用户 → 409）。本期不支持调整父子关系（会牵连子树路径）。
- **数据权限**：5 档（全部/本部门/本部门及下级/仅本人/自定义部门），
  多角色取最宽档（1>3>2>5>4）；`internal/pkg/datascope` 纯 GORM scope。
  已落地：用户管理列表/详情、采购订单列表、工单列表（单据 `dept_id` 创建时取 claims）。
  主数据/库存/财务为共享数据，本期**不**套 scope。
- **防爆破**：用户名 + IP 双维度固定窗口计数（15 分钟）；失败 3 次强制图形验证码，
  失败 5 次锁定 15 分钟（10007/423，不透露锁定维度）。
- **图形验证码（后端）**：`golang.org/x/image/basicfont` 自绘 4 位数字 PNG
  （干扰线+噪点），base64 data URI，Redis 5 分钟、一次性（用后即焚）。
- **强制改密**：`mustchg:<uid>` 标记，中间件白名单外一律 10009/403；
  管理员重置/新建时可置位，自助改密后清除。
- **审计**：登录成功/失败由 service 落库（覆盖认证前失败）；其余写操作由
  `middleware.Audit` 在 `c.Next()` 后**异步**落库，JSON body ≤4KB 且
  password/old_password/new_password/token/refresh_token/captcha_code 递归打码；
  审计查询接口（分页 + 用户/模块/动作/时间区间过滤）。

## 1. 数据模型（迁移 0008）

- `sys_user`：email/dept_id/must_change_pwd/pwd_changed_at/last_login_at/last_login_ip/deleted_at；
  唯一索引改为 `uk_sys_user_username_alive ... WHERE deleted_at IS NULL`；
- 新表 `sys_department`（code 唯一、parent_id、物化路径 path、leader_id）；
- 新表 `sys_role_dept(role_id, dept_id)`：data_scope=5 自定义部门集；
- `sys_audit_log`：trace_id/tenant_id/user_agent/status/duration_ms + 复合索引；
- 新表 `sys_login_attempt`：登录尝试流水（锁定判定在 Redis，本表只留痕）；
- `pur_purchase_order` / `prod_work_order` 增加 `dept_id` + 索引；
- 种子新权限码 `iam:dept`、`iam:audit:view` 并授予 admin 角色。

## 2. 接口

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| POST | /api/v1/login | 公开（限流） | 登录，达阈值需 captcha_id/captcha_code |
| GET | /api/v1/auth/captcha | 公开（限流） | 图形验证码 data URI |
| POST | /api/v1/refresh | 公开 | refresh 旋转 |
| POST | /api/v1/auth/logout | 登录（改密白名单） | jti 拉黑 |
| POST | /api/v1/auth/change-password | 登录（改密白名单） | 自助改密，旧 token 全失效 |
| GET | /api/v1/users/profile | 登录（白名单） | 当前用户信息 |
| GET | /api/v1/users/permissions | 登录（白名单） | 当前用户权限码 |
| GET | /api/v1/iam/users | `iam:user` | 用户分页（套数据权限） |
| GET | /api/v1/iam/users/:id | `iam:user` | 用户详情（水平越权 → 10004） |
| POST | /api/v1/iam/users | `iam:user` | 创建用户 |
| PUT | /api/v1/iam/users/:id | `iam:user` | 更新用户/角色 |
| DELETE | /api/v1/iam/users/:id | `iam:user` | 软删除（禁删自己/admin） |
| POST | /api/v1/iam/users/:id/reset-password | `iam:user` | 重置密码；空 body 生成随机密码 |
| PUT | /api/v1/iam/users/:id/roles | `iam:user` | 全量替换用户角色 |
| GET | /api/v1/iam/roles | `iam:role` | 角色列表 |
| POST | /api/v1/iam/roles | `iam:role` | 创建角色（data_scope 1-5） |
| GET | /api/v1/iam/roles/:id | `iam:role` | 角色详情（含权限/部门 ID） |
| PUT | /api/v1/iam/roles/:id | `iam:role` | 改名/数据档（code 不可改） |
| DELETE | /api/v1/iam/roles/:id | `iam:role` | 删除（admin/有绑定者拒） |
| GET/PUT | /api/v1/iam/roles/:id/permissions | `iam:role` | 角色权限查/全量替换 |
| GET/PUT | /api/v1/iam/roles/:id/departments | `iam:role` | 自定义部门查/替换 |
| GET | /api/v1/iam/permissions | 登录 | 全量权限树 |
| GET | /api/v1/iam/departments | 登录 | 部门树（表单共用） |
| POST/PUT/DELETE | /api/v1/iam/departments[/{id}] | `iam:dept` | 部门写操作 |
| GET | /api/v1/iam/audit-logs | `iam:audit:view` | 审计分页查询 |

## 3. 关键逻辑

### 3.1 登录流程
1. `loginlock:user:<u>` / `loginlock:ip:<ip>` 任一存在 → 10007/423；
2. `loginfail:user/ip` 任一计数 ≥ 3 → 必须校验验证码（`captcha:<id>` GET 后立即 DEL，
   错误/过期/重放 → 10008/400）；
3. 查用户（软删不可见，统一凭证错误，防账号枚举）→ status → bcrypt；
4. 失败：双维度计数 INCR（首次 EXPIRE 900s）+ 写 sys_login_attempt + 审计 login_failed；
   达 5 写锁定键；
5. 成功：清失败计数、更新 last_login_at/ip、同步 mustchg 标记、写 login_success 审计、签发双 token。

### 3.2 JWT Claims
```go
type Claims struct {
    UserID    int64    `json:"user_id"`
    Username  string   `json:"username"`
    TenantID  int64    `json:"tenant_id"`
    DeptID    int64    `json:"dept_id,omitempty"`
    DataScope int      `json:"data_scope,omitempty"` // 多角色合并后的最宽档
    Roles     []string `json:"roles,omitempty"`
    Perms     []string `json:"perms,omitempty"` // *:*:* 超管
    JTI       string   `json:"jti,omitempty"`
    jwt.RegisteredClaims
}
```
CheckClaims：jti 在黑名单 → 拒；`pwd:chg:<uid>` 时间戳晚于 iat → 拒。
Redis 异常 fail-open（记日志），与 P0 行为一致。

### 3.3 数据权限（internal/pkg/datascope）
- 1=全部：不加条件；2=本部门：`<dept_col> = claims.dept_id`；
- 3=本部门及下级：`<dept_col> IN (path LIKE 物化路径前缀)`；
- 4=仅本人：`<owner_col> = user_id`；5=自定义：`<dept_col> IN (sys_role_dept)`；
- 列缺失/非法档位 → `WHERE 1=0`（fail-closed，不泄漏全表）；
- 多角色 `Merge`：空集按「仅本人」，取最宽档，顺序 1>3>2>5>4。

### 3.4 审计
- 登录审计由 service 写（认证前失败也要留痕），module=`auth`；
- `middleware.Audit` 挂在 JWT 组内，只记 POST/PUT/PATCH/DELETE，module 取 URL 首段，
  action=`METHOD <路由模板>`，异步写、失败只记日志；body 仅 JSON ≤4KB 并递归打码。

### 3.5 错误码
10006=409 冲突（唯一约束/删除守卫/admin 保护）；10007=423 已锁定；
10008=400 验证码错误；10009=403 强制改密。

## 4. 测试要点（已实现，见 internal/iam/service/integration_test.go）
角色生命周期与 admin 保护；权限替换（非法 ID 拒绝）与权限树不丢节点；软删后登录失败/
同名重建/列表不可见；重置密码随机 12 位 + 强制改密；改密后旧 token 失效；
5 次失败锁定（第 6 次 10007）；验证码阈值/错误/一次性；5 档可见性矩阵 + scope4 越权 10004；
部门删除守卫；审计留痕/时间区间校验；中间件打码与强制改密白名单（httptest）。
