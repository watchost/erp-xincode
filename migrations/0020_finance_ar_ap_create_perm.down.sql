-- Copyright 2026 zhouhouping. All Rights Reserved.

-- 仅撤销本迁移新增的 warehouse 角色授权（0001 起 admin 已拥有 mdm:* 权限，不动）
DELETE FROM sys_role_permission
WHERE role_id IN (SELECT id FROM sys_role WHERE code = 'warehouse')
  AND permission_id IN (SELECT id FROM sys_permission WHERE code IN ('mdm:warehouse', 'mdm:location', 'mdm:material'));

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN ('finance:receivable:create', 'finance:payable:create')
);
DELETE FROM sys_permission WHERE code IN ('finance:receivable:create', 'finance:payable:create');
