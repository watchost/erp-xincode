-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P0: 补充路由权限码并授予 admin 角色。
-- 与 internal/routes/routes.go 中的 Perm* 常量保持同步。

INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('dashboard:llm',       '仪表智能分析', 2, NULL, NULL),
    ('finance:budget',      '预算查看',     1, NULL, '/finance/budgets'),
    ('mdm:location',        '库位管理',     1, NULL, '/mdm/locations'),
    ('device:manage',       '设备管理',     1, NULL, '/device'),
    ('llm:chat',            '智能助手',     1, NULL, '/llm'),
    ('openapi:admin',       'OpenAPI 管理', 1, NULL, '/openapi')
ON CONFLICT (code) DO NOTHING;

-- 将所有权限授予 admin 角色（role_id=1）。
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE NOT EXISTS (
    SELECT 1 FROM sys_role_permission rp
    WHERE rp.role_id = 1 AND rp.permission_id = p.id
);
