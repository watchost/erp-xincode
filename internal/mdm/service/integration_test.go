// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"errors"
	"strconv"
	"testing"

	"erp-system/internal/mdm/dto"
	"erp-system/internal/mdm/model"
	mdmRepository "erp-system/internal/mdm/repository"
	mdmService "erp-system/internal/mdm/service"
	apperrors "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
	"gorm.io/gorm"
)

func appErrCode(err error) int {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func newMDMService(gdb *gorm.DB) *mdmService.MDMService {
	return mdmService.NewMDMService(
		mdmRepository.NewMaterialRepository(gdb),
		mdmRepository.NewSupplierRepository(gdb),
		mdmRepository.NewWarehouseRepository(gdb),
		mdmRepository.NewLocationRepository(gdb),
		mdmRepository.NewCategoryRepository(gdb),
		mdmRepository.NewCustomerRepository(gdb),
		nil, // 分类/客户链路不触达库存守卫
	)
}

// 分类树：父子层级、物化路径、删除守卫（有子分类/有物料时拒绝）。
func TestCategoryTree_PathAndDeleteGuard(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	root := model.MdmMaterialCategory{Code: "RAW", Name: "原材料"}
	if err := svc.CreateCategory(ctx, &root); err != nil {
		t.Fatalf("创建根分类失败: %v", err)
	}
	if root.Path != "/"+itoa(root.ID)+"/" {
		t.Fatalf("根分类物化路径错误: %s", root.Path)
	}

	child := model.MdmMaterialCategory{Code: "RAW-METAL", Name: "金属", ParentID: &root.ID}
	if err := svc.CreateCategory(ctx, &child); err != nil {
		t.Fatalf("创建子分类失败: %v", err)
	}
	want := "/" + itoa(root.ID) + "/" + itoa(child.ID) + "/"
	if child.Path != want {
		t.Fatalf("子分类物化路径错误: got %s want %s", child.Path, want)
	}

	// 重复编码拒绝
	dup := model.MdmMaterialCategory{Code: "RAW", Name: "重复"}
	if err := svc.CreateCategory(ctx, &dup); appErrCode(err) != 10005 {
		t.Fatalf("重复编码应返回 10005, got %v", err)
	}

	// 父分类不存在拒绝
	badParent := int64(999999)
	orphan := model.MdmMaterialCategory{Code: "ORPHAN", Name: "孤儿", ParentID: &badParent}
	if err := svc.CreateCategory(ctx, &orphan); appErrCode(err) != 10100 {
		t.Fatalf("父分类不存在应返回 10100, got %v", err)
	}

	// 有子分类时删除拒绝
	if err := svc.DeleteCategory(ctx, root.ID); appErrCode(err) != 10006 {
		t.Fatalf("有子分类删除应返回 10006, got %v", err)
	}

	// 分类下有物料时删除拒绝
	mat := model.MdmMaterial{MaterialCode: "M-CAT-1", Name: "分类物料", Unit: "个", CategoryID: child.ID, Status: 1}
	if err := svc.CreateMaterial(ctx, &mat); err != nil {
		t.Fatalf("创建物料失败: %v", err)
	}
	if err := svc.DeleteCategory(ctx, child.ID); appErrCode(err) != 10006 {
		t.Fatalf("有物料删除应返回 10006, got %v", err)
	}

	// 树形结构装配正确
	tree, err := svc.ListCategoryTree(ctx)
	if err != nil {
		t.Fatalf("查询分类树失败: %v", err)
	}
	if len(tree) != 1 || tree[0].Code != "RAW" {
		t.Fatalf("分类树根节点错误: %+v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].Code != "RAW-METAL" {
		t.Fatalf("分类树子节点装配错误: %+v", tree[0].Children)
	}

}

// 分类软删除后编码可重建（部分唯一索引）。
func TestCategory_RecreateAfterSoftDelete(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	c := model.MdmMaterialCategory{Code: "TMP", Name: "临时"}
	if err := svc.CreateCategory(ctx, &c); err != nil {
		t.Fatalf("创建分类失败: %v", err)
	}
	if err := svc.DeleteCategory(ctx, c.ID); err != nil {
		t.Fatalf("删除分类失败: %v", err)
	}
	c2 := model.MdmMaterialCategory{Code: "TMP", Name: "临时v2"}
	if err := svc.CreateCategory(ctx, &c2); err != nil {
		t.Fatalf("软删除后重建同名分类失败: %v", err)
	}
	if c2.ID == c.ID {
		t.Fatal("重建分类应分配新 ID")
	}
}

// 客户 CRUD：编码唯一、软删除、分页查询。
func TestCustomer_CRUD(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	cu := model.MdmCustomer{CustomerCode: "C001", Name: "测试客户", CreditLimit: testsupport.D("50000"), Level: 2, Status: 1}
	if err := svc.CreateCustomer(ctx, &cu); err != nil {
		t.Fatalf("创建客户失败: %v", err)
	}

	dup := model.MdmCustomer{CustomerCode: "C001", Name: "重复客户"}
	if err := svc.CreateCustomer(ctx, &dup); appErrCode(err) != 10005 {
		t.Fatalf("重复客户编码应返回 10005, got %v", err)
	}

	vo, err := svc.GetCustomer(ctx, cu.ID)
	if err != nil {
		t.Fatalf("查询客户失败: %v", err)
	}
	if !vo.CreditLimit.Equal(testsupport.D("50000")) || vo.Level != 2 {
		t.Fatalf("客户字段回读错误: %+v", vo)
	}

	list, total, err := svc.ListCustomers(ctx, dto.CustomerListReq{CustomerCode: "C00", Page: 1, PageSize: 20})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("客户分页查询错误: total=%d len=%d err=%v", total, len(list), err)
	}

	if err := svc.DeleteCustomer(ctx, cu.ID); err != nil {
		t.Fatalf("删除客户失败: %v", err)
	}
	if _, err := svc.GetCustomer(ctx, cu.ID); appErrCode(err) != 10100 {
		t.Fatalf("删除后查询应返回 10100, got %v", err)
	}

	// 软删除后同编码可重建
	cu2 := model.MdmCustomer{CustomerCode: "C001", Name: "测试客户v2"}
	if err := svc.CreateCustomer(ctx, &cu2); err != nil {
		t.Fatalf("软删除后重建同编码客户失败: %v", err)
	}
}

// 物料新字段：采购/库存单位与换算率落库回读。
func TestMaterial_UnitConversionFields(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	rate := testsupport.D("12.5")
	m := model.MdmMaterial{
		MaterialCode:   "M-UNIT-1",
		Name:           "单位换算物料",
		Unit:           "支",
		PurchaseUnit:   "箱",
		StockUnit:      "支",
		ConversionRate: &rate,
		Status:         1,
	}
	if err := svc.CreateMaterial(ctx, &m); err != nil {
		t.Fatalf("创建物料失败: %v", err)
	}

	vo, err := svc.GetMaterial(ctx, m.ID)
	if err != nil {
		t.Fatalf("查询物料失败: %v", err)
	}
	if vo.PurchaseUnit != "箱" || vo.StockUnit != "支" {
		t.Fatalf("单位字段回读错误: %+v", vo)
	}
	if vo.ConversionRate == nil || !vo.ConversionRate.Equal(testsupport.D("12.5")) {
		t.Fatalf("换算率回读错误: %v", vo.ConversionRate)
	}

	// 不传换算率时落库默认值 1
	m2 := model.MdmMaterial{MaterialCode: "M-UNIT-2", Name: "默认换算物料", Unit: "个", Status: 1}
	if err := svc.CreateMaterial(ctx, &m2); err != nil {
		t.Fatalf("创建默认换算物料失败: %v", err)
	}
	vo2, err := svc.GetMaterial(ctx, m2.ID)
	if err != nil {
		t.Fatalf("查询默认换算物料失败: %v", err)
	}
	if vo2.ConversionRate == nil || !vo2.ConversionRate.Equal(testsupport.D("1")) {
		t.Fatalf("默认换算率应为 1: %v", vo2.ConversionRate)
	}
}

// P1.5 S5 回归：客户部分更新（只发 customer_code+name）不得清零风控字段，
// 也不得破坏 created_at。
func TestCustomer_PartialUpdateKeepsRiskFields(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	cu := model.MdmCustomer{
		CustomerCode: "C-PART-1",
		Name:         "部分更新客户",
		CreditLimit:  testsupport.D("8888"),
		Level:        2,
		Status:       1,
	}
	if err := svc.CreateCustomer(ctx, &cu); err != nil {
		t.Fatalf("创建客户失败: %v", err)
	}
	before, err := svc.GetCustomer(ctx, cu.ID)
	if err != nil {
		t.Fatalf("查询客户失败: %v", err)
	}

	// 模拟客户端只发 {"customer_code","name"}：其余字段指针全 nil。
	newCode, newName := "C-PART-1", "改名客户"
	if err := svc.UpdateCustomer(ctx, cu.ID, dto.CustomerUpdateReq{CustomerCode: &newCode, Name: &newName}); err != nil {
		t.Fatalf("部分更新客户失败: %v", err)
	}

	after, err := svc.GetCustomer(ctx, cu.ID)
	if err != nil {
		t.Fatalf("更新后查询客户失败: %v", err)
	}
	if after.Name != newName {
		t.Fatalf("名称未更新: %+v", after)
	}
	if !after.CreditLimit.Equal(testsupport.D("8888")) {
		t.Fatalf("credit_limit 被改写: got %s want 8888", after.CreditLimit)
	}
	if after.Level != 2 || after.Status != 1 {
		t.Fatalf("level/status 被改写: got level=%d status=%d want 2/1", after.Level, after.Status)
	}
	if after.CreatedAt != before.CreatedAt {
		t.Fatalf("created_at 被改写: got %s want %s", after.CreatedAt, before.CreatedAt)
	}

	// 显式传零值应生效（与「未传」区分）：credit_limit=0 是合法的显式清零。
	zero := testsupport.D("0")
	if err := svc.UpdateCustomer(ctx, cu.ID, dto.CustomerUpdateReq{CreditLimit: &zero}); err != nil {
		t.Fatalf("显式清零 credit_limit 失败: %v", err)
	}
	vo, err := svc.GetCustomer(ctx, cu.ID)
	if err != nil {
		t.Fatalf("查询客户失败: %v", err)
	}
	if !vo.CreditLimit.IsZero() {
		t.Fatalf("显式传 0 应清零 credit_limit: got %s", vo.CreditLimit)
	}
}

// P1.6 H6 回归：不传 conversion_rate 更新物料成功且原值保留；传 0/负数被拒绝。
func TestMaterial_UpdateConversionRateGuard(t *testing.T) {
	gdb := testsupport.NewDB(t)
	svc := newMDMService(gdb)
	ctx := testsupport.Ctx()

	rate := testsupport.D("12.5")
	m := model.MdmMaterial{
		MaterialCode:   "M-UPD-1",
		Name:           "换算率守卫物料",
		Unit:           "支",
		PurchaseUnit:   "箱",
		StockUnit:      "支",
		ConversionRate: &rate,
		Status:         1,
	}
	if err := svc.CreateMaterial(ctx, &m); err != nil {
		t.Fatalf("创建物料失败: %v", err)
	}
	before, err := svc.GetMaterial(ctx, m.ID)
	if err != nil {
		t.Fatalf("查询物料失败: %v", err)
	}

	// 不传 conversion_rate：必须成功且原值 12.5 保留（修复前此处 23502 → 10300）。
	newName := "换算率守卫物料v2"
	if err := svc.UpdateMaterial(ctx, m.ID, dto.MaterialUpdateReq{Name: &newName}); err != nil {
		t.Fatalf("不传 conversion_rate 更新物料失败: %v", err)
	}
	after, err := svc.GetMaterial(ctx, m.ID)
	if err != nil {
		t.Fatalf("更新后查询物料失败: %v", err)
	}
	if after.Name != newName {
		t.Fatalf("名称未更新: %+v", after)
	}
	if after.ConversionRate == nil || !after.ConversionRate.Equal(testsupport.D("12.5")) {
		t.Fatalf("conversion_rate 未保留: got %v", after.ConversionRate)
	}
	if after.CreatedAt != before.CreatedAt {
		t.Fatalf("created_at 被改写: got %s want %s", after.CreatedAt, before.CreatedAt)
	}

	// 传 0 或负数必须拒绝（10005）。
	zero := testsupport.D("0")
	if err := svc.UpdateMaterial(ctx, m.ID, dto.MaterialUpdateReq{ConversionRate: &zero}); appErrCode(err) != 10005 {
		t.Fatalf("conversion_rate=0 应返回 10005, got %v", err)
	}
	neg := testsupport.D("-2")
	if err := svc.UpdateMaterial(ctx, m.ID, dto.MaterialUpdateReq{ConversionRate: &neg}); appErrCode(err) != 10005 {
		t.Fatalf("conversion_rate=-2 应返回 10005, got %v", err)
	}

	// 传正数正常覆盖。
	two := testsupport.D("2")
	if err := svc.UpdateMaterial(ctx, m.ID, dto.MaterialUpdateReq{ConversionRate: &two}); err != nil {
		t.Fatalf("更新 conversion_rate=2 失败: %v", err)
	}
	vo, err := svc.GetMaterial(ctx, m.ID)
	if err != nil {
		t.Fatalf("查询物料失败: %v", err)
	}
	if vo.ConversionRate == nil || !vo.ConversionRate.Equal(two) {
		t.Fatalf("conversion_rate 应为 2: got %v", vo.ConversionRate)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
