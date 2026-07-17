// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

type SysUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	RealName     string    `json:"real_name"`
	Phone        string    `json:"phone"`
	Status       int       `json:"status"`
	TenantID     int64     `json:"tenant_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SysRole struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	DataScope int       `json:"data_scope"`
	CreatedAt time.Time `json:"created_at"`
}

type SysPermission struct {
	ID        int64     `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Type      int       `json:"type"`
	ParentID  int64     `json:"parent_id"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type SysRolePermission struct {
	RoleID       int64 `json:"role_id"`
	PermissionID int64 `json:"permission_id"`
}

type SysUserRole struct {
	UserID int64 `json:"user_id"`
	RoleID int64 `json:"role_id"`
}

type SysAuditLog struct {
	ID          int64           `json:"id"`
	UserID      int64           `json:"user_id"`
	Action      string          `json:"action"`
	Module      string          `json:"module"`
	IP          string          `json:"ip"`
	ReqParam    JSON            `json:"req_param"`
	ResSummary  string          `json:"res_summary"`
	CreatedAt   time.Time       `json:"created_at"`
}

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j *JSON) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}
	result := json.RawMessage{}
	err := json.Unmarshal(bytes, &result)
	*j = JSON(result)
	return err
}

func (SysUser) TableName() string { return "sys_user" }
func (SysRole) TableName() string { return "sys_role" }
func (SysPermission) TableName() string { return "sys_permission" }
func (SysRolePermission) TableName() string { return "sys_role_permission" }
func (SysUserRole) TableName() string { return "sys_user_role" }
func (SysAuditLog) TableName() string { return "sys_audit_log" }
