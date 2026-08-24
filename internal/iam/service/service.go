// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/iam/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
)

// TokenBlacklist 抽象 Redis 黑名单，便于测试。
// 实现由 redis.Client 满足（Set/Get/Del + Exists）。
type TokenBlacklist interface {
	Set(ctx context.Context, key string, value interface{}, expiration int) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
}

type AuthService interface {
	Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error)
	RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error)
	Logout(ctx context.Context, jti string, accessExpire time.Duration) error
	ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordReq) error
	GetUserByID(ctx context.Context, userID int64) (*model.SysUser, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
}

type UserService interface {
	ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error)
	GetUser(ctx context.Context, id int64) (*dto.UserVO, error)
	CreateUser(ctx context.Context, req dto.CreateUserReq) error
	UpdateUser(ctx context.Context, id int64, req dto.UpdateUserReq) error
}

type IAMService struct {
	db            *gorm.DB
	txManager     *db.TxManager
	userRepo      repository.UserRepository
	roleRepo      repository.RoleRepository
	permRepo      repository.PermissionRepository
	userRoleRepo  repository.UserRoleRepository
	auditLogRepo  repository.AuditLogRepository
	jwtSecret     string
	accessExpire  time.Duration
	refreshExpire time.Duration
	blacklist     TokenBlacklist
}

func NewIAMService(
	dbConn *gorm.DB,
	txManager *db.TxManager,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	permRepo repository.PermissionRepository,
	userRoleRepo repository.UserRoleRepository,
	auditLogRepo repository.AuditLogRepository,
	jwtSecret string,
	accessExpire, refreshExpire time.Duration,
	blacklist TokenBlacklist,
) *IAMService {
	return &IAMService{
		db:            dbConn,
		txManager:     txManager,
		userRepo:      userRepo,
		roleRepo:      roleRepo,
		permRepo:      permRepo,
		userRoleRepo:  userRoleRepo,
		auditLogRepo:  auditLogRepo,
		jwtSecret:     jwtSecret,
		accessExpire:  accessExpire,
		refreshExpire: refreshExpire,
		blacklist:     blacklist,
	}
}

func (s *IAMService) issueTokens(ctx context.Context, user *model.SysUser) (dto.LoginRes, error) {
	roles, err := s.userRoleRepo.GetUserRoles(user.ID)
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "加载角色失败")
	}
	roleCodes := make([]string, 0, len(roles))
	roleIDs := make([]int64, 0, len(roles))
	for _, r := range roles {
		roleCodes = append(roleCodes, r.Code)
		roleIDs = append(roleIDs, r.ID)
	}
	permSet := make(map[string]struct{})
	for _, rid := range roleIDs {
		perms, err := s.userRoleRepo.GetRolePermissions(rid)
		if err != nil {
			return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "加载权限失败")
		}
		for _, p := range perms {
			permSet[p.Code] = struct{}{}
		}
	}
	permCodes := make([]string, 0, len(permSet))
	for code := range permSet {
		permCodes = append(permCodes, code)
	}

	jti := uuid.NewString()
	claims := auth.Claims{
		UserID:   user.ID,
		Username: user.Username,
		TenantID: user.TenantID,
		Roles:    roleCodes,
		Perms:    permCodes,
		JTI:      jti,
	}
	accessToken, err := auth.GenerateToken(s.jwtSecret, claims, s.accessExpire)
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "生成token失败")
	}

	refreshToken := uuid.NewString()
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	if err := s.blacklist.Set(ctx, refreshKey, user.ID, int(s.refreshExpire.Seconds())); err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "存储refresh token失败")
	}

	return dto.LoginRes{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessExpire.Seconds()),
	}, nil
}

func (s *IAMService) Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		// 不暴露用户是否存在，统一返回用户名或密码错误
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}

	if user.Status != 1 {
		return dto.LoginRes{}, errors.New(10001, 401, "用户已禁用")
	}

	if !auth.ValidatePassword(user.PasswordHash, req.Password) {
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}

	logger.Log(ctx).Info("user login success", "username", user.Username)
	return s.issueTokens(ctx, user)
}

func (s *IAMService) RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error) {
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	userIDStr, err := s.blacklist.Get(ctx, refreshKey)
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

	// 刷新即轮换：删除旧 refresh token，签发新的对
	if err := s.blacklist.Del(ctx, refreshKey); err != nil {
		logger.Log(ctx).Warn("failed to delete old refresh token", "error", err)
	}

	return s.issueTokens(ctx, user)
}

// Logout 将 access token 的 jti 加入黑名单，TTL 与 token 剩余有效期对齐。
func (s *IAMService) Logout(ctx context.Context, jti string, accessExpire time.Duration) error {
	if jti == "" {
		return errors.New(10005, 400, "缺少token标识")
	}
	key := fmt.Sprintf("jti:blacklist:%s", jti)
	ttl := int(accessExpire.Seconds())
	if ttl <= 0 {
		ttl = int(s.accessExpire.Seconds())
	}
	if err := s.blacklist.Set(ctx, key, "1", ttl); err != nil {
		return errors.Wrap(err, 10300, 500, "登出失败")
	}
	return nil
}

// IsBlacklisted 供中间件在每次请求时调用。
func (s *IAMService) IsBlacklisted(ctx context.Context, jti string) bool {
	if jti == "" || s.blacklist == nil {
		return false
	}
	exists, err := s.blacklist.Exists(ctx, fmt.Sprintf("jti:blacklist:%s", jti))
	if err != nil {
		logger.Log(ctx).Warn("check jti blacklist failed", "error", err)
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
	return s.userRepo.UpdateFields(nil, userID, map[string]interface{}{
		"password_hash": hash,
		"updated_at":    time.Now(),
	})
}

func (s *IAMService) GetUserByID(ctx context.Context, userID int64) (*model.SysUser, error) {
	return s.userRepo.FindByID(userID)
}

func (s *IAMService) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	roles, err := s.userRoleRepo.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}
	permSet := make(map[string]struct{})
	for _, role := range roles {
		perms, err := s.userRoleRepo.GetRolePermissions(role.ID)
		if err != nil {
			return nil, err
		}
		for _, perm := range perms {
			permSet[perm.Code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(permSet))
	for code := range permSet {
		codes = append(codes, code)
	}
	return codes, nil
}

func (s *IAMService) ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error) {
	users, total, err := s.userRepo.List(req.Username, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.UserVO, 0, len(users))
	for _, u := range users {
		vos = append(vos, toUserVO(&u, nil))
	}
	return vos, total, nil
}

func (s *IAMService) GetUser(ctx context.Context, id int64) (*dto.UserVO, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(10100, 404, "用户不存在")
	}
	roles, err := s.userRoleRepo.GetUserRoles(id)
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(roles))
	for _, r := range roles {
		codes = append(codes, r.Code)
	}
	vo := toUserVO(user, codes)
	return &vo, nil
}

func (s *IAMService) CreateUser(ctx context.Context, req dto.CreateUserReq) error {
	existing, _ := s.userRepo.FindByUsername(req.Username)
	if existing != nil {
		return errors.New(10005, 400, "用户名已存在")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "加密密码失败")
	}

	status := req.Status
	if status == 0 {
		status = 1
	}
	user := &model.SysUser{
		Username:     req.Username,
		PasswordHash: hash,
		RealName:     req.RealName,
		Phone:        req.Phone,
		Status:       status,
		TenantID:     req.TenantID,
	}

	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.userRepo.Create(tx, user); err != nil {
			return err
		}
		if len(req.RoleIDs) > 0 {
			if err := s.userRoleRepo.ReplaceUserRoles(tx, user.ID, req.RoleIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *IAMService) UpdateUser(ctx context.Context, id int64, req dto.UpdateUserReq) error {
	if _, err := s.userRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "用户不存在")
	}
	fields := make(map[string]interface{})
	if req.RealName != nil {
		fields["real_name"] = *req.RealName
	}
	if req.Phone != nil {
		fields["phone"] = *req.Phone
	}
	if req.Status != nil {
		fields["status"] = *req.Status
	}
	if len(fields) > 0 {
		fields["updated_at"] = time.Now()
	}
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.userRepo.UpdateFields(tx, id, fields); err != nil {
			return err
		}
		// RoleIDs 为 nil 表示不修改；非 nil（即使为空切片）则全量替换
		if req.RoleIDs != nil {
			if err := s.userRoleRepo.ReplaceUserRoles(tx, id, req.RoleIDs); err != nil {
				return err
			}
		}
		return nil
	})
}

func toUserVO(u *model.SysUser, roles []string) dto.UserVO {
	return dto.UserVO{
		ID:        u.ID,
		Username:  u.Username,
		RealName:  u.RealName,
		Phone:     u.Phone,
		Status:    u.Status,
		TenantID:  u.TenantID,
		Roles:     roles,
		CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}
