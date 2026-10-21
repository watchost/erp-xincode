// Copyright 2026 zhouhouping. All Rights Reserved.

package routes

import (
	"github.com/gin-gonic/gin"

	dashboardHandler "erp-system/internal/dashboard/handler"
	deviceHandler "erp-system/internal/device/handler"
	financeHandler "erp-system/internal/finance/handler"
	iamHandler "erp-system/internal/iam/handler"
	llmHandler "erp-system/internal/llm/handler"
	mdmHandler "erp-system/internal/mdm/handler"
	openapiHandler "erp-system/internal/openapi/handler"
	"erp-system/internal/pkg/middleware"
	productionHandler "erp-system/internal/production/handler"
	purchaseHandler "erp-system/internal/purchase/handler"
	warehouseHandler "erp-system/internal/warehouse/handler"
)

// Permission codes used for route guards. Keep in sync with sys_permission seed.
const (
	PermDashboardView = "dashboard:view"
	PermDashboardLLM  = "dashboard:llm"

	PermWarehouseInbound   = "warehouse:inbound"
	PermWarehouseOutbound  = "warehouse:outbound"
	PermWarehouseInventory = "warehouse:inventory"

	PermPurchaseOrderView    = "purchase:order:view"
	PermPurchaseOrderCreate  = "purchase:order:create"
	PermPurchaseOrderApprove = "purchase:order:approve"
	PermPurchaseOrderCancel  = "purchase:order:cancel"
	PermPurchaseInbound      = "purchase:inbound"

	PermProductionWOView   = "production:wo:view"
	PermProductionWOCreate = "production:wo:create"
	PermProductionOutbound = "production:outbound"
	PermProductionReceipt  = "production:receipt"

	PermFinanceCost    = "finance:cost"
	PermFinanceReports = "finance:reports"
	PermFinanceBudget  = "finance:budget"
	PermFinanceAccount = "finance:account"

	PermMDMMaterial  = "mdm:material"
	PermMDMSupplier  = "mdm:supplier"
	PermMDMWarehouse = "mdm:warehouse"
	PermMDMLocation  = "mdm:location"
	PermMDMCategory  = "mdm:category"
	PermMDMCustomer  = "mdm:customer"

	PermIAMUser      = "iam:user"
	PermIAMRole      = "iam:role"
	PermIAMDept      = "iam:dept"
	PermIAMAuditView = "iam:audit:view"

	PermDeviceManage = "device:manage"
	PermLLMChat      = "llm:chat"
	PermLLMPrompt    = "llm:prompt"
	PermOpenAPIAdmin = "openapi:admin"
)

// Deps bundles all HTTP handlers and infrastructure config needed for routing.
type Deps struct {
	JWTSecret    string
	JWTBlacklist middleware.BlacklistChecker   // jti 黑名单 + 改密失效校验，为 nil 时不校验
	LoginLimit   gin.HandlerFunc               // 可选：登录/验证码限流中间件，为 nil 时不限流
	ForceChange  middleware.ForceChangeChecker // 可选：强制改密状态查询
	Audit        gin.HandlerFunc               // 可选：写操作审计中间件，为 nil 时不审计
	IAM          *iamHandler.IAMHandler
	MDM          *mdmHandler.MDMHandler
	Warehouse    *warehouseHandler.WarehouseHandler
	Purchase     *purchaseHandler.PurchaseHandler
	Production   *productionHandler.ProductionHandler
	Finance      *financeHandler.FinanceHandler
	Dashboard    *dashboardHandler.DashboardHandler
	Device       *deviceHandler.DeviceHandler
	LLM          *llmHandler.LLMHandler
	OpenAPI      *openapiHandler.OpenAPIHandler
}

// RegisterRoutes wires all HTTP endpoints.
// - Public endpoints (login/refresh/oauth token) are outside the JWT group.
// - All /api/v1 business endpoints require a valid JWT.
// - Mutating/sensitive endpoints additionally require a permission code.
func RegisterRoutes(r *gin.Engine, d Deps) {
	// 公开接口：不需要登录。登录接口叠加限流中间件（若装配）以缓解在线爆破。
	loginChain := make([]gin.HandlerFunc, 0, 2)
	if d.LoginLimit != nil {
		loginChain = append(loginChain, d.LoginLimit)
	}
	loginChain = append(loginChain, d.IAM.Login)
	r.POST("/api/v1/login", loginChain...)
	r.POST("/api/v1/refresh", d.IAM.RefreshToken)

	// 图形验证码（公开，同样套登录限流，防接口刷量）
	captchaChain := make([]gin.HandlerFunc, 0, 2)
	if d.LoginLimit != nil {
		captchaChain = append(captchaChain, d.LoginLimit)
	}
	captchaChain = append(captchaChain, d.IAM.GetCaptcha)
	r.GET("/api/v1/auth/captcha", captchaChain...)

	// OpenAPI 对外网关：OAuth2 取 token 不走内部 JWT
	r.POST("/api/v1/oauth/token", d.OpenAPI.GetToken)
	r.POST("/api/v1/oauth/refresh", d.OpenAPI.RefreshToken)

	api := r.Group("/api/v1")
	api.Use(middleware.JWTAuth(d.JWTSecret, d.JWTBlacklist))
	if d.ForceChange != nil {
		api.Use(middleware.ForcePasswordChange(d.ForceChange))
	}
	if d.Audit != nil {
		api.Use(d.Audit)
	}
	{
		// 当前用户信息 / 登出 / 改密：只要登录即可
		api.GET("/users/profile", d.IAM.GetUserInfo)
		api.GET("/users/permissions", d.IAM.GetPermissions)
		api.POST("/auth/logout", d.IAM.Logout)
		api.POST("/auth/change-password", d.IAM.ChangePassword)

		// 部门树/权限树：登录即可（用户表单、角色授权页都要用）
		api.GET("/iam/departments", d.IAM.DepartmentTree)
		api.GET("/iam/permissions", d.IAM.PermissionTree)

		// IAM 用户管理
		iamG := api.Group("/iam", middleware.RequirePermission(PermIAMUser))
		{
			iamG.GET("/users", d.IAM.ListUsers)
			iamG.GET("/users/:id", d.IAM.GetUser)
			iamG.POST("/users", d.IAM.CreateUser)
			iamG.PUT("/users/:id", d.IAM.UpdateUser)
			iamG.DELETE("/users/:id", d.IAM.DeleteUser)
			iamG.POST("/users/:id/reset-password", d.IAM.ResetPassword)
			iamG.PUT("/users/:id/roles", d.IAM.AssignUserRoles)
		}

		// IAM 角色管理
		roleG := api.Group("/iam", middleware.RequirePermission(PermIAMRole))
		{
			roleG.GET("/roles", d.IAM.ListRoles)
			roleG.POST("/roles", d.IAM.CreateRole)
			roleG.GET("/roles/:id", d.IAM.GetRoleDetail)
			roleG.PUT("/roles/:id", d.IAM.UpdateRole)
			roleG.DELETE("/roles/:id", d.IAM.DeleteRole)
			roleG.GET("/roles/:id/permissions", d.IAM.GetRoleDetail)
			roleG.PUT("/roles/:id/permissions", d.IAM.AssignPermissions)
			roleG.GET("/roles/:id/departments", d.IAM.GetRoleDetail)
			roleG.PUT("/roles/:id/departments", d.IAM.AssignRoleDepts)
		}

		// IAM 部门管理（树查询登录即可，写操作需 iam:dept）
		deptG := api.Group("/iam", middleware.RequirePermission(PermIAMDept))
		{
			deptG.POST("/departments", d.IAM.CreateDepartment)
			deptG.PUT("/departments/:id", d.IAM.UpdateDepartment)
			deptG.DELETE("/departments/:id", d.IAM.DeleteDepartment)
		}

		// 审计日志查询
		api.GET("/iam/audit-logs", middleware.RequirePermission(PermIAMAuditView), d.IAM.ListAuditLogs)

		mdm := api.Group("/mdm")
		{
			mdm.POST("/materials", middleware.RequirePermission(PermMDMMaterial), d.MDM.CreateMaterial)
			mdm.GET("/materials", middleware.RequirePermission(PermMDMMaterial), d.MDM.ListMaterials)
			mdm.GET("/materials/:id", middleware.RequirePermission(PermMDMMaterial), d.MDM.GetMaterial)
			mdm.PUT("/materials/:id", middleware.RequirePermission(PermMDMMaterial), d.MDM.UpdateMaterial)
			mdm.DELETE("/materials/:id", middleware.RequirePermission(PermMDMMaterial), d.MDM.DeleteMaterial)

			mdm.POST("/suppliers", middleware.RequirePermission(PermMDMSupplier), d.MDM.CreateSupplier)
			mdm.GET("/suppliers", middleware.RequirePermission(PermMDMSupplier), d.MDM.ListSuppliers)
			mdm.GET("/suppliers/:id", middleware.RequirePermission(PermMDMSupplier), d.MDM.GetSupplier)
			mdm.PUT("/suppliers/:id", middleware.RequirePermission(PermMDMSupplier), d.MDM.UpdateSupplier)
			mdm.DELETE("/suppliers/:id", middleware.RequirePermission(PermMDMSupplier), d.MDM.DeleteSupplier)

			mdm.POST("/warehouses", middleware.RequirePermission(PermMDMWarehouse), d.MDM.CreateWarehouse)
			mdm.GET("/warehouses", middleware.RequirePermission(PermMDMWarehouse), d.MDM.ListWarehouses)
			mdm.GET("/warehouses/:id", middleware.RequirePermission(PermMDMWarehouse), d.MDM.GetWarehouse)
			mdm.PUT("/warehouses/:id", middleware.RequirePermission(PermMDMWarehouse), d.MDM.UpdateWarehouse)
			mdm.DELETE("/warehouses/:id", middleware.RequirePermission(PermMDMWarehouse), d.MDM.DeleteWarehouse)

			mdm.POST("/locations", middleware.RequirePermission(PermMDMLocation), d.MDM.CreateLocation)
			mdm.GET("/locations", middleware.RequirePermission(PermMDMLocation), d.MDM.ListLocations)
			mdm.GET("/locations/:id", middleware.RequirePermission(PermMDMLocation), d.MDM.GetLocation)
			mdm.PUT("/locations/:id", middleware.RequirePermission(PermMDMLocation), d.MDM.UpdateLocation)
			mdm.DELETE("/locations/:id", middleware.RequirePermission(PermMDMLocation), d.MDM.DeleteLocation)

			mdm.GET("/categories/tree", middleware.RequirePermission(PermMDMCategory), d.MDM.GetCategoryTree)
			mdm.POST("/categories", middleware.RequirePermission(PermMDMCategory), d.MDM.CreateCategory)
			mdm.PUT("/categories/:id", middleware.RequirePermission(PermMDMCategory), d.MDM.UpdateCategory)
			mdm.DELETE("/categories/:id", middleware.RequirePermission(PermMDMCategory), d.MDM.DeleteCategory)

			mdm.POST("/customers", middleware.RequirePermission(PermMDMCustomer), d.MDM.CreateCustomer)
			mdm.GET("/customers", middleware.RequirePermission(PermMDMCustomer), d.MDM.ListCustomers)
			mdm.GET("/customers/:id", middleware.RequirePermission(PermMDMCustomer), d.MDM.GetCustomer)
			mdm.PUT("/customers/:id", middleware.RequirePermission(PermMDMCustomer), d.MDM.UpdateCustomer)
			mdm.DELETE("/customers/:id", middleware.RequirePermission(PermMDMCustomer), d.MDM.DeleteCustomer)
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
			purchase.POST("/orders/:order_no/cancel", middleware.RequirePermission(PermPurchaseOrderCancel), d.Purchase.CancelOrder)
			purchase.POST("/inbound/scan", middleware.RequirePermission(PermPurchaseInbound), d.Purchase.InboundScan)
		}

		production := api.Group("/production")
		{
			production.GET("/work-orders", middleware.RequirePermission(PermProductionWOView), d.Production.ListWorkOrders)
			production.POST("/work-orders", middleware.RequirePermission(PermProductionWOCreate), d.Production.CreateWorkOrder)
			production.POST("/work-orders/:work_order_no/release", middleware.RequirePermission(PermProductionWOCreate), d.Production.ReleaseWorkOrder)
			production.POST("/work-orders/:work_order_no/close", middleware.RequirePermission(PermProductionWOCreate), d.Production.CloseWorkOrder)
			production.POST("/material-issue/scan", middleware.RequirePermission(PermProductionOutbound), d.Production.MaterialIssueScan)
			production.POST("/receipt", middleware.RequirePermission(PermProductionReceipt), d.Production.ProductionReceipt)
			production.POST("/bom", middleware.RequirePermission(PermProductionWOCreate), d.Production.CreateBom)
		}

		finance := api.Group("/finance")
		{
			finance.GET("/cost-cards", middleware.RequirePermission(PermFinanceCost), d.Finance.ListCostCards)
			finance.GET("/cost-summary", middleware.RequirePermission(PermFinanceCost), d.Finance.GetCostSummary)
			finance.GET("/account-entries", middleware.RequirePermission(PermFinanceReports), d.Finance.ListAccountEntries)
			finance.GET("/financial-report", middleware.RequirePermission(PermFinanceReports), d.Finance.GetFinancialReport)
			finance.GET("/budgets", middleware.RequirePermission(PermFinanceBudget), d.Finance.ListBudgets)
			finance.POST("/budgets", middleware.RequirePermission(PermFinanceBudget), d.Finance.CreateBudget)

			finance.GET("/accounts/tree", middleware.RequirePermission(PermFinanceAccount), d.Finance.ListAccountTree)
			finance.POST("/accounts", middleware.RequirePermission(PermFinanceAccount), d.Finance.CreateAccount)
			finance.PUT("/accounts/:id", middleware.RequirePermission(PermFinanceAccount), d.Finance.UpdateAccount)
			finance.DELETE("/accounts/:id", middleware.RequirePermission(PermFinanceAccount), d.Finance.DeleteAccount)

			finance.GET("/vouchers", middleware.RequirePermission(PermFinanceReports), d.Finance.ListVouchers)
			finance.GET("/vouchers/:id", middleware.RequirePermission(PermFinanceReports), d.Finance.GetVoucher)
			finance.POST("/period-close", middleware.RequirePermission(PermFinanceAccount), d.Finance.ClosePeriod)

			finance.GET("/income-statement", middleware.RequirePermission(PermFinanceReports), d.Finance.GetIncomeStatement)
			finance.GET("/cash-flow", middleware.RequirePermission(PermFinanceReports), d.Finance.GetCashFlow)
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
			device.POST("/:device_code/command", d.Device.SendCommand)
			device.GET("/:device_code/messages", d.Device.ListMessages)
		}

		llm := api.Group("/llm", middleware.RequirePermission(PermLLMChat))
		{
			llm.POST("/chat", d.LLM.Chat)
			llm.POST("/chat/stream", d.LLM.ChatStream)
			llm.GET("/sessions", d.LLM.ListSessions)
			llm.GET("/sessions/:session_id/history", d.LLM.GetHistory)
		}

		prompts := api.Group("/llm/prompts", middleware.RequirePermission(PermLLMPrompt))
		{
			prompts.GET("", d.LLM.ListPromptTemplates)
			prompts.POST("", d.LLM.CreatePromptTemplate)
			prompts.PUT("/:id", d.LLM.UpdatePromptTemplate)
			prompts.DELETE("/:id", d.LLM.DeletePromptTemplate)
		}

		openapi := api.Group("/openapi", middleware.RequirePermission(PermOpenAPIAdmin))
		{
			openapi.POST("/webhooks", d.OpenAPI.CreateWebhook)
			openapi.POST("/sync", d.OpenAPI.Sync)
			openapi.GET("/clients", d.OpenAPI.ListClients)
		}
	}
}
