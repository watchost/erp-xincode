// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"erp-system/internal/device/dto"
	"erp-system/internal/device/gateway"
	"erp-system/internal/device/model"
	"erp-system/internal/device/repository"
	"erp-system/internal/pkg/errors"
)

// 设备模块错误码使用 601xx 段（6 段其余已被财务占用）。
const (
	errCodeDeviceExists     = 60101
	errCodeDeviceNotFound   = 60102
	errCodeInvalidScanEvent = 60103
	errCodeDeviceOffline    = 60104
	errCodeCommandTimeout   = 60105
)

// commandAckTimeout 下发指令等待设备 ack 的超时。
const commandAckTimeout = 30 * time.Second

type DeviceService struct {
	deviceRepo repository.DeviceRepository
	msgRepo    repository.DeviceMessageRepository
	hub        *gateway.Hub // 可为 nil（未启动 WS 网关时指令一律按离线处理）
}

func NewDeviceService(deviceRepo repository.DeviceRepository, msgRepo repository.DeviceMessageRepository) *DeviceService {
	return &DeviceService{deviceRepo: deviceRepo, msgRepo: msgRepo}
}

// SetHub 注入 WebSocket 网关（main 装配时调用）。
func (s *DeviceService) SetHub(hub *gateway.Hub) { s.hub = hub }

func (s *DeviceService) Register(ctx context.Context, req dto.DeviceRegisterReq) (*dto.DeviceVO, error) {
	existing, _ := s.deviceRepo.FindByCode(req.DeviceCode)
	if existing != nil {
		return nil, errors.New(errCodeDeviceExists, 409, "设备编码已存在")
	}

	var configBytes []byte
	if req.Config != "" {
		configBytes = []byte(req.Config)
	}

	// 生成接入凭据：api_key 可检索，api_secret 只存 SHA256 摘要、明文仅此一次返回。
	apiKey, apiSecret, err := generateCredentials()
	if err != nil {
		return nil, err
	}

	device := &model.DevDevice{
		DeviceCode:    req.DeviceCode,
		Type:          req.Type,
		Brand:         req.Brand,
		Protocol:      req.Protocol,
		Status:        model.DeviceStatusOffline,
		Config:        configBytes,
		APIKey:        apiKey,
		APISecretHash: hashSecret(apiSecret),
	}

	if err := s.deviceRepo.Create(device); err != nil {
		return nil, err
	}

	vo := s.buildDeviceVO(device)
	vo.APIKey = apiKey
	vo.APISecret = apiSecret
	return vo, nil
}

func generateCredentials() (apiKey, apiSecret string, err error) {
	keyBuf := make([]byte, 16)
	secretBuf := make([]byte, 32)
	if _, err = rand.Read(keyBuf); err != nil {
		return "", "", err
	}
	if _, err = rand.Read(secretBuf); err != nil {
		return "", "", err
	}
	return "dk_" + hex.EncodeToString(keyBuf), hex.EncodeToString(secretBuf), nil
}

// hashSecret 与网关鉴权一致：HMAC 密钥 = SHA256(secret) 的 hex 摘要。
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func (s *DeviceService) Heartbeat(ctx context.Context, deviceCode string) error {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return errors.New(errCodeDeviceNotFound, 404, "设备不存在")
	}

	now := time.Now()
	device.LastHeartbeat = now
	device.Status = model.DeviceStatusOnline

	return s.deviceRepo.Update(device)
}

func (s *DeviceService) List(ctx context.Context, req dto.DeviceListReq) ([]dto.DeviceVO, int64, error) {
	devices, total, err := s.deviceRepo.List(req.DeviceCode, req.Type, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.DeviceVO
	for _, d := range devices {
		vos = append(vos, *s.buildDeviceVO(&d))
	}

	return vos, total, nil
}

func (s *DeviceService) GetByCode(ctx context.Context, deviceCode string) (*dto.DeviceVO, error) {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return nil, errors.New(errCodeDeviceNotFound, 404, "设备不存在")
	}
	return s.buildDeviceVO(device), nil
}

// SendCommand 落库一条下行消息并经网关推送，等待设备 ack（超时置失败）。
func (s *DeviceService) SendCommand(ctx context.Context, deviceCode string, req dto.DeviceCommandReq) (*dto.DeviceCommandVO, error) {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return nil, errors.New(errCodeDeviceNotFound, 404, "设备不存在")
	}

	msg := &model.DevDeviceMessage{
		MessageID: gateway.NewMessageID(),
		DeviceID:  device.ID,
		Direction: model.MessageDirectionDown,
		MsgType:   req.MsgType,
		Payload:   req.Payload,
		Status:    model.MessageStatusPending,
		TenantID:  device.TenantID,
	}
	if err := s.msgRepo.Create(msg); err != nil {
		return nil, err
	}

	if s.hub == nil || !s.hub.IsOnline(deviceCode) {
		s.failMessage(msg)
		return nil, errors.New(errCodeDeviceOffline, 409, "设备离线，指令已置为失败")
	}

	_, ok := s.hub.SendAndWait(deviceCode, gateway.Envelope{
		MessageID: msg.MessageID,
		Type:      req.MsgType,
		Payload:   req.Payload,
		Timestamp: time.Now().Unix(),
	}, commandAckTimeout)
	if !ok {
		s.hub.FailMessage(msg.MessageID, msg.DeviceID)
		// 超时消息已落库置失败，message_id 可经消息流水查询。
		return nil, errors.New(errCodeCommandTimeout, 504, "指令超时未确认")
	}

	msg.Status = model.MessageStatusSuccess
	return &dto.DeviceCommandVO{MessageID: msg.MessageID, Status: msg.Status, Acked: true}, nil
}

func (s *DeviceService) failMessage(msg *model.DevDeviceMessage) {
	now := time.Now()
	msg.Status = model.MessageStatusFailed
	msg.AckedAt = &now
	_ = s.msgRepo.Update(msg)
}

// ListMessages 设备消息流水（direction 0=全部 1=上行 2=下发）。
func (s *DeviceService) ListMessages(ctx context.Context, deviceCode string, direction, page, pageSize int) ([]dto.DeviceMessageVO, int64, error) {
	device, err := s.deviceRepo.FindByCode(deviceCode)
	if err != nil {
		return nil, 0, errors.New(errCodeDeviceNotFound, 404, "设备不存在")
	}
	list, total, err := s.msgRepo.ListByDevice(device.ID, direction, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.DeviceMessageVO, 0, len(list))
	for _, m := range list {
		vo := dto.DeviceMessageVO{
			ID:        m.ID,
			MessageID: m.MessageID,
			Direction: m.Direction,
			MsgType:   m.MsgType,
			Payload:   m.Payload,
			Status:    m.Status,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		}
		if m.AckedAt != nil {
			vo.AckedAt = m.AckedAt.Format(time.RFC3339)
		}
		vos = append(vos, vo)
	}
	return vos, total, nil
}

func (s *DeviceService) buildDeviceVO(d *model.DevDevice) *dto.DeviceVO {
	typeName := ""
	switch d.Type {
	case model.DeviceTypeScanner:
		typeName = "扫码枪"
	case model.DeviceTypeRFID:
		typeName = "RFID读写器"
	case model.DeviceTypePDA:
		typeName = "PDA手持终端"
	case model.DeviceTypeWeigh:
		typeName = "电子秤"
	}

	statusName := ""
	switch d.Status {
	case model.DeviceStatusOffline:
		statusName = "离线"
	case model.DeviceStatusOnline:
		statusName = "在线"
	case model.DeviceStatusMaintain:
		statusName = "维护中"
	}

	return &dto.DeviceVO{
		ID:            d.ID,
		DeviceCode:    d.DeviceCode,
		Type:          d.Type,
		TypeName:      typeName,
		Brand:         d.Brand,
		Protocol:      d.Protocol,
		Status:        d.Status,
		StatusName:    statusName,
		LastHeartbeat: d.LastHeartbeat.Format(time.RFC3339),
		CreatedAt:     d.CreatedAt.Format(time.RFC3339),
	}
}

func (s *DeviceService) ParseScanEvent(data []byte) (*dto.DeviceScanEvent, error) {
	var event dto.DeviceScanEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, errors.New(errCodeInvalidScanEvent, 400, "无效的扫描事件数据")
	}
	return &event, nil
}
