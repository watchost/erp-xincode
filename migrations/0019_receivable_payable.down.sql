-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN (
        'finance:receivable:view', 'finance:payable:view', 'finance:receipt:view', 'finance:receipt:create',
        'finance:payment:view', 'finance:payment:create', 'finance:settlement:create', 'finance:aging:view'
    )
);

DELETE FROM sys_permission WHERE code IN (
    'finance:receivable:view', 'finance:payable:view', 'finance:receipt:view', 'finance:receipt:create',
    'finance:payment:view', 'finance:payment:create', 'finance:settlement:create', 'finance:aging:view'
);

DROP TABLE IF EXISTS fin_settlement;
DROP TABLE IF EXISTS fin_payment;
DROP TABLE IF EXISTS fin_receipt;
DROP TABLE IF EXISTS fin_payable;
DROP TABLE IF EXISTS fin_receivable;
