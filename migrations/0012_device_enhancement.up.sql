-- Copyright 2026 zhouhouping. All Rights Reserved.
-- P1 §2.6 设备模块：WebSocket 接入鉴权凭据与消息流水表。

ALTER TABLE dev_device
    ADD COLUMN IF NOT EXISTS api_key VARCHAR(128),
    ADD COLUMN IF NOT EXISTS api_secret_hash VARCHAR(128),
    ADD COLUMN IF NOT EXISTS last_ip VARCHAR(64),
    ADD COLUMN IF NOT EXISTS firmware_version VARCHAR(32),
    ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ;

CREATE UNIQUE INDEX uk_dev_device_api_key ON dev_device(api_key) WHERE api_key IS NOT NULL;

CREATE TABLE dev_device_message (
    id           BIGSERIAL PRIMARY KEY,
    message_id   VARCHAR(64) NOT NULL,
    device_id    BIGINT NOT NULL,
    direction    SMALLINT NOT NULL,        -- 1=上行 2=下发
    msg_type     VARCHAR(32) NOT NULL,     -- heartbeat/scan/rfid_read/ack/command/print/online/offline
    payload      JSONB,
    status       SMALLINT NOT NULL DEFAULT 0,  -- 0=待处理 1=成功 2=失败
    tenant_id    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    acked_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_dev_message_id ON dev_device_message(message_id);
CREATE INDEX idx_dev_msg_device ON dev_device_message(device_id, created_at DESC);
CREATE INDEX idx_dev_msg_pending ON dev_device_message(status, created_at) WHERE direction = 2 AND status = 0;
