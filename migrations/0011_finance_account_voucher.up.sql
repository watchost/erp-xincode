-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1 §2.5 财务补全：
--   1. fin_account 会计科目表（树形，1xxx-6xxx，含常用科目种子）；
--   2. 凭证拆分主表/分录表：原 fin_voucher（扁平分录行）改名为 fin_voucher_entry 并补 voucher_id，
--      新建 fin_voucher 凭证主表（凭证号/会计期间/借贷合计），历史数据按凭证号回填；
--   3. 期末结转防重：period_close 凭证对 biz_no（期间）加部分唯一索引；
--   4. 新增 finance:account 权限码并授予 admin。

-- 1. 会计科目表
CREATE TABLE fin_account (
    id          BIGSERIAL PRIMARY KEY,
    code        VARCHAR(32) NOT NULL,
    name        VARCHAR(64) NOT NULL,
    parent_id   BIGINT,
    level       SMALLINT NOT NULL,
    category    SMALLINT NOT NULL,    -- 1=资产 2=负债 3=权益 4=成本 5=收入 6=费用
    direction   SMALLINT NOT NULL,    -- 1=借 2=贷
    is_leaf     BOOLEAN NOT NULL DEFAULT TRUE,
    tenant_id   BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_account_code ON fin_account(code);

INSERT INTO fin_account (code, name, parent_id, level, category, direction, is_leaf) VALUES
    ('1001', '库存现金',     NULL, 1, 1, 1, TRUE),
    ('1002', '银行存款',     NULL, 1, 1, 1, TRUE),
    ('1122', '应收账款',     NULL, 1, 1, 1, TRUE),
    ('1403', '原材料',       NULL, 1, 1, 1, TRUE),
    ('1405', '库存商品',     NULL, 1, 1, 1, TRUE),
    ('2202', '应付账款',     NULL, 1, 2, 2, TRUE),
    ('4001', '实收资本',     NULL, 1, 3, 2, TRUE),
    ('4103', '本年利润',     NULL, 1, 3, 2, TRUE),
    ('5001', '生产成本',     NULL, 1, 4, 1, TRUE),
    ('5101', '制造费用',     NULL, 1, 4, 1, TRUE),
    ('6001', '主营业务收入', NULL, 1, 5, 2, TRUE),
    ('6401', '主营业务成本', NULL, 1, 6, 1, TRUE),
    ('6601', '销售费用',     NULL, 1, 6, 1, TRUE),
    ('6602', '管理费用',     NULL, 1, 6, 1, TRUE),
    ('6603', '财务费用',     NULL, 1, 6, 1, TRUE)
ON CONFLICT (code) DO NOTHING;

-- 2. 凭证主表/分录表拆分
ALTER TABLE fin_voucher RENAME TO fin_voucher_entry;
ALTER TABLE fin_voucher_entry ADD COLUMN voucher_id BIGINT;

CREATE TABLE fin_voucher (
    id           BIGSERIAL PRIMARY KEY,
    voucher_no   VARCHAR(64) NOT NULL,
    period       VARCHAR(7)  NOT NULL,   -- 会计期间 YYYY-MM
    summary      VARCHAR(255),
    biz_type     VARCHAR(32),
    biz_no       VARCHAR(64),
    debit_total  NUMERIC(18,2) NOT NULL DEFAULT 0,
    credit_total NUMERIC(18,2) NOT NULL DEFAULT 0,
    posted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_by   BIGINT,
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_voucher_no ON fin_voucher(voucher_no);
CREATE INDEX idx_fin_voucher_period ON fin_voucher(period);
CREATE INDEX idx_fin_voucher_biz ON fin_voucher(biz_type, biz_no);

-- 历史数据回填：entry_no 形如 <凭证号>-NN，按凭证号聚合出主表行
INSERT INTO fin_voucher (voucher_no, period, summary, biz_type, biz_no, debit_total, credit_total, posted_at, created_at)
SELECT left(entry_no, length(entry_no) - 3),
       to_char(min(created_at), 'YYYY-MM'),
       max(description),
       max(biz_type),
       max(biz_no),
       sum(debit_amount),
       sum(credit_amount),
       min(created_at),
       min(created_at)
FROM fin_voucher_entry
GROUP BY left(entry_no, length(entry_no) - 3);

UPDATE fin_voucher_entry e
SET voucher_id = v.id
FROM fin_voucher v
WHERE left(e.entry_no, length(e.entry_no) - 3) = v.voucher_no;

ALTER TABLE fin_voucher_entry ALTER COLUMN voucher_id SET NOT NULL;
CREATE INDEX idx_fin_entry_voucher ON fin_voucher_entry(voucher_id);
CREATE INDEX idx_fin_entry_account ON fin_voucher_entry(account_code);

-- 3. 期末结转一个期间只能结转一次
CREATE UNIQUE INDEX uk_fin_voucher_period_close ON fin_voucher(biz_no) WHERE biz_type = 'period_close';

-- 4. 权限码
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('finance:account', '科目与凭证管理', 1, NULL, '/finance/accounts')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE p.code = 'finance:account'
  AND NOT EXISTS (
      SELECT 1 FROM sys_role_permission rp
      WHERE rp.role_id = 1 AND rp.permission_id = p.id
  );
