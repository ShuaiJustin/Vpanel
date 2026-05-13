// Package api provides HTTP API routes and handlers for the V Panel application.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/handlers"
	"v/internal/api/middleware"
	"v/internal/auth"
	"v/internal/cache"
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
	"v/internal/entitlement"
	"v/internal/ip"
	logservice "v/internal/log"
	"v/internal/logger"
	"v/internal/node"
	"v/internal/notification"
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
	engine                    *gin.Engine
	config                    *config.Config
	logger                    logger.Logger
	authService               *auth.Service
	proxyManager              proxy.Manager
	repos                     *repository.Repositories
	settingsService           *settings.Service
	notificationService       *notification.Service
	trialService              *trial.Service
	entitlementService        *entitlement.Service
	logService                *logservice.Service
	certificateService        CertificateService
	nodeHealthChecker         *node.HealthChecker
	nodeTrafficResetScheduler *node.TrafficResetScheduler
	runtimeReconciler         *entitlement.RuntimeReconciler
	nodeRecoveryTracker       *handlers.NodeRecoveryTracker
	cache                     cache.Cache
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

	return &Router{
		engine:                    engine,
		config:                    cfg,
		logger:                    log,
		authService:               authService,
		proxyManager:              proxyManager,
		repos:                     repos,
		settingsService:           settingsService,
		notificationService:       notification.NewService(&notification.NotificationConfig{}),
		logService:                logService,
		certificateService:        certService,
		nodeHealthChecker:         nil, // 将在 Setup() 中初始化
		nodeTrafficResetScheduler: nil,
		runtimeReconciler:         nil,
		nodeRecoveryTracker:       nil,
	}
}

// Setup configures all routes and middleware.
func (r *Router) Setup() {
	// Global middleware
	r.engine.Use(middleware.Recovery(r.logger))
	r.engine.Use(middleware.SecureHeaders())
	r.engine.Use(middleware.Compression())
	r.engine.Use(middleware.LoggerWithService(r.logger, r.logService))
	r.engine.Use(middleware.CORS(r.config.Server.CORSOrigins))
	r.engine.Use(middleware.RequestID())
	r.engine.Use(middleware.ErrorHandler(r.logger)) // 统一错误处理

	// SECURITY: Add CSRF protection for authenticated requests
	// Note: CSRF middleware will skip GET/HEAD/OPTIONS and unauthenticated requests
	r.engine.Use(middleware.CSRFTokenProvider()) // Provide CSRF tokens to clients
	r.engine.Use(middleware.CSRFProtection())    // Validate CSRF tokens on state-changing operations

	// Removed global rate limit - too restrictive for development
	// r.engine.Use(middleware.RateLimit(100)) // 100 requests per second per IP

	statsCache := cache.NewMemoryCache(cache.Config{
		DefaultTTL:     30 * time.Second,
		MaxMemoryItems: 512,
		KeyPrefix:      "stats:",
	})

	// Store cache in router for use by handlers
	r.cache = statsCache

	// Create handlers
	authHandler := handlers.NewAuthHandler(r.authService, r.repos.User, r.repos.LoginHistory, r.logger).
		WithSecuritySettings(r.settingsService).
		WithRoleRepository(r.repos.Role)
	proxyHandler := handlers.NewProxyHandlerWithTraffic(r.proxyManager, r.repos.Proxy, r.repos.Traffic, r.logger).
		WithNodeRepository(r.repos.Node).
		WithUserRepositories(r.repos.User, r.repos.Trial).
		WithIPTracker(ip.NewTracker(r.repos.DB())).
		WithCache(statsCache)
	systemHandler := handlers.NewSystemHandler(r.config, r.logger)
	healthHandler := handlers.NewHealthHandler(r.repos, r.logger, nil)
	roleHandler := handlers.NewRoleHandler(r.logger, r.repos.Role)
	statsHandler := handlers.NewStatsHandler(r.logger, r.repos, statsCache)
	settingsHandler := handlers.NewSettingsHandler(r.logger, r.settingsService)
	xrayReleasesHandler := handlers.NewXrayReleasesHandler(r.logger)
	certificateHandler := handlers.NewCertificateHandler(r.repos.Certificate, r.repos.Node, r.certificateService, r.logger)
	logHandler := handlers.NewLogHandler(r.logService, r.logger)

	// Create IP restriction service and handler
	ipService, err := ip.NewService(r.repos.DB(), &ip.ServiceConfig{})
	if err != nil {
		r.logger.Error("Failed to create IP service", logger.F("error", err))
		// Continue without IP service - don't block application startup
		ipService = nil
	}

	// Always create handler - it will handle nil service gracefully
	ipRestrictionHandler := handlers.NewIPRestrictionHandler(r.logger, ipService)
	ipRestrictionMiddleware := middleware.NewIPRestrictionMiddleware(ipService, r.logger)
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
	).WithNodeRepository(r.repos.Node)
	subscriptionHandler := handlers.NewSubscriptionHandler(subscriptionService, r.logger, r.config.Server.SubscriptionUpdateInterval).
		WithIPService(ipService)

	// Create commercial services
	planService := plan.NewService(r.repos.Plan, r.logger)
	balanceService := balance.NewService(r.repos.Balance, r.logger).WithRechargeRepository(r.repos.BalanceRecharge)
	couponService := coupon.NewService(r.repos.Coupon, r.logger)
	orderService := order.NewService(r.repos.Order, r.repos.Plan, r.logger, nil).
		WithUserRepository(r.repos.User).
		WithBalanceRefunder(balanceService.Refund)
	paymentService := payment.NewService(orderService, r.logger).WithBalanceService(balanceService).WithRechargeHandler(balanceService)
	r.registerConfiguredPaymentGateways(paymentService)
	r.loadStoredPaymentSettings(context.Background(), paymentService)
	r.loadStoredNotificationSettings(context.Background())
	settingsHandler.
		WithValidateHook(func(ctx context.Context, systemSettings *settings.SystemSettings) error {
			return r.validateSystemSettings(systemSettings)
		}).
		WithAfterSaveHook(func(ctx context.Context, systemSettings *settings.SystemSettings) error {
			if err := r.applyPaymentSettings(paymentService, systemSettings); err != nil {
				return err
			}
			return r.applyNotificationSettings(systemSettings)
		}).
		WithTestEmailHook(func(ctx context.Context, systemSettings *settings.SystemSettings, to string) error {
			return r.sendTestEmail(systemSettings, to)
		})

	// Create payment retry service
	retryService := payment.NewRetryService(r.repos.Order, paymentService, nil, r.logger)

	inviteService := invite.NewService(r.repos.Invite, r.logger, &invite.Config{BaseURL: r.config.GetBaseURL()})
	commissionService := commission.NewService(r.repos.Invite, balanceService, r.logger, nil)
	invoiceService := invoice.NewService(r.repos.Invoice, r.repos.Order, r.logger, nil)
	refundService := refund.NewService(r.repos.Order, balanceService, commissionService, r.logger)
	trialService := trial.NewService(r.repos.Trial, r.repos.User, r.logger, nil)
	r.trialService = trialService
	proxyHandler.WithTrialService(trialService)
	orderService.WithTrialMarker(trialService)
	r.entitlementService = entitlement.NewService(
		r.repos.User,
		r.repos.Trial,
		r.repos.Proxy,
		r.repos.Node,
		r.repos.UserNodeAssignment,
		trialService,
		r.logger,
	).WithProxyManager(r.proxyManager).WithSettingsService(r.settingsService).WithPauseRepository(r.repos.Pause)
	authHandler.WithEntitlementService(r.entitlementService)
	orderService.WithAfterPlanAppliedHook(func(ctx context.Context, userID int64) error {
		_, _, err := r.entitlementService.GetAccessibleProxies(ctx, userID)
		return err
	})
	subscriptionService.WithEntitlementService(r.entitlementService)
	planChangeService := planchange.NewService(r.repos.PlanChange, r.repos.Plan, r.repos.User, orderService, balanceService, r.logger)

	// Create pause service
	pauseService := pause.NewService(r.repos.Pause, r.repos.User, r.logger, nil)
	r.nodeRecoveryTracker = handlers.NewNodeRecoveryTracker(r.logger)

	// Create gift card service
	giftCardService := giftcard.NewService(r.repos.GiftCard, balanceService, r.logger)

	// Create currency service
	currencyService := currency.NewService(r.repos.ExchangeRate, nil, nil, r.logger)
	planCurrencyService := plan.NewCurrencyService(planService, currencyService, r.repos.PlanPrice, r.logger)

	// Create node management services
	nodeService := node.NewService(
		r.repos.Node,
		r.repos.UserNodeAssignment,
		r.repos.Proxy,
		r.logger,
	).WithCache(statsCache)
	nodeGroupService := node.NewGroupService(r.repos.NodeGroup, r.repos.Node, r.logger)
	r.nodeHealthChecker = node.NewHealthChecker(nil, r.repos.Node, r.repos.Proxy, r.repos.Certificate, r.repos.HealthCheck, r.logger)
	r.nodeHealthChecker.SetNotificationService(r.notificationService)
	nodeTrafficService := node.NewTrafficService(
		r.repos.DB(),
		r.repos.NodeTraffic,
		r.repos.Traffic,
		r.repos.Proxy,
		r.repos.User,
		r.repos.Node,
		r.repos.NodeGroup,
		r.logger,
	).
		WithCache(statsCache).
		WithUserAccessCheck(func(ctx context.Context, userID int64) error {
			_, err := r.entitlementService.EvaluateExistingAccess(ctx, userID)
			return err
		}).
		WithAccessRevokedHook(func(ctx context.Context, userID int64, nodeIDs []int64, reason string) {
			source := "node_traffic_service"
			syncReason := fmt.Sprintf("user %d access revoked after traffic update: %s", userID, reason)
			for _, nodeID := range nodeIDs {
				r.nodeRecoveryTracker.QueueConfigSyncCommand(nodeID, source, syncReason)
				r.nodeRecoveryTracker.QueueXrayRestartCommand(nodeID, source, "apply synced entitlement config")
			}
		}).
		WithNodeTrafficLimitExceededHook(func(ctx context.Context, nodeID int64, reason string) {
			source := "node_traffic_service"
			r.nodeRecoveryTracker.QueueConfigSyncCommand(nodeID, source, reason)
			r.nodeRecoveryTracker.QueueXrayRestartCommand(nodeID, source, "apply synced over-limit node config")
		}).
		WithNodeTrafficAlertHook(func(ctx context.Context, alert *node.NodeTrafficAlert) {
			if alert == nil || r.notificationService == nil {
				return
			}

			if err := r.notificationService.NotifyNodeTrafficAlert(notification.NodeTrafficAlertData{
				NodeID:           alert.NodeID,
				NodeName:         alert.NodeName,
				Level:            alert.Level,
				TrafficTotal:     alert.TrafficTotal,
				TrafficLimit:     alert.TrafficLimit,
				UsagePercent:     alert.UsagePercent,
				ThresholdPercent: alert.ThresholdPercent,
				Timestamp:        alert.TriggeredAt,
			}); err != nil {
				r.logger.Warn("failed to send node traffic alert notification",
					logger.Err(err),
					logger.F("node_id", alert.NodeID),
					logger.F("level", alert.Level),
				)
			}
		})

	r.nodeTrafficResetScheduler = node.NewTrafficResetScheduler(nil, nodeTrafficService, r.logger)
	r.runtimeReconciler = entitlement.NewRuntimeReconciler(nil, r.entitlementService, r.repos.Proxy, r.repos.Node, r.logger)
	systemHandler.WithRuntimeReconciler(r.runtimeReconciler)
	nodeDeployService := node.NewRemoteDeployService(r.logger, r.repos.Node)
	proxyHandler.WithRecoveryTracker(r.nodeRecoveryTracker)
	r.entitlementService.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		r.nodeRecoveryTracker.QueueConfigSyncCommand(nodeID, source, reason)
		r.nodeRecoveryTracker.QueueXrayRestartCommand(nodeID, source, "apply synced entitlement config")
	})

	// Create Xray config generator for nodes
	configGenerator := xray.NewConfigGenerator(r.repos.Proxy, r.repos.Certificate, r.repos.Node, r.logger).
		WithUserAccessCheck(func(ctx context.Context, userID int64) error {
			_, err := r.entitlementService.EvaluateExistingAccess(ctx, userID)
			return err
		})
	nodeConfigTestHandler := handlers.NewNodeConfigTestHandler(configGenerator, r.logger)
	nodeAgentHandler := handlers.NewNodeAgentHandler(nodeService, nodeTrafficService, r.repos.Node, configGenerator, r.nodeRecoveryTracker, r.logger).
		WithIPService(ipService)

	// Create node management handlers
	var geoService *ip.GeolocationService
	if ipService != nil {
		geoService = ipService.GeoService()
	}
	nodeHandler := handlers.NewNodeHandler(nodeService, nodeGroupService, nodeDeployService, r.nodeRecoveryTracker, r.logger).
		WithEntitlementService(r.entitlementService)

	// Add cache support for async diagnosis if available
	if r.cache != nil {
		nodeHandler = nodeHandler.WithCache(r.cache)
	}
	nodeNameSuggestionHandler := handlers.NewNodeNameSuggestionHandler(r.logger, geoService)
	nodeGroupHandler := handlers.NewNodeGroupHandler(nodeGroupService, r.logger)
	nodeHealthHandler := handlers.NewNodeHealthHandler(r.nodeHealthChecker, r.repos.HealthCheck, r.repos.Node, r.logger)
	nodeStatsHandler := handlers.NewNodeStatsHandler(nodeTrafficService, nodeService, nodeGroupService, r.logger)
	nodeDeployHandler := handlers.NewNodeDeployHandler(nodeDeployService, nodeService, r.config, r.logger).
		WithEntitlementService(r.entitlementService)
	nodeNetworkOptimizationHandler := handlers.NewNodeNetworkOptimizationHandler(r.repos.Node, nodeDeployService, r.nodeRecoveryTracker, r.logger)
	agentDownloadHandler := handlers.NewAgentDownloadHandler(r.logger)
	if r.nodeHealthChecker != nil {
		r.nodeHealthChecker.SetOnStatusChange(func(nodeID int64, oldStatus, newStatus string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if newStatus == repository.NodeStatusOnline {
				r.nodeRecoveryTracker.QueueConfigSyncCommand(nodeID, "health_checker", "node recovered to online; refresh node config")
				return
			}
			if newStatus != repository.NodeStatusUnhealthy {
				return
			}

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
	orderHandler := handlers.NewOrderHandler(orderService, r.logger).WithRefundService(refundService)
	paymentHandler := handlers.NewPaymentHandlerWithRetry(paymentService, retryService, r.logger)
	balanceHandler := handlers.NewBalanceHandler(balanceService, r.logger).WithPaymentService(paymentService)
	couponHandler := handlers.NewCouponHandler(couponService, r.logger)
	inviteHandler := handlers.NewInviteHandler(inviteService, commissionService, r.logger)
	invoiceHandler := handlers.NewInvoiceHandler(invoiceService, r.logger)
	reportHandler := handlers.NewReportHandler(orderService, r.repos, r.logger)
	trialHandler := handlers.NewTrialHandler(trialService, r.logger)
	planChangeHandler := handlers.NewPlanChangeHandler(planChangeService, r.logger)
	currencyHandler := handlers.NewCurrencyHandler(currencyService, planCurrencyService, r.logger)
	pauseHandler := handlers.NewPauseHandler(pauseService, r.logger)
	giftCardHandler := handlers.NewGiftCardHandler(giftCardService, r.logger)

	// Initialize system roles
	ctx := context.Background()
	if err := roleHandler.InitSystemRoles(ctx); err != nil {
		r.logger.Error("Failed to initialize system roles", logger.F("error", err))
	}

	// Auth middleware
	authMiddleware := middleware.NewAuthMiddleware(r.authService, r.logger).
		WithUserRepository(r.repos.User).
		WithRoleRepository(r.repos.Role)

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
		subscriptionPublic.HEAD("/api/subscription/:token", subscriptionHandler.GetContent)
		subscriptionPublic.GET("/s/:code", subscriptionHandler.GetShortContent)
		subscriptionPublic.HEAD("/s/:code", subscriptionHandler.GetShortContent)
	}

	// API routes
	api := r.engine.Group("/api")
	{
		// Error reporting endpoint (public)
		errorReportHandler := handlers.NewErrorReportHandler(r.logger)
		api.POST("/errors/report", errorReportHandler.ReportErrors)

		// Agent binary download endpoint (public, for remote deployment).
		//
		// 该端点供远程节点执行部署脚本时 curl/wget 拉取 agent 二进制用。
		// 出于远程部署便利性保持公开，但必须限流以防带宽/资源 DoS，并避免
		// 匿名批量拉取——二进制本身不包含机密但每个请求数十 MB。
		// 限制：每 IP 每小时 12 次（相当于平均 5 分钟一次），足够正常部署
		// 节奏，扫描类批量下载会被挡下。
		agentDownloadRateLimiter := middleware.NewAgentDownloadRateLimiter(12)
		api.GET("/admin/nodes/agent/download",
			agentDownloadRateLimiter.RateLimit(),
			agentDownloadHandler.DownloadAgent)

		// Auth routes (public)
		auth := api.Group("/auth")
		{
			// SECURITY: Add rate limiting to prevent brute force attacks
			auth.POST("/login", middleware.AuthRateLimit("login"), authHandler.Login)
			auth.POST("/refresh", authHandler.RefreshToken)
		}

		// SSE endpoint (placeholder - returns 204 No Content to avoid HTML fallback)
		api.GET("/sse/xray-events", func(c *gin.Context) {
			c.Status(204)
		})

		// Protected routes
		protected := api.Group("")
		protected.Use(authMiddleware.Authenticate())
		if ipService != nil {
			protected.Use(ipRestrictionMiddleware.CheckIPRestriction(func(userID int64) int {
				return ipRestrictionHandler.ResolveUserMaxConcurrentIPs(userID)
			}))
		}
		{
			// Auth routes (protected)
			protected.POST("/auth/logout", authHandler.Logout)
			protected.GET("/auth/me", authHandler.GetCurrentUser)
			protected.PUT("/auth/me", authHandler.UpdateCurrentUser)
			protected.PUT("/auth/password", authHandler.ChangePassword)

			// Proxy routes - with access control for traffic limits and expiration
			proxies := protected.Group("/proxies")
			proxies.Use(accessControlMiddleware.CheckProxyAccess())
			{
				proxies.GET("", authMiddleware.RequirePermission("proxy:read"), proxyHandler.List)
				proxies.POST("", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Create)
				proxies.POST("/batch", authMiddleware.RequirePermission("proxy:write"), proxyHandler.BatchOperation)
				proxies.GET("/:id", authMiddleware.RequirePermission("proxy:read"), proxyHandler.Get)
				proxies.PUT("/:id", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Update)
				proxies.DELETE("/:id", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Delete)
				proxies.GET("/:id/link", authMiddleware.RequirePermission("proxy:read"), proxyHandler.GetShareLink)
				proxies.POST("/:id/toggle", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Toggle)
				proxies.POST("/:id/start", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Start)
				proxies.POST("/:id/stop", authMiddleware.RequirePermission("proxy:write"), proxyHandler.Stop)
				proxies.GET("/:id/stats", authMiddleware.RequirePermission("proxy:read"), proxyHandler.GetStats)
			}

			// System routes
			system := protected.Group("/system")
			{
				system.GET("/info", authMiddleware.RequirePermission("system:read"), systemHandler.GetInfo)
				system.GET("/status", authMiddleware.RequirePermission("system:read"), systemHandler.GetDetailedStatus)
				system.GET("/stats", authMiddleware.RequirePermission("system:read"), systemHandler.GetStats)
			}

			// Admin system routes
			adminSystem := protected.Group("/admin/system")
			{
				adminSystem.POST("/runtime-reconcile", authMiddleware.RequirePermission("system:write"), systemHandler.AdminTriggerRuntimeReconcile)
				adminSystem.POST("/restart-panel", authMiddleware.RequirePermission("system:write"), systemHandler.AdminRestartPanel)
			}

			// Role routes
			roles := protected.Group("/roles")
			{
				roles.GET("", authMiddleware.RequirePermission("role:read"), roleHandler.ListRoles)
				roles.POST("", authMiddleware.RequirePermission("role:write"), roleHandler.CreateRole)
				roles.GET("/:id", authMiddleware.RequirePermission("role:read"), roleHandler.GetRole)
				roles.PUT("/:id", authMiddleware.RequirePermission("role:write"), roleHandler.UpdateRole)
				roles.DELETE("/:id", authMiddleware.RequirePermission("role:write"), roleHandler.DeleteRole)
			}

			// Permissions route
			protected.GET("/permissions", authMiddleware.RequirePermission("role:read"), roleHandler.GetPermissions)

			// Stats routes
			stats := protected.Group("/stats")
			{
				stats.GET("/dashboard", authMiddleware.RequirePermission("stats:read"), statsHandler.GetDashboardStats)
				stats.GET("/protocol", authMiddleware.RequirePermission("stats:read"), statsHandler.GetProtocolStats)
				stats.GET("/traffic", authMiddleware.RequirePermission("stats:read"), statsHandler.GetTrafficStats)
				stats.GET("/user", authMiddleware.RequirePermission("stats:read"), statsHandler.GetUserStats)
				stats.GET("/detailed", authMiddleware.RequirePermission("stats:read"), statsHandler.GetDetailedStats)
			}

			// Subscription routes (user)
			subscriptionRoutes := protected.Group("/subscription")
			{
				subscriptionRoutes.GET("/link", subscriptionHandler.GetLink)
				subscriptionRoutes.GET("/info", subscriptionHandler.GetInfo)
				subscriptionRoutes.GET("/access-ips", subscriptionHandler.GetAccessIPs)
				subscriptionRoutes.POST("/regenerate", subscriptionHandler.Regenerate)
			}

			// User management (admin only)
			users := protected.Group("/users")
			{
				users.GET("", authMiddleware.RequirePermission("user:read"), authHandler.ListUsers)
				users.POST("", authMiddleware.RequirePermission("user:write"), authHandler.CreateUser)
				users.GET("/:id", authMiddleware.RequirePermission("user:read"), authHandler.GetUser)
				users.PUT("/:id", authMiddleware.RequirePermission("user:write"), authHandler.UpdateUser)
				users.DELETE("/:id", authMiddleware.RequirePermission("user:write"), authHandler.DeleteUser)
				users.POST("/:id/enable", authMiddleware.RequirePermission("user:write"), authHandler.EnableUser)
				users.POST("/:id/disable", authMiddleware.RequirePermission("user:write"), authHandler.DisableUser)
				users.POST("/:id/rebuild-auto-proxies", authMiddleware.RequirePermission("user:write"), authHandler.RebuildAutoProxies)
				users.POST("/:id/reset-password", authMiddleware.RequirePermission("user:write"), authHandler.ResetPassword)
				users.GET("/:id/login-history", authMiddleware.RequirePermission("user:read"), authHandler.GetLoginHistory)
				users.DELETE("/:id/login-history", authMiddleware.RequirePermission("user:write"), authHandler.ClearLoginHistory)
			}

			// Settings routes (admin only)
			settingsRoutes := protected.Group("/settings")
			{
				settingsRoutes.GET("", authMiddleware.RequirePermission("system:read"), settingsHandler.GetSettings)
				settingsRoutes.PUT("", authMiddleware.RequirePermission("system:write"), settingsHandler.UpdateSettings)
				settingsRoutes.POST("/test-email", authMiddleware.RequirePermission("system:write"), settingsHandler.TestEmail)
				settingsRoutes.POST("/test-db", authMiddleware.RequirePermission("system:write"), settingsHandler.TestDatabase)
				settingsRoutes.POST("/backup-db", authMiddleware.RequirePermission("system:write"), settingsHandler.BackupDatabase)
				settingsRoutes.POST("/backup", authMiddleware.RequirePermission("system:write"), settingsHandler.BackupSettings)
				settingsRoutes.POST("/restore", authMiddleware.RequirePermission("system:write"), settingsHandler.RestoreSettings)
				settingsRoutes.GET("/xray", authMiddleware.RequirePermission("system:read"), settingsHandler.GetXraySettings)
				settingsRoutes.POST("/xray", authMiddleware.RequirePermission("system:write"), settingsHandler.UpdateXraySettings)
				settingsRoutes.GET("/protocols", authMiddleware.RequirePermission("system:read"), settingsHandler.GetProtocolSettings)
				settingsRoutes.POST("/protocols", authMiddleware.RequirePermission("system:write"), settingsHandler.UpdateProtocolSettings)
				settingsRoutes.GET("/auto-proxy", authMiddleware.RequirePermission("system:read"), settingsHandler.GetAutoProxySettings)
				settingsRoutes.POST("/auto-proxy", authMiddleware.RequirePermission("system:write"), settingsHandler.UpdateAutoProxySettings)
			}

			// Certificates routes (admin only)
			certificatesRoutes := protected.Group("/certificates")
			{
				certificatesRoutes.GET("", authMiddleware.RequirePermission("system:read"), certificateHandler.List)
				certificatesRoutes.GET("/all", authMiddleware.RequirePermission("system:read"), certificateHandler.ListAll) // 用于下拉选择
				certificatesRoutes.GET("/:id", authMiddleware.RequirePermission("system:read"), certificateHandler.Get)
				certificatesRoutes.GET("/domain/:domain", authMiddleware.RequirePermission("system:read"), certificateHandler.GetByDomain)
				certificatesRoutes.POST("", authMiddleware.RequirePermission("system:write"), certificateHandler.Create)
				certificatesRoutes.PUT("/:id", authMiddleware.RequirePermission("system:write"), certificateHandler.Update)
				certificatesRoutes.DELETE("/:id", authMiddleware.RequirePermission("system:write"), certificateHandler.Delete)
				certificatesRoutes.POST("/apply", authMiddleware.RequirePermission("system:write"), certificateHandler.Apply)
				certificatesRoutes.POST("/:id/renew", authMiddleware.RequirePermission("system:write"), certificateHandler.Renew)
				certificatesRoutes.GET("/:id/validate", authMiddleware.RequirePermission("system:read"), certificateHandler.Validate)
				certificatesRoutes.GET("/:id/backup", authMiddleware.RequirePermission("system:read"), certificateHandler.Backup)
				certificatesRoutes.GET("/expiring", authMiddleware.RequirePermission("system:read"), certificateHandler.GetExpiring)

				// 证书分配到节点
				certificatesRoutes.POST("/:id/assign", authMiddleware.RequirePermission("system:write"), certificateHandler.AssignToNodes)
				certificatesRoutes.GET("/:id/nodes", authMiddleware.RequirePermission("system:read"), certificateHandler.GetAssignedNodes)
				certificatesRoutes.DELETE("/:id/nodes/:nodeId", authMiddleware.RequirePermission("system:write"), certificateHandler.UnassignFromNode)
			}

			// Logs routes (admin only)
			logsRoutes := protected.Group("/logs")
			{
				logsRoutes.GET("", authMiddleware.RequirePermission("system:read"), logHandler.ListLogs)
				logsRoutes.GET("/export", authMiddleware.RequirePermission("system:read"), logHandler.ExportLogs)
				logsRoutes.GET("/:id", authMiddleware.RequirePermission("system:read"), logHandler.GetLog)
				logsRoutes.DELETE("", authMiddleware.RequirePermission("system:write"), logHandler.DeleteLogs)
				logsRoutes.POST("/cleanup", authMiddleware.RequirePermission("system:write"), logHandler.Cleanup)
			}

			// Admin subscription routes (admin only)
			adminSubscriptions := protected.Group("/admin/subscriptions")
			{
				adminSubscriptions.GET("", authMiddleware.RequirePermission("user:read"), subscriptionHandler.AdminList)
				adminSubscriptions.DELETE("/:user_id", authMiddleware.RequirePermission("user:write"), subscriptionHandler.AdminRevoke)
				adminSubscriptions.POST("/:user_id/reset-stats", authMiddleware.RequirePermission("user:write"), subscriptionHandler.AdminResetStats)
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
				orders.GET("/by-order-no/:orderNo", orderHandler.GetOrderByOrderNo)
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
				balanceRoutes.POST("/recharge", balanceHandler.CreateRecharge)
				balanceRoutes.GET("/recharge/status/:orderNo", balanceHandler.GetRechargeStatus)
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
			{
				adminPlans.GET("", authMiddleware.RequirePermission("system:write"), planHandler.ListAllPlans)
				adminPlans.POST("", authMiddleware.RequirePermission("system:write"), planHandler.CreatePlan)
				adminPlans.PUT("/:id", authMiddleware.RequirePermission("system:write"), planHandler.UpdatePlan)
				adminPlans.DELETE("/:id", authMiddleware.RequirePermission("system:write"), planHandler.DeletePlan)
				adminPlans.PUT("/:id/status", authMiddleware.RequirePermission("system:write"), planHandler.TogglePlanStatus)
				adminPlans.PUT("/:id/prices", authMiddleware.RequirePermission("system:write"), currencyHandler.SetPlanPrices)
				adminPlans.DELETE("/:id/prices/:currency", authMiddleware.RequirePermission("system:write"), currencyHandler.DeletePlanPrice)
			}

			// Admin currency routes
			adminCurrencies := protected.Group("/admin/currencies")
			{
				adminCurrencies.POST("/update-rates", authMiddleware.RequirePermission("system:write"), currencyHandler.UpdateExchangeRates)
			}

			// Admin order routes
			adminOrders := protected.Group("/admin/orders")
			{
				adminOrders.GET("", authMiddleware.RequirePermission("system:write"), orderHandler.ListAllOrders)
				adminOrders.GET("/:id", authMiddleware.RequirePermission("system:write"), orderHandler.GetOrder)
				adminOrders.PUT("/:id/status", authMiddleware.RequirePermission("system:write"), orderHandler.UpdateOrderStatus)
				adminOrders.POST("/:id/refund", authMiddleware.RequirePermission("system:write"), orderHandler.RefundOrder)
			}

			// Admin balance routes
			adminBalance := protected.Group("/admin/balance")
			{
				adminBalance.GET("/recharge-orders", authMiddleware.RequirePermission("system:write"), balanceHandler.ListAdminRechargeOrders)
				adminBalance.GET("/users/:userID", authMiddleware.RequirePermission("system:write"), balanceHandler.AdminGetUserBalance)
				adminBalance.GET("/users/:userID/transactions", authMiddleware.RequirePermission("system:write"), balanceHandler.AdminGetUserTransactions)
				adminBalance.POST("/adjust", authMiddleware.RequirePermission("system:write"), balanceHandler.AdjustBalance)
			}

			// Admin coupon routes
			adminCoupons := protected.Group("/admin/coupons")
			{
				adminCoupons.GET("", authMiddleware.RequirePermission("system:write"), couponHandler.ListCoupons)
				adminCoupons.POST("", authMiddleware.RequirePermission("system:write"), couponHandler.CreateCoupon)
				adminCoupons.PUT("/:id", authMiddleware.RequirePermission("system:write"), couponHandler.UpdateCoupon)
				adminCoupons.DELETE("/:id", authMiddleware.RequirePermission("system:write"), couponHandler.DeleteCoupon)
				adminCoupons.POST("/batch", authMiddleware.RequirePermission("system:write"), couponHandler.GenerateBatchCodes)
			}

			// Admin invoice routes
			adminInvoices := protected.Group("/admin/invoices")
			{
				adminInvoices.POST("/generate", authMiddleware.RequirePermission("system:write"), invoiceHandler.GenerateInvoice)
			}

			// Admin report routes
			adminReports := protected.Group("/admin/reports")
			{
				adminReports.GET("/overview", authMiddleware.RequirePermission("system:write"), reportHandler.GetCommercialOverview)
				adminReports.GET("/revenue", authMiddleware.RequirePermission("system:write"), reportHandler.GetRevenueReport)
				adminReports.GET("/orders", authMiddleware.RequirePermission("system:write"), reportHandler.GetOrderStats)
				adminReports.GET("/failed-payments", authMiddleware.RequirePermission("system:write"), paymentHandler.GetFailedPaymentStats)
				adminReports.GET("/pause-stats", authMiddleware.RequirePermission("system:write"), pauseHandler.AdminGetPauseStats)
			}

			// Admin trial routes
			adminTrials := protected.Group("/admin/trials")
			{
				adminTrials.GET("", authMiddleware.RequirePermission("system:write"), trialHandler.AdminListTrials)
				adminTrials.GET("/stats", authMiddleware.RequirePermission("system:write"), trialHandler.AdminGetTrialStats)
				adminTrials.POST("/grant", authMiddleware.RequirePermission("system:write"), trialHandler.AdminGrantTrial)
				adminTrials.GET("/user/:user_id", authMiddleware.RequirePermission("system:write"), trialHandler.AdminGetTrialByUser)
				adminTrials.POST("/expire", authMiddleware.RequirePermission("system:write"), trialHandler.AdminExpireTrials)
			}

			// Admin pause routes
			adminPause := protected.Group("/admin/subscription/pause")
			{
				adminPause.GET("/stats", authMiddleware.RequirePermission("system:write"), pauseHandler.AdminGetPauseStats)
				adminPause.POST("/auto-resume", authMiddleware.RequirePermission("system:write"), pauseHandler.AdminTriggerAutoResume)
			}

			// Admin gift card routes
			adminGiftCards := protected.Group("/admin/gift-cards")
			{
				adminGiftCards.GET("", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminListGiftCards)
				adminGiftCards.POST("/batch", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminCreateBatch)
				adminGiftCards.GET("/stats", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminGetStats)
				adminGiftCards.GET("/:id", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminGetGiftCard)
				adminGiftCards.PUT("/:id/status", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminSetStatus)
				adminGiftCards.DELETE("/:id", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminDeleteGiftCard)
				adminGiftCards.GET("/batch/:batch_id/stats", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminGetBatchStats)
			}

			// User gift card stats (for compatibility)
			giftCardStats := protected.Group("/gift-cards")
			{
				giftCardStats.GET("/stats", authMiddleware.RequirePermission("system:write"), giftCardHandler.AdminGetStats)
			}

			// ==================== Node Management Routes ====================

			// Admin node routes
			adminNodes := protected.Group("/admin/nodes")
			{
				// Node CRUD
				adminNodes.GET("", authMiddleware.RequirePermission("system:read"), nodeHandler.List)
				adminNodes.POST("", authMiddleware.RequirePermission("system:write"), nodeHandler.Create)
				adminNodes.GET("/name-suggestion", authMiddleware.RequirePermission("system:read"), nodeNameSuggestionHandler.Suggest)
				adminNodes.GET("/statistics", authMiddleware.RequirePermission("system:read"), nodeHandler.GetStatistics)

				// Remote deployment (必须在 /:id 之前，避免被参数路由匹配)
				// Agent 下载已移到公开路由
				adminNodes.POST("/test-connection", authMiddleware.RequirePermission("system:write"), nodeDeployHandler.TestConnection)

				adminNodes.GET("/:id", authMiddleware.RequirePermission("system:read"), nodeHandler.Get)
				adminNodes.GET("/:id/traffic-diagnostic", authMiddleware.RequirePermission("system:read"), nodeHandler.GetTrafficDiagnostic)
				adminNodes.GET("/:id/install-status", authMiddleware.RequirePermission("system:read"), nodeHandler.GetInstallStatus)
				adminNodes.GET("/:id/network-optimization", authMiddleware.RequirePermission("system:read"), nodeNetworkOptimizationHandler.GetProfile)
				adminNodes.POST("/:id/network-optimization/inspect", authMiddleware.RequirePermission("system:write"), nodeNetworkOptimizationHandler.Inspect)
				adminNodes.POST("/:id/network-optimization/apply", authMiddleware.RequirePermission("system:write"), nodeNetworkOptimizationHandler.Apply)
				adminNodes.POST("/:id/network-optimization/rollback", authMiddleware.RequirePermission("system:write"), nodeNetworkOptimizationHandler.Rollback)
				adminNodes.PUT("/:id", authMiddleware.RequirePermission("system:write"), nodeHandler.Update)
				adminNodes.DELETE("/:id", authMiddleware.RequirePermission("system:write"), nodeHandler.Delete)
				adminNodes.PUT("/:id/status", authMiddleware.RequirePermission("system:write"), nodeHandler.UpdateStatus)

				// Token management
				adminNodes.POST("/:id/token", authMiddleware.RequirePermission("system:write"), nodeHandler.GenerateToken)
				adminNodes.POST("/:id/token/rotate", authMiddleware.RequirePermission("system:write"), nodeHandler.RotateToken)
				adminNodes.POST("/:id/token/revoke", authMiddleware.RequirePermission("system:write"), nodeHandler.RevokeToken)

				// Config preview (for testing)
				adminNodes.GET("/:id/config/preview", authMiddleware.RequirePermission("system:read"), nodeConfigTestHandler.PreviewConfig)

				// Remote deployment
				adminNodes.POST("/:id/deploy", authMiddleware.RequirePermission("system:write"), nodeDeployHandler.DeployAgent)
				adminNodes.GET("/:id/deploy/script", authMiddleware.RequirePermission("system:read"), nodeDeployHandler.GetDeployScript)
				adminNodes.POST("/:id/core/start", authMiddleware.RequirePermission("system:write"), nodeHandler.StartCore)
				adminNodes.POST("/:id/core/restart", authMiddleware.RequirePermission("system:write"), nodeHandler.RestartCore)
				adminNodes.POST("/:id/core/sync-config", authMiddleware.RequirePermission("system:write"), nodeHandler.SyncCoreConfig)
				adminNodes.POST("/:id/core/install-version", authMiddleware.RequirePermission("system:write"), nodeHandler.InstallCoreVersion)

				// Xray version list from GitHub (panel proxy, no local state).
				// Used by the per-node "切换版本" dropdown.
				adminNodes.GET("/xray/available-versions", authMiddleware.RequirePermission("system:read"), xrayReleasesHandler.List)

				// Health check routes
				adminNodes.POST("/:id/health-check", authMiddleware.RequirePermission("system:write"), nodeHealthHandler.CheckNode)
				adminNodes.GET("/:id/health-history", authMiddleware.RequirePermission("system:read"), nodeHealthHandler.GetHistory)
				adminNodes.GET("/:id/health-latest", authMiddleware.RequirePermission("system:read"), nodeHealthHandler.GetLatest)
				adminNodes.GET("/:id/health-stats", authMiddleware.RequirePermission("system:read"), nodeHealthHandler.GetHealthStats)
				adminNodes.POST("/health-check", authMiddleware.RequirePermission("system:write"), nodeHealthHandler.CheckAll)
				adminNodes.GET("/cluster-health", authMiddleware.RequirePermission("system:read"), nodeHealthHandler.GetClusterHealth)

				// Traffic statistics routes
				adminNodes.GET("/traffic/total", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTotalTraffic)
				adminNodes.GET("/traffic/by-node", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTrafficStatsByNode)
				adminNodes.GET("/traffic/by-group", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTrafficStatsByGroup)
				adminNodes.GET("/traffic/aggregated", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetAggregatedStats)
				adminNodes.GET("/traffic/realtime", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetRealTimeStats)
				adminNodes.POST("/traffic", authMiddleware.RequirePermission("system:write"), nodeStatsHandler.RecordTraffic)
				adminNodes.POST("/traffic/batch", authMiddleware.RequirePermission("system:write"), nodeStatsHandler.RecordTrafficBatch)
				adminNodes.POST("/traffic/cleanup", authMiddleware.RequirePermission("system:write"), nodeStatsHandler.CleanupOldRecords)
				adminNodes.GET("/:id/traffic", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTrafficByNode)
				adminNodes.GET("/:id/traffic/top-users", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTopUsersByTraffic)
			}

			// Admin node group routes
			adminNodeGroups := protected.Group("/admin/node-groups")
			{
				// Group CRUD
				adminNodeGroups.GET("", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.List)
				adminNodeGroups.POST("", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.Create)
				adminNodeGroups.GET("/with-stats", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.ListWithStats)
				adminNodeGroups.GET("/stats", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.GetAllStats)
				adminNodeGroups.GET("/:id", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.Get)
				adminNodeGroups.PUT("/:id", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.Update)
				adminNodeGroups.DELETE("/:id", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.Delete)
				adminNodeGroups.GET("/:id/stats", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.GetWithStats)

				// Group membership management
				adminNodeGroups.GET("/:id/nodes", authMiddleware.RequirePermission("system:read"), nodeGroupHandler.GetNodes)
				adminNodeGroups.PUT("/:id/nodes", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.SetNodes)
				adminNodeGroups.POST("/:id/nodes/:node_id", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.AddNode)
				adminNodeGroups.DELETE("/:id/nodes/:node_id", authMiddleware.RequirePermission("system:write"), nodeGroupHandler.RemoveNode)

				// Group traffic statistics
				adminNodeGroups.GET("/:id/traffic", authMiddleware.RequirePermission("system:read"), nodeStatsHandler.GetTrafficByGroup)
			}

			// Health checker control routes
			healthChecker := protected.Group("/admin/health-checker")
			{
				healthChecker.GET("/status", authMiddleware.RequirePermission("system:read"), nodeHealthHandler.GetCheckerStatus)
				healthChecker.POST("/start", authMiddleware.RequirePermission("system:write"), nodeHealthHandler.StartChecker)
				healthChecker.POST("/stop", authMiddleware.RequirePermission("system:write"), nodeHealthHandler.StopChecker)
				healthChecker.PUT("/config", authMiddleware.RequirePermission("system:write"), nodeHealthHandler.UpdateCheckerConfig)
			}

			// Admin user routes (node traffic and IP management)
			adminUsers := protected.Group("/admin/users")
			{
				// Node traffic routes
				adminUsers.GET("/:id/node-traffic", authMiddleware.RequirePermission("user:read"), nodeStatsHandler.GetTrafficByUser)
				adminUsers.GET("/:id/node-traffic/breakdown", authMiddleware.RequirePermission("user:read"), nodeStatsHandler.GetUserTrafficBreakdown)

				// IP management routes
				adminUsers.GET("/:id/online-ips", authMiddleware.RequirePermission("user:read"), ipRestrictionHandler.GetUserOnlineIPs)
				adminUsers.POST("/:id/kick-ip", authMiddleware.RequirePermission("user:write"), ipRestrictionHandler.KickUserIP)
			}

			// Admin IP restriction routes
			adminIPRestriction := protected.Group("/admin/ip-restrictions")
			{
				adminIPRestriction.GET("/stats", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetStats)
				adminIPRestriction.GET("/online", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetAllOnlineIPs)
				adminIPRestriction.GET("/history", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetAllIPHistory)
			}

			adminIPWhitelist := protected.Group("/admin/ip-whitelist")
			{
				adminIPWhitelist.GET("", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetWhitelist)
				adminIPWhitelist.POST("", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.AddWhitelist)
				adminIPWhitelist.DELETE("/:id", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.DeleteWhitelist)
				adminIPWhitelist.POST("/import", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.ImportWhitelist)
			}

			adminIPBlacklist := protected.Group("/admin/ip-blacklist")
			{
				adminIPBlacklist.GET("", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetBlacklist)
				adminIPBlacklist.POST("", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.AddBlacklist)
				adminIPBlacklist.DELETE("/:id", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.DeleteBlacklist)
			}

			adminIPSettings := protected.Group("/admin/settings")
			{
				adminIPSettings.GET("/ip-restriction", authMiddleware.RequirePermission("system:read"), ipRestrictionHandler.GetIPRestrictionSettings)
				adminIPSettings.PUT("/ip-restriction", authMiddleware.RequirePermission("system:write"), ipRestrictionHandler.UpdateIPRestrictionSettings)
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

		// Node Agent routes (token-based auth, rate limited, body size limited)
		agentRateLimiter := middleware.NewAgentRateLimiter(30) // 30 req/min per IP
		nodeAgent := api.Group("/node")
		nodeAgent.Use(agentRateLimiter.RateLimit())
		nodeAgent.Use(middleware.MaxBodySize(2 * 1024 * 1024)) // 2MB max body
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

		r.engine.Use(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			if strings.HasPrefix(c.Request.URL.Path, "/downloads/") {
				c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			}
			c.Next()
		})

		// Serve static assets (js, css, images, etc.)
		r.engine.Static("/assets", staticPath+"/assets")
		r.engine.Static("/downloads", staticPath+"/downloads")

		serveFaviconSVG := func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=86400")
			c.Header("Content-Type", "image/svg+xml")
			c.File(staticPath + "/favicon.svg")
		}
		serveStaticIcon := func(filename, contentType string) gin.HandlerFunc {
			return func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age=86400")
				c.Header("Content-Type", contentType)
				c.File(staticPath + "/" + filename)
			}
		}
		serveFaviconICO := func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age=86400")
			icoPath := staticPath + "/favicon.ico"
			if _, err := os.Stat(icoPath); err == nil {
				c.File(icoPath)
				return
			}
			c.Header("Content-Type", "image/svg+xml")
			c.File(staticPath + "/favicon.svg")
		}

		// Serve favicon files before SPA fallback so browsers do not receive index.html.
		r.engine.GET("/favicon.svg", serveFaviconSVG)
		r.engine.HEAD("/favicon.svg", serveFaviconSVG)
		r.engine.GET("/favicon.ico", serveFaviconICO)
		r.engine.HEAD("/favicon.ico", serveFaviconICO)
		r.engine.GET("/favicon-32.png", serveStaticIcon("favicon-32.png", "image/png"))
		r.engine.HEAD("/favicon-32.png", serveStaticIcon("favicon-32.png", "image/png"))
		r.engine.GET("/apple-touch-icon.png", serveStaticIcon("apple-touch-icon.png", "image/png"))
		r.engine.HEAD("/apple-touch-icon.png", serveStaticIcon("apple-touch-icon.png", "image/png"))

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

func (r *Router) validateSystemSettings(systemSettings *settings.SystemSettings) error {
	if err := r.validatePaymentSettings(systemSettings); err != nil {
		return err
	}
	return r.validateEmailSettings(systemSettings)
}

func (r *Router) loadStoredNotificationSettings(ctx context.Context) {
	if r.notificationService == nil || r.settingsService == nil {
		return
	}

	systemSettings, err := r.settingsService.GetSystemSettings(ctx)
	if err != nil {
		r.logger.Warn("Failed to load persisted notification settings", logger.Err(err))
		return
	}

	if err := r.applyNotificationSettings(systemSettings); err != nil {
		r.logger.Warn("Failed to apply persisted notification settings", logger.Err(err))
	}
}

func (r *Router) validateEmailSettings(systemSettings *settings.SystemSettings) error {
	if systemSettings == nil {
		return nil
	}

	hasEmailConfig := strings.TrimSpace(systemSettings.SMTPHost) != "" ||
		strings.TrimSpace(systemSettings.SMTPUser) != "" ||
		strings.TrimSpace(systemSettings.SMTPFrom) != "" ||
		strings.TrimSpace(systemSettings.SMTPAlertEmail) != "" ||
		strings.TrimSpace(systemSettings.SMTPPassword) != "" ||
		systemSettings.SMTPPort != 0

	if !hasEmailConfig {
		return nil
	}

	if strings.TrimSpace(systemSettings.SMTPHost) == "" {
		return fmt.Errorf("smtp host is required")
	}
	if systemSettings.SMTPPort < 1 || systemSettings.SMTPPort > 65535 {
		return fmt.Errorf("smtp port must be between 1 and 65535")
	}
	if strings.TrimSpace(systemSettings.SMTPUser) == "" {
		return fmt.Errorf("smtp user is required")
	}
	if strings.TrimSpace(systemSettings.SMTPPassword) == "" {
		return fmt.Errorf("smtp password is required")
	}
	if systemSettings.SMTPFrom != "" {
		if _, err := mail.ParseAddress(systemSettings.SMTPFrom); err != nil {
			return fmt.Errorf("smtp from address is invalid")
		}
	}
	if systemSettings.SMTPAlertEmail != "" {
		if _, err := mail.ParseAddress(systemSettings.SMTPAlertEmail); err != nil {
			return fmt.Errorf("alert email is invalid")
		}
	}

	return nil
}

func (r *Router) applyNotificationSettings(systemSettings *settings.SystemSettings) error {
	if r.notificationService == nil {
		return nil
	}

	r.notificationService.UpdateConfig(r.buildNotificationConfig(systemSettings))
	if r.nodeHealthChecker != nil {
		r.nodeHealthChecker.SetNotificationService(r.notificationService)
	}

	return nil
}

func (r *Router) sendTestEmail(systemSettings *settings.SystemSettings, to string) error {
	if err := r.validateEmailSettings(systemSettings); err != nil {
		return err
	}
	if r.notificationService == nil {
		return fmt.Errorf("notification service is unavailable")
	}

	r.notificationService.UpdateConfig(r.buildNotificationConfig(systemSettings))

	recipient := strings.TrimSpace(firstNonEmpty(to, systemSettings.SMTPAlertEmail, systemSettings.SMTPUser))
	if recipient == "" {
		return fmt.Errorf("test email recipient is required")
	}

	baseURL := r.config.GetBaseURL()
	if baseURL == "" && systemSettings != nil {
		baseURL = strings.TrimSpace(systemSettings.PanelAPIDomain)
	}

	subject := "V Panel 测试邮件"
	body := "这是一封来自 V Panel 的测试邮件。\n\n如果您收到此邮件，说明当前 SMTP 配置可用。"
	if baseURL != "" {
		body += "\n\n当前面板地址: " + baseURL
	}

	return r.notificationService.SendEmail(recipient, subject, body)
}

func (r *Router) buildNotificationConfig(systemSettings *settings.SystemSettings) *notification.NotificationConfig {
	if systemSettings == nil {
		return &notification.NotificationConfig{
			EnabledTypes:    map[notification.NotificationType]bool{},
			EnabledChannels: map[notification.NotificationChannel]bool{},
		}
	}

	emailEnabled := allNonEmpty(systemSettings.SMTPHost, systemSettings.SMTPUser, systemSettings.SMTPPassword) && systemSettings.SMTPPort > 0
	telegramEnabled := allNonEmpty(systemSettings.TelegramBotToken, systemSettings.TelegramChatID)

	return &notification.NotificationConfig{
		SMTPHost:         strings.TrimSpace(systemSettings.SMTPHost),
		SMTPPort:         systemSettings.SMTPPort,
		SMTPUser:         strings.TrimSpace(systemSettings.SMTPUser),
		SMTPPassword:     systemSettings.SMTPPassword,
		SMTPFrom:         firstNonEmpty(systemSettings.SMTPFrom, systemSettings.SMTPUser),
		AdminEmail:       firstNonEmpty(systemSettings.SMTPAlertEmail, systemSettings.SMTPUser),
		SiteName:         firstNonEmpty(systemSettings.SiteName, "V Panel"),
		TelegramBotToken: strings.TrimSpace(systemSettings.TelegramBotToken),
		TelegramChatID:   strings.TrimSpace(systemSettings.TelegramChatID),
		EnabledTypes: map[notification.NotificationType]bool{
			notification.NotificationNewDevice:        true,
			notification.NotificationIPLimitReached:   true,
			notification.NotificationSuspiciousIP:     true,
			notification.NotificationDeviceKicked:     true,
			notification.NotificationAutoBlacklisted:  true,
			notification.NotificationNodeStatusChange: true,
			notification.NotificationNodeTrafficAlert: true,
		},
		EnabledChannels: map[notification.NotificationChannel]bool{
			notification.ChannelEmail:    emailEnabled,
			notification.ChannelTelegram: telegramEnabled,
		},
	}
}

func (r *Router) registerConfiguredPaymentGateways(paymentService *payment.Service) {
	if paymentService == nil {
		return
	}

	gateways, err := r.buildPaymentGateways(nil)
	if err != nil {
		r.logger.Warn("Failed to initialize payment gateways from static config", logger.Err(err))
		return
	}

	paymentService.ReplaceGateways(gateways)
}

func (r *Router) loadStoredPaymentSettings(ctx context.Context, paymentService *payment.Service) {
	if paymentService == nil || r.settingsService == nil {
		return
	}

	allSettings, err := r.settingsService.GetAll(ctx)
	if err != nil {
		r.logger.Warn("Failed to load persisted payment settings", logger.Err(err))
		return
	}

	if !hasStoredPaymentSettings(allSettings) {
		return
	}

	systemSettings, err := r.settingsService.GetSystemSettings(ctx)
	if err != nil {
		r.logger.Warn("Failed to hydrate persisted payment settings", logger.Err(err))
		return
	}

	if err := r.applyPaymentSettings(paymentService, systemSettings); err != nil {
		r.logger.Warn("Failed to apply persisted payment settings", logger.Err(err))
	}
}

func (r *Router) validatePaymentSettings(systemSettings *settings.SystemSettings) error {
	_, err := r.buildPaymentGateways(systemSettings)
	return err
}

func (r *Router) applyPaymentSettings(paymentService *payment.Service, systemSettings *settings.SystemSettings) error {
	if paymentService == nil {
		return nil
	}

	gateways, err := r.buildPaymentGateways(systemSettings)
	if err != nil {
		return err
	}

	paymentService.ReplaceGateways(gateways)
	return nil
}

func (r *Router) buildPaymentGateways(systemSettings *settings.SystemSettings) (map[string]payment.PaymentGateway, error) {
	baseURL := strings.TrimSuffix(r.config.GetBaseURL(), "/")
	cfg := mergePaymentSettings(r.config.Payment, systemSettings)
	gateways := make(map[string]payment.PaymentGateway)

	if cfg.Alipay.Enabled {
		if !allNonEmpty(cfg.Alipay.AppID, cfg.Alipay.PrivateKey, cfg.Alipay.AlipayPublicKey) {
			return nil, fmt.Errorf("alipay is enabled but configuration is incomplete")
		}

		gateway, err := payment.NewAlipayGateway(&payment.AlipayConfig{
			AppID:           cfg.Alipay.AppID,
			PrivateKey:      cfg.Alipay.PrivateKey,
			AlipayPublicKey: cfg.Alipay.AlipayPublicKey,
			NotifyURL:       firstNonEmpty(cfg.Alipay.NotifyURL, paymentNotifyURL(baseURL, "alipay")),
			ReturnURL:       firstNonEmpty(cfg.Alipay.ReturnURL, paymentReturnURL(baseURL)),
			IsSandbox:       cfg.Alipay.IsSandbox,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize alipay gateway: %w", err)
		}

		gateways[gateway.Name()] = gateway
	}

	if cfg.WeChat.Enabled {
		if !allNonEmpty(cfg.WeChat.AppID, cfg.WeChat.MchID, cfg.WeChat.APIKey) {
			return nil, fmt.Errorf("wechat is enabled but configuration is incomplete")
		}

		gateway, err := payment.NewWeChatGateway(&payment.WeChatConfig{
			AppID:     cfg.WeChat.AppID,
			MchID:     cfg.WeChat.MchID,
			APIKey:    cfg.WeChat.APIKey,
			NotifyURL: firstNonEmpty(cfg.WeChat.NotifyURL, paymentNotifyURL(baseURL, "wechat")),
			IsSandbox: cfg.WeChat.IsSandbox,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize wechat gateway: %w", err)
		}

		gateways[gateway.Name()] = gateway
	}

	return gateways, nil
}

func paymentNotifyURL(baseURL string, method string) string {
	if baseURL == "" {
		return ""
	}
	return baseURL + "/api/payments/callback/" + method
}

func paymentReturnURL(baseURL string) string {
	if baseURL == "" {
		return ""
	}
	return baseURL + "/user/orders"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func allNonEmpty(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

func hasStoredPaymentSettings(allSettings map[string]string) bool {
	for _, key := range []string{
		"payment_alipay_enabled",
		"payment_alipay_app_id",
		"payment_alipay_private_key",
		"payment_alipay_public_key",
		"payment_alipay_notify_url",
		"payment_alipay_return_url",
		"payment_alipay_sandbox",
		"payment_wechat_enabled",
		"payment_wechat_app_id",
		"payment_wechat_mch_id",
		"payment_wechat_api_key",
		"payment_wechat_notify_url",
		"payment_wechat_sandbox",
	} {
		if _, exists := allSettings[key]; exists {
			return true
		}
	}
	return false
}

func mergePaymentSettings(base config.PaymentConfig, systemSettings *settings.SystemSettings) config.PaymentConfig {
	if systemSettings == nil {
		return base
	}

	merged := base
	merged.Alipay.Enabled = systemSettings.PaymentAlipayEnabled
	merged.Alipay.AppID = firstNonEmpty(systemSettings.PaymentAlipayAppID, merged.Alipay.AppID)
	merged.Alipay.PrivateKey = firstNonEmpty(systemSettings.PaymentAlipayPrivateKey, merged.Alipay.PrivateKey)
	merged.Alipay.AlipayPublicKey = firstNonEmpty(systemSettings.PaymentAlipayPublicKey, merged.Alipay.AlipayPublicKey)
	merged.Alipay.NotifyURL = firstNonEmpty(systemSettings.PaymentAlipayNotifyURL, merged.Alipay.NotifyURL)
	merged.Alipay.ReturnURL = firstNonEmpty(systemSettings.PaymentAlipayReturnURL, merged.Alipay.ReturnURL)
	merged.Alipay.IsSandbox = systemSettings.PaymentAlipaySandbox

	merged.WeChat.Enabled = systemSettings.PaymentWeChatEnabled
	merged.WeChat.AppID = firstNonEmpty(systemSettings.PaymentWeChatAppID, merged.WeChat.AppID)
	merged.WeChat.MchID = firstNonEmpty(systemSettings.PaymentWeChatMchID, merged.WeChat.MchID)
	merged.WeChat.APIKey = firstNonEmpty(systemSettings.PaymentWeChatAPIKey, merged.WeChat.APIKey)
	merged.WeChat.NotifyURL = firstNonEmpty(systemSettings.PaymentWeChatNotifyURL, merged.WeChat.NotifyURL)
	merged.WeChat.IsSandbox = systemSettings.PaymentWeChatSandbox

	return merged
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

	// Add cache support for announcement service if available
	if r.cache != nil {
		announcementService = announcementService.WithCache(r.cache)
	}

	helpService := help.NewService(r.repos.HelpArticle)
	portalNodeService := portalnode.NewService(r.repos.Proxy, r.repos.User, r.repos.Node).
		WithEntitlementService(r.entitlementService)
	statsService := stats.NewService(r.repos.DB(), r.repos.Traffic, r.repos.NodeTraffic, r.repos.User)

	// Create portal handlers
	portalAuthHandler := handlers.NewPortalAuthHandler(portalAuthService, r.authService, r.repos.User, r.repos.Proxy, r.logger).
		WithEmailSender(r.notificationService, r.config.GetBaseURL()).
		WithTelegramSender(r.notificationService).
		WithAvatarStoragePath(filepath.Join("data", "avatars")).
		WithRoleRepository(r.repos.Role).
		WithEntitlementService(r.entitlementService)
	portalDashboardHandler := handlers.NewPortalDashboardHandler(r.repos.User, statsService, announcementService, r.logger).
		WithEntitlementService(r.entitlementService)
	portalNodeHandler := handlers.NewPortalNodeHandler(portalNodeService, r.nodeRecoveryTracker, r.logger)
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
			// SECURITY: Add rate limiting to prevent brute force attacks
			portalAuth.POST("/register", middleware.AuthRateLimit("register"), portalAuthHandler.Register)
			portalAuth.POST("/login", middleware.AuthRateLimit("login"), portalAuthHandler.Login)
			portalAuth.POST("/forgot-password", middleware.AuthRateLimit("password-reset"), portalAuthHandler.ForgotPassword)
			portalAuth.POST("/reset-password", middleware.AuthRateLimit("password-reset"), portalAuthHandler.ResetPassword)
			portalAuth.GET("/verify-email", portalAuthHandler.VerifyEmail)
			portalAuth.GET("/avatar/:filename", portalAuthHandler.GetAvatar)
			portalAuth.POST("/2fa/login", middleware.AuthRateLimit("login"), portalAuthHandler.Verify2FALogin)
		}

		// Public help center routes
		portalHelp := portal.Group("/help")
		{
			portalHelp.GET("/articles", portalHelpHandler.ListArticles)
			portalHelp.GET("/articles/:slug", portalHelpHandler.GetArticle)
			portalHelp.GET("/search", portalHelpHandler.SearchArticles)
			portalHelp.GET("/featured", portalHelpHandler.GetFeaturedArticles)
			portalHelp.GET("/categories", portalHelpHandler.GetCategories)
			portalHelp.POST("/articles/:slug/helpful",
				middleware.HelpfulVoteRateLimit(),
				portalHelpHandler.MarkHelpful)
		}

		// Protected portal routes
		portalProtected := portal.Group("")
		portalProtected.Use(portalAuthMiddleware.Authenticate())
		{
			// Auth routes (protected)
			portalProtected.POST("/auth/logout", portalAuthHandler.Logout)
			portalProtected.GET("/auth/profile", portalAuthHandler.GetProfile)
			portalProtected.PUT("/auth/profile", portalAuthHandler.UpdateProfile)
			portalProtected.POST("/auth/avatar", portalAuthHandler.UploadAvatar)
			portalProtected.POST("/auth/telegram/bind",
				middleware.UserActionRateLimit("telegram-bind"),
				portalAuthHandler.BindTelegram)
			portalProtected.DELETE("/auth/telegram/bind", portalAuthHandler.UnbindTelegram)
			portalProtected.POST("/auth/verify-email/resend", portalAuthHandler.ResendVerificationEmail)
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
			portalProtected.POST("/nodes/:id/ping",
				middleware.UserActionRateLimit("latency-test"),
				portalNodeHandler.TestLatency)

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

// StartNodeTrafficResetScheduler starts the monthly node traffic reset scheduler.
func (r *Router) StartNodeTrafficResetScheduler(ctx context.Context) error {
	if r.nodeTrafficResetScheduler == nil {
		r.logger.Warn("节点流量重置调度器未初始化，跳过启动")
		return nil
	}
	if err := r.nodeTrafficResetScheduler.Start(ctx); err != nil {
		r.logger.Error("启动节点流量重置调度器失败", logger.Err(err))
		return err
	}
	return nil
}

// StopNodeTrafficResetScheduler stops the monthly node traffic reset scheduler.
func (r *Router) StopNodeTrafficResetScheduler(ctx context.Context) error {
	if r.nodeTrafficResetScheduler == nil {
		return nil
	}
	if err := r.nodeTrafficResetScheduler.Stop(ctx); err != nil {
		r.logger.Error("停止节点流量重置调度器失败", logger.Err(err))
		return err
	}
	return nil
}

// StartRuntimeReconciler starts the background stale runtime reconciler.
func (r *Router) StartRuntimeReconciler(ctx context.Context) error {
	if r.runtimeReconciler == nil {
		r.logger.Warn("运行时巡检器未初始化，跳过启动")
		return nil
	}
	if err := r.runtimeReconciler.Start(ctx); err != nil {
		r.logger.Error("启动运行时巡检器失败", logger.Err(err))
		return err
	}
	return nil
}

// StopRuntimeReconciler stops the background stale runtime reconciler.
func (r *Router) StopRuntimeReconciler(ctx context.Context) error {
	if r.runtimeReconciler == nil {
		return nil
	}
	if err := r.runtimeReconciler.Stop(ctx); err != nil {
		r.logger.Error("停止运行时巡检器失败", logger.Err(err))
		return err
	}
	return nil
}
