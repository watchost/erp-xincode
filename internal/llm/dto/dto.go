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

type AnalysisContext struct {
	UserQuestion string                 `json:"user_question"`
	BusinessData map[string]interface{}  `json:"business_data"`
	UserRole     string                 `json:"user_role"`
	TimeRange    string                 `json:"time_range"`
}
