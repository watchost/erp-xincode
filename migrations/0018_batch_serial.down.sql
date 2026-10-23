-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN (
        'warehouse:batch:view', 'warehouse:batch:create', 'warehouse:serial:view', 'warehouse:serial:create', 'warehouse:serial:update'
    )
);

DELETE FROM sys_permission WHERE code IN (
    'warehouse:batch:view', 'warehouse:batch:create', 'warehouse:serial:view', 'warehouse:serial:create', 'warehouse:serial:update'
);

DROP TABLE IF EXISTS inv_serial;
DROP TABLE IF EXISTS inv_batch;
