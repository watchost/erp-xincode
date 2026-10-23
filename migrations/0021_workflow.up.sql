-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P3 §4.1 审批工作流：流程定义 / 实例 / 待办任务 / 审批记录

CREATE TABLE wf_definition (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL,          -- 业务类型，如 purchase_order
    name        VARCHAR(128) NOT NULL,
    version     INT          NOT NULL DEFAULT 1,
    definition  JSONB        NOT NULL,          -- {"nodes":[...]}，见 docs/development/features/workflow.md
    status      SMALLINT     NOT NULL DEFAULT 0, -- 0=草稿 1=已部署（同 code 仅一个版本为 1） 2=已停用
    tenant_id   BIGINT       NOT NULL DEFAULT 0,
    created_by  BIGINT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_wf_def_code_ver ON wf_definition(code, version);
-- 同一业务类型同一时间只能有一个已部署版本
CREATE UNIQUE INDEX uk_wf_def_deployed ON wf_definition(code) WHERE status = 1;

CREATE TABLE wf_instance (
    id            BIGSERIAL PRIMARY KEY,
    instance_no   VARCHAR(64) NOT NULL,
    def_id        BIGINT      NOT NULL,
    biz_type      VARCHAR(64) NOT NULL,
    biz_no        VARCHAR(64) NOT NULL,
    variables     JSONB,                         -- 启动时的业务变量（用于节点条件，如 amount）
    status        SMALLINT    NOT NULL,          -- 1=审批中 2=通过 3=驳回 4=撤销
    starter_id    BIGINT      NOT NULL,
    current_node  VARCHAR(64),
    tenant_id     BIGINT      NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_wf_inst_no ON wf_instance(instance_no);
CREATE INDEX idx_wf_inst_biz ON wf_instance(biz_type, biz_no);
-- 一个单据同一时间只能有一个运行中的实例
CREATE UNIQUE INDEX uk_wf_inst_running ON wf_instance(biz_type, biz_no) WHERE status = 1;

CREATE TABLE wf_task (
    id             BIGSERIAL PRIMARY KEY,
    instance_id    BIGINT      NOT NULL,
    node_code      VARCHAR(64) NOT NULL,
    node_name      VARCHAR(128),
    sign_mode      VARCHAR(8)  NOT NULL DEFAULT 'any', -- any=或签 all=会签
    assignee_id    BIGINT      NOT NULL,
    status         SMALLINT    NOT NULL DEFAULT 1,      -- 1=待办 2=已通过 3=已转办 4=已驳回 5=已取消（或签他人已处理/流程结束）
    source_task_id BIGINT,                               -- 加签/转办来源任务
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at    TIMESTAMPTZ
);
CREATE INDEX idx_wf_task_todo ON wf_task(assignee_id, status);
CREATE INDEX idx_wf_task_inst ON wf_task(instance_id, node_code);

CREATE TABLE wf_approval (
    id           BIGSERIAL PRIMARY KEY,
    instance_id  BIGINT      NOT NULL,
    task_id      BIGINT,
    node_code    VARCHAR(64),
    operator_id  BIGINT      NOT NULL,
    action       VARCHAR(16) NOT NULL,  -- start/approve/reject/transfer/add_sign/cancel
    comment      TEXT,
    target_id    BIGINT,                -- 转办/加签的目标用户
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_wf_approval_inst ON wf_approval(instance_id, created_at);

-- 采购订单新增「审批中」状态 15（草稿 10 → 提交 15 → 工作流通过后 20 已审批）
-- 状态列为 SMALLINT，无 CHECK 约束，无需改表。

-- 默认流程：采购订单 2 级审批（部门负责人 → 管理层或签）
INSERT INTO wf_definition (code, name, version, definition, status) VALUES
('purchase_order', '采购订单审批', 1,
 '{"nodes":[
   {"code":"start","type":"start"},
   {"code":"dept_leader","type":"approval","name":"部门负责人审批","assignee":{"type":"dept_leader"},"sign_mode":"any"},
   {"code":"manager","type":"approval","name":"管理层审批","assignee":{"type":"role","value":"manager"},"sign_mode":"any"},
   {"code":"end","type":"end"}
 ]}'::jsonb, 1)
ON CONFLICT (code, version) DO NOTHING;

-- 权限
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('workflow:task',       '审批待办处理', 2, NULL, NULL),
    ('workflow:definition', '流程定义管理', 2, NULL, '/workflow/definitions'),
    ('purchase:order:submit', '采购订单提交审批', 2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN ('workflow:task', 'workflow:definition', 'purchase:order:submit')
ON CONFLICT DO NOTHING;

-- 审批人需要看到自己的待办：所有现有角色都授予 workflow:task（是否真有待办由任务分配决定）
INSERT INTO sys_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM sys_role r, sys_permission p
WHERE p.code = 'workflow:task'
ON CONFLICT DO NOTHING;
