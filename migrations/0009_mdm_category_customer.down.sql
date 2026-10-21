-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission rp
USING sys_permission p
WHERE rp.permission_id = p.id AND p.code IN ('mdm:category', 'mdm:customer');
DELETE FROM sys_permission WHERE code IN ('mdm:category', 'mdm:customer');

DROP TABLE IF EXISTS mdm_customer;
DROP TABLE IF EXISTS mdm_material_category;

ALTER TABLE mdm_material
    DROP COLUMN IF EXISTS purchase_unit,
    DROP COLUMN IF EXISTS stock_unit,
    DROP COLUMN IF EXISTS conversion_rate;
