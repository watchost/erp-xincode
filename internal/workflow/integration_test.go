// Copyright 2026 zhouhouping. All Rights Reserved.

package workflow_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	apperrors "erp-system/internal/pkg/errors"
	purchaseDto "erp-system/internal/purchase/dto"
	purchaseService "erp-system/internal/purchase/service"
	"erp-system/internal/testsupport"
	"erp-system/internal/workflow"
)

func code(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// mkUser 建一个启用用户；deptID<=0 表示无部门；roleCode 非空时绑定该角色。
func mkUser(t *testing.T, gdb *gorm.DB, name string, deptID int64, roleCode string) int64 {
	t.Helper()
	var id int64
	var dept any
	if deptID > 0 {
		dept = deptID
	}
	if err := gdb.Raw(`INSERT INTO sys_user (username, password_hash, real_name, phone, status, dept_id)
		VALUES (?, 'x', ?, '', 1, ?) RETURNING id`, name, name, dept).Scan(&id).Error; err != nil {
		t.Fatalf("建用户 %s 失败: %v", name, err)
	}
	if roleCode != "" {
		if err := gdb.Exec(`INSERT INTO sys_user_role (user_id, role_id) SELECT ?, id FROM sys_role WHERE code = ?`, id, roleCode).Error; err != nil {
			t.Fatalf("绑定角色失败: %v", err)
		}
	}
	return id
}

func mkDept(t *testing.T, gdb *gorm.DB, code string, parent *int64, leader int64) int64 {
	t.Helper()
	var id int64
	var l any
	if leader > 0 {
		l = leader
	}
	if err := gdb.Raw(`INSERT INTO sys_department (code, name, parent_id, leader_id) VALUES (?, ?, ?, ?) RETURNING id`,
		code, code, parent, l).Scan(&id).Error; err != nil {
		t.Fatalf("建部门失败: %v", err)
	}
	return id
}

type org struct {
	starter, leader, mgr1, mgr2, other int64
}

// mkOrg：部门（负责人 leader）下有发起人 starter；mgr1/mgr2 拥有 manager 角色；other 为普通用户。
// 测试库里已有的 manager 角色用户会干扰或签断言，所以先清空该角色的成员。
func mkOrg(t *testing.T, gdb *gorm.DB, p string) org {
	gdb.Exec(`DELETE FROM sys_user_role WHERE role_id IN (SELECT id FROM sys_role WHERE code = 'manager')`)
	leader := mkUser(t, gdb, p+"_leader", 0, "")
	dept := mkDept(t, gdb, p+"_dept", nil, leader)
	gdb.Exec(`UPDATE sys_user SET dept_id = ? WHERE id = ?`, dept, leader)
	return org{
		starter: mkUser(t, gdb, p+"_starter", dept, ""),
		leader:  leader,
		mgr1:    mkUser(t, gdb, p+"_mgr1", 0, "manager"),
		mgr2:    mkUser(t, gdb, p+"_mgr2", 0, "manager"),
		other:   mkUser(t, gdb, p+"_other", 0, ""),
	}
}

func as(uid int64) context.Context { return testsupport.CtxWithUser(uid) }

func todoOf(t *testing.T, svc *workflow.Service, uid int64) []workflow.TaskVO {
	t.Helper()
	list, _, err := svc.ListTodo(as(uid), 1, 50)
	if err != nil {
		t.Fatalf("查待办失败: %v", err)
	}
	return list
}

func onlyTodo(t *testing.T, svc *workflow.Service, uid int64, bizNo string) workflow.TaskVO {
	t.Helper()
	var hit []workflow.TaskVO
	for _, v := range todoOf(t, svc, uid) {
		if v.BizNo == bizNo {
			hit = append(hit, v)
		}
	}
	if len(hit) != 1 {
		t.Fatalf("用户 %d 对 %s 应恰有 1 条待办，实际 %d", uid, bizNo, len(hit))
	}
	return hit[0]
}

func orderStatus(t *testing.T, gdb *gorm.DB, orderNo string) int {
	var s int
	gdb.Table("pur_purchase_order").Select("status").Where("order_no = ?", orderNo).Scan(&s)
	return s
}

// 装配：采购 service + 工作流（使用 0021 默认部署的 purchase_order 2 级流程）
func setup(t *testing.T, p string) (*gorm.DB, *workflow.Service, testsupport.PurchaseDeps, org, string) {
	gdb := testsupport.NewDB(t)
	o := mkOrg(t, gdb, p)
	fx := testsupport.CreateFixture(t, gdb, p)
	wh := testsupport.NewWarehouseService(gdb)
	fin := testsupport.NewFinanceService(gdb)
	deps := testsupport.NewPurchaseService(gdb, wh, fin)
	wf := workflow.NewService(gdb)
	wf.Register(purchaseService.BizTypePurchaseOrder, deps.Service)
	deps.Service.SetWorkflow(workflow.BizGateway{S: wf})

	vo, err := deps.Service.CreateOrder(as(o.starter), purchaseDto.CreatePurchaseOrderReq{
		SupplierID: fx.Supplier.ID,
		Items:      []purchaseDto.PurchaseOrderItemReq{{MaterialID: fx.Material.ID, Qty: testsupport.D("10"), Price: testsupport.D("25")}},
	})
	if err != nil {
		t.Fatalf("建采购单失败: %v", err)
	}
	return gdb, wf, deps, o, vo.OrderNo
}

// 主链路：提交 → 部门负责人通过 → 管理层或签（任一人）通过 → 订单已审批（20），另一位管理层的待办被取消。
func TestPurchaseOrder_TwoLevelApproval(t *testing.T) {
	gdb, wf, deps, o, orderNo := setup(t, "wf2lv")

	instNo, err := deps.Service.SubmitOrder(as(o.starter), orderNo)
	if err != nil {
		t.Fatalf("提交审批失败: %v", err)
	}
	if orderStatus(t, gdb, orderNo) != purchaseService.OrderStatusPendingApproval {
		t.Fatal("提交后订单应为审批中(15)")
	}
	if err := deps.Service.ApproveOrder(as(o.starter), orderNo); code(err) != 40010 {
		t.Fatalf("启用流程后直接审批应被拒(40010)，实际 %v", err)
	}

	t1 := onlyTodo(t, wf, o.leader, orderNo)
	if t1.NodeCode != "dept_leader" {
		t.Fatalf("第一级应为部门负责人，实际 %s", t1.NodeCode)
	}
	if err := wf.Approve(as(o.other), t1.TaskID, ""); code(err) != 80005 {
		t.Fatalf("非审批人审批应返回 80005，实际 %v", err)
	}
	if err := wf.Approve(as(o.leader), t1.TaskID, "同意"); err != nil {
		t.Fatalf("一级审批失败: %v", err)
	}
	if orderStatus(t, gdb, orderNo) != purchaseService.OrderStatusPendingApproval {
		t.Fatal("一级通过后仍应为审批中")
	}

	t2 := onlyTodo(t, wf, o.mgr1, orderNo)
	onlyTodo(t, wf, o.mgr2, orderNo)
	if err := wf.Approve(as(o.mgr1), t2.TaskID, "同意"); err != nil {
		t.Fatalf("二级审批失败: %v", err)
	}
	if orderStatus(t, gdb, orderNo) != purchaseService.OrderStatusApproved {
		t.Fatal("两级通过后订单应为已审批(20)")
	}
	for _, v := range todoOf(t, wf, o.mgr2) {
		if v.BizNo == orderNo {
			t.Fatal("或签：另一位管理层的待办应已取消")
		}
	}
	detail, err := wf.GetInstance(as(o.starter), instNo)
	if err != nil {
		t.Fatalf("查审批历史失败: %v", err)
	}
	if detail.Instance.Status != workflow.InstApproved || len(detail.Approvals) != 3 {
		t.Fatalf("实例应为通过、审批记录 3 条(start+2 approve)，实际 状态 %d 记录 %d", detail.Instance.Status, len(detail.Approvals))
	}
	done, _, _ := wf.ListDone(as(o.leader), 1, 20)
	if len(done) == 0 {
		t.Fatal("部门负责人的已办应包含该任务")
	}
}

// 驳回：意见必填；驳回后订单退回草稿，可重新提交。
func TestPurchaseOrder_RejectBackToDraftAndResubmit(t *testing.T) {
	gdb, wf, deps, o, orderNo := setup(t, "wfrej")
	if _, err := deps.Service.SubmitOrder(as(o.starter), orderNo); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	t1 := onlyTodo(t, wf, o.leader, orderNo)
	if err := wf.Reject(as(o.leader), t1.TaskID, "  "); code(err) != 80008 {
		t.Fatalf("驳回意见为空应返回 80008，实际 %v", err)
	}
	if err := wf.Reject(as(o.leader), t1.TaskID, "价格偏高"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	if orderStatus(t, gdb, orderNo) != purchaseService.OrderStatusDraft {
		t.Fatal("驳回后订单应退回草稿")
	}
	if err := wf.Approve(as(o.leader), t1.TaskID, ""); code(err) != 80006 {
		t.Fatalf("已驳回任务再次审批应返回 80006，实际 %v", err)
	}
	if _, err := deps.Service.SubmitOrder(as(o.starter), orderNo); err != nil {
		t.Fatalf("驳回后应可重新提交: %v", err)
	}
	onlyTodo(t, wf, o.leader, orderNo)
}

// 同一单据只能有一个运行中的流程；发起人撤销后订单回到草稿，非发起人不能撤销。
func TestWorkflow_SingleRunningAndCancel(t *testing.T) {
	gdb, wf, deps, o, orderNo := setup(t, "wfcan")
	instNo, err := deps.Service.SubmitOrder(as(o.starter), orderNo)
	if err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	if _, err := wf.Start(as(o.starter), workflow.StartReq{BizType: "purchase_order", BizNo: orderNo}); code(err) != 80007 {
		t.Fatalf("重复启动应返回 80007，实际 %v", err)
	}
	if err := wf.Cancel(as(o.other), instNo, ""); code(err) != 80011 {
		t.Fatalf("非发起人撤销应返回 80011，实际 %v", err)
	}
	if err := wf.Cancel(as(o.starter), instNo, "改单"); err != nil {
		t.Fatalf("撤销失败: %v", err)
	}
	if orderStatus(t, gdb, orderNo) != purchaseService.OrderStatusDraft {
		t.Fatal("撤销后订单应回到草稿")
	}
	if len(todoOf(t, wf, o.leader)) != 0 {
		t.Fatal("撤销后不应留下待办")
	}
}

// 转办：原任务变为已转办，目标用户获得待办并可完成审批。
// 加签：本节点变为会签，原审批人与加签人都通过后才进入下一级。
func TestWorkflow_TransferAndAddSign(t *testing.T) {
	_, wf, deps, o, orderNo := setup(t, "wfts")
	if _, err := deps.Service.SubmitOrder(as(o.starter), orderNo); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	t1 := onlyTodo(t, wf, o.leader, orderNo)
	if err := wf.Transfer(as(o.leader), t1.TaskID, o.leader, ""); code(err) != 80012 {
		t.Fatalf("转办给本节点已有待办的人应返回 80012，实际 %v", err)
	}
	if err := wf.Transfer(as(o.leader), t1.TaskID, o.other, "请代审"); err != nil {
		t.Fatalf("转办失败: %v", err)
	}
	if len(todoOf(t, wf, o.leader)) != 0 {
		t.Fatal("转办后原审批人不应再有待办")
	}
	tOther := onlyTodo(t, wf, o.other, orderNo)

	// other 加签 leader：本节点需两人都通过
	if err := wf.AddSign(as(o.other), tOther.TaskID, o.leader, "请负责人一起看"); err != nil {
		t.Fatalf("加签失败: %v", err)
	}
	tLeader := onlyTodo(t, wf, o.leader, orderNo)
	if err := wf.Approve(as(o.other), tOther.TaskID, ""); err != nil {
		t.Fatalf("审批失败: %v", err)
	}
	if len(todoOf(t, wf, o.mgr1)) != 0 {
		t.Fatal("加签后为会签：加签人未审批前不应进入下一级")
	}
	if err := wf.Approve(as(o.leader), tLeader.TaskID, ""); err != nil {
		t.Fatalf("加签人审批失败: %v", err)
	}
	onlyTodo(t, wf, o.mgr1, orderNo)
}

// 会签(all) + 指定人(user) + 条件节点：金额 <= 阈值时跳过第二个节点；找不到审批人时启动失败并整体回滚。
func TestWorkflow_CountersignConditionAndNoAssignee(t *testing.T) {
	gdb := testsupport.NewDB(t)
	o := mkOrg(t, gdb, "wfall")
	wf := workflow.NewService(gdb)
	ctx := as(o.starter)
	bizType := fmt.Sprintf("it_all_%d", o.starter)

	def, _ := json.Marshal(map[string]any{"nodes": []map[string]any{
		{"code": "start", "type": "start"},
		{"code": "both", "type": "approval", "name": "会签", "sign_mode": "all",
			"assignee": map[string]any{"type": "user", "value": []int64{o.mgr1, o.mgr2}}},
		{"code": "big", "type": "approval", "name": "大额",
			"assignee": map[string]any{"type": "user", "value": o.leader},
			"condition": map[string]any{"field": "amount", "op": ">", "value": "10000"}},
		{"code": "end", "type": "end"},
	}})
	d, err := wf.CreateDefinition(ctx, workflow.CreateDefinitionReq{Code: bizType, Name: "会签测试", Definition: def})
	if err != nil {
		t.Fatalf("建定义失败: %v", err)
	}
	if _, err := wf.Start(ctx, workflow.StartReq{BizType: bizType, BizNo: "X1"}); code(err) != 80002 {
		t.Fatalf("未部署时启动应返回 80002，实际 %v", err)
	}
	if err := wf.DeployDefinition(ctx, d.ID); err != nil {
		t.Fatalf("部署失败: %v", err)
	}

	inst, err := wf.Start(ctx, workflow.StartReq{BizType: bizType, BizNo: "X1",
		Variables: map[string]decimal.Decimal{"amount": testsupport.D("500")}})
	if err != nil {
		t.Fatalf("启动失败: %v", err)
	}
	a := onlyTodo(t, wf, o.mgr1, "X1")
	b := onlyTodo(t, wf, o.mgr2, "X1")
	if err := wf.Approve(as(o.mgr1), a.TaskID, ""); err != nil {
		t.Fatalf("会签一人通过失败: %v", err)
	}
	if det, _ := wf.GetInstance(ctx, inst.InstanceNo); det.Instance.Status != workflow.InstRunning {
		t.Fatal("会签：只有一人通过时流程应仍在审批中")
	}
	if err := wf.Approve(as(o.mgr2), b.TaskID, ""); err != nil {
		t.Fatalf("会签二人通过失败: %v", err)
	}
	if det, _ := wf.GetInstance(ctx, inst.InstanceNo); det.Instance.Status != workflow.InstApproved {
		t.Fatal("金额 500 不满足 >10000，应跳过大额节点直接通过")
	}

	// 停用两个审批人 → 找不到审批人，启动失败，且不留下实例
	gdb.Exec(`UPDATE sys_user SET status = 0 WHERE id IN (?, ?)`, o.mgr1, o.mgr2)
	if _, err := wf.Start(ctx, workflow.StartReq{BizType: bizType, BizNo: "X2"}); code(err) != 80009 {
		t.Fatalf("无可用审批人应返回 80009，实际 %v", err)
	}
	var n int64
	gdb.Table("wf_instance").Where("biz_type = ? AND biz_no = 'X2'", bizType).Count(&n)
	if n != 0 {
		t.Fatal("启动失败应整体回滚，不留实例")
	}
}
