-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1.5 S1：工单累计领料成本/已结转入库成本，完工入库成本改为从 5001 余额按比例结转。

ALTER TABLE prod_work_order
    ADD COLUMN IF NOT EXISTS issued_cost NUMERIC(18,2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS received_cost NUMERIC(18,2) NOT NULL DEFAULT 0;
