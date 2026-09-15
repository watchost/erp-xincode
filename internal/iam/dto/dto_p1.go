// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

// CaptchaVO 图形验证码响应。Image 为可直接放入 <img src> 的 data URI。
type CaptchaVO struct {
	CaptchaID string `json:"captcha_id"`
	Image     string `json:"image"`
}

// ResetPasswordReq 管理员重置密码；Password 为空时由服务端生成 12 位随机密码。
type ResetPasswordReq struct {
	Password string `json:"password" binding:"omitempty,min=6,max=128"`
}

type ResetPasswordRes struct {
	// Password 明文新密码仅此一次返回，不落库、不记审计明文。
	Password string `json:"password"`
}

type AssignUserRolesReq struct {
	RoleIDs []int64 `json:"role_ids" binding:"required,dive"`
}

// ---- 角色 ----

type CreateRoleReq struct {
	Code      string `json:"code" binding:"required,min=2,max=64"`
	Name      string `json:"name" binding:"required,max=64"`
	DataScope int    `json:"data_scope" binding:"oneof=1 2 3 4 5"`
}

// UpdateRoleReq 角色编码不可改（编码是日志/配置里的稳定标识）。
type UpdateRoleReq struct {
	Name      *string `json:"name" binding:"omitempty,max=64"`
	DataScope *int    `json:"data_scope" binding:"omitempty,oneof=1 2 3 4 5"`
}

type AssignPermsReq struct {
	PermissionIDs []int64 `json:"permission_ids" binding:"required,min=1,dive"`
}

type AssignRoleDeptsReq struct {
	DeptIDs []int64 `json:"dept_ids" binding:"required,dive"`
}

type RoleDetailVO struct {
	RoleVO
	PermissionIDs []int64 `json:"permission_ids"`
	DeptIDs       []int64 `json:"dept_ids"`
}

// ---- 部门 ----

type DepartmentReq struct {
	Code     string `json:"code" binding:"required,min=2,max=32"`
	Name     string `json:"name" binding:"required,max=64"`
	ParentID *int64 `json:"parent_id"`
	LeaderID *int64 `json:"leader_id"`
}

type DepartmentVO struct {
	ID       int64          `json:"id"`
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	ParentID *int64         `json:"parent_id"`
	LeaderID *int64         `json:"leader_id"`
	Children []DepartmentVO `json:"children,omitempty"`
}

// ---- 审计日志 ----

type AuditLogListReq struct {
	UserID    *int64 `form:"user_id"`
	Module    string `form:"module"`
	Action    string `form:"action"`
	StartTime string `form:"start_time"` // RFC3339 或 2006-01-02 15:04:05
	EndTime   string `form:"end_time"`
	Page      int    `form:"page" binding:"required,min=1"`
	PageSize  int    `form:"page_size" binding:"required,min=1,max=100"`
}

type AuditLogVO struct {
	ID         int64  `json:"id"`
	UserID     *int64 `json:"user_id"`
	Action     string `json:"action"`
	Module     string `json:"module"`
	IP         string `json:"ip"`
	ReqParam   string `json:"req_param,omitempty"`
	TraceID    string `json:"trace_id"`
	UserAgent  string `json:"user_agent"`
	Status     int    `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	CreatedAt  string `json:"created_at"`
}
