-- 0008 IAM 完整化（P1 §2.1）：部门树、数据权限、软删除、审计字段、登录尝试。

-- 用户表补字段
ALTER TABLE sys_user
    ADD COLUMN IF NOT EXISTS email VARCHAR(128),
    ADD COLUMN IF NOT EXISTS dept_id BIGINT,
    ADD COLUMN IF NOT EXISTS must_change_pwd BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pwd_changed_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS last_login_ip VARCHAR(64),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- 用户名唯一索引改为部分索引：软删除后同名账号可以重建
DROP INDEX IF EXISTS uk_sys_user_username;
CREATE UNIQUE INDEX uk_sys_user_username_alive ON sys_user(username) WHERE deleted_at IS NULL;
CREATE INDEX idx_sys_user_dept ON sys_user(dept_id) WHERE deleted_at IS NULL;

-- 部门树（path 为物化路径，形如 /1/3/）
CREATE TABLE IF NOT EXISTS sys_department (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    path        VARCHAR(255) NOT NULL DEFAULT '/',
    leader_id   BIGINT,
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_sys_department_code ON sys_department(code);
CREATE INDEX IF NOT EXISTS idx_sys_department_parent ON sys_department(parent_id);
CREATE INDEX IF NOT EXISTS idx_sys_department_path ON sys_department(path);

-- 自定义数据权限（data_scope=5）的角色-部门关联
CREATE TABLE IF NOT EXISTS sys_role_dept (
    role_id BIGINT NOT NULL,
    dept_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, dept_id)
);

-- 审计日志补字段
ALTER TABLE sys_audit_log
    ADD COLUMN IF NOT EXISTS trace_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS user_agent VARCHAR(255),
    ADD COLUMN IF NOT EXISTS status SMALLINT,
    ADD COLUMN IF NOT EXISTS duration_ms INT;
CREATE INDEX IF NOT EXISTS idx_audit_trace ON sys_audit_log(trace_id);
CREATE INDEX IF NOT EXISTS idx_audit_module_time ON sys_audit_log(module, created_at DESC);

-- 登录尝试流水（账号锁定判定以 Redis 计数为准，本表用于审计/追踪）
CREATE TABLE IF NOT EXISTS sys_login_attempt (
    id          BIGSERIAL PRIMARY KEY,
    username    VARCHAR(64),
    ip          VARCHAR(64) NOT NULL,
    success     BOOLEAN NOT NULL,
    fail_reason VARCHAR(64),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_login_attempt_ip ON sys_login_attempt(ip, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_login_attempt_user ON sys_login_attempt(username, created_at DESC);

-- 业务单据补 dept_id，供数据权限 scope 2/3/5 过滤
ALTER TABLE pur_purchase_order ADD COLUMN IF NOT EXISTS dept_id BIGINT;
ALTER TABLE prod_work_order ADD COLUMN IF NOT EXISTS dept_id BIGINT;
CREATE INDEX IF NOT EXISTS idx_pur_order_dept ON pur_purchase_order(dept_id);
CREATE INDEX IF NOT EXISTS idx_prod_wo_dept ON prod_work_order(dept_id);

-- 新权限码：部门管理、审计日志查看
INSERT INTO sys_permission (code, name, type, parent_id, path)
VALUES
    ('iam:dept', '部门管理', 1, NULL, '/iam/departments'),
    ('iam:audit:view', '审计日志查看', 1, NULL, '/iam/audit-logs')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id FROM sys_permission p
WHERE p.code IN ('iam:dept', 'iam:audit:view')
  AND NOT EXISTS (
      SELECT 1 FROM sys_role_permission rp WHERE rp.role_id = 1 AND rp.permission_id = p.id
  );
