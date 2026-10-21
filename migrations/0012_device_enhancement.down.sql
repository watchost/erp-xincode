-- Copyright 2026 zhouhouping. All Rights Reserved.

DROP TABLE IF EXISTS dev_device_message;

ALTER TABLE dev_device
    DROP COLUMN IF EXISTS api_key,
    DROP COLUMN IF EXISTS api_secret_hash,
    DROP COLUMN IF EXISTS last_ip,
    DROP COLUMN IF EXISTS firmware_version,
    DROP COLUMN IF EXISTS tenant_id,
    DROP COLUMN IF EXISTS updated_at;
