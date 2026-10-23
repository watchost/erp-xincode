-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN (
        'warehouse:check:view', 'warehouse:check:create', 'warehouse:check:freeze', 'warehouse:check:entry', 'warehouse:check:approve', 'warehouse:check:cancel',
        'warehouse:transfer:view', 'warehouse:transfer:create', 'warehouse:transfer:approve', 'warehouse:transfer:ship', 'warehouse:transfer:receive', 'warehouse:transfer:cancel'
    )
);

DELETE FROM sys_permission WHERE code IN (
    'warehouse:check:view', 'warehouse:check:create', 'warehouse:check:freeze', 'warehouse:check:entry', 'warehouse:check:approve', 'warehouse:check:cancel',
    'warehouse:transfer:view', 'warehouse:transfer:create', 'warehouse:transfer:approve', 'warehouse:transfer:ship', 'warehouse:transfer:receive', 'warehouse:transfer:cancel'
);

DROP TABLE IF EXISTS inv_transfer_item;
DROP TABLE IF EXISTS inv_transfer;
DROP TABLE IF EXISTS inv_stock_check_item;
DROP TABLE IF EXISTS inv_stock_check;
