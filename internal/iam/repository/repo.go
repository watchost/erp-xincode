// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"gorm.io/gorm"
	"erp-system/internal/iam/model"
)

type UserRepository interface {
	FindByUsername(username string) (*model.SysUser, error)
	FindByID(id int64) (*model.SysUser, error)
	Create(user *model.SysUser) error
	Update(user *model.SysUser) error
	List(username string, page, pageSize int) ([]model.SysUser, int64, error)
}

type RoleRepository interface {
	FindByCode(code string) (*model.SysRole, error)
	FindByID(id int64) (*model.SysRole, error)
	List() ([]model.SysRole, error)
}

type PermissionRepository interface {
	FindByCodes(codes []string) ([]model.SysPermission, error)
	List() ([]model.SysPermission, error)
}

type UserRoleRepository interface {
	GetUserRoles(userID int64) ([]model.SysRole, error)
	GetRolePermissions(roleID int64) ([]model.SysPermission, error)
}

type AuditLogRepository interface {
	Create(log *model.SysAuditLog) error
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) FindByUsername(username string) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) FindByID(id int64) (*model.SysUser, error) {
	var user model.SysUser
	if err := r.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) Create(user *model.SysUser) error {
	return r.db.Create(user).Error
}

func (r *userRepo) Update(user *model.SysUser) error {
	return r.db.Save(user).Error
}

func (r *userRepo) List(username string, page, pageSize int) ([]model.SysUser, int64, error) {
	var users []model.SysUser
	var total int64
	query := r.db.Model(&model.SysUser{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

type roleRepo struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepo{db: db}
}

func (r *roleRepo) FindByCode(code string) (*model.SysRole, error) {
	var role model.SysRole
	if err := r.db.Where("code = ?", code).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepo) FindByID(id int64) (*model.SysRole, error) {
	var role model.SysRole
	if err := r.db.Where("id = ?", id).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *roleRepo) List() ([]model.SysRole, error) {
	var roles []model.SysRole
	if err := r.db.Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

type permissionRepo struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepo{db: db}
}

func (r *permissionRepo) FindByCodes(codes []string) ([]model.SysPermission, error) {
	var perms []model.SysPermission
	if err := r.db.Where("code IN (?)", codes).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

func (r *permissionRepo) List() ([]model.SysPermission, error) {
	var perms []model.SysPermission
	if err := r.db.Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
}

type userRoleRepo struct {
	db *gorm.DB
}

func NewUserRoleRepository(db *gorm.DB) UserRoleRepository {
	return &userRoleRepo{db: db}
}

func (r *userRoleRepo) GetUserRoles(userID int64) ([]model.SysRole, error) {
	var roles []model.SysRole
	err := r.db.Joins("JOIN sys_user_role ON sys_user_role.role_id = sys_role.id").
		Where("sys_user_role.user_id = ?", userID).
		Find(&roles).Error
	return roles, err
}

func (r *userRoleRepo) GetRolePermissions(roleID int64) ([]model.SysPermission, error) {
	var perms []model.SysPermission
	err := r.db.Joins("JOIN sys_role_permission ON sys_role_permission.permission_id = sys_permission.id").
		Where("sys_role_permission.role_id = ?", roleID).
		Find(&perms).Error
	return perms, err
}

type auditLogRepo struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) AuditLogRepository {
	return &auditLogRepo{db: db}
}

func (r *auditLogRepo) Create(log *model.SysAuditLog) error {
	return r.db.Create(log).Error
}
