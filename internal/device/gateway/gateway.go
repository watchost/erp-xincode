// Copyright 2026 zhouhouping. All Rights Reserved.

// Package gateway 实现设备接入 WebSocket 网关（docs/development/features/device.md §3）。
//
// 鉴权：ws://host:port/ws?api_key=xxx&timestamp=xxx&sign=xxx，
// sign = hex(HMAC-SHA256(key=api_secret_hash, message=api_key+timestamp))，
// timestamp 为秒级 Unix 时间，允许 ±5min 时钟偏移。服务端只存 secret 的
// SHA256 摘要（api_secret_hash 列）并以该摘要作为 HMAC 密钥，不落明文。
package gateway

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"erp-system/internal/device/model"
	"erp-system/internal/device/repository"
)

const (
	// clockSkew 鉴权时间戳允许偏移。
	clockSkew = 5 * time.Minute
	// writeWait 单次写超时。
	writeWait = 10 * time.Second
	// pongWait 读侧等待 pong/任意消息的最大空闲（须大于心跳超时的清扫周期）。
	pongWait = 90 * time.Second
	// sendBuf 每连接下行缓冲，写满即断开慢连接。
	sendBuf = 64
)

// Envelope 是网关与设备之间的统一消息信封。
type Envelope struct {
	MessageID string          `json:"message_id"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	AckID     string          `json:"ack_id,omitempty"` // type=ack 时回填被确认的消息 ID
	Timestamp int64           `json:"timestamp"`
}

// conn 表示一台在线设备的长连接。
type conn struct {
	device   *model.DevDevice
	ws       *websocket.Conn
	send     chan []byte
	lastSeen time.Time
}

// Hub 维护 deviceCode → 连接的注册表，处理上下行消息与 ack 关联。
type Hub struct {
	deviceRepo repository.DeviceRepository
	msgRepo    repository.DeviceMessageRepository

	mu      sync.RWMutex
	conns   map[string]*conn // key: device_code
	pending map[string]chan Envelope

	upgrader websocket.Upgrader
}

func NewHub(deviceRepo repository.DeviceRepository, msgRepo repository.DeviceMessageRepository) *Hub {
	return &Hub{
		deviceRepo: deviceRepo,
		msgRepo:    msgRepo,
		conns:      make(map[string]*conn),
		pending:    make(map[string]chan Envelope),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			// 设备接入走独立端口 + HMAC 鉴权，跨源校验由签名承担。
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// VerifySign 校验接入签名：sign = HMAC-SHA256(secretHash, api_key+timestamp)。
func VerifySign(secretHash, apiKey, timestamp, sign string) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if d := time.Since(time.Unix(ts, 0)); d > clockSkew || d < -clockSkew {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secretHash))
	mac.Write([]byte(apiKey + timestamp))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sign))
}

// IsOnline 查询设备是否持有活跃连接。
func (h *Hub) IsOnline(deviceCode string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.conns[deviceCode]
	return ok
}

// OnlineCount 返回当前在线连接数（测试与监控用）。
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// HandleWS 是 /ws 的 HTTP 入口：鉴权 → 升级 → 注册 → 读写泵。
func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	apiKey := r.URL.Query().Get("api_key")
	timestamp := r.URL.Query().Get("timestamp")
	sign := r.URL.Query().Get("sign")
	if apiKey == "" || timestamp == "" || sign == "" {
		http.Error(w, "missing auth params", http.StatusUnauthorized)
		return
	}
	device, err := h.deviceRepo.FindByAPIKey(apiKey)
	if err != nil || device.APISecretHash == "" {
		http.Error(w, "unknown api_key", http.StatusUnauthorized)
		return
	}
	if !VerifySign(device.APISecretHash, apiKey, timestamp, sign) {
		http.Error(w, "bad sign", http.StatusUnauthorized)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	c := &conn{device: device, ws: ws, send: make(chan []byte, sendBuf), lastSeen: time.Now()}
	h.register(c)

	now := time.Now()
	device.Status = model.DeviceStatusOnline
	device.LastHeartbeat = now
	device.LastIP = r.RemoteAddr
	_ = h.deviceRepo.Update(device)
	h.persistUp(device, model.MsgTypeOnline, nil)

	go h.writePump(c)
	h.readPump(c)

	if h.unregister(c) {
		// 只有「注册表里仍是本连接」才置离线：被新连接踢掉的旧连接不能误置。
		device.Status = model.DeviceStatusOffline
		_ = h.deviceRepo.Update(device)
		h.persistUp(device, model.MsgTypeOffline, nil)
	}
}

func (h *Hub) register(c *conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// 同设备重复连接：踢掉旧连接（后发优先）。
	if old, ok := h.conns[c.device.DeviceCode]; ok {
		close(old.send)
		delete(h.conns, c.device.DeviceCode)
	}
	h.conns[c.device.DeviceCode] = c
}

// unregister 仅当注册表里仍是本连接时移除并返回 true（被踢的旧连接不能顶掉新连接）。
func (h *Hub) unregister(c *conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.conns[c.device.DeviceCode]; ok && cur == c {
		delete(h.conns, c.device.DeviceCode)
		return true
	}
	return false
}

func (h *Hub) readPump(c *conn) {
	c.ws.SetReadLimit(1 << 20)
	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.lastSeen = time.Now()
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		c.lastSeen = time.Now()
		var env Envelope
		if json.Unmarshal(data, &env) != nil {
			continue
		}
		h.handleUp(c, &env)
	}
}

func (h *Hub) writePump(c *conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case data, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if c.ws.WriteMessage(websocket.TextMessage, data) != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if c.ws.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		}
	}
}

// handleUp 处理上行消息：ack 关联待确认命令，其余落库并刷新心跳。
func (h *Hub) handleUp(c *conn, env *Envelope) {
	if env.Type == model.MsgTypeAck {
		h.mu.Lock()
		ch, ok := h.pending[env.AckID]
		delete(h.pending, env.AckID)
		h.mu.Unlock()
		// 先落库再唤醒等待方，保证 SendAndWait 返回时消息状态已可见。
		h.ackMessage(env.AckID, true)
		if ok {
			ch <- *env
		}
		return
	}

	c.device.LastHeartbeat = time.Now()
	_ = h.deviceRepo.Update(c.device)
	h.persistUp(c.device, env.Type, env.Payload)
}

func (h *Hub) persistUp(device *model.DevDevice, msgType string, payload json.RawMessage) {
	if h.msgRepo == nil {
		return
	}
	msg := &model.DevDeviceMessage{
		MessageID: newMessageID(),
		DeviceID:  device.ID,
		Direction: model.MessageDirectionUp,
		MsgType:   msgType,
		Payload:   payload,
		Status:    model.MessageStatusSuccess,
		TenantID:  device.TenantID,
	}
	_ = h.msgRepo.Create(msg)
}

// Send 向在线设备推送一条已落库的下行消息，不等待 ack（异步指令）。
func (h *Hub) Send(deviceCode string, env Envelope) bool {
	data, err := json.Marshal(env)
	if err != nil {
		return false
	}
	h.mu.RLock()
	c, ok := h.conns[deviceCode]
	h.mu.RUnlock()
	if !ok {
		return false
	}
	select {
	case c.send <- data:
		return true
	default:
		// 慢连接缓冲写满：断开让设备重连，避免无界积压。
		close(c.send)
		return false
	}
}

// SendAndWait 推送并等待设备 ack；timeout 内未 ack 返回 false。
// 调用方（command 流程）负责据此把消息状态推进到 成功/失败。
func (h *Hub) SendAndWait(deviceCode string, env Envelope, timeout time.Duration) (Envelope, bool) {
	h.mu.Lock()
	if _, ok := h.conns[deviceCode]; !ok {
		h.mu.Unlock()
		return Envelope{}, false
	}
	ch := make(chan Envelope, 1)
	h.pending[env.MessageID] = ch
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.pending, env.MessageID)
		h.mu.Unlock()
	}()

	if !h.Send(deviceCode, env) {
		return Envelope{}, false
	}
	select {
	case ack := <-ch:
		return ack, true
	case <-time.After(timeout):
		return Envelope{}, false
	}
}

// ackMessage 把下行消息标记为已确认/失败。
func (h *Hub) ackMessage(messageID string, ok bool) {
	if h.msgRepo == nil {
		return
	}
	msg, err := h.msgRepo.FindByMessageID(messageID)
	if err != nil {
		return
	}
	now := time.Now()
	msg.AckedAt = &now
	if ok {
		msg.Status = model.MessageStatusSuccess
	} else {
		msg.Status = model.MessageStatusFailed
	}
	_ = h.msgRepo.Update(msg)
}

// FailMessage 显式把下行消息标记失败（超时未 ack）。
func (h *Hub) FailMessage(messageID string) { h.ackMessage(messageID, false) }

// SweepOffline 把超过 heartbeatTimeout 无心跳的在线设备置为离线；调用方按 10s 周期触发。
func (h *Hub) SweepOffline(heartbeatTimeout time.Duration) (int64, error) {
	return h.deviceRepo.MarkOfflineBefore(time.Now().Add(-heartbeatTimeout))
}

// CloseAll 关停网关时断开全部连接（服务优雅退出）。
func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for code, c := range h.conns {
		close(c.send)
		delete(h.conns, code)
	}
}

var msgIDSeq struct {
	mu sync.Mutex
	n  int64
}

// newMessageID 生成进程内唯一消息 ID（前缀 DEVMSG-，纳秒+序号防撞）。
func newMessageID() string {
	msgIDSeq.mu.Lock()
	defer msgIDSeq.mu.Unlock()
	msgIDSeq.n++
	return "DEVMSG-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatInt(msgIDSeq.n, 36)
}

// Shutdown 关闭所有连接（http.Server.Shutdown 前的钩子）。
func (h *Hub) Shutdown(ctx context.Context) error {
	h.CloseAll()
	return nil
}

// NewMessageID 生成消息 ID（service 层下发命令时用同一格式）。
func NewMessageID() string { return newMessageID() }
