# 多租户与组织架构

## 1. 目标
支持多租户 SaaS 化部署，租户间数据完全隔离；支持部门树和数据权限。

## 2. 数据模型
见 [schema.md P3-1](../schema.md#p3-1-多租户)。

- `sys_tenant` 租户表；
- `sys_department` 部门树（已在 IAM 中定义）；
- 所有业务表增加 `tenant_id`。

## 3. 租户隔离实现

### 3.1 租户识别
- JWT Claims 含 `tenant_id`；
- OpenAPI token 绑定 client 到 tenant；
- 中间件 `TenantContext()` 从 token 解析 tenant_id 写入 context。

### 3.2 自动过滤
GORM Scopes：
```go
func TenantScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        tenantID := auth.GetTenantID(ctx)
        return db.Where("tenant_id = ?", tenantID)
    }
}
```

所有 repository 的查询必须 `.Scopes(TenantScope(ctx))`。用 GORM 回调/Plugin 全局注入，避免遗漏。

### 3.3 写入自动设置
GORM BeforeCreate 回调自动从 context 取 tenant_id 填充。

## 4. 租户管理（平台管理员）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST | /admin/tenants | 租户 CRUD |
| POST | /admin/tenants/:id/disable | 停用 |
| GET | /admin/tenants/:id/stats | 使用统计 |

租户配置：名称、LOGO、币种、会计期间规则、套餐/过期时间。

## 5. 数据权限
与 IAM 的 `data_scope` 联动：
- 全部：不附加部门条件；
- 本部门：`dept_id = ?`；
- 本部门及下级：`dept_id IN (子部门ID列表，预计算)`；
- 仅本人：`created_by = ?`。

实现为 GORM Scope：
```go
func DataScope(ctx context.Context) func(*gorm.DB) *gorm.DB { ... }
```
repository 组合：`db.Scopes(TenantScope(ctx), DataScope(ctx))`。

## 6. 边界条件
- 跨租户查询绝对禁止（平台管理员接口显式绕过但需独立权限）；
- 租户过期：禁止登录和写操作；
- 租户数据备份：按 tenant_id 逻辑导出；
- 唯一性约束必须包含 tenant_id（如 `UNIQUE(tenant_id, code)`），否则不同租户编码冲突。

## 7. 测试要点
- 租户 A 的 token 不能访问租户 B 的数据；
- data_scope=仅本人时只能看自己的单据；
- 租户过期后写操作拒绝；
- 唯一约束在不同租户间不冲突。
