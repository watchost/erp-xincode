// Copyright 2026 zhouhouping. All Rights Reserved.

package message_test

import (
	"context"
	"encoding/json"
	stdErrors "errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/message"
	"erp-system/internal/pkg/auth"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
	warehouseDto "erp-system/internal/warehouse/dto"
)

func code(err error) int {
	var ae *apperrors.AppError
	if stdErrors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func mkUser(t *testing.T, gdb *gorm.DB, tenantID int64, roleCode string) int64 {
	t.Helper()
	var id int64
	name := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	if err := gdb.Raw(`INSERT INTO sys_user (username, password_hash, real_name, phone, status, tenant_id)
		VALUES (?, 'x', ?, '', 1, ?) RETURNING id`, name, name, tenantID).Scan(&id).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	if roleCode != "" {
		gdb.Exec(`INSERT INTO sys_user_role (user_id, role_id) SELECT ?, id FROM sys_role WHERE code = ?`, id, roleCode)
	}
	return id
}

func countMsg(gdb *gorm.DB, receiver int64, category string) int64 {
	var n int64
	gdb.Table("msg_message").Where("receiver_id = ? AND category = ?", receiver, category).Count(&n)
	return n
}

// §4.8 验收：库存低于安全库存时自动发送站内信；只在跌破时提醒一次；不发给其他租户的用户。
func TestStockLow_NotifiesOnceWhenCrossingBelow(t *testing.T) {
	gdb := testsupport.NewDB(t)
	fx := testsupport.CreateFixture(t, gdb, "stklow")
	wh := testsupport.NewWarehouseService(gdb)
	msgSvc := message.NewService(gdb)
	wh.SetStockAlerter(msgSvc)
	ctx := testsupport.Ctx()

	keeper := mkUser(t, gdb, 0, "warehouse")
	other := mkUser(t, gdb, 9, "warehouse") // 其他租户的仓管，不应收到
	gdb.Exec("UPDATE mdm_material SET min_stock = 10 WHERE id = ?", fx.Material.ID)

	if _, err := wh.Inbound(ctx, warehouseDto.InboundScanReq{
		MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
		LocationCode: fx.Location.LocationCode, Qty: testsupport.D("15"), UnitCost: testsupport.D("2"),
	}); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	out := func(q string) {
		t.Helper()
		if _, err := wh.Outbound(ctx, warehouseDto.OutboundScanReq{
			MaterialCode: fx.Material.MaterialCode, WarehouseCode: fx.Warehouse.Code,
			LocationCode: fx.Location.LocationCode, Qty: testsupport.D(q),
		}); err != nil {
			t.Fatalf("出库 %s 失败: %v", q, err)
		}
	}
	out("4") // 15 → 11，仍 ≥ 10
	if n := countMsg(gdb, keeper, message.CategoryStockLow); n != 0 {
		t.Fatalf("未跌破安全库存不应提醒，实际 %d", n)
	}
	out("3") // 11 → 8，跌破
	if n := countMsg(gdb, keeper, message.CategoryStockLow); n != 1 {
		t.Fatalf("跌破安全库存应提醒 1 次，实际 %d", n)
	}
	out("2") // 8 → 6，已低于，不重复
	if n := countMsg(gdb, keeper, message.CategoryStockLow); n != 1 {
		t.Fatalf("已低于安全库存后不应重复提醒，实际 %d", n)
	}
	if n := countMsg(gdb, other, message.CategoryStockLow); n != 0 {
		t.Fatal("其他租户的用户不应收到预警")
	}
	var m message.MsgMessage
	gdb.Where("receiver_id = ? AND category = ?", keeper, message.CategoryStockLow).First(&m)
	if m.BizNo != fx.Material.MaterialCode || m.TenantID != 0 {
		t.Fatalf("预警内容不正确: %+v", m)
	}
}

// 站内信与业务同事务：事务回滚时消息不落库。
func TestNotifyTx_RollsBackWithBusiness(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := message.NewService(gdb)
	u := mkUser(t, gdb, 0, "")
	errBiz := stdErrors.New("业务失败")
	err := gdb.Transaction(func(tx *gorm.DB) error {
		if err := svc.NotifyTx(context.Background(), tx, message.Event{Category: message.CategorySystem, Receivers: []int64{u}, Title: "x"}); err != nil {
			return err
		}
		return errBiz
	})
	if !stdErrors.Is(err, errBiz) {
		t.Fatalf("期望业务错误，实际 %v", err)
	}
	if countMsg(gdb, u, message.CategorySystem) != 0 {
		t.Fatal("事务回滚后不应留下消息")
	}
}

// 收件箱按 租户 + 接收人 隔离；未读数、单条已读、全部已读。
func TestInbox_ScopedByTenantAndReceiver(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := message.NewService(gdb)
	u1 := mkUser(t, gdb, 0, "")
	u2 := mkUser(t, gdb, 0, "")
	ins := func(tid, uid int64, cat string) int64 {
		var id int64
		gdb.Raw(`INSERT INTO msg_message (tenant_id, receiver_id, category, title) VALUES (?, ?, ?, 't') RETURNING id`, tid, uid, cat).Scan(&id)
		return id
	}
	ins(0, u1, message.CategoryApprovalTodo)
	ins(0, u1, message.CategoryStockLow)
	other := ins(0, u2, message.CategoryStockLow)
	foreign := ins(7, u1, message.CategorySystem) // 同一 user_id 但属于租户 7

	ctx := auth.WithUserID(auth.WithTenantID(context.Background(), 0), u1)
	total, by, err := svc.UnreadCount(ctx)
	if err != nil || total != 2 || by[message.CategoryApprovalTodo] != 1 || by[message.CategoryStockLow] != 1 {
		t.Fatalf("u1 未读应为 2（各类 1），实际 %d %v err=%v", total, by, err)
	}
	list, n, _ := svc.List(ctx, "", false, 1, 20)
	if n != 2 || len(list) != 2 {
		t.Fatalf("u1 收件箱应 2 条，实际 %d", n)
	}
	if err := svc.MarkRead(ctx, other); code(err) != 82001 {
		t.Fatalf("不能标记他人的消息，期望 82001，实际 %v", err)
	}
	if err := svc.MarkRead(ctx, foreign); code(err) != 82001 {
		t.Fatalf("不能标记其他租户的消息，期望 82001，实际 %v", err)
	}
	if updated, _ := svc.MarkAllRead(ctx); updated != 2 {
		t.Fatalf("全部已读应更新 2 条，实际 %d", updated)
	}
	if total, _, _ := svc.UnreadCount(ctx); total != 0 {
		t.Fatal("全部已读后未读数应为 0")
	}
}

type fakeWebhook struct{ sent []int64 }

func (f *fakeWebhook) Name() string { return "webhook" }
func (f *fakeWebhook) Send(_ context.Context, _ string, m *message.MsgMessage) error {
	f.sent = append(f.sent, m.ID)
	return nil
}

// 外部渠道：订阅了才派发；webhook 发出、email 未配置记为 skipped（不伪造成功）；同一条不重复派发。
func TestDispatch_SentAndSkipped(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := message.NewService(gdb)
	hook := &fakeWebhook{}
	svc.SetChannel(hook)
	u := mkUser(t, gdb, 0, "")
	plain := mkUser(t, gdb, 0, "")

	ctx := auth.WithUserID(auth.WithTenantID(context.Background(), 0), u)
	if err := svc.UpsertSubscription(ctx, message.SubscriptionReq{Category: message.CategoryStockLow, Channel: "webhook", Target: "https://example.invalid/hook", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpsertSubscription(ctx, message.SubscriptionReq{Category: message.CategoryStockLow, Channel: "email", Target: "a@b.c", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.UpsertSubscription(ctx, message.SubscriptionReq{Category: message.CategoryStockLow, Channel: "webhook", Target: "http://insecure", Enabled: true}); code(err) != 82004 {
		t.Fatalf("非 https 的 Webhook 应拒绝(82004)，实际 %v", err)
	}
	if err := svc.Notify(ctx, message.Event{Category: message.CategoryStockLow, Receivers: []int64{u, plain}, Title: "低库存"}); err != nil {
		t.Fatal(err)
	}
	var plainMsg message.MsgMessage
	gdb.Where("receiver_id = ?", plain).First(&plainMsg)
	if plainMsg.DispatchStatus != message.DispatchNone {
		t.Fatal("没有订阅外部渠道的接收人不应进入派发队列")
	}

	n, err := svc.DispatchOnce(context.Background(), 50)
	if err != nil || n != 1 {
		t.Fatalf("应派发 1 条，实际 %d err=%v", n, err)
	}
	var m message.MsgMessage
	gdb.Where("receiver_id = ?", u).First(&m)
	var res map[string]string
	if m.ChannelResult != nil {
		_ = json.Unmarshal(*m.ChannelResult, &res)
	}
	if m.DispatchStatus != message.DispatchDone || res["webhook"] != "sent" || len(res["email"]) < 7 || res["email"][:7] != "skipped" {
		t.Fatalf("派发结果不正确: status=%d result=%v", m.DispatchStatus, res)
	}
	if len(hook.sent) != 1 || hook.sent[0] != m.ID {
		t.Fatalf("webhook 应发送 1 次，实际 %v", hook.sent)
	}
	if n, _ := svc.DispatchOnce(context.Background(), 50); n != 0 {
		t.Fatal("已派发的消息不应重复派发")
	}
}
