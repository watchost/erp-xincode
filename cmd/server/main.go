// Copyright 2026 zhouhouping. All Rights Reserved.

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"erp-system/internal/pkg/db"
	"erp-system/internal/pkg/redis"
	"erp-system/internal/pkg/logger"
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
	iamSvc := iamService.NewIAMService(iamUserRepo, iamRoleRepo, iamPermRepo, iamUserRoleRepo, iamAuditLogRepo, jwtSecret, 2*time.Hour, 24*time.Hour, rdb)
	iamH := iamHandler.NewIAMHandler(iamSvc, jwtSecret)

	// MDM模块
	mdmMaterialRepo := mdmRepository.NewMaterialRepository(dbConn)
	mdmSupplierRepo := mdmRepository.NewSupplierRepository(dbConn)
	mdmWarehouseRepo := mdmRepository.NewWarehouseRepository(dbConn)
	mdmLocationRepo := mdmRepository.NewLocationRepository(dbConn)
	mdmSvc := mdmService.NewMDMService(mdmMaterialRepo, mdmSupplierRepo, mdmWarehouseRepo, mdmLocationRepo)
	mdmH := mdmHandler.NewMDMHandler(mdmSvc)

	// 仓库模块
	warehouseInvRepo := warehouseRepository.NewInventoryRepository(dbConn)
	warehouseLedgerRepo := warehouseRepository.NewStockLedgerRepository(dbConn)
	warehouseSvc := warehouseService.NewWarehouseService(txManager, warehouseInvRepo, warehouseLedgerRepo, mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo)
	warehouseH := warehouseHandler.NewWarehouseHandler(warehouseSvc)

	// 采购模块
	purchaseOrderRepo := purchaseRepository.NewPurchaseOrderRepository(dbConn)
	purchaseInboundRepo := purchaseRepository.NewPurchaseInboundRepository(dbConn)
	purchaseSvc := purchaseService.NewPurchaseService(txManager, purchaseOrderRepo, purchaseInboundRepo, mdmSupplierRepo, mdmMaterialRepo, warehouseSvc)
	purchaseH := purchaseHandler.NewPurchaseHandler(purchaseSvc)

	// 生产模块
	productionWorkOrderRepo := productionRepository.NewWorkOrderRepository(dbConn)
	productionBomRepo := productionRepository.NewBomRepository(dbConn)
	productionSvc := productionService.NewProductionService(txManager, productionWorkOrderRepo, productionBomRepo, mdmMaterialRepo, warehouseSvc)
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
	llmGateway := gateway.NewQwenGateway("", "qwen-turbo")
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
	r := gin.Default()
	routes.RegisterRoutes(r, iamH, mdmH, warehouseH, purchaseH, productionH, financeH, dashboardH, deviceH, llmH, openapiH)

	port := viper.GetString("server.port")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("ERP System starting on :%s...\n", port)
	fmt.Println("Copyright 2026 zhouhouping. All Rights Reserved.")
	log.Fatal(r.Run(":" + port))
}

func initConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Printf("Warning: Failed to read config: %v", err)
	}
}
