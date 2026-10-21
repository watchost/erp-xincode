-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1 §2.2 MDM 补全：物料分类树、客户主数据（P2 销售模块复用）、物料单位换算字段。

-- 物料：采购/库存单位与换算率（1 采购单位 = conversion_rate 库存单位）
ALTER TABLE mdm_material
    ADD COLUMN IF NOT EXISTS purchase_unit VARCHAR(16),
    ADD COLUMN IF NOT EXISTS stock_unit VARCHAR(16),
    ADD COLUMN IF NOT EXISTS conversion_rate NUMERIC(18,4) NOT NULL DEFAULT 1;

-- 物料分类树（物化路径 /1/5/，与 sys_department 同一套写法）
CREATE TABLE mdm_material_category (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    path        VARCHAR(255),
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_mdm_mat_cat_code ON mdm_material_category(code) WHERE deleted_at IS NULL;

-- 客户主数据（P2 销售模块使用，表先建）
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

-- 新权限码并授予 admin 角色（role_id=1），与 routes.go Perm* 常量同步
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('mdm:category', '物料分类', 1, NULL, '/mdm/categories'),
    ('mdm:customer', '客户管理', 1, NULL, '/mdm/customers')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE p.code IN ('mdm:category', 'mdm:customer')
  AND NOT EXISTS (
      SELECT 1 FROM sys_role_permission rp
      WHERE rp.role_id = 1 AND rp.permission_id = p.id
  );
