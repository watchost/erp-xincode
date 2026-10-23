-- Copyright 2026 zhouhouping. All Rights Reserved.

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN ('workflow:task', 'workflow:definition', 'purchase:order:submit')
);
DELETE FROM sys_permission WHERE code IN ('workflow:task', 'workflow:definition', 'purchase:order:submit');

-- 回滚前把审批中的采购订单退回草稿，避免出现旧代码不认识的状态 15
UPDATE pur_purchase_order SET status = 10 WHERE status = 15;

DROP TABLE IF EXISTS wf_approval;
DROP TABLE IF EXISTS wf_task;
DROP TABLE IF EXISTS wf_instance;
DROP TABLE IF EXISTS wf_definition;
