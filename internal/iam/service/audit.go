// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"encoding/json"
	"time"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/iam/repository"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
)

// writeAudit 落一条审计日志。审计是旁路能力：任何失败只记日志，不阻断主流程，
// 因此登录失败（无事务上下文、请求即将返回）也可安全调用。
// writeAudit 落一条认证模块（auth）审计日志。
func (s *IAMService) writeAudit(ctx context.Context, userID *int64, tenantID int64,
	action, ip, ua string, status int, param map[string]string,
) {
	if s.auditRepo == nil {
		return
	}
	var raw model.JSON
	if len(param) > 0 {
		b, err := json.Marshal(param)
		if err != nil {
			logger.Log(ctx).Warn("marshal audit param failed", "error", err)
		} else {
			raw = model.JSON(b)
		}
	}
	entry := &model.SysAuditLog{
		UserID:    userID,
		TenantID:  tenantID,
		Module:    "auth",
		Action:    action,
		IP:        ip,
		UserAgent: ua,
		Status:    status,
		ReqParam:  raw,
	}
	if err := s.auditRepo.Create(entry); err != nil {
		logger.Log(ctx).Warn("write audit log failed", "action", action, "error", err)
	}
}

// parseAuditTime 接受 RFC3339 与 "2006-01-02 15:04:05" 两种格式。
func parseAuditTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *IAMService) ListAuditLogs(ctx context.Context, req dto.AuditLogListReq) ([]dto.AuditLogVO, int64, error) {
	start, err := parseAuditTime(req.StartTime)
	if err != nil {
		return nil, 0, errors.New(10005, 400, "开始时间格式错误")
	}
	end, err := parseAuditTime(req.EndTime)
	if err != nil {
		return nil, 0, errors.New(10005, 400, "结束时间格式错误")
	}
	if start != nil && end != nil && end.Before(*start) {
		return nil, 0, errors.New(10005, 400, "结束时间不能早于开始时间")
	}
	logs, total, err := s.auditRepo.ListPage(repository.AuditQuery{
		UserID:    req.UserID,
		Module:    req.Module,
		Action:    req.Action,
		StartTime: start,
		EndTime:   end,
		Page:      req.Page,
		PageSize:  req.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.AuditLogVO, 0, len(logs))
	for _, l := range logs {
		vos = append(vos, dto.AuditLogVO{
			ID:         l.ID,
			UserID:     l.UserID,
			Action:     l.Action,
			Module:     l.Module,
			IP:         l.IP,
			ReqParam:   string(l.ReqParam),
			TraceID:    l.TraceID,
			UserAgent:  l.UserAgent,
			Status:     l.Status,
			DurationMs: l.DurationMs,
			CreatedAt:  l.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return vos, total, nil
}
