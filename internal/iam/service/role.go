// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"

	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/pkg/datascope"
	"erp-system/internal/pkg/errors"
)

const builtinAdminRoleCode = "admin"

func modelRole(code, name string, scope int) *model.SysRole {
	return &model.SysRole{Code: code, Name: name, DataScope: scope}
}

func (s *IAMService) ListRoles(ctx context.Context) ([]dto.RoleVO, error) {
	roles, err := s.roleRepo.List()
	if err != nil {
		return nil, err
	}
	vos := make([]dto.RoleVO, 0, len(roles))
	for _, r := range roles {
		vos = append(vos, dto.RoleVO{ID: r.ID, Code: r.Code, Name: r.Name, DataScope: r.DataScope})
	}
	return vos, nil
}

func (s *IAMService) GetRoleDetail(ctx context.Context, id int64) (*dto.RoleDetailVO, error) {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return nil, errors.New(10100, 404, "角色不存在")
	}
	permIDs, err := s.roleRepo.ListPermissionIDsByRole(id)
	if err != nil {
		return nil, err
	}
	deptIDs, err := s.roleRepo.ListRoleDeptIDs([]int64{id})
	if err != nil {
		return nil, err
	}
	if permIDs == nil {
		permIDs = []int64{}
	}
	if deptIDs == nil {
		deptIDs = []int64{}
	}
	return &dto.RoleDetailVO{
		RoleVO:        dto.RoleVO{ID: role.ID, Code: role.Code, Name: role.Name, DataScope: role.DataScope},
		PermissionIDs: permIDs,
		DeptIDs:       deptIDs,
	}, nil
}

func (s *IAMService) CreateRole(ctx context.Context, req dto.CreateRoleReq) error {
	if existing, _ := s.roleRepo.FindByCode(req.Code); existing != nil {
		return errors.New(10006, 409, "角色编码已存在")
	}
	return s.roleRepo.Create(nil, modelRole(req.Code, req.Name, req.DataScope))
}

func (s *IAMService) UpdateRole(ctx context.Context, id int64, req dto.UpdateRoleReq) error {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return errors.New(10100, 404, "角色不存在")
	}
	fields := make(map[string]interface{})
	if req.Name != nil {
		fields["name"] = *req.Name
	}
	if req.DataScope != nil {
		if role.Code == builtinAdminRoleCode && *req.DataScope != datascope.ScopeAll {
			return errors.New(10006, 409, "内置管理员角色必须保持全部数据权限")
		}
		fields["data_scope"] = *req.DataScope
	}
	return s.roleRepo.UpdateFields(nil, id, fields)
}

func (s *IAMService) DeleteRole(ctx context.Context, id int64) error {
	role, err := s.roleRepo.FindByID(id)
	if err != nil {
		return errors.New(10100, 404, "角色不存在")
	}
	if role.Code == builtinAdminRoleCode {
		return errors.New(10006, 409, "内置管理员角色不可删除")
	}
	n, err := s.roleRepo.CountUsersByRole(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return errors.New(10006, 409, "角色仍有用户绑定，不能删除")
	}
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		if err := s.roleRepo.ReplaceRolePermissions(tx, id, nil); err != nil {
			return err
		}
		if err := s.roleRepo.ReplaceRoleDepts(tx, id, nil); err != nil {
			return err
		}
		return s.roleRepo.Delete(tx, id)
	})
}

func (s *IAMService) AssignPermissions(ctx context.Context, roleID int64, req dto.AssignPermsReq) error {
	if _, err := s.roleRepo.FindByID(roleID); err != nil {
		return errors.New(10100, 404, "角色不存在")
	}
	permIDs := uniqueInt64(req.PermissionIDs)
	perms, err := s.permRepo.FindByIDs(permIDs)
	if err != nil {
		return errors.Wrap(err, 10300, 500, "校验权限失败")
	}
	if len(perms) != len(permIDs) {
		return errors.New(10005, 400, "存在非法权限ID")
	}
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return s.roleRepo.ReplaceRolePermissions(tx, roleID, permIDs)
	})
}

func (s *IAMService) AssignRoleDepts(ctx context.Context, roleID int64, req dto.AssignRoleDeptsReq) error {
	if _, err := s.roleRepo.FindByID(roleID); err != nil {
		return errors.New(10100, 404, "角色不存在")
	}
	deptIDs := uniqueInt64(req.DeptIDs)
	for _, did := range deptIDs {
		if _, err := s.deptRepo.FindByID(did); err != nil {
			return errors.New(10005, 400, "部门不存在")
		}
	}
	// 非 data_scope=5 档位允许预配部门（保存顺序无要求），但仅在 5 时生效。
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		return s.roleRepo.ReplaceRoleDepts(tx, roleID, deptIDs)
	})
}

// PermissionTree 返回全部权限并按 parent_id 组装成树。
// 先按 parent 建 ID 邻接表，再从根递归出参，避免值拷贝顺序导致的孙节点丢失。
func (s *IAMService) PermissionTree(ctx context.Context) ([]dto.PermissionVO, error) {
	perms, err := s.permRepo.List()
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]model.SysPermission, len(perms))
	children := make(map[int64][]int64)
	roots := make([]int64, 0)
	for _, p := range perms {
		byID[p.ID] = p
		if p.ParentID == 0 {
			roots = append(roots, p.ID)
		} else {
			children[p.ParentID] = append(children[p.ParentID], p.ID)
		}
	}
	var build func(id int64) dto.PermissionVO
	build = func(id int64) dto.PermissionVO {
		p := byID[id]
		vo := dto.PermissionVO{ID: p.ID, Code: p.Code, Name: p.Name, Type: p.Type, ParentID: p.ParentID, Path: p.Path}
		for _, cid := range children[id] {
			vo.Children = append(vo.Children, build(cid))
		}
		return vo
	}
	tree := make([]dto.PermissionVO, 0, len(roots))
	for _, id := range roots {
		tree = append(tree, build(id))
	}
	return tree, nil
}
