-- Copyright 2026 zhouhouping. All Rights Reserved.
-- 回滚 P2 §3.1 销售模块。

DELETE FROM sys_role_permission
WHERE permission_id IN (
    SELECT id FROM sys_permission
    WHERE code IN ('sal:order:view', 'sal:order:create', 'sal:order:approve',
                   'sal:order:cancel', 'sal:outbound', 'sal:return')
);
DELETE FROM sys_permission
WHERE code IN ('sal:order:view', 'sal:order:create', 'sal:order:approve',
               'sal:order:cancel', 'sal:outbound', 'sal:return');

DROP TABLE IF EXISTS sal_sales_return_item;
DROP TABLE IF EXISTS sal_sales_return;
DROP TABLE IF EXISTS sal_sales_outbound_item;
DROP TABLE IF EXISTS sal_sales_outbound;
DROP TABLE IF EXISTS sal_sales_order_item;
DROP TABLE IF EXISTS sal_sales_order;
