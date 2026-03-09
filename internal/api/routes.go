// Package api provides HTTP API routes and handlers for the V Panel application.
package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/handlers"
	"v/internal/api/middleware"
	"v/internal/auth"
	"v/internal/certificate"
	"v/internal/commercial/balance"
	"v/internal/commercial/commission"
	"v/internal/commercial/coupon"
	"v/internal/commercial/currency"
	"v/internal/commercial/giftcard"
	"v/internal/commercial/invite"
	"v/internal/commercial/invoice"
	"v/internal/commercial/order"
	"v/internal/commercial/pause"
	"v/internal/commercial/payment"
	"v/internal/commercial/plan"
	"v/internal/commercial/planchange"
	"v/internal/commercial/refund"
	"v/internal/commercial/trial"
	"v/internal/config"
	"v/internal/database/repository"
	"v/internal/ip"
	logservice "v/internal/log"
	"v/internal/logger"
	"v/internal/node"
	"v/internal/portal/announcement"
	portalauth "v/internal/portal/auth"
	"v/internal/portal/help"
	portalnode "v/internal/portal/node"
	"v/internal/portal/stats"
	"v/internal/portal/ticket"
	"v/internal/proxy"
	"v/internal/settings"
	"v/internal/subscription"
	"v/internal/xray"
)

// Router manages API routes.
type Router struct {
	engine             *gin.Engine
	config             *config.Config
	logger             logger.Logger
	authService        *auth.Service
	proxyManager       proxy.Manager
	repos              *repository.Repositories
	settingsService    *settings.Service
	xrayManager        xray.Manager
	logService         *logservice.Service
	certificateService CertificateService
	nodeHealthChecker  *node.HealthChecker
}

// CertificateService defines the interface for certificate operations.
type CertificateService interface {
	Apply(ctx context.Context, req *certificate.ApplyRequest) (*repository.Certificate, error)
	Upload(ctx context.Context, domain string, certData, keyData []byte) (*repository.Certificate, error)
	Renew(ctx context.Context, certID int64) error
	DeployToAssignedNodes(ctx context.Context, certID int64) error
}

// NewRouter creates a new API router.
func NewRouter(
	cfg *config.Config,
	log logger.Logger,
	authService *auth.Service,
	proxyManager proxy.Manager,
	repos *repository.Repositories,
	logService *logservice.Service,
	certService CertificateService,
) *Router {
	// Set Gin mode based on config
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Create settings service
	settingsService := settings.NewService(repos.Settings)

	// Create Xray manager
	xrayManager := xray.NewManager(xray.Config{
		BinaryPath: cfg.Xray.BinaryPath,
		ConfigPath: cfg.Xray.ConfigPath,
		BackupDir:  cfg.Xray.BackupDir,
	}, log)

	return &Router{
		engine:             engine,
		config:             cfg,
		logger:             log,
		authService:        authService,
		proxyManager:       proxyManager,
		repos:              repos,
		settingsService:    settingsService,
		xrayManager:        xrayManager,
		logService:         logService,
		certificateService: certService,
		nodeHealthChecker:  nil, // 将在 Setup() 中初始化
	}
}

// Setup configures all routes and middleware.
func (r *Router) Setup() {
	// Global middleware
	r.engine.Use(middleware.Recovery(r.logger))
	r.engine.Use(middleware.SecureHeaders())
	r.engine.Use(middleware.LoggerWithService(r.logger, r.logService))
	r.engine.Use(middleware.CORS(r.config.Server.CORSOrigins))
	r.engine.Use(middleware.RequestID())
	r.engine.Use(middleware.ErrorHandler(r.logger)) // 统一错误处理
	// Removed global rate limit - too restrictive for development
	// r.engine.Use(middleware.RateLimit(100)) // 100 requests per second per IP

	// Create handlers
	authHandler := handlers.NewAuthHandler(r.authService, r.repos.User, r.repos.LoginHistory, r.logger)
	proxyHandler := handlers.NewProxyHandlerWithTraffic(r.proxyManager, r.repos.Proxy, r.repos.Traffic, r.logger)
	systemHandler := handlers.NewSystemHandler(r.config, r.logger)
	healthHandler := handlers.NewHealthHandler(r.repos, r.logger, r.xrayManager, nil)
	roleHandler := handlers.NewRoleHandler(r.logger, r.repos.Role)
	statsHandler := handlers.NewStatsHandler(r.logger, r.repos, nil)
	settingsHandler := handlers.NewSettingsHandler(r.logger, r.settingsService)
	xrayHandler := handlers.NewXrayHandler(r.xrayManager, r.logger)
	certificateHandler := handlers.NewCertificateHandler(r.repos.Certificate, r.repos.Node, r.certificateService, r.logger)
	logHandler := handlers.NewLogHandler(r.logService, r.logger)

	// Create IP restriction service and handler
	ipServiceConfig := &ip.ServiceConfig{
		GeoConfig: &ip.GeolocationConfig{
			DatabasePath: "", // Disable GeoIP database to avoid initialization errors
			CacheTTL:     24 * time.Hour,
		},
	}
	ipService, err := ip.NewService(r.repos.DB(), ipServiceConfig)
	if err != nil {
		r.logger.Error("Failed to create IP service", logger.F("error", err))
		// Continue without IP service - don't block application startup
		ipService = nil
	}

	// Always create handler - it will handle nil service gracefully
	ipRestrictionHandler := handlers.NewIPRestrictionHandler(r.logger, ipService)
	if ipService == nil {
		r.logger.Warn("IP restriction service is disabled due to initialization failure")
	}

	// Create subscription service and handler
	subscriptionService := subscription.NewService(
		r.repos.Subscription,
		r.repos.User,
		r.repos.Proxy,
		r.logger,
		r.config.GetBaseURL(),
	)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService, r.logger)

	// Create commercial services
	planService := plan.NewService(r.repos.Plan, r.logger)
	balanceService := balance.NewService(r.repos.Balance, r.logger)
	couponService := coupon.NewService(r.repos.Coupon, r.logger)
	orderService := order.NewService(r.repos.Order, r.repos.Plan, r.logger, nil)
	paymentService := payment.NewService(orderService, r.logger)

	// Create payment retry service
	retryService := payment.NewRetryService(r.repos.Order, paymentService, nil, r.logger)

	inviteService := invite.NewService(r.repos.Invite, r.logger, &invite.Config{BaseURL: r.config.GetBaseURL()})
	commissionService := commission.NewService(r.repos.Invite, balanceService, r.logger, nil)
	invoiceService := invoice.NewService(r.repos.Invoice, r.repos.Order, r.logger, nil)
	refundService := refund.NewService(r.repos.Order, balanceService, commissionService, r.logger)
	trialService := trial.NewService(r.repos.Trial, r.repos.User, r.logger, nil)
	planChangeService := planchange.NewService(r.repos.PlanChange, r.repos.Plan, r.repos.User, orderService, balanceService, r.logger)

	// Create pause service
	pauseService := pause.NewService(r.repos.Pause, r.repos.User, r.logger, nil)

	// Create gift card service
	giftCardService := giftcard.NewService(r.repos.GiftCard, balanceService, r.logger)

	// Create currency service
	currencyService := currency.NewService(r.repos.ExchangeRate, nil, nil, r.logger)
	planCurrencyService := plan.NewCurrencyService(planService, currencyService, r.repos.PlanPrice, r.logger)

	// Create node management services
	nodeService := node.NewService(
		r.repos.Node,
		r.repos.UserNodeAssignment,
		r.logger,
	)
	nodeGroupService := node.NewGroupService(r.repos.NodeGroup, r.repos.Node, r.logger)
	r.nodeHealthChecker = node.NewHealthChecker(nil, r.repos.Node, r.repos.Certificate, r.repos.HealthCheck, r.logger)
	nodeTrafficService := node.NewTrafficService(r.repos.NodeTraffic, r.repos.NodeGroup, r.logger)
	nodeDeployService := node.NewRemoteDeployService(r.logger, r.repos.Node)
	nodeRecoveryTracker := handlers.NewNodeRecoveryTracker(r.logger)

	// Create Xray config generator for nodes
	configGenerator := xray.NewConfigGenerator(r.repos.Proxy, r.repos.Certificate, r.logger)
	nodeConfigTestHandler := handlers.NewNodeConfigTestHandler(configGenerator, r.logger)
	nodeAgentHandler := handlers.NewNodeAgentHandler(nodeService, r.repos.Node, configGenerator, nodeRecoveryTracker, r.logger)

	// Create node management handlers
	nodeHandler := handlers.NewNodeHandler(nodeService, nodeDeployService, nodeRecoveryTracker, r.logger)
	nodeGroupHandler := handlers.NewNodeGroupHandler(nodeGroupService, r.logger)
	nodeHealthHandler := handlers.NewNodeHealthHandler(r.nodeHealthChecker, r.repos.HealthCheck, r.repos.Node, r.logger)
	nodeStatsHandler := handlers.NewNodeStatsHandler(nodeTrafficService, nodeService, nodeGroupService, r.logger)
	nodeDeployHandler := handlers.NewNodeDeployHandler(nodeDeployService, nodeService, r.config, r.logger)
	agentDownloadHandler := handlers.NewAgentDownloadHandler(r.logger)
	if r.nodeHealthChecker != nil {
		r.nodeHealthChecker.SetOnStatusChange(func(nodeID int64, oldStatus, newStatus string) {
			if newStatus != repository.NodeStatusUnhealthy {
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			nodeData, err := r.repos.Node.GetByID(ctx, nodeID)
			if err != nil {
				r.logger.Warn("获取节点状态失败，无法排队恢复命令", logger.Err(err), logger.F("node_id", nodeID))
				return
			}
			if !nodeData.XrayRunning {
				nodeAgentHandler.QueueXrayRecoveryCommand(nodeID, "health_checker", "health checker marked node unhealthy while xray was down")
			}
		})
	}

	// Create commercial handlers
	planHandler := handlers.NewPlanHandler(planService, r.logger)
	orderHandler := handlers.NewOrderHandler(orderService, r.logger)
	paymentHandler := handlers.NewPaymentHandlerWithRetry(paymentService, retryService, r.logger)
	balanceHandler := handlers.NewBalanceHandler(balanceService, r.logger)
	couponHandler := handlers.NewCouponHandler(couponService, r.logger)
	inviteHandler := handlers.NewInviteHandler(inviteService, commissionService, r.logger)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceService, r.logger)
	reportHandler := handlers.NewReportHandler(orderService, r.logger)
	trialHandler := handlers.NewTrialHandler(trialService, r.logger)
	planChangeHandler := handlers.NewPlanChangeHandler(planChangeService, r.logger)
	currencyHandler := handlers.NewCurrencyHandler(currencyService, planCurrencyService, r.logger)
	pauseHandler := handlers.NewPauseHandler(pauseService, r.logger)
	giftCardHandler := handlers.NewGiftCardHandler(giftCardService, r.logger)
	_ = refundService // Will be used in admin routes

	// Initialize system roles
	ctx := context.Background()
	if err := roleHandler.InitSystemRoles(ctx); err != nil {
		r.logger.Error("Failed to initialize system roles", logger.F("error", err))
	}

	// Auth middleware
	authMiddleware := middleware.NewAuthMiddleware(r.authService, r.logger)

	// Access control middleware (checks traffic limits and expiration)
	accessControlMiddleware := middleware.NewAccessControlMiddleware(r.repos.User, r.logger)

	// Subscription rate limiter (60 requests per hour per token/IP)
	subscriptionRateLimiter := middleware.NewSubscriptionRateLimiter(60)

	// Public routes
	r.engine.GET("/health", healthHandler.Health)
	r.engine.GET("/ready", healthHandler.Ready)

	// Public subscription routes (token-based access, no auth required)
	// Apply rate limiting: 60 requests per hour per token/IP
	subscriptionPublic := r.engine.Group("")
	subscriptionPublic.Use(subscriptionRateLimiter.RateLimit())
	{
		subscriptionPublic.GET("/api/subscription/:token", subscriptionHandler.GetContent)
		subscriptionPublic.GET("/s/:code", subscriptionHandler.GetShortContent)
	}

	// API routes
	api := r.engine.Group("/api")
	{
		// Error reporting endpoint (public)
		errorReportHandler := handlers.NewErrorReportHandler(r.logger)
		api.POST("/errors/report", errorReportHandler.ReportErrors)

		// Agent download endpoint (public, for remote deployment)
		// 注意：这是公开端点，用于远程节点下载 Agent
		api.GET("/admin/nodes/agent/download", agentDownloadHandler.DownloadAgent)

		// Auth routes (public)
		auth := api.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
		}

		// SSE endpoint (placeholder - returns 204 No Content to avoid HTML fallback)
		api.GET("/sse/xray-events", func(c *gin.Context) {
			c.Status(204)
		})

		// Protected routes
		protected := api.Group("")
		protected.Use(authMiddleware.Authenticate())
		{
			// Auth routes (protected)
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/auth/me", authHandler.GetCurrentUser)
			protected.PUT("/auth/password", authHandler.ChangePassword)

			// Proxy routes - with access control for traffic limits and expiration
			proxies := protected.Group("/proxies")
			proxies.Use(accessControlMiddleware.CheckProxyAccess())
			{
				proxies.GET("", proxyHandler.List)
				proxies.POST("", proxyHandler.Create)
				proxies.POST("/batch", proxyHandler.BatchOperation)
				proxies.GET("/:id", proxyHandler.Get)
				proxies.PUT("/:id", proxyHandler.Update)
				proxies.DELETE("/:id", proxyHandler.Delete)
				proxies.GET("/:id/link", proxyHandler.GetShareLink)
				proxies.POST("/:id/toggle", proxyHandler.Toggle)
				proxies.POST("/:id/start", proxyHandler.Start)
				proxies.POST("/:id/stop", proxyHandler.Stop)
				proxies.GET("/:id/stats", proxyHandler.GetStats)
			}

			// System routes
			system := protected.Group("/system")
			{
				system.GET("/info", systemHandler.GetInfo)
				system.GET("/status", systemHandler.GetDetailedStatus)
				system.GET("/stats", systemHandler.GetStats)
			}

			// Role routes
			roles := protected.Group("/roles")
			{
				roles.GET("", roleHandler.ListRoles)
				roles.POST("", roleHandler.CreateRole)
				roles.GET("/:id", roleHandler.GetRole)
				roles.PUT("/:id", roleHandler.UpdateRole)
				roles.DELETE("/:id", roleHandler.DeleteRole)
			}

			// Permissions route
			protected.GET("/permissions", roleHandler.GetPermissions)

			// Stats routes
			stats := protected.Group("/stats")
			{
				stats.GET("/dashboard", statsHandler.GetDashboardStats)
				stats.GET("/protocol", statsHandler.GetProtocolStats)
				stats.GET("/traffic", statsHandler.GetTrafficStats)
				stats.GET("/user", statsHandler.GetUserStats)
				stats.GET("/detailed", statsHandler.GetDetailedStats)
			}

			// Subscription routes (user)
			subscriptionRoutes := protected.Group("/subscription")
			{
				subscriptionRoutes.GET("/link", subscriptionHandler.GetLink)
				subscriptionRoutes.GET("/info", subscriptionHandler.GetInfo)
				subscriptionRoutes.POST("/regenerate", subscriptionHandler.Regenerate)
			}

			// User management (admin only)
			users := protected.Group("/users")
			users.Use(authMiddleware.RequireRole("admin"))
			{
				users.GET("", authHandler.ListUsers)
				users.POST("", authHandler.CreateUser)
				users.GET("/:id", authHandler.GetUser)
				users.PUT("/:id", authHandler.UpdateUser)
				users.DELETE("/:id", authHandler.DeleteUser)
				users.POST("/:id/enable", authHandler.EnableUser)
				users.POST("/:id/disable", authHandler.DisableUser)
				users.POST("/:id/reset-password", authHandler.ResetPassword)
				users.GET("/:id/login-history", authHandler.GetLoginHistory)
				users.DELETE("/:id/login-history", authHandler.ClearLoginHistory)
			}

			// Settings routes (admin only)
			settingsRoutes := protected.Group("/settings")
			settingsRoutes.Use(authMiddleware.RequireRole("admin"))
			{
				settingsRoutes.GET("", settingsHandler.GetSettings)
				settingsRoutes.PUT("", settingsHandler.UpdateSettings)
				settingsRoutes.POST("/backup", settingsHandler.BackupSettings)
				settingsRoutes.POST("/restore", settingsHandler.RestoreSettings)
				settingsRoutes.GET("/xray", settingsHandler.GetXraySettings)
				settingsRoutes.POST("/xray", settingsHandler.UpdateXraySettings)
				settingsRoutes.GET("/protocols", settingsHandler.GetProtocolSettings)
				settingsRoutes.POST("/protocols", settingsHandler.UpdateProtocolSettings)
			}

			// Xray routes (admin only)
			xrayRoutes := protected.Group("/xray")
			xrayRoutes.Use(authMiddleware.RequireRole("admin"))
			{
				xrayRoutes.GET("/status", xrayHandler.GetStatus)
				xrayRoutes.POST("/start", xrayHandler.Start)
				xrayRoutes.POST("/stop", xrayHandler.Stop)
				xrayRoutes.POST("/restart", xrayHandler.Restart)
				xrayRoutes.GET("/config", xrayHandler.GetConfig)
				xrayRoutes.PUT("/config", xrayHandler.UpdateConfig)
				xrayRoutes.POST("/validate", xrayHandler.ValidateConfig)
				xrayRoutes.POST("/test-config", xrayHandler.TestConfig)
				xrayRoutes.GET("/version", xrayHandler.GetVersion)
				xrayRoutes.GET("/versions", xrayHandler.GetVersions)
				xrayRoutes.POST("/sync-versions", xrayHandler.SyncVersions)
				xrayRoutes.GET("/check-updates", xrayHandler.CheckUpdates)
				xrayRoutes.POST("/download", xrayHandler.Download)
				xrayRoutes.POST("/install", xrayHandler.Install)
				xrayRoutes.POST("/update", xrayHandler.Update)
				xrayRoutes.POST("/switch-version", xrayHandler.SwitchVersion)
			}

			// Certificates routes (admin only)
			certificatesRoutes := protected.Group("/certificates")
			certificatesRoutes.Use(authMiddleware.RequireRole("admin"))
			{
				certificatesRoutes.GET("", certificateHandler.List)
				certificatesRoutes.GET("/all", certificateHandler.ListAll) // 用于下拉选择
				certificatesRoutes.GET("/:id", certificateHandler.Get)
				certificatesRoutes.GET("/domain/:domain", certificateHandler.GetByDomain)
				certificatesRoutes.POST("", certificateHandler.Create)
				certificatesRoutes.PUT("/:id", certificateHandler.Update)
				certificatesRoutes.DELETE("/:id", certificateHandler.Delete)
				certificatesRoutes.POST("/apply", certificateHandler.Apply)
				certificatesRoutes.POST("/:id/renew", certificateHandler.Renew)
				certificatesRoutes.GET("/:id/validate", certificateHandler.Validate)
				certificatesRoutes.GET("/:id/backup", certificateHandler.Backup)
				certificatesRoutes.GET("/expiring", certificateHandler.GetExpiring)

				// 证书分配到节点
				certificatesRoutes.POST("/:id/assign", certificateHandler.AssignToNodes)
				certificatesRoutes.GET("/:id/nodes", certificateHandler.GetAssignedNodes)
				certificatesRoutes.DELETE("/:id/nodes/:nodeId", certificateHandler.UnassignFromNode)
			}

			// Logs routes (admin only)
			logsRoutes := protected.Group("/logs")
			logsRoutes.Use(authMiddleware.RequireRole("admin"))
			{
				logsRoutes.GET("", logHandler.ListLogs)
				logsRoutes.GET("/export", logHandler.ExportLogs)
				logsRoutes.GET("/:id", logHandler.GetLog)
				logsRoutes.DELETE("", logHandler.DeleteLogs)
				logsRoutes.POST("/cleanup", logHandler.Cleanup)
			}

			// Admin subscription routes (admin only)
			adminSubscriptions := protected.Group("/admin/subscriptions")
			adminSubscriptions.Use(authMiddleware.RequireRole("admin"))
			{
				adminSubscriptions.GET("", subscriptionHandler.AdminList)
				adminSubscriptions.DELETE("/:user_id", subscriptionHandler.AdminRevoke)
				adminSubscriptions.POST("/:user_id/reset-stats", subscriptionHandler.AdminResetStats)
			}

			// ==================== Commercial System Routes ====================

			// Plan routes (public - list active plans)
			plans := protected.Group("/plans")
			{
				plans.GET("", planHandler.ListActivePlans)
				plans.GET("/:id", planHandler.GetPlan)
				plans.GET("/:id/prices", currencyHandler.GetPlanPrices)
			}

			// Currency routes (public)
			currencies := protected.Group("/currencies")
			{
				currencies.GET("", currencyHandler.GetSupportedCurrencies)
				currencies.GET("/detect", currencyHandler.DetectCurrency)
				currencies.GET("/rate", currencyHandler.GetExchangeRate)
				currencies.POST("/convert", currencyHandler.ConvertAmount)
			}

			// Plans with prices (currency-aware)
			protected.GET("/plans-with-prices", currencyHandler.GetPlansWithPrices)

			// Order routes (user)
			orders := protected.Group("/orders")
			{
				orders.POST("", orderHandler.CreateOrder)
				orders.GET("", orderHandler.ListUserOrders)
				orders.GET("/:id", orderHandler.GetOrder)
				orders.POST("/:id/cancel", orderHandler.CancelOrder)
			}

			// Payment routes
			payments := protected.Group("/payments")
			{
				payments.POST("/create", paymentHandler.CreatePayment)
				payments.GET("/status/:orderNo", paymentHandler.GetPaymentStatus)
				payments.GET("/methods", paymentHandler.ListAvailablePaymentMethods)
				payments.POST("/switch-method", paymentHandler.SwitchPaymentMethod)
				payments.POST("/retry", paymentHandler.RetryPayment)
				payments.GET("/retry/:orderID", paymentHandler.GetRetryInfo)
			}

			// Balance routes (user)
			balanceRoutes := protected.Group("/balance")
			{
				balanceRoutes.GET("", balanceHandler.GetBalance)
				balanceRoutes.GET("/transactions", balanceHandler.GetTransactions)
			}

			// Coupon routes (user - validate only)
			coupons := protected.Group("/coupons")
			{
				coupons.POST("/validate", couponHandler.ValidateCoupon)
			}

			// Invite routes (user)
			invites := protected.Group("/invite")
			{
				invites.GET("/code", inviteHandler.GetInviteCode)
				invites.GET("/referrals", inviteHandler.GetReferrals)
				invites.GET("/stats", inviteHandler.GetInviteStats)
				invites.GET("/commissions", inviteHandler.GetCommissions)
				invites.GET("/earnings", inviteHandler.GetCommissionSummary)
			}

			// Invoice routes (user)
			invoices := protected.Group("/invoices")
			{
				invoices.GET("", invoiceHandler.ListInvoices)
				invoices.GET("/:id/download", invoiceHandler.DownloadInvoice)
			}

			// Trial routes (user)
			trials := protected.Group("/trial")
			{
				trials.GET("", trialHandler.GetTrialStatus)
				trials.POST("/activate", trialHandler.ActivateTrial)
			}

			// Plan change routes (user)
			planChanges := protected.Group("/plan-change")
			{
				planChanges.POST("/calculate", planChangeHandler.CalculatePlanChange)
				planChanges.POST("/upgrade", planChangeHandler.UpgradePlan)
				planChanges.POST("/downgrade", planChangeHandler.DowngradePlan)
				planChanges.GET("/downgrade", planChangeHandler.GetPendingDowngrade)
				planChanges.DELETE("/downgrade", planChangeHandler.CancelPendingDowngrade)
			}

			// Subscription pause routes (user)
			subscriptionPause := protected.Group("/subscription/pause")
			{
				subscriptionPause.GET("", pauseHandler.GetPauseStatus)
				subscriptionPause.POST("", pauseHandler.PauseSubscription)
				subscriptionPause.GET("/history", pauseHandler.GetPauseHistory)
			}
			protected.POST("/subscription/resume", pauseHandler.ResumeSubscription)

			// Gift card routes (user)
			giftCards := protected.Group("/gift-cards")
			{
				giftCards.POST("/redeem", giftCardHandler.RedeemGiftCard)
				giftCards.GET("", giftCardHandler.ListUserGiftCards)
				giftCards.POST("/validate", giftCardHandler.ValidateGiftCard)
			}

			// ==================== Admin Commercial Routes ====================

			// Admin plan routes
			adminPlans := protected.Group("/admin/plans")
			adminPlans.Use(authMiddleware.RequireRole("admin"))
			{
				adminPlans.GET("", planHandler.ListAllPlans)
				adminPlans.POST("", planHandler.CreatePlan)
				adminPlans.PUT("/:id", planHandler.UpdatePlan)
				adminPlans.DELETE("/:id", planHandler.DeletePlan)
				adminPlans.PUT("/:id/status", planHandler.TogglePlanStatus)
				adminPlans.PUT("/:id/prices", currencyHandler.SetPlanPrices)
				adminPlans.DELETE("/:id/prices/:currency", currencyHandler.DeletePlanPrice)
			}

			// Admin currency routes
			adminCurrencies := protected.Group("/admin/currencies")
			adminCurrencies.Use(authMiddleware.RequireRole("admin"))
			{
				adminCurrencies.POST("/update-rates", currencyHandler.UpdateExchangeRates)
			}

			// Admin order routes
			adminOrders := protected.Group("/admin/orders")
			adminOrders.Use(authMiddleware.RequireRole("admin"))
			{
				adminOrders.GET("", orderHandler.ListAllOrders)
				adminOrders.PUT("/:id/status", orderHandler.UpdateOrderStatus)
			}

			// Admin balance routes
			adminBalance := protected.Group("/admin/balance")
			adminBalance.Use(authMiddleware.RequireRole("admin"))
			{
				adminBalance.POST("/adjust", balanceHandler.AdjustBalance)
			}

			// Admin coupon routes
			adminCoupons := protected.Group("/admin/coupons")
			adminCoupons.Use(authMiddleware.RequireRole("admin"))
			{
				adminCoupons.GET("", couponHandler.ListCoupons)
				adminCoupons.POST("", couponHandler.CreateCoupon)
				adminCoupons.DELETE("/:id", couponHandler.DeleteCoupon)
				adminCoupons.POST("/batch", couponHandler.GenerateBatchCodes)
			}

			// Admin invoice routes
			adminInvoices := protected.Group("/admin/invoices")
			adminInvoices.Use(authMiddleware.RequireRole("admin"))
			{
				adminInvoices.POST("/generate", invoiceHandler.GenerateInvoice)
			}

			// Admin report routes
			adminReports := protected.Group("/admin/reports")
			adminReports.Use(authMiddleware.RequireRole("admin"))
			{
				adminReports.GET("/revenue", reportHandler.GetRevenueReport)
				adminReports.GET("/orders", reportHandler.GetOrderStats)
				adminReports.GET("/failed-payments", paymentHandler.GetFailedPaymentStats)
				adminReports.GET("/pause-stats", pauseHandler.AdminGetPauseStats)
			}

			// Admin trial routes
			adminTrials := protected.Group("/admin/trials")
			adminTrials.Use(authMiddleware.RequireRole("admin"))
			{
				adminTrials.GET("", trialHandler.AdminListTrials)
				adminTrials.GET("/stats", trialHandler.AdminGetTrialStats)
				adminTrials.POST("/grant", trialHandler.AdminGrantTrial)
				adminTrials.GET("/user/:user_id", trialHandler.AdminGetTrialByUser)
				adminTrials.POST("/expire", trialHandler.AdminExpireTrials)
			}

			// Admin pause routes
			adminPause := protected.Group("/admin/subscription/pause")
			adminPause.Use(authMiddleware.RequireRole("admin"))
			{
				adminPause.GET("/stats", pauseHandler.AdminGetPauseStats)
				adminPause.POST("/auto-resume", pauseHandler.AdminTriggerAutoResume)
			}

			// Admin gift card routes
			adminGiftCards := protected.Group("/admin/gift-cards")
			adminGiftCards.Use(authMiddleware.RequireRole("admin"))
			{
				adminGiftCards.GET("", giftCardHandler.AdminListGiftCards)
				adminGiftCards.POST("/batch", giftCardHandler.AdminCreateBatch)
				adminGiftCards.GET("/stats", giftCardHandler.AdminGetStats)
				adminGiftCards.GET("/:id", giftCardHandler.AdminGetGiftCard)
				adminGiftCards.PUT("/:id/status", giftCardHandler.AdminSetStatus)
				adminGiftCards.DELETE("/:id", giftCardHandler.AdminDeleteGiftCard)
				adminGiftCards.GET("/batch/:batch_id/stats", giftCardHandler.AdminGetBatchStats)
			}

			// User gift card stats (for compatibility)
			giftCardStats := protected.Group("/gift-cards")
			{
				giftCardStats.GET("/stats", giftCardHandler.AdminGetStats)
			}

			// ==================== Node Management Routes ====================

			// Admin node routes
			adminNodes := protected.Group("/admin/nodes")
			adminNodes.Use(authMiddleware.RequireRole("admin"))
			{
				// Node CRUD
				adminNodes.GET("", nodeHandler.List)
				adminNodes.POST("", nodeHandler.Create)
				adminNodes.GET("/statistics", nodeHandler.GetStatistics)

				// Remote deployment (必须在 /:id 之前，避免被参数路由匹配)
				// Agent 下载已移到公开路由
				adminNodes.POST("/test-connection", nodeDeployHandler.TestConnection)

				adminNodes.GET("/:id", nodeHandler.Get)
				adminNodes.GET("/:id/install-status", nodeHandler.GetInstallStatus)
				adminNodes.PUT("/:id", nodeHandler.Update)
				adminNodes.DELETE("/:id", nodeHandler.Delete)
				adminNodes.PUT("/:id/status", nodeHandler.UpdateStatus)

				// Token management
				adminNodes.POST("/:id/token", nodeHandler.GenerateToken)
				adminNodes.POST("/:id/token/rotate", nodeHandler.RotateToken)
				adminNodes.POST("/:id/token/revoke", nodeHandler.RevokeToken)

				// Config preview (for testing)
				adminNodes.GET("/:id/config/preview", nodeConfigTestHandler.PreviewConfig)

				// Remote deployment
				adminNodes.POST("/:id/deploy", nodeDeployHandler.DeployAgent)
				adminNodes.GET("/:id/deploy/script", nodeDeployHandler.GetDeployScript)

				// Health check routes
				adminNodes.POST("/:id/health-check", nodeHealthHandler.CheckNode)
				adminNodes.GET("/:id/health-history", nodeHealthHandler.GetHistory)
				adminNodes.GET("/:id/health-latest", nodeHealthHandler.GetLatest)
				adminNodes.GET("/:id/health-stats", nodeHealthHandler.GetHealthStats)
				adminNodes.POST("/health-check", nodeHealthHandler.CheckAll)
				adminNodes.GET("/cluster-health", nodeHealthHandler.GetClusterHealth)

				// Traffic statistics routes
				adminNodes.GET("/traffic/total", nodeStatsHandler.GetTotalTraffic)
				adminNodes.GET("/traffic/by-node", nodeStatsHandler.GetTrafficStatsByNode)
				adminNodes.GET("/traffic/by-group", nodeStatsHandler.GetTrafficStatsByGroup)
				adminNodes.GET("/traffic/aggregated", nodeStatsHandler.GetAggregatedStats)
				adminNodes.GET("/traffic/realtime", nodeStatsHandler.GetRealTimeStats)
				adminNodes.POST("/traffic", nodeStatsHandler.RecordTraffic)
				adminNodes.POST("/traffic/batch", nodeStatsHandler.RecordTrafficBatch)
				adminNodes.POST("/traffic/cleanup", nodeStatsHandler.CleanupOldRecords)
				adminNodes.GET("/:id/traffic", nodeStatsHandler.GetTrafficByNode)
				adminNodes.GET("/:id/traffic/top-users", nodeStatsHandler.GetTopUsersByTraffic)
			}

			// Admin node group routes
			adminNodeGroups := protected.Group("/admin/node-groups")
			adminNodeGroups.Use(authMiddleware.RequireRole("admin"))
			{
				// Group CRUD
				adminNodeGroups.GET("", nodeGroupHandler.List)
				adminNodeGroups.POST("", nodeGroupHandler.Create)
				adminNodeGroups.GET("/with-stats", nodeGroupHandler.ListWithStats)
				adminNodeGroups.GET("/stats", nodeGroupHandler.GetAllStats)
				adminNodeGroups.GET("/:id", nodeGroupHandler.Get)
				adminNodeGroups.PUT("/:id", nodeGroupHandler.Update)
				adminNodeGroups.DELETE("/:id", nodeGroupHandler.Delete)
				adminNodeGroups.GET("/:id/stats", nodeGroupHandler.GetWithStats)

				// Group membership management
				adminNodeGroups.GET("/:id/nodes", nodeGroupHandler.GetNodes)
				adminNodeGroups.PUT("/:id/nodes", nodeGroupHandler.SetNodes)
				adminNodeGroups.POST("/:id/nodes/:node_id", nodeGroupHandler.AddNode)
				adminNodeGroups.DELETE("/:id/nodes/:node_id", nodeGroupHandler.RemoveNode)

				// Group traffic statistics
				adminNodeGroups.GET("/:id/traffic", nodeStatsHandler.GetTrafficByGroup)
			}

			// Health checker control routes
			healthChecker := protected.Group("/admin/health-checker")
			healthChecker.Use(authMiddleware.RequireRole("admin"))
			{
				healthChecker.GET("/status", nodeHealthHandler.GetCheckerStatus)
				healthChecker.POST("/start", nodeHealthHandler.StartChecker)
				healthChecker.POST("/stop", nodeHealthHandler.StopChecker)
				healthChecker.PUT("/config", nodeHealthHandler.UpdateCheckerConfig)
			}

			// Admin user routes (node traffic and IP management)
			adminUsers := protected.Group("/admin/users")
			adminUsers.Use(authMiddleware.RequireRole("admin"))
			{
				// Node traffic routes
				adminUsers.GET("/:id/node-traffic", nodeStatsHandler.GetTrafficByUser)
				adminUsers.GET("/:id/node-traffic/breakdown", nodeStatsHandler.GetUserTrafficBreakdown)

				// IP management routes
				adminUsers.GET("/:id/online-ips", ipRestrictionHandler.GetUserOnlineIPs)
				adminUsers.POST("/:id/kick-ip", ipRestrictionHandler.KickUserIP)
			}

			// Admin IP restriction routes
			adminIPRestriction := protected.Group("/admin/ip-restrictions")
			adminIPRestriction.Use(authMiddleware.RequireRole("admin"))
			{
				adminIPRestriction.GET("/stats", ipRestrictionHandler.GetStats)
				adminIPRestriction.GET("/online", ipRestrictionHandler.GetAllOnlineIPs)
				adminIPRestriction.GET("/history", ipRestrictionHandler.GetAllIPHistory)
			}

			adminIPWhitelist := protected.Group("/admin/ip-whitelist")
			adminIPWhitelist.Use(authMiddleware.RequireRole("admin"))
			{
				adminIPWhitelist.GET("", ipRestrictionHandler.GetWhitelist)
				adminIPWhitelist.POST("", ipRestrictionHandler.AddWhitelist)
				adminIPWhitelist.DELETE("/:id", ipRestrictionHandler.DeleteWhitelist)
				adminIPWhitelist.POST("/import", ipRestrictionHandler.ImportWhitelist)
			}

			adminIPBlacklist := protected.Group("/admin/ip-blacklist")
			adminIPBlacklist.Use(authMiddleware.RequireRole("admin"))
			{
				adminIPBlacklist.GET("", ipRestrictionHandler.GetBlacklist)
				adminIPBlacklist.POST("", ipRestrictionHandler.AddBlacklist)
				adminIPBlacklist.DELETE("/:id", ipRestrictionHandler.DeleteBlacklist)
			}

			adminIPSettings := protected.Group("/admin/settings")
			adminIPSettings.Use(authMiddleware.RequireRole("admin"))
			{
				adminIPSettings.GET("/ip-restriction", ipRestrictionHandler.GetIPRestrictionSettings)
				adminIPSettings.PUT("/ip-restriction", ipRestrictionHandler.UpdateIPRestrictionSettings)
			}

			// User IP routes
			userDevices := protected.Group("/user/devices")
			{
				userDevices.GET("", ipRestrictionHandler.GetUserDevices)
				userDevices.POST("/:ip/kick", ipRestrictionHandler.KickUserDevice)
			}

			protected.GET("/user/ip-stats", ipRestrictionHandler.GetUserIPStats)
			protected.GET("/user/ip-history", ipRestrictionHandler.GetUserIPHistory)
		}

		// Payment callback routes (public - no auth required)
		api.POST("/payments/callback/:method", paymentHandler.HandleCallback)

		// Node Agent routes (token-based auth, no user auth required)
		nodeAgent := api.Group("/node")
		{
			nodeAgent.POST("/register", nodeAgentHandler.Register)
			nodeAgent.POST("/heartbeat", nodeAgentHandler.Heartbeat)
			nodeAgent.POST("/command/result", nodeAgentHandler.ReportCommandResult)
			nodeAgent.GET("/:id/config", nodeAgentHandler.GetConfig)
		}

		// Portal routes (user-facing API)
		r.setupPortalRoutes(api)
	}

	// Static files for frontend (if enabled)
	if r.config.Server.StaticPath != "" {
		staticPath := r.config.Server.StaticPath
		r.logger.Info("serving frontend static files", logger.F("static_path", staticPath))

		// Serve static assets (js, css, images, etc.)
		r.engine.Static("/assets", staticPath+"/assets")

		// Serve favicon
		r.engine.GET("/favicon.ico", func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(staticPath + "/favicon.ico")
		})

		// Serve documentation files
		r.engine.Static("/docs", "Docs")

		// SPA fallback - serve index.html for all other routes (except API routes)
		r.engine.NoRoute(func(c *gin.Context) {
			// Don't serve index.html for API routes
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{
					"code":    404,
					"message": "API endpoint not found",
					"error":   "The requested API endpoint does not exist",
				})
				return
			}

			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.File(staticPath + "/index.html")
		})
	}
}

// Engine returns the underlying Gin engine.
func (r *Router) Engine() *gin.Engine {
	return r.engine
}

// setupPortalRoutes configures the user portal API routes.
func (r *Router) setupPortalRoutes(api *gin.RouterGroup) {
	// Create portal services
	portalAuthService := portalauth.NewService(r.repos.User, r.repos.AuthToken)
	ticketService := ticket.NewService(r.repos.Ticket, r.repos.User)
	announcementService := announcement.NewService(r.repos.Announcement)
	helpService := help.NewService(r.repos.HelpArticle)
	portalNodeService := portalnode.NewService(r.repos.Proxy, r.repos.User)
	statsService := stats.NewService(r.repos.Traffic, r.repos.User)

	// Create portal handlers
	portalAuthHandler := handlers.NewPortalAuthHandler(portalAuthService, r.authService, r.repos.User, r.repos.Proxy, r.logger)
	portalDashboardHandler := handlers.NewPortalDashboardHandler(r.repos.User, statsService, announcementService, r.logger)
	portalNodeHandler := handlers.NewPortalNodeHandler(portalNodeService, r.logger)
	portalTicketHandler := handlers.NewPortalTicketHandler(ticketService, r.logger)
	portalAnnouncementHandler := handlers.NewPortalAnnouncementHandler(announcementService, r.logger)
	portalStatsHandler := handlers.NewPortalStatsHandler(statsService, r.logger)
	portalHelpHandler := handlers.NewPortalHelpHandler(helpService, r.logger)

	// Portal auth middleware
	portalAuthMiddleware := middleware.NewPortalAuthMiddleware(r.authService, r.repos.User, r.logger)

	// Portal routes group
	portal := api.Group("/portal")
	{
		// Public auth routes
		portalAuth := portal.Group("/auth")
		{
			portalAuth.POST("/register", portalAuthHandler.Register)
			portalAuth.POST("/login", portalAuthHandler.Login)
			portalAuth.POST("/forgot-password", portalAuthHandler.ForgotPassword)
			portalAuth.POST("/reset-password", portalAuthHandler.ResetPassword)
			portalAuth.GET("/verify-email", portalAuthHandler.VerifyEmail)
			portalAuth.POST("/2fa/login", portalAuthHandler.Verify2FALogin)
		}

		// Protected portal routes
		portalProtected := portal.Group("")
		portalProtected.Use(portalAuthMiddleware.Authenticate())
		{
			// Auth routes (protected)
			portalProtected.POST("/auth/logout", portalAuthHandler.Logout)
			portalProtected.GET("/auth/profile", portalAuthHandler.GetProfile)
			portalProtected.PUT("/auth/profile", portalAuthHandler.UpdateProfile)
			portalProtected.PUT("/auth/password", portalAuthHandler.ChangePassword)
			portalProtected.POST("/auth/2fa/enable", portalAuthHandler.Enable2FA)
			portalProtected.POST("/auth/2fa/verify", portalAuthHandler.Verify2FA)
			portalProtected.POST("/auth/2fa/disable", portalAuthHandler.Disable2FA)

			// Dashboard routes
			portalProtected.GET("/dashboard", portalDashboardHandler.GetDashboard)
			portalProtected.GET("/dashboard/traffic", portalDashboardHandler.GetTrafficSummary)
			portalProtected.GET("/dashboard/announcements", portalDashboardHandler.GetRecentAnnouncements)

			// Node routes
			portalProtected.GET("/nodes", portalNodeHandler.ListNodes)
			portalProtected.GET("/nodes/:id", portalNodeHandler.GetNode)
			portalProtected.POST("/nodes/:id/ping", portalNodeHandler.TestLatency)

			// Ticket routes
			portalProtected.GET("/tickets", portalTicketHandler.ListTickets)
			portalProtected.POST("/tickets", portalTicketHandler.CreateTicket)
			portalProtected.GET("/tickets/:id", portalTicketHandler.GetTicket)
			portalProtected.POST("/tickets/:id/reply", portalTicketHandler.ReplyTicket)
			portalProtected.POST("/tickets/:id/close", portalTicketHandler.CloseTicket)
			portalProtected.POST("/tickets/:id/reopen", portalTicketHandler.ReopenTicket)

			// Announcement routes
			portalProtected.GET("/announcements", portalAnnouncementHandler.ListAnnouncements)
			portalProtected.GET("/announcements/:id", portalAnnouncementHandler.GetAnnouncement)
			portalProtected.POST("/announcements/:id/read", portalAnnouncementHandler.MarkAsRead)
			portalProtected.GET("/announcements/unread-count", portalAnnouncementHandler.GetUnreadCount)

			// Stats routes
			portalProtected.GET("/stats/traffic", portalStatsHandler.GetTrafficStats)
			portalProtected.GET("/stats/usage", portalStatsHandler.GetUsageStats)
			portalProtected.GET("/stats/daily", portalStatsHandler.GetDailyTraffic)
			portalProtected.GET("/stats/export", portalStatsHandler.ExportStats)

			// Help routes
			portalProtected.GET("/help/articles", portalHelpHandler.ListArticles)
			portalProtected.GET("/help/articles/:slug", portalHelpHandler.GetArticle)
			portalProtected.GET("/help/search", portalHelpHandler.SearchArticles)
			portalProtected.GET("/help/featured", portalHelpHandler.GetFeaturedArticles)
			portalProtected.GET("/help/categories", portalHelpHandler.GetCategories)
			portalProtected.POST("/help/articles/:slug/helpful", portalHelpHandler.MarkHelpful)
		}
	}
}

// StartHealthChecker 启动健康检查服务
func (r *Router) StartHealthChecker(ctx context.Context) error {
	r.logger.Info("尝试启动健康检查服务...")

	if r.nodeHealthChecker == nil {
		r.logger.Warn("健康检查服务未初始化，跳过启动")
		return nil
	}

	r.logger.Info("健康检查器已初始化，正在启动...")

	if err := r.nodeHealthChecker.Start(ctx); err != nil {
		r.logger.Error("启动健康检查服务失败", logger.Err(err))
		return err
	}

	r.logger.Info("健康检查服务已成功启动")
	return nil
}

// StopHealthChecker 停止健康检查服务
func (r *Router) StopHealthChecker(ctx context.Context) error {
	if r.nodeHealthChecker == nil {
		return nil
	}

	if err := r.nodeHealthChecker.Stop(ctx); err != nil {
		r.logger.Error("停止健康检查服务失败", logger.Err(err))
		return err
	}

	r.logger.Info("健康检查服务已停止")
	return nil
}
