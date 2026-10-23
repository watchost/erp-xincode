-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P3 §4.2–§4.4：多租户、审计、站内消息
--   1. sys_tenant 租户表（id=0 为默认/平台租户，现有数据全部归属租户 0）
--   2. 给缺少 tenant_id 的业务表补列（NOT NULL DEFAULT 0，现有数据自动归入租户 0）
--   3. 业务编码/单号唯一约束改为按租户唯一（索引名保持不变，代码中有按名称识别的冲突翻译）
--   4. 站内消息 msg_message / msg_subscription
--   5. 安全库存沿用已有 mdm_material.min_stock（低于时发站内信）
--   6. 权限

-- 1. 租户 ------------------------------------------------------------------
CREATE TABLE sys_tenant (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(64)  NOT NULL,
    name        VARCHAR(128) NOT NULL,
    status      SMALLINT     NOT NULL DEFAULT 1,   -- 1=启用 0=停用
    expire_at   TIMESTAMPTZ,                       -- NULL=不过期；过期后禁止登录和写操作
    config      JSONB        NOT NULL DEFAULT '{}'::jsonb, -- 租户级配置：logo/currency/period_rule/plan 等
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sys_tenant_code ON sys_tenant(code);
INSERT INTO sys_tenant (id, code, name, config) VALUES (0, 'default', '默认租户', '{"currency":"CNY"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- 2. 补 tenant_id -----------------------------------------------------------
DO $$
DECLARE t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'mdm_material','mdm_supplier','mdm_warehouse','mdm_location',
        'inv_inventory','inv_stock_ledger','inv_stock_check_item','inv_transfer_item',
        'pur_purchase_order','pur_purchase_order_item','pur_purchase_inbound',
        'prod_bom','prod_bom_item','prod_work_order','prod_work_order_bom',
        'prod_production_receipt','prod_production_outbound',
        'sal_sales_order_item','sal_sales_outbound_item','sal_sales_return_item',
        'fin_budget','fin_cost','fin_settlement',
        'wf_task','wf_approval',
        'llm_session','llm_message','dev_device_log','open_client'
    ] LOOP
        IF to_regclass(t) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0', t);
            EXECUTE format('CREATE INDEX IF NOT EXISTS %I ON %I(tenant_id)', 'idx_' || t || '_tenant', t);
        END IF;
    END LOOP;
END $$;

-- 3. 按租户唯一 --------------------------------------------------------------
DROP INDEX IF EXISTS uk_mdm_material_code;
CREATE UNIQUE INDEX uk_mdm_material_code ON mdm_material(tenant_id, material_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_supplier_code;
CREATE UNIQUE INDEX uk_mdm_supplier_code ON mdm_supplier(tenant_id, supplier_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_warehouse_code;
CREATE UNIQUE INDEX uk_mdm_warehouse_code ON mdm_warehouse(tenant_id, code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_mat_cat_code;
CREATE UNIQUE INDEX uk_mdm_mat_cat_code ON mdm_material_category(tenant_id, code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_customer_code;
CREATE UNIQUE INDEX uk_mdm_customer_code ON mdm_customer(tenant_id, customer_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_sys_department_code;
CREATE UNIQUE INDEX uk_sys_department_code ON sys_department(tenant_id, code);

DROP INDEX IF EXISTS uk_pur_order_no;
CREATE UNIQUE INDEX uk_pur_order_no ON pur_purchase_order(tenant_id, order_no);
DROP INDEX IF EXISTS uk_pur_inbound_no;
CREATE UNIQUE INDEX uk_pur_inbound_no ON pur_purchase_inbound(tenant_id, inbound_no);
DROP INDEX IF EXISTS uk_prod_wo_no;
CREATE UNIQUE INDEX uk_prod_wo_no ON prod_work_order(tenant_id, wo_no);
DROP INDEX IF EXISTS uk_prod_receipt_no;
CREATE UNIQUE INDEX uk_prod_receipt_no ON prod_production_receipt(tenant_id, receipt_no);
DROP INDEX IF EXISTS uk_sal_order_no;
CREATE UNIQUE INDEX uk_sal_order_no ON sal_sales_order(tenant_id, order_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_sal_outbound_no;
CREATE UNIQUE INDEX uk_sal_outbound_no ON sal_sales_outbound(tenant_id, outbound_no);
DROP INDEX IF EXISTS uk_sal_return_no;
CREATE UNIQUE INDEX uk_sal_return_no ON sal_sales_return(tenant_id, return_no);
DROP INDEX IF EXISTS uk_inv_check_no;
CREATE UNIQUE INDEX uk_inv_check_no ON inv_stock_check(tenant_id, check_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_inv_transfer_no;
CREATE UNIQUE INDEX uk_inv_transfer_no ON inv_transfer(tenant_id, transfer_no) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS uk_fin_account_code;
CREATE UNIQUE INDEX uk_fin_account_code ON fin_account(tenant_id, code);
DROP INDEX IF EXISTS uk_fin_voucher_no;
CREATE UNIQUE INDEX uk_fin_voucher_no ON fin_voucher(tenant_id, voucher_no);
DROP INDEX IF EXISTS uk_fin_voucher_period_close;
CREATE UNIQUE INDEX uk_fin_voucher_period_close ON fin_voucher(tenant_id, biz_no) WHERE biz_type = 'period_close';
DROP INDEX IF EXISTS uk_fin_budget_no;
CREATE UNIQUE INDEX uk_fin_budget_no ON fin_budget(tenant_id, budget_no);
DROP INDEX IF EXISTS uk_fin_budget_period;
CREATE UNIQUE INDEX uk_fin_budget_period ON fin_budget(tenant_id, budget_type, year, month);
DROP INDEX IF EXISTS uk_fin_recv_no;
CREATE UNIQUE INDEX uk_fin_recv_no ON fin_receivable(tenant_id, receivable_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_fin_pay_no;
CREATE UNIQUE INDEX uk_fin_pay_no ON fin_payable(tenant_id, payable_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_fin_receipt_no;
CREATE UNIQUE INDEX uk_fin_receipt_no ON fin_receipt(tenant_id, receipt_no);
DROP INDEX IF EXISTS uk_fin_payment_no;
CREATE UNIQUE INDEX uk_fin_payment_no ON fin_payment(tenant_id, payment_no);

DROP INDEX IF EXISTS uk_wf_def_code_ver;
CREATE UNIQUE INDEX uk_wf_def_code_ver ON wf_definition(tenant_id, code, version);
DROP INDEX IF EXISTS uk_wf_def_deployed;
CREATE UNIQUE INDEX uk_wf_def_deployed ON wf_definition(tenant_id, code) WHERE status = 1;
DROP INDEX IF EXISTS uk_wf_inst_running;
CREATE UNIQUE INDEX uk_wf_inst_running ON wf_instance(tenant_id, biz_type, biz_no) WHERE status = 1;
DROP INDEX IF EXISTS uk_llm_prompt_code;
CREATE UNIQUE INDEX uk_llm_prompt_code ON llm_prompt_template(tenant_id, code);
-- 保持全局唯一（有意不改）：sys_user.username（登录不带租户）、dev_device.device_code / api_key、
-- 幂等键、消息 ID、wf_instance.instance_no、open_client.client_id、token。

-- 4. 站内消息 ----------------------------------------------------------------
CREATE TABLE msg_message (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL DEFAULT 0,
    receiver_id     BIGINT       NOT NULL,
    category        VARCHAR(32)  NOT NULL,          -- approval_todo / stock_low / system
    title           VARCHAR(200) NOT NULL,
    content         TEXT,
    biz_type        VARCHAR(64),
    biz_no          VARCHAR(64),
    link            VARCHAR(255),                   -- 前端跳转路径
    is_read         BOOLEAN      NOT NULL DEFAULT FALSE,
    read_at         TIMESTAMPTZ,
    dispatch_status SMALLINT     NOT NULL DEFAULT 0, -- 外部渠道：0=无需派发 1=待派发 2=已处理（结果见 channel_result）
    channel_result  JSONB,                          -- {"webhook":"sent","email":"skipped: 未配置 SMTP",...}
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_msg_receiver ON msg_message(tenant_id, receiver_id, is_read, created_at DESC);
CREATE INDEX idx_msg_dispatch ON msg_message(dispatch_status) WHERE dispatch_status = 1;

CREATE TABLE msg_subscription (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT      NOT NULL DEFAULT 0,
    user_id     BIGINT      NOT NULL,
    category    VARCHAR(32) NOT NULL,               -- 同 msg_message.category
    channel     VARCHAR(16) NOT NULL,               -- email / dingtalk / wecom / webhook（站内信始终发送，不需订阅）
    target      VARCHAR(500),                       -- 邮箱地址或机器人/Webhook URL
    enabled     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_msg_sub ON msg_subscription(tenant_id, user_id, category, channel);

-- 5. 安全库存：沿用已有的 mdm_material.min_stock 作为安全库存（NULL=不预警），不新增列。

-- 6. 权限 -------------------------------------------------------------------
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('platform:tenant', '平台租户管理', 2, NULL, '/admin/tenants'),
    ('tenant:config',   '本租户配置',   2, NULL, NULL),
    ('message:view',    '站内消息',     2, NULL, '/messages')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN ('platform:tenant', 'tenant:config', 'message:view')
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT r.id, p.id FROM sys_role r, sys_permission p WHERE p.code = 'message:view'
ON CONFLICT DO NOTHING;
