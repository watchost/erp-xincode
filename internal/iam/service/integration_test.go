// Copyright 2026 zhouhouping. All Rights Reserved.

package service_test

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"gorm.io/gorm"

	"erp-system/internal/iam/dto"
	iamModel "erp-system/internal/iam/model"
	"erp-system/internal/pkg/auth"
	pkgerr "erp-system/internal/pkg/errors"
	"erp-system/internal/testsupport"
)

// setup 装配真实 Postgres + 假 KV 的 IAM service。sys_* 表不做 TRUNCATE，
// 用例一律使用纳秒级后缀的唯一用户名/编码，避免跨用例互相污染。
func setup(t *testing.T) (*gorm.DB, testsupport.IAMDeps, string) {
	t.Helper()
	gdb := testsupport.NewDB(t)
	deps := testsupport.NewIAMService(gdb)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1e10)
	return gdb, deps, suffix
}

func appErrorCode(err error) int {
	var ae *pkgerr.AppError
	if stderrors.As(err, &ae) {
		return ae.Code
	}
	return 0
}

func i64(v int64) *int64 { return &v }

// loginAndCtx 走完整登录，解析 access token 后按 JWTAuth 的方式重建标准 ctx。
func loginAndCtx(t *testing.T, deps testsupport.IAMDeps, username, password string) (context.Context, dto.LoginRes) {
	t.Helper()
	res, err := deps.Service.Login(context.Background(), dto.LoginReq{Username: username, Password: password})
	if err != nil {
		t.Fatalf("登录失败 user=%s: %v", username, err)
	}
	claims, err := auth.ParseToken(testsupport.IAMJWTSecret, res.AccessToken)
	if err != nil {
		t.Fatalf("解析 token 失败: %v", err)
	}
	ctx := auth.WithUserID(context.Background(), claims.UserID)
	ctx = auth.WithUsername(ctx, claims.Username)
	ctx = auth.WithTenantID(ctx, claims.TenantID)
	ctx = auth.WithDeptID(ctx, claims.DeptID)
	ctx = auth.WithDataScope(ctx, claims.DataScope)
	ctx = auth.WithJTI(ctx, claims.JTI)
	return ctx, res
}

func createTestUser(t *testing.T, deps testsupport.IAMDeps, nameSuffix string, roleIDs []int64, deptID *int64) string {
	t.Helper()
	username := "it_" + nameSuffix
	err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username,
		Password: "Passw0rd!",
		RealName: "集成用户" + nameSuffix,
		DeptID:   deptID,
		RoleIDs:  roleIDs,
		Status:   1,
	})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return username
}

// ---- 用例 1：角色生命周期 ----

func TestRoleLifecycle(t *testing.T) {
	_, deps, suffix := setup(t)
	code := "rt" + suffix

	err := deps.Service.CreateRole(context.Background(), dto.CreateRoleReq{Code: code, Name: "测试角色", DataScope: 2})
	if err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	// code 重复 → 409
	if code := appErrorCode(deps.Service.CreateRole(context.Background(),
		dto.CreateRoleReq{Code: code, Name: "重复", DataScope: 2})); code != 10006 {
		t.Fatalf("重复 code 期望 10006，得到 %d", code)
	}

	roles, err := deps.Service.ListRoles(context.Background())
	if err != nil {
		t.Fatalf("角色列表失败: %v", err)
	}
	var roleID int64
	for _, r := range roles {
		if r.Code == code {
			roleID = r.ID
		}
	}
	if roleID == 0 {
		t.Fatal("列表中找不到新建角色")
	}

	newName := "改名角色"
	if err := deps.Service.UpdateRole(context.Background(), roleID,
		dto.UpdateRoleReq{Name: &newName}); err != nil {
		t.Fatalf("更新角色失败: %v", err)
	}
	detail, err := deps.Service.GetRoleDetail(context.Background(), roleID)
	if err != nil {
		t.Fatalf("角色详情失败: %v", err)
	}
	if detail.Name != newName {
		t.Fatalf("角色名未更新，得到 %s", detail.Name)
	}

	// 内置 admin 角色不可删除，且必须保持全部数据权限
	if err := deps.Service.DeleteRole(context.Background(), 1); err == nil {
		t.Fatal("删除内置 admin 角色应当被拒绝")
	}
	lockedScope := 4
	if err := deps.Service.UpdateRole(context.Background(), 1,
		dto.UpdateRoleReq{DataScope: &lockedScope}); err == nil {
		t.Fatal("admin 角色改为非全部数据权限应当被拒绝")
	}

	// 绑定了用户的角色不可删除
	boundSuffix := suffix + "b"
	createTestUser(t, deps, boundSuffix, []int64{roleID}, nil)
	if err := deps.Service.DeleteRole(context.Background(), roleID); err == nil {
		t.Fatal("删除仍有用户绑定的角色应当被拒绝")
	}

	// 无绑定的新角色可删除
	freeCode := "rf" + suffix
	if err := deps.Service.CreateRole(context.Background(),
		dto.CreateRoleReq{Code: freeCode, Name: "待删", DataScope: 4}); err != nil {
		t.Fatalf("创建待删角色失败: %v", err)
	}
	roles, _ = deps.Service.ListRoles(context.Background())
	var freeID int64
	for _, r := range roles {
		if r.Code == freeCode {
			freeID = r.ID
		}
	}
	if err := deps.Service.DeleteRole(context.Background(), freeID); err != nil {
		t.Fatalf("删除无绑定角色失败: %v", err)
	}
}

// ---- 用例 2：权限分配与权限树 ----

func TestPermissionAssignmentAndTree(t *testing.T) {
	gdb, deps, suffix := setup(t)
	code := "rp" + suffix
	if err := deps.Service.CreateRole(context.Background(),
		dto.CreateRoleReq{Code: code, Name: "授权角色", DataScope: 4}); err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	roles, _ := deps.Service.ListRoles(context.Background())
	var roleID int64
	for _, r := range roles {
		if r.Code == code {
			roleID = r.ID
		}
	}

	// 非法权限 ID → 拒绝
	if err := deps.Service.AssignPermissions(context.Background(), roleID,
		dto.AssignPermsReq{PermissionIDs: []int64{99999999}}); err == nil {
		t.Fatal("分配不存在的权限应当被拒绝")
	}

	// 取一个种子权限（iam:user）
	tree, err := deps.Service.PermissionTree(context.Background())
	if err != nil {
		t.Fatalf("权限树查询失败: %v", err)
	}
	if len(tree) == 0 {
		t.Fatal("权限树不应为空")
	}
	var permID int64
	var walk func(nodes []dto.PermissionVO)
	walk = func(nodes []dto.PermissionVO) {
		for _, n := range nodes {
			if n.Code == "iam:user" {
				permID = n.ID
			}
			walk(n.Children)
		}
	}
	walk(tree)
	if permID == 0 {
		t.Fatal("种子权限 iam:user 不存在")
	}
	// 种子权限当前是平铺结构；树构建的正确性以「节点不丢」为准。
	var permCount int64
	gdb.Model(&iamModel.SysPermission{}).Count(&permCount)
	var flatten func(nodes []dto.PermissionVO) int
	flatten = func(nodes []dto.PermissionVO) int {
		n := len(nodes)
		for _, node := range nodes {
			n += flatten(node.Children)
		}
		return n
	}
	if int64(flatten(tree)) != permCount {
		t.Fatalf("权限树丢节点: 树=%d 表=%d", flatten(tree), permCount)
	}

	if err := deps.Service.AssignPermissions(context.Background(), roleID,
		dto.AssignPermsReq{PermissionIDs: []int64{permID}}); err != nil {
		t.Fatalf("分配权限失败: %v", err)
	}
	detail, err := deps.Service.GetRoleDetail(context.Background(), roleID)
	if err != nil {
		t.Fatalf("角色详情失败: %v", err)
	}
	if len(detail.PermissionIDs) != 1 || detail.PermissionIDs[0] != permID {
		t.Fatalf("权限替换结果不正确: %+v", detail.PermissionIDs)
	}
}

// ---- 用例 3：用户软删除 ----

func TestUserSoftDelete(t *testing.T) {
	gdb, deps, suffix := setup(t)
	username := createTestUser(t, deps, suffix, nil, nil)

	var userID int64
	row := struct{ ID int64 }{}
	gdb.Raw("SELECT id FROM sys_user WHERE username = ?", username).Scan(&row)
	userID = row.ID
	if userID == 0 {
		t.Fatal("找不到测试用户")
	}

	// 删自己 / 删种子 admin 一律拒绝（用 scope=All 的管理员 ctx）
	adminCtx := auth.WithDataScope(auth.WithUserID(context.Background(), 1), 1)
	if err := deps.Service.DeleteUser(adminCtx, 1); err == nil {
		t.Fatal("删除自己/admin 应当被拒绝")
	}

	if err := deps.Service.DeleteUser(context.Background(), userID); err != nil {
		t.Fatalf("软删除失败: %v", err)
	}
	if _, err := deps.Service.Login(context.Background(),
		dto.LoginReq{Username: username, Password: "Passw0rd!"}); appErrorCode(err) != 10001 {
		t.Fatalf("软删后登录期望 10001，得到 %d (%v)", appErrorCode(err), err)
	}

	// 同名可重建（部分唯一索引生效）
	if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username, Password: "Passw0rd!", RealName: "重建用户", Status: 1,
	}); err != nil {
		t.Fatalf("同名重建失败: %v", err)
	}

	// 列表中只出现存活用户一条
	users, total, err := deps.Service.ListUsers(adminCtx, dto.UserListReq{Username: username, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("用户列表失败: %v", err)
	}
	if total != 1 || len(users) != 1 {
		t.Fatalf("软删用户不应出现在列表，total=%d len=%d", total, len(users))
	}

	// 删除不存在的用户 → 404
	if err := deps.Service.DeleteUser(context.Background(), userID); appErrorCode(err) != 10100 {
		t.Fatalf("重复删除期望 10100，得到 %d", appErrorCode(err))
	}
}

// ---- 用例 4：管理员重置密码 ----

func TestResetPassword(t *testing.T) {
	_, deps, suffix := setup(t)
	username := createTestUser(t, deps, suffix, nil, nil)

	// 空密码 → 服务端生成 12 位随机密码并一次性返回
	res, err := deps.Service.ResetPassword(context.Background(), mustUserID(t, deps, username),
		dto.ResetPasswordReq{})
	if err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	if len(res.Password) != 12 {
		t.Fatalf("随机密码长度期望 12，得到 %d (%q)", len(res.Password), res.Password)
	}
	if _, err := deps.Service.Login(context.Background(),
		dto.LoginReq{Username: username, Password: res.Password}); err != nil {
		t.Fatalf("随机密码应当可登录: %v", err)
	}
	if _, err := deps.Service.Login(context.Background(),
		dto.LoginReq{Username: username, Password: "Passw0rd!"}); err == nil {
		t.Fatal("旧密码应当登录失败")
	}

	// 显式新密码与旧密码相同 → 拒绝
	createAgain := "itx" + suffix
	_ = deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: createAgain, Password: "Passw0rd!", RealName: "x", Status: 1,
	})
	if _, err := deps.Service.ResetPassword(context.Background(), mustUserID(t, deps, createAgain),
		dto.ResetPasswordReq{Password: "Passw0rd!"}); err == nil {
		t.Fatal("重置为相同密码应当被拒绝")
	}
}

// ---- 用例 5：强制改密 ----

func TestForceChangePassword(t *testing.T) {
	_, deps, suffix := setup(t)
	username := "fc" + suffix
	if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username, Password: "Passw0rd!", RealName: "强制改密",
		MustChangePwd: true, Status: 1,
	}); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	uid := mustUserID(t, deps, username)
	if !deps.Service.MustChangePassword(context.Background(), uid) {
		t.Fatal("must_change_pwd 用户应命中强制改密标记")
	}
	login, err := deps.Service.Login(context.Background(),
		dto.LoginReq{Username: username, Password: "Passw0rd!"})
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if !login.MustChangePwd {
		t.Fatal("登录响应应携带 must_change_pwd=true")
	}
	if err := deps.Service.ChangePassword(context.Background(), uid, dto.ChangePasswordReq{
		OldPassword: "Passw0rd!", NewPassword: "N3wP@ssw0rd",
	}); err != nil {
		t.Fatalf("改密失败: %v", err)
	}
	if deps.Service.MustChangePassword(context.Background(), uid) {
		t.Fatal("改密后强制标记应清除")
	}
}

// ---- 用例 6：改密/重置后旧 token 立即失效 ----

func TestTokenInvalidatedAfterPasswordChange(t *testing.T) {
	_, deps, suffix := setup(t)
	username := "inv" + suffix
	if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username, Password: "Passw0rd!", RealName: "失效测试", Status: 1,
	}); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	_, login := loginAndCtx(t, deps, username, "Passw0rd!")
	oldClaims, err := auth.ParseToken(testsupport.IAMJWTSecret, login.AccessToken)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond) // pwd:chg 是秒级时间戳，跨过签发那一秒
	if _, err := deps.Service.ResetPassword(context.Background(),
		mustUserID(t, deps, username), dto.ResetPasswordReq{Password: "R3set!Pass1"}); err != nil {
		t.Fatalf("重置密码失败: %v", err)
	}
	if deps.Service.CheckClaims(context.Background(), oldClaims) {
		t.Fatal("重置密码前签发的 token 应当失效")
	}

	// 新密码登录得到的新 token 有效
	_, newLogin := loginAndCtx(t, deps, username, "R3set!Pass1")
	newClaims, _ := auth.ParseToken(testsupport.IAMJWTSecret, newLogin.AccessToken)
	if !deps.Service.CheckClaims(context.Background(), newClaims) {
		t.Fatal("重置后新签发的 token 应当有效")
	}
}

// ---- 用例 7：连续失败锁定 ----

func TestLoginLockout(t *testing.T) {
	gdb, deps, suffix := setup(t)
	username := "lock" + suffix
	if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username, Password: "Passw0rd!", RealName: "锁定测试", Status: 1,
	}); err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	// 前 3 次坏密码 → 10001；第 3 次后达到验证码阈值，第 4/5 次无验证码直接 10008
	// （失败计数照样累加），第 5 次累加时写锁定键。
	for i := 0; i < 3; i++ {
		_, err := deps.Service.Login(context.Background(),
			dto.LoginReq{Username: username, Password: "wrong-pwd"})
		if appErrorCode(err) != 10001 {
			t.Fatalf("第 %d 次坏密码期望 10001，得到 %d", i+1, appErrorCode(err))
		}
	}
	for i := 0; i < 2; i++ {
		_, err := deps.Service.Login(context.Background(),
			dto.LoginReq{Username: username, Password: "wrong-pwd"})
		if appErrorCode(err) != 10008 {
			t.Fatalf("第 %d 次无验证码期望 10008，得到 %d", i+4, appErrorCode(err))
		}
	}
	// 第 6 次即使密码正确也锁定 → 10007
	_, err := deps.Service.Login(context.Background(),
		dto.LoginReq{Username: username, Password: "Passw0rd!"})
	if appErrorCode(err) != 10007 {
		t.Fatalf("锁定后期望 10007，得到 %d (%v)", appErrorCode(err), err)
	}

	var failCount int64
	gdb.Model(&iamModel.SysLoginAttempt{}).Where("username = ? AND success = ?", username, false).Count(&failCount)
	if failCount != 5 {
		t.Fatalf("sys_login_attempt 期望 5 行失败记录，得到 %d", failCount)
	}
}

// ---- 用例 8：验证码门控与一次性 ----

func TestCaptchaGate(t *testing.T) {
	_, deps, suffix := setup(t)

	// run 用独立 IP 跑一个用户的场景，避免 IP 维度计数跨场景叠加导致提前锁定。
	run := func(name, clientIP string, gateAttempt func() int) {
		t.Helper()
		username := "cap" + suffix + name
		if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
			Username: username, Password: "Passw0rd!", RealName: "验证码测试", Status: 1,
		}); err != nil {
			t.Fatalf("创建用户失败: %v", err)
		}
		ctx := auth.WithClientIP(context.Background(), clientIP)
		// 3 次坏密码达到验证码阈值
		for i := 0; i < 3; i++ {
			_, err := deps.Service.Login(ctx, dto.LoginReq{Username: username, Password: "bad"})
			if code := appErrorCode(err); code != 10001 {
				t.Fatalf("第 %d 次坏密码期望 10001，得到 %d", i+1, code)
			}
		}
		if got := gateAttempt(); got != 10008 {
			t.Fatalf("%s: 门控期望 10008，得到 %d", name, got)
		}

		// 新验证码 + 正确密码 → 登录成功，且验证码一次性消费
		fresh, err := deps.Service.GetCaptcha(ctx)
		if err != nil {
			t.Fatal(err)
		}
		freshCode, _ := deps.KV.Get(ctx, "captcha:"+fresh.CaptchaID)
		if freshCode == "" {
			t.Fatal("验证码答案未写入 KV")
		}
		if _, err := deps.Service.Login(ctx, dto.LoginReq{
			Username: username, Password: "Passw0rd!",
			CaptchaID: fresh.CaptchaID, CaptchaCode: freshCode,
		}); err != nil {
			t.Fatalf("%s: 验证码通过后登录应成功: %v", name, err)
		}
		if v, _ := deps.KV.Get(ctx, "captcha:"+fresh.CaptchaID); v != "" {
			t.Fatalf("%s: 验证码用后应当即删除", name)
		}
	}

	// 场景 A：达到阈值后不带验证码
	run("a", "10.10.0.1", func() int {
		ctx := auth.WithClientIP(context.Background(), "10.10.0.1")
		u := "cap" + suffix + "a"
		_, err := deps.Service.Login(ctx, dto.LoginReq{Username: u, Password: "Passw0rd!"})
		return appErrorCode(err)
	})

	// 场景 B：错误/已消费的验证码
	run("b", "10.10.0.2", func() int {
		ctx := auth.WithClientIP(context.Background(), "10.10.0.2")
		u := "cap" + suffix + "b"
		_, err := deps.Service.Login(ctx, dto.LoginReq{
			Username: u, Password: "Passw0rd!", CaptchaID: "not-exist", CaptchaCode: "0000",
		})
		return appErrorCode(err)
	})
}

// ---- 用例 9：5 档数据权限可见性矩阵 ----

func TestDataScopeVisibility(t *testing.T) {
	_, deps, suffix := setup(t)

	// 部门树：HQ → A、B
	mkDept := func(code string, parent *int64) int64 {
		req := dto.DepartmentReq{Code: "DS" + code + suffix, Name: code, ParentID: parent}
		if err := deps.Service.CreateDepartment(context.Background(), req); err != nil {
			t.Fatalf("创建部门失败: %v", err)
		}
		tree, err := deps.Service.DepartmentTree(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		var find func(nodes []dto.DepartmentVO) int64
		find = func(nodes []dto.DepartmentVO) int64 {
			for _, n := range nodes {
				if n.Code == req.Code {
					return n.ID
				}
				if id := find(n.Children); id != 0 {
					return id
				}
			}
			return 0
		}
		id := find(tree)
		if id == 0 {
			t.Fatalf("部门 %s 未找到", code)
		}
		return id
	}
	hqID := mkDept("H", nil)
	aID := mkDept("A", i64(hqID))
	bID := mkDept("B", i64(hqID))

	// 五个档位角色 + 五个观察者（均挂 HQ），再在 HQ/A/B 各放一个目标用户
	mkRole := func(scope int) int64 {
		code := fmt.Sprintf("rs%d%s", scope, suffix)
		if err := deps.Service.CreateRole(context.Background(),
			dto.CreateRoleReq{Code: code, Name: code, DataScope: scope}); err != nil {
			t.Fatal(err)
		}
		roles, _ := deps.Service.ListRoles(context.Background())
		for _, r := range roles {
			if r.Code == code {
				return r.ID
			}
		}
		return 0
	}
	roleAll, roleDept, roleTree, roleSelf, roleCustom := mkRole(1), mkRole(2), mkRole(3), mkRole(4), mkRole(5)
	if err := deps.Service.AssignRoleDepts(context.Background(), roleCustom,
		dto.AssignRoleDeptsReq{DeptIDs: []int64{bID}}); err != nil {
		t.Fatalf("自定义部门分配失败: %v", err)
	}

	vAll := createTestUser(t, deps, suffix+"a1", []int64{roleAll}, i64(hqID))
	vDept := createTestUser(t, deps, suffix+"a2", []int64{roleDept}, i64(hqID))
	vTree := createTestUser(t, deps, suffix+"a3", []int64{roleTree}, i64(hqID))
	vSelf := createTestUser(t, deps, suffix+"a4", []int64{roleSelf}, i64(hqID))
	vCustom := createTestUser(t, deps, suffix+"a5", []int64{roleCustom}, i64(hqID))
	uHQ := createTestUser(t, deps, suffix+"th", nil, i64(hqID))
	uA := createTestUser(t, deps, suffix+"ta", nil, i64(aID))
	uB := createTestUser(t, deps, suffix+"tb", nil, i64(bID))

	ctxAll, _ := loginAndCtx(t, deps, vAll, "Passw0rd!")
	ctxDept, _ := loginAndCtx(t, deps, vDept, "Passw0rd!")
	ctxTree, _ := loginAndCtx(t, deps, vTree, "Passw0rd!")
	ctxSelf, _ := loginAndCtx(t, deps, vSelf, "Passw0rd!")
	ctxCustom, _ := loginAndCtx(t, deps, vCustom, "Passw0rd!")

	list := func(ctx context.Context) ([]dto.UserVO, int64) {
		users, total, err := deps.Service.ListUsers(ctx, dto.UserListReq{Page: 1, PageSize: 500})
		if err != nil {
			t.Fatalf("列表查询失败: %v", err)
		}
		return users, total
	}
	contains := func(users []dto.UserVO, username string) bool {
		for _, u := range users {
			if u.Username == username {
				return true
			}
		}
		return false
	}

	// scope1 全部：三个目标都可见
	users, _ := list(ctxAll)
	for _, name := range []string{uHQ, uA, uB} {
		if !contains(users, name) {
			t.Fatalf("scope1 应可见 %s", name)
		}
	}

	// scope2 本部门：仅 HQ（uHQ），不见 A/B
	users, _ = list(ctxDept)
	if !contains(users, uHQ) || contains(users, uA) || contains(users, uB) {
		t.Fatalf("scope2 可见性错误: hq=%v a=%v b=%v", contains(users, uHQ), contains(users, uA), contains(users, uB))
	}

	// scope3 本部门及下级：三个都可见
	users, _ = list(ctxTree)
	if !contains(users, uHQ) || !contains(users, uA) || !contains(users, uB) {
		t.Fatal("scope3 应覆盖 HQ/A/B 全部用户")
	}

	// scope4 仅本人
	users, total := list(ctxSelf)
	if total != 1 || len(users) != 1 || users[0].Username != vSelf {
		t.Fatalf("scope4 应只见自己，total=%d", total)
	}
	// scope4 直接访问他人 → 10004
	if _, err := deps.Service.GetUser(ctxSelf, mustUserID(t, deps, uB)); appErrorCode(err) != 10004 {
		t.Fatalf("scope4 访问他人期望 10004，得到 %d", appErrorCode(err))
	}

	// scope5 自定义（仅 B）：只见 uB
	users, _ = list(ctxCustom)
	if !contains(users, uB) || contains(users, uA) || contains(users, uHQ) {
		t.Fatal("scope5 应只可见 B 部门用户")
	}
}

// ---- 用例 10：部门删除守卫 ----

func TestDepartmentGuards(t *testing.T) {
	_, deps, suffix := setup(t)
	parentCode := "DG" + suffix
	childCode := "DC" + suffix
	if err := deps.Service.CreateDepartment(context.Background(),
		dto.DepartmentReq{Code: parentCode, Name: "父部门"}); err != nil {
		t.Fatal(err)
	}
	pid := deptIDByCode(t, deps, parentCode)
	if err := deps.Service.CreateDepartment(context.Background(),
		dto.DepartmentReq{Code: childCode, Name: "子部门", ParentID: i64(pid)}); err != nil {
		t.Fatal(err)
	}
	cid := deptIDByCode(t, deps, childCode)

	// 不存在的父部门 → 拒绝
	if err := deps.Service.CreateDepartment(context.Background(),
		dto.DepartmentReq{Code: "DX" + suffix, Name: "悬空", ParentID: i64(99999999)}); err == nil {
		t.Fatal("挂到不存在的父部门应当被拒绝")
	}
	// 有子部门 → 409
	if err := deps.Service.DeleteDepartment(context.Background(), pid); appErrorCode(err) != 10006 {
		t.Fatalf("有子部门时期望 10006，得到 %d", appErrorCode(err))
	}
	// 部门下有用户 → 409
	createTestUser(t, deps, suffix+"dg", nil, i64(cid))
	if err := deps.Service.DeleteDepartment(context.Background(), cid); appErrorCode(err) != 10006 {
		t.Fatalf("有用户时期望 10006，得到 %d", appErrorCode(err))
	}
	// 不存在的部门 → 404
	if err := deps.Service.DeleteDepartment(context.Background(), 99999999); appErrorCode(err) != 10100 {
		t.Fatalf("删除不存在部门期望 10100，得到 %d", appErrorCode(err))
	}
}

// ---- 用例 11：审计登录留痕与查询 ----

func TestAuditLoginLogs(t *testing.T) {
	_, deps, suffix := setup(t)
	username := "aud" + suffix
	if err := deps.Service.CreateUser(context.Background(), dto.CreateUserReq{
		Username: username, Password: "Passw0rd!", RealName: "审计测试", Status: 1,
	}); err != nil {
		t.Fatal(err)
	}
	deps.Service.Login(context.Background(), dto.LoginReq{Username: username, Password: "wrong"})
	deps.Service.Login(context.Background(), dto.LoginReq{Username: username, Password: "Passw0rd!"})

	// 成功留痕
	logs, total, err := deps.Service.ListAuditLogs(context.Background(), dto.AuditLogListReq{
		Action: "login_success", Module: "auth",
		StartTime: time.Now().Add(-time.Minute).Format(time.RFC3339),
		EndTime:   time.Now().Add(time.Minute).Format(time.RFC3339),
		Page:      1, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("审计查询失败: %v", err)
	}
	found := false
	for _, l := range logs {
		if l.Status == 200 {
			found = true
		}
	}
	if !found || total == 0 {
		t.Fatal("未查到 login_success 审计记录")
	}

	// 失败留痕
	logs, _, err = deps.Service.ListAuditLogs(context.Background(), dto.AuditLogListReq{
		Action: "login_failed", Page: 1, PageSize: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) == 0 || logs[0].Status != 401 {
		t.Fatalf("失败审计状态期望 401，得到 %+v", logs)
	}

	// 时间区间倒置 → 参数错误
	_, _, err = deps.Service.ListAuditLogs(context.Background(), dto.AuditLogListReq{
		StartTime: time.Now().Format(time.RFC3339),
		EndTime:   time.Now().Add(-time.Hour).Format(time.RFC3339),
		Page:      1, PageSize: 50,
	})
	if appErrorCode(err) != 10005 {
		t.Fatalf("倒置时间区间期望 10005，得到 %d", appErrorCode(err))
	}
}

// ---- helpers ----

func mustUserID(t *testing.T, deps testsupport.IAMDeps, username string) int64 {
	t.Helper()
	users, _, err := deps.Service.ListUsers(
		auth.WithDataScope(auth.WithUserID(context.Background(), 1), 1),
		dto.UserListReq{Username: username, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("查用户失败: %v", err)
	}
	for _, u := range users {
		if u.Username == username {
			return u.ID
		}
	}
	t.Fatalf("用户 %s 不存在", username)
	return 0
}

func deptIDByCode(t *testing.T, deps testsupport.IAMDeps, code string) int64 {
	t.Helper()
	tree, err := deps.Service.DepartmentTree(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var walk func(nodes []dto.DepartmentVO) int64
	walk = func(nodes []dto.DepartmentVO) int64 {
		for _, n := range nodes {
			if n.Code == code {
				return n.ID
			}
			if id := walk(n.Children); id != 0 {
				return id
			}
		}
		return 0
	}
	id := walk(tree)
	if id == 0 {
		t.Fatalf("部门 %s 不存在", code)
	}
	return id
}
