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
	CreatedAt string `json:"created_at"`
}

type UserListReq struct {
	Username string `json:"username"`
	Page     int    `json:"page" binding:"min=1"`
	PageSize int    `json:"page_size" binding:"min=1,max=100"`
}

type RoleVO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	DataScope int    `json:"data_scope"`
}

type PermissionVO struct {
	ID        int64           `json:"id"`
	Code      string          `json:"code"`
	Name      string          `json:"name"`
	Type      int             `json:"type"`
	ParentID  int64           `json:"parent_id"`
	Path      string          `json:"path"`
	Children  []PermissionVO  `json:"children"`
}
