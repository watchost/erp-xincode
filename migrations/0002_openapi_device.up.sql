-- Copyright 2026 zhouhouping. All Rights Reserved.

CREATE TABLE open_client (
    id            BIGSERIAL PRIMARY KEY,
    client_id     VARCHAR(64)  NOT NULL,
    client_secret VARCHAR(128) NOT NULL,
    name          VARCHAR(128) NOT NULL,
    status        SMALLINT     NOT NULL DEFAULT 1,
    scopes        JSONB,
    expires_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_open_client_id ON open_client(client_id);

CREATE TABLE open_token (
    id            BIGSERIAL PRIMARY KEY,
    client_id     VARCHAR(64) NOT NULL,
    access_token  VARCHAR(128) NOT NULL,
    refresh_token VARCHAR(128) NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uk_open_access_token ON open_token(access_token);
CREATE UNIQUE INDEX uk_open_refresh_token ON open_token(refresh_token);

CREATE TABLE open_webhook (
    id         BIGSERIAL PRIMARY KEY,
    client_id  VARCHAR(64) NOT NULL,
    event      VARCHAR(64) NOT NULL,
    url        VARCHAR(255) NOT NULL,
    secret     VARCHAR(128),
    status     SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_webhook_client ON open_webhook(client_id, event);

CREATE TABLE open_sync_log (
    id         BIGSERIAL PRIMARY KEY,
    client_id  VARCHAR(64) NOT NULL,
    sync_type  VARCHAR(32) NOT NULL,
    sync_data  JSONB,
    status     SMALLINT NOT NULL DEFAULT 1,
    error_msg  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sync_log_client ON open_sync_log(client_id, created_at DESC);

CREATE TABLE dev_device_log (
    id         BIGSERIAL PRIMARY KEY,
    device_id  BIGINT NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    content    JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dev_log_device ON dev_device_log(device_id, created_at DESC);

INSERT INTO open_client (client_id, client_secret, name, status, scopes) VALUES
('erp-system-open', 'erp-system-secret-zhouhouping-copyright-2026', '系统默认开放API客户端', 1, '["read","write"]'::jsonb);