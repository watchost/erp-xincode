-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P2 §3.2 库存盘点 与 §3.3 库存调拨 模块
-- 状态机规范：
--   盘点：10 草稿 → 20 已冻结 → 30 已盘点待审 → 40 已完成(自动调账出入库) → 50 已取消
--   调拨：10 草稿 → 20 已审批 → 30 在途(已调出发货) → 40 已完成(已调入收货) → 50 已取消

CREATE TABLE inv_stock_check (
    id                BIGSERIAL PRIMARY KEY,
    check_no          VARCHAR(64)   NOT NULL,
    warehouse_id      BIGINT        NOT NULL,
    location_id       BIGINT,
    status            SMALLINT      NOT NULL DEFAULT 10,
    frozen            BOOLEAN       NOT NULL DEFAULT FALSE,
    total_diff_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    remark            TEXT,
    approved_by       BIGINT,
    approved_at       TIMESTAMPTZ,
    created_by        BIGINT,
    tenant_id         BIGINT        NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_inv_check_no ON inv_stock_check(check_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_inv_check_wh_status ON inv_stock_check(warehouse_id, status);

CREATE TABLE inv_stock_check_item (
    id          BIGSERIAL PRIMARY KEY,
    check_id    BIGINT        NOT NULL REFERENCES inv_stock_check(id) ON DELETE CASCADE,
    material_id BIGINT        NOT NULL,
    location_id BIGINT,
    book_qty    NUMERIC(18,3) NOT NULL,
    actual_qty  NUMERIC(18,3) NOT NULL,
    diff_qty    NUMERIC(18,3) NOT NULL,
    unit_cost   NUMERIC(18,4) NOT NULL DEFAULT 0,
    diff_amount NUMERIC(18,2) NOT NULL DEFAULT 0
);
CREATE INDEX idx_inv_check_item ON inv_stock_check_item(check_id);

CREATE TABLE inv_transfer (
    id                BIGSERIAL PRIMARY KEY,
    transfer_no       VARCHAR(64)   NOT NULL,
    from_warehouse_id BIGINT        NOT NULL,
    to_warehouse_id   BIGINT        NOT NULL,
    status            SMALLINT      NOT NULL DEFAULT 10,
    total_amount      NUMERIC(18,2) NOT NULL DEFAULT 0,
    shipped_at        TIMESTAMPTZ,
    received_at       TIMESTAMPTZ,
    remark            TEXT,
    created_by        BIGINT,
    tenant_id         BIGINT        NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_inv_transfer_no ON inv_transfer(transfer_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_inv_transfer_status ON inv_transfer(status);

CREATE TABLE inv_transfer_item (
    id               BIGSERIAL PRIMARY KEY,
    transfer_id      BIGINT        NOT NULL REFERENCES inv_transfer(id) ON DELETE CASCADE,
    material_id      BIGINT        NOT NULL,
    qty              NUMERIC(18,3) NOT NULL,
    shipped_qty      NUMERIC(18,3) NOT NULL DEFAULT 0,
    received_qty     NUMERIC(18,3) NOT NULL DEFAULT 0,
    unit_cost        NUMERIC(18,4) NOT NULL DEFAULT 0,
    total_amount     NUMERIC(18,2) NOT NULL DEFAULT 0,
    from_location_id BIGINT,
    to_location_id   BIGINT
);
CREATE INDEX idx_inv_transfer_item ON inv_transfer_item(transfer_id);

-- 注册盘点与调拨权限并授予 admin (role_id=1) 和 warehouse (role_id=2)
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('warehouse:check:view',     '库存盘点查看', 1, NULL, '/warehouse/stock-checks'),
    ('warehouse:check:create',   '库存盘点创建', 2, NULL, NULL),
    ('warehouse:check:freeze',   '库存盘点冻结', 2, NULL, NULL),
    ('warehouse:check:entry',    '库存盘点录入', 2, NULL, NULL),
    ('warehouse:check:approve',  '库存盘点审批', 2, NULL, NULL),
    ('warehouse:check:cancel',   '库存盘点取消', 2, NULL, NULL),
    ('warehouse:transfer:view',    '库存调拨查看', 1, NULL, '/warehouse/transfers'),
    ('warehouse:transfer:create',  '库存调拨创建', 2, NULL, NULL),
    ('warehouse:transfer:approve', '库存调拨审批', 2, NULL, NULL),
    ('warehouse:transfer:ship',    '库存调拨发货', 2, NULL, NULL),
    ('warehouse:transfer:receive', '库存调拨收货', 2, NULL, NULL),
    ('warehouse:transfer:cancel',  '库存调拨取消', 2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN (
    'warehouse:check:view', 'warehouse:check:create', 'warehouse:check:freeze', 'warehouse:check:entry', 'warehouse:check:approve', 'warehouse:check:cancel',
    'warehouse:transfer:view', 'warehouse:transfer:create', 'warehouse:transfer:approve', 'warehouse:transfer:ship', 'warehouse:transfer:receive', 'warehouse:transfer:cancel'
)
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM sys_role r, sys_permission p
WHERE r.code = 'warehouse' AND p.code IN (
    'warehouse:check:view', 'warehouse:check:create', 'warehouse:check:freeze', 'warehouse:check:entry', 'warehouse:check:approve', 'warehouse:check:cancel',
    'warehouse:transfer:view', 'warehouse:transfer:create', 'warehouse:transfer:approve', 'warehouse:transfer:ship', 'warehouse:transfer:receive', 'warehouse:transfer:cancel'
)
ON CONFLICT DO NOTHING;
