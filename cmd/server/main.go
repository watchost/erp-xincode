// Copyright 2026 zhouhouping. All Rights Reserved.

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"erp-system/internal/pkg/bizno"
	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/idemp"
	"erp-system/internal/pkg/logger"
	"erp-system/internal/pkg/middleware"
	migratetool "erp-system/internal/pkg/migrate"
	"erp-system/internal/pkg/redis"
	"erp-system/internal/routes"

	iamHandler "erp-system/internal/iam/handler"
	iamRepository "erp-system/internal/iam/repository"
	iamService "erp-system/internal/iam/service"

	mdmHandler "erp-system/internal/mdm/handler"
	mdmRepository "erp-system/internal/mdm/repository"
	mdmService "erp-system/internal/mdm/service"

	warehouseHandler "erp-system/internal/warehouse/handler"
	warehouseRepository "erp-system/internal/warehouse/repository"
	warehouseService "erp-system/internal/warehouse/service"

	purchaseHandler "erp-system/internal/purchase/handler"
	purchaseRepository "erp-system/internal/purchase/repository"
	purchaseService "erp-system/internal/purchase/service"

	productionHandler "erp-system/internal/production/handler"
	productionRepository "erp-system/internal/production/repository"
	productionService "erp-system/internal/production/service"

	financeHandler "erp-system/internal/finance/handler"
	financeRepository "erp-system/internal/finance/repository"
	financeService "erp-system/internal/finance/service"

	dashboardHandler "erp-system/internal/dashboard/handler"
	dashboardService "erp-system/internal/dashboard/service"

	deviceHandler "erp-system/internal/device/handler"
	deviceRepository "erp-system/internal/device/repository"
	deviceService "erp-system/internal/device/service"

	"erp-system/internal/llm/gateway"
	llmHandler "erp-system/internal/llm/handler"
	llmRepository "erp-system/internal/llm/repository"
	llmService "erp-system/internal/llm/service"

	openapiHandler "erp-system/internal/openapi/handler"
	openapiRepository "erp-system/internal/openapi/repository"
	openapiService "erp-system/internal/openapi/service"
)

// @title ERP System API
// @version 1.0
// @description ERP系统API接口文档
// @contact.name zhouhouping
// @contact.email zhouhouping@example.com
// @license.name Copyright 2026 zhouhouping. All Rights Reserved.

func main() {
	initConfig()
	logger.Init()

	dbConn := db.Init()

	// 可选：启动时自动执行数据库迁移（MIGRATE_AUTO=true）。
	// 默认关闭，生产环境建议显式执行 `erp-migrate up`（compose 中为一次性迁移任务）。
	if viper.GetBool("migrate.auto") {
		dsn := fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			envOr("DB_HOST", "localhost"),
			envOr("DB_PORT", "5432"),
			envOr("DB_USER", "erp_user"),
			envOr("DB_PASSWORD", "erp_password_zhouhouping_2026"),
			envOr("DB_NAME", "erp_system"),
			envOr("DB_SSLMODE", "disable"),
		)
		if err := migratetool.Up(dsn); err != nil {
			log.Fatalf("auto migrate failed: %v", err)
		}
		log.Println("database migrations applied (MIGRATE_AUTO=true)")
	}

	rdb := redis.Init()
	txManager := db.NewTxManager(dbConn)

	// IAM模块
	iamUserRepo := iamRepository.NewUserRepository(dbConn)
	iamRoleRepo := iamRepository.NewRoleRepository(dbConn)
	iamPermRepo := iamRepository.NewPermissionRepository(dbConn)
	iamUserRoleRepo := iamRepository.NewUserRoleRepository(dbConn)
	iamAuditLogRepo := iamRepository.NewAuditLogRepository(dbConn)

	jwtSecret := viper.GetString("jwt.secret")
	if jwtSecret == "" || len(jwtSecret) < 16 {
		log.Fatalf("jwt.secret 必须配置且长度 >= 16")
	}
	accessExpire := viper.GetDuration("jwt.access_token_expire")
	if accessExpire == 0 {
		accessExpire = 30 * time.Minute
	}
	refreshExpire := viper.GetDuration("jwt.refresh_token_expire")
	if refreshExpire == 0 {
		refreshExpire = 7 * 24 * time.Hour
	}

	iamSvc := iamService.NewIAMService(
		dbConn, txManager,
		iamUserRepo, iamRoleRepo, iamPermRepo, iamUserRoleRepo, iamAuditLogRepo,
		jwtSecret, accessExpire, refreshExpire, rdb,
	)
	iamH := iamHandler.NewIAMHandler(iamSvc, accessExpire)

	// MDM模块：仅仓储（repo）层先构建；service 依赖仓库服务做删除引用守卫，故后建。
	mdmMaterialRepo := mdmRepository.NewMaterialRepository(dbConn)
	mdmSupplierRepo := mdmRepository.NewSupplierRepository(dbConn)
	mdmWarehouseRepo := mdmRepository.NewWarehouseRepository(dbConn)
	mdmLocationRepo := mdmRepository.NewLocationRepository(dbConn)

	// 公共基础设施：单号生成器、幂等键守卫
	numberGen := bizno.NewGenerator(rdb)
	idempGuard := idemp.NewGuard(rdb, 24*time.Hour)

	// 仓库模块：先于 MDM/采购/生产构建——MDM 删除守卫、采购入库、生产领退料都
	// 在各自事务内调用 warehouseService 的同事务库存方法。
	warehouseInvRepo := warehouseRepository.NewInventoryRepository(dbConn)
	warehouseLedgerRepo := warehouseRepository.NewStockLedgerRepository(dbConn)
	warehouseSvc := warehouseService.NewWarehouseService(
		txManager, warehouseInvRepo, warehouseLedgerRepo,
		mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		numberGen, idempGuard,
	)
	warehouseH := warehouseHandler.NewWarehouseHandler(warehouseSvc)

	// 财务模块：采购入库/生产领退料需在同事务内过账，故先于采购/生产构建，
	// 作为 financeService.Poster 注入下游业务服务。
	financeCostCardRepo := financeRepository.NewCostCardRepository(dbConn)
	financeAccountEntryRepo := financeRepository.NewAccountEntryRepository(dbConn)
	financeBudgetRepo := financeRepository.NewBudgetRepository(dbConn)
	financeSvc := financeService.NewFinanceService(
		txManager, financeCostCardRepo, financeAccountEntryRepo, financeBudgetRepo,
		mdmMaterialRepo, numberGen,
	)
	financeH := financeHandler.NewFinanceHandler(financeSvc)

	// MDM service：warehouseSvc 作为 InventoryChecker 注入，删除主数据前做库存引用守卫。
	mdmSvc := mdmService.NewMDMService(mdmMaterialRepo, mdmSupplierRepo, mdmWarehouseRepo, mdmLocationRepo, warehouseSvc)
	mdmH := mdmHandler.NewMDMHandler(mdmSvc)

	// 采购模块
	purchaseOrderRepo := purchaseRepository.NewPurchaseOrderRepository(dbConn)
	purchaseInboundRepo := purchaseRepository.NewPurchaseInboundRepository(dbConn)
	purchaseSvc := purchaseService.NewPurchaseService(
		txManager, purchaseOrderRepo, purchaseInboundRepo,
		mdmSupplierRepo, mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		warehouseSvc, financeSvc, numberGen, idempGuard,
	)
	purchaseH := purchaseHandler.NewPurchaseHandler(purchaseSvc)

	// 生产模块
	productionWorkOrderRepo := productionRepository.NewWorkOrderRepository(dbConn)
	productionReceiptRepo := productionRepository.NewProductionReceiptRepository(dbConn)
	productionBomRepo := productionRepository.NewBomRepository(dbConn)
	productionSvc := productionService.NewProductionService(
		txManager, productionWorkOrderRepo, productionReceiptRepo, productionBomRepo,
		mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		warehouseSvc, financeSvc, numberGen, idempGuard,
	)
	productionH := productionHandler.NewProductionHandler(productionSvc)

	// LLM 网关（仪表盘智能分析与 LLM 会话共用；未配置 api_key 时仪表盘回退为本地数据摘要）
	llmGateway := gateway.NewQwenGateway(viper.GetString("llm.api_key"), viper.GetString("llm.model"))
	llmEnabled := viper.GetString("llm.api_key") != ""

	// 仪表模块
	dashboardSvc := dashboardService.NewDashboardService(txManager, dbConn, llmGateway, llmEnabled)
	dashboardH := dashboardHandler.NewDashboardHandler(dashboardSvc)

	// 设备模块
	deviceRepo := deviceRepository.NewDeviceRepository(dbConn)
	deviceSvc := deviceService.NewDeviceService(deviceRepo)
	deviceH := deviceHandler.NewDeviceHandler(deviceSvc)

	// LLM模块
	llmSessionRepo := llmRepository.NewSessionRepository(dbConn)
	llmMessageRepo := llmRepository.NewMessageRepository(dbConn)
	llmSvc := llmService.NewLLMService(llmSessionRepo, llmMessageRepo, llmGateway)
	llmH := llmHandler.NewLLMHandler(llmSvc)

	// OpenAPI模块
	openapiClientRepo := openapiRepository.NewClientRepository(dbConn)
	openapiTokenRepo := openapiRepository.NewTokenRepository(dbConn)
	openapiWebhookRepo := openapiRepository.NewWebhookRepository(dbConn)
	openapiSyncLogRepo := openapiRepository.NewSyncLogRepository(dbConn)
	openapiSvc := openapiService.NewOpenAPIService(openapiClientRepo, openapiTokenRepo, openapiWebhookRepo, openapiSyncLogRepo)
	openapiH := openapiHandler.NewOpenAPIHandler(openapiSvc)

	// 路由注册
	mode := viper.GetString("server.mode")
	if mode != "" {
		gin.SetMode(mode)
	}
	r := gin.New()
	r.Use(gin.Logger(), middleware.Recovery(), middleware.TraceID())
	if origins := viper.GetStringSlice("cors.allow_origins"); len(origins) > 0 {
		r.Use(middleware.CORS(origins))
	}
	if reqTimeout := viper.GetDuration("server.timeout"); reqTimeout > 0 {
		r.Use(middleware.Timeout(reqTimeout))
	}

	// 健康检查（公开，不走 JWT）：
	//   /healthz 存活探针——进程能响应即 200；
	//   /readyz  就绪探针——探测 DB 与 Redis，任一不可用返回 503。
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := gin.H{}
		ready := true
		if sqlDB, err := dbConn.DB(); err != nil {
			ready = false
			checks["db"] = "fail: " + err.Error()
		} else if err := sqlDB.PingContext(ctx); err != nil {
			ready = false
			checks["db"] = "fail: " + err.Error()
		} else {
			checks["db"] = "ok"
		}
		if err := rdb.Client().Ping(ctx).Err(); err != nil {
			ready = false
			checks["redis"] = "fail: " + err.Error()
		} else {
			checks["redis"] = "ok"
		}

		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "checks": checks})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
	})

	routes.RegisterRoutes(r, routes.Deps{
		JWTSecret:    jwtSecret,
		JWTBlacklist: iamSvc.IsBlacklisted,
		LoginLimit:   middleware.LoginRateLimit(rdb, 10, time.Minute),
		IAM:          iamH,
		MDM:          mdmH,
		Warehouse:    warehouseH,
		Purchase:     purchaseH,
		Production:   productionH,
		Finance:      financeH,
		Dashboard:    dashboardH,
		Device:       deviceH,
		LLM:          llmH,
		OpenAPI:      openapiH,
	})

	host := viper.GetString("server.host")
	port := viper.GetString("server.port")
	if port == "" {
		port = "8080"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       viper.GetDuration("server.read_timeout"),
		WriteTimeout:      viper.GetDuration("server.write_timeout"),
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		fmt.Printf("ERP System starting on %s...\n", addr)
		fmt.Println("Copyright 2026 zhouhouping. All Rights Reserved.")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, draining...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	log.Println("server exited")
}

func envOr(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")

	// 环境变量覆盖：JWT_SECRET / DB_HOST / SERVER_PORT 等
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Failed to read config file: %v", err)
	}
}
