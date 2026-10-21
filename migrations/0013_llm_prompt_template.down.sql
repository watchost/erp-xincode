-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission
WHERE permission_id IN (SELECT id FROM sys_permission WHERE code = 'llm:prompt');
DELETE FROM sys_permission WHERE code = 'llm:prompt';

DROP TABLE IF EXISTS llm_prompt_template;
