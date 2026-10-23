-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P2 §3.5 应收应付与收付款核销、账龄分析模块

CREATE TABLE fin_receivable (
    id              BIGSERIAL PRIMARY KEY,
    receivable_no   VARCHAR(64)   NOT NULL,
    customer_id     BIGINT        NOT NULL,
    source_type     SMALLINT      NOT NULL DEFAULT 1, -- 1=销售出库 2=手工录入
    source_no       VARCHAR(64),
    amount          NUMERIC(18,2) NOT NULL,
    received_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    due_at          DATE          NOT NULL,
    period          VARCHAR(7)    NOT NULL,
    status          SMALLINT      NOT NULL DEFAULT 1, -- 1=待收款 2=部分收款 3=已结清 5=已取消
    remark          TEXT,
    tenant_id       BIGINT        NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_fin_recv_no ON fin_receivable(receivable_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_fin_recv_customer ON fin_receivable(customer_id, status);

CREATE TABLE fin_payable (
    id            BIGSERIAL PRIMARY KEY,
    payable_no    VARCHAR(64)   NOT NULL,
    supplier_id   BIGINT        NOT NULL,
    source_type   SMALLINT      NOT NULL DEFAULT 1, -- 1=采购入库 2=手工录入
    source_no     VARCHAR(64),
    amount        NUMERIC(18,2) NOT NULL,
    paid_amount   NUMERIC(18,2) NOT NULL DEFAULT 0,
    due_at        DATE          NOT NULL,
    period        VARCHAR(7)    NOT NULL,
    status        SMALLINT      NOT NULL DEFAULT 1, -- 1=待付款 2=部分付款 3=已结清 5=已取消
    remark        TEXT,
    tenant_id     BIGINT        NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_fin_pay_no ON fin_payable(payable_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_fin_pay_supplier ON fin_payable(supplier_id, status);

CREATE TABLE fin_receipt (
    id             BIGSERIAL PRIMARY KEY,
    receipt_no     VARCHAR(64)   NOT NULL,
    customer_id    BIGINT        NOT NULL,
    amount         NUMERIC(18,2) NOT NULL,
    receipt_date   DATE          NOT NULL,
    payment_method SMALLINT      NOT NULL DEFAULT 2, -- 1=现金 2=银行转账 3=票据
    bank_account   VARCHAR(64),
    remark         TEXT,
    created_by     BIGINT,
    tenant_id      BIGINT        NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_receipt_no ON fin_receipt(receipt_no);

CREATE TABLE fin_payment (
    id             BIGSERIAL PRIMARY KEY,
    payment_no     VARCHAR(64)   NOT NULL,
    supplier_id    BIGINT        NOT NULL,
    amount         NUMERIC(18,2) NOT NULL,
    payment_date   DATE          NOT NULL,
    payment_method SMALLINT      NOT NULL DEFAULT 2, -- 1=现金 2=银行转账 3=票据
    bank_account   VARCHAR(64),
    remark         TEXT,
    created_by     BIGINT,
    tenant_id      BIGINT        NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_fin_payment_no ON fin_payment(payment_no);

CREATE TABLE fin_settlement (
    id              BIGSERIAL PRIMARY KEY,
    settlement_no   VARCHAR(64)   NOT NULL,
    settlement_type SMALLINT      NOT NULL, -- 1=收款核销 2=付款核销
    bill_id         BIGINT        NOT NULL,
    target_id       BIGINT        NOT NULL,
    amount          NUMERIC(18,2) NOT NULL,
    created_by      BIGINT,
    created_at      TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_fin_settle_bill ON fin_settlement(settlement_type, bill_id);
CREATE INDEX idx_fin_settle_target ON fin_settlement(settlement_type, target_id);

-- 注册应收应付权限
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('finance:receivable:view',   '应收账款查看',   1, NULL, '/finance/receivables'),
    ('finance:payable:view',      '应付账款查看',   1, NULL, '/finance/payables'),
    ('finance:receipt:view',      '收款单查看',     1, NULL, '/finance/receipts'),
    ('finance:receipt:create',    '收款单创建',     2, NULL, NULL),
    ('finance:payment:view',      '付款单查看',     1, NULL, '/finance/payments'),
    ('finance:payment:create',    '付款单创建',     2, NULL, NULL),
    ('finance:settlement:create', '收付款核销',     2, NULL, NULL),
    ('finance:aging:view',        '账龄分析报表',   1, NULL, '/finance/reports/aging')
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, id FROM sys_permission WHERE code IN (
    'finance:receivable:view', 'finance:payable:view', 'finance:receipt:view', 'finance:receipt:create',
    'finance:payment:view', 'finance:payment:create', 'finance:settlement:create', 'finance:aging:view'
)
ON CONFLICT DO NOTHING;
