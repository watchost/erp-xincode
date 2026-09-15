// Copyright 2026 zhouhouping. All Rights Reserved.

package datascope_test

import (
	"testing"

	"erp-system/internal/pkg/datascope"
)

func TestMerge(t *testing.T) {
	cases := []struct {
		name string
		in   []int
		want int
	}{
		{"无角色默认仅本人", nil, datascope.ScopeSelf},
		{"仅本人+自定义取自定义", []int{datascope.ScopeSelf, datascope.ScopeCustomDept}, datascope.ScopeCustomDept},
		{"本部门+自定义取本部门", []int{datascope.ScopeCustomDept, datascope.ScopeDept}, datascope.ScopeDept},
		{"本部门+部门下级取下级", []int{datascope.ScopeDept, datascope.ScopeDeptAndChildren}, datascope.ScopeDeptAndChildren},
		{"任一全部即全部", []int{datascope.ScopeSelf, datascope.ScopeAll}, datascope.ScopeAll},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := datascope.Merge(c.in); got != c.want {
				t.Fatalf("Merge(%v) = %d, want %d", c.in, got, c.want)
			}
		})
	}
}

func TestCanAccessMatrix(t *testing.T) {
	// 用户 7 属部门 10；模式 3/5 可见部门集合 [10, 11, 12]
	const uid, d = 7, int64(10)
	deptSet := []int64{10, 11, 12}

	type row struct {
		owner, dept int64
	}
	own, otherDept, outside := row{owner: 7, dept: 10}, row{owner: 8, dept: 11}, row{owner: 9, dept: 99}

	check := func(scope int, r row) bool {
		return datascope.CanAccess(scope, uid, d, deptSet, r.owner, r.dept)
	}

	if !check(datascope.ScopeAll, outside) {
		t.Fatal("全部数据应可见任意记录")
	}
	if !check(datascope.ScopeDept, own) || check(datascope.ScopeDept, otherDept) {
		t.Fatal("本部门档只能看同部门")
	}
	if !check(datascope.ScopeDeptAndChildren, otherDept) || check(datascope.ScopeDeptAndChildren, outside) {
		t.Fatal("部门及下级档可见集合内、不可见集合外")
	}
	if !check(datascope.ScopeCustomDept, otherDept) || check(datascope.ScopeCustomDept, outside) {
		t.Fatal("自定义档按部门集合判定")
	}
	if !check(datascope.ScopeSelf, own) || check(datascope.ScopeSelf, otherDept) {
		t.Fatal("仅本人档只能看自己")
	}

	// 无部门用户在部门档下应被拒
	if datascope.CanAccess(datascope.ScopeDept, uid, 0, nil, 7, 0) {
		t.Fatal("无部门用户不应命中任何部门数据")
	}
}
