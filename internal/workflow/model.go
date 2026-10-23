// Copyright 2026 zhouhouping. All Rights Reserved.

// Package workflow 实现 P3 §4.1 审批工作流引擎：
// 流程定义（JSON 节点）、实例、待办任务、审批记录；
// 支持多级审批、会签(all)/或签(any)、加签、转办、驳回、撤销；
// 审批人规则：指定人 user、角色 role、部门负责人 dept_leader、发起人上级 initiator_leader、上级部门负责人 dept_leader_up。
package workflow

import (
	"encoding/json"
	"time"

	"github.com/shopspring/decimal"
)

const (
	DefStatusDraft    = 0
	DefStatusDeployed = 1
	DefStatusDisabled = 2

	InstRunning  = 1
	InstApproved = 2
	InstRejected = 3
	InstCanceled = 4

	TaskTodo        = 1
	TaskApproved    = 2
	TaskTransferred = 3
	TaskRejected    = 4
	TaskCanceled    = 5

	SignAny = "any" // 或签：任一人通过即进入下一节点
	SignAll = "all" // 会签：所有人通过才进入下一节点

	ActionStart    = "start"
	ActionApprove  = "approve"
	ActionReject   = "reject"
	ActionTransfer = "transfer"
	ActionAddSign  = "add_sign"
	ActionCancel   = "cancel"
)

type WfDefinition struct {
	ID         int64           `json:"id"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Version    int             `json:"version"`
	Definition json.RawMessage `gorm:"type:jsonb" json:"definition"`
	Status     int16           `json:"status"`
	TenantID   int64           `json:"tenant_id"`
	CreatedBy  int64           `json:"created_by"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type WfInstance struct {
	ID          int64           `json:"id"`
	InstanceNo  string          `json:"instance_no"`
	DefID       int64           `json:"def_id"`
	BizType     string          `json:"biz_type"`
	BizNo       string          `json:"biz_no"`
	Variables   json.RawMessage `gorm:"type:jsonb" json:"variables"`
	Status      int16           `json:"status"`
	StarterID   int64           `json:"starter_id"`
	CurrentNode string          `json:"current_node"`
	TenantID    int64           `json:"tenant_id"`
	CreatedAt   time.Time       `json:"created_at"`
	FinishedAt  *time.Time      `json:"finished_at"`
}

type WfTask struct {
	ID           int64      `json:"id"`
	InstanceID   int64      `json:"instance_id"`
	NodeCode     string     `json:"node_code"`
	NodeName     string     `json:"node_name"`
	SignMode     string     `json:"sign_mode"`
	AssigneeID   int64      `json:"assignee_id"`
	Status       int16      `json:"status"`
	SourceTaskID *int64     `json:"source_task_id"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

type WfApproval struct {
	ID         int64     `json:"id"`
	InstanceID int64     `json:"instance_id"`
	TaskID     *int64    `json:"task_id"`
	NodeCode   string    `json:"node_code"`
	OperatorID int64     `json:"operator_id"`
	Action     string    `json:"action"`
	Comment    string    `json:"comment"`
	TargetID   *int64    `json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (WfDefinition) TableName() string { return "wf_definition" }
func (WfInstance) TableName() string   { return "wf_instance" }
func (WfTask) TableName() string       { return "wf_task" }
func (WfApproval) TableName() string   { return "wf_approval" }

// ---- 流程定义 JSON ----

type FlowDef struct {
	Nodes []NodeDef `json:"nodes"`
}

type NodeDef struct {
	Code      string        `json:"code"`
	Type      string        `json:"type"` // start / approval / end
	Name      string        `json:"name"`
	Assignee  *AssigneeRule `json:"assignee,omitempty"`
	SignMode  string        `json:"sign_mode,omitempty"`
	Condition *Condition    `json:"condition,omitempty"` // 不满足时跳过本节点
}

type AssigneeRule struct {
	Type  string          `json:"type"`  // user / role / dept_leader / dept_leader_up / initiator_leader
	Value json.RawMessage `json:"value"` // user: 用户 ID 或 ID 数组；role: 角色 code
}

type Condition struct {
	Field string          `json:"field"`
	Op    string          `json:"op"` // > >= < <= == !=
	Value decimal.Decimal `json:"value"`
}

// ---- DTO ----

type CreateDefinitionReq struct {
	Code       string          `json:"code" binding:"required,max=64"`
	Name       string          `json:"name" binding:"required,max=128"`
	Definition json.RawMessage `json:"definition" binding:"required"`
}

type StartReq struct {
	BizType   string                     `json:"biz_type" binding:"required,max=64"`
	BizNo     string                     `json:"biz_no" binding:"required,max=64"`
	Variables map[string]decimal.Decimal `json:"variables"`
}

type ActionReq struct {
	Comment string `json:"comment" binding:"max=1000"`
}

type TargetReq struct {
	TargetUserID int64  `json:"target_user_id" binding:"required,gt=0"`
	Comment      string `json:"comment" binding:"max=1000"`
}

type TaskVO struct {
	TaskID       int64      `json:"task_id"`
	InstanceNo   string     `json:"instance_no"`
	BizType      string     `json:"biz_type"`
	BizNo        string     `json:"biz_no"`
	NodeCode     string     `json:"node_code"`
	NodeName     string     `json:"node_name"`
	SignMode     string     `json:"sign_mode"`
	Status       int16      `json:"status"`
	StarterID    int64      `json:"starter_id"`
	InstanceStat int16      `json:"instance_status"`
	CreatedAt    time.Time  `json:"created_at"`
	FinishedAt   *time.Time `json:"finished_at"`
}

type InstanceDetailVO struct {
	Instance  WfInstance   `json:"instance"`
	Tasks     []WfTask     `json:"tasks"`
	Approvals []WfApproval `json:"approvals"`
}
