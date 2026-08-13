# 00 · 开发约定

> 所有后续开发必须遵循本约定。违反约定的 PR 不予合并。

---

## 1. 目录与分层

### 1.1 后端模块结构

每个业务模块统一采用五层结构：

```
internal/<module>/
├── model/        # GORM 模型 + 枚举常量 + TableName
├── dto/          # 请求/响应结构体（带 binding/validate 标签）
├── repository/   # 数据访问，仅操作 *gorm.DB
├── service/      # 业务逻辑、事务编排
└── handler/      # HTTP handler，只做参数解析 + 调用 service + 返回响应
```

规则：
- handler 不直接操作 repository；
- repository 方法必须接收 `*gorm.DB`（不要自己持有 db 又开事务），便于在 service 事务里复用；
- service 是唯一允许开启事务的地方；
- model 不包含业务方法，只定义数据结构；
- dto 与 model 分离，禁止直接 `ShouldBindJSON(&model.SysUser{})`；
- 跨模块调用只能通过 service 接口（如采购 service 调仓库 service 的 `InboundTx(tx, req)`）。

### 1.2 Repository 模板

```go
type MaterialRepository struct{ db *gorm.DB }

func NewMaterialRepository(db *gorm.DB) *MaterialRepository {
    return &MaterialRepository{db: db}
}

// 每个方法的第一个参数可以是 tx 或 db
func (r *MaterialRepository) FindByID(db *gorm.DB, id int64) (*model.MdmMaterial, error) {
    var m model.MdmMaterial
    if err := db.First(&m, id).Error; err != nil {
        return nil, err
    }
    return &m, nil
}

// 或者通过 WithTx 包装：
func (r *MaterialRepository) WithTx(tx *gorm.DB) *MaterialRepository {
    return &MaterialRepository{db: tx}
}
```

推荐第二种：service 里 `r.WithTx(tx).FindByID(id)`，避免每个方法传 db。

### 1.3 Service 事务模板

```go
func (s *PurchaseService) InboundScan(ctx context.Context, req dto.InboundReq) error {
    return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
        // 1. 校验（读在事务内，必要时 FOR UPDATE）
        order, err := s.orderRepo.WithTx(tx).FindByNo(req.OrderNo)
        if err != nil { return err }

        // 2. 原子条件 UPDATE（库存增减用 RowsAffected 校验）
        if err := s.invRepo.WithTx(tx).IncrAvailable(tx, invID, req.Qty); err != nil {
            return err
        }

        // 3. 写台账
        if err := s.ledgerRepo.WithTx(tx).Create(ledger); err != nil {
            return err
        }

        // 4. 更新单据
        res := tx.Model(...).Where("received_qty + ? <= qty", req.Qty).
            Update("received_qty", gorm.Expr("received_qty + ?", req.Qty))
        if res.Error != nil { return res.Error }
        if res.RowsAffected == 0 { return errors.New("超出订单数量") }

        // 5. 财务凭证（同事务）
        if err := s.voucherRepo.WithTx(tx).Create(voucher); err != nil {
            return err
        }
        return nil
    })
}
```

### 1.4 共享包

`internal/pkg/` 下已有的：
- `auth`：JWT 签发/解析
- `db`：DB 初始化、TxManager
- `redis`：Redis 客户端
- `logger`：结构化日志
- `response`：统一响应
- `errors`：错误码
- `middleware`：中间件

新增共享代码放这里，业务模块不要重复造轮子。

---

## 2. API 规范

### 2.1 路由命名

- 全小写，中划线分隔（`/purchase-orders` 而非 `/purchaseOrders`）；
- 资源用复数名词；
- 动作用子路径：`POST /purchase-orders/:no/approve`、`POST /purchase-orders/:no/release`；
- 版本前缀 `/api/v1/`；
- OpenAPI 第三方接口用 `/openapi/v1/`。

### 2.2 统一响应格式

成功：
```json
{ "code": 0, "msg": "ok", "data": { ... } }
```

失败：
```json
{ "code": 10001, "msg": "未认证", "data": null }
```

分页：
```json
{
  "code": 0,
  "data": {
    "list": [...],
    "total": 123,
    "page": 1,
    "page_size": 20
  }
}
```

### 2.3 HTTP 状态码

修复当前"所有错误返 200"的问题，按真实语义返回：

| 场景 | HTTP |
|---|---|
| 成功 | 200 |
| 创建成功 | 201 |
| 删除成功 | 204 |
| 参数错误 | 400 |
| 未认证 | 401 |
| 无权限 | 403 |
| 资源不存在 | 404 |
| 冲突（重复单号、幂等冲突） | 409 |
| 前置条件失败（状态机非法） | 422 |
| 限流 | 429 |
| 服务端错误 | 500 |

业务错误码继续用 body 里的 `code`，但 HTTP 状态码必须匹配。

### 2.4 幂等

所有写接口（POST/PUT/PATCH）必须支持幂等：
- 客户端在 Header 带 `Idempotency-Key: <uuid>`；
- 服务端 Redis 记录 `idempotency:<key>`，24h 内重复请求返回首次结果；
- 扫码类业务接口同时用业务单号 + 物料 + 库位的唯一约束兜底。

### 2.5 分页参数

```
GET /api/v1/materials?page=1&page_size=20&keyword=...&status=1
```
- `page` 默认 1，≥1；
- `page_size` 默认 20，1-100；
- 排序用 `sort=created_at:desc`，**禁止前端直接传列名到 SQL**，用白名单映射。

### 2.6 权限码

格式：`<module>:<resource>:<action>`，例如：
- `purchase:order:view`、`purchase:order:create`、`purchase:order:approve`
- `warehouse:inbound`、`warehouse:outbound`
- `iam:user:manage`、`iam:role:manage`

每个路由必须有对应权限码，在 `routes.go` 里显式声明：

```go
purchase.POST("/orders", middleware.RequirePermission("purchase:order:create"), h.CreateOrder)
```

---

## 3. 数据库约定

### 3.1 表名/列名

- 表名：`<模块前缀>_<业务名>`，前缀见现有约定（`sys_/mdm_/pur_/prod_/inv_/fin_/sal_/dev_/llm_/open_/wf_/msg_`）；
- 主键统一 `id BIGSERIAL`；
- 时间字段 `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`、`updated_at`；
- 软删除用 `deleted_at TIMESTAMPTZ`（GORM `gorm.DeletedAt`）；
- 金额 `NUMERIC(18,2)`、数量 `NUMERIC(18,3)`、成本/单价 `NUMERIC(18,4)`；
- 状态用 `SMALLINT`，枚举值在 model 里定义常量；
- JSONB 字段统一用 `datatypes.JSON`（gorm.io/datatypes）；
- 所有业务表必须有 `tenant_id BIGINT NOT NULL DEFAULT 0`（P3 多租户启用时立即生效）。

### 3.2 索引

- 外键列建索引；
- 经常查询的筛选条件组合建联合索引；
- 唯一约束用 `UNIQUE` 索引（业务单号、编码）；
- 台账类按时间分区（数据量大时）。

### 3.3 外键

当前迁移无外键。后续：
- 强一致关系（如订单明细→订单）建外键 + `ON DELETE CASCADE`；
- 跨模块引用（如入库单→物料）不建物理外键（避免模块耦合），应用层保证引用完整性；
- 字典/配置表建外键。

### 3.4 迁移

- 使用 golang-migrate，文件 `migrations/NNNN_name.up.sql` / `.down.sql`；
- 每次改动一对 up/down；
- 禁止修改已合并的迁移，新增迁移做变更；
- 迁移必须在 CI 里对空库跑一遍 up + down + up 验证。

---

## 4. 事务与并发

1. **单服务单事务**：一个业务操作（如采购入库）只开一个事务，跨 repository 调用共享同一个 `tx`。
2. **禁止事务内做外部 IO**：HTTP 调用、发消息、LLM 调用必须在事务提交后做（用 Outbox 或事件）。
3. **库存增减用原子 UPDATE**：
   ```go
   res := tx.Model(&model.InvInventory{}).
       Where("id = ? AND available_qty >= ?", id, qty).
       Updates(map[string]interface{}{
           "qty": gorm.Expr("qty - ?", qty),
           "available_qty": gorm.Expr("available_qty - ?", qty),
       })
   if res.RowsAffected == 0 { return ErrInsufficientStock }
   ```
4. **需要读取后判断的场景**用 `SELECT ... FOR UPDATE`：
   ```go
   tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inv, id)
   ```
5. **幂等键**在事务开始前检查 Redis 或唯一约束。

---

## 5. 金额与数值

- Go 端禁用 `float64` 表示金额/数量；
- 统一用 `github.com/shopspring/decimal`：
  ```go
  type MdmMaterial struct {
      StandardCost decimal.Decimal `gorm:"type:numeric(18,4)"`
  }
  ```
- 前端用 `decimal.js` 或整数分展示，禁止直接浮点运算；
- 计算移动平均成本时：
  ```
  new_avg = (old_avg * old_qty + in_cost * in_qty) / (old_qty + in_qty)
  ```
- 凭证借贷差额必须为 0（用 `decimal.Equal` 判断）。

---

## 6. 错误处理

### 6.1 错误码

`internal/pkg/errors/errors.go` 定义错误码：

```go
const (
    ErrSuccess      = 0
    ErrUnauthorized = 10001
    ErrForbidden    = 10002
    ErrParam        = 10003
    ErrNotFound     = 10004
    ErrConflict     = 10005
    ErrInternal     = 10300
    // 业务错误从 20000 开始
    ErrInsufficientStock = 20001
    ErrOrderOverReceived = 20002
    ErrInvalidStatus     = 20003
)
```

每个错误码映射 HTTP 状态码（`codeHTTP` map）。

### 6.2 抛错

```go
return errors.New(errors.ErrInsufficientStock, "库存不足")
```

### 6.3 禁止

- 禁止 `_ = err`；
- 禁止 `panic`（除了不可恢复的启动错误）；
- 禁止把 `err.Error()` 直接拼到响应里返回客户端（可能泄漏内部结构）；
- 禁止在 repository 里返回裸 `errors.New("xxx")`，用错误码。

---

## 7. 日志

- 用结构化日志（logrus JSON 或 zap）；
- 每个请求带 `trace_id`（中间件生成）、`user_id`、`tenant_id`；
- 日志级别：debug（开发）/info（默认）/warn/error；
- 禁止记录密码、token、完整身份证/银行卡号（脱敏）；
- error 级别日志必须带 stack trace。

---

## 8. 配置

- 所有配置通过 viper 读取；
- `viper.AutomaticEnv()` + `SetEnvKeyReplacer(".", "_")`；
- 敏感值（密码、密钥、API Key）禁止有默认值，启动时缺失直接 fatal；
- 非敏感值可有默认值；
- config.yaml 只放非敏感默认值，真实值通过环境变量注入。

---

## 9. 前端约定

### 9.1 目录

```
web/src/
├── api/           # 按模块拆分的 axios 调用
├── views/         # 页面，按模块分目录
├── components/    # 可复用组件
├── layouts/       # 布局
├── router/        # 路由 + 守卫
├── store/         # Pinia
├── utils/         # request、auth、format
├── directives/    # v-permission 等自定义指令
└── styles/
```

### 9.2 API 调用

- 所有接口在 `api/` 定义，不在 view 里直接 `axios.get`；
- 请求函数返回 `res.data`；
- catch 块**禁止**弹"成功"或写 mock 数据，必须显示错误。

### 9.3 权限

- 路由 `meta: { permissions: ['xxx'] }`；
- 按钮用 `v-permission="'xxx'"`；
- 菜单由后端权限动态生成。

### 9.4 表单

- 所有 `el-form` 必须有 `ref`、`:rules`、`prop`；
- 提交前 `await formRef.value.validate()`；
- 金额/数量输入用 `el-input-number`，`:min="0.001"`（禁止 0）。

### 9.5 Token

- 短期改 localStorage + 严格 CSP；
- 中期改 httpOnly cookie + CSRF token；
- axios 响应拦截器对 HTTP 401 清登录态跳登录页。

---

## 10. 测试

- 每个 service 必须有单元测试；
- 涉及数据库的用 testcontainers-go 起真实 PostgreSQL；
- 测试命名 `TestService_Method_Scenario`；
- 表驱动测试；
- CI 覆盖率门槛：service 层 ≥ 60%，关键库存/财务路径必须 100%。

---

## 11. Code Review Checklist

每个 PR 自检：

- [ ] 是否有 SQL 注入风险（拼接 SQL、order by 字段）？
- [ ] 写操作是否在事务里？跨表更新是否原子？
- [ ] 库存/金额是否用了原子 UPDATE 或行锁？
- [ ] 是否有 `_ = err`？
- [ ] 权限码是否挂载到路由？
- [ ] DTO 是否与 model 分离？
- [ ] 是否有硬编码密钥/ID？
- [ ] 前端是否有表单校验？catch 是否正确处理？
- [ ] 是否加了必要的测试？
- [ ] 迁移文件是否有 down？
