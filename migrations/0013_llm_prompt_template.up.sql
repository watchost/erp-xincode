-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1 §2.7 LLM：Prompt 模板管理。

CREATE TABLE llm_prompt_template (
    id         BIGSERIAL PRIMARY KEY,
    code       VARCHAR(64) NOT NULL,
    name       VARCHAR(128) NOT NULL,
    scene      VARCHAR(32) NOT NULL DEFAULT 'chat',  -- chat=对话系统提示 dashboard=仪表盘分析
    content    TEXT NOT NULL,
    enabled    SMALLINT NOT NULL DEFAULT 1,
    tenant_id  BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_llm_prompt_code ON llm_prompt_template(code);

INSERT INTO llm_prompt_template (code, name, scene, content) VALUES
    ('chat_system', '对话系统提示', 'chat', '你是ERP系统的智能分析助手，帮助用户进行业务数据分析和决策支持。回答基于事实，不要编造数据。'),
    ('dashboard_analysis', '仪表盘分析提示', 'dashboard', '你是 ERP 系统的智能分析助手，基于提供的真实经营数据给出简洁、专业的分析与建议，不要编造数据。')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('llm:prompt', 'Prompt模板管理', 1, NULL, '/llm/prompts')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE p.code IN ('llm:prompt')
  AND NOT EXISTS (
      SELECT 1 FROM sys_role_permission rp
      WHERE rp.role_id = 1 AND rp.permission_id = p.id
  );
