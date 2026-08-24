-- Copyright 2026 zhouhouping. All Rights Reserved.
DELETE FROM sys_role_permission
WHERE role_id = 1
  AND permission_id IN (
      SELECT id FROM sys_permission
      WHERE code IN ('dashboard:llm','finance:budget','mdm:location','device:manage','llm:chat','openapi:admin')
  );
DELETE FROM sys_permission
WHERE code IN ('dashboard:llm','finance:budget','mdm:location','device:manage','llm:chat','openapi:admin');
