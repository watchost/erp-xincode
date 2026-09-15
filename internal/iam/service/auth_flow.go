// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/captcha"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
)

// Redis key 定义。
func mustChangeKey(userID int64) string  { return fmt.Sprintf("mustchg:%d", userID) }
func pwdChangedKey(userID int64) string  { return fmt.Sprintf("pwd:chg:%d", userID) }
func failUserKey(username string) string { return "loginfail:user:" + username }
func failIPKey(ip string) string         { return "loginfail:ip:" + ip }
func lockUserKey(username string) string { return "loginlock:user:" + username }
func lockIPKey(ip string) string         { return "loginlock:ip:" + ip }
func captchaKey(id string) string        { return "captcha:" + id }

// GetCaptcha 生成 4 位数字验证码，答案存 Redis 5 分钟，一次性使用。
func (s *IAMService) GetCaptcha(ctx context.Context) (dto.CaptchaVO, error) {
	code, img, err := captcha.Generate()
	if err != nil {
		return dto.CaptchaVO{}, errors.Wrap(err, 10300, 500, "生成验证码失败")
	}
	id := uuid.NewString()
	if err := s.kv.Set(ctx, captchaKey(id), code, int(captchaTTL.Seconds())); err != nil {
		return dto.CaptchaVO{}, errors.Wrap(err, 10300, 500, "存储验证码失败")
	}
	return dto.CaptchaVO{CaptchaID: id, Image: img}, nil
}

func (s *IAMService) Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error) {
	ip := auth.ClientIPFromContext(ctx)
	ua := auth.UserAgentFromContext(ctx)

	// 1. 账号或 IP 已锁定：直接拒绝（不透露命中维度）
	locked, err := s.kv.Exists(ctx, lockUserKey(req.Username))
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "登录状态校验失败")
	}
	ipLocked, err := s.kv.Exists(ctx, lockIPKey(ip))
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "登录状态校验失败")
	}
	if locked || ipLocked {
		return dto.LoginRes{}, errors.New(10007, 423, "账号已锁定，请 15 分钟后再试")
	}

	// 2. 任一维度失败达到验证码阈值，强制校验验证码
	userFails := s.counter(ctx, failUserKey(req.Username))
	ipFails := s.counter(ctx, failIPKey(ip))
	if userFails >= captchaThreshold || ipFails >= captchaThreshold {
		if !s.verifyCaptcha(ctx, req.CaptchaID, req.CaptchaCode) {
			s.recordFailure(ctx, req.Username, ip, ua, "captcha")
			return dto.LoginRes{}, errors.New(10008, 400, "验证码错误或已失效")
		}
	}

	// 3. 查用户（软删用户查不到，统一按凭证错误处理，避免账号枚举）
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		s.recordFailure(ctx, req.Username, ip, ua, "bad_credentials")
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}
	if user.Status != 1 {
		s.recordFailure(ctx, req.Username, ip, ua, "disabled")
		return dto.LoginRes{}, errors.New(10001, 401, "用户已禁用")
	}
	if !auth.ValidatePassword(user.PasswordHash, req.Password) {
		s.recordFailure(ctx, req.Username, ip, ua, "bad_credentials")
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}

	// 4. 成功：清失败计数、留痕、更新登录信息
	if err := s.kv.Del(ctx, failUserKey(req.Username), failIPKey(ip)); err != nil {
		logger.Log(ctx).Warn("clear login fail counters failed", "error", err)
	}
	s.recordSuccess(ctx, user, ip, ua)
	if err := s.userRepo.UpdateLastLogin(user.ID, time.Now(), ip); err != nil {
		logger.Log(ctx).Warn("update last login failed", "error", err)
	}
	// 以数据库标志为准同步强制改密 Redis 标记（供中间件无 DB 查询判定）
	if user.MustChangePwd {
		_ = s.kv.Set(ctx, mustChangeKey(user.ID), "1", int(s.accessExpire.Seconds()))
	} else {
		_ = s.kv.Del(ctx, mustChangeKey(user.ID))
	}

	logger.Log(ctx).Info("user login success", "username", user.Username)
	return s.issueTokens(ctx, user)
}

// verifyCaptcha 校验后立即删除答案，保证一次性。错误/过期均返回 false。
func (s *IAMService) verifyCaptcha(ctx context.Context, id, code string) bool {
	id = strings.TrimSpace(id)
	code = strings.TrimSpace(code)
	if id == "" || code == "" {
		return false
	}
	expected, err := s.kv.Get(ctx, captchaKey(id))
	if err != nil || expected == "" {
		return false
	}
	_ = s.kv.Del(ctx, captchaKey(id))
	return strings.EqualFold(expected, code)
}

func (s *IAMService) counter(ctx context.Context, key string) int64 {
	v, err := s.kv.Get(ctx, key)
	if err != nil || v == "" {
		return 0
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// recordFailure 累加用户/IP 两个维度的失败计数（固定窗口 15 分钟），
// 达锁定阈值写锁定键，并落库登录尝试 + 审计日志。
func (s *IAMService) recordFailure(ctx context.Context, username, ip, ua, reason string) {
	s.bumpCounter(ctx, failUserKey(username))
	ipCount := s.bumpCounter(ctx, failIPKey(ip))
	userCount := s.counter(ctx, failUserKey(username))
	if userCount >= lockThreshold {
		if err := s.kv.Set(ctx, lockUserKey(username), "1", int(lockTTL.Seconds())); err != nil {
			logger.Log(ctx).Warn("set user lock failed", "error", err)
		}
	}
	if ipCount >= lockThreshold {
		if err := s.kv.Set(ctx, lockIPKey(ip), "1", int(lockTTL.Seconds())); err != nil {
			logger.Log(ctx).Warn("set ip lock failed", "error", err)
		}
	}
	status := 401
	if reason == "captcha" {
		status = 400
	}
	s.writeAttempt(false, username, ip, reason)
	s.writeAudit(ctx, nil, 0, "login_failed", ip, ua, status, map[string]string{"username": username, "reason": reason})
}

func (s *IAMService) recordSuccess(ctx context.Context, user *model.SysUser, ip, ua string) {
	s.writeAttempt(true, user.Username, ip, "")
	s.writeAudit(ctx, &user.ID, user.TenantID, "login_success", ip, ua, 200, map[string]string{"username": user.Username})
}

// bumpCounter INCR 且仅在首次计数时设置窗口 TTL，返回累加后的值。
func (s *IAMService) bumpCounter(ctx context.Context, key string) int64 {
	n, err := s.kv.Incr(ctx, key)
	if err != nil {
		logger.Log(ctx).Warn("incr login counter failed", "key", key, "error", err)
		return 0
	}
	if n == 1 {
		if _, err := s.kv.Expire(ctx, key, failWindow); err != nil {
			logger.Log(ctx).Warn("expire login counter failed", "key", key, "error", err)
		}
	}
	return n
}

func (s *IAMService) writeAttempt(success bool, username, ip, reason string) {
	if s.attemptRepo == nil {
		return
	}
	a := &model.SysLoginAttempt{
		Username:   username,
		IP:         ip,
		Success:    success,
		FailReason: reason,
	}
	if err := s.attemptRepo.Create(a); err != nil {
		logger.Log(context.Background()).Warn("write login attempt failed", "error", err)
	}
}

func (s *IAMService) RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error) {
	refreshKey := "refresh:" + refreshToken
	userIDStr, err := s.kv.Get(ctx, refreshKey)
	if err != nil {
		return dto.LoginRes{}, errors.New(10002, 401, "refresh token无效或已过期")
	}
	var userID int64
	if _, err := fmt.Sscanf(userIDStr, "%d", &userID); err != nil || userID == 0 {
		return dto.LoginRes{}, errors.New(10002, 401, "refresh token无效")
	}
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return dto.LoginRes{}, errors.New(10001, 401, "用户不存在")
	}
	if user.Status != 1 {
		return dto.LoginRes{}, errors.New(10001, 401, "用户已禁用")
	}
	if err := s.kv.Del(ctx, refreshKey); err != nil {
		logger.Log(ctx).Warn("failed to delete old refresh token", "error", err)
	}
	return s.issueTokens(ctx, user)
}

// Logout 将 access token 的 jti 加入黑名单，TTL 与 token 有效期对齐。
func (s *IAMService) Logout(ctx context.Context, jti string, accessExpire time.Duration) error {
	if jti == "" {
		return errors.New(10005, 400, "缺少token标识")
	}
	key := "jti:blacklist:" + jti
	ttl := int(accessExpire.Seconds())
	if ttl <= 0 {
		ttl = int(s.accessExpire.Seconds())
	}
	if err := s.kv.Set(ctx, key, "1", ttl); err != nil {
		return errors.Wrap(err, 10300, 500, "登出失败")
	}
	return nil
}

// CheckClaims 供 JWTAuth 中间件逐请求调用：jti 黑名单 + 改密时间戳双重判定。
// Redis 异常时与旧版一致 fail-open（只记日志），避免缓存故障锁死全站。
func (s *IAMService) CheckClaims(ctx context.Context, c *auth.Claims) bool {
	if c == nil || s.kv == nil {
		return true
	}
	exists, err := s.kv.Exists(ctx, "jti:blacklist:"+c.JTI)
	if err != nil {
		logger.Log(ctx).Warn("check jti blacklist failed", "error", err)
	} else if exists {
		return false
	}
	changed, err := s.kv.Get(ctx, pwdChangedKey(c.UserID))
	if err != nil || changed == "" {
		return true
	}
	ts, err := strconv.ParseInt(changed, 10, 64)
	if err != nil {
		return true
	}
	if c.IssuedAt == nil {
		return true
	}
	return !c.IssuedAt.Time.Before(time.Unix(ts, 0))
}

// MustChangePassword 供 ForcePasswordChange 中间件调用。
func (s *IAMService) MustChangePassword(ctx context.Context, userID int64) bool {
	exists, err := s.kv.Exists(ctx, mustChangeKey(userID))
	if err != nil {
		logger.Log(ctx).Warn("check must-change-password failed", "error", err)
		return false
	}
	return exists
}

func (s *IAMService) ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordReq) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return errors.New(10100, 404, "用户不存在")
	}
	if !auth.ValidatePassword(user.PasswordHash, req.OldPassword) {
		return errors.New(10001, 401, "原密码错误")
	}
	if strings.EqualFold(req.OldPassword, req.NewPassword) {
		return errors.New(10005, 400, "新密码不能与旧密码相同")
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "加密密码失败")
	}
	now := time.Now()
	if err := s.userRepo.UpdateFields(nil, userID, map[string]interface{}{
		"password_hash":   hash,
		"must_change_pwd": false,
		"pwd_changed_at":  now,
		"updated_at":      now,
	}); err != nil {
		return err
	}
	if err := s.kv.Del(ctx, mustChangeKey(userID)); err != nil {
		logger.Log(ctx).Warn("clear must-change flag failed", "error", err)
	}
	return s.markPasswordChanged(ctx, userID)
}

// markPasswordChanged 记录改密时间戳：早于该时刻签发的 token 一律失效。
func (s *IAMService) markPasswordChanged(ctx context.Context, userID int64) error {
	if err := s.kv.Set(ctx, pwdChangedKey(userID), strconv.FormatInt(time.Now().Unix(), 10),
		int(s.accessExpire.Seconds())); err != nil {
		return errors.Wrap(err, 10300, 500, "记录密码变更失败")
	}
	return nil
}
