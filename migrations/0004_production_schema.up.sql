-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P0：对齐生产模块 Go 模型与数据库结构（审计 P0-6）。
-- 原 migration 中工单表为 wo_no/plan_qty/status(默认10)/bom_id，缺少
-- produced_qty/plan_start_at/plan_end_at/actual_*/created_by；
-- 工单物料表为 req_qty/picked_qty；prod_bom/prod_bom_item 完全缺失。
-- 本迁移把结构对齐到 internal/production/model。

-- 1. prod_work_order：补齐列，重命名 wo_no -> work_order_no。
ALTER TABLE prod_work_order
    RENAME COLUMN wo_no TO work_order_no;

ALTER TABLE prod_work_order
    ALTER COLUMN status DROP DEFAULT;

ALTER TABLE prod_work_order
    ADD COLUMN IF NOT EXISTS produced_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS plan_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS plan_end_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_start_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS actual_end_at   TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS created_by    BIGINT;

-- 旧默认值 10 与 Go 常量（1 草稿/2 下达/3 生产中/4 完成）不一致，把历史数据归一为草稿。
UPDATE prod_work_order SET status = 1 WHERE status = 10;
ALTER TABLE prod_work_order ALTER COLUMN status SET DEFAULT 1;

-- 2. prod_work_order_bom：重命名列并补 unit。
ALTER TABLE prod_work_order_bom
    RENAME COLUMN req_qty TO plan_qty;
ALTER TABLE prod_work_order_bom
    RENAME COLUMN picked_qty TO issued_qty;
ALTER TABLE prod_work_order_bom
    ADD COLUMN IF NOT EXISTS unit VARCHAR(16);

-- 3. 新建 BOM 表。
CREATE TABLE IF NOT EXISTS prod_bom (
    id              BIGSERIAL PRIMARY KEY,
    product_id      BIGINT NOT NULL,
    bom_version     VARCHAR(32) NOT NULL,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    effective_start TIMESTAMPTZ NOT NULL DEFAULT now(),
    effective_end   TIMESTAMPTZ,
    created_by      BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_prod_bom_version ON prod_bom(product_id, bom_version);
CREATE INDEX IF NOT EXISTS idx_prod_bom_product ON prod_bom(product_id, is_active);

CREATE TABLE IF NOT EXISTS prod_bom_item (
    id           BIGSERIAL PRIMARY KEY,
    bom_id       BIGINT NOT NULL,
    material_id  BIGINT NOT NULL,
    qty          NUMERIC(18,4) NOT NULL,
    unit         VARCHAR(16) NOT NULL,
    scrap_rate   NUMERIC(8,4) NOT NULL DEFAULT 0,
    sequence     INT NOT NULL DEFAULT 0,
    CONSTRAINT fk_prod_bom_item_bom FOREIGN KEY (bom_id) REFERENCES prod_bom(id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_prod_bom_item_bom ON prod_bom_item(bom_id, sequence);

-- 4. 生产入库单（P1 会用到，先把表建出来避免后续模型/SQL 报错）。
CREATE TABLE IF NOT EXISTS prod_production_receipt (
    id            BIGSERIAL PRIMARY KEY,
    receipt_no    VARCHAR(64) NOT NULL,
    work_order_id BIGINT NOT NULL,
    material_id   BIGINT NOT NULL,
    warehouse_id  BIGINT NOT NULL,
    qty           NUMERIC(18,3) NOT NULL,
    unit_cost     NUMERIC(18,4) NOT NULL DEFAULT 0,
    status        SMALLINT NOT NULL DEFAULT 1,
    created_by    BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS uk_prod_receipt_no ON prod_production_receipt(receipt_no);
CREATE INDEX IF NOT EXISTS idx_prod_receipt_wo ON prod_production_receipt(work_order_id);
