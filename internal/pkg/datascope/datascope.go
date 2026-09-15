// Copyright 2026 zhouhouping. All Rights Reserved.

// Package datascope 提供基于角色 data_scope 的行级数据权限过滤。
//
// 档位（与 sys_role.data_scope 一致）：
//
//	1 全部数据           不加条件
//	2 本部门             dept_col = 调用者部门
//	3 本部门及下级部门    dept_col IN 子孙部门集合（含本部门，调用方先查部门树解析）
//	4 仅本人             owner_col = 调用者用户 ID
//	5 自定义部门          dept_col IN sys_role_dept 配置的部门集合
//
// 多角色合并取最宽档（1>3>2>5>4）；自定义部门集合并集。合并在 IAM 登录/查询层完成，
// 本包只做无状态的条件构造，便于单测与复用。主数据/库存/财务等共享数据不套 scope。
package datascope

import "gorm.io/gorm"

const (
	ScopeAll             = 1
	ScopeDept            = 2
	ScopeDeptAndChildren = 3
	ScopeSelf            = 4
	ScopeCustomDept      = 5
)

// Params 描述一次查询的数据权限上下文。
// OwnerCol 是单据属主列（如 created_by；用户管理场景传 "id"）。
// DeptCol 是单据部门列（如 dept_id）。DeptIDs 是模式 3/5 已解析好的部门集合。
type Params struct {
	Scope    int
	UserID   int64
	DeptID   int64
	DeptIDs  []int64
	OwnerCol string
	DeptCol  string
}

// Apply 返回可直接传入 db.Scopes(...) 的 GORM 条件。
func Apply(p Params) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		switch p.Scope {
		case ScopeAll:
			return db
		case ScopeDept:
			if p.DeptID == 0 || p.DeptCol == "" {
				return db.Where("1 = 0")
			}
			return db.Where(p.DeptCol+" = ?", p.DeptID)
		case ScopeDeptAndChildren, ScopeCustomDept:
			if p.DeptCol == "" || len(p.DeptIDs) == 0 {
				return db.Where("1 = 0")
			}
			return db.Where(p.DeptCol+" IN ?", p.DeptIDs)
		case ScopeSelf:
			if p.OwnerCol == "" {
				return db.Where("1 = 0")
			}
			return db.Where(p.OwnerCol+" = ?", p.UserID)
		default:
			// 未知档位按最严处理，默认放行等于权限失守
			return db.Where("1 = 0")
		}
	}
}

// Merge 合并多角色的 data_scope 取最宽档，返回合并后的档位。
// 排序：全部(1) > 部门及下级(3) > 本部门(2) > 自定义(5) > 仅本人(4)。
// 无角色时默认最严（仅本人），由调用方决定无角色用户的处置。
func Merge(scopes []int) int {
	if len(scopes) == 0 {
		return ScopeSelf
	}
	best := ScopeSelf
	rank := map[int]int{ScopeAll: 5, ScopeDeptAndChildren: 4, ScopeDept: 3, ScopeCustomDept: 2, ScopeSelf: 1}
	for _, s := range scopes {
		if rank[s] > rank[best] {
			best = s
		}
	}
	return best
}

// CanAccess 判定属主为 recordOwnerID、部门为 recordDeptID 的单条记录是否可见。
// 用于 GET 详情类接口拿到行之后的二次校验。deptIDs 为模式 3/5 解析好的集合。
func CanAccess(scope int, userID, userDeptID int64, deptIDs []int64,
	recordOwnerID, recordDeptID int64) bool {
	switch scope {
	case ScopeAll:
		return true
	case ScopeDept:
		return userDeptID != 0 && recordDeptID == userDeptID
	case ScopeDeptAndChildren, ScopeCustomDept:
		for _, id := range deptIDs {
			if recordDeptID == id {
				return true
			}
		}
		return false
	case ScopeSelf:
		return recordOwnerID == userID
	default:
		return false
	}
}
