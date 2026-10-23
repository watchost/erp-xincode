// Copyright 2026 zhouhouping. All Rights Reserved.

package gateway_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"erp-system/internal/device/dto"
	"erp-system/internal/device/gateway"
	"erp-system/internal/device/model"
	"erp-system/internal/device/repository"
	deviceService "erp-system/internal/device/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

// signFor 按设备接入协议计算签名：HMAC-SHA256(SHA256(secret), api_key+timestamp)。
func signFor(apiKey, apiSecret, timestamp string) string {
	sum := sha256.Sum256([]byte(apiSecret))
	mac := hmac.New(sha256.New, []byte(hex.EncodeToString(sum[:])))
	mac.Write([]byte(apiKey + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

func errCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

// 鉴权签名：合法通过、过期/篡改拒绝。
func TestVerifySign(t *testing.T) {
	sum := sha256.Sum256([]byte("s3cret"))
	hash := hex.EncodeToString(sum[:])
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	mac := hmac.New(sha256.New, []byte(hash))
	mac.Write([]byte("dk_abc" + ts))
	good := hex.EncodeToString(mac.Sum(nil))

	if !gateway.VerifySign(hash, "dk_abc", ts, good) {
		t.Fatal("合法签名应通过")
	}
	if gateway.VerifySign(hash, "dk_abc", ts, "deadbeef") {
		t.Fatal("错误签名应拒绝")
	}
	stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	if gateway.VerifySign(hash, "dk_abc", stale, good) {
		t.Fatal("过期时间戳应拒绝")
	}
	if gateway.VerifySign(hash, "dk_abc", "not-a-number", good) {
		t.Fatal("非法时间戳应拒绝")
	}
}

type deviceFixture struct {
	gdb       *gorm.DB
	svc       *deviceService.DeviceService
	hub       *gateway.Hub
	server    *httptest.Server
	msgRepo   repository.DeviceMessageRepository
	vo        *dto.DeviceVO
	deviceURL string
}

func setupDevice(t *testing.T) *deviceFixture {
	t.Helper()
	gdb := testsupport.NewDB(t)
	deviceRepo := repository.NewDeviceRepository(gdb)
	msgRepo := repository.NewDeviceMessageRepository(gdb)
	hub := gateway.NewHub(deviceRepo, msgRepo)
	svc := deviceService.NewDeviceService(deviceRepo, msgRepo)
	svc.SetHub(hub)
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWS))
	t.Cleanup(server.Close)

	vo, err := svc.Register(testsupport.Ctx(), dto.DeviceRegisterReq{
		DeviceCode: "DEV-IT-001", Type: model.DeviceTypePDA, Protocol: "ws",
	})
	if err != nil {
		t.Fatalf("注册设备失败: %v", err)
	}
	if vo.APIKey == "" || vo.APISecret == "" {
		t.Fatal("注册应返回 api_key/api_secret")
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	return &deviceFixture{gdb: gdb, svc: svc, hub: hub, server: server, msgRepo: msgRepo, vo: vo, deviceURL: wsURL}
}

func (f *deviceFixture) dial(t *testing.T, tsOffset time.Duration) *websocket.Conn {
	t.Helper()
	return f.dialAs(t, f.vo, tsOffset)
}

func (f *deviceFixture) dialAs(t *testing.T, vo *dto.DeviceVO, tsOffset time.Duration) *websocket.Conn {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Add(tsOffset).Unix(), 10)
	url := fmt.Sprintf("%s?api_key=%s&timestamp=%s&sign=%s",
		f.deviceURL, vo.APIKey, ts, signFor(vo.APIKey, vo.APISecret, ts))
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("设备接入失败: %v", err)
	}
	return c
}

// waitOnline 轮询等待设备进入注册表。
func (f *deviceFixture) waitOnline(t *testing.T, deviceCode string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !f.hub.IsOnline(deviceCode) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !f.hub.IsOnline(deviceCode) {
		t.Fatalf("设备 %s 接入后应在线", deviceCode)
	}
}

// 完整链路：注册 → WS 接入 → 在线 → 下发指令 → ack → 消息状态推进为成功。
func TestGateway_CommandAckFlow(t *testing.T) {
	f := setupDevice(t)
	ctx := testsupport.Ctx()

	// 设备离线时下发 → 60104，消息落库置失败
	if _, err := f.svc.SendCommand(ctx, f.vo.DeviceCode, dto.DeviceCommandReq{
		MsgType: model.MsgTypePrint, Payload: json.RawMessage(`{"label":"A1"}`),
	}); errCode(err) != 60104 {
		t.Fatalf("设备离线应返回 60104，实际 %v", err)
	}
	_, total, err := f.svc.ListMessages(ctx, f.vo.DeviceCode, 2, 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("离线指令应落库 1 条失败消息: total=%d err=%v", total, err)
	}

	c := f.dial(t, 0)
	defer c.Close()

	// 等待连接注册完成
	deadline := time.Now().Add(2 * time.Second)
	for !f.hub.IsOnline(f.vo.DeviceCode) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !f.hub.IsOnline(f.vo.DeviceCode) {
		t.Fatal("设备接入后应在线")
	}

	// 客户端协程：收到下行指令即回 ack
	go func() {
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			var env gateway.Envelope
			if json.Unmarshal(data, &env) != nil {
				continue
			}
			ack, _ := json.Marshal(gateway.Envelope{
				MessageID: gateway.NewMessageID(),
				Type:      model.MsgTypeAck,
				AckID:     env.MessageID,
				Timestamp: time.Now().Unix(),
			})
			if c.WriteMessage(websocket.TextMessage, ack) != nil {
				return
			}
		}
	}()

	vo, err := f.svc.SendCommand(ctx, f.vo.DeviceCode, dto.DeviceCommandReq{
		MsgType: model.MsgTypePrint, Payload: json.RawMessage(`{"label":"A2"}`),
	})
	if err != nil {
		t.Fatalf("在线指令下发失败: %v", err)
	}
	if !vo.Acked || vo.Status != model.MessageStatusSuccess {
		t.Fatalf("指令应被确认: %+v", vo)
	}

	msg, err := f.msgRepo.FindByMessageID(vo.MessageID)
	if err != nil {
		t.Fatalf("查询指令消息失败: %v", err)
	}
	if msg.Status != model.MessageStatusSuccess || msg.AckedAt == nil {
		t.Fatalf("消息应推进为成功并记录 ack 时间: %+v", msg)
	}

	// 上行心跳：刷新 last_heartbeat 并落库
	hb, _ := json.Marshal(gateway.Envelope{MessageID: gateway.NewMessageID(), Type: model.MsgTypeHeartbeat, Timestamp: time.Now().Unix()})
	if err := c.WriteMessage(websocket.TextMessage, hb); err != nil {
		t.Fatalf("发送心跳失败: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for {
		_, upTotal, _ := f.svc.ListMessages(ctx, f.vo.DeviceCode, 1, 1, 20)
		if upTotal >= 2 || time.Now().After(deadline) { // online + heartbeat
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, upTotal, _ := f.svc.ListMessages(ctx, f.vo.DeviceCode, 1, 1, 20)
	if upTotal < 2 {
		t.Fatalf("上行消息（online+heartbeat）应落库，实际 %d 条", upTotal)
	}
}

// 同设备重复连接：旧连接被踢，注册表只保留新连接。
func TestGateway_DuplicateConnectKicksOld(t *testing.T) {
	f := setupDevice(t)

	c1 := f.dial(t, 0)
	deadline := time.Now().Add(2 * time.Second)
	for !f.hub.IsOnline(f.vo.DeviceCode) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.hub.OnlineCount() != 1 {
		t.Fatalf("首个连接后在线数应为 1，实际 %d", f.hub.OnlineCount())
	}

	c2 := f.dial(t, 0)
	defer c2.Close()
	deadline = time.Now().Add(2 * time.Second)
	for f.hub.OnlineCount() != 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if f.hub.OnlineCount() != 1 {
		t.Fatalf("重复连接后在线数应仍为 1，实际 %d", f.hub.OnlineCount())
	}

	// 旧连接应收到关闭帧
	_ = c1.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := c1.ReadMessage(); err == nil {
		t.Fatal("旧连接应被踢断")
	}
}

// 心跳超时清扫：心跳新鲜的设备保持在线，超时的被置为离线。
func TestGateway_SweepOffline(t *testing.T) {
	f := setupDevice(t)

	c := f.dial(t, 0)
	defer c.Close()
	deadline := time.Now().Add(2 * time.Second)
	for !f.hub.IsOnline(f.vo.DeviceCode) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	// 心跳新鲜：清扫不误伤。
	n, err := f.hub.SweepOffline(60 * time.Second)
	if err != nil || n != 0 {
		t.Fatalf("心跳新鲜不应被清扫: n=%d err=%v", n, err)
	}

	// 把最后心跳拨回 2 分钟前，再清扫 → 置离线。
	if err := f.gdb.Model(&model.DevDevice{}).
		Where("device_code = ?", f.vo.DeviceCode).
		Update("last_heartbeat", time.Now().Add(-2*time.Minute)).Error; err != nil {
		t.Fatalf("回拨心跳失败: %v", err)
	}
	n, err = f.hub.SweepOffline(60 * time.Second)
	if err != nil || n != 1 {
		t.Fatalf("超时设备应被清扫 1 台: n=%d err=%v", n, err)
	}
	var dev model.DevDevice
	if err := f.gdb.Where("device_code = ?", f.vo.DeviceCode).First(&dev).Error; err != nil {
		t.Fatal(err)
	}
	if dev.Status != model.DeviceStatusOffline {
		t.Fatalf("超时设备应置为离线，实际 status=%d", dev.Status)
	}
}

// P1.5 S3：B 设备拿 A 设备的指令 message_id 回 ack 属于跨设备伪造——
// 不得推进消息状态、不得唤醒 A 的等待方；A 本人回 ack 才生效。
func TestGateway_CrossDeviceAckRejected(t *testing.T) {
	f := setupDevice(t)
	ctx := testsupport.Ctx()

	// 第二台设备
	voB, err := f.svc.Register(ctx, dto.DeviceRegisterReq{
		DeviceCode: "DEV-IT-002", Type: model.DeviceTypePDA, Protocol: "ws",
	})
	if err != nil {
		t.Fatalf("注册第二台设备失败: %v", err)
	}
	cA := f.dial(t, 0)
	defer cA.Close()
	cB := f.dialAs(t, voB, 0)
	defer cB.Close()
	f.waitOnline(t, f.vo.DeviceCode)
	f.waitOnline(t, voB.DeviceCode)

	// A 的待确认指令落库
	msg := &model.DevDeviceMessage{
		MessageID: gateway.NewMessageID(), DeviceID: f.vo.ID, Direction: model.MessageDirectionDown,
		MsgType: model.MsgTypeCommand, Status: model.MessageStatusPending,
	}
	if err := f.msgRepo.Create(msg); err != nil {
		t.Fatalf("落库指令失败: %v", err)
	}

	// 后台等待 A 的 ack（短超时）
	ackResult := make(chan bool, 1)
	go func() {
		_, ok := f.hub.SendAndWait(f.vo.DeviceCode, gateway.Envelope{
			MessageID: msg.MessageID, Type: model.MsgTypeCommand, Timestamp: time.Now().Unix(),
		}, 500*time.Millisecond)
		ackResult <- ok
	}()

	// A 收到下行指令（本人不回 ack）
	var down gateway.Envelope
	_ = cA.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := cA.ReadMessage()
		if err != nil {
			t.Fatalf("A 应收到下行指令: %v", err)
		}
		if json.Unmarshal(data, &down) == nil && down.MessageID == msg.MessageID {
			break
		}
	}

	// B 伪造 A 的 ack
	forged, _ := json.Marshal(gateway.Envelope{
		MessageID: gateway.NewMessageID(), Type: model.MsgTypeAck,
		AckID: msg.MessageID, Timestamp: time.Now().Unix(),
	})
	if err := cB.WriteMessage(websocket.TextMessage, forged); err != nil {
		t.Fatalf("B 发送伪造 ack 失败: %v", err)
	}

	// 等待方应超时（伪造 ack 不唤醒）
	if ok := <-ackResult; ok {
		t.Fatal("跨设备伪造 ack 不应唤醒等待方")
	}
	// 消息状态不得被伪造 ack 推进
	stored, err := f.msgRepo.FindByMessageID(msg.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.MessageStatusPending {
		t.Fatalf("伪造 ack 不应推进消息状态，实际 status=%d", stored.Status)
	}

	// A 本人回 ack：立即生效
	genuine, _ := json.Marshal(gateway.Envelope{
		MessageID: gateway.NewMessageID(), Type: model.MsgTypeAck,
		AckID: msg.MessageID, Timestamp: time.Now().Unix(),
	})
	if err := cA.WriteMessage(websocket.TextMessage, genuine); err != nil {
		t.Fatalf("A 发送 ack 失败: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for stored.Status == model.MessageStatusPending && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		stored, _ = f.msgRepo.FindByMessageID(msg.MessageID)
	}
	if stored.Status != model.MessageStatusSuccess {
		t.Fatalf("A 本人 ack 应推进为成功，实际 status=%d", stored.Status)
	}
}

// P1.5 H8：超时置失败后迟到的 ack 不能把终态翻转回成功。
func TestGateway_LateAckKeepsTerminalState(t *testing.T) {
	f := setupDevice(t)

	c := f.dial(t, 0)
	defer c.Close()
	f.waitOnline(t, f.vo.DeviceCode)

	msg := &model.DevDeviceMessage{
		MessageID: gateway.NewMessageID(), DeviceID: f.vo.ID, Direction: model.MessageDirectionDown,
		MsgType: model.MsgTypeCommand, Status: model.MessageStatusPending,
	}
	if err := f.msgRepo.Create(msg); err != nil {
		t.Fatalf("落库指令失败: %v", err)
	}

	// 设备收到但迟迟不回 ack → 等待方 200ms 超时
	go func() {
		_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			_, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			var env gateway.Envelope
			if json.Unmarshal(data, &env) != nil || env.MessageID != msg.MessageID {
				continue
			}
			time.Sleep(400 * time.Millisecond) // 迟到 ack
			ack, _ := json.Marshal(gateway.Envelope{
				MessageID: gateway.NewMessageID(), Type: model.MsgTypeAck,
				AckID: env.MessageID, Timestamp: time.Now().Unix(),
			})
			_ = c.WriteMessage(websocket.TextMessage, ack)
			return
		}
	}()

	if _, ok := f.hub.SendAndWait(f.vo.DeviceCode, gateway.Envelope{
		MessageID: msg.MessageID, Type: model.MsgTypeCommand, Timestamp: time.Now().Unix(),
	}, 200*time.Millisecond); ok {
		t.Fatal("设备未在超时内 ack，应返回 false")
	}
	// 等待方按超时流程置失败
	f.hub.FailMessage(msg.MessageID, msg.DeviceID)

	stored, err := f.msgRepo.FindByMessageID(msg.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != model.MessageStatusFailed {
		t.Fatalf("超时后应置失败，实际 status=%d", stored.Status)
	}

	// 迟到 ack 到达后终态不得翻转
	time.Sleep(600 * time.Millisecond)
	stored, _ = f.msgRepo.FindByMessageID(msg.MessageID)
	if stored.Status != model.MessageStatusFailed {
		t.Fatalf("迟到 ack 不应翻转终态，实际 status=%d", stored.Status)
	}
}

// P1.5 S2：慢连接缓冲写满断开、并发 Send/CloseAll 不再触发 close/send 竞态 panic。
func TestGateway_ConcurrentCloseNoPanic(t *testing.T) {
	f := setupDevice(t)

	c := f.dial(t, 0)
	defer c.Close()
	f.waitOnline(t, f.vo.DeviceCode)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 300; i++ {
			// 设备端不读取：缓冲很快写满，触发慢连接断开路径
			f.hub.Send(f.vo.DeviceCode, gateway.Envelope{
				MessageID: gateway.NewMessageID(), Type: model.MsgTypeCommand, Timestamp: time.Now().Unix(),
			})
		}
	}()
	// 并发关停：旧实现 close(c.send) 与慢连接 close(c.send) 会双重 close panic
	time.Sleep(5 * time.Millisecond)
	f.hub.CloseAll()
	<-done
}

// P1.5 H10：清扫不仅置 DB 离线，还要断开注册表里的陈旧连接（消灭幻影在线）。
// 用毫秒级超时模拟「接入后设备再无任何消息/pong」的陈旧连接。
func TestGateway_SweepDisconnectsStaleConn(t *testing.T) {
	f := setupDevice(t)

	c := f.dial(t, 0)
	defer c.Close()
	f.waitOnline(t, f.vo.DeviceCode)

	// 让 lastSeen 自然老化（30s ping 周期内不会有 pong 刷新），然后按 100ms 阈值清扫
	time.Sleep(300 * time.Millisecond)
	if _, err := f.hub.SweepOffline(100 * time.Millisecond); err != nil {
		t.Fatalf("清扫失败: %v", err)
	}
	if f.hub.IsOnline(f.vo.DeviceCode) {
		t.Fatal("被清扫设备的连接应移出注册表")
	}
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("被清扫设备的连接应被断开")
	}
}

// 错误签名/未知 api_key 拒绝升级。
func TestGateway_AuthReject(t *testing.T) {
	f := setupDevice(t)

	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bad := fmt.Sprintf("%s?api_key=%s&timestamp=%s&sign=%s", f.deviceURL, f.vo.APIKey, ts, "bad")
	if _, _, err := websocket.DefaultDialer.Dial(bad, nil); err == nil {
		t.Fatal("错误签名应拒绝接入")
	}
	unknown := fmt.Sprintf("%s?api_key=%s&timestamp=%s&sign=%s", f.deviceURL, "dk_none", ts, "bad")
	if _, _, err := websocket.DefaultDialer.Dial(unknown, nil); err == nil {
		t.Fatal("未知 api_key 应拒绝接入")
	}
}
