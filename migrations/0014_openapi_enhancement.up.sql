-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1 §2.8 OpenAPI：token scope/吊销、webhook 投递表、client_secret 迁移为 bcrypt。

ALTER TABLE open_token
    ADD COLUMN IF NOT EXISTS scope VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS revoked SMALLINT NOT NULL DEFAULT 0;

CREATE TABLE open_webhook_delivery (
    id            BIGSERIAL PRIMARY KEY,
    webhook_id    BIGINT NOT NULL,
    event         VARCHAR(64) NOT NULL,
    payload       JSONB,
    status        SMALLINT NOT NULL DEFAULT 0,  -- 0=待投递 1=成功 2=死信(重试耗尽)
    attempts      INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at  TIMESTAMPTZ
);
CREATE INDEX idx_webhook_delivery_pending ON open_webhook_delivery(status, next_retry_at) WHERE status = 0;
CREATE INDEX idx_webhook_delivery_hook ON open_webhook_delivery(webhook_id, created_at DESC);

-- 存量明文 secret → bcrypt（pgcrypto 的 crypt/gen_salt('bf') 与 golang bcrypt 兼容）。
CREATE EXTENSION IF NOT EXISTS pgcrypto;
UPDATE open_client
SET client_secret = crypt(client_secret, gen_salt('bf'))
WHERE client_secret NOT LIKE '$2$%';
