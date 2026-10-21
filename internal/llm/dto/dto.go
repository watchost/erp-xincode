// Copyright 2026 zhouhouping. All Rights Reserved.

package dto

type ChatReq struct {
	SessionID int64  `json:"session_id"`
	Question  string `json:"question" binding:"required"`
}

type ChatRes struct {
	SessionID int64  `json:"session_id"`
	Answer    string `json:"answer"`
	Intent    string `json:"intent"`
}

type SessionVO struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"created_at"`
}

type MessageVO struct {
	ID        int64  `json:"id"`
	Role      int    `json:"role"`
	RoleName  string `json:"role_name"`
	Content   string `json:"content"`
	Intent    string `json:"intent"`
	CreatedAt string `json:"created_at"`
}

// PromptTemplateReq 模板创建/更新。Enabled 用指针区分「未传」与「置 0」。
type PromptTemplateReq struct {
	Code    string `json:"code" binding:"required"`
	Name    string `json:"name" binding:"required"`
	Scene   string `json:"scene" binding:"required,oneof=chat dashboard"`
	Content string `json:"content" binding:"required"`
	Enabled *int   `json:"enabled"`
}

type PromptTemplateVO struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	Scene     string `json:"scene"`
	Content   string `json:"content"`
	Enabled   int    `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

type AnalysisContext struct {
	UserQuestion string                 `json:"user_question"`
	BusinessData map[string]interface{} `json:"business_data"`
	UserRole     string                 `json:"user_role"`
	TimeRange    string                 `json:"time_range"`
}
