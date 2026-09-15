-- 回滚：price 先按 2 位四舍五入再收窄，避免超精度值导致 ALTER 失败。
UPDATE pur_purchase_order_item SET price = round(price, 2);
ALTER TABLE pur_purchase_order_item ALTER COLUMN price TYPE NUMERIC(18,2);
