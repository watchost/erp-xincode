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
	openapiMiddleware "erp-system/internal/openapi/middleware"
	"erp-system/internal/pkg/middleware"
	"erp-system/internal/pkg/printer"
	productionHandler "erp-system/internal/production/handler"
	purchaseHandler "erp-system/internal/purchase/handler"
	salesHandler "erp-system/internal/sales/handler"
	warehouseHandler "erp-system/internal/warehouse/handler"
)

// Permission codes used for route guards. Keep in sync with sys_permission seed.
const (
	PermDashboardView = "dashboard:view"
	PermDashboardLLM  = "dashboard:llm"

	PermWarehouseInbound   = "warehouse:inbound"
	PermWarehouseOutbound  = "warehouse:outbound"
	PermWarehouseInventory = "warehouse:inventory"

	PermWarehouseCheckView    = "warehouse:check:view"
	PermWarehouseCheckCreate  = "warehouse:check:create"
	PermWarehouseCheckFreeze  = "warehouse:check:freeze"
	PermWarehouseCheckEntry   = "warehouse:check:entry"
	PermWarehouseCheckApprove = "warehouse:check:approve"
	PermWarehouseCheckCancel  = "warehouse:check:cancel"

	PermWarehouseTransferView    = "warehouse:transfer:view"
	PermWarehouseTransferCreate  = "warehouse:transfer:create"
	PermWarehouseTransferApprove = "warehouse:transfer:approve"
	PermWarehouseTransferShip    = "warehouse:transfer:ship"
	PermWarehouseTransferReceive = "warehouse:transfer:receive"
	PermWarehouseTransferCancel  = "warehouse:transfer:cancel"

	PermWarehouseBatchView    = "warehouse:batch:view"
	PermWarehouseBatchCreate  = "warehouse:batch:create"
	PermWarehouseSerialView   = "warehouse:serial:view"
	PermWarehouseSerialCreate = "warehouse:serial:create"
	PermWarehouseSerialUpdate = "warehouse:serial:update"

	PermPurchaseOrderView    = "purchase:order:view"
	PermPurchaseOrderCreate  = "purchase:order:create"
	PermPurchaseOrderApprove = "purchase:order:approve"
	PermPurchaseOrderCancel  = "purchase:order:cancel"
	PermPurchaseInbound      = "purchase:inbound"

	PermSalesOrderView    = "sal:order:view"
	PermSalesOrderCreate  = "sal:order:create"
	PermSalesOrderApprove = "sal:order:approve"
	PermSalesOrderCancel  = "sal:order:cancel"
	PermSalesOutbound     = "sal:outbound"
	PermSalesReturn       = "sal:return"

	PermProductionWOView   = "production:wo:view"
	PermProductionWOCreate = "production:wo:create"
	PermProductionOutbound = "production:outbound"
	PermProductionReceipt  = "production:receipt"

	PermFinanceCost    = "finance:cost"
	PermFinanceReports = "finance:reports"
	PermFinanceBudget  = "finance:budget"
	PermFinanceAccount = "finance:account"

	PermFinanceReceivableView   = "finance:receivable:view"
	PermFinanceReceivableCreate = "finance:receivable:create"
	PermFinancePayableView      = "finance:payable:view"
	PermFinancePayableCreate    = "finance:payable:create"
	PermFinanceReceiptView      = "finance:receipt:view"
	PermFinanceReceiptCreate    = "finance:receipt:create"
	PermFinancePaymentView      = "finance:payment:view"
	PermFinancePaymentCreate    = "finance:payment:create"
	PermFinanceSettlementCreate = "finance:settlement:create"
	PermFinanceAgingView        = "finance:aging:view"

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
	OpenAPIAuth  gin.HandlerFunc               // 可选：OpenAPI Bearer 校验，为 nil 时不注册资源 API
	IAM          *iamHandler.IAMHandler
	MDM          *mdmHandler.MDMHandler
	Warehouse    *warehouseHandler.WarehouseHandler
	Purchase     *purchaseHandler.PurchaseHandler
	Sales        *salesHandler.SalesHandler
	Production   *productionHandler.ProductionHandler
	Finance      *financeHandler.FinanceHandler
	Dashboard    *dashboardHandler.DashboardHandler
	Device       *deviceHandler.DeviceHandler
	LLM          *llmHandler.LLMHandler
	OpenAPI      *openapiHandler.OpenAPIHandler
	Printer      *printer.PrinterHandler
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

	// 标准资源 API：独立 Bearer token 校验 + scope 检查（client_id 从 token 提取）。
	if d.OpenAPIAuth != nil {
		v1 := r.Group("/openapi/v1", d.OpenAPIAuth)
		{
			v1.GET("/materials", openapiMiddleware.RequireScope("read"), d.OpenAPI.ListMaterials)
			v1.GET("/inventory", openapiMiddleware.RequireScope("read"), d.OpenAPI.ListInventory)
			v1.GET("/orders/:order_no", openapiMiddleware.RequireScope("read"), d.OpenAPI.GetOrder)
			v1.POST("/orders", openapiMiddleware.RequireScope("write"), d.OpenAPI.CreateOrder)
			v1.POST("/webhooks", openapiMiddleware.RequireScope("write"), d.OpenAPI.CreateWebhook)
			v1.POST("/sync", openapiMiddleware.RequireScope("write"), d.OpenAPI.Sync)
		}
	}

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

			// P2 §3.7 主数据 CSV 导入导出（UTF-8 BOM）
			mdm.GET("/materials-export", middleware.RequirePermission(PermMDMMaterial), d.MDM.ExportMaterials)
			mdm.POST("/materials-import", middleware.RequirePermission(PermMDMMaterial), d.MDM.ImportMaterials)
			mdm.GET("/customers-export", middleware.RequirePermission(PermMDMCustomer), d.MDM.ExportCustomers)
			mdm.POST("/customers-import", middleware.RequirePermission(PermMDMCustomer), d.MDM.ImportCustomers)
			mdm.GET("/suppliers-export", middleware.RequirePermission(PermMDMSupplier), d.MDM.ExportSuppliers)
			mdm.POST("/suppliers-import", middleware.RequirePermission(PermMDMSupplier), d.MDM.ImportSuppliers)
		}

		// P2 §3.7 单据打印与条码（登录即可用，打印内容由调用方提供）
		if d.Printer != nil {
			printG := api.Group("/print")
			printG.POST("/document", d.Printer.RenderDocument)
			printG.GET("/barcode", d.Printer.RenderBarcodeSVG)
		}

		warehouse := api.Group("/warehouse")
		{
			warehouse.POST("/inbound/scan", middleware.RequirePermission(PermWarehouseInbound), d.Warehouse.InboundScan)
			warehouse.POST("/outbound/scan", middleware.RequirePermission(PermWarehouseOutbound), d.Warehouse.OutboundScan)
			warehouse.GET("/inventory", middleware.RequirePermission(PermWarehouseInventory), d.Warehouse.ListInventory)
			warehouse.GET("/stock-alerts", middleware.RequirePermission(PermWarehouseInventory), d.Warehouse.GetStockAlerts)

			// 盘点
			warehouse.POST("/stock-checks", middleware.RequirePermission(PermWarehouseCheckCreate), d.Warehouse.CreateStockCheck)
			warehouse.GET("/stock-checks", middleware.RequirePermission(PermWarehouseCheckView), d.Warehouse.ListStockChecks)
			warehouse.GET("/stock-checks/:id", middleware.RequirePermission(PermWarehouseCheckView), d.Warehouse.GetStockCheck)
			warehouse.POST("/stock-checks/:id/freeze", middleware.RequirePermission(PermWarehouseCheckFreeze), d.Warehouse.FreezeStockCheck)
			warehouse.POST("/stock-checks/:id/items", middleware.RequirePermission(PermWarehouseCheckEntry), d.Warehouse.EntryStockCheck)
			warehouse.POST("/stock-checks/:id/approve", middleware.RequirePermission(PermWarehouseCheckApprove), d.Warehouse.ApproveStockCheck)
			warehouse.POST("/stock-checks/:id/cancel", middleware.RequirePermission(PermWarehouseCheckCancel), d.Warehouse.CancelStockCheck)

			// 调拨
			warehouse.POST("/transfers", middleware.RequirePermission(PermWarehouseTransferCreate), d.Warehouse.CreateTransfer)
			warehouse.GET("/transfers", middleware.RequirePermission(PermWarehouseTransferView), d.Warehouse.ListTransfers)
			warehouse.GET("/transfers/:id", middleware.RequirePermission(PermWarehouseTransferView), d.Warehouse.GetTransfer)
			warehouse.POST("/transfers/:id/approve", middleware.RequirePermission(PermWarehouseTransferApprove), d.Warehouse.ApproveTransfer)
			warehouse.POST("/transfers/:id/ship", middleware.RequirePermission(PermWarehouseTransferShip), d.Warehouse.ShipTransfer)
			warehouse.POST("/transfers/:id/receive", middleware.RequirePermission(PermWarehouseTransferReceive), d.Warehouse.ReceiveTransfer)
			warehouse.POST("/transfers/:id/cancel", middleware.RequirePermission(PermWarehouseTransferCancel), d.Warehouse.CancelTransfer)

			// 批次
			warehouse.POST("/batches", middleware.RequirePermission(PermWarehouseBatchCreate), d.Warehouse.CreateBatch)
			warehouse.GET("/batches", middleware.RequirePermission(PermWarehouseBatchView), d.Warehouse.ListBatches)
			warehouse.GET("/batches/:id", middleware.RequirePermission(PermWarehouseBatchView), d.Warehouse.GetBatch)
			warehouse.POST("/batches/recommend-fefo", middleware.RequirePermission(PermWarehouseBatchView), d.Warehouse.RecommendFEFO)

			// 序列号
			warehouse.POST("/serials", middleware.RequirePermission(PermWarehouseSerialCreate), d.Warehouse.CreateSerials)
			warehouse.GET("/serials", middleware.RequirePermission(PermWarehouseSerialView), d.Warehouse.ListSerials)
			warehouse.GET("/serials/:id", middleware.RequirePermission(PermWarehouseSerialView), d.Warehouse.GetSerial)
			warehouse.PUT("/serials/:id/status", middleware.RequirePermission(PermWarehouseSerialUpdate), d.Warehouse.UpdateSerialStatus)
		}

		purchase := api.Group("/purchase")
		{
			purchase.GET("/orders", middleware.RequirePermission(PermPurchaseOrderView), d.Purchase.ListOrders)
			purchase.POST("/orders", middleware.RequirePermission(PermPurchaseOrderCreate), d.Purchase.CreateOrder)
			purchase.POST("/orders/:order_no/approve", middleware.RequirePermission(PermPurchaseOrderApprove), d.Purchase.ApproveOrder)
			purchase.POST("/orders/:order_no/cancel", middleware.RequirePermission(PermPurchaseOrderCancel), d.Purchase.CancelOrder)
			purchase.POST("/inbound/scan", middleware.RequirePermission(PermPurchaseInbound), d.Purchase.InboundScan)
		}

		sales := api.Group("/sales")
		{
			sales.GET("/orders", middleware.RequirePermission(PermSalesOrderView), d.Sales.ListOrders)
			sales.POST("/orders", middleware.RequirePermission(PermSalesOrderCreate), d.Sales.CreateOrder)
			sales.GET("/orders/:order_no", middleware.RequirePermission(PermSalesOrderView), d.Sales.GetOrder)
			sales.PUT("/orders/:order_no", middleware.RequirePermission(PermSalesOrderCreate), d.Sales.UpdateOrder)
			sales.POST("/orders/:order_no/approve", middleware.RequirePermission(PermSalesOrderApprove), d.Sales.ApproveOrder)
			sales.POST("/orders/:order_no/cancel", middleware.RequirePermission(PermSalesOrderCancel), d.Sales.CancelOrder)
			sales.POST("/outbound/scan", middleware.RequirePermission(PermSalesOutbound), d.Sales.OutboundScan)
			sales.GET("/outbound", middleware.RequirePermission(PermSalesOrderView), d.Sales.ListOutbounds)
			sales.POST("/returns", middleware.RequirePermission(PermSalesReturn), d.Sales.CreateReturn)
			sales.GET("/returns/:return_no", middleware.RequirePermission(PermSalesOrderView), d.Sales.GetReturn)
			sales.POST("/returns/:return_no/receive", middleware.RequirePermission(PermSalesReturn), d.Sales.ReceiveReturn)
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
			production.GET("/bom/explode/:product_id", middleware.RequirePermission(PermProductionWOView), d.Production.ExplodeBom)
			production.POST("/bom/cost-rollup/:product_id", middleware.RequirePermission(PermProductionWOView), d.Production.CalculateBomCostRollup)
			production.GET("/cost-variance", middleware.RequirePermission(PermProductionWOView), d.Production.GetCostVariance)
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

			// 应收应付与对账
			finance.GET("/receivables", middleware.RequirePermission(PermFinanceReceivableView), d.Finance.ListReceivables)
			finance.POST("/receivables", middleware.RequirePermission(PermFinanceReceivableCreate), d.Finance.CreateReceivable)
			finance.GET("/receivables/:id", middleware.RequirePermission(PermFinanceReceivableView), d.Finance.GetReceivable)

			finance.GET("/payables", middleware.RequirePermission(PermFinancePayableView), d.Finance.ListPayables)
			finance.POST("/payables", middleware.RequirePermission(PermFinancePayableCreate), d.Finance.CreatePayable)
			finance.GET("/payables/:id", middleware.RequirePermission(PermFinancePayableView), d.Finance.GetPayable)

			finance.GET("/receipts", middleware.RequirePermission(PermFinanceReceiptView), d.Finance.ListReceipts)
			finance.POST("/receipts", middleware.RequirePermission(PermFinanceReceiptCreate), d.Finance.CreateReceipt)
			finance.GET("/receipts/:id", middleware.RequirePermission(PermFinanceReceiptView), d.Finance.GetReceipt)

			finance.GET("/payments", middleware.RequirePermission(PermFinancePaymentView), d.Finance.ListPayments)
			finance.POST("/payments", middleware.RequirePermission(PermFinancePaymentCreate), d.Finance.CreatePayment)
			finance.GET("/payments/:id", middleware.RequirePermission(PermFinancePaymentView), d.Finance.GetPayment)

			finance.POST("/settlements", middleware.RequirePermission(PermFinanceSettlementCreate), d.Finance.CreateSettlement)
			finance.GET("/reports/aging", middleware.RequirePermission(PermFinanceAgingView), d.Finance.GetAgingReport)
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

		// 管理端：客户端生命周期管理（webhook/sync 走 /openapi/v1 Bearer 通道）。
		openapi := api.Group("/openapi", middleware.RequirePermission(PermOpenAPIAdmin))
		{
			openapi.GET("/clients", d.OpenAPI.ListClients)
			openapi.POST("/clients", d.OpenAPI.CreateClient)
		}
	}
}
