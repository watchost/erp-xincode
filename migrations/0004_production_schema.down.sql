-- Copyright 2026 zhouhouping. All Rights Reserved.
DROP TABLE IF EXISTS prod_production_receipt;
DROP TABLE IF EXISTS prod_bom_item;
DROP TABLE IF EXISTS prod_bom;

ALTER TABLE prod_work_order_bom
    DROP COLUMN IF EXISTS unit;
ALTER TABLE prod_work_order_bom
    RENAME COLUMN issued_qty TO picked_qty;
ALTER TABLE prod_work_order_bom
    RENAME COLUMN plan_qty TO req_qty;

ALTER TABLE prod_work_order
    DROP COLUMN IF EXISTS produced_qty,
    DROP COLUMN IF EXISTS plan_start_at,
    DROP COLUMN IF EXISTS plan_end_at,
    DROP COLUMN IF EXISTS actual_start_at,
    DROP COLUMN IF EXISTS actual_end_at,
    DROP COLUMN IF EXISTS created_by;
ALTER TABLE prod_work_order
    RENAME COLUMN work_order_no TO wo_no;
ALTER TABLE prod_work_order
    ALTER COLUMN status SET DEFAULT 10;
