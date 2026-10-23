// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/iam/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/datascope"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/errors"
)

// KVStore 抽象 Redis（黑名单/登录计数/验证码/强制改密标记），便于测试造假。
// *redis.RedisClient 天然满足。
type KVStore interface {
	Set(ctx context.Context, key string, value interface{}, expiration int) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, key string) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) (bool, error)
}

// 登录防爆破参数。验证码阈值低于锁定阈值，避免验证码永远无法先于锁定生效。
const (
	captchaThreshold = 3
	lockThreshold    = 5
	failWindow       = 15 * time.Minute
	lockTTL          = 15 * time.Minute
	captchaTTL       = 5 * time.Minute
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginReq) (dto.LoginRes, error)
	RefreshToken(ctx context.Context, refreshToken string) (dto.LoginRes, error)
	Logout(ctx context.Context, userID int64, jti string, accessExpire time.Duration) error
	ChangePassword(ctx context.Context, userID int64, req dto.ChangePasswordReq) error
	GetUserByID(ctx context.Context, userID int64) (*model.SysUser, error)
	GetUserPermissions(ctx context.Context, userID int64) ([]string, error)
	GetCaptcha(ctx context.Context) (dto.CaptchaVO, error)
}

type UserService interface {
	ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error)
	GetUser(ctx context.Context, id int64) (*dto.UserVO, error)
	CreateUser(ctx context.Context, req dto.CreateUserReq) error
	UpdateUser(ctx context.Context, id int64, req dto.UpdateUserReq) error
	DeleteUser(ctx context.Context, id int64) error
	ResetPassword(ctx context.Context, id int64, req dto.ResetPasswordReq) (dto.ResetPasswordRes, error)
	AssignUserRoles(ctx context.Context, id int64, req dto.AssignUserRolesReq) error
}

type IAMService struct {
	db            *gorm.DB
	txManager     *db.TxManager
	userRepo      repository.UserRepository
	roleRepo      repository.RoleRepository
	permRepo      repository.PermissionRepository
	userRoleRepo  repository.UserRoleRepository
	deptRepo      repository.DepartmentRepository
	auditRepo     repository.AuditLogRepository
	attemptRepo   repository.LoginAttemptRepository
	tenantCheck   func(ctx context.Context, tenantID int64, write bool) error // 可选：P3 租户状态检查
	jwtSecret     string
	accessExpire  time.Duration
	refreshExpire time.Duration
	kv            KVStore
}

func NewIAMService(
	dbConn *gorm.DB,
	txManager *db.TxManager,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	permRepo repository.PermissionRepository,
	userRoleRepo repository.UserRoleRepository,
	deptRepo repository.DepartmentRepository,
	auditRepo repository.AuditLogRepository,
	attemptRepo repository.LoginAttemptRepository,
	jwtSecret string,
	accessExpire, refreshExpire time.Duration,
	kv KVStore,
) *IAMService {
	return &IAMService{
		db:            dbConn,
		txManager:     txManager,
		userRepo:      userRepo,
		roleRepo:      roleRepo,
		permRepo:      permRepo,
		userRoleRepo:  userRoleRepo,
		deptRepo:      deptRepo,
		auditRepo:     auditRepo,
		attemptRepo:   attemptRepo,
		jwtSecret:     jwtSecret,
		accessExpire:  accessExpire,
		refreshExpire: refreshExpire,
		kv:            kv,
	}
}

// principal 是登录用户解析后的权限画像：合并数据权限档与部门集合。
type principal struct {
	roles         []model.SysRole
	roleIDs       []int64
	roleCodes     []string
	permCodes     []string
	mergedScope   int
	customDeptIDs []int64
}

func (s *IAMService) loadPrincipal(user *model.SysUser) (principal, error) {
	var p principal
	roles, err := s.userRoleRepo.GetUserRoles(user.ID)
	if err != nil {
		return p, errors.Wrap(err, 10300, 500, "加载角色失败")
	}
	p.roles = roles
	scopes := make([]int, 0, len(roles))
	for _, r := range roles {
		p.roleCodes = append(p.roleCodes, r.Code)
		p.roleIDs = append(p.roleIDs, r.ID)
		scopes = append(scopes, r.DataScope)
	}
	p.mergedScope = datascope.Merge(scopes)

	permSet := make(map[string]struct{})
	for _, rid := range p.roleIDs {
		perms, err := s.userRoleRepo.GetRolePermissions(rid)
		if err != nil {
			return p, errors.Wrap(err, 10300, 500, "加载权限失败")
		}
		for _, perm := range perms {
			permSet[perm.Code] = struct{}{}
		}
	}
	p.permCodes = make([]string, 0, len(permSet))
	for code := range permSet {
		p.permCodes = append(p.permCodes, code)
	}

	if p.mergedScope == datascope.ScopeCustomDept {
		deptIDs, err := s.roleRepo.ListRoleDeptIDs(p.roleIDs)
		if err != nil {
			return p, errors.Wrap(err, 10300, 500, "加载数据权限部门失败")
		}
		p.customDeptIDs = deptIDs
	}
	return p, nil
}

func (s *IAMService) issueTokens(ctx context.Context, user *model.SysUser) (dto.LoginRes, error) {
	// P3 §4.2：租户停用或过期时禁止登录/刷新（登录与 refresh 都经过这里）
	if s.tenantCheck != nil {
		if err := s.tenantCheck(ctx, user.TenantID, true); err != nil {
			return dto.LoginRes{}, err
		}
	}
	p, err := s.loadPrincipal(user)
	if err != nil {
		return dto.LoginRes{}, err
	}

	jti := uuid.NewString()
	var deptID int64
	if user.DeptID != nil {
		deptID = *user.DeptID
	}
	claims := auth.Claims{
		UserID:    user.ID,
		Username:  user.Username,
		TenantID:  user.TenantID,
		DeptID:    deptID,
		DataScope: p.mergedScope,
		Roles:     p.roleCodes,
		Perms:     p.permCodes,
		JTI:       jti,
	}
	accessToken, err := auth.GenerateToken(s.jwtSecret, claims, s.accessExpire)
	if err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "生成token失败")
	}

	refreshToken := uuid.NewString()
	refreshKey := "refresh:" + refreshToken
	// 值携带签发时间戳（userID|issuedAtUnix）：改密/重置/登出写入的吊销时间戳
	// 据此判定旧 refresh token 是否仍然有效（P1.5 H1）。
	refreshVal := fmt.Sprintf("%d|%d", user.ID, time.Now().Unix())
	if err := s.kv.Set(ctx, refreshKey, refreshVal, int(s.refreshExpire.Seconds())); err != nil {
		return dto.LoginRes{}, errors.Wrap(err, 10300, 500, "存储refresh token失败")
	}

	// P1.5 H2：每次签发（登录与 refresh 换发都经过这里）以 DB 标志为准同步
	// 强制改密标记，真标记与 refresh 会话同寿命，杜绝「refresh 一次永久绕过」。
	if user.MustChangePwd {
		_ = s.kv.Set(ctx, mustChangeKey(user.ID), "1", int(s.refreshExpire.Seconds()))
	} else {
		_ = s.kv.Set(ctx, mustChangeKey(user.ID), "0", int(s.accessExpire.Seconds()))
	}

	return dto.LoginRes{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		ExpiresIn:     int64(s.accessExpire.Seconds()),
		MustChangePwd: user.MustChangePwd,
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

// DataScopeResolver 供业务模块（采购/生产等）解析当前请求的数据权限上下文。
// 由 IAMService 实现，业务包通过本地窄接口消费，避免反向依赖 iam 包。
type DataScopeResolver interface {
	ResolveDataScope(ctx context.Context, ownerCol, deptCol string) (datascope.Params, error)
}

// ResolveDataScope 依据 claims 中的数据档（多角色已在登录时合并取最宽）解析过滤参数：
// scope3 现查部门子孙集合；scope5 现查角色自定义部门集合；其余档位纯 claims 即可。
// claim 缺省（0，旧 token）按全部数据处理，保持向后兼容。
func (s *IAMService) ResolveDataScope(ctx context.Context, ownerCol, deptCol string) (datascope.Params, error) {
	uid := auth.UserIDFromContext(ctx)
	deptID := auth.DeptIDFromContext(ctx)
	scope := auth.DataScopeFromContext(ctx)
	if scope == 0 {
		scope = datascope.ScopeAll // 兼容无该 claim 的旧 token
	}
	params := datascope.Params{
		Scope:    scope,
		UserID:   uid,
		DeptID:   deptID,
		OwnerCol: ownerCol,
		DeptCol:  deptCol,
	}
	if scope == datascope.ScopeDeptAndChildren && deptID != 0 {
		ids, err := s.deptRepo.SubDeptIDs(deptID)
		if err != nil {
			return params, errors.Wrap(err, 10300, 500, "解析部门范围失败")
		}
		params.DeptIDs = ids
	}
	if scope == datascope.ScopeCustomDept {
		roles, err := s.userRoleRepo.GetUserRoles(uid)
		if err != nil {
			return params, errors.Wrap(err, 10300, 500, "加载角色失败")
		}
		ids := make([]int64, 0)
		for _, r := range roles {
			if r.DataScope == datascope.ScopeCustomDept {
				ids = append(ids, r.ID)
			}
		}
		deptIDs, err := s.roleRepo.ListRoleDeptIDs(ids)
		if err != nil {
			return params, errors.Wrap(err, 10300, 500, "加载自定义部门失败")
		}
		params.DeptIDs = deptIDs
	}
	return params, nil
}

// resolveUserScope 用户管理专用：属主列是 sys_user.id，部门列 dept_id。
func (s *IAMService) resolveUserScope(ctx context.Context) (datascope.Params, error) {
	return s.ResolveDataScope(ctx, "id", "dept_id")
}

// SetTenantChecker 装配租户状态检查（停用/过期租户禁止登录）。
func (s *IAMService) SetTenantChecker(fn func(ctx context.Context, tenantID int64, write bool) error) {
	s.tenantCheck = fn
}
