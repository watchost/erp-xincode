# ERP-Xincode 开发文档

> 编写时间: 2026-08-12
> 仓库: github.com/watchost/erp-xincode
> 适用企业: 条码扫描器制造企业（库仕达/Gtcodestar）

---

## 一、当前功能盘点

### 已有模块及功能

| 模块 | 代码行数 | 已实现功能 | 数据库表 |
|------|---------|-----------|---------|
| IAM (身份认证) | 692 | 用户CRUD、角色管理、权限定义、登录 | sys_user, sys_role, sys_permission, sys_role_permission, sys_user_role, sys_audit_log |
| MDM (主数据) | 790 | 物料管理、供应商管理、仓库管理、库位管理 | mdm_material, mdm_supplier, mdm_warehouse, mdm_location |
| 仓库管理 | 590 | 库存查询、入库、出库、扫码出入库、库存预警 | inv_inventory, inv_stock_ledger |
| 采购管理 | 572 | 采购订单、采购入库、入库扫码、审批 | pur_purchase_order, pur_purchase_order_item, pur_purchase_inbound |
| 生产管理 | 779 | 工单管理、BOM管理、生产领料、生产出库 | prod_work_order, prod_work_order_bom, prod_production_outbound |
| 财务管理 | 698 | 凭证管理、成本卡片、成本汇总、预算管理 | fin_voucher, fin_cost |
| 仪表盘 | 160 | 经营概览、最近订单、库存预警、LLM分析 | - |
| 设备管理 | 529 | 设备注册、心跳、扫码枪/RFID/PDA适配器(stub) | dev_device, dev_device_log |
| LLM助手 | 502 | 会话管理、消息记录、通义千问接入 | llm_session, llm_message |
| OpenAPI | 530 | OAuth2客户端、Token管理、Webhook、同步日志 | open_client, open_token, open_webhook, open_sync_log |

### 前端页面

| 页面 | 路由 |
|------|------|
| 登录 | /login |
| 仪表盘 | /dashboard |
| 仓库-入库 | /warehouse/inbound |
| 仓库-出库 | /warehouse/outbound |
| 仓库-库存 | /warehouse/inventory |
| 采购-订单 | /purchase/orders |
| 采购-入库 | /purchase/inbound |
| 生产-工单 | /production/work-orders |
| 生产-出库 | /production/outbound |
| 财务-成本 | /finance/cost |
| 财务-报表 | /finance/reports |
| 主数据-物料 | /mdm/materials |
| 主数据-供应商 | /mdm/suppliers |
| 主数据-仓库 | /mdm/warehouses |
| 设备管理 | /device/list |
| 用户管理 | /iam/users |
| 角色管理 | /iam/roles |

### 技术栈

- **后端**: Go 1.21 + Gin + GORM + PostgreSQL + Redis
- **前端**: Vue 3 + Vite 5 + Element Plus + Pinia + Axios
- **部署**: Docker + docker-compose + Nginx

---

## 二、功能缺失分析

### 对比成熟ERP系统，缺失的核心模块

| 缺失模块 | 重要程度 | 说明 |
|----------|---------|------|
| **销售管理** | ⭐⭐⭐ | 无销售订单、客户管理、报价、发货流程 |
| **客户管理 (CRM)** | ⭐⭐⭐ | 无客户档案、联系人、客户分级 |
| **库存调拨** | ⭐⭐⭐ | 无仓库间调拨功能 |
| **库存盘点** | ⭐⭐⭐ | 无盘点单、盈亏处理 |
| **质量检验** | ⭐⭐⭐ | 无IQC来料检验、OQC出货检验、不良品管理 |
| **条码追溯** | ⭐⭐⭐ | 制造业核心需求，无SN序列号管理、产品追溯 |
| **工时管理** | ⭐⭐ | 无工时记录、人工成本核算 |
| **供应商评价** | ⭐⭐ | 无供应商绩效评估 |
| **审批工作流** | ⭐⭐ | 无可配置审批流，仅简单approve |
| **报表中心** | ⭐⭐ | 无可配置报表、导出Excel |
| **消息通知** | ⭐⭐ | 无站内消息、邮件通知 |
| **批次管理** | ⭐⭐ | 无物料批次、先进先出控制 |
| **多组织/多工厂** | ⭐ | tenant_id已定义但未使用 |
| **汇率管理** | ⭐ | 外贸业务需要 |
| **报关单据** | ⭐ | 外贸出口需要 |

---

## 三、功能建议（结合条码扫描器制造企业业务）

### 3.1 核心业务流程建议

#### 销售 → 生产 → 采购 → 仓库 完整链路

```
销售订单 → 客户下单
    ↓
MRP运算 → 检查库存 → 生成采购建议 + 生产建议
    ↓
采购订单 → 供应商发货 → 来料检验(IQC) → 入库
    ↓
生产工单 → BOM展开 → 领料 → 生产 → 质检(OQC) → 入库
    ↓
发货 → 销售订单关闭
```

### 3.2 条码追溯系统（制造业核心）

| 功能 | 说明 |
|------|------|
| SN序列号管理 | 每台扫描器分配唯一SN，记录生产日期、批次、工单 |
| 产品追溯 | 输入SN，查询：使用的物料批次、生产工单、质检记录、发货客户 |
| 物料批次管理 | 物料按批次入库，记录供应商、生产日期、有效期 |
| 正反向追溯 | 正向：物料→产品→客户；反向：客户→产品→物料→供应商 |
| 防伪查询 | 客户输入SN验证产品真伪 |

### 3.3 质量管理系统

| 功能 | 说明 |
|------|------|
| IQC来料检验 | 采购入库前检验，记录不良率 |
| IPQC过程检验 | 生产过程中检验 |
| OQC出货检验 | 成品出厂前检验 |
| 不良品管理 | 不良品登记、返工、报废 |
| 质量报表 | 不良率统计、Pareto分析 |

### 3.4 销售与客户管理

| 功能 | 说明 |
|------|------|
| 客户档案 | 客户信息、联系人、信用额度 |
| 销售报价 | 按客户/数量阶梯报价 |
| 销售订单 | 接单、排产、发货跟踪 |
| 发货管理 | 拣货、装箱、发货、物流跟踪 |
| 外贸单据 | PI(形式发票)、CI(商业发票)、PL(装箱单) |

### 3.5 设备集成（结合Gtcodestar产品）

| 功能 | 说明 |
|------|------|
| 扫码枪集成 | 仓库扫码出入库（已有部分实现） |
| RFID集成 | 批量盘点、资产追踪 |
| PDA终端 | 仓库移动作业 |
| 生产看板 | 实时显示工单进度 |
| 设备监控 | 扫码枪使用频率、故障率 |

### 3.6 财务增强

| 功能 | 说明 |
|------|------|
| 应收应付 | 客户应收、供应商应付 |
| 发票管理 | 开票记录、税率 |
| 核销管理 | 收付款核销 |
| 外汇管理 | 汇率记录、外币结算 |
| 成本核算 | 按工单核算材料+人工+制造费用 |

---

## 四、开发路线图

### Phase 0 - 安全修复 (1-2周) 🔴

> 上线前必须完成，详见 AUDIT.md

- [ ] 实现JWT认证（C1/C2）
- [ ] 实现RBAC权限中间件（C3）
- [ ] 移除硬编码密钥，改用环境变量（C4）
- [ ] 移除前端默认凭据（C6）
- [ ] 注册CORS/Recovery中间件（C7）
- [ ] 修复Viper环境变量（H1）
- [ ] 数据库端口不暴露（H3）

### Phase 1 - 核心业务闭环 (3-4周) 🟠

> 实现销售→生产→采购→仓库完整链路

- [ ] 客户管理模块 (CRM)
  - 客户档案、联系人、信用额度
  - `POST /api/v1/customers` - CRUD
  - `GET /api/v1/customers/:id/orders` - 客户订单历史
- [ ] 销售管理模块
  - 报价单、销售订单、发货单
  - `POST /api/v1/sales/orders` - 创建销售订单
  - `POST /api/v1/sales/shipments` - 创建发货单
  - 发货时自动扣减库存
- [ ] 库存调拨
  - `POST /api/v1/warehouse/transfer` - 仓库间调拨
- [ ] 库存盘点
  - `POST /api/v1/warehouse/stocktake` - 创建盘点单
  - 盘盈/盘亏自动生成库存调整单
- [ ] 批次管理
  - 物料入库时记录批次号、生产日期、有效期
  - 出库时按FIFO自动推荐批次

### Phase 2 - 制造业增强 (4-6周) 🟡

> 条码追溯、质量管理、MRP

- [ ] SN序列号管理
  - 产品入库时生成SN
  - `GET /api/v1/traceability/:sn` - 正反向追溯
  - 记录SN对应的工单、物料批次、质检结果
- [ ] 质量管理系统 (QMS)
  - `POST /api/v1/qms/iqc` - 来料检验
  - `POST /api/v1/qms/oqc` - 出货检验
  - 不良品登记与处理
  - 质量报表（不良率、Pareto）
- [ ] MRP物料需求计划
  - 根据销售订单 + BOM + 库存 → 生成采购建议 + 生产建议
  - `POST /api/v1/mrp/run` - MRP运算
- [ ] 工时管理
  - 工单关联工时记录
  - 人工成本核算
- [ ] 审批工作流
  - 可配置多级审批（采购审批、生产审批）
  - `POST /api/v1/workflow/definitions` - 定义审批流
  - `POST /api/v1/workflow/submit` - 提交审批

### Phase 3 - 外贸与财务增强 (4-6周) 🟢

> 针对库仕达外贸业务

- [ ] 外贸单据管理
  - PI (Proforma Invoice)
  - CI (Commercial Invoice)
  - PL (Packing List)
  - 自动生成PDF
- [ ] 汇率管理
  - 每日汇率记录
  - 外币销售订单换算
- [ ] 应收应付
  - 客户应收账款
  - 供应商应付账款
  - 收付款核销
- [ ] 发票管理
  - 开票记录
  - 税率配置
- [ ] 成本核算
  - 按工单归集：材料成本 + 人工成本 + 制造费用
  - 标准成本 vs 实际成本差异分析

### Phase 4 - 智能化与集成 (持续迭代)

- [ ] 报表中心
  - 可配置报表模板
  - 导出Excel/PDF
  - 定时邮件推送
- [ ] 消息通知
  - 站内消息
  - 钉钉/邮件通知
  - 库存预警、审批提醒
- [ ] 设备集成深化
  - 扫码枪数据采集（生产计数、防错）
  - RFID批量盘点
  - 生产看板实时展示
- [ ] LLM增强
  - 智能问答（库存查询、订单状态）
  - 智能分析（销售趋势、库存优化建议）
  - 自然语言报表生成
- [ ] OpenAPI增强
  - 客户在线下单
  - 产品真伪查询
  - 库存可用量查询

---

## 五、技术规范

### 5.1 代码规范

#### Go后端

```
internal/
├── {module}/           # 业务模块
│   ├── handler/        # HTTP处理器（参数校验、调用service）
│   ├── service/        # 业务逻辑
│   ├── repository/     # 数据访问
│   ├── model/          # 数据模型
│   └── dto/            # 请求/响应DTO
├── pkg/                # 公共组件
│   ├── middleware/     # 中间件
│   ├── db/             # 数据库
│   ├── redis/          # 缓存
│   ├── auth/           # 认证
│   ├── logger/         # 日志
│   └── errors/         # 错误定义
└── routes/             # 路由注册
```

**命名规范：**
- 文件名：小写+下划线 `purchase_order.go`
- 结构体：驼峰 `PurchaseOrder`
- 接口：驼峰+er后缀 `Repository` → `PurchaseOrderRepository`
- 函数：驼峰，构造函数用New前缀 `NewPurchaseService()`
- 常量：全大写+下划线 `MAX_RETRY_COUNT`

**错误处理：**
```go
// 不要忽略错误
// ❌ result := db.First(&user, id)
// ✅
result := db.First(&user, id)
if result.Error != nil {
    return nil, fmt.Errorf("查询用户失败: %w", result.Error)
}
```

**DTO绑定（防止字段注入）：**
```go
// 不要直接绑定到Model
// ❌ c.ShouldBindJSON(&user)
// ✅
type CreateUserDTO struct {
    Username string `json:"username" binding:"required"`
    Password string `json:"password" binding:"required,min=6"`
    Email    string `json:"email" binding:"omitempty,email"`
}
c.ShouldBindJSON(&dto)
user := model.SysUser{
    Username: dto.Username,
    Email:    dto.Email,
}
```

#### Vue前端

```
web/src/
├── views/{module}/     # 页面组件
├── components/         # 公共组件
├── store/{module}.js   # 状态管理
├── router/index.js     # 路由
├── api/{module}.js     # API调用
└── utils/              # 工具函数
```

### 5.2 API设计规范

**RESTful风格：**
```
GET    /api/v1/{resource}           # 列表（支持分页、筛选）
GET    /api/v1/{resource}/:id        # 详情
POST   /api/v1/{resource}            # 创建
PUT    /api/v1/{resource}/:id        # 更新
DELETE /api/v1/{resource}/:id        # 删除
POST   /api/v1/{resource}/:id/action # 自定义动作
```

**统一响应格式：**
```json
{
    "code": 0,
    "message": "success",
    "data": { ... },
    "trace_id": "xxx"
}
```

**分页参数：**
```
GET /api/v1/materials?page=1&page_size=20&keyword=GT&status=1
```

**错误码规范：**
| 范围 | 模块 |
|------|------|
| 10xxx | 通用错误 |
| 20xxx | IAM |
| 30xxx | MDM |
| 40xxx | 仓库 |
| 50xxx | 采购 |
| 60xxx | 生产 |
| 70xxx | 财务 |
| 80xxx | 设备 |
| 90xxx | OpenAPI |

### 5.3 数据库规范

**命名：**
- 表名：`模块前缀_表名`，如 `pur_purchase_order`
- 字段名：snake_case，如 `created_at`、`material_code`
- 索引名：`idx_表名_字段`，如 `idx_inv_inventory_material_id`
- 外键名：`fk_表名_关联表`，如 `fk_pur_order_item_order`

**必备字段（每张表）：**
```sql
id          BIGSERIAL PRIMARY KEY,
created_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
updated_at  TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
created_by  BIGINT,
updated_by  BIGINT,
deleted_at  TIMESTAMP WITH TIME ZONE,  -- 软删除
```

**新增表的Migration模板：**
```sql
-- +goose Up
CREATE TABLE sls_sales_order (
    id              BIGSERIAL PRIMARY KEY,
    order_no        VARCHAR(32) NOT NULL UNIQUE,
    customer_id     BIGINT NOT NULL,
    order_date      DATE NOT NULL,
    delivery_date   DATE,
    status          VARCHAR(20) DEFAULT 'draft',
    total_amount    DECIMAL(12,2) DEFAULT 0,
    currency        VARCHAR(3) DEFAULT 'CNY',
    remark          TEXT,
    created_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by      BIGINT,
    deleted_at      TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_sls_order_customer ON sls_sales_order(customer_id);
CREATE INDEX idx_sls_order_status ON sls_sales_order(status);
CREATE INDEX idx_sls_order_date ON sls_sales_order(order_date);

-- +goose Down
DROP TABLE IF EXISTS sls_sales_order;
```

---

## 六、模块详细设计

### 6.1 销售管理模块（新增）

**表前缀：** `sls_`

**数据表：**

| 表名 | 说明 |
|------|------|
| sls_customer | 客户档案 |
| sls_customer_contact | 客户联系人 |
| sls_quotation | 报价单 |
| sls_quotation_item | 报价明细 |
| sls_sales_order | 销售订单 |
| sls_sales_order_item | 销售订单明细 |
| sls_shipment | 发货单 |
| sls_shipment_item | 发货明细 |

**API设计：**
```
# 客户管理
GET    /api/v1/customers
POST   /api/v1/customers
GET    /api/v1/customers/:id
PUT    /api/v1/customers/:id
GET    /api/v1/customers/:id/orders

# 销售订单
GET    /api/v1/sales/orders
POST   /api/v1/sales/orders
GET    /api/v1/sales/orders/:id
PUT    /api/v1/sales/orders/:id/approve
POST   /api/v1/sales/orders/:id/ship

# 发货
POST   /api/v1/sales/shipments
GET    /api/v1/sales/shipments
```

### 6.2 条码追溯模块（新增）

**表前缀：** `trc_`

**数据表：**

| 表名 | 说明 |
|------|------|
| trc_serial_number | SN序列号主表 |
| trc_sn_material | SN关联物料批次 |
| trc_sn_quality | SN质检记录 |
| trc_material_batch | 物料批次 |

**API设计：**
```
# 序列号
POST   /api/v1/traceability/sn        # 生成SN（产品入库时）
GET    /api/v1/traceability/:sn        # 追溯查询（正向+反向）
GET    /api/v1/traceability/:sn/timeline # SN全生命周期时间线

# 批次
POST   /api/v1/traceability/batch      # 创建物料批次（入库时）
GET    /api/v1/traceability/batch/:id  # 批次追溯
```

### 6.3 质量管理模块（新增）

**表前缀：** `qms_`

**数据表：**

| 表名 | 说明 |
|------|------|
| qms_inspection_order | 检验单 |
| qms_inspection_item | 检验项目 |
| qms_defect_record | 不良品记录 |

**API设计：**
```
# 检验
POST   /api/v1/qms/iqc               # 来料检验
POST   /api/v1/qms/ipqc              # 过程检验
POST   /api/v1/qms/oqc               # 出货检验
GET    /api/v1/qms/inspections       # 检验单列表
GET    /api/v1/qms/reports           # 质量报表
```

### 6.4 现有模块增强

**仓库管理：**
- 新增：库存调拨、库存盘点、安全库存预警
- 新增：批次管理（先进先出）
- 优化：N+1查询修复

**采购管理：**
- 新增：供应商评价
- 新增：采购价格历史
- 优化：入库事务原子性

**生产管理：**
- 新增：工时记录、工单排产
- 新增：MRP运算
- 优化：BOM多级展开

**财务管理：**
- 新增：应收应付、发票管理
- 新增：成本核算（按工单）
- 优化：错误处理

**IAM：**
- 新增：RBAC中间件实际生效
- 新增：操作日志记录
- 新增：密码策略

**仪表盘：**
- 新增：销售统计、生产进度
- 新增：实时看板
- 优化：LLM分析接入真实数据

---

## 七、开发约定

### Git工作流
```
main          # 生产分支
├── develop   # 开发分支
│   ├── feature/sales-module    # 功能分支
│   ├── feature/qms-module
│   └── fix/jwt-auth
```

### 提交规范
```
feat: 新增销售订单功能
fix: 修复JWT认证中间件
refactor: 重构仓库Service层
docs: 更新开发文档
test: 添加采购Service单元测试
chore: 更新依赖
```

### 代码审查清单
- [ ] 是否使用DTO绑定（不直接绑定Model）
- [ ] 错误是否正确处理（无 `_ =` 忽略）
- [ ] 是否存在N+1查询
- [ ] 是否有硬编码密码/密钥
- [ ] 新增API是否有权限控制
- [ ] 新增表是否有必备字段（created_at等）
- [ ] 是否有对应单元测试

---

*文档维护: Hermes Agent*
