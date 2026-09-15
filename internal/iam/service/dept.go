// Copyright 2026 zhouhouping. All Rights Reserved.

package service

import (
	"context"
	"strconv"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	"erp-system/internal/iam/model"
	"erp-system/internal/pkg/errors"
)

func (s *IAMService) CreateDepartment(ctx context.Context, req dto.DepartmentReq) error {
	if existing, _ := s.deptRepo.FindByCode(req.Code); existing != nil {
		return errors.New(10006, 409, "部门编码已存在")
	}
	var parentPath string
	if req.ParentID != nil && *req.ParentID != 0 {
		parent, err := s.deptRepo.FindByID(*req.ParentID)
		if err != nil {
			return errors.New(10005, 400, "上级部门不存在")
		}
		parentPath = parent.Path
	}
	dept := &model.SysDepartment{
		Code:     req.Code,
		Name:     req.Name,
		ParentID: req.ParentID,
		LeaderID: req.LeaderID,
		Path:     "/", // 先落库拿 ID，再回填物化路径
	}
	if err := s.deptRepo.Create(nil, dept); err != nil {
		return err
	}
	dept.Path = parentPath + strconv.FormatInt(dept.ID, 10) + "/"
	return s.deptRepo.UpdateFields(nil, dept.ID, map[string]interface{}{"path": dept.Path})
}

// UpdateDepartment 仅允许改编码/名称/负责人；调整父子关系会影响子树物化路径，本期不支持。
func (s *IAMService) UpdateDepartment(ctx context.Context, id int64, req dto.DepartmentReq) error {
	if _, err := s.deptRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "部门不存在")
	}
	if existing, _ := s.deptRepo.FindByCode(req.Code); existing != nil && existing.ID != id {
		return errors.New(10006, 409, "部门编码已存在")
	}
	return s.deptRepo.UpdateFields(nil, id, map[string]interface{}{
		"code":       req.Code,
		"name":       req.Name,
		"leader_id":  req.LeaderID,
		"updated_at": time.Now(),
	})
}

func (s *IAMService) DeleteDepartment(ctx context.Context, id int64) error {
	if _, err := s.deptRepo.FindByID(id); err != nil {
		return errors.New(10100, 404, "部门不存在")
	}
	if n, err := s.deptRepo.CountChildren(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(10006, 409, "存在下级部门，不能删除")
	}
	if n, err := s.deptRepo.CountUsers(id); err != nil {
		return err
	} else if n > 0 {
		return errors.New(10006, 409, "部门下仍有用户，不能删除")
	}
	return s.txManager.WithTx(ctx, func(tx *gorm.DB) error {
		// 自定义数据权限中残留的部门关联一并清理
		if err := tx.Where("dept_id = ?", id).Delete(&model.SysRoleDept{}).Error; err != nil {
			return err
		}
		// 解绑用户部门（UpdateColumn 直接写 NULL，绕过零值省略）
		if err := tx.Table("sys_user").Where("dept_id = ?", id).
			UpdateColumn("dept_id", nil).Error; err != nil {
			return err
		}
		return s.deptRepo.Delete(tx, id)
	})
}

// DepartmentTree 返回部门树（用户管理下拉与部门管理页共用）。
func (s *IAMService) DepartmentTree(ctx context.Context) ([]dto.DepartmentVO, error) {
	depts, err := s.deptRepo.List()
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]dto.DepartmentVO, len(depts))
	childIDs := make(map[int64][]int64)
	roots := make([]int64, 0)
	for _, d := range depts {
		byID[d.ID] = dto.DepartmentVO{
			ID: d.ID, Code: d.Code, Name: d.Name,
			ParentID: d.ParentID, LeaderID: d.LeaderID,
		}
		if d.ParentID == nil || *d.ParentID == 0 {
			roots = append(roots, d.ID)
		} else {
			childIDs[*d.ParentID] = append(childIDs[*d.ParentID], d.ID)
		}
	}
	var attach func(id int64) dto.DepartmentVO
	attach = func(id int64) dto.DepartmentVO {
		vo := byID[id]
		for _, cid := range childIDs[id] {
			vo.Children = append(vo.Children, attach(cid))
		}
		return vo
	}
	tree := make([]dto.DepartmentVO, 0, len(roots))
	for _, id := range roots {
		tree = append(tree, attach(id))
	}
	return tree, nil
}
