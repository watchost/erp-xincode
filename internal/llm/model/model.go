// Copyright 2026 zhouhouping. All Rights Reserved.

package model

import (
	"time"
)

type LlmSession struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	ContextMeta []byte    `json:"context_meta"`
	CreatedAt   time.Time `json:"created_at"`
}

type LlmMessage struct {
	ID        int64     `json:"id"`
	SessionID int64     `json:"session_id"`
	Role      int       `json:"role"`
	Content   string    `json:"content"`
	Intent    string    `json:"intent"`
	MetaJSON  []byte    `json:"meta_json"`
	CreatedAt time.Time `json:"created_at"`
}

const (
	RoleUser      = 1
	RoleAssistant = 2
	RoleSystem    = 3
)

// LlmPromptTemplate Prompt 模板（按场景取用，支持启停）。
type LlmPromptTemplate struct {
	ID        int64      `json:"id"`
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Scene     string     `json:"scene"` // chat=对话系统提示 dashboard=仪表盘分析
	Content   string     `json:"content"`
	Enabled   int        `json:"enabled"`
	TenantID  int64      `json:"tenant_id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"-"`
}

const (
	PromptSceneChat      = "chat"
	PromptSceneDashboard = "dashboard"
)

func (LlmSession) TableName() string        { return "llm_session" }
func (LlmMessage) TableName() string        { return "llm_message" }
func (LlmPromptTemplate) TableName() string { return "llm_prompt_template" }
