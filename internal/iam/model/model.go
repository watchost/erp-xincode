// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type SysUser struct {
	ID            int64          `json:"id"`
	Username      string         `json:"username"`
	PasswordHash  string         `json:"-"`
	RealName      string         `json:"real_name"`
	Phone         string         `json:"phone"`
	Email         string         `json:"email"`
	Status        int            `json:"status"`
	TenantID      int64          `json:"tenant_id"`
	DeptID        *int64         `json:"dept_id"`
	MustChangePwd bool           `json:"must_change_pwd"`
	PwdChangedAt  *time.Time     `json:"pwd_changed_at"`
	LastLoginAt   *time.Time     `json:"last_login_at"`
	LastLoginIP   string         `json:"last_login_ip"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
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

// SysRoleDept 仅用于 data_scope=5（自定义部门）的角色。
type SysRoleDept struct {
	RoleID int64 `json:"role_id"`
	DeptID int64 `json:"dept_id"`
}

// SysDepartment 部门树；Path 为物化路径（根="/1/"，子="/1/3/"），便于子孙查询。
type SysDepartment struct {
	ID        int64           `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	ParentID  *int64          `json:"parent_id"`
	Path      string          `json:"path"`
	LeaderID  *int64          `json:"leader_id"`
	TenantID  int64           `json:"tenant_id"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Children  []SysDepartment `gorm:"-" json:"children,omitempty"`
}

type SysAuditLog struct {
	ID         int64     `json:"id"`
	UserID     *int64    `json:"user_id"`
	Action     string    `json:"action"`
	Module     string    `json:"module"`
	IP         string    `json:"ip"`
	ReqParam   JSON      `json:"req_param"`
	ResSummary string    `json:"res_summary"`
	TraceID    string    `json:"trace_id"`
	TenantID   int64     `json:"tenant_id"`
	UserAgent  string    `json:"user_agent"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

// SysLoginAttempt 登录尝试流水；锁定判定在 Redis，本表只做审计留痕。
type SysLoginAttempt struct {
	ID         int64     `json:"id"`
	Username   string    `json:"username"`
	IP         string    `json:"ip"`
	Success    bool      `json:"success"`
	FailReason string    `json:"fail_reason"`
	CreatedAt  time.Time `json:"created_at"`
}

type JSON json.RawMessage

func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return json.RawMessage(j).MarshalJSON()
}

func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}
	result := json.RawMessage{}
	err := json.Unmarshal(bytes, &result)
	*j = JSON(result)
	return err
}

func (SysUser) TableName() string           { return "sys_user" }
func (SysRole) TableName() string           { return "sys_role" }
func (SysPermission) TableName() string     { return "sys_permission" }
func (SysRolePermission) TableName() string { return "sys_role_permission" }
func (SysUserRole) TableName() string       { return "sys_user_role" }
func (SysRoleDept) TableName() string       { return "sys_role_dept" }
func (SysDepartment) TableName() string     { return "sys_department" }
func (SysAuditLog) TableName() string       { return "sys_audit_log" }
func (SysLoginAttempt) TableName() string   { return "sys_login_attempt" }
