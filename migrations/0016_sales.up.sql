-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P2 §3.1 销售模块：销售订单/出库/退货（客户主数据 mdm_customer 已在 0009 建好）。
-- 状态码与采购对称：10 草稿 → 20 已审批 → 30 部分发货 → 40 已完成 → 50 已取消。

CREATE TABLE sal_sales_order (
    id               BIGSERIAL PRIMARY KEY,
    order_no         VARCHAR(64)  NOT NULL,
    customer_id      BIGINT       NOT NULL,
    status           SMALLINT     NOT NULL DEFAULT 10,
    total_amount     NUMERIC(18,2) NOT NULL DEFAULT 0,
    discount_amount  NUMERIC(18,2) NOT NULL DEFAULT 0,
    plan_delivery_at TIMESTAMPTZ,
    approved_at      TIMESTAMPTZ,
    approved_by      BIGINT,
    remark           TEXT,
    created_by       BIGINT,
    dept_id          BIGINT       NOT NULL DEFAULT 0,
    tenant_id        BIGINT       NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at       TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_sal_order_no ON sal_sales_order(order_no) WHERE deleted_at IS NULL;
CREATE INDEX idx_sal_order_customer ON sal_sales_order(customer_id, status);

CREATE TABLE sal_sales_order_item (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT        NOT NULL REFERENCES sal_sales_order(id) ON DELETE CASCADE,
    material_id BIGINT        NOT NULL,
    qty         NUMERIC(18,3) NOT NULL,
    shipped_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    price       NUMERIC(18,2) NOT NULL,
    discount    NUMERIC(6,4)  NOT NULL DEFAULT 0,
    amount      NUMERIC(18,2) NOT NULL
);
CREATE INDEX idx_sal_order_item ON sal_sales_order_item(order_id);

CREATE TABLE sal_sales_outbound (
    id              BIGSERIAL PRIMARY KEY,
    outbound_no     VARCHAR(64)  NOT NULL,
    order_id        BIGINT,
    customer_id     BIGINT,
    warehouse_id    BIGINT,
    status          SMALLINT     NOT NULL DEFAULT 1,
    cost_amount     NUMERIC(18,2) NOT NULL DEFAULT 0,
    idempotency_key VARCHAR(64),
    created_by      BIGINT,
    tenant_id       BIGINT       NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sal_outbound_no ON sal_sales_outbound(outbound_no);
CREATE UNIQUE INDEX uk_sal_outbound_idem ON sal_sales_outbound(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX idx_sal_outbound_order ON sal_sales_outbound(order_id);

-- 出库明细：unit_cost 为发货时点移动加权平均成本（退货按此成本回填库存）。
CREATE TABLE sal_sales_outbound_item (
    id           BIGSERIAL PRIMARY KEY,
    outbound_id  BIGINT        NOT NULL REFERENCES sal_sales_outbound(id) ON DELETE CASCADE,
    material_id  BIGINT        NOT NULL,
    qty          NUMERIC(18,3) NOT NULL,
    returned_qty NUMERIC(18,3) NOT NULL DEFAULT 0,
    unit_cost    NUMERIC(18,4) NOT NULL DEFAULT 0,
    cost_amount  NUMERIC(18,2) NOT NULL DEFAULT 0
);
CREATE INDEX idx_sal_outbound_item ON sal_sales_outbound_item(outbound_id);

CREATE TABLE sal_sales_return (
    id           BIGSERIAL PRIMARY KEY,
    return_no    VARCHAR(64)  NOT NULL,
    outbound_id  BIGINT,
    customer_id  BIGINT       NOT NULL,
    warehouse_id BIGINT       NOT NULL,
    status       SMALLINT     NOT NULL DEFAULT 10, -- 10 待入库 → 20 已入库 → 50 已取消
    total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
    reason       VARCHAR(255),
    created_by   BIGINT,
    tenant_id    BIGINT       NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_sal_return_no ON sal_sales_return(return_no);

CREATE TABLE sal_sales_return_item (
    id               BIGSERIAL PRIMARY KEY,
    return_id        BIGINT        NOT NULL REFERENCES sal_sales_return(id) ON DELETE CASCADE,
    outbound_item_id BIGINT        NOT NULL,
    material_id      BIGINT        NOT NULL,
    qty              NUMERIC(18,3) NOT NULL,
    price            NUMERIC(18,2) NOT NULL,
    amount           NUMERIC(18,2) NOT NULL
);
CREATE INDEX idx_sal_return_item ON sal_sales_return_item(return_id);

-- 新权限码并授予 admin 角色（role_id=1），与 routes.go Perm* 常量同步
INSERT INTO sys_permission (code, name, type, parent_id, path) VALUES
    ('sal:order:view',    '销售订单查看', 1, NULL, '/sales/orders'),
    ('sal:order:create',  '销售订单创建', 2, NULL, NULL),
    ('sal:order:approve', '销售订单审批', 2, NULL, NULL),
    ('sal:order:cancel',  '销售订单取消', 2, NULL, NULL),
    ('sal:outbound',      '销售出库',     2, NULL, NULL),
    ('sal:return',        '销售退货',     2, NULL, NULL)
ON CONFLICT (code) DO NOTHING;

INSERT INTO sys_role_permission (role_id, permission_id)
SELECT 1, p.id
FROM sys_permission p
WHERE p.code IN ('sal:order:view', 'sal:order:create', 'sal:order:approve',
                 'sal:order:cancel', 'sal:outbound', 'sal:return')
  AND NOT EXISTS (
      SELECT 1 FROM sys_role_permission rp
      WHERE rp.role_id = 1 AND rp.permission_id = p.id
  );
