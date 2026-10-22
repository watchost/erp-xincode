-- Copyright 2026 zhouhouping. All Rights Reserved.

DROP TABLE IF EXISTS open_webhook_delivery;

ALTER TABLE open_token
    DROP COLUMN IF EXISTS scope,
    DROP COLUMN IF EXISTS revoked;

-- bcrypt 不可逆：down 仅能把种子客户端恢复为初始明文 secret。
UPDATE open_client
SET client_secret = 'erp-system-secret-zhouhouping-copyright-2026'
WHERE client_id = 'erp-system-open';
