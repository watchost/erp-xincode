-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1：状态码常量与库表 DEFAULT 最终统一（DEVELOPMENT §1.4 剩余项）。
-- 采购订单：1草稿→10、2已审批→20、5部分收货→30、3已完成→40、4已取消→50
-- 生产工单：1草稿→10、2已下达→20、3生产中→30、4已完成→40（50=已关闭由代码新增）
-- 目标值（≥10）与源值（≤5）不相交，单向 UPDATE 无碰撞。

UPDATE pur_purchase_order SET status = CASE status
    WHEN 1 THEN 10 WHEN 2 THEN 20 WHEN 5 THEN 30 WHEN 3 THEN 40 WHEN 4 THEN 50
    ELSE status END;
ALTER TABLE pur_purchase_order ALTER COLUMN status SET DEFAULT 10;

UPDATE prod_work_order SET status = CASE status
    WHEN 1 THEN 10 WHEN 2 THEN 20 WHEN 3 THEN 30 WHEN 4 THEN 40
    ELSE status END;
ALTER TABLE prod_work_order ALTER COLUMN status SET DEFAULT 10;
