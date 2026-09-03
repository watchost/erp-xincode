-- Copyright 2026 zhouhouping. All Rights Reserved.
-- 回滚 0005_finance_schema：删除对齐后的财务表（fin_budget 为新建，fin_cost/fin_voucher 重建）。
DROP TABLE IF EXISTS fin_budget;
DROP TABLE IF EXISTS fin_voucher;
DROP TABLE IF EXISTS fin_cost;
