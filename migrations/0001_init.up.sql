-- Copyright 2026 zhouhouping. All Rights Reserved.

CREATE TABLE sys_user (
    id              BIGSERIAL PRIMARY KEY,
    username        VARCHAR(64)  NOT NULL,
    password_hash   VARCHAR(128) NOT NULL,
    real_name       VARCHAR(64),
    phone           VARCHAR(32),
    status          SMALLINT     NOT NULL DEFAULT 1,
    tenant_id       BIGINT       DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sys_user_username ON sys_user(username);

CREATE TABLE sys_role (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    data_scope  SMALLINT    NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sys_role_code ON sys_role(code);

CREATE TABLE sys_permission (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(128) NOT NULL,
    name        VARCHAR(64)  NOT NULL,
    type        SMALLINT     NOT NULL,
    parent_id   BIGINT,
    path        VARCHAR(255)
);
CREATE UNIQUE INDEX uk_sys_permission_code ON sys_permission(code);

CREATE TABLE sys_role_permission (
    role_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    PRIMARY KEY(role_id, permission_id)
);

CREATE TABLE sys_user_role (
    user_id BIGINT NOT NULL,
    role_id BIGINT NOT NULL,
    PRIMARY KEY(user_id, role_id)
);

CREATE TABLE sys_audit_log (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT,
    action      VARCHAR(64)  NOT NULL,
    module      VARCHAR(32)  NOT NULL,
    ip          VARCHAR(64),
    req_param   JSONB,
    res_summary TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_user_time ON sys_audit_log(user_id, created_at DESC);

CREATE TABLE mdm_material (
    id            BIGSERIAL PRIMARY KEY,
    material_code VARCHAR(64)  NOT NULL,
    name          VARCHAR(128) NOT NULL,
    spec          VARCHAR(128),
    category_id   BIGINT,
    unit          VARCHAR(16),
    cost_method   SMALLINT     NOT NULL DEFAULT 1,
    attributes    JSONB,
    status        SMALLINT     NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_mdm_material_code ON mdm_material(material_code);
CREATE INDEX idx_material_attr ON mdm_material USING GIN(attributes);

CREATE TABLE mdm_supplier (
    id            BIGSERIAL PRIMARY KEY,
    supplier_code VARCHAR(64) NOT NULL,
    name          VARCHAR(128) NOT NULL,
    contact       VARCHAR(64),
    level         SMALLINT     DEFAULT 3,
    attributes    JSONB,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_mdm_supplier_code ON mdm_supplier(supplier_code);

CREATE TABLE mdm_warehouse (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    type        SMALLINT     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE mdm_location (
    id           BIGSERIAL PRIMARY KEY,
    warehouse_id BIGINT NOT NULL,
    location_code VARCHAR(32) NOT NULL,
    zone         VARCHAR(32),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_mdm_location ON mdm_location(warehouse_id, location_code);

CREATE TABLE pur_purchase_order (
    id            BIGSERIAL PRIMARY KEY,
    order_no      VARCHAR(64)  NOT NULL,
    supplier_id   BIGINT       NOT NULL,
    status        SMALLINT     NOT NULL DEFAULT 10,
    total_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    plan_arrive_at TIMESTAMPTZ,
    created_by    BIGINT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    approved_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_pur_order_no ON pur_purchase_order(order_no);
CREATE INDEX idx_pur_order_supplier ON pur_purchase_order(supplier_id, status);

CREATE TABLE pur_purchase_order_item (
    id            BIGSERIAL PRIMARY KEY,
    order_id      BIGINT       NOT NULL,
    material_id   BIGINT       NOT NULL,
    qty           NUMERIC(18,3) NOT NULL,
    received_qty  NUMERIC(18,3) NOT NULL DEFAULT 0,
    price         NUMERIC(18,2) NOT NULL,
    received_json JSONB
);
CREATE INDEX idx_pur_order_item ON pur_purchase_order_item(order_id);

CREATE TABLE pur_purchase_inbound (
    id           BIGSERIAL PRIMARY KEY,
    inbound_no    VARCHAR(64) NOT NULL,
    order_id      BIGINT,
    supplier_id   BIGINT,
    warehouse_id  BIGINT,
    status        SMALLINT NOT NULL DEFAULT 1,
    cost_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_by    BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_pur_inbound_no ON pur_purchase_inbound(inbound_no);

CREATE TABLE prod_work_order (
    id            BIGSERIAL PRIMARY KEY,
    wo_no         VARCHAR(64) NOT NULL,
    product_id    BIGINT NOT NULL,
    plan_qty      NUMERIC(18,3) NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 10,
    bom_id        BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_prod_wo_no ON prod_work_order(wo_no);

CREATE TABLE prod_work_order_bom (
    id           BIGSERIAL PRIMARY KEY,
    work_order_id BIGINT NOT NULL,
    material_id  BIGINT NOT NULL,
    req_qty      NUMERIC(18,3) NOT NULL,
    picked_qty   NUMERIC(18,3) NOT NULL DEFAULT 0,
    is_substitute BOOLEAN DEFAULT FALSE
);
CREATE INDEX idx_prod_wo_bom ON prod_work_order_bom(work_order_id);

CREATE TABLE prod_production_outbound (
    id           BIGSERIAL PRIMARY KEY,
    outbound_no  VARCHAR(64) NOT NULL,
    work_order_id BIGINT,
    warehouse_id BIGINT,
    cost_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inv_inventory (
    id           BIGSERIAL PRIMARY KEY,
    material_id   BIGINT NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    location_id   BIGINT,
    qty           NUMERIC(18,3) NOT NULL DEFAULT 0,
    available_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    avg_cost      NUMERIC(18,4) NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (material_id, warehouse_id, location_id)
);
CREATE INDEX idx_inv_material ON inv_inventory(material_id);

CREATE TABLE inv_stock_ledger (
    id           BIGSERIAL PRIMARY KEY,
    material_id   BIGINT NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    biz_type      SMALLINT NOT NULL,
    biz_no        VARCHAR(64),
    change_qty    NUMERIC(18,3) NOT NULL,
    after_qty     NUMERIC(18,3) NOT NULL,
    cost_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_inv_ledger_material ON inv_stock_ledger(material_id, created_at DESC);
CREATE INDEX idx_inv_ledger_biz ON inv_stock_ledger(biz_type, biz_no);

CREATE TABLE fin_voucher (
    id           BIGSERIAL PRIMARY KEY,
    voucher_no   VARCHAR(64) NOT NULL,
    biz_type     SMALLINT NOT NULL,
    biz_no       VARCHAR(64),
    debit        NUMERIC(18,2) NOT NULL DEFAULT 0,
    credit       NUMERIC(18,2) NOT NULL DEFAULT 0,
    period       VARCHAR(7)  NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_voucher_no ON fin_voucher(voucher_no);

CREATE TABLE fin_cost (
    id            BIGSERIAL PRIMARY KEY,
    cost_type     SMALLINT NOT NULL,
    ref_id        BIGINT,
    period        VARCHAR(7) NOT NULL,
    amount        NUMERIC(18,2) NOT NULL DEFAULT 0
);

CREATE TABLE dev_device (
    id            BIGSERIAL PRIMARY KEY,
    device_code   VARCHAR(64) NOT NULL,
    type          SMALLINT NOT NULL,
    brand         VARCHAR(64),
    protocol      VARCHAR(16) NOT NULL,
    status        SMALLINT NOT NULL DEFAULT 0,
    last_heartbeat TIMESTAMPTZ,
    config        JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_dev_device_code ON dev_device(device_code);
CREATE INDEX idx_device_cfg ON dev_device USING GIN(config);

CREATE TABLE llm_session (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT NOT NULL,
    title        VARCHAR(255),
    context_meta JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE llm_message (
    id           BIGSERIAL PRIMARY KEY,
    session_id   BIGINT NOT NULL,
    role         SMALLINT NOT NULL,
    content      TEXT NOT NULL,
    intent       VARCHAR(64),
    meta_json    JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_llm_msg_session ON llm_message(session_id, created_at);

INSERT INTO sys_user (username, password_hash, real_name, phone, status) VALUES
('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMye.IjzqAKL9xL5jvMFVdNJHvGCgTq/VEq', '系统管理员', '13800138000', 1);

INSERT INTO sys_role (code, name, data_scope) VALUES
('admin', '超级管理员', 1),
('manager', '管理层', 1),
('warehouse', '仓库管理员', 1),
('purchase', '采购专员', 1),
('production', '生产计划员', 1),
('finance', '财务人员', 1);

INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
('dashboard:view', '仪表页面', 1, NULL, '/dashboard'),
('warehouse:inbound', '入库作业', 1, NULL, '/warehouse/inbound'),
('warehouse:outbound', '出库作业', 1, NULL, '/warehouse/outbound'),
('warehouse:inventory', '库存管理', 1, NULL, '/warehouse/inventory'),
('purchase:order:view', '采购订单查看', 1, NULL, '/purchase/orders'),
('purchase:order:create', '采购订单创建', 2, NULL, NULL),
('purchase:order:approve', '采购订单审批', 2, NULL, NULL),
('purchase:inbound', '采购入库', 1, NULL, '/purchase/inbound'),
('production:wo:view', '生产工单查看', 1, NULL, '/production/work-orders'),
('production:wo:create', '生产工单创建', 2, NULL, NULL),
('production:outbound', '生产领料', 1, NULL, '/production/outbound'),
('finance:cost', '成本核算', 1, NULL, '/finance/cost'),
('finance:reports', '收支报表', 1, NULL, '/finance/reports'),
('mdm:material', '物料管理', 1, NULL, '/mdm/materials'),
('mdm:supplier', '供应商管理', 1, NULL, '/mdm/suppliers'),
('mdm:warehouse', '仓库管理', 1, NULL, '/mdm/warehouses'),
('iam:user', '用户管理', 1, NULL, '/iam/users'),
('iam:role', '角色管理', 1, NULL, '/iam/roles');

INSERT INTO sys_user_role (user_id, role_id) VALUES
(1, 1);

INSERT INTO sys_role_permission (role_id, permission_id) VALUES
(1, 1), (1, 2), (1, 3), (1, 4), (1, 5), (1, 6), (1, 7), (1, 8),
(1, 9), (1, 10), (1, 11), (1, 12), (1, 13), (1, 14), (1, 15), (1, 16), (1, 17);

INSERT INTO mdm_warehouse (code, name, type) VALUES
('WH001', '主仓库', 1),
('WH002', '良品仓', 2),
('WH003', '不良品仓', 3);

INSERT INTO mdm_location (warehouse_id, location_code, zone) VALUES
(1, 'A-01-01', 'A区'),
(1, 'A-01-02', 'A区'),
(1, 'A-02-01', 'A区'),
(1, 'B-01-01', 'B区'),
(2, 'GP-01-01', '良品区'),
(3, 'NG-01-01', '不良品区');
