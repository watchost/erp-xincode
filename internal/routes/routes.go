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

func RegisterRoutes(r *gin.Engine,
	iamH *iamHandler.IAMHandler,
	mdmH *mdmHandler.MDMHandler,
	warehouseH *warehouseHandler.WarehouseHandler,
	purchaseH *purchaseHandler.PurchaseHandler,
	productionH *productionHandler.ProductionHandler,
	financeH *financeHandler.FinanceHandler,
	dashboardH *dashboardHandler.DashboardHandler,
	deviceH *deviceHandler.DeviceHandler,
	llmH *llmHandler.LLMHandler,
	openapiH *openapiHandler.OpenAPIHandler,
) {
	r.POST("/api/v1/login", iamH.Login)
	r.POST("/api/v1/refresh", iamH.RefreshToken)

	r.POST("/api/v1/oauth/token", openapiH.GetToken)
	r.POST("/api/v1/oauth/refresh", openapiH.RefreshToken)

	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth())
	{
		api.GET("/users/profile", iamH.GetUserInfo)
		api.GET("/users/permissions", iamH.GetPermissions)
		api.GET("/users", iamH.ListUsers)
		api.GET("/users/:id", iamH.GetUser)
		api.POST("/users", iamH.CreateUser)
		api.PUT("/users/:id", iamH.UpdateUser)

		mdm := api.Group("/mdm")
		{
			mdm.POST("/materials", mdmH.CreateMaterial)
			mdm.GET("/materials", mdmH.ListMaterials)
			mdm.GET("/materials/:id", mdmH.GetMaterial)
			mdm.PUT("/materials/:id", mdmH.UpdateMaterial)

			mdm.POST("/suppliers", mdmH.CreateSupplier)
			mdm.GET("/suppliers", mdmH.ListSuppliers)
			mdm.GET("/suppliers/:id", mdmH.GetSupplier)
			mdm.PUT("/suppliers/:id", mdmH.UpdateSupplier)

			mdm.POST("/warehouses", mdmH.CreateWarehouse)
			mdm.GET("/warehouses", mdmH.ListWarehouses)

			mdm.POST("/locations", mdmH.CreateLocation)
			mdm.GET("/locations", mdmH.ListLocations)
		}

		warehouse := api.Group("/warehouse")
		{
			warehouse.POST("/inbound/scan", warehouseH.InboundScan)
			warehouse.POST("/outbound/scan", warehouseH.OutboundScan)
			warehouse.GET("/inventory", warehouseH.ListInventory)
			warehouse.GET("/stock-alerts", warehouseH.GetStockAlerts)
		}

		purchase := api.Group("/purchase")
		{
			purchase.POST("/orders", purchaseH.CreateOrder)
			purchase.POST("/orders/:order_no/approve", purchaseH.ApproveOrder)
			purchase.GET("/orders", purchaseH.ListOrders)
			purchase.POST("/inbound/scan", purchaseH.InboundScan)
		}

		production := api.Group("/production")
		{
			production.POST("/work-orders", productionH.CreateWorkOrder)
			production.POST("/work-orders/:work_order_no/release", productionH.ReleaseWorkOrder)
			production.GET("/work-orders", productionH.ListWorkOrders)
			production.POST("/material-issue/scan", productionH.MaterialIssueScan)
			production.POST("/bom", productionH.CreateBom)
		}

		finance := api.Group("/finance")
		{
			finance.GET("/cost-cards", financeH.ListCostCards)
			finance.GET("/cost-summary", financeH.GetCostSummary)
			finance.GET("/account-entries", financeH.ListAccountEntries)
			finance.GET("/financial-report", financeH.GetFinancialReport)
			finance.GET("/budgets", financeH.ListBudgets)
		}

		dashboard := api.Group("/dashboard")
		{
			dashboard.GET("/overview", dashboardH.GetOverview)
			dashboard.GET("/stock-alerts", dashboardH.GetStockAlerts)
			dashboard.GET("/recent-orders", dashboardH.GetRecentOrders)
			dashboard.POST("/llm/analysis", dashboardH.GetLLMAnalysis)
		}

		device := api.Group("/device")
		{
			device.POST("/register", deviceH.Register)
			device.POST("/heartbeat", deviceH.Heartbeat)
			device.GET("/list", deviceH.List)
			device.GET("/:device_code", deviceH.GetByCode)
		}

		llm := api.Group("/llm")
		{
			llm.POST("/chat", llmH.Chat)
			llm.GET("/sessions", llmH.ListSessions)
			llm.GET("/sessions/:session_id/history", llmH.GetHistory)
		}

		openapi := api.Group("/openapi")
		{
			openapi.POST("/webhooks", openapiH.CreateWebhook)
			openapi.POST("/sync", openapiH.Sync)
			openapi.GET("/clients", openapiH.ListClients)
		}
	}
}
