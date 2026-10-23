// Copyright 2026 zhouhouping. All Rights Reserved.

package workflow

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
)

// Approvable 由接入工作流的业务 service 实现。
// 回调与审批动作在同一个数据库事务内执行：业务状态更新失败时，审批记录一并回滚。
type Approvable interface {
	OnApproved(ctx context.Context, tx *gorm.DB, bizNo string) error
	OnRejected(ctx context.Context, tx *gorm.DB, bizNo string, reason string) error
}

// 错误码段 80xxx：工作流
var (
	ErrDefNotFound      = errors.New(80001, 404, "流程定义不存在")
	ErrNoDeployedDef    = errors.New(80002, 400, "该业务类型没有已部署的流程定义")
	ErrInstanceNotFound = errors.New(80003, 404, "流程实例不存在")
	ErrTaskNotFound     = errors.New(80004, 404, "待办任务不存在")
	ErrNotAssignee      = errors.New(80005, 403, "不是该任务的审批人")
	ErrTaskDone         = errors.New(80006, 409, "任务已处理")
	ErrAlreadyRunning   = errors.New(80007, 409, "该单据已有运行中的审批流程")
	ErrCommentRequired  = errors.New(80008, 400, "驳回时审批意见必填")
	ErrNoAssignee       = errors.New(80009, 400, "审批节点找不到可用审批人")
	ErrInvalidDef       = errors.New(80010, 400, "流程定义不合法")
	ErrNotStarter       = errors.New(80011, 403, "只有发起人可以撤销流程")
	ErrInvalidTarget    = errors.New(80012, 400, "目标用户不存在、已停用或已在本节点审批")
)

type Service struct {
	db        *gorm.DB
	mu        sync.RWMutex
	callbacks map[string]Approvable
	notifier  TodoNotifier
	seq       uint64
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, callbacks: map[string]Approvable{}}
}

// Register 注册业务类型回调（map[bizType]Approvable）。
func (s *Service) Register(bizType string, a Approvable) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.callbacks[bizType] = a
}

func (s *Service) callback(bizType string) Approvable {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.callbacks[bizType]
}

// ---------------- 流程定义 ----------------

// ParseFlow 校验并解析定义 JSON：必须恰有一个 start、一个 end，审批节点必须配置审批人，节点 code 唯一。
func ParseFlow(raw json.RawMessage) (*FlowDef, error) {
	var f FlowDef
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, errors.New(ErrInvalidDef.Code, 400, "流程定义 JSON 解析失败: "+err.Error())
	}
	seen := map[string]bool{}
	starts, ends, approvals := 0, 0, 0
	for _, n := range f.Nodes {
		if n.Code == "" || seen[n.Code] {
			return nil, errors.New(ErrInvalidDef.Code, 400, fmt.Sprintf("节点 code 为空或重复: %q", n.Code))
		}
		seen[n.Code] = true
		switch n.Type {
		case "start":
			starts++
		case "end":
			ends++
		case "approval":
			approvals++
			if n.Assignee == nil {
				return nil, errors.New(ErrInvalidDef.Code, 400, "审批节点缺少 assignee: "+n.Code)
			}
			switch n.Assignee.Type {
			case "user", "role", "dept_leader", "dept_leader_up", "initiator_leader":
			default:
				return nil, errors.New(ErrInvalidDef.Code, 400, "不支持的审批人类型: "+n.Assignee.Type)
			}
			if n.SignMode != "" && n.SignMode != SignAny && n.SignMode != SignAll {
				return nil, errors.New(ErrInvalidDef.Code, 400, "sign_mode 只能是 any 或 all: "+n.Code)
			}
		default:
			return nil, errors.New(ErrInvalidDef.Code, 400, "当前版本仅支持 start/approval/end 节点: "+n.Type)
		}
	}
	if starts != 1 || ends != 1 || approvals == 0 || f.Nodes[0].Type != "start" || f.Nodes[len(f.Nodes)-1].Type != "end" {
		return nil, errors.New(ErrInvalidDef.Code, 400, "流程必须以 start 开头、end 结尾，且至少一个审批节点")
	}
	return &f, nil
}

func (s *Service) CreateDefinition(ctx context.Context, req CreateDefinitionReq) (*WfDefinition, error) {
	if _, err := ParseFlow(req.Definition); err != nil {
		return nil, err
	}
	var def WfDefinition
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVer int
		if err := tx.Model(&WfDefinition{}).Where("tenant_id = ? AND code = ?", tenantOf(ctx), req.Code).
			Select("COALESCE(MAX(version), 0)").Scan(&maxVer).Error; err != nil {
			return err
		}
		def = WfDefinition{
			Code: req.Code, Name: req.Name, Version: maxVer + 1, Definition: req.Definition, TenantID: tenantOf(ctx),
			Status: DefStatusDraft, CreatedBy: auth.UserIDFromContext(ctx),
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
		return tx.Create(&def).Error
	})
	if err != nil {
		return nil, err
	}
	return &def, nil
}

func (s *Service) ListDefinitions(ctx context.Context, code string) ([]WfDefinition, error) {
	var list []WfDefinition
	q := s.db.WithContext(ctx).Order("code, version DESC")
	if code != "" {
		q = q.Where("code = ?", code)
	}
	return list, q.Find(&list).Error
}

// DeployDefinition 部署指定版本；同 code 其他已部署版本自动停用。已运行的实例继续按原版本流转。
func (s *Service) DeployDefinition(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var def WfDefinition
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&def, id).Error; err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return ErrDefNotFound
			}
			return err
		}
		if _, err := ParseFlow(def.Definition); err != nil {
			return err
		}
		if err := tx.Model(&WfDefinition{}).Where("tenant_id = ? AND code = ? AND status = ? AND id <> ?", def.TenantID, def.Code, DefStatusDeployed, id).
			Updates(map[string]any{"status": DefStatusDisabled, "updated_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Model(&def).Updates(map[string]any{"status": DefStatusDeployed, "updated_at": time.Now()}).Error
	})
}

// HasDeployed 判断某业务类型是否有已部署的流程（业务 service 用来决定是否允许直接审批）。
func (s *Service) HasDeployed(ctx context.Context, tx *gorm.DB, bizType string) (bool, error) {
	db := s.db
	if tx != nil {
		db = tx
	}
	var n int64
	err := db.WithContext(ctx).Model(&WfDefinition{}).Where("tenant_id = ? AND code = ? AND status = ?", tenantOf(ctx), bizType, DefStatusDeployed).Count(&n).Error
	return n > 0, err
}

// ---------------- 启动 ----------------

// Start 启动流程（独立事务）。
func (s *Service) Start(ctx context.Context, req StartReq) (*WfInstance, error) {
	var inst *WfInstance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		inst, err = s.StartTx(ctx, tx, req.BizType, req.BizNo, req.Variables)
		return err
	})
	return inst, err
}

// StartTx 在调用方事务内启动流程（业务单据提交与流程启动原子化）。
func (s *Service) StartTx(ctx context.Context, tx *gorm.DB, bizType, bizNo string, vars map[string]decimal.Decimal) (*WfInstance, error) {
	starter := auth.UserIDFromContext(ctx)
	if starter <= 0 {
		return nil, errors.New(10002, 401, "未登录")
	}
	// 显式按租户取定义与建实例：不依赖插件，保证审批人解析/通知都使用发起人所在租户
	tenantID, _ := auth.TenantFromContext(ctx)
	var def WfDefinition
	if err := tx.Where("tenant_id = ? AND code = ? AND status = ?", tenantID, bizType, DefStatusDeployed).First(&def).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoDeployedDef
		}
		return nil, err
	}
	flow, err := ParseFlow(def.Definition)
	if err != nil {
		return nil, err
	}
	var running int64
	if err := tx.Model(&WfInstance{}).Where("tenant_id = ? AND biz_type = ? AND biz_no = ? AND status = ?", tenantID, bizType, bizNo, InstRunning).
		Count(&running).Error; err != nil {
		return nil, err
	}
	if running > 0 {
		return nil, ErrAlreadyRunning
	}
	varsJSON, _ := json.Marshal(vars)
	inst := &WfInstance{
		InstanceNo: s.nextNo(), DefID: def.ID, BizType: bizType, BizNo: bizNo, TenantID: tenantID,
		Variables: varsJSON, Status: InstRunning, StarterID: starter, CreatedAt: time.Now(),
	}
	if err := tx.Create(inst).Error; err != nil {
		// 并发提交时由部分唯一索引 uk_wf_inst_running 兜底
		if strings.Contains(err.Error(), "uk_wf_inst_running") {
			return nil, ErrAlreadyRunning
		}
		return nil, err
	}
	if err := s.record(tx, inst.ID, nil, "start", starter, ActionStart, "", nil); err != nil {
		return nil, err
	}
	if err := s.advance(ctx, tx, inst, flow, 0); err != nil {
		return nil, err
	}
	return inst, nil
}

// advance 从 flow.Nodes[fromIdx] 之后寻找下一个需要处理的审批节点并生成待办；到达 end 则结束流程并回调业务。
func (s *Service) advance(ctx context.Context, tx *gorm.DB, inst *WfInstance, flow *FlowDef, fromIdx int) error {
	vars := map[string]decimal.Decimal{}
	_ = json.Unmarshal(inst.Variables, &vars)

	for i := fromIdx + 1; i < len(flow.Nodes); i++ {
		n := flow.Nodes[i]
		if n.Type == "end" {
			now := time.Now()
			inst.Status, inst.CurrentNode, inst.FinishedAt = InstApproved, "end", &now
			if err := tx.Model(inst).Updates(map[string]any{
				"status": InstApproved, "current_node": "end", "finished_at": now,
			}).Error; err != nil {
				return err
			}
			if cb := s.callback(inst.BizType); cb != nil {
				return cb.OnApproved(ctx, tx, inst.BizNo)
			}
			return nil
		}
		if n.Type != "approval" || !conditionMatch(n.Condition, vars) {
			continue
		}
		assignees, err := s.resolveAssignees(tx, inst.TenantID, n.Assignee, inst.StarterID)
		if err != nil {
			return err
		}
		mode := n.SignMode
		if mode == "" {
			mode = SignAny
		}
		for _, uid := range assignees {
			if err := tx.Create(&WfTask{
				InstanceID: inst.ID, NodeCode: n.Code, NodeName: n.Name, SignMode: mode,
				AssigneeID: uid, Status: TaskTodo, CreatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
		}
		if err := s.notifyTodo(ctx, tx, inst, assignees, n.Name); err != nil {
			return err
		}
		inst.CurrentNode = n.Code
		return tx.Model(inst).Update("current_node", n.Code).Error
	}
	return ErrInvalidDef
}

// TodoNotifier 新建待办时的通知（P3 §4.4，由 message 包实现，避免 workflow 依赖 message）。
// 在审批事务内调用：流程回滚时消息一并回滚。
type TodoNotifier interface {
	ApprovalTodoTx(ctx context.Context, tx *gorm.DB, tenantID int64, assignees []int64, bizType, bizNo, nodeName, instanceNo string) error
}

func (s *Service) SetNotifier(n TodoNotifier) { s.notifier = n }

func (s *Service) notifyTodo(ctx context.Context, tx *gorm.DB, inst *WfInstance, assignees []int64, nodeName string) error {
	if s.notifier == nil || len(assignees) == 0 {
		return nil
	}
	return s.notifier.ApprovalTodoTx(ctx, tx, inst.TenantID, assignees, inst.BizType, inst.BizNo, nodeName, inst.InstanceNo)
}

func conditionMatch(c *Condition, vars map[string]decimal.Decimal) bool {
	if c == nil {
		return true
	}
	v, ok := vars[c.Field]
	if !ok {
		// 缺少条件变量时不跳过节点：宁可多审一级，也不放过审批
		return true
	}
	switch c.Op {
	case ">":
		return v.GreaterThan(c.Value)
	case ">=":
		return v.GreaterThanOrEqual(c.Value)
	case "<":
		return v.LessThan(c.Value)
	case "<=":
		return v.LessThanOrEqual(c.Value)
	case "==":
		return v.Equal(c.Value)
	case "!=":
		return !v.Equal(c.Value)
	}
	return true
}

// resolveAssignees 解析审批人；只返回本租户内存在、启用、未删除的用户。找不到任何人时返回 ErrNoAssignee（不静默跳过）。
// sys_user_role / sys_role 是全局表（没有 tenant_id），所以最终统一经 filterActive 按 sys_user.tenant_id 限定。
func (s *Service) resolveAssignees(tx *gorm.DB, tenantID int64, rule *AssigneeRule, starterID int64) ([]int64, error) {
	var ids []int64
	switch rule.Type {
	case "user":
		var one int64
		if err := json.Unmarshal(rule.Value, &one); err == nil && one > 0 {
			ids = []int64{one}
		} else if err := json.Unmarshal(rule.Value, &ids); err != nil {
			return nil, errors.New(ErrInvalidDef.Code, 400, "user 审批人 value 应为用户 ID 或 ID 数组")
		}
	case "role":
		var code string
		if err := json.Unmarshal(rule.Value, &code); err != nil || code == "" {
			return nil, errors.New(ErrInvalidDef.Code, 400, "role 审批人 value 应为角色 code")
		}
		if err := tx.Table("sys_user_role ur").Select("ur.user_id").
			Joins("JOIN sys_role r ON r.id = ur.role_id").
			Where("r.code = ?", code).Scan(&ids).Error; err != nil {
			return nil, err
		}
	case "dept_leader", "initiator_leader", "dept_leader_up":
		leader, err := s.findLeader(tx, starterID, rule.Type == "dept_leader_up")
		if err != nil {
			return nil, err
		}
		if leader > 0 {
			ids = []int64{leader}
		}
	}
	active, err := s.filterActive(tx, tenantID, ids)
	if err != nil {
		return nil, err
	}
	if len(active) == 0 {
		return nil, errors.New(ErrNoAssignee.Code, 400, fmt.Sprintf("审批节点找不到可用审批人（规则 %s）", rule.Type))
	}
	return active, nil
}

// findLeader 查找发起人的部门负责人。
// 发起人本人就是部门负责人（或 up=true）时，取上级部门负责人，避免自己审批自己。
// 说明：sys_user 没有"直属上级"字段，initiator_leader 按"部门负责人"口径实现。
func (s *Service) findLeader(tx *gorm.DB, starterID int64, up bool) (int64, error) {
	var deptID *int64
	if err := tx.Table("sys_user").Select("dept_id").Where("id = ?", starterID).Scan(&deptID).Error; err != nil {
		return 0, err
	}
	if deptID == nil {
		return 0, nil
	}
	type dept struct {
		ParentID *int64
		LeaderID *int64
	}
	cur := *deptID
	skipSelf := up
	for depth := 0; depth < 20 && cur > 0; depth++ {
		var d dept
		if err := tx.Table("sys_department").Select("parent_id, leader_id").Where("id = ?", cur).Scan(&d).Error; err != nil {
			return 0, err
		}
		if !skipSelf && d.LeaderID != nil && *d.LeaderID > 0 && *d.LeaderID != starterID {
			return *d.LeaderID, nil
		}
		skipSelf = false
		if d.ParentID == nil {
			return 0, nil
		}
		cur = *d.ParentID
	}
	return 0, nil
}

func (s *Service) filterActive(tx *gorm.DB, tenantID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []int64
	err := tx.Table("sys_user").Select("id").
		Where("id IN ? AND tenant_id = ? AND status = 1 AND deleted_at IS NULL", ids, tenantID).Order("id").Scan(&out).Error
	return out, err
}

// ---------------- 任务处理 ----------------

func (s *Service) lockTask(tx *gorm.DB, taskID, operator int64) (*WfTask, *WfInstance, *FlowDef, int, error) {
	var t WfTask
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&t, taskID).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, 0, ErrTaskNotFound
		}
		return nil, nil, nil, 0, err
	}
	var inst WfInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&inst, t.InstanceID).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	if t.AssigneeID != operator {
		return nil, nil, nil, 0, ErrNotAssignee
	}
	if t.Status != TaskTodo || inst.Status != InstRunning {
		return nil, nil, nil, 0, ErrTaskDone
	}
	var def WfDefinition
	if err := tx.First(&def, inst.DefID).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	flow, err := ParseFlow(def.Definition)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	idx := -1
	for i, n := range flow.Nodes {
		if n.Code == t.NodeCode {
			idx = i
		}
	}
	if idx < 0 {
		return nil, nil, nil, 0, ErrInvalidDef
	}
	return &t, &inst, flow, idx, nil
}

func (s *Service) finishTask(tx *gorm.DB, t *WfTask, status int16) error {
	now := time.Now()
	t.Status, t.FinishedAt = status, &now
	return tx.Model(t).Updates(map[string]any{"status": status, "finished_at": now}).Error
}

// Approve 通过。或签：本节点其余待办取消并进入下一节点；会签：本节点全部通过后才进入下一节点。
func (s *Service) Approve(ctx context.Context, taskID int64, comment string) error {
	op := auth.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, inst, flow, idx, err := s.lockTask(tx, taskID, op)
		if err != nil {
			return err
		}
		if err := s.finishTask(tx, t, TaskApproved); err != nil {
			return err
		}
		if err := s.record(tx, inst.ID, &t.ID, t.NodeCode, op, ActionApprove, comment, nil); err != nil {
			return err
		}
		if t.SignMode == SignAll {
			var pending int64
			if err := tx.Model(&WfTask{}).Where("instance_id = ? AND node_code = ? AND status = ?", inst.ID, t.NodeCode, TaskTodo).
				Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return nil
			}
		} else if err := s.cancelNodeTodos(tx, inst.ID, t.NodeCode); err != nil {
			return err
		}
		return s.advance(ctx, tx, inst, flow, idx)
	})
}

// Reject 驳回：流程结束（状态=驳回），取消所有待办，回调业务 OnRejected（如采购订单退回草稿）。意见必填。
func (s *Service) Reject(ctx context.Context, taskID int64, comment string) error {
	if strings.TrimSpace(comment) == "" {
		return ErrCommentRequired
	}
	op := auth.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, inst, _, _, err := s.lockTask(tx, taskID, op)
		if err != nil {
			return err
		}
		if err := s.finishTask(tx, t, TaskRejected); err != nil {
			return err
		}
		if err := s.record(tx, inst.ID, &t.ID, t.NodeCode, op, ActionReject, comment, nil); err != nil {
			return err
		}
		return s.closeInstance(ctx, tx, inst, InstRejected, comment)
	})
}

// Transfer 转办：本任务标记为已转办，同节点为目标用户生成新待办（会签/或签属性不变）。
func (s *Service) Transfer(ctx context.Context, taskID, target int64, comment string) error {
	op := auth.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, inst, _, _, err := s.lockTask(tx, taskID, op)
		if err != nil {
			return err
		}
		if err := s.checkTarget(tx, inst, t.NodeCode, target); err != nil {
			return err
		}
		if err := s.finishTask(tx, t, TaskTransferred); err != nil {
			return err
		}
		if err := tx.Create(&WfTask{
			InstanceID: inst.ID, NodeCode: t.NodeCode, NodeName: t.NodeName, SignMode: t.SignMode,
			AssigneeID: target, Status: TaskTodo, SourceTaskID: &t.ID, CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := s.notifyTodo(ctx, tx, inst, []int64{target}, t.NodeName); err != nil {
			return err
		}
		return s.record(tx, inst.ID, &t.ID, t.NodeCode, op, ActionTransfer, comment, &target)
	})
}

// AddSign 加签：在当前节点为目标用户追加一条待办，本人任务保持待办。
// 加签后本节点按会签处理（所有待办都通过才进入下一节点），否则加签人的意见可能被或签直接跳过。
func (s *Service) AddSign(ctx context.Context, taskID, target int64, comment string) error {
	op := auth.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, inst, _, _, err := s.lockTask(tx, taskID, op)
		if err != nil {
			return err
		}
		if err := s.checkTarget(tx, inst, t.NodeCode, target); err != nil {
			return err
		}
		if err := tx.Model(&WfTask{}).Where("instance_id = ? AND node_code = ? AND status = ?", inst.ID, t.NodeCode, TaskTodo).
			Update("sign_mode", SignAll).Error; err != nil {
			return err
		}
		if err := tx.Create(&WfTask{
			InstanceID: inst.ID, NodeCode: t.NodeCode, NodeName: t.NodeName, SignMode: SignAll,
			AssigneeID: target, Status: TaskTodo, SourceTaskID: &t.ID, CreatedAt: time.Now(),
		}).Error; err != nil {
			return err
		}
		if err := s.notifyTodo(ctx, tx, inst, []int64{target}, t.NodeName); err != nil {
			return err
		}
		return s.record(tx, inst.ID, &t.ID, t.NodeCode, op, ActionAddSign, comment, &target)
	})
}

// checkTarget 目标用户必须是本租户内存在且启用的用户，且在本节点没有待办（避免重复）。
func (s *Service) checkTarget(tx *gorm.DB, inst *WfInstance, node string, target int64) error {
	active, err := s.filterActive(tx, inst.TenantID, []int64{target})
	if err != nil {
		return err
	}
	if len(active) == 0 {
		return ErrInvalidTarget
	}
	var dup int64
	if err := tx.Model(&WfTask{}).Where("instance_id = ? AND node_code = ? AND assignee_id = ? AND status = ?", inst.ID, node, target, TaskTodo).
		Count(&dup).Error; err != nil {
		return err
	}
	if dup > 0 {
		return ErrInvalidTarget
	}
	return nil
}

// Cancel 发起人撤销运行中的流程。业务回调按驳回处理（单据退回草稿）。
func (s *Service) Cancel(ctx context.Context, instanceNo, comment string) error {
	op := auth.UserIDFromContext(ctx)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var inst WfInstance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("instance_no = ?", instanceNo).First(&inst).Error; err != nil {
			if stdErrors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstanceNotFound
			}
			return err
		}
		if inst.StarterID != op {
			return ErrNotStarter
		}
		if inst.Status != InstRunning {
			return ErrTaskDone
		}
		return s.cancelRunning(ctx, tx, &inst, op, comment)
	})
}

// CancelByBizTx 业务单据取消时同步撤销其运行中的流程（无运行实例时不做任何事）。不回调业务（由业务自身处理状态）。
func (s *Service) CancelByBizTx(ctx context.Context, tx *gorm.DB, bizType, bizNo string) error {
	var inst WfInstance
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("tenant_id = ? AND biz_type = ? AND biz_no = ? AND status = ?", tenantOf(ctx), bizType, bizNo, InstRunning).First(&inst).Error
	if stdErrors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	op := auth.UserIDFromContext(ctx)
	if err := s.record(tx, inst.ID, nil, inst.CurrentNode, op, ActionCancel, "业务单据取消", nil); err != nil {
		return err
	}
	if err := s.cancelNodeTodos(tx, inst.ID, ""); err != nil {
		return err
	}
	now := time.Now()
	return tx.Model(&inst).Updates(map[string]any{"status": InstCanceled, "finished_at": now}).Error
}

func (s *Service) cancelRunning(ctx context.Context, tx *gorm.DB, inst *WfInstance, op int64, comment string) error {
	if err := s.record(tx, inst.ID, nil, inst.CurrentNode, op, ActionCancel, comment, nil); err != nil {
		return err
	}
	return s.closeInstance(ctx, tx, inst, InstCanceled, "发起人撤销: "+comment)
}

func (s *Service) closeInstance(ctx context.Context, tx *gorm.DB, inst *WfInstance, status int16, reason string) error {
	if err := s.cancelNodeTodos(tx, inst.ID, ""); err != nil {
		return err
	}
	now := time.Now()
	if err := tx.Model(inst).Updates(map[string]any{"status": status, "finished_at": now}).Error; err != nil {
		return err
	}
	if cb := s.callback(inst.BizType); cb != nil {
		return cb.OnRejected(ctx, tx, inst.BizNo, reason)
	}
	return nil
}

// cancelNodeTodos 取消实例的待办；node 为空表示取消全部节点。
func (s *Service) cancelNodeTodos(tx *gorm.DB, instID int64, node string) error {
	q := tx.Model(&WfTask{}).Where("instance_id = ? AND status = ?", instID, TaskTodo)
	if node != "" {
		q = q.Where("node_code = ?", node)
	}
	return q.Updates(map[string]any{"status": TaskCanceled, "finished_at": time.Now()}).Error
}

func (s *Service) record(tx *gorm.DB, instID int64, taskID *int64, node string, op int64, action, comment string, target *int64) error {
	return tx.Create(&WfApproval{
		InstanceID: instID, TaskID: taskID, NodeCode: node, OperatorID: op,
		Action: action, Comment: comment, TargetID: target, CreatedAt: time.Now(),
	}).Error
}

// nextNo 生成实例号：WF + 秒级时间 + 微秒 + 进程内序号；唯一性最终由 uk_wf_inst_no 兜底。
func (s *Service) nextNo() string {
	s.mu.Lock()
	s.seq++
	n := s.seq % 10000
	s.mu.Unlock()
	now := time.Now()
	return fmt.Sprintf("WF%s%06d%04d", now.Format("20060102150405"), now.Nanosecond()/1000, n)
}

// ---------------- 查询 ----------------

func (s *Service) listTasks(ctx context.Context, userID int64, todo bool, page, size int) ([]TaskVO, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := s.db.WithContext(ctx).Table("wf_task t").
		Joins("JOIN wf_instance i ON i.id = t.instance_id").
		Where("t.assignee_id = ?", userID)
	if todo {
		q = q.Where("t.status = ? AND i.status = ?", TaskTodo, InstRunning)
	} else {
		q = q.Where("t.status IN ?", []int16{TaskApproved, TaskTransferred, TaskRejected})
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []TaskVO
	order := "t.created_at ASC"
	if !todo {
		order = "t.finished_at DESC"
	}
	err := q.Select(`t.id AS task_id, i.instance_no, i.biz_type, i.biz_no, t.node_code, t.node_name, t.sign_mode,
		t.status, i.starter_id, i.status AS instance_stat, t.created_at, t.finished_at`).
		Order(order).Offset((page - 1) * size).Limit(size).Scan(&list).Error
	return list, total, err
}

func (s *Service) ListTodo(ctx context.Context, page, size int) ([]TaskVO, int64, error) {
	return s.listTasks(ctx, auth.UserIDFromContext(ctx), true, page, size)
}

func (s *Service) ListDone(ctx context.Context, page, size int) ([]TaskVO, int64, error) {
	return s.listTasks(ctx, auth.UserIDFromContext(ctx), false, page, size)
}

// GetInstance 审批历史：实例 + 全部任务 + 审批记录（按时间）。
func (s *Service) GetInstance(ctx context.Context, instanceNo string) (*InstanceDetailVO, error) {
	var inst WfInstance
	if err := s.db.WithContext(ctx).Where("instance_no = ?", instanceNo).First(&inst).Error; err != nil {
		if stdErrors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	out := &InstanceDetailVO{Instance: inst}
	if err := s.db.WithContext(ctx).Where("instance_id = ?", inst.ID).Order("id").Find(&out.Tasks).Error; err != nil {
		return nil, err
	}
	if err := s.db.WithContext(ctx).Where("instance_id = ?", inst.ID).Order("id").Find(&out.Approvals).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// LatestInstanceNo 返回某单据最近一次流程实例号（业务详情页跳转审批历史用）；没有时返回空串。
func (s *Service) LatestInstanceNo(ctx context.Context, bizType, bizNo string) (string, error) {
	var no string
	err := s.db.WithContext(ctx).Model(&WfInstance{}).Select("instance_no").
		Where("biz_type = ? AND biz_no = ?", bizType, bizNo).Order("id DESC").Limit(1).Scan(&no).Error
	return no, err
}

// BizGateway 把 Service 适配为业务模块使用的窄接口（StartTx 只返回实例号），
// 业务包无需引用 workflow 的模型类型。
type BizGateway struct{ S *Service }

func (g BizGateway) HasDeployed(ctx context.Context, tx *gorm.DB, bizType string) (bool, error) {
	return g.S.HasDeployed(ctx, tx, bizType)
}

func (g BizGateway) StartTx(ctx context.Context, tx *gorm.DB, bizType, bizNo string, vars map[string]decimal.Decimal) (string, error) {
	inst, err := g.S.StartTx(ctx, tx, bizType, bizNo, vars)
	if err != nil {
		return "", err
	}
	return inst.InstanceNo, nil
}

func (g BizGateway) CancelByBizTx(ctx context.Context, tx *gorm.DB, bizType, bizNo string) error {
	return g.S.CancelByBizTx(ctx, tx, bizType, bizNo)
}

// tenantOf 当前上下文租户（没有租户上下文时为 0，即默认租户）。
func tenantOf(ctx context.Context) int64 {
	tid, _ := auth.TenantFromContext(ctx)
	return tid
}
