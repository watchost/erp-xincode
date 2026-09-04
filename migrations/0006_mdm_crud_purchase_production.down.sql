-- Copyright 2026 zhouhouping. All Rights Reserved.
-- 回滚 0006：恢复唯一索引为非部分索引、移除新增列与权限码。

DROP INDEX IF EXISTS uk_mdm_warehouse_code;

DROP INDEX IF EXISTS uk_mdm_location;
CREATE UNIQUE INDEX uk_mdm_location ON mdm_location(warehouse_id, location_code);

DROP INDEX IF EXISTS uk_mdm_supplier_code;
CREATE UNIQUE INDEX uk_mdm_supplier_code ON mdm_supplier(supplier_code);

DROP INDEX IF EXISTS uk_mdm_material_code;
CREATE UNIQUE INDEX uk_mdm_material_code ON mdm_material(material_code);

ALTER TABLE pur_purchase_order DROP COLUMN IF EXISTS approved_by;

ALTER TABLE mdm_material
    DROP COLUMN IF EXISTS min_stock,
    DROP COLUMN IF EXISTS max_stock,
    DROP COLUMN IF EXISTS batch_managed,
    DROP COLUMN IF EXISTS serial_managed,
    DROP COLUMN IF EXISTS shelf_life_days;

ALTER TABLE mdm_material  DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE mdm_supplier  DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE mdm_warehouse DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE mdm_location  DROP COLUMN IF EXISTS deleted_at;

DELETE FROM sys_role_permission
WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN ('production:receipt', 'purchase:order:cancel')
);
DELETE FROM sys_permission WHERE code IN ('production:receipt', 'purchase:order:cancel');
