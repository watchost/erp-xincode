-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission rp
USING sys_permission p
WHERE rp.permission_id = p.id AND p.code = 'finance:account';
DELETE FROM sys_permission WHERE code = 'finance:account';

DROP TABLE IF EXISTS fin_voucher;
ALTER TABLE fin_voucher_entry DROP COLUMN IF EXISTS voucher_id;
ALTER TABLE fin_voucher_entry RENAME TO fin_voucher;

DROP TABLE IF EXISTS fin_account;
