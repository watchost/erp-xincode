-- Copyright 2026 zhouhouping. All Rights Reserved.
-- 注意：回滚会把唯一约束恢复为全局唯一；若已有多个租户使用相同编码，回滚会失败，需先清理非 0 租户数据。

DELETE FROM sys_role_permission WHERE permission_id IN (
    SELECT id FROM sys_permission WHERE code IN ('platform:tenant', 'tenant:config', 'message:view')
);
DELETE FROM sys_permission WHERE code IN ('platform:tenant', 'tenant:config', 'message:view');

DROP TABLE IF EXISTS msg_subscription;
DROP TABLE IF EXISTS msg_message;

DROP INDEX IF EXISTS uk_mdm_material_code;
CREATE UNIQUE INDEX uk_mdm_material_code ON mdm_material(material_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_supplier_code;
CREATE UNIQUE INDEX uk_mdm_supplier_code ON mdm_supplier(supplier_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_warehouse_code;
CREATE UNIQUE INDEX uk_mdm_warehouse_code ON mdm_warehouse(code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_mat_cat_code;
CREATE UNIQUE INDEX uk_mdm_mat_cat_code ON mdm_material_category(code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_mdm_customer_code;
CREATE UNIQUE INDEX uk_mdm_customer_code ON mdm_customer(customer_code) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_sys_department_code;
CREATE UNIQUE INDEX uk_sys_department_code ON sys_department(code);
DROP INDEX IF EXISTS uk_pur_order_no;
CREATE UNIQUE INDEX uk_pur_order_no ON pur_purchase_order(order_no);
DROP INDEX IF EXISTS uk_pur_inbound_no;
CREATE UNIQUE INDEX uk_pur_inbound_no ON pur_purchase_inbound(inbound_no);
DROP INDEX IF EXISTS uk_prod_wo_no;
CREATE UNIQUE INDEX uk_prod_wo_no ON prod_work_order(wo_no);
DROP INDEX IF EXISTS uk_prod_receipt_no;
CREATE UNIQUE INDEX uk_prod_receipt_no ON prod_production_receipt(receipt_no);
DROP INDEX IF EXISTS uk_sal_order_no;
CREATE UNIQUE INDEX uk_sal_order_no ON sal_sales_order(order_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_sal_outbound_no;
CREATE UNIQUE INDEX uk_sal_outbound_no ON sal_sales_outbound(outbound_no);
DROP INDEX IF EXISTS uk_sal_return_no;
CREATE UNIQUE INDEX uk_sal_return_no ON sal_sales_return(return_no);
DROP INDEX IF EXISTS uk_inv_check_no;
CREATE UNIQUE INDEX uk_inv_check_no ON inv_stock_check(check_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_inv_transfer_no;
CREATE UNIQUE INDEX uk_inv_transfer_no ON inv_transfer(transfer_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_fin_account_code;
CREATE UNIQUE INDEX uk_fin_account_code ON fin_account(code);
DROP INDEX IF EXISTS uk_fin_voucher_no;
CREATE UNIQUE INDEX uk_fin_voucher_no ON fin_voucher(voucher_no);
DROP INDEX IF EXISTS uk_fin_voucher_period_close;
CREATE UNIQUE INDEX uk_fin_voucher_period_close ON fin_voucher(biz_no) WHERE biz_type = 'period_close';
DROP INDEX IF EXISTS uk_fin_budget_no;
CREATE UNIQUE INDEX uk_fin_budget_no ON fin_budget(budget_no);
DROP INDEX IF EXISTS uk_fin_budget_period;
CREATE UNIQUE INDEX uk_fin_budget_period ON fin_budget(budget_type, year, month);
DROP INDEX IF EXISTS uk_fin_recv_no;
CREATE UNIQUE INDEX uk_fin_recv_no ON fin_receivable(receivable_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_fin_pay_no;
CREATE UNIQUE INDEX uk_fin_pay_no ON fin_payable(payable_no) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS uk_fin_receipt_no;
CREATE UNIQUE INDEX uk_fin_receipt_no ON fin_receipt(receipt_no);
DROP INDEX IF EXISTS uk_fin_payment_no;
CREATE UNIQUE INDEX uk_fin_payment_no ON fin_payment(payment_no);
DROP INDEX IF EXISTS uk_wf_def_code_ver;
CREATE UNIQUE INDEX uk_wf_def_code_ver ON wf_definition(code, version);
DROP INDEX IF EXISTS uk_wf_def_deployed;
CREATE UNIQUE INDEX uk_wf_def_deployed ON wf_definition(code) WHERE status = 1;
DROP INDEX IF EXISTS uk_wf_inst_running;
CREATE UNIQUE INDEX uk_wf_inst_running ON wf_instance(biz_type, biz_no) WHERE status = 1;
DROP INDEX IF EXISTS uk_llm_prompt_code;
CREATE UNIQUE INDEX uk_llm_prompt_code ON llm_prompt_template(code);

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
            EXECUTE format('ALTER TABLE %I DROP COLUMN IF EXISTS tenant_id', t);
        END IF;
    END LOOP;
END $$;

DROP TABLE IF EXISTS sys_tenant;
