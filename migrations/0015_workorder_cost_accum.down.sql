-- Copyright 2026 zhouhouping. All Rights Reserved.

ALTER TABLE prod_work_order
    DROP COLUMN IF EXISTS issued_cost,
    DROP COLUMN IF EXISTS received_cost;
