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
	"erp-system/internal/pkg/redis"
	"erp-system/internal/routes"

	iamHandler "erp-system/internal/iam/handler"
	iamService "erp-system/internal/iam/service"
	iamRepository "erp-system/internal/iam/repository"

	mdmHandler "erp-system/internal/mdm/handler"
	mdmService "erp-system/internal/mdm/service"
	mdmRepository "erp-system/internal/mdm/repository"

	warehouseHandler "erp-system/internal/warehouse/handler"
	warehouseService "erp-system/internal/warehouse/service"
	warehouseRepository "erp-system/internal/warehouse/repository"

	purchaseHandler "erp-system/internal/purchase/handler"
	purchaseService "erp-system/internal/purchase/service"
	purchaseRepository "erp-system/internal/purchase/repository"

	productionHandler "erp-system/internal/production/handler"
	productionService "erp-system/internal/production/service"
	productionRepository "erp-system/internal/production/repository"

	financeHandler "erp-system/internal/finance/handler"
	financeService "erp-system/internal/finance/service"
	financeRepository "erp-system/internal/finance/repository"

	dashboardHandler "erp-system/internal/dashboard/handler"
	dashboardService "erp-system/internal/dashboard/service"

	deviceHandler "erp-system/internal/device/handler"
	deviceService "erp-system/internal/device/service"
	deviceRepository "erp-system/internal/device/repository"

	llmHandler "erp-system/internal/llm/handler"
	llmService "erp-system/internal/llm/service"
	llmRepository "erp-system/internal/llm/repository"
	"erp-system/internal/llm/gateway"

	openapiHandler "erp-system/internal/openapi/handler"
	openapiService "erp-system/internal/openapi/service"
	openapiRepository "erp-system/internal/openapi/repository"
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

	// MDM模块
	mdmMaterialRepo := mdmRepository.NewMaterialRepository(dbConn)
	mdmSupplierRepo := mdmRepository.NewSupplierRepository(dbConn)
	mdmWarehouseRepo := mdmRepository.NewWarehouseRepository(dbConn)
	mdmLocationRepo := mdmRepository.NewLocationRepository(dbConn)
	mdmSvc := mdmService.NewMDMService(mdmMaterialRepo, mdmSupplierRepo, mdmWarehouseRepo, mdmLocationRepo)
	mdmH := mdmHandler.NewMDMHandler(mdmSvc)

	// 公共基础设施：单号生成器、幂等键守卫
	numberGen := bizno.NewGenerator(rdb)
	idempGuard := idemp.NewGuard(rdb, 24*time.Hour)

	// 仓库模块
	warehouseInvRepo := warehouseRepository.NewInventoryRepository(dbConn)
	warehouseLedgerRepo := warehouseRepository.NewStockLedgerRepository(dbConn)
	warehouseSvc := warehouseService.NewWarehouseService(
		txManager, warehouseInvRepo, warehouseLedgerRepo,
		mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		numberGen, idempGuard,
	)
	warehouseH := warehouseHandler.NewWarehouseHandler(warehouseSvc)

	// 采购模块
	purchaseOrderRepo := purchaseRepository.NewPurchaseOrderRepository(dbConn)
	purchaseInboundRepo := purchaseRepository.NewPurchaseInboundRepository(dbConn)
	purchaseSvc := purchaseService.NewPurchaseService(
		txManager, purchaseOrderRepo, purchaseInboundRepo,
		mdmSupplierRepo, mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		warehouseSvc, numberGen, idempGuard,
	)
	purchaseH := purchaseHandler.NewPurchaseHandler(purchaseSvc)

	// 生产模块
	productionWorkOrderRepo := productionRepository.NewWorkOrderRepository(dbConn)
	productionBomRepo := productionRepository.NewBomRepository(dbConn)
	productionSvc := productionService.NewProductionService(
		txManager, productionWorkOrderRepo, productionBomRepo,
		mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		warehouseSvc, numberGen, idempGuard,
	)
	productionH := productionHandler.NewProductionHandler(productionSvc)

	// 财务模块
	financeCostCardRepo := financeRepository.NewCostCardRepository(dbConn)
	financeAccountEntryRepo := financeRepository.NewAccountEntryRepository(dbConn)
	financeBudgetRepo := financeRepository.NewBudgetRepository(dbConn)
	financeSvc := financeService.NewFinanceService(txManager, financeCostCardRepo, financeAccountEntryRepo, financeBudgetRepo, mdmMaterialRepo)
	financeH := financeHandler.NewFinanceHandler(financeSvc)

	// 仪表模块
	dashboardSvc := dashboardService.NewDashboardService(txManager, dbConn)
	dashboardH := dashboardHandler.NewDashboardHandler(dashboardSvc)

	// 设备模块
	deviceRepo := deviceRepository.NewDeviceRepository(dbConn)
	deviceSvc := deviceService.NewDeviceService(deviceRepo)
	deviceH := deviceHandler.NewDeviceHandler(deviceSvc)

	// LLM模块
	llmSessionRepo := llmRepository.NewSessionRepository(dbConn)
	llmMessageRepo := llmRepository.NewMessageRepository(dbConn)
	llmGateway := gateway.NewQwenGateway(viper.GetString("llm.api_key"), viper.GetString("llm.model"))
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

	routes.RegisterRoutes(r, routes.Deps{
		JWTSecret:  jwtSecret,
		IAM:        iamH,
		MDM:        mdmH,
		Warehouse:  warehouseH,
		Purchase:   purchaseH,
		Production: productionH,
		Finance:    financeH,
		Dashboard:  dashboardH,
		Device:     deviceH,
		LLM:        llmH,
		OpenAPI:    openapiH,
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
