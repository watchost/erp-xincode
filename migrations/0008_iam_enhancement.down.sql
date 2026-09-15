-- 0008 回滚：注意会移除审计追踪列与数据权限列，仅用于开发/测试环境回退。

DELETE FROM sys_role_permission
WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN ('iam:dept', 'iam:audit:view')
);
DELETE FROM sys_permission WHERE code IN ('iam:dept', 'iam:audit:view');

DROP INDEX IF EXISTS idx_prod_wo_dept;
DROP INDEX IF EXISTS idx_pur_order_dept;
ALTER TABLE prod_work_order DROP COLUMN IF EXISTS dept_id;
ALTER TABLE pur_purchase_order DROP COLUMN IF EXISTS dept_id;

DROP TABLE IF EXISTS sys_login_attempt;
DROP INDEX IF EXISTS idx_audit_module_time;
DROP INDEX IF EXISTS idx_audit_trace;
ALTER TABLE sys_audit_log
    DROP COLUMN IF EXISTS duration_ms,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS trace_id;

DROP TABLE IF EXISTS sys_role_dept;
DROP TABLE IF EXISTS sys_department;

DROP INDEX IF EXISTS idx_sys_user_dept;
DROP INDEX IF EXISTS uk_sys_user_username_alive;
CREATE UNIQUE INDEX uk_sys_user_username ON sys_user(username);
ALTER TABLE sys_user
    DROP COLUMN IF EXISTS deleted_at,
    DROP COLUMN IF EXISTS last_login_ip,
    DROP COLUMN IF EXISTS last_login_at,
    DROP COLUMN IF EXISTS pwd_changed_at,
    DROP COLUMN IF EXISTS must_change_pwd,
    DROP COLUMN IF EXISTS dept_id,
    DROP COLUMN IF EXISTS email;
