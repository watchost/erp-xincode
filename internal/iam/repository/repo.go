// Copyright 2026 zhouhouping. All Rights Reserved.

package repository

import (
	"time"

	"erp-system/internal/iam/model"
	"gorm.io/gorm"
)

// UserRepository 负责 sys_user 的读写。
// 写方法接收 *gorm.DB 以便 service 在同一事务里组合多张表。
type UserRepository interface {
	FindByUsername(username string) (*model.SysUser, error)
	FindByID(id int64) (*model.SysUser, error)
	Create(tx *gorm.DB, user *model.SysUser) error
	UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error
	// SoftDelete 置 deleted_at；唯一索引是部分索引，软删后同名可重建。
	SoftDelete(tx *gorm.DB, id int64) error
	UpdateLastLogin(id int64, at time.Time, ip string) error
	List(username string, page, pageSize int) ([]model.SysUser, int64, error)
	// ListPage 在 List 基础上叠加数据权限 scope（datascope.Apply 生成）。
	ListPage(username string, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.SysUser, int64, error)
}

type RoleRepository interface {
	FindByCode(code string) (*model.SysRole, error)
	FindByID(id int64) (*model.SysRole, error)
	FindByIDs(ids []int64) ([]model.SysRole, error)
	List() ([]model.SysRole, error)
	Create(tx *gorm.DB, role *model.SysRole) error
	UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error
	Delete(tx *gorm.DB, id int64) error
	CountUsersByRole(roleID int64) (int64, error)
	// ReplaceRolePermissions 事务内全量替换角色权限绑定。
	ReplaceRolePermissions(tx *gorm.DB, roleID int64, permissionIDs []int64) error
	ListPermissionIDsByRole(roleID int64) ([]int64, error)
	// ReplaceRoleDepts / ListRoleDeptIDs 用于 data_scope=5 的自定义部门集。
	ReplaceRoleDepts(tx *gorm.DB, roleID int64, deptIDs []int64) error
	ListRoleDeptIDs(roleIDs []int64) ([]int64, error)
}

type PermissionRepository interface {
	FindByCodes(codes []string) ([]model.SysPermission, error)
	FindByIDs(ids []int64) ([]model.SysPermission, error)
	List() ([]model.SysPermission, error)
}

type UserRoleRepository interface {
	GetUserRoles(userID int64) ([]model.SysRole, error)
	GetRolePermissions(roleID int64) ([]model.SysPermission, error)
	// ReplaceUserRoles 在同一事务里删除旧绑定再写入新绑定。
	ReplaceUserRoles(tx *gorm.DB, userID int64, roleIDs []int64) error
}

type DepartmentRepository interface {
	Create(tx *gorm.DB, dept *model.SysDepartment) error
	FindByCode(code string) (*model.SysDepartment, error)
	FindByID(id int64) (*model.SysDepartment, error)
	UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error
	Delete(tx *gorm.DB, id int64) error
	List() ([]model.SysDepartment, error)
	CountChildren(parentID int64) (int64, error)
	CountUsers(deptID int64) (int64, error)
	// SubDeptIDs 返回本部门及全部子孙部门 ID（物化路径 LIKE 前缀匹配）。
	SubDeptIDs(deptID int64) ([]int64, error)
}

type AuditLogRepository interface {
	Create(log *model.SysAuditLog) error
	ListPage(req AuditQuery) ([]model.SysAuditLog, int64, error)
}

// AuditQuery 是审计日志分页查询条件（时间为闭区间）。
type AuditQuery struct {
	UserID    *int64
	Module    string
	Action    string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

type LoginAttemptRepository interface {
	Create(a *model.SysLoginAttempt) error
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

func (r *userRepo) Create(tx *gorm.DB, user *model.SysUser) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Create(user).Error
}

// UpdateFields 使用 map 白名单更新，避免 Save 整行覆盖造成的 Mass Assignment。
func (r *userRepo) UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	if len(fields) == 0 {
		return nil
	}
	return tx.Model(&model.SysUser{}).Where("id = ?", id).Updates(fields).Error
}

func (r *userRepo) List(username string, page, pageSize int) ([]model.SysUser, int64, error) {
	return r.ListPage(username, page, pageSize)
}

func (r *userRepo) SoftDelete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Model(&model.SysUser{}).Where("id = ?", id).
		UpdateColumn("deleted_at", time.Now()).Error
}

func (r *userRepo) UpdateLastLogin(id int64, at time.Time, ip string) error {
	return r.db.Model(&model.SysUser{}).Where("id = ?", id).
		Updates(map[string]interface{}{"last_login_at": at, "last_login_ip": ip}).Error
}

func (r *userRepo) ListPage(username string, page, pageSize int, scopes ...func(*gorm.DB) *gorm.DB) ([]model.SysUser, int64, error) {
	var users []model.SysUser
	var total int64
	query := r.db.Model(&model.SysUser{}).Scopes(scopes...)
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

func (r *roleRepo) FindByIDs(ids []int64) ([]model.SysRole, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var roles []model.SysRole
	if err := r.db.Where("id IN ?", ids).Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepo) List() ([]model.SysRole, error) {
	var roles []model.SysRole
	if err := r.db.Order("id ASC").Find(&roles).Error; err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *roleRepo) Create(tx *gorm.DB, role *model.SysRole) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Create(role).Error
}

func (r *roleRepo) UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	if len(fields) == 0 {
		return nil
	}
	return tx.Model(&model.SysRole{}).Where("id = ?", id).Updates(fields).Error
}

func (r *roleRepo) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Where("id = ?", id).Delete(&model.SysRole{}).Error
}

func (r *roleRepo) CountUsersByRole(roleID int64) (int64, error) {
	var n int64
	err := r.db.Model(&model.SysUserRole{}).Where("role_id = ?", roleID).Count(&n).Error
	return n, err
}

func (r *roleRepo) ReplaceRolePermissions(tx *gorm.DB, roleID int64, permissionIDs []int64) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.Where("role_id = ?", roleID).Delete(&model.SysRolePermission{}).Error; err != nil {
		return err
	}
	if len(permissionIDs) == 0 {
		return nil
	}
	rows := make([]model.SysRolePermission, 0, len(permissionIDs))
	for _, pid := range permissionIDs {
		rows = append(rows, model.SysRolePermission{RoleID: roleID, PermissionID: pid})
	}
	return tx.Create(&rows).Error
}

func (r *roleRepo) ListPermissionIDsByRole(roleID int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.Model(&model.SysRolePermission{}).
		Where("role_id = ?", roleID).Pluck("permission_id", &ids).Error
	return ids, err
}

func (r *roleRepo) ReplaceRoleDepts(tx *gorm.DB, roleID int64, deptIDs []int64) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.Where("role_id = ?", roleID).Delete(&model.SysRoleDept{}).Error; err != nil {
		return err
	}
	if len(deptIDs) == 0 {
		return nil
	}
	rows := make([]model.SysRoleDept, 0, len(deptIDs))
	for _, did := range deptIDs {
		rows = append(rows, model.SysRoleDept{RoleID: roleID, DeptID: did})
	}
	return tx.Create(&rows).Error
}

func (r *roleRepo) ListRoleDeptIDs(roleIDs []int64) ([]int64, error) {
	ids := make([]int64, 0)
	if len(roleIDs) == 0 {
		return ids, nil
	}
	err := r.db.Model(&model.SysRoleDept{}).
		Where("role_id IN ?", roleIDs).Distinct().Pluck("dept_id", &ids).Error
	return ids, err
}

type permissionRepo struct {
	db *gorm.DB
}

func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepo{db: db}
}

func (r *permissionRepo) FindByIDs(ids []int64) ([]model.SysPermission, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var perms []model.SysPermission
	if err := r.db.Where("id IN ?", ids).Find(&perms).Error; err != nil {
		return nil, err
	}
	return perms, nil
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

func (r *userRoleRepo) ReplaceUserRoles(tx *gorm.DB, userID int64, roleIDs []int64) error {
	if tx == nil {
		tx = r.db
	}
	if err := tx.Where("user_id = ?", userID).Delete(&model.SysUserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	bindings := make([]model.SysUserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		bindings = append(bindings, model.SysUserRole{UserID: userID, RoleID: rid})
	}
	return tx.Create(&bindings).Error
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

func (r *auditLogRepo) ListPage(q AuditQuery) ([]model.SysAuditLog, int64, error) {
	var logs []model.SysAuditLog
	var total int64
	query := r.db.Model(&model.SysAuditLog{})
	if q.UserID != nil {
		query = query.Where("user_id = ?", *q.UserID)
	}
	if q.Module != "" {
		query = query.Where("module = ?", q.Module)
	}
	if q.Action != "" {
		query = query.Where("action = ?", q.Action)
	}
	if q.StartTime != nil {
		query = query.Where("created_at >= ?", *q.StartTime)
	}
	if q.EndTime != nil {
		query = query.Where("created_at <= ?", *q.EndTime)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("created_at DESC").
		Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

type departmentRepo struct {
	db *gorm.DB
}

func NewDepartmentRepository(db *gorm.DB) DepartmentRepository {
	return &departmentRepo{db: db}
}

func (r *departmentRepo) Create(tx *gorm.DB, dept *model.SysDepartment) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Create(dept).Error
}

func (r *departmentRepo) FindByCode(code string) (*model.SysDepartment, error) {
	var d model.SysDepartment
	if err := r.db.Where("code = ?", code).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *departmentRepo) FindByID(id int64) (*model.SysDepartment, error) {
	var d model.SysDepartment
	if err := r.db.Where("id = ?", id).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *departmentRepo) UpdateFields(tx *gorm.DB, id int64, fields map[string]interface{}) error {
	if tx == nil {
		tx = r.db
	}
	if len(fields) == 0 {
		return nil
	}
	return tx.Model(&model.SysDepartment{}).Where("id = ?", id).Updates(fields).Error
}

func (r *departmentRepo) Delete(tx *gorm.DB, id int64) error {
	if tx == nil {
		tx = r.db
	}
	return tx.Where("id = ?", id).Delete(&model.SysDepartment{}).Error
}

func (r *departmentRepo) List() ([]model.SysDepartment, error) {
	var depts []model.SysDepartment
	if err := r.db.Order("path ASC, id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *departmentRepo) CountChildren(parentID int64) (int64, error) {
	var n int64
	err := r.db.Model(&model.SysDepartment{}).Where("parent_id = ?", parentID).Count(&n).Error
	return n, err
}

func (r *departmentRepo) CountUsers(deptID int64) (int64, error) {
	var n int64
	err := r.db.Model(&model.SysUser{}).Where("dept_id = ?", deptID).Count(&n).Error
	return n, err
}

func (r *departmentRepo) SubDeptIDs(deptID int64) ([]int64, error) {
	parent, err := r.FindByID(deptID)
	if err != nil {
		return nil, err
	}
	var ids []int64
	// parent.Path 形如 /1/3/；其自身及所有子孙 path 均以该前缀开头。
	if err := r.db.Model(&model.SysDepartment{}).
		Where("path LIKE ?", parent.Path+"%").
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

type loginAttemptRepo struct {
	db *gorm.DB
}

func NewLoginAttemptRepository(db *gorm.DB) LoginAttemptRepository {
	return &loginAttemptRepo{db: db}
}

func (r *loginAttemptRepo) Create(a *model.SysLoginAttempt) error {
	return r.db.Create(a).Error
}
