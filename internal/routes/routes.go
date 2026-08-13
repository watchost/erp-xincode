// Copyright 2026 zhouhouping. All Rights Reserved.

package routes

import (
	"github.com/gin-gonic/gin"

	iamHandler "erp-system/internal/iam/handler"
	mdmHandler "erp-system/internal/mdm/handler"
	warehouseHandler "erp-system/internal/warehouse/handler"
	purchaseHandler "erp-system/internal/purchase/handler"
	productionHandler "erp-system/internal/production/handler"
	financeHandler "erp-system/internal/finance/handler"
	dashboardHandler "erp-system/internal/dashboard/handler"
	deviceHandler "erp-system/internal/device/handler"
	llmHandler "erp-system/internal/llm/handler"
	openapiHandler "erp-system/internal/openapi/handler"
	"erp-system/internal/pkg/middleware"
)

// Permission codes used for route guards. Keep in sync with sys_permission seed.
const (
	PermDashboardView    = "dashboard:view"
	PermDashboardLLM     = "dashboard:llm"

	PermWarehouseInbound   = "warehouse:inbound"
	PermWarehouseOutbound  = "warehouse:outbound"
	PermWarehouseInventory = "warehouse:inventory"

	PermPurchaseOrderView    = "purchase:order:view"
	PermPurchaseOrderCreate  = "purchase:order:create"
	PermPurchaseOrderApprove = "purchase:order:approve"
	PermPurchaseInbound      = "purchase:inbound"

	PermProductionWOView   = "production:wo:view"
	PermProductionWOCreate = "production:wo:create"
	PermProductionOutbound = "production:outbound"

	PermFinanceCost    = "finance:cost"
	PermFinanceReports = "finance:reports"
	PermFinanceBudget  = "finance:budget"

	PermMDMMaterial  = "mdm:material"
	PermMDMSupplier  = "mdm:supplier"
	PermMDMWarehouse = "mdm:warehouse"
	PermMDMLocation  = "mdm:location"

	PermIAMUser = "iam:user"
	PermIAMRole = "iam:role"

	PermDeviceManage = "device:manage"
	PermLLMChat      = "llm:chat"
	PermOpenAPIAdmin = "openapi:admin"
)

// Deps bundles all HTTP handlers and infrastructure config needed for routing.
type Deps struct {
	JWTSecret   string
	IAM         *iamHandler.IAMHandler
	MDM         *mdmHandler.MDMHandler
	Warehouse   *warehouseHandler.WarehouseHandler
	Purchase    *purchaseHandler.PurchaseHandler
	Production  *productionHandler.ProductionHandler
	Finance     *financeHandler.FinanceHandler
	Dashboard   *dashboardHandler.DashboardHandler
	Device      *deviceHandler.DeviceHandler
	LLM         *llmHandler.LLMHandler
	OpenAPI     *openapiHandler.OpenAPIHandler
}

// RegisterRoutes wires all HTTP endpoints.
// - Public endpoints (login/refresh/oauth token) are outside the JWT group.
// - All /api/v1 business endpoints require a valid JWT.
// - Mutating/sensitive endpoints additionally require a permission code.
func RegisterRoutes(r *gin.Engine, d Deps) {
	// 公开接口：不需要登录
	r.POST("/api/v1/login", d.IAM.Login)
	r.POST("/api/v1/refresh", d.IAM.RefreshToken)

	// OpenAPI 对外网关：OAuth2 取 token 不走内部 JWT
	r.POST("/api/v1/oauth/token", d.OpenAPI.GetToken)
	r.POST("/api/v1/oauth/refresh", d.OpenAPI.RefreshToken)

	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(d.JWTSecret))
	{
		// 当前用户信息 / 登出 / 改密：只要登录即可
		api.GET("/users/profile", d.IAM.GetUserInfo)
		api.GET("/users/permissions", d.IAM.GetPermissions)
		api.POST("/auth/logout", d.IAM.Logout)
		api.POST("/auth/change-password", d.IAM.ChangePassword)

		// IAM 用户管理
		iamG := api.Group("/iam", middleware.RequirePermission(PermIAMUser))
		{
			iamG.GET("/users", d.IAM.ListUsers)
			iamG.GET("/users/:id", d.IAM.GetUser)
			iamG.POST("/users", d.IAM.CreateUser)
			iamG.PUT("/users/:id", d.IAM.UpdateUser)
		}

		mdm := api.Group("/mdm")
		{
			mdm.POST("/materials", middleware.RequirePermission(PermMDMMaterial), d.MDM.CreateMaterial)
			mdm.GET("/materials", middleware.RequirePermission(PermMDMMaterial), d.MDM.ListMaterials)
			mdm.GET("/materials/:id", middleware.RequirePermission(PermMDMMaterial), d.MDM.GetMaterial)
			mdm.PUT("/materials/:id", middleware.RequirePermission(PermMDMMaterial), d.MDM.UpdateMaterial)

			mdm.POST("/suppliers", middleware.RequirePermission(PermMDMSupplier), d.MDM.CreateSupplier)
			mdm.GET("/suppliers", middleware.RequirePermission(PermMDMSupplier), d.MDM.ListSuppliers)
			mdm.GET("/suppliers/:id", middleware.RequirePermission(PermMDMSupplier), d.MDM.GetSupplier)
			mdm.PUT("/suppliers/:id", middleware.RequirePermission(PermMDMSupplier), d.MDM.UpdateSupplier)

			mdm.POST("/warehouses", middleware.RequirePermission(PermMDMWarehouse), d.MDM.CreateWarehouse)
			mdm.GET("/warehouses", middleware.RequirePermission(PermMDMWarehouse), d.MDM.ListWarehouses)

			mdm.POST("/locations", middleware.RequirePermission(PermMDMLocation), d.MDM.CreateLocation)
			mdm.GET("/locations", middleware.RequirePermission(PermMDMLocation), d.MDM.ListLocations)
		}

		warehouse := api.Group("/warehouse")
		{
			warehouse.POST("/inbound/scan", middleware.RequirePermission(PermWarehouseInbound), d.Warehouse.InboundScan)
			warehouse.POST("/outbound/scan", middleware.RequirePermission(PermWarehouseOutbound), d.Warehouse.OutboundScan)
			warehouse.GET("/inventory", middleware.RequirePermission(PermWarehouseInventory), d.Warehouse.ListInventory)
			warehouse.GET("/stock-alerts", middleware.RequirePermission(PermWarehouseInventory), d.Warehouse.GetStockAlerts)
		}

		purchase := api.Group("/purchase")
		{
			purchase.GET("/orders", middleware.RequirePermission(PermPurchaseOrderView), d.Purchase.ListOrders)
			purchase.POST("/orders", middleware.RequirePermission(PermPurchaseOrderCreate), d.Purchase.CreateOrder)
			purchase.POST("/orders/:order_no/approve", middleware.RequirePermission(PermPurchaseOrderApprove), d.Purchase.ApproveOrder)
			purchase.POST("/inbound/scan", middleware.RequirePermission(PermPurchaseInbound), d.Purchase.InboundScan)
		}

		production := api.Group("/production")
		{
			production.GET("/work-orders", middleware.RequirePermission(PermProductionWOView), d.Production.ListWorkOrders)
			production.POST("/work-orders", middleware.RequirePermission(PermProductionWOCreate), d.Production.CreateWorkOrder)
			production.POST("/work-orders/:work_order_no/release", middleware.RequirePermission(PermProductionWOCreate), d.Production.ReleaseWorkOrder)
			production.POST("/material-issue/scan", middleware.RequirePermission(PermProductionOutbound), d.Production.MaterialIssueScan)
			production.POST("/bom", middleware.RequirePermission(PermProductionWOCreate), d.Production.CreateBom)
		}

		finance := api.Group("/finance")
		{
			finance.GET("/cost-cards", middleware.RequirePermission(PermFinanceCost), d.Finance.ListCostCards)
			finance.GET("/cost-summary", middleware.RequirePermission(PermFinanceCost), d.Finance.GetCostSummary)
			finance.GET("/account-entries", middleware.RequirePermission(PermFinanceReports), d.Finance.ListAccountEntries)
			finance.GET("/financial-report", middleware.RequirePermission(PermFinanceReports), d.Finance.GetFinancialReport)
			finance.GET("/budgets", middleware.RequirePermission(PermFinanceBudget), d.Finance.ListBudgets)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/overview", middleware.RequirePermission(PermDashboardView), d.Dashboard.GetOverview)
			dashboard.GET("/stock-alerts", middleware.RequirePermission(PermDashboardView), d.Dashboard.GetStockAlerts)
			dashboard.GET("/recent-orders", middleware.RequirePermission(PermDashboardView), d.Dashboard.GetRecentOrders)
			dashboard.POST("/llm/analysis", middleware.RequirePermission(PermDashboardLLM), d.Dashboard.GetLLMAnalysis)
		}

		device := api.Group("/device", middleware.RequirePermission(PermDeviceManage))
		{
			device.POST("/register", d.Device.Register)
			device.POST("/heartbeat", d.Device.Heartbeat)
			device.GET("/list", d.Device.List)
			device.GET("/:device_code", d.Device.GetByCode)
		}

		llm := api.Group("/llm", middleware.RequirePermission(PermLLMChat))
		{
			llm.POST("/chat", d.LLM.Chat)
			llm.GET("/sessions", d.LLM.ListSessions)
			llm.GET("/sessions/:session_id/history", d.LLM.GetHistory)
		}

		openapi := api.Group("/openapi", middleware.RequirePermission(PermOpenAPIAdmin))
		{
			openapi.POST("/webhooks", d.OpenAPI.CreateWebhook)
			openapi.POST("/sync", d.OpenAPI.Sync)
			openapi.GET("/clients", d.OpenAPI.ListClients)
		}
	}
}
