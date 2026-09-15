-- 采购单价按约定（docs/development/00-conventions.md §5）加宽到 NUMERIC(18,4)，
-- 支持分以下单价（如 0.025 元/件）。纯加宽，不丢精度。
ALTER TABLE pur_purchase_order_item ALTER COLUMN price TYPE NUMERIC(18,4);
