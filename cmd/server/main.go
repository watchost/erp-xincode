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
	"erp-system/internal/pkg/decimalx"
	"erp-system/internal/pkg/idemp"
	"erp-system/internal/pkg/logger"
	"erp-system/internal/pkg/metrics"
	"erp-system/internal/pkg/middleware"
	migratetool "erp-system/internal/pkg/migrate"
	"erp-system/internal/pkg/printer"
	"erp-system/internal/pkg/redis"
	"erp-system/internal/routes"

	iamHandler "erp-system/internal/iam/handler"
	iamModel "erp-system/internal/iam/model"
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
	salesHandler "erp-system/internal/sales/handler"
	salesRepository "erp-system/internal/sales/repository"
	salesService "erp-system/internal/sales/service"

	productionHandler "erp-system/internal/production/handler"
	productionRepository "erp-system/internal/production/repository"
	productionService "erp-system/internal/production/service"

	financeHandler "erp-system/internal/finance/handler"
	financeRepository "erp-system/internal/finance/repository"
	financeService "erp-system/internal/finance/service"

	dashboardHandler "erp-system/internal/dashboard/handler"
	dashboardService "erp-system/internal/dashboard/service"

	deviceGateway "erp-system/internal/device/gateway"
	deviceHandler "erp-system/internal/device/handler"
	deviceRepository "erp-system/internal/device/repository"
	deviceService "erp-system/internal/device/service"

	"erp-system/internal/llm/gateway"
	llmHandler "erp-system/internal/llm/handler"
	llmRepository "erp-system/internal/llm/repository"
	llmService "erp-system/internal/llm/service"

	openapiHandler "erp-system/internal/openapi/handler"
	openapiMiddleware "erp-system/internal/openapi/middleware"
	openapiRepository "erp-system/internal/openapi/repository"
	openapiService "erp-system/internal/openapi/service"
	openapiWorkerPkg "erp-system/internal/openapi/worker"
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
	// 注册 decimal 入参校验标签（decgt/decgte/declte），必须在首个 HTTP 请求绑定前完成。
	decimalx.InitGin()

	dbConn := db.Init()
	if sqlDB, err := dbConn.DB(); err == nil {
		metrics.RegisterDBStats(sqlDB)
	}

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
	iamDeptRepo := iamRepository.NewDepartmentRepository(dbConn)
	iamAuditLogRepo := iamRepository.NewAuditLogRepository(dbConn)
	iamAttemptRepo := iamRepository.NewLoginAttemptRepository(dbConn)

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
		iamUserRepo, iamRoleRepo, iamPermRepo, iamUserRoleRepo,
		iamDeptRepo, iamAuditLogRepo, iamAttemptRepo,
		jwtSecret, accessExpire, refreshExpire, rdb,
	)
	iamH := iamHandler.NewIAMHandler(iamSvc, accessExpire)

	// MDM模块：仅仓储（repo）层先构建；service 依赖仓库服务做删除引用守卫，故后建。
	mdmMaterialRepo := mdmRepository.NewMaterialRepository(dbConn)
	mdmSupplierRepo := mdmRepository.NewSupplierRepository(dbConn)
	mdmWarehouseRepo := mdmRepository.NewWarehouseRepository(dbConn)
	mdmLocationRepo := mdmRepository.NewLocationRepository(dbConn)
	mdmCategoryRepo := mdmRepository.NewCategoryRepository(dbConn)
	mdmCustomerRepo := mdmRepository.NewCustomerRepository(dbConn)

	// 公共基础设施：单号生成器、幂等键守卫
	numberGen := bizno.NewGenerator(rdb)
	idempGuard := idemp.NewGuard(rdb, 24*time.Hour)

	// 仓库模块：先于 MDM/采购/生产构建——MDM 删除守卫、采购入库、生产领退料都
	// 在各自事务内调用 warehouseService 的同事务库存方法。
	warehouseInvRepo := warehouseRepository.NewInventoryRepository(dbConn)
	warehouseLedgerRepo := warehouseRepository.NewStockLedgerRepository(dbConn)
	warehouseCheckRepo := warehouseRepository.NewStockCheckRepository(dbConn)
	warehouseTransferRepo := warehouseRepository.NewTransferRepository(dbConn)
	warehouseBatchRepo := warehouseRepository.NewBatchRepository(dbConn)
	warehouseSerialRepo := warehouseRepository.NewSerialRepository(dbConn)
	warehouseSvc := warehouseService.NewWarehouseService(
		txManager, warehouseInvRepo, warehouseLedgerRepo,
		mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		numberGen, idempGuard,
	)
	warehouseSvc.SetStockCheckRepository(warehouseCheckRepo)
	warehouseSvc.SetTransferRepository(warehouseTransferRepo)
	warehouseSvc.SetBatchRepository(warehouseBatchRepo)
	warehouseSvc.SetSerialRepository(warehouseSerialRepo)
	warehouseH := warehouseHandler.NewWarehouseHandler(warehouseSvc)

	// 财务模块：采购入库/生产领退料需在同事务内过账，故先于采购/生产构建，
	// 作为 financeService.Poster 注入下游业务服务。
	financeCostCardRepo := financeRepository.NewCostCardRepository(dbConn)
	financeAccountEntryRepo := financeRepository.NewAccountEntryRepository(dbConn)
	financeBudgetRepo := financeRepository.NewBudgetRepository(dbConn)
	financeVoucherRepo := financeRepository.NewVoucherRepository(dbConn)
	financeAccountRepo := financeRepository.NewAccountRepository(dbConn)
	financeRecvRepo := financeRepository.NewReceivableRepository(dbConn)
	financePayRepo := financeRepository.NewPayableRepository(dbConn)
	financeReceiptRepo := financeRepository.NewReceiptRepository(dbConn)
	financePaymentRepo := financeRepository.NewPaymentRepository(dbConn)
	financeSettleRepo := financeRepository.NewSettlementRepository(dbConn)
	financeSvc := financeService.NewFinanceService(
		txManager, financeCostCardRepo, financeAccountEntryRepo, financeVoucherRepo, financeAccountRepo, financeBudgetRepo,
		mdmMaterialRepo, numberGen,
	)
	financeSvc.SetReceivableRepository(financeRecvRepo)
	financeSvc.SetPayableRepository(financePayRepo)
	financeSvc.SetReceiptRepository(financeReceiptRepo)
	financeSvc.SetPaymentRepository(financePaymentRepo)
	financeSvc.SetSettlementRepository(financeSettleRepo)
	financeSvc.SetCustomerRepository(mdmCustomerRepo)
	financeSvc.SetSupplierRepository(mdmSupplierRepo)
	financeH := financeHandler.NewFinanceHandler(financeSvc)
	warehouseSvc.SetFinance(financeSvc)

	// MDM service：warehouseSvc 作为 InventoryChecker 注入，删除主数据前做库存引用守卫。
	mdmSvc := mdmService.NewMDMService(mdmMaterialRepo, mdmSupplierRepo, mdmWarehouseRepo, mdmLocationRepo, mdmCategoryRepo, mdmCustomerRepo, warehouseSvc)
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
	// 采购单/工单列表套用 IAM 5 档数据权限；单据 dept_id 创建时取自 claims。
	purchaseSvc.SetDataScopeResolver(iamSvc)

	// 销售模块（P2 §3.1）：订单/出库/退货，与采购对称。
	salesOrderRepo := salesRepository.NewSalesOrderRepository(dbConn)
	salesOutboundRepo := salesRepository.NewSalesOutboundRepository(dbConn)
	salesReturnRepo := salesRepository.NewSalesReturnRepository(dbConn)
	salesSvc := salesService.NewSalesService(
		txManager, salesOrderRepo, salesOutboundRepo, salesReturnRepo,
		mdmCustomerRepo, mdmMaterialRepo, mdmWarehouseRepo, mdmLocationRepo,
		warehouseSvc, financeSvc, numberGen, idempGuard,
	)
	salesH := salesHandler.NewSalesHandler(salesSvc)
	salesSvc.SetDataScopeResolver(iamSvc)

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
	productionSvc.SetDataScopeResolver(iamSvc)

	// LLM 网关（仪表盘智能分析与 LLM 会话共用；未配置 api_key 时仪表盘回退为本地数据摘要）
	llmGateway := gateway.NewQwenGateway(viper.GetString("llm.api_key"), viper.GetString("llm.model"))
	llmEnabled := viper.GetString("llm.api_key") != ""

	// 仪表模块
	dashboardSvc := dashboardService.NewDashboardService(txManager, dbConn, llmGateway, llmEnabled)
	dashboardH := dashboardHandler.NewDashboardHandler(dashboardSvc)

	// 设备模块
	deviceRepo := deviceRepository.NewDeviceRepository(dbConn)
	deviceMsgRepo := deviceRepository.NewDeviceMessageRepository(dbConn)
	deviceHub := deviceGateway.NewHub(deviceRepo, deviceMsgRepo)
	deviceSvc := deviceService.NewDeviceService(deviceRepo, deviceMsgRepo)
	deviceSvc.SetHub(deviceHub)
	deviceH := deviceHandler.NewDeviceHandler(deviceSvc)

	// LLM模块
	llmSessionRepo := llmRepository.NewSessionRepository(dbConn)
	llmMessageRepo := llmRepository.NewMessageRepository(dbConn)
	llmPromptRepo := llmRepository.NewPromptTemplateRepository(dbConn)
	llmSvc := llmService.NewLLMService(llmSessionRepo, llmMessageRepo, llmPromptRepo, llmGateway)
	llmH := llmHandler.NewLLMHandler(llmSvc)

	// OpenAPI模块
	openapiClientRepo := openapiRepository.NewClientRepository(dbConn)
	openapiTokenRepo := openapiRepository.NewTokenRepository(dbConn)
	openapiWebhookRepo := openapiRepository.NewWebhookRepository(dbConn)
	openapiDeliveryRepo := openapiRepository.NewWebhookDeliveryRepository(dbConn)
	openapiSyncLogRepo := openapiRepository.NewSyncLogRepository(dbConn)
	openapiSvc := openapiService.NewOpenAPIService(openapiClientRepo, openapiTokenRepo, openapiWebhookRepo, openapiDeliveryRepo, openapiSyncLogRepo)
	openapiResourceSvc := openapiService.NewResourceService(
		dbConn, txManager,
		purchaseOrderRepo,
		mdmSupplierRepo,
		mdmMaterialRepo,
		numberGen,
	)
	openapiH := openapiHandler.NewOpenAPIHandler(openapiSvc, openapiResourceSvc)
	// webhook 异步投递 worker（重试 + 死信）。
	openapiWorker := openapiWorkerPkg.NewWorker(openapiDeliveryRepo, openapiWebhookRepo)
	workerCtx, workerStop := context.WithCancel(context.Background())
	go openapiWorker.Run(workerCtx)

	// 路由注册
	mode := viper.GetString("server.mode")
	if mode != "" {
		gin.SetMode(mode)
	}
	r := gin.New()
	r.Use(gin.Logger(), middleware.Recovery(), metrics.GinMiddleware(), middleware.TraceID(), middleware.RequestMeta())
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

	// Prometheus 监控指标（公开抓取）与 pprof（内网调试）
	r.GET("/metrics", metrics.Handler())
	metrics.RegisterPProf(r)

	// 审计落库适配：middleware.AuditEntry → iam model（接口定义在 middleware 包，避免反向依赖）。
	auditSink := middleware.AuditSinkFunc(func(ctx context.Context, e middleware.AuditEntry) {
		entry := &iamModel.SysAuditLog{
			UserID:     e.UserID,
			TenantID:   e.TenantID,
			Module:     e.Module,
			Action:     e.Action,
			IP:         e.IP,
			UserAgent:  e.UserAgent,
			Status:     e.Status,
			DurationMs: e.DurationMs,
			TraceID:    e.TraceID,
			ReqParam:   iamModel.JSON(e.ReqParam),
		}
		if err := iamAuditLogRepo.Create(entry); err != nil {
			logger.Log(ctx).Warn("write audit log failed", "error", err)
		}
	})

	routes.RegisterRoutes(r, routes.Deps{
		JWTSecret:    jwtSecret,
		JWTBlacklist: iamSvc.CheckClaims,
		ForceChange:  iamSvc.MustChangePassword,
		Audit:        middleware.Audit(auditSink),
		LoginLimit:   middleware.LoginRateLimit(rdb, 10, time.Minute),
		IAM:          iamH,
		MDM:          mdmH,
		Warehouse:    warehouseH,
		Purchase:     purchaseH,
		Sales:        salesH,
		Production:   productionH,
		Finance:      financeH,
		Dashboard:    dashboardH,
		Device:       deviceH,
		LLM:          llmH,
		OpenAPI:      openapiH,
		Printer:      printer.NewPrinterHandler(),
		OpenAPIAuth:  openapiMiddleware.BearerAuth(openapiSvc.ValidateAccessToken),
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

	// 设备接入 WebSocket 网关：独立端口 + HMAC 签名鉴权（docs/development/features/device.md）。
	wsPort := viper.GetInt("device.websocket_port")
	heartbeatTimeout := viper.GetDuration("device.heartbeat_timeout")
	if heartbeatTimeout <= 0 {
		heartbeatTimeout = 60 * time.Second
	}
	wsSrv := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", host, wsPort),
		Handler:           http.HandlerFunc(deviceHub.HandleWS),
		ReadHeaderTimeout: 10 * time.Second,
	}
	wsStop := make(chan struct{})
	if wsPort > 0 {
		go func() {
			fmt.Printf("Device gateway (ws) starting on port %d...\n", wsPort)
			if err := wsSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("device gateway error: %v", err)
			}
		}()
		// 心跳超时清扫：每 10s 把超时无心跳的在线设备置为离线。
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if _, err := deviceHub.SweepOffline(heartbeatTimeout); err != nil {
						logger.Log(context.Background()).Warn("device heartbeat sweep failed", "error", err)
					}
				case <-wsStop:
					return
				}
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, draining...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	close(wsStop)
	workerStop()
	deviceHub.CloseAll()
	if wsPort > 0 {
		if err := wsSrv.Shutdown(ctx); err != nil {
			log.Printf("device gateway shutdown failed: %v", err)
		}
	}
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
