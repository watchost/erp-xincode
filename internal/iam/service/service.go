// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/iam/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/logger"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error)
	RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error)
	GetUserByID(ctx context.Context, userID int64) (*model.SysUser, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
}

type UserService interface {
	ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error)
	GetUser(ctx context.Context, id int64) (*dto.UserVO, error)
	CreateUser(ctx context.Context, user *model.SysUser) error
	UpdateUser(ctx context.Context, user *model.SysUser) error
}

type IAMService struct {
	userRepo       repository.UserRepository
	roleRepo       repository.RoleRepository
	permRepo       repository.PermissionRepository
	userRoleRepo   repository.UserRoleRepository
	auditLogRepo   repository.AuditLogRepository
	jwtSecret      string
	accessExpire   time.Duration
	refreshExpire  time.Duration
	redisClient    interface {
		Set(ctx context.Context, key string, value interface{}, expiration int) error
		Get(ctx context.Context, key string) (string, error)
		Del(ctx context.Context, keys ...string) error
	}
}

func NewIAMService(
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	permRepo repository.PermissionRepository,
	userRoleRepo repository.UserRoleRepository,
	auditLogRepo repository.AuditLogRepository,
	jwtSecret string,
	accessExpire, refreshExpire time.Duration,
	redisClient interface {
		Set(ctx context.Context, key string, value interface{}, expiration int) error
		Get(ctx context.Context, key string) (string, error)
		Del(ctx context.Context, keys ...string) error
	},
) *IAMService {
	return &IAMService{
		userRepo:      userRepo,
		roleRepo:      roleRepo,
		permRepo:      permRepo,
		userRoleRepo:  userRoleRepo,
		auditLogRepo:  auditLogRepo,
		jwtSecret:     jwtSecret,
		accessExpire:  accessExpire,
		refreshExpire: refreshExpire,
		redisClient:   redisClient,
	}
}

func (s *IAMService) Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error) {
	user, err := s.userRepo.FindByUsername(req.Username)
	if err != nil {
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}

	if user.Status != 1 {
		return dto.LoginRes{}, errors.New(10001, 401, "用户已禁用")
	}

	if !auth.ValidatePassword(user.PasswordHash, req.Password) {
		return dto.LoginRes{}, errors.New(10001, 401, "用户名或密码错误")
	}

	accessToken, err := auth.GenerateToken(s.jwtSecret, user.ID, user.Username, s.accessExpire)
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "生成token失败")
	}

	refreshToken := uuid.NewString()
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	err = s.redisClient.Set(ctx, refreshKey, user.ID, int(s.refreshExpire.Seconds()))
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "存储refresh token失败")
	}

	logger.Log(ctx).Info("user login success", "username", user.Username)

	return dto.LoginRes{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessExpire.Seconds()),
	}, nil
}

func (s *IAMService) RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error) {
	refreshKey := fmt.Sprintf("refresh:%s", refreshToken)
	userIDStr, err := s.redisClient.Get(ctx, refreshKey)
	if err != nil {
		return dto.LoginRes{}, errors.New(10002, 401, "refresh token无效")
	}

	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return dto.LoginRes{}, errors.New(10001, 401, "用户不存在")
	}

	err = s.redisClient.Del(ctx, refreshKey)
	if err != nil {
		logger.Log(ctx).Warn("failed to delete old refresh token", "error", err)
	}

	accessToken, err := auth.GenerateToken(s.jwtSecret, user.ID, user.Username, s.accessExpire)
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "生成token失败")
	}

	newRefreshToken := uuid.NewString()
	newRefreshKey := fmt.Sprintf("refresh:%s", newRefreshToken)
	err = s.redisClient.Set(ctx, newRefreshKey, userID, int(s.refreshExpire.Seconds()))
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "存储refresh token失败")
	}

	return dto.LoginRes{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(s.accessExpire.Seconds()),
	}, nil
}

func (s *IAMService) GetUserByID(ctx context.Context, userID int64) (*model.SysUser, error) {
	return s.userRepo.FindByID(userID)
}

func (s *IAMService) GetUserPermissions(ctx context.Context, userID int64) ([]string, error) {
	roles, err := s.userRoleRepo.GetUserRoles(userID)
	if err != nil {
		return nil, err
	}

	var permCodes []string
	for _, role := range roles {
		perms, err := s.userRoleRepo.GetRolePermissions(role.ID)
		if err != nil {
			return nil, err
		}
		for _, perm := range perms {
			permCodes = append(permCodes, perm.Code)
		}
	}

	return permCodes, nil
}

func (s *IAMService) ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error) {
	users, total, err := s.userRepo.List(req.Username, req.Page, req.PageSize)
	if err != nil {
		return nil, 0, err
	}

	var vos []dto.UserVO
	for _, u := range users {
		vos = append(vos, dto.UserVO{
			ID:        u.ID,
			Username:  u.Username,
			RealName:  u.RealName,
			Phone:     u.Phone,
			Status:    u.Status,
			CreatedAt: u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return vos, total, nil
}

func (s *IAMService) GetUser(ctx context.Context, id int64) (*dto.UserVO, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(10100, 404, "用户不存在")
	}
	return &dto.UserVO{
		ID:        user.ID,
		Username:  user.Username,
		RealName:  user.RealName,
		Phone:     user.Phone,
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *IAMService) CreateUser(ctx context.Context, user *model.SysUser) error {
	existing, _ := s.userRepo.FindByUsername(user.Username)
	if existing != nil {
		return errors.New(10005, 400, "用户名已存在")
	}

	hash, err := auth.HashPassword(user.PasswordHash)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "加密密码失败")
	}
	user.PasswordHash = hash

	return s.userRepo.Create(user)
}

func (s *IAMService) UpdateUser(ctx context.Context, user *model.SysUser) error {
	return s.userRepo.Update(user)
}
