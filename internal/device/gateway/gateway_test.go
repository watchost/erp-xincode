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
	ts := strconv.FormatInt(time.Now().Add(tsOffset).Unix(), 10)
	url := fmt.Sprintf("%s?api_key=%s&timestamp=%s&sign=%s",
		f.deviceURL, f.vo.APIKey, ts, signFor(f.vo.APIKey, f.vo.APISecret, ts))
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("设备接入失败: %v", err)
	}
	return c
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
