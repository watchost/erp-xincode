// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type LoginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginRes struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RefreshReq struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type UserVO struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	Phone     string `json:"phone"`
	Status    int    `json:"status"`
	TenantID  int64  `json:"tenant_id"`
	Roles     []string `json:"roles,omitempty"`
	CreatedAt string `json:"created_at"`
}

type UserListReq struct {
	Username string `form:"username"`
	Page     int    `form:"page" binding:"required,min=1"`
	PageSize int    `form:"page_size" binding:"required,min=1,max=100"`
}

// CreateUserReq 是创建用户的入参，独立于 model.SysUser，避免 Mass Assignment
// 与 PasswordHash `json:"-"` 导致的密码永远无法绑定问题。
type CreateUserReq struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=6,max=128"`
	RealName string `json:"real_name" binding:"required,max=64"`
	Phone    string `json:"phone" binding:"omitempty,max=20"`
	Status   int    `json:"status"`
	TenantID int64  `json:"tenant_id"`
	RoleIDs  []int64 `json:"role_ids"`
}

// UpdateUserReq 只允许更新白名单字段，不能改用户名/租户/密码。
type UpdateUserReq struct {
	RealName *string `json:"real_name" binding:"omitempty,max=64"`
	Phone    *string `json:"phone" binding:"omitempty,max=20"`
	Status   *int    `json:"status" binding:"omitempty,oneof=0 1"`
	RoleIDs  []int64 `json:"role_ids"`
}

// ChangePasswordReq 用户改密。
type ChangePasswordReq struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6,max=128"`
}

type RoleVO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	DataScope int    `json:"data_scope"`
}

type PermissionVO struct {
	ID       int64          `json:"id"`
	Code     string         `json:"code"`
	Name     string         `json:"name"`
	Type     int            `json:"type"`
	ParentID int64          `json:"parent_id"`
	Path     string         `json:"path"`
	Children []PermissionVO `json:"children"`
}
