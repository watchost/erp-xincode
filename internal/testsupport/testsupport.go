// Copyright 2026 zhouhouping. All Rights Reserved.

// Package testsupport 为 service 层集成测试提供真实 Postgres 环境：
// 自动连接测试库、执行内嵌迁移、清理业务表、构造主数据夹具与装配 service。
//
// 连接串由 TEST_DATABASE_DSN 指定（pgx 关键字 DSN），默认指向本地 erp_it 库。
// 数据库不可达时测试 t.Skip 而非失败，保证无 DB 环境下 `go test ./...` 仍可通过。
package testsupport

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	mdmModel "erp-system/internal/mdm/model"
	mdmRepo "erp-system/internal/mdm/repository"
	"erp-system/internal/pkg/auth"
	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	migratetool "erp-system/internal/pkg/migrate"

	financeRepo "erp-system/internal/finance/repository"
	financeService "erp-system/internal/finance/service"
	purchaseRepo "erp-system/internal/purchase/repository"
	purchaseService "erp-system/internal/purchase/service"
	salesRepo "erp-system/internal/sales/repository"
	salesService "erp-system/internal/sales/service"
	warehouseRepo "erp-system/internal/warehouse/repository"
	warehouseService "erp-system/internal/warehouse/service"
)

// businessTables 是每个用例前需要清空的全部业务表（CASCADE 处理外键）。
var businessTables = []string{
	"inv_stock_ledger",
	"inv_inventory",
	"pur_purchase_inbound",
	"pur_purchase_order_item",
	"pur_purchase_order",
	"sal_sales_return_item",
	"sal_sales_return",
	"sal_sales_outbound_item",
	"sal_sales_outbound",
	"sal_sales_order_item",
	"sal_sales_order",
	"fin_cost",
	"fin_voucher_entry",
	"fin_voucher",
	"fin_budget",
	"dev_device_message",
	"dev_device",
	"llm_message",
	"llm_session",
	"prod_production_receipt",
	"prod_production_outbound",
	"prod_work_order_bom",
	"prod_work_order",
	"prod_bom_item",
	"prod_bom",
	"mdm_location",
	"mdm_warehouse",
	"mdm_supplier",
	"mdm_material",
	"mdm_customer",
	"mdm_material_category",
}

var (
	migrateOnce sync.Once
	migrateErr  error
)

// baseDSN 返回不指定 dbname 的连接参数（主机/账号），用于自动建库。
func baseDSN(dbname string) string {
	host := envOr("TEST_DB_HOST", envOr("DB_HOST", "localhost"))
	port := envOr("TEST_DB_PORT", envOr("DB_PORT", "5432"))
	user := envOr("TEST_DB_USER", envOr("DB_USER", "erp_user"))
	pass := os.Getenv("TEST_DB_PASSWORD") // 不回退到服务口令，避免误用
	if pass == "" {
		pass = "erp_password_zhouhouping_2026"
	}
	sslmode := envOr("TEST_DB_SSLMODE", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, pass, dbname, sslmode)
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// packageDBName 根据调用方测试文件路径生成「每包一个」的隔离库名，
// 例如 internal/warehouse/service → erp_it_warehouse_service。
// go test 默认并行执行不同包，共享库会让 TRUNCATE 互相破坏夹具。
func packageDBName() string {
	pc := make([]uintptr, 16)
	n := runtime.Callers(0, pc)
	frames := runtime.CallersFrames(pc[:n])
	for {
		frame, more := frames.Next()
		if strings.HasSuffix(frame.File, "_test.go") && strings.Contains(frame.File, "/internal/") {
			rel := frame.File[strings.Index(frame.File, "/internal/")+len("/internal/"):]
			parts := strings.Split(path.Dir(rel), "/")
			if len(parts) >= 2 {
				return "erp_it_" + parts[len(parts)-2] + "_" + parts[len(parts)-1]
			}
			return "erp_it_" + parts[len(parts)-1]
		}
		if !more {
			return "erp_it_unknown"
		}
	}
}

var (
	provMu   sync.Mutex
	provErrs = map[string]error{}
)

// provisionOnce 保证每个测试进程只重建一次自己的隔离库。
func provisionOnce(name string) error {
	provMu.Lock()
	defer provMu.Unlock()
	if err, ok := provErrs[name]; ok {
		return err
	}
	err := provisionDB(name)
	provErrs[name] = err
	return err
}

// provisionDB 删除并重建指定测试库后执行全部迁移。
func provisionDB(name string) error {
	// 连维护库执行 CREATE/DROP DATABASE（不能在事务中，也不能连在目标库上）。
	admin, err := gorm.Open(postgres.Open(baseDSN("postgres")), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("连接维护库失败: %w", err)
	}
	sqlAdmin, _ := admin.DB()
	defer sqlAdmin.Close()

	if err := admin.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = ? AND pid <> pg_backend_pid()", name).Error; err != nil {
		return err
	}
	if err := admin.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)).Error; err != nil {
		return err
	}
	if err := admin.Exec(fmt.Sprintf("CREATE DATABASE %s", name)).Error; err != nil {
		return err
	}
	sqlAdmin.Close()
	return migratetool.Up(baseDSN(name))
}

// NewDB 连接（必要时自动创建）测试库并确保迁移为最新；数据库不可达时 Skip。
//   - 设置 TEST_DATABASE_DSN 时直接使用该库（CI 可为每个包提供独立库）；
//   - 否则自动创建「每包一个」的 erp_it_<module>_<layer> 隔离库。
//
// 返回的 *gorm.DB 在用例结束后自动关闭。
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		name := packageDBName()
		if err := provisionOnce(name); err != nil {
			t.Skipf("测试数据库不可用，跳过集成测试: %v", err)
		}
		dsn = baseDSN(name)
	} else {
		migrateOnce.Do(func() { migrateErr = migratetool.Up(dsn) })
		if migrateErr != nil {
			t.Skipf("测试数据库不可用，跳过集成测试: %v", migrateErr)
		}
	}

	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skipf("连接测试数据库失败，跳过集成测试: %v", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		t.Skipf("获取底层连接池失败: %v", err)
	}
	// 显式限制连接数，确保并发用例里 FOR UPDATE 真正发生竞争而非串行排队。
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	t.Cleanup(func() { _ = sqlDB.Close() })
	Reset(t, gdb)
	return gdb
}

// Reset 清空全部业务表并重置序列。
func Reset(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	for _, table := range businessTables {
		if err := gdb.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", table)).Error; err != nil {
			t.Fatalf("清理表 %s 失败: %v", table, err)
		}
	}
}

// Fixture 聚合一条主链路需要的主数据。
type Fixture struct {
	Material  *mdmModel.MdmMaterial
	Supplier  *mdmModel.MdmSupplier
	Warehouse *mdmModel.MdmWarehouse
	Location  *mdmModel.MdmLocation
}

// CreateFixture 写入物料/供应商/仓库/库位，codeSuffix 保证并行用例编码唯一。
func CreateFixture(t *testing.T, gdb *gorm.DB, codeSuffix string) Fixture {
	t.Helper()
	mat := &mdmModel.MdmMaterial{
		MaterialCode: "MAT-IT-" + codeSuffix,
		Name:         "集成测试物料-" + codeSuffix,
		Unit:         "个",
		CostMethod:   1,
		Status:       1,
	}
	if err := gdb.Create(mat).Error; err != nil {
		t.Fatalf("创建物料夹具失败: %v", err)
	}
	sup := &mdmModel.MdmSupplier{SupplierCode: "SUP-IT-" + codeSuffix, Name: "集成测试供应商", Level: 1}
	if err := gdb.Create(sup).Error; err != nil {
		t.Fatalf("创建供应商夹具失败: %v", err)
	}
	wh := &mdmModel.MdmWarehouse{Code: "WH-IT-" + codeSuffix, Name: "集成测试仓", Type: 1}
	if err := gdb.Create(wh).Error; err != nil {
		t.Fatalf("创建仓库夹具失败: %v", err)
	}
	loc := &mdmModel.MdmLocation{WarehouseID: wh.ID, LocationCode: "A-01-" + codeSuffix, Zone: "A"}
	if err := gdb.Create(loc).Error; err != nil {
		t.Fatalf("创建库位夹具失败: %v", err)
	}
	return Fixture{Material: mat, Supplier: sup, Warehouse: wh, Location: loc}
}

// CreateCustomer 写入客户主数据夹具（creditLimit=0 表示不限额）。
func CreateCustomer(t *testing.T, gdb *gorm.DB, codeSuffix string, creditLimit decimal.Decimal) *mdmModel.MdmCustomer {
	t.Helper()
	c := &mdmModel.MdmCustomer{
		CustomerCode: "CUS-IT-" + codeSuffix,
		Name:         "集成测试客户-" + codeSuffix,
		CreditLimit:  creditLimit,
		Level:        1,
		Status:       1,
	}
	if err := gdb.Create(c).Error; err != nil {
		t.Fatalf("创建客户夹具失败: %v", err)
	}
	return c
}

// CreateWarehouse 写入仓库夹具。
func CreateWarehouse(t *testing.T, gdb *gorm.DB, code, name string) *mdmModel.MdmWarehouse {
	t.Helper()
	w := &mdmModel.MdmWarehouse{Code: code, Name: name, Type: 1}
	if err := gdb.Create(w).Error; err != nil {
		t.Fatalf("创建仓库夹具失败: %v", err)
	}
	return w
}

// CreateLocation 写入库位夹具。
func CreateLocation(t *testing.T, gdb *gorm.DB, warehouseID int64, locationCode, zone string) *mdmModel.MdmLocation {
	t.Helper()
	loc := &mdmModel.MdmLocation{WarehouseID: warehouseID, LocationCode: locationCode, Zone: zone}
	if err := gdb.Create(loc).Error; err != nil {
		t.Fatalf("创建库位夹具失败: %v", err)
	}
	return loc
}

// Ctx 返回带用户 ID 的上下文（单据 CreatedBy 取登录用户）。
func Ctx() context.Context {
	return context.Background()
}

// CtxWithUser 注入用户 ID（auth.WithUserID 的测试快捷方式）。
func CtxWithUser(userID int64) context.Context {
	return auth.WithUserID(context.Background(), userID)
}

// NewWarehouseService 装配仓库 service（Redis 用 nil：单号走进程内兜底，幂等守卫关闭）。
func NewWarehouseService(gdb *gorm.DB) *warehouseService.WarehouseService {
	svc := warehouseService.NewWarehouseService(
		db.NewTxManager(gdb),
		warehouseRepo.NewInventoryRepository(gdb),
		warehouseRepo.NewStockLedgerRepository(gdb),
		mdmRepo.NewMaterialRepository(gdb),
		mdmRepo.NewWarehouseRepository(gdb),
		mdmRepo.NewLocationRepository(gdb),
		bizno.NewGenerator(nil),
		nil,
	)
	svc.SetStockCheckRepository(warehouseRepo.NewStockCheckRepository(gdb))
	svc.SetTransferRepository(warehouseRepo.NewTransferRepository(gdb))
	return svc
}

// NewFinanceService 装配财务 service。
func NewFinanceService(gdb *gorm.DB) *financeService.FinanceService {
	return financeService.NewFinanceService(
		db.NewTxManager(gdb),
		financeRepo.NewCostCardRepository(gdb),
		financeRepo.NewAccountEntryRepository(gdb),
		financeRepo.NewVoucherRepository(gdb),
		financeRepo.NewAccountRepository(gdb),
		financeRepo.NewBudgetRepository(gdb),
		mdmRepo.NewMaterialRepository(gdb),
		bizno.NewGenerator(nil),
	)
}

// FinanceGateway 是采购 service 财务网关的公开镜像（私有接口不能跨包命名），
// 供测试注入故障桩。
type FinanceGateway interface {
	financeService.Poster
	ReserveBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error
	ReleaseBudgetTx(ctx context.Context, tx *gorm.DB, budgetType string, amount decimal.Decimal) error
}

// D 把十进制字符串字面量转成 decimal.Decimal，供测试用例精确构造金额/数量
// （禁止用 float64 字面量引入二进制误差）。入参非法时 panic，符合测试夹具语义。
func D(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// PurchaseDeps 聚合采购 service 与其依赖，便于用例直接查库断言。
type PurchaseDeps struct {
	Service   *purchaseService.PurchaseService
	OrderRepo purchaseRepo.PurchaseOrderRepository
}

// IdempotencyGuard 是采购/仓库 service 幂等守卫参数的公开镜像。
type IdempotencyGuard interface {
	Acquire(ctx context.Context, scope, key string) error
}

// SalesDeps 聚合销售 service 与其依赖，便于用例直接查库断言。
type SalesDeps struct {
	Service      *salesService.SalesService
	OrderRepo    salesRepo.SalesOrderRepository
	OutboundRepo salesRepo.SalesOutboundRepository
	ReturnRepo   salesRepo.SalesReturnRepository
}

// NewSalesService 装配销售 service，过账用真实 FinanceService。
func NewSalesService(gdb *gorm.DB, wh *warehouseService.WarehouseService, fin *financeService.FinanceService) SalesDeps {
	return NewSalesServiceWith(gdb, wh, fin, nil)
}

// NewSalesServiceWith 允许注入自定义过账器（故障桩）与幂等守卫。
func NewSalesServiceWith(
	gdb *gorm.DB,
	wh *warehouseService.WarehouseService,
	fin financeService.Poster,
	guard IdempotencyGuard,
) SalesDeps {
	orderRepo := salesRepo.NewSalesOrderRepository(gdb)
	outboundRepo := salesRepo.NewSalesOutboundRepository(gdb)
	returnRepo := salesRepo.NewSalesReturnRepository(gdb)
	return SalesDeps{
		Service: salesService.NewSalesService(
			db.NewTxManager(gdb),
			orderRepo,
			outboundRepo,
			returnRepo,
			mdmRepo.NewCustomerRepository(gdb),
			mdmRepo.NewMaterialRepository(gdb),
			mdmRepo.NewWarehouseRepository(gdb),
			mdmRepo.NewLocationRepository(gdb),
			wh,
			fin,
			bizno.NewGenerator(nil),
			guard,
		),
		OrderRepo:    orderRepo,
		OutboundRepo: outboundRepo,
		ReturnRepo:   returnRepo,
	}
}

// NewPurchaseService 装配采购 service，财务网关用真实 FinanceService。
func NewPurchaseService(gdb *gorm.DB, wh *warehouseService.WarehouseService, fin *financeService.FinanceService) PurchaseDeps {
	return NewPurchaseServiceWith(gdb, wh, fin, nil)
}

// NewPurchaseServiceWith 允许注入自定义财务网关（故障桩）与幂等守卫。
func NewPurchaseServiceWith(
	gdb *gorm.DB,
	wh *warehouseService.WarehouseService,
	fin FinanceGateway,
	guard IdempotencyGuard,
) PurchaseDeps {
	orderRepo := purchaseRepo.NewPurchaseOrderRepository(gdb)
	return PurchaseDeps{
		Service: purchaseService.NewPurchaseService(
			db.NewTxManager(gdb),
			orderRepo,
			purchaseRepo.NewPurchaseInboundRepository(gdb),
			mdmRepo.NewSupplierRepository(gdb),
			mdmRepo.NewMaterialRepository(gdb),
			mdmRepo.NewWarehouseRepository(gdb),
			mdmRepo.NewLocationRepository(gdb),
			wh,
			fin,
			bizno.NewGenerator(nil),
			guard,
		),
		OrderRepo: orderRepo,
	}
}
