-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P0：对齐财务模块 Go 模型与数据库结构（审计 P0-7 / P0-8 的 schema 部分）。
-- 旧 fin_cost / fin_voucher 的列名与类型与 internal/finance/model 完全不符：
--   * fin_cost.cost_type 是 smallint，而模型 FinCostCard.CostType 为 varchar
--     （material/labor/overhead），且缺 product_id / cost_date / source_type / source_id；
--   * fin_voucher 用 voucher_no/debit/credit，而模型 FinAccountEntry 需要
--     entry_no/account_code/account_name/debit_amount/credit_amount/balance/description；
--   * fin_budget 表缺失。
-- 这两张表此前从未被写入（凭证/成本卡的生成逻辑尚未接入业务流，审计 P0-8），
-- 故直接 DROP 重建以匹配模型；fin_budget 新建。

-- 1. 成本卡 fin_cost（对应 model.FinCostCard）
DROP TABLE IF EXISTS fin_cost;
CREATE TABLE fin_cost (
    id          BIGSERIAL    PRIMARY KEY,
    product_id  BIGINT       NOT NULL,
    cost_type   VARCHAR(32)  NOT NULL,              -- material / labor / overhead
    amount      NUMERIC(18,4) NOT NULL DEFAULT 0,
    cost_date   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    source_type VARCHAR(32),
    source_id   BIGINT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_fin_cost_product  ON fin_cost(product_id);
CREATE INDEX idx_fin_cost_date     ON fin_cost(cost_date);
CREATE INDEX idx_fin_cost_type_date ON fin_cost(cost_type, cost_date);

-- 2. 会计分录 fin_voucher（对应 model.FinAccountEntry，当前为扁平分录表）
DROP TABLE IF EXISTS fin_voucher;
CREATE TABLE fin_voucher (
    id            BIGSERIAL     PRIMARY KEY,
    entry_no      VARCHAR(64)   NOT NULL,
    account_code  VARCHAR(32)   NOT NULL,
    account_name  VARCHAR(128),
    debit_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    credit_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    balance       NUMERIC(18,2) NOT NULL DEFAULT 0,
    biz_type      VARCHAR(32),
    biz_no        VARCHAR(64),
    description   VARCHAR(255),
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_voucher_entry_no ON fin_voucher(entry_no);
CREATE INDEX idx_fin_voucher_account ON fin_voucher(account_code);
CREATE INDEX idx_fin_voucher_biz_no  ON fin_voucher(biz_no);
CREATE INDEX idx_fin_voucher_created ON fin_voucher(created_at);

-- 3. 预算 fin_budget（对应 model.FinBudget）
CREATE TABLE IF NOT EXISTS fin_budget (
    id          BIGSERIAL     PRIMARY KEY,
    budget_no   VARCHAR(64)   NOT NULL,
    budget_type VARCHAR(32)   NOT NULL,             -- purchase / production / marketing / admin
    amount      NUMERIC(18,2) NOT NULL DEFAULT 0,
    used_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    year        INT           NOT NULL,
    month       INT           NOT NULL,
    status      SMALLINT      NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_budget_no     ON fin_budget(budget_no);
CREATE UNIQUE INDEX uk_fin_budget_period ON fin_budget(budget_type, year, month);
