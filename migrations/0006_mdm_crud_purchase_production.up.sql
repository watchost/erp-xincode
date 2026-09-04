-- Copyright 2026 zhouhouping. All Rights Reserved.
-- 收口主链路：MDM 主数据写操作（软删除 + 物料扩展字段）、采购订单审批人。
--   * mdm_material/mdm_supplier/mdm_warehouse/mdm_location 增加 deleted_at 软删除列，
--     唯一索引改为部分唯一索引（WHERE deleted_at IS NULL），软删后编码可复用；
--   * 补 mdm_warehouse(code) 唯一索引（原先缺失）；
--   * mdm_material 增加库存/批次管理字段（min_stock 替换库存预警硬编码，batch/serial 为 P2 预留）；
--   * pur_purchase_order 增加 approved_by；
--   * 补充权限码：生产入库 production:receipt、采购订单取消 purchase:order:cancel。

-- 1. MDM 软删除列
ALTER TABLE mdm_material  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE mdm_supplier  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE mdm_warehouse ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;
ALTER TABLE mdm_location  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 2. 物料库存/批次管理字段
ALTER TABLE mdm_material
    ADD COLUMN IF NOT EXISTS min_stock       NUMERIC(18,3),
    ADD COLUMN IF NOT EXISTS max_stock       NUMERIC(18,3),
    ADD COLUMN IF NOT EXISTS batch_managed   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS serial_managed  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS shelf_life_days INT;

-- 3. 采购订单审批人
ALTER TABLE pur_purchase_order ADD COLUMN IF NOT EXISTS approved_by BIGINT;

-- 4. 唯一索引改为部分唯一索引（软删行不参与唯一性约束）
DROP INDEX IF EXISTS uk_mdm_material_code;
CREATE UNIQUE INDEX uk_mdm_material_code ON mdm_material(material_code) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uk_mdm_supplier_code;
CREATE UNIQUE INDEX uk_mdm_supplier_code ON mdm_supplier(supplier_code) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uk_mdm_location;
CREATE UNIQUE INDEX uk_mdm_location ON mdm_location(warehouse_id, location_code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_mdm_warehouse_code ON mdm_warehouse(code) WHERE deleted_at IS NULL;

-- 5. 补充权限码并授予 admin 角色（role_id=1）
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('production:receipt',    '生产入库/报工', 2, NULL, NULL),
    ('purchase:order:cancel', '采购订单取消',  2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE NOT EXISTS (
    SELECT 1 FROM sys_role_permission rp
    WHERE rp.role_id = 1 AND rp.permission_id = p.id
);
