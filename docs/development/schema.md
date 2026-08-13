# 数据库变更设计

> 本文档列出 P0-P3 所有阶段需要新增/修改的表。当前仓库已有 `0001_init`、`0002_openapi_device`、`0003_iam_permissions`、`0004_production_schema` 四组 SQL 迁移；正式迁移工具（golang-migrate/goose）和 `schema_migrations` 版本表仍待引入。
> 所有新表默认含 `id BIGSERIAL PK`、`created_at`、`updated_at`，业务表带 `tenant_id BIGINT NOT NULL DEFAULT 0`、`deleted_at TIMESTAMPTZ`（软删除）。

---

## P0：整改阶段

### P0-1 修正生产模块模型与建表

```sql
-- 0003_fix_production_schema.up.sql

-- BOM 主表（当前不存在）
CREATE TABLE prod_bom (
    id           BIGSERIAL PRIMARY KEY,
    bom_no       VARCHAR(64) NOT NULL,
    product_id   BIGINT NOT NULL,
    version      INT NOT NULL DEFAULT 1,
    status       SMALLINT NOT NULL DEFAULT 1,  -- 1=草稿 2=启用 3=停用
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    remark       TEXT,
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_prod_bom_no ON prod_bom(bom_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_prod_bom_product ON prod_bom(product_id, is_active) WHERE deleted_at IS NULL;

-- BOM 明细
CREATE TABLE prod_bom_item (
    id            BIGSERIAL PRIMARY KEY,
    bom_id        BIGINT NOT NULL,
    parent_id     BIGINT,                      -- 子件层级（NULL=首层）
    material_id   BIGINT NOT NULL,
    qty           NUMERIC(18,3) NOT NULL,      -- 用量
    loss_rate     NUMERIC(6,4) NOT NULL DEFAULT 0,  -- 损耗率
    unit          VARCHAR(16),
    is_substitute BOOLEAN NOT NULL DEFAULT FALSE,
    substitute_group VARCHAR(32),              -- 替代料组
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_prod_bom_item_bom ON prod_bom_item(bom_id);

-- 修正 prod_work_order：补齐模型中用到的列
ALTER TABLE prod_work_order
    ADD COLUMN IF NOT EXISTS work_order_no VARCHAR(64),
    ADD COLUMN IF NOT EXISTS produced_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS plan_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS plan_end_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_end_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_by BIGINT,
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 兼容旧列：保留 wo_no，把它和 work_order_no 同步；最终废弃 wo_no
UPDATE prod_work_order SET work_order_no = wo_no WHERE work_order_no IS NULL;
CREATE UNIQUE INDEX uk_prod_wo_no_new ON prod_work_order(work_order_no) WHERE deleted_at IS NULL;

ALTER TABLE prod_work_order_bom
    ADD COLUMN IF NOT EXISTS plan_qty NUMERIC(18,3),
    ADD COLUMN IF NOT EXISTS issued_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS unit VARCHAR(16);
UPDATE prod_work_order_bom SET plan_qty = req_qty WHERE plan_qty IS NULL;
UPDATE prod_work_order_bom SET issued_qty = picked_qty WHERE issued_qty IS NULL;

-- 生产入库单（当前缺）
CREATE TABLE prod_production_receipt (
    id            BIGSERIAL PRIMARY KEY,
    receipt_no    VARCHAR(64) NOT NULL,
    work_order_id BIGINT NOT NULL,
    product_id    BIGINT NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    location_id   BIGINT,
    qty           NUMERIC(18,3) NOT NULL,
    cost_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    status        SMALLINT NOT NULL DEFAULT 1,
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_prod_receipt_no ON prod_production_receipt(receipt_no);
CREATE INDEX idx_prod_receipt_wo ON prod_production_receipt(work_order_id);
```

### P0-2 修正财务模块

```sql
-- 0004_fix_finance_schema.up.sql

-- 会计科目表
CREATE TABLE fin_account (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    level       SMALLINT NOT NULL,
    category    SMALLINT NOT NULL,    -- 1=资产 2=负债 3=权益 4=成本 5=收入 6=费用
    direction   SMALLINT NOT NULL,    -- 1=借 2=贷
    is_leaf     BOOLEAN NOT NULL DEFAULT TRUE,
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_account_code ON fin_account(code);

-- 重建凭证主表（对齐模型字段）
ALTER TABLE fin_voucher
    ADD COLUMN IF NOT EXISTS entry_no VARCHAR(64),
    ADD COLUMN IF NOT EXISTS summary VARCHAR(255),
    ADD COLUMN IF NOT EXISTS posted_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_by BIGINT,
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;

UPDATE fin_voucher SET entry_no = voucher_no WHERE entry_no IS NULL;

-- 凭证明细表（fin_voucher 单表不足以支持多分录）
CREATE TABLE fin_voucher_entry (
    id            BIGSERIAL PRIMARY KEY,
    voucher_id    BIGINT NOT NULL,
    line_no       SMALLINT NOT NULL,
    account_code  VARCHAR(32) NOT NULL,
    account_name  VARCHAR(64),
    debit_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    credit_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    summary       VARCHAR(255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_fin_entry_voucher ON fin_voucher_entry(voucher_id);
CREATE INDEX idx_fin_entry_account ON fin_voucher_entry(account_code);

-- 重建 fin_cost 为成本卡/成本发生
ALTER TABLE fin_cost
    ADD COLUMN IF NOT EXISTS product_id BIGINT,
    ADD COLUMN IF NOT EXISTS cost_date TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS source_type SMALLINT,
    ADD COLUMN IF NOT EXISTS source_no VARCHAR(64),
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;
-- 将 cost_type SMALLINT 保留为成本类型（1=材料 2=人工 3=制造费用），类型已匹配
-- 删除模型中不存在的 ref_id 前先迁移数据（如有）

-- 预算表（当前不存在）
CREATE TABLE fin_budget (
    id            BIGSERIAL PRIMARY KEY,
    account_code  VARCHAR(32) NOT NULL,
    period        VARCHAR(7) NOT NULL,    -- YYYY-MM
    amount        NUMERIC(18,2) NOT NULL,
    used_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_budget ON fin_budget(account_code, period, tenant_id);
```

### P0-3 库存增强

```sql
-- 0005_inventory_enhancement.up.sql

-- 台账增加幂等唯一约束
CREATE UNIQUE INDEX IF NOT EXISTS uk_inv_ledger_biz
    ON inv_stock_ledger(biz_type, biz_no, material_id, warehouse_id)
    WHERE biz_no IS NOT NULL;

-- 入库单/出库单表（当前库存变动靠台账，没有正式的出入库单据表）
CREATE TABLE inv_inbound (
    id            BIGSERIAL PRIMARY KEY,
    inbound_no    VARCHAR(64) NOT NULL,
    biz_type      SMALLINT NOT NULL,       -- 1=采购入库 2=生产入库 3=退货入库 4=调拨入库 5=盘盈
    biz_no        VARCHAR(64),
    warehouse_id  BIGINT NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 1,
    total_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(64),
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_inbound_no ON inv_inbound(inbound_no);
CREATE UNIQUE INDEX uk_inv_inbound_idem ON inv_inbound(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE inv_inbound_item (
    id           BIGSERIAL PRIMARY KEY,
    inbound_id   BIGINT NOT NULL,
    material_id  BIGINT NOT NULL,
    location_id  BIGINT,
    qty          NUMERIC(18,3) NOT NULL,
    unit_cost    NUMERIC(18,4) NOT NULL DEFAULT 0,
    batch_id     BIGINT,
    serial_no    VARCHAR(64)
);
CREATE INDEX idx_inv_inbound_item ON inv_inbound_item(inbound_id);

CREATE TABLE inv_outbound (
    id            BIGSERIAL PRIMARY KEY,
    outbound_no   VARCHAR(64) NOT NULL,
    biz_type      SMALLINT NOT NULL,       -- 1=销售出库 2=生产领料 3=退货出库 4=调拨出库 5=盘亏
    biz_no        VARCHAR(64),
    warehouse_id  BIGINT NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 1,
    total_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(64),
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_outbound_no ON inv_outbound(outbound_no);
CREATE UNIQUE INDEX uk_inv_outbound_idem ON inv_outbound(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE inv_outbound_item (
    id           BIGSERIAL PRIMARY KEY,
    outbound_id  BIGINT NOT NULL,
    material_id  BIGINT NOT NULL,
    location_id  BIGINT NOT NULL,
    qty          NUMERIC(18,3) NOT NULL,
    unit_cost    NUMERIC(18,4) NOT NULL DEFAULT 0,
    batch_id     BIGINT,
    serial_no    VARCHAR(64)
);
CREATE INDEX idx_inv_outbound_item ON inv_outbound_item(outbound_id);

-- 物料分类树
CREATE TABLE mdm_material_category (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    path        VARCHAR(255),         -- 物化路径 /1/5/12/
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_mdm_mat_cat_code ON mdm_material_category(code) WHERE deleted_at IS NULL;
```

---

## P1：补全阶段

### P1-1 IAM 与审计

```sql
-- 0006_iam_enhancement.up.sql

-- 用户表补字段
ALTER TABLE sys_user
    ADD COLUMN IF NOT EXISTS email VARCHAR(128),
    ADD COLUMN IF NOT EXISTS dept_id BIGINT,
    ADD COLUMN IF NOT EXISTS must_change_pwd BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pwd_changed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(64),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 部门
CREATE TABLE sys_department (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    path        VARCHAR(255),
    leader_id   BIGINT,
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 审计日志增加字段
ALTER TABLE sys_audit_log
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(255),
    ADD COLUMN IF NOT EXISTS status SMALLINT,
    ADD COLUMN IF NOT EXISTS duration_ms INT;
CREATE INDEX IF NOT EXISTS idx_audit_trace ON sys_audit_log(trace_id);
CREATE INDEX IF NOT EXISTS idx_audit_module_time ON sys_audit_log(module, created_at DESC);

-- 登录失败记录（用于防爆破）
CREATE TABLE sys_login_attempt (
    id           BIGSERIAL PRIMARY KEY,
    username     VARCHAR(64),
    ip           VARCHAR(64) NOT NULL,
    success      BOOLEAN NOT NULL,
    fail_reason  VARCHAR(64),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_login_attempt_ip ON sys_login_attempt(ip, created_at DESC);
CREATE INDEX idx_login_attempt_user ON sys_login_attempt(username, created_at DESC);
```

### P1-2 设备模块

```sql
-- 0007_device_enhancement.up.sql

ALTER TABLE dev_device
    ADD COLUMN IF NOT EXISTS api_key VARCHAR(128),
    ADD COLUMN IF NOT EXISTS api_secret_hash VARCHAR(128),
    ADD COLUMN IF NOT EXISTS last_ip VARCHAR(64),
    ADD COLUMN IF NOT EXISTS firmware_version VARCHAR(32),
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;

CREATE TABLE dev_device_message (
    id           BIGSERIAL PRIMARY KEY,
    device_id    BIGINT NOT NULL,
    direction    SMALLINT NOT NULL,   -- 1=上行 2=下发
    msg_type     VARCHAR(32) NOT NULL,
    payload      JSONB,
    status       SMALLINT NOT NULL DEFAULT 0,  -- 0=待处理 1=成功 2=失败
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dev_msg_device ON dev_device_message(device_id, created_at DESC);
```

### P1-3 LLM

```sql
-- 0008_llm_enhancement.up.sql

ALTER TABLE llm_session
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE TABLE llm_prompt_template (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64) NOT NULL,
    name        VARCHAR(128) NOT NULL,
    template    TEXT NOT NULL,
    variables   JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_llm_tpl_code ON llm_prompt_template(code);
```

### P1-4 单号生成器

```sql
-- 0009_seq_generator.up.sql
-- 用独立表做按日/按月单号序列（也可以直接用 PG sequence）

CREATE TABLE sys_seq (
    seq_key    VARCHAR(64) PRIMARY KEY,   -- 例如 IB:20260813
    current_val BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- service 层在事务里 SELECT ... FOR UPDATE UPDATE current_val=current_val+1 RETURNING current_val
```

---

## P2：扩展阶段

### P2-1 销售模块

```sql
-- 0010_sales.up.sql

CREATE TABLE mdm_customer (
    id             BIGSERIAL PRIMARY KEY,
    customer_code  VARCHAR(64) NOT NULL,
    name           VARCHAR(128) NOT NULL,
    contact        VARCHAR(64),
    phone          VARCHAR(32),
    address        VARCHAR(255),
    credit_limit   NUMERIC(18,2) NOT NULL DEFAULT 0,
    level          SMALLINT DEFAULT 3,
    attributes     JSONB,
    status         SMALLINT NOT NULL DEFAULT 1,
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_mdm_customer_code ON mdm_customer(customer_code) WHERE deleted_at IS NULL;

CREATE TABLE sal_sales_order (
    id             BIGSERIAL PRIMARY KEY,
    order_no       VARCHAR(64) NOT NULL,
    customer_id    BIGINT NOT NULL,
    status         SMALLINT NOT NULL DEFAULT 10,
    total_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    plan_delivery_at TIMESTAMPTZ,
    approved_at    TIMESTAMPTZ,
    approved_by    BIGINT,
    remark         TEXT,
    created_by     BIGINT,
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_sal_order_no ON sal_sales_order(order_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_sal_order_customer ON sal_sales_order(customer_id, status);

CREATE TABLE sal_sales_order_item (
    id            BIGSERIAL PRIMARY KEY,
    order_id      BIGINT NOT NULL,
    material_id   BIGINT NOT NULL,
    qty           NUMERIC(18,3) NOT NULL,
    shipped_qty   NUMERIC(18,3) NOT NULL DEFAULT 0,
    price         NUMERIC(18,2) NOT NULL,
    discount      NUMERIC(6,4) NOT NULL DEFAULT 0,
    amount        NUMERIC(18,2) NOT NULL
);
CREATE INDEX idx_sal_order_item ON sal_sales_order_item(order_id);

CREATE TABLE sal_sales_outbound (
    id            BIGSERIAL PRIMARY KEY,
    outbound_no   VARCHAR(64) NOT NULL,
    order_id      BIGINT,
    customer_id   BIGINT,
    warehouse_id  BIGINT,
    status        SMALLINT NOT NULL DEFAULT 1,
    cost_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(64),
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sal_outbound_no ON sal_sales_outbound(outbound_no);
CREATE UNIQUE INDEX uk_sal_outbound_idem ON sal_sales_outbound(idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE sal_sales_return (
    id            BIGSERIAL PRIMARY KEY,
    return_no     VARCHAR(64) NOT NULL,
    outbound_id   BIGINT,
    customer_id   BIGINT NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 1,
    total_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    reason        VARCHAR(255),
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sal_return_no ON sal_sales_return(return_no);
```

### P2-2 盘点

```sql
-- 0011_stock_check.up.sql

CREATE TABLE inv_stock_check (
    id            BIGSERIAL PRIMARY KEY,
    check_no      VARCHAR(64) NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    location_id   BIGINT,
    status        SMALLINT NOT NULL DEFAULT 10, -- 10=草稿 20=盘点中 30=待审批 40=已完成 50=已取消
    frozen        BOOLEAN NOT NULL DEFAULT FALSE,
    total_diff_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    remark        TEXT,
    approved_by   BIGINT,
    approved_at   TIMESTAMPTZ,
    created_by    BIGINT,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_check_no ON inv_stock_check(check_no);

CREATE TABLE inv_stock_check_item (
    id             BIGSERIAL PRIMARY KEY,
    check_id       BIGINT NOT NULL,
    material_id    BIGINT NOT NULL,
    location_id    BIGINT,
    batch_id       BIGINT,
    book_qty       NUMERIC(18,3) NOT NULL,
    actual_qty     NUMERIC(18,3) NOT NULL,
    diff_qty       NUMERIC(18,3) NOT NULL,
    unit_cost      NUMERIC(18,4) NOT NULL DEFAULT 0,
    diff_amount    NUMERIC(18,2) NOT NULL DEFAULT 0
);
CREATE INDEX idx_inv_check_item ON inv_stock_check_item(check_id);
```

### P2-3 调拨

```sql
-- 0012_transfer.up.sql

CREATE TABLE inv_transfer (
    id              BIGSERIAL PRIMARY KEY,
    transfer_no     VARCHAR(64) NOT NULL,
    from_warehouse_id BIGINT NOT NULL,
    to_warehouse_id BIGINT NOT NULL,
    status          SMALLINT NOT NULL DEFAULT 10, -- 10=草稿 20=已审批 30=在途 40=已完成 50=已取消
    total_amount    NUMERIC(18,2) NOT NULL DEFAULT 0,
    shipped_at      TIMESTAMPTZ,
    received_at     TIMESTAMPTZ,
    remark          TEXT,
    created_by      BIGINT,
    tenant_id       BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_transfer_no ON inv_transfer(transfer_no);

CREATE TABLE inv_transfer_item (
    id            BIGSERIAL PRIMARY KEY,
    transfer_id   BIGINT NOT NULL,
    material_id   BIGINT NOT NULL,
    qty           NUMERIC(18,3) NOT NULL,
    unit_cost     NUMERIC(18,4) NOT NULL DEFAULT 0,
    from_location_id BIGINT,
    to_location_id BIGINT
);
CREATE INDEX idx_inv_transfer_item ON inv_transfer_item(transfer_id);
```

### P2-4 批次/序列号

```sql
-- 0013_batch_serial.up.sql

CREATE TABLE inv_batch (
    id             BIGSERIAL PRIMARY KEY,
    batch_no       VARCHAR(64) NOT NULL,
    material_id    BIGINT NOT NULL,
    produced_at    DATE,
    expired_at     DATE,
    supplier_id    BIGINT,
    inbound_no     VARCHAR(64),
    qty_in         NUMERIC(18,3) NOT NULL,
    qty_remaining  NUMERIC(18,3) NOT NULL,
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_batch ON inv_batch(batch_no, material_id);
CREATE INDEX idx_inv_batch_exp ON inv_batch(expired_at) WHERE qty_remaining > 0;

CREATE TABLE inv_serial (
    id           BIGSERIAL PRIMARY KEY,
    serial_no    VARCHAR(128) NOT NULL,
    material_id  BIGINT NOT NULL,
    batch_id     BIGINT,
    status       SMALLINT NOT NULL DEFAULT 1, -- 1=在库 2=已售 3=在修 4=报废
    warehouse_id BIGINT,
    location_id  BIGINT,
    inbound_no   VARCHAR(64),
    outbound_no  VARCHAR(64),
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_inv_serial ON inv_serial(serial_no, material_id);
```

### P2-5 应收应付

```sql
-- 0014_receivable_payable.up.sql

CREATE TABLE fin_receivable (
    id            BIGSERIAL PRIMARY KEY,
    receivable_no VARCHAR(64) NOT NULL,
    customer_id   BIGINT NOT NULL,
    source_type   SMALLINT NOT NULL,     -- 1=销售出库
    source_no     VARCHAR(64),
    amount        NUMERIC(18,2) NOT NULL,
    received_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    due_at        DATE NOT NULL,
    period        VARCHAR(7) NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 1,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_recv_no ON fin_receivable(receivable_no);
CREATE INDEX idx_fin_recv_customer ON fin_receivable(customer_id, status);

CREATE TABLE fin_payable (
    id            BIGSERIAL PRIMARY KEY,
    payable_no    VARCHAR(64) NOT NULL,
    supplier_id   BIGINT NOT NULL,
    source_type   SMALLINT NOT NULL,     -- 1=采购入库
    source_no     VARCHAR(64),
    amount        NUMERIC(18,2) NOT NULL,
    paid_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    due_at        DATE NOT NULL,
    period        VARCHAR(7) NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 1,
    tenant_id     BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_pay_no ON fin_payable(payable_no);
CREATE INDEX idx_fin_pay_supplier ON fin_payable(supplier_id, status);

CREATE TABLE fin_receipt (
    id              BIGSERIAL PRIMARY KEY,
    receipt_no      VARCHAR(64) NOT NULL,
    customer_id     BIGINT NOT NULL,
    amount          NUMERIC(18,2) NOT NULL,
    receipt_date    DATE NOT NULL,
    payment_method  SMALLINT,           -- 1=现金 2=银行 3=票据
    bank_account    VARCHAR(64),
    remark          TEXT,
    created_by      BIGINT,
    tenant_id       BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_receipt_no ON fin_receipt(receipt_no);

CREATE TABLE fin_payment (
    id              BIGSERIAL PRIMARY KEY,
    payment_no      VARCHAR(64) NOT NULL,
    supplier_id     BIGINT NOT NULL,
    amount          NUMERIC(18,2) NOT NULL,
    payment_date    DATE NOT NULL,
    payment_method  SMALLINT,
    bank_account    VARCHAR(64),
    remark          TEXT,
    created_by      BIGINT,
    tenant_id       BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_payment_no ON fin_payment(payment_no);

-- 核销表（收付款与应收应付多对多）
CREATE TABLE fin_settlement (
    id             BIGSERIAL PRIMARY KEY,
    settlement_no  VARCHAR(64) NOT NULL,
    direction      SMALLINT NOT NULL,   -- 1=收 2=付
    receipt_id     BIGINT,
    payment_id     BIGINT,
    receivable_id  BIGINT,
    payable_id     BIGINT,
    amount         NUMERIC(18,2) NOT NULL,
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

---

## P3：平台化

### P3-1 多租户

```sql
-- 0015_tenant.up.sql

CREATE TABLE sys_tenant (
    id           BIGSERIAL PRIMARY KEY,
    code         VARCHAR(32) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    status       SMALLINT NOT NULL DEFAULT 1,
    config       JSONB,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sys_tenant_code ON sys_tenant(code);

INSERT INTO sys_tenant (id, code, name) VALUES (0, 'default', '默认租户');
```

### P3-2 审批工作流

```sql
-- 0016_workflow.up.sql

CREATE TABLE wf_definition (
    id             BIGSERIAL PRIMARY KEY,
    code           VARCHAR(64) NOT NULL,     -- purchase_order / sales_order
    name           VARCHAR(128) NOT NULL,
    version        INT NOT NULL DEFAULT 1,
    definition     JSONB NOT NULL,          -- 节点/连线配置
    status         SMALLINT NOT NULL DEFAULT 1,
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_wf_def_code_ver ON wf_definition(code, version);

CREATE TABLE wf_instance (
    id             BIGSERIAL PRIMARY KEY,
    instance_no    VARCHAR(64) NOT NULL,
    def_id         BIGINT NOT NULL,
    biz_type       VARCHAR(64) NOT NULL,
    biz_no         VARCHAR(64) NOT NULL,
    status         SMALLINT NOT NULL,      -- 1=审批中 2=通过 3=驳回 4=撤销
    starter_id     BIGINT NOT NULL,
    current_node   VARCHAR(64),
    tenant_id      BIGINT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_wf_inst_no ON wf_instance(instance_no);
CREATE INDEX idx_wf_inst_biz ON wf_instance(biz_type, biz_no);

CREATE TABLE wf_task (
    id           BIGSERIAL PRIMARY KEY,
    instance_id  BIGINT NOT NULL,
    node_code    VARCHAR(64) NOT NULL,
    assignee_id  BIGINT,
    assignee_role VARCHAR(64),
    status       SMALLINT NOT NULL DEFAULT 1, -- 1=待办 2=已办 3=转办
    comment      TEXT,
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at  TIMESTAMPTZ
);
CREATE INDEX idx_wf_task_assignee ON wf_task(assignee_id, status);

CREATE TABLE wf_approval (
    id           BIGSERIAL PRIMARY KEY,
    instance_id  BIGINT NOT NULL,
    task_id      BIGINT NOT NULL,
    approver_id  BIGINT NOT NULL,
    action       SMALLINT NOT NULL,       -- 1=同意 2=驳回 3=转办 4=加签
    comment      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### P3-3 消息通知

```sql
-- 0017_message.up.sql

CREATE TABLE msg_template (
    id           BIGSERIAL PRIMARY KEY,
    code         VARCHAR(64) NOT NULL,
    name         VARCHAR(128) NOT NULL,
    channel      VARCHAR(32) NOT NULL,   -- inapp/email/dingtalk/wechat/webhook
    title_tpl    VARCHAR(255),
    content_tpl  TEXT NOT NULL,
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_msg_tpl ON msg_template(code, channel);

CREATE TABLE msg_message (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    template_code VARCHAR(64),
    title        VARCHAR(255),
    content      TEXT NOT NULL,
    channel      VARCHAR(32) NOT NULL,
    is_read      BOOLEAN NOT NULL DEFAULT FALSE,
    read_at      TIMESTAMPTZ,
    biz_type     VARCHAR(64),
    biz_no       VARCHAR(64),
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_msg_user ON msg_message(user_id, is_read, created_at DESC);
```

### P3-4 文件附件

```sql
-- 0018_attachment.up.sql

CREATE TABLE sys_attachment (
    id           BIGSERIAL PRIMARY KEY,
    file_name    VARCHAR(255) NOT NULL,
    file_path    VARCHAR(512) NOT NULL,
    file_size    BIGINT NOT NULL,
    mime_type    VARCHAR(128),
    md5          VARCHAR(32),
    biz_type     VARCHAR(64),
    biz_no       VARCHAR(64),
    uploaded_by  BIGINT,
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_att_biz ON sys_attachment(biz_type, biz_no);
```

---

## 所有表共同字段规范

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | BIGSERIAL PK | 主键 |
| `tenant_id` | BIGINT NOT NULL DEFAULT 0 | 租户 ID，默认 0 |
| `created_at` | TIMESTAMPTZ DEFAULT now() | 创建时间 |
| `updated_at` | TIMESTAMPTZ DEFAULT now() | 更新时间（GORM 自动维护） |
| `deleted_at` | TIMESTAMPTZ NULL | 软删除（需要的表加） |
| `created_by` | BIGINT | 创建人 user_id |

所有金额/数量字段：
- 金额：`NUMERIC(18,2)`
- 单价/成本：`NUMERIC(18,4)`
- 数量：`NUMERIC(18,3)`
- Go 端：`decimal.Decimal`，禁止 float64
