-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P2 缺陷修复：应收单/应付单手工创建为写操作，需独立创建权限（0019 已发布，不得原地修改）

INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('finance:receivable:create', '应收单创建', 2, NULL, NULL),
    ('finance:payable:create',    '应付单创建', 2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN ('finance:receivable:create', 'finance:payable:create')
ON CONFLICT DO NOTHING;

-- PDA 手持端需要读取仓库/库位/物料主数据；warehouse 角色此前未授权，仅仓库角色账号会被拒绝。
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT r.id, p.id
FROM sys_role r, sys_permission p
WHERE r.code = 'warehouse' AND p.code IN ('mdm:warehouse', 'mdm:location', 'mdm:material')
ON CONFLICT DO NOTHING;
