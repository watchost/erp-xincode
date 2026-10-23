-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P2 §3.4 批次与序列号管理模块
-- 支持食品/医药/电子等高精溯源品类的批次全生命周期、FEFO(先到期先出)推荐与序列号状态流转。

CREATE TABLE inv_batch (
    id             BIGSERIAL PRIMARY KEY,
    batch_no       VARCHAR(64)   NOT NULL,
    material_id    BIGINT        NOT NULL,
    produced_at    DATE,
    expired_at     DATE,
    supplier_id    BIGINT,
    inbound_no     VARCHAR(64),
    qty_in         NUMERIC(18,3) NOT NULL,
    qty_remaining  NUMERIC(18,3) NOT NULL,
    tenant_id      BIGINT        NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_inv_batch ON inv_batch(batch_no, material_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_inv_batch_exp ON inv_batch(material_id, expired_at) WHERE qty_remaining > 0;

CREATE TABLE inv_serial (
    id           BIGSERIAL PRIMARY KEY,
    serial_no    VARCHAR(128) NOT NULL,
    material_id  BIGINT       NOT NULL,
    batch_id     BIGINT,
    status       SMALLINT     NOT NULL DEFAULT 1, -- 1=在库 2=已售 3=在修 4=报废
    warehouse_id BIGINT,
    location_id  BIGINT,
    inbound_no   VARCHAR(64),
    outbound_no  VARCHAR(64),
    tenant_id    BIGINT       NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_inv_serial ON inv_serial(serial_no, material_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_inv_serial_status ON inv_serial(material_id, status);

-- 注册批次与序列号权限
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('warehouse:batch:view',     '批次查看与追溯', 1, NULL, '/warehouse/batches'),
    ('warehouse:batch:create',   '批次创建与入库', 2, NULL, NULL),
    ('warehouse:serial:view',    '序列号查看',     1, NULL, '/warehouse/serials'),
    ('warehouse:serial:create',  '序列号录入',     2, NULL, NULL),
    ('warehouse:serial:update',  '序列号状态变更', 2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN (
    'warehouse:batch:view', 'warehouse:batch:create', 'warehouse:serial:view', 'warehouse:serial:create', 'warehouse:serial:update'
)
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM sys_role r, sys_permission p
WHERE r.code = 'warehouse' AND p.code IN (
    'warehouse:batch:view', 'warehouse:batch:create', 'warehouse:serial:view', 'warehouse:serial:create', 'warehouse:serial:update'
)
ON CONFLICT DO NOTHING;
