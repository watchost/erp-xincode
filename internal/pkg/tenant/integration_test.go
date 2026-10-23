// Copyright 2026 zhouhouping. All Rights Reserved.

package tenant_test

import (
	"context"
	stdErrors "errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/auth"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/pkg/tenant"
	"erp-system/internal/testsupport"
)

func code(err error) int {
	var ae *apperrors.AppError
	if stdErrors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func ctxT(tid int64) context.Context { return auth.WithTenantID(context.Background(), tid) }

// §4.8 验收：两个租户数据隔离（经 ctx 的 GORM 查询/更新/创建）。
func TestTenantIsolation_ViaContext(t *testing.T) {
	gdb := testsupport.NewDB(t)
	a, b := ctxT(101), ctxT(202)

	ma := &mdmModel.MdmMaterial{MaterialCode: "ISO-1", Name: "A 的物料", Unit: "个", CostMethod: 1, Status: 1}
	mb := &mdmModel.MdmMaterial{MaterialCode: "ISO-1", Name: "B 的物料", Unit: "个", CostMethod: 1, Status: 1, TenantID: 101} // 伪造租户
	if err := gdb.WithContext(a).Create(ma).Error; err != nil {
		t.Fatalf("租户 A 建物料失败: %v", err)
	}
	if err := gdb.WithContext(b).Create(mb).Error; err != nil {
		t.Fatalf("租户 B 用相同编码建物料应成功（唯一约束含 tenant_id）: %v", err)
	}
	if ma.TenantID != 101 || mb.TenantID != 202 {
		t.Fatalf("创建时应强制写入上下文租户：A=%d B=%d", ma.TenantID, mb.TenantID)
	}

	var got mdmModel.MdmMaterial
	if err := gdb.WithContext(a).First(&got, mb.ID).Error; !stdErrors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("租户 A 按 ID 读取 B 的物料应查不到，实际 err=%v name=%s", err, got.Name)
	}
	var list []mdmModel.MdmMaterial
	gdb.WithContext(a).Where("material_code = ?", "ISO-1").Find(&list)
	if len(list) != 1 || list[0].ID != ma.ID {
		t.Fatalf("租户 A 按编码查询应只看到自己的 1 条，实际 %d", len(list))
	}
	var n int64
	gdb.WithContext(b).Model(&mdmModel.MdmMaterial{}).Where("material_code = ?", "ISO-1").Count(&n)
	if n != 1 {
		t.Fatalf("Count 也应按租户过滤，实际 %d", n)
	}
	var ids []int64
	gdb.WithContext(a).Table("mdm_material m").Select("m.id").Where("m.material_code = ?", "ISO-1").Scan(&ids)
	if len(ids) != 1 || ids[0] != ma.ID {
		t.Fatalf("带别名的 Table 查询也应过滤，实际 %v", ids)
	}

	res := gdb.WithContext(a).Model(&mdmModel.MdmMaterial{}).Where("id = ?", mb.ID).Update("name", "被篡改")
	if res.Error != nil || res.RowsAffected != 0 {
		t.Fatalf("跨租户更新应影响 0 行，实际 %d err=%v", res.RowsAffected, res.Error)
	}
	gdb.WithContext(b).First(&got, mb.ID)
	if got.Name != "B 的物料" {
		t.Fatal("B 的数据不应被 A 修改")
	}
	res = gdb.WithContext(a).Delete(&mdmModel.MdmMaterial{}, mb.ID)
	if res.RowsAffected != 0 {
		t.Fatal("跨租户删除应影响 0 行")
	}
}

// 已知缺口（如实记录）：repository 方法不带 ctx，插件拿不到租户，查询不会被过滤。
// 本测试断言缺口存在；repository 透传 ctx 改造完成后应改为断言查不到。
func TestTenantIsolation_KnownGap_RepoWithoutCtx(t *testing.T) {
	gdb := testsupport.NewDB(t)
	mb := &mdmModel.MdmMaterial{MaterialCode: "GAP-1", Name: "B", Unit: "个", CostMethod: 1, Status: 1}
	if err := gdb.WithContext(ctxT(202)).Create(mb).Error; err != nil {
		t.Fatal(err)
	}
	got, err := mdmRepo.NewMaterialRepository(gdb).FindByID(mb.ID)
	if err != nil || got == nil {
		t.Fatalf("缺口描述已失效（repo 已能隔离？）请更新文档与本测试: err=%v", err)
	}
}

func TestTenantCreate_CopiesSeedsAndAdmin(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := tenant.NewService(gdb)
	platform := ctxT(0)
	admin := fmt.Sprintf("tadm_%d", time.Now().UnixNano())

	tn, err := svc.Create(platform, tenant.CreateReq{Code: "acme", Name: "ACME", AdminUsername: admin, AdminPassword: "Passw0rd!"})
	if err != nil {
		t.Fatalf("建租户失败: %v", err)
	}
	var seed, copied, crossParent int64
	gdb.Table("fin_account").Where("tenant_id = 0").Count(&seed)
	gdb.Table("fin_account").Where("tenant_id = ?", tn.ID).Count(&copied)
	if seed == 0 || copied != seed {
		t.Fatalf("科目表应完整复制：种子 %d，新租户 %d", seed, copied)
	}
	gdb.Raw(`SELECT COUNT(*) FROM fin_account c JOIN fin_account p ON p.id = c.parent_id
		WHERE c.tenant_id = ? AND p.tenant_id <> c.tenant_id`, tn.ID).Scan(&crossParent)
	if crossParent != 0 {
		t.Fatalf("复制后的科目 parent_id 不应指向其他租户，发现 %d 条", crossParent)
	}
	var defs int64
	gdb.Table("wf_definition").Where("tenant_id = ? AND code = 'purchase_order' AND status = 1", tn.ID).Count(&defs)
	if defs != 1 {
		t.Fatal("应复制已部署的采购审批流程")
	}
	var adminRole int64
	gdb.Raw(`SELECT COUNT(*) FROM sys_user u JOIN sys_user_role ur ON ur.user_id = u.id JOIN sys_role r ON r.id = ur.role_id
		WHERE u.username = ? AND u.tenant_id = ? AND r.code = 'admin'`, admin, tn.ID).Scan(&adminRole)
	if adminRole != 1 {
		t.Fatal("应创建该租户的 admin 用户并绑定 admin 角色")
	}

	if _, err := svc.Create(ctxT(tn.ID), tenant.CreateReq{Code: "x", Name: "x", AdminUsername: admin + "x", AdminPassword: "Passw0rd!"}); code(err) != 81004 {
		t.Fatalf("非平台租户建租户应返回 81004，实际 %v", err)
	}
	if _, err := svc.Create(platform, tenant.CreateReq{Code: "acme", Name: "dup", AdminUsername: admin + "y", AdminPassword: "Passw0rd!"}); code(err) != 81005 {
		t.Fatalf("租户编码重复应返回 81005，实际 %v", err)
	}
}

func TestTenantCheck_DisabledAndExpired(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := tenant.NewService(gdb)
	platform := ctxT(0)
	ok, err := svc.Create(platform, tenant.CreateReq{Code: "chk", Name: "chk",
		AdminUsername: fmt.Sprintf("tchk_%d", time.Now().UnixNano()), AdminPassword: "Passw0rd!"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Check(context.Background(), ok.ID, true); err != nil {
		t.Fatalf("启用租户应通过: %v", err)
	}
	if err := svc.SetStatus(platform, ok.ID, tenant.StatusDisabled); err != nil {
		t.Fatal(err)
	}
	if err := svc.Check(context.Background(), ok.ID, false); code(err) != 81002 {
		t.Fatalf("停用租户应返回 81002（读也拒绝），实际 %v", err)
	}
	if err := svc.SetStatus(platform, 0, tenant.StatusDisabled); err == nil {
		t.Fatal("默认租户不能停用")
	}

	past := time.Now().Add(-time.Hour)
	exp, err := svc.Create(platform, tenant.CreateReq{Code: "exp", Name: "exp", ExpireAt: &past,
		AdminUsername: fmt.Sprintf("texp_%d", time.Now().UnixNano()), AdminPassword: "Passw0rd!"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Check(context.Background(), exp.ID, false); err != nil {
		t.Fatalf("过期租户读操作应放行: %v", err)
	}
	if err := svc.Check(context.Background(), exp.ID, true); code(err) != 81003 {
		t.Fatalf("过期租户写操作应返回 81003，实际 %v", err)
	}
}
