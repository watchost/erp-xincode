// Copyright 2026 zhouhouping. All Rights Reserved.

package tenant

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	"erp-system/internal/pkg/auth"
)

// 纯单元测试：不连数据库，直接构造 gorm.Statement 验证插件逻辑。

type tModel struct {
	ID       int64
	TenantID int64
	Name     string
}

func (tModel) TableName() string { return "tmodel" }

type tNoTenant struct {
	ID   int64
	Name string
}

func (tNoTenant) TableName() string { return "tmodel" }

func newPlugin() *Plugin { return &Plugin{tables: map[string]bool{"tmodel": true, "mdm_material": true}} }

func newDB(ctx context.Context) *gorm.DB {
	db := &gorm.DB{Config: &gorm.Config{}}
	db.Statement = &gorm.Statement{DB: db, Context: ctx, Clauses: map[string]clause.Clause{}}
	return db
}

func whereExprs(db *gorm.DB) []clause.Expression {
	c, ok := db.Statement.Clauses["WHERE"]
	if !ok {
		return nil
	}
	w, _ := c.Expression.(clause.Where)
	return w.Exprs
}

func TestRealTable(t *testing.T) {
	cases := []struct{ expr, real, ref string }{
		{"wf_task t", "wf_task", "t"},
		{`"sys_user" AS u`, "sys_user", "u"},
		{"mdm_material", "mdm_material", "mdm_material"},
	}
	for _, c := range cases {
		st := &gorm.Statement{TableExpr: &clause.Expr{SQL: c.expr}}
		real, ref := RealTable(st)
		if real != c.real || ref != c.ref {
			t.Errorf("%q：期望 (%s,%s)，实际 (%s,%s)", c.expr, c.real, c.ref, real, ref)
		}
	}
	if real, ref := RealTable(&gorm.Statement{Table: "mdm_material"}); real != "mdm_material" || ref != "mdm_material" {
		t.Errorf("Table 字段解析错误: %s %s", real, ref)
	}
}

func TestFilter_AddsTenantConditionWithAlias(t *testing.T) {
	p := newPlugin()
	db := newDB(auth.WithTenantID(context.Background(), 5))
	db.Statement.TableExpr = &clause.Expr{SQL: "mdm_material m"}
	db.Statement.Table = "m"
	p.filter(db)
	exprs := whereExprs(db)
	want := clause.Eq{Column: clause.Column{Table: "m", Name: "tenant_id"}, Value: int64(5)}
	if len(exprs) != 1 || !reflect.DeepEqual(exprs[0], want) {
		t.Fatalf("应追加 m.tenant_id = 5，实际 %#v", exprs)
	}
}

func TestFilter_NoOpCases(t *testing.T) {
	p := newPlugin()
	// 没有租户上下文
	db := newDB(context.Background())
	db.Statement.Table = "mdm_material"
	p.filter(db)
	if whereExprs(db) != nil {
		t.Error("没有租户上下文时不应过滤")
	}
	// 显式绕过
	db = newDB(auth.WithSkipTenant(auth.WithTenantID(context.Background(), 5)))
	db.Statement.Table = "mdm_material"
	p.filter(db)
	if whereExprs(db) != nil {
		t.Error("WithSkipTenant 时不应过滤")
	}
	// 不含 tenant_id 的表
	db = newDB(auth.WithTenantID(context.Background(), 5))
	db.Statement.Table = "sys_role"
	p.filter(db)
	if whereExprs(db) != nil {
		t.Error("未登记的表不应过滤")
	}
	// Raw SQL 已拼好：无法注入（已知限制）
	db = newDB(auth.WithTenantID(context.Background(), 5))
	db.Statement.Table = "mdm_material"
	db.Statement.SQL.WriteString("SELECT 1")
	p.filter(db)
	if whereExprs(db) != nil {
		t.Error("Raw SQL 不应被改写")
	}
}

func TestFilterWrite_RequiresExistingWhere(t *testing.T) {
	p := newPlugin()
	db := newDB(auth.WithTenantID(context.Background(), 5))
	db.Statement.Table = "mdm_material"
	p.filterWrite(db)
	if whereExprs(db) != nil {
		t.Fatal("没有 WHERE 的全表更新不应被插件补条件（交给 GORM 拦截）")
	}
	db.Statement.AddClause(clause.Where{Exprs: []clause.Expression{clause.Eq{Column: "id", Value: 1}}})
	p.filterWrite(db)
	if len(whereExprs(db)) != 2 {
		t.Fatalf("已有 WHERE 时应追加租户条件，实际 %#v", whereExprs(db))
	}
}

func parse(t *testing.T, v any) *schema.Schema {
	s, err := schema.Parse(v, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestStamp_OverridesTenantOnCreate(t *testing.T) {
	p := newPlugin()
	db := newDB(auth.WithTenantID(context.Background(), 7))
	db.Statement.Table = "tmodel"
	db.Statement.Schema = parse(t, &tModel{})
	m := tModel{Name: "a", TenantID: 99} // 调用方伪造租户
	db.Statement.ReflectValue = reflect.ValueOf(&m).Elem()
	p.stamp(db)
	if db.Error != nil || m.TenantID != 7 {
		t.Fatalf("创建时应强制写入上下文租户 7，实际 %d err=%v", m.TenantID, db.Error)
	}

	list := []tModel{{Name: "x"}, {Name: "y", TenantID: 3}}
	db = newDB(auth.WithTenantID(context.Background(), 8))
	db.Statement.Table = "tmodel"
	db.Statement.Schema = parse(t, &tModel{})
	db.Statement.ReflectValue = reflect.ValueOf(&list).Elem()
	p.stamp(db)
	if list[0].TenantID != 8 || list[1].TenantID != 8 {
		t.Fatalf("批量创建应全部写入租户 8，实际 %+v", list)
	}
}

func TestStamp_FailClosedWhenModelLacksTenantField(t *testing.T) {
	p := newPlugin()
	db := newDB(auth.WithTenantID(context.Background(), 7))
	db.Statement.Table = "tmodel"
	db.Statement.Schema = parse(t, &tNoTenant{})
	m := tNoTenant{Name: "a"}
	db.Statement.ReflectValue = reflect.ValueOf(&m).Elem()
	p.stamp(db)
	if db.Error == nil {
		t.Fatal("非 0 租户写入缺少 TenantID 字段的模型应报错（否则会以租户 0 落库）")
	}
	db = newDB(auth.WithTenantID(context.Background(), 0))
	db.Statement.Table = "tmodel"
	db.Statement.Schema = parse(t, &tNoTenant{})
	db.Statement.ReflectValue = reflect.ValueOf(&m).Elem()
	p.stamp(db)
	if db.Error != nil {
		t.Fatalf("租户 0 依赖列默认值，不应报错: %v", db.Error)
	}
}
