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
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"erp-system/internal/device/model"
	"erp-system/internal/device/repository"
	"erp-system/internal/pkg/logger"
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
	device *model.DevDevice
	ws     *websocket.Conn
	send   chan []byte
	// done 关闭即令写泵退出；closeOnce 保证踢连接/慢连接/关停等多路并发
	// 关闭只执行一次——send 通道永不关闭，杜绝 close/send 竞态 panic（P1.5 S2）。
	done      chan struct{}
	closeOnce sync.Once
	lastSeen  atomic.Int64 // UnixNano：读泵写、心跳清扫读
}

func newConn(device *model.DevDevice, ws *websocket.Conn) *conn {
	c := &conn{device: device, ws: ws, send: make(chan []byte, sendBuf), done: make(chan struct{})}
	c.touch()
	return c
}

// close 幂等关闭：通知写泵退出并断开 socket（gorilla 的 Close 可与读写并发）。
func (c *conn) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.ws.Close()
	})
}

func (c *conn) touch()            { c.lastSeen.Store(time.Now().UnixNano()) }
func (c *conn) seen() time.Time   { return time.Unix(0, c.lastSeen.Load()) }

// pendingEntry 等待 ack 的下行指令，deviceID 用于 ack 归属校验（P1.5 S3）。
type pendingEntry struct {
	deviceID int64
	ch       chan Envelope
}

// Hub 维护 deviceCode → 连接的注册表，处理上下行消息与 ack 关联。
type Hub struct {
	deviceRepo repository.DeviceRepository
	msgRepo    repository.DeviceMessageRepository

	mu      sync.RWMutex
	conns   map[string]*conn // key: device_code
	pending map[string]pendingEntry

	upgrader websocket.Upgrader
}

func NewHub(deviceRepo repository.DeviceRepository, msgRepo repository.DeviceMessageRepository) *Hub {
	return &Hub{
		deviceRepo: deviceRepo,
		msgRepo:    msgRepo,
		conns:      make(map[string]*conn),
		pending:    make(map[string]pendingEntry),
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

	c := newConn(device, ws)
	h.register(c)
	// 任意退出路径（含 readPump panic 被 http server 回收）都断开 socket、
	// 停掉写泵，杜绝连接泄漏（P1.5 H9）。
	defer c.close()

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
	// 同设备重复连接：踢掉旧连接（后发优先）——停写泵并断开旧 socket，
	// 旧连接的 readPump 随读错误退出（P1.5 H9）。
	if old, ok := h.conns[c.device.DeviceCode]; ok {
		delete(h.conns, c.device.DeviceCode)
		old.close()
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
		c.touch()
		// 纯保活连接也刷新 DB 心跳，否则只有 ping/pong 的设备会被
		// 心跳清扫误判离线（P1.5 H10）。ping 周期 30s，写库频率可接受。
		c.device.LastHeartbeat = time.Now()
		_ = h.deviceRepo.Update(c.device)
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		c.touch()
		var env Envelope
		if json.Unmarshal(data, &env) != nil {
			continue
		}
		h.handleUp(c, &env)
	}
}

func (h *Hub) writePump(c *conn) {
	// 独立 goroutine：任何意外 panic 都不能拖垮整个进程（P1.5 S2）。
	defer func() {
		if r := recover(); r != nil {
			logger.Log(context.Background()).Error("device gateway writePump panic",
				"device_code", c.device.DeviceCode, "panic", r)
		}
	}()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		default:
		}
		select {
		case data := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if c.ws.WriteMessage(websocket.TextMessage, data) != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if c.ws.WriteMessage(websocket.PingMessage, nil) != nil {
				return
			}
		case <-c.done:
			return
		}
	}
}

// handleUp 处理上行消息：ack 关联待确认命令，其余落库并刷新心跳。
func (h *Hub) handleUp(c *conn, env *Envelope) {
	if env.Type == model.MsgTypeAck {
		h.mu.Lock()
		entry, ok := h.pending[env.AckID]
		delete(h.pending, env.AckID)
		h.mu.Unlock()
		// P1.5 S3：ack 必须来自指令目标设备本人——跨设备伪造/串台 ack 直接丢弃。
		if ok && entry.deviceID != c.device.ID {
			return
		}
		// 先落库再唤醒等待方，保证 SendAndWait 返回时消息状态已可见。
		// 迟到 ack（等待方已超时置失败）经 MarkAckedIfPending 的终态守卫被忽略（P1.5 H8）。
		h.ackMessage(env.AckID, c.device.ID, true)
		if ok {
			entry.ch <- *env
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
	case <-c.done:
		return false
	default:
		// 慢连接缓冲写满：断开让设备重连，避免无界积压。
		c.close()
		return false
	}
}

// SendAndWait 推送并等待设备 ack；timeout 内未 ack 返回 false。
// 调用方（command 流程）负责据此把消息状态推进到 成功/失败。
func (h *Hub) SendAndWait(deviceCode string, env Envelope, timeout time.Duration) (Envelope, bool) {
	h.mu.Lock()
	connection, ok := h.conns[deviceCode]
	if !ok {
		h.mu.Unlock()
		return Envelope{}, false
	}
	ch := make(chan Envelope, 1)
	h.pending[env.MessageID] = pendingEntry{deviceID: connection.device.ID, ch: ch}
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

// ackMessage 把下行消息标记为已确认/失败：归属 deviceID 且仍待确认才生效
// （单条条件 UPDATE，串台 ack 与迟到 ack 都无法改写终态）。
func (h *Hub) ackMessage(messageID string, deviceID int64, ok bool) {
	if h.msgRepo == nil {
		return
	}
	status := model.MessageStatusSuccess
	if !ok {
		status = model.MessageStatusFailed
	}
	_, _ = h.msgRepo.MarkAckedIfPending(messageID, deviceID, status, time.Now())
}

// FailMessage 显式把下行消息标记失败（超时未 ack）。
func (h *Hub) FailMessage(messageID string, deviceID int64) { h.ackMessage(messageID, deviceID, false) }

// SweepOffline 把超过 heartbeatTimeout 无心跳的在线设备置为离线；调用方按 10s 周期触发。
// 同时断开注册表里的陈旧连接，避免 DB 已离线但 IsOnline 仍为 true 的幻影在线（P1.5 H10）。
func (h *Hub) SweepOffline(heartbeatTimeout time.Duration) (int64, error) {
	threshold := time.Now().Add(-heartbeatTimeout)
	n, err := h.deviceRepo.MarkOfflineBefore(threshold)

	h.mu.Lock()
	for code, c := range h.conns {
		if c.seen().Before(threshold) {
			delete(h.conns, code)
			c.close()
		}
	}
	h.mu.Unlock()
	return n, err
}

// CloseAll 关停网关时断开全部连接（服务优雅退出）。
func (h *Hub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for code, c := range h.conns {
		delete(h.conns, code)
		c.close()
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
