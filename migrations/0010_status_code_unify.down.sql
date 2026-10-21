-- Copyright 2026 zhouhouping. All Rights Reserved.

UPDATE pur_purchase_order SET status = CASE status
    WHEN 10 THEN 1 WHEN 20 THEN 2 WHEN 30 THEN 5 WHEN 40 THEN 3 WHEN 50 THEN 4
    ELSE status END;
ALTER TABLE pur_purchase_order ALTER COLUMN status SET DEFAULT 10;

UPDATE prod_work_order SET status = CASE status
    WHEN 10 THEN 1 WHEN 20 THEN 2 WHEN 30 THEN 3 WHEN 40 THEN 4
    ELSE status END;
ALTER TABLE prod_work_order ALTER COLUMN status SET DEFAULT 1;
