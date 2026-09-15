// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"crypto/rand"
	"math/big"
	"strings"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/datascope"
	"erp-system/internal/pkg/errors"
)

const seedAdminID = 1

// resetAlphabet 去掉易混字符 0/O/1/I/L，便于人工抄写。
const resetAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz"

func (s *IAMService) ListUsers(ctx context.Context, req dto.UserListReq) ([]dto.UserVO, int64, error) {
	scopeParams, err := s.resolveUserScope(ctx)
	if err != nil {
		return nil, 0, err
	}
	users, total, err := s.userRepo.ListPage(req.Username, req.Page, req.PageSize, datascope.Apply(scopeParams))
	if err != nil {
		return nil, 0, err
	}
	vos := make([]dto.UserVO, 0, len(users))
	for i := range users {
		vo := toUserVO(&users[i], nil)
		s.fillDeptName(&vo, users[i].DeptID)
		vos = append(vos, vo)
	}
	return vos, total, nil
}

func (s *IAMService) GetUser(ctx context.Context, id int64) (*dto.UserVO, error) {
	scopeParams, err := s.resolveUserScope(ctx)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(10100, 404, "用户不存在")
	}
	recordOwner := user.ID
	var recordDept int64
	if user.DeptID != nil {
		recordDept = *user.DeptID
	}
	if !datascope.CanAccess(scopeParams.Scope, scopeParams.UserID, scopeParams.DeptID,
		scopeParams.DeptIDs, recordOwner, recordDept) {
		return nil, errors.New(10004, 403, "无权查看该用户")
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
	s.fillDeptName(&vo, user.DeptID)
	return &vo, nil
}

func (s *IAMService) fillDeptName(vo *dto.UserVO, deptID *int64) {
	if deptID == nil {
		return
	}
	if d, err := s.deptRepo.FindByID(*deptID); err == nil {
		vo.DeptName = d.Name
	}
}

func (s *IAMService) CreateUser(ctx context.Context, req dto.CreateUserReq) error {
	existing, _ := s.userRepo.FindByUsername(req.Username)
	if existing != nil {
		return errors.New(10005, 400, "用户名已存在")
	}
	if req.DeptID != nil {
		if _, err := s.deptRepo.FindByID(*req.DeptID); err != nil {
			return errors.New(10005, 400, "部门不存在")
		}
	}
	if len(req.RoleIDs) > 0 {
		roles, err := s.roleRepo.FindByIDs(req.RoleIDs)
		if err != nil {
			return errors.Wrap(err, 10300, 500, "校验角色失败")
		}
		if len(roles) != len(uniqueInt64(req.RoleIDs)) {
			return errors.New(10005, 400, "角色不存在")
		}
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "加密密码失败")
	}
	status := req.Status
	if status == 0 {
		status = 1
	}
	now := time.Now()
	user := &model.SysUser{
		Username:      req.Username,
		PasswordHash:  hash,
		RealName:      req.RealName,
		Phone:         req.Phone,
		Email:         req.Email,
		DeptID:        req.DeptID,
		MustChangePwd: req.MustChangePwd,
		Status:        status,
		TenantID:      req.TenantID,
	}
	if !req.MustChangePwd {
		user.PwdChangedAt = &now
	}

	err = s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.userRepo.Create(tx, user); err != nil {
			return err
		}
		if len(req.RoleIDs) > 0 {
			return s.userRoleRepo.ReplaceUserRoles(tx, user.ID, uniqueInt64(req.RoleIDs))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if req.MustChangePwd {
		if err := s.kv.Set(ctx, mustChangeKey(user.ID), "1", int(s.accessExpire.Seconds())); err != nil {
			return errors.Wrap(err, 10300, 500, "标记强制改密失败")
		}
	}
	return nil
}

func (s *IAMService) UpdateUser(ctx context.Context, id int64, req dto.UpdateUserReq) error {
	if _, err := s.userRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "用户不存在")
	}
	if req.DeptID != nil && *req.DeptID != 0 {
		if _, err := s.deptRepo.FindByID(*req.DeptID); err != nil {
			return errors.New(10005, 400, "部门不存在")
		}
	}
	fields := make(map[string]interface{})
	if req.RealName != nil {
		fields["real_name"] = *req.RealName
	}
	if req.Phone != nil {
		fields["phone"] = *req.Phone
	}
	if req.Email != nil {
		fields["email"] = *req.Email
	}
	if req.DeptID != nil {
		if *req.DeptID == 0 {
			fields["dept_id"] = nil
		} else {
			fields["dept_id"] = *req.DeptID
		}
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
		// RoleIDs 为 nil 不修改；非 nil（含空切片）全量替换
		if req.RoleIDs != nil {
			roles, err := s.roleRepo.FindByIDs(req.RoleIDs)
			if err != nil {
				return errors.Wrap(err, 10300, 500, "校验角色失败")
			}
			if len(roles) != len(uniqueInt64(req.RoleIDs)) {
				return errors.New(10005, 400, "角色不存在")
			}
			if err := s.userRoleRepo.ReplaceUserRoles(tx, id, uniqueInt64(req.RoleIDs)); err != nil {
				return err
			}
		}
		return nil
	})
}

// AssignUserRoles 独立的角色分配端点（PUT /iam/users/:id/roles）。
func (s *IAMService) AssignUserRoles(ctx context.Context, id int64, req dto.AssignUserRolesReq) error {
	if _, err := s.userRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "用户不存在")
	}
	roleIDs := uniqueInt64(req.RoleIDs)
	roles, err := s.roleRepo.FindByIDs(roleIDs)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "校验角色失败")
	}
	if len(roles) != len(roleIDs) {
		return errors.New(10005, 400, "角色不存在")
	}
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return s.userRoleRepo.ReplaceUserRoles(tx, id, roleIDs)
	})
}

func (s *IAMService) DeleteUser(ctx context.Context, id int64) error {
	caller := auth.UserIDFromContext(ctx)
	if id == caller {
		return errors.New(10006, 409, "不能删除当前登录用户")
	}
	if id == seedAdminID {
		return errors.New(10006, 409, "内置管理员账号不可删除")
	}
	if _, err := s.userRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "用户不存在")
	}
	err := s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.userRoleRepo.ReplaceUserRoles(tx, id, nil); err != nil {
			return err
		}
		return s.userRepo.SoftDelete(tx, id)
	})
	if err != nil {
		return err
	}
	// 清理强制改密标记；refresh token 无法枚举，留待自然过期。
	_ = s.kv.Del(ctx, mustChangeKey(id))
	return nil
}

func (s *IAMService) ResetPassword(ctx context.Context, id int64, req dto.ResetPasswordReq) (dto.ResetPasswordRes, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return dto.ResetPasswordRes{}, errors.New(10100, 404, "用户不存在")
	}
	newPassword := strings.TrimSpace(req.Password)
	generated := false
	if newPassword == "" {
		newPassword, err = randomPassword(12)
		if err != nil {
			return dto.ResetPasswordRes{}, errors.Wrap(err, 10300, 500, "生成随机密码失败")
		}
		generated = true
	}
	if auth.ValidatePassword(user.PasswordHash, newPassword) {
		return dto.ResetPasswordRes{}, errors.New(10005, 400, "新密码不能与旧密码相同")
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return dto.ResetPasswordRes{}, errors.Wrap(err, 10300, 500, "加密密码失败")
	}
	if err := s.userRepo.UpdateFields(nil, id, map[string]interface{}{
		"password_hash":   hash,
		"must_change_pwd": true,
		"pwd_changed_at":  nil,
		"updated_at":      time.Now(),
	}); err != nil {
		return dto.ResetPasswordRes{}, err
	}
	if err := s.kv.Set(ctx, mustChangeKey(id), "1", int(s.accessExpire.Seconds())); err != nil {
		return dto.ResetPasswordRes{}, errors.Wrap(err, 10300, 500, "标记强制改密失败")
	}
	// 重置后旧 access token 立即失效
	if err := s.markPasswordChanged(ctx, id); err != nil {
		return dto.ResetPasswordRes{}, err
	}
	res := dto.ResetPasswordRes{}
	if generated {
		res.Password = newPassword
	}
	return res, nil
}

func randomPassword(n int) (string, error) {
	b := make([]byte, n)
	for i := range b {
		idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(resetAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = resetAlphabet[idx.Int64()]
	}
	return string(b), nil
}

func uniqueInt64(in []int64) []int64 {
	seen := make(map[int64]struct{}, len(in))
	out := make([]int64, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func toUserVO(u *model.SysUser, roles []string) dto.UserVO {
	vo := dto.UserVO{
		ID:            u.ID,
		Username:      u.Username,
		RealName:      u.RealName,
		Phone:         u.Phone,
		Email:         u.Email,
		Status:        u.Status,
		TenantID:      u.TenantID,
		DeptID:        u.DeptID,
		MustChangePwd: u.MustChangePwd,
		Roles:         roles,
		CreatedAt:     u.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if u.LastLoginAt != nil {
		vo.LastLoginAt = u.LastLoginAt.Format("2006-01-02 15:04:05")
	}
	return vo
}
