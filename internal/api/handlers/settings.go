// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"v/internal/api/middleware"
	"v/internal/database"
	"v/internal/database/migrator"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/monitor"
	"v/internal/settings"
	"v/pkg/errors"
)

// SettingsHandler handles settings-related requests.
type SettingsHandler struct {
	logger          logger.Logger
	settingsService *settings.Service
	sourceDB        *gorm.DB // current running DB; used as the source for migrations
	runtimeDBDriver string
	runtimeDBDSN    string
	runtimeDBPath   string
	auditService    monitor.AuditService
	certRepo        repository.CertificateRepository // for ApplyCertificate
	dataDir         string                           // base dir for writing applied panel certs
	validateHook    func(context.Context, *settings.SystemSettings) error
	afterSaveHook   func(context.Context, *settings.SystemSettings) error
	testEmailHook   func(context.Context, *settings.SystemSettings, string) error
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(log logger.Logger, settingsService *settings.Service) *SettingsHandler {
	return &SettingsHandler{
		logger:          log,
		settingsService: settingsService,
	}
}

// WithSourceDB injects the currently-active database. Required for the
// MigrateDatabase endpoint to read rows out of the running DB.
func (h *SettingsHandler) WithSourceDB(db *gorm.DB) *SettingsHandler {
	h.sourceDB = db
	return h
}

// WithRuntimeDatabaseConfig injects the database settings used at process
// startup. The database tab stores target connection fields for test/migrate,
// so manual backups must not infer the active DB from those persisted values.
func (h *SettingsHandler) WithRuntimeDatabaseConfig(driver, dsn, path string) *SettingsHandler {
	h.runtimeDBDriver = strings.TrimSpace(driver)
	h.runtimeDBDSN = strings.TrimSpace(dsn)
	h.runtimeDBPath = strings.TrimSpace(path)
	return h
}

// WithAuditService wires the audit log emitter for operations like
// "settings updated". Optional: without it, those operations don't emit
// audit entries.
func (h *SettingsHandler) WithAuditService(audit monitor.AuditService) *SettingsHandler {
	h.auditService = audit
	return h
}

// WithValidateHook registers an optional settings validation hook.
func (h *SettingsHandler) WithValidateHook(hook func(context.Context, *settings.SystemSettings) error) *SettingsHandler {
	h.validateHook = hook
	return h
}

// WithAfterSaveHook registers an optional hook executed after settings are saved.
func (h *SettingsHandler) WithAfterSaveHook(hook func(context.Context, *settings.SystemSettings) error) *SettingsHandler {
	h.afterSaveHook = hook
	return h
}

// WithTestEmailHook registers an optional hook for sending a test email.
func (h *SettingsHandler) WithTestEmailHook(hook func(context.Context, *settings.SystemSettings, string) error) *SettingsHandler {
	h.testEmailHook = hook
	return h
}

// WithCertificateRepo injects the certificate repository so ApplyCertificate
// can materialize a DB-stored cert+key onto disk for the panel's TLS.
func (h *SettingsHandler) WithCertificateRepo(repo repository.CertificateRepository) *SettingsHandler {
	h.certRepo = repo
	return h
}

// WithDataDir tells ApplyCertificate where to drop the materialized cert/key
// files. Should be a writable volume; defaults to "/app/data" if unset.
func (h *SettingsHandler) WithDataDir(dir string) *SettingsHandler {
	h.dataDir = strings.TrimSpace(dir)
	return h
}

// GetSettings returns all system settings.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	ctx := c.Request.Context()

	systemSettings, err := h.settingsService.GetSystemSettings(ctx)
	if err != nil {
		h.logger.Error("Failed to get settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}
	systemSettings.RuntimeDatabase = h.runtimeDatabaseInfo()

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    systemSettings,
	})
}

// UpdateSettingsRequest represents an update settings request.
type UpdateSettingsRequest struct {
	SiteName            *string `json:"site_name"`
	SiteDescription     *string `json:"site_description"`
	AllowRegistration   *bool   `json:"allow_registration"`
	DefaultTrafficLimit *int64  `json:"default_traffic_limit"`
	DefaultExpiryDays   *int    `json:"default_expiry_days"`
	SessionTimeout      *int    `json:"session_timeout"`
	EnableIPWhitelist   *bool   `json:"enable_ip_whitelist"`
	IPWhitelist         *string `json:"ip_whitelist"`
	EnableLoginLock     *bool   `json:"enable_login_lock"`
	MaxLoginAttempts    *int    `json:"max_login_attempts"`
	LockDuration        *int    `json:"lock_duration"`

	// Panel settings
	PanelAccessIP  *string `json:"panel_access_ip"`
	PanelPort      *int    `json:"panel_port"`
	PanelBasePath  *string `json:"panel_base_path"`
	PublicURL      *string `json:"public_url"`
	CORSOrigins    *string `json:"cors_origins"`
	ProxyMode      *string `json:"proxy_mode"`
	Timezone       *string `json:"timezone"`
	PanelCertPath  *string `json:"panel_cert_path"`
	PanelKeyPath   *string `json:"panel_key_path"`
	PanelAPIDomain *string `json:"panel_api_domain"`

	// DB settings
	DBType     *string `json:"db_type"`
	DBHost     *string `json:"db_host"`
	DBPort     *int    `json:"db_port"`
	DBName     *string `json:"db_name"`
	DBUser     *string `json:"db_user"`
	DBPassword *string `json:"db_password"`
	SQLitePath *string `json:"sqlite_path"`

	// Log settings
	LogLevel           *string `json:"log_level"`
	LogRetentionDays   *int    `json:"log_retention_days"`
	LogPath            *string `json:"log_path"`
	EnableAccessLog    *bool   `json:"enable_access_log"`
	EnableOperationLog *bool   `json:"enable_operation_log"`

	// SMTP settings
	SMTPHost       *string `json:"smtp_host"`
	SMTPPort       *int    `json:"smtp_port"`
	SMTPUser       *string `json:"smtp_user"`
	SMTPFrom       *string `json:"smtp_from"`
	SMTPAlertEmail *string `json:"smtp_alert_email"`
	SMTPPassword   *string `json:"smtp_password"`

	// Telegram settings
	TelegramBotToken *string `json:"telegram_bot_token"`
	TelegramChatID   *string `json:"telegram_chat_id"`

	// Rate limiting
	RateLimitEnabled  *bool `json:"rate_limit_enabled"`
	RateLimitRequests *int  `json:"rate_limit_requests"`
	RateLimitWindow   *int  `json:"rate_limit_window"`

	// Payment settings
	PaymentAlipayEnabled    *bool   `json:"payment_alipay_enabled"`
	PaymentAlipayAppID      *string `json:"payment_alipay_app_id"`
	PaymentAlipayPrivateKey *string `json:"payment_alipay_private_key"`
	PaymentAlipayPublicKey  *string `json:"payment_alipay_public_key"`
	PaymentAlipayNotifyURL  *string `json:"payment_alipay_notify_url"`
	PaymentAlipayReturnURL  *string `json:"payment_alipay_return_url"`
	PaymentAlipaySandbox    *bool   `json:"payment_alipay_sandbox"`
	PaymentWeChatEnabled    *bool   `json:"payment_wechat_enabled"`
	PaymentWeChatAppID      *string `json:"payment_wechat_app_id"`
	PaymentWeChatMchID      *string `json:"payment_wechat_mch_id"`
	PaymentWeChatAPIKey     *string `json:"payment_wechat_api_key"`
	PaymentWeChatNotifyURL  *string `json:"payment_wechat_notify_url"`
	PaymentWeChatSandbox    *bool   `json:"payment_wechat_sandbox"`

	// Xray settings
	XrayConfigTemplate *string `json:"xray_config_template"`
}

func hasPersistedPaymentSettings(values map[string]string) bool {
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
		if _, ok := values[key]; ok {
			return true
		}
	}

	return false
}

func shouldPersistPaymentSettings(req *UpdateSettingsRequest, currentValues map[string]string) bool {
	if req == nil {
		return hasPersistedPaymentSettings(currentValues)
	}

	if req.PaymentAlipayEnabled != nil ||
		req.PaymentAlipayAppID != nil ||
		req.PaymentAlipayPrivateKey != nil ||
		req.PaymentAlipayPublicKey != nil ||
		req.PaymentAlipayNotifyURL != nil ||
		req.PaymentAlipayReturnURL != nil ||
		req.PaymentAlipaySandbox != nil ||
		req.PaymentWeChatEnabled != nil ||
		req.PaymentWeChatAppID != nil ||
		req.PaymentWeChatMchID != nil ||
		req.PaymentWeChatAPIKey != nil ||
		req.PaymentWeChatNotifyURL != nil ||
		req.PaymentWeChatSandbox != nil {
		return true
	}

	return hasPersistedPaymentSettings(currentValues)
}

// UpdateSettings updates system settings.
func (h *SettingsHandler) UpdateSettings(c *gin.Context) {
	ctx := c.Request.Context()

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	currentValues, err := h.settingsService.GetAll(ctx)
	if err != nil {
		h.logger.Error("Failed to get current raw settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}

	// Get current settings
	currentSettings, err := h.settingsService.GetSystemSettings(ctx)
	if err != nil {
		h.logger.Error("Failed to get current settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}

	// Apply updates (only update fields that are provided)
	if req.SiteName != nil {
		currentSettings.SiteName = *req.SiteName
	}
	if req.SiteDescription != nil {
		currentSettings.SiteDescription = *req.SiteDescription
	}
	if req.AllowRegistration != nil {
		currentSettings.AllowRegistration = *req.AllowRegistration
	}
	if req.DefaultTrafficLimit != nil {
		currentSettings.DefaultTrafficLimit = *req.DefaultTrafficLimit
	}
	if req.DefaultExpiryDays != nil {
		currentSettings.DefaultExpiryDays = *req.DefaultExpiryDays
	}
	if req.SessionTimeout != nil {
		currentSettings.SessionTimeout = *req.SessionTimeout
	}
	if req.EnableIPWhitelist != nil {
		currentSettings.EnableIPWhitelist = *req.EnableIPWhitelist
	}
	if req.IPWhitelist != nil {
		currentSettings.IPWhitelist = strings.TrimSpace(*req.IPWhitelist)
	}
	if req.EnableLoginLock != nil {
		currentSettings.EnableLoginLock = *req.EnableLoginLock
	}
	if req.MaxLoginAttempts != nil {
		currentSettings.MaxLoginAttempts = *req.MaxLoginAttempts
	}
	if req.LockDuration != nil {
		currentSettings.LockDuration = *req.LockDuration
	}
	// Panel settings
	if req.PanelAccessIP != nil {
		currentSettings.PanelAccessIP = strings.TrimSpace(*req.PanelAccessIP)
	}
	if req.PanelPort != nil {
		currentSettings.PanelPort = *req.PanelPort
	}
	if req.PanelBasePath != nil {
		basePath, normErr := normalizePanelBasePath(*req.PanelBasePath)
		if normErr != nil {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"panel_base_path": normErr.Error(),
			}))
			return
		}
		currentSettings.PanelBasePath = basePath
	}
	if req.PublicURL != nil {
		publicURL, normErr := normalizePublicURL(*req.PublicURL)
		if normErr != nil {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"public_url": normErr.Error(),
			}))
			return
		}
		currentSettings.PublicURL = publicURL
	}
	if req.CORSOrigins != nil {
		corsOrigins, normErr := normalizeCORSOrigins(*req.CORSOrigins)
		if normErr != nil {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"cors_origins": normErr.Error(),
			}))
			return
		}
		currentSettings.CORSOrigins = corsOrigins
	}
	if req.ProxyMode != nil {
		currentSettings.ProxyMode = strings.TrimSpace(*req.ProxyMode)
	}
	if req.Timezone != nil {
		currentSettings.Timezone = strings.TrimSpace(*req.Timezone)
	}
	if req.PanelCertPath != nil {
		currentSettings.PanelCertPath = strings.TrimSpace(*req.PanelCertPath)
	}
	if req.PanelKeyPath != nil {
		currentSettings.PanelKeyPath = strings.TrimSpace(*req.PanelKeyPath)
	}
	if req.PanelAPIDomain != nil {
		currentSettings.PanelAPIDomain = *req.PanelAPIDomain
	}
	if req.DBType != nil {
		currentSettings.DBType = strings.TrimSpace(*req.DBType)
	}
	if req.DBHost != nil {
		currentSettings.DBHost = strings.TrimSpace(*req.DBHost)
	}
	if req.DBPort != nil {
		currentSettings.DBPort = *req.DBPort
	}
	if req.DBName != nil {
		currentSettings.DBName = strings.TrimSpace(*req.DBName)
	}
	if req.DBUser != nil {
		currentSettings.DBUser = strings.TrimSpace(*req.DBUser)
	}
	if req.DBPassword != nil {
		currentSettings.DBPassword = *req.DBPassword
	}
	if req.SQLitePath != nil {
		currentSettings.SQLitePath = strings.TrimSpace(*req.SQLitePath)
	}
	if req.LogLevel != nil {
		currentSettings.LogLevel = strings.TrimSpace(*req.LogLevel)
	}
	if req.LogRetentionDays != nil {
		currentSettings.LogRetentionDays = *req.LogRetentionDays
	}
	if req.LogPath != nil {
		currentSettings.LogPath = strings.TrimSpace(*req.LogPath)
	}
	if req.EnableAccessLog != nil {
		currentSettings.EnableAccessLog = *req.EnableAccessLog
	}
	if req.EnableOperationLog != nil {
		currentSettings.EnableOperationLog = *req.EnableOperationLog
	}
	if req.SMTPHost != nil {
		currentSettings.SMTPHost = *req.SMTPHost
	}
	if req.SMTPPort != nil {
		currentSettings.SMTPPort = *req.SMTPPort
	}
	if req.SMTPUser != nil {
		currentSettings.SMTPUser = *req.SMTPUser
	}
	if req.SMTPFrom != nil {
		currentSettings.SMTPFrom = *req.SMTPFrom
	}
	if req.SMTPAlertEmail != nil {
		currentSettings.SMTPAlertEmail = *req.SMTPAlertEmail
	}
	if req.SMTPPassword != nil {
		currentSettings.SMTPPassword = *req.SMTPPassword
	}
	if req.TelegramBotToken != nil {
		currentSettings.TelegramBotToken = *req.TelegramBotToken
	}
	if req.TelegramChatID != nil {
		currentSettings.TelegramChatID = *req.TelegramChatID
	}
	if req.RateLimitEnabled != nil {
		currentSettings.RateLimitEnabled = *req.RateLimitEnabled
	}
	if req.RateLimitRequests != nil {
		currentSettings.RateLimitRequests = *req.RateLimitRequests
	}
	if req.RateLimitWindow != nil {
		currentSettings.RateLimitWindow = *req.RateLimitWindow
	}
	if req.PaymentAlipayEnabled != nil {
		currentSettings.PaymentAlipayEnabled = *req.PaymentAlipayEnabled
	}
	if req.PaymentAlipayAppID != nil {
		currentSettings.PaymentAlipayAppID = *req.PaymentAlipayAppID
	}
	if req.PaymentAlipayPrivateKey != nil {
		currentSettings.PaymentAlipayPrivateKey = *req.PaymentAlipayPrivateKey
	}
	if req.PaymentAlipayPublicKey != nil {
		currentSettings.PaymentAlipayPublicKey = *req.PaymentAlipayPublicKey
	}
	if req.PaymentAlipayNotifyURL != nil {
		currentSettings.PaymentAlipayNotifyURL = *req.PaymentAlipayNotifyURL
	}
	if req.PaymentAlipayReturnURL != nil {
		currentSettings.PaymentAlipayReturnURL = *req.PaymentAlipayReturnURL
	}
	if req.PaymentAlipaySandbox != nil {
		currentSettings.PaymentAlipaySandbox = *req.PaymentAlipaySandbox
	}
	if req.PaymentWeChatEnabled != nil {
		currentSettings.PaymentWeChatEnabled = *req.PaymentWeChatEnabled
	}
	if req.PaymentWeChatAppID != nil {
		currentSettings.PaymentWeChatAppID = *req.PaymentWeChatAppID
	}
	if req.PaymentWeChatMchID != nil {
		currentSettings.PaymentWeChatMchID = *req.PaymentWeChatMchID
	}
	if req.PaymentWeChatAPIKey != nil {
		currentSettings.PaymentWeChatAPIKey = *req.PaymentWeChatAPIKey
	}
	if req.PaymentWeChatNotifyURL != nil {
		currentSettings.PaymentWeChatNotifyURL = *req.PaymentWeChatNotifyURL
	}
	if req.PaymentWeChatSandbox != nil {
		currentSettings.PaymentWeChatSandbox = *req.PaymentWeChatSandbox
	}
	if req.XrayConfigTemplate != nil {
		currentSettings.XrayConfigTemplate = *req.XrayConfigTemplate
	}

	currentSettings.SMTPPasswordConfigured = strings.TrimSpace(currentSettings.SMTPPassword) != ""
	currentSettings.PaymentAlipayPrivateKeyConfigured = strings.TrimSpace(currentSettings.PaymentAlipayPrivateKey) != ""
	currentSettings.PaymentWeChatAPIKeyConfigured = strings.TrimSpace(currentSettings.PaymentWeChatAPIKey) != ""

	if currentSettings.SessionTimeout <= 0 {
		middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
			"session_timeout": "session timeout must be greater than 0 minutes",
		}))
		return
	}

	if err := validatePanelSettings(currentSettings); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	if err := validateIPWhitelist(currentSettings.IPWhitelist); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
			"ip_whitelist": err.Error(),
		}))
		return
	}

	if currentSettings.EnableIPWhitelist && len(splitIPWhitelist(currentSettings.IPWhitelist)) == 0 {
		middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
			"ip_whitelist": "at least one IP or CIDR entry is required when IP whitelist is enabled",
		}))
		return
	}

	if currentSettings.EnableLoginLock {
		if currentSettings.MaxLoginAttempts <= 0 {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"max_login_attempts": "max login attempts must be greater than 0",
			}))
			return
		}
		if currentSettings.LockDuration <= 0 {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"lock_duration": "lock duration must be greater than 0 minutes",
			}))
			return
		}
	}

	if h.validateHook != nil {
		if err := h.validateHook(ctx, currentSettings); err != nil {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"error": err.Error(),
			}))
			return
		}
	}

	// Save updated settings
	if err := h.settingsService.UpdateSystemSettingsWithOptions(ctx, currentSettings, settings.UpdateOptions{
		IncludePaymentSettings: shouldPersistPaymentSettings(&req, currentValues),
	}); err != nil {
		h.logger.Error("Failed to update settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("update settings", err))
		return
	}

	if h.afterSaveHook != nil {
		if err := h.afterSaveHook(ctx, currentSettings); err != nil {
			h.logger.Error("Failed to apply settings after save", logger.F("error", err))
			middleware.RespondWithError(c, errors.NewInternalError("apply settings", err))
			return
		}
	}

	h.logger.Info("Settings updated")

	if h.auditService != nil && h.auditService.Enabled() {
		actorID, _ := currentUserIDFromContext(c)
		_ = h.auditService.Log(ctx, &monitor.AuditEntry{
			UserID:       &actorID,
			Username:     c.GetString("username"),
			Action:       monitor.ActionSettingsUpdate,
			ResourceType: monitor.ResourceSettings,
			IPAddress:    c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			RequestID:    c.GetString("request_id"),
			Status:       monitor.StatusSuccess,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "settings updated",
		"data":    currentSettings,
	})
}

type TestDatabaseRequest struct {
	DBType     string `json:"db_type"`
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBName     string `json:"db_name"`
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"`
	SQLitePath string `json:"sqlite_path"`
}

func buildDatabaseTestConfig(req TestDatabaseRequest) (*database.Config, error) {
	driver := strings.ToLower(strings.TrimSpace(req.DBType))
	if driver == "" {
		driver = "sqlite"
	}

	cfg := &database.Config{Driver: driver}
	switch driver {
	case "sqlite":
		path := strings.TrimSpace(req.SQLitePath)
		if path == "" {
			return nil, fmt.Errorf("sqlite target path is required")
		}
		cfg.DSN = path
	case "mysql":
		host := strings.TrimSpace(req.DBHost)
		if host == "" {
			host = "localhost"
		}
		port := req.DBPort
		if port <= 0 {
			port = 3306
		}
		name := strings.TrimSpace(req.DBName)
		if name == "" {
			return nil, fmt.Errorf("database name is required")
		}
		cfg.DSN = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			strings.TrimSpace(req.DBUser),
			req.DBPassword,
			host,
			port,
			name,
		)
	case "postgres", "postgresql":
		host := strings.TrimSpace(req.DBHost)
		if host == "" {
			host = "localhost"
		}
		port := req.DBPort
		if port <= 0 {
			port = 5432
		}
		name := strings.TrimSpace(req.DBName)
		if name == "" {
			return nil, fmt.Errorf("database name is required")
		}
		cfg.Driver = "postgres"
		cfg.DSN = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			host,
			port,
			strings.TrimSpace(req.DBUser),
			req.DBPassword,
			name,
		)
	default:
		return nil, fmt.Errorf("unsupported database driver")
	}

	return cfg, nil
}

func (h *SettingsHandler) runtimeDatabaseInfo() *settings.RuntimeDatabaseInfo {
	driver := strings.ToLower(strings.TrimSpace(h.runtimeDBDriver))
	if driver == "" {
		driver = "sqlite"
	}
	if driver == "sqlite3" {
		driver = "sqlite"
	}

	info := &settings.RuntimeDatabaseInfo{Driver: driver}
	dsn := strings.TrimSpace(h.runtimeDBDSN)
	path := strings.TrimSpace(h.runtimeDBPath)

	if driver == "sqlite" {
		if parsed, err := sqliteBackupPathFromDSN(dsn); err == nil && parsed != "" {
			info.Path = parsed
		} else if path != "" {
			info.Path = path
		} else if dsn != "" {
			info.Path = dsn
		} else {
			info.Path = "./data/v.db"
		}
		return info
	}

	if dsn != "" {
		info.DSNMasked = maskDSN(dsn)
	}
	return info
}

func sameFilePath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}

	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil && errB == nil {
		return filepath.Clean(absA) == filepath.Clean(absB)
	}

	return filepath.Clean(a) == filepath.Clean(b)
}

func (h *SettingsHandler) validateMigrationTarget(cfg *database.Config) error {
	if cfg == nil {
		return fmt.Errorf("target database config is required")
	}
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver != "sqlite" && driver != "sqlite3" {
		return nil
	}

	targetPath, err := sqliteBackupPathFromDSN(cfg.DSN)
	if err != nil {
		return err
	}
	if targetPath == "" {
		return fmt.Errorf("sqlite target path is required")
	}

	sourcePath, ok, err := h.currentSQLiteDatabasePath(nil)
	if err != nil || !ok || sourcePath == "" {
		return err
	}
	if sameFilePath(sourcePath, targetPath) {
		return fmt.Errorf("target SQLite path must be different from the currently-running database")
	}

	return nil
}

func normalizePanelBasePath(value string) (string, error) {
	basePath := strings.TrimSpace(value)
	if basePath == "" || basePath == "/" {
		return "/", nil
	}
	if strings.Contains(basePath, "://") {
		return "", fmt.Errorf("base path must be a path like /vpanel, not a full URL")
	}
	if strings.ContainsAny(basePath, " \t\r\n?#") {
		return "", fmt.Errorf("base path must not contain whitespace, query strings, or fragments")
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")
	if basePath == "" {
		return "/", nil
	}
	if strings.Contains(basePath, "//") {
		return "", fmt.Errorf("base path must not contain repeated slashes")
	}
	return basePath, nil
}

func normalizePublicURL(value string) (string, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("public URL must be a complete URL like https://panel.example.com")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("public URL scheme must be http or https")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("public URL must not include user info, query strings, or fragments")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("public URL must be an origin only; use panel base path for URL paths")
	}
	return strings.TrimSuffix(parsed.Scheme+"://"+parsed.Host, "/"), nil
}

func normalizeCORSOrigins(value string) (string, error) {
	raw := strings.ReplaceAll(value, "\n", ",")
	parts := strings.Split(raw, ",")
	normalized := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if origin == "*" {
			if _, ok := seen[origin]; !ok {
				seen[origin] = struct{}{}
				normalized = append(normalized, origin)
			}
			continue
		}
		publicURL, err := normalizePublicURL(origin)
		if err != nil {
			return "", fmt.Errorf("invalid CORS origin %q: %w", origin, err)
		}
		if _, ok := seen[publicURL]; ok {
			continue
		}
		seen[publicURL] = struct{}{}
		normalized = append(normalized, publicURL)
	}
	return strings.Join(normalized, ", "), nil
}

func validatePanelSettings(systemSettings *settings.SystemSettings) error {
	if systemSettings == nil {
		return nil
	}

	if systemSettings.PanelPort < 1 || systemSettings.PanelPort > 65535 {
		return fmt.Errorf("panel port must be between 1 and 65535")
	}

	basePath, err := normalizePanelBasePath(systemSettings.PanelBasePath)
	if err != nil {
		return err
	}
	systemSettings.PanelBasePath = basePath

	publicURL, err := normalizePublicURL(systemSettings.PublicURL)
	if err != nil {
		return err
	}
	systemSettings.PublicURL = publicURL

	corsOrigins, err := normalizeCORSOrigins(systemSettings.CORSOrigins)
	if err != nil {
		return err
	}
	systemSettings.CORSOrigins = corsOrigins

	certPath := strings.TrimSpace(systemSettings.PanelCertPath)
	keyPath := strings.TrimSpace(systemSettings.PanelKeyPath)
	if (certPath == "") != (keyPath == "") {
		return fmt.Errorf("panel TLS cert path and key path must be provided together")
	}
	systemSettings.PanelCertPath = certPath
	systemSettings.PanelKeyPath = keyPath

	if tz := strings.TrimSpace(systemSettings.Timezone); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return fmt.Errorf("timezone is invalid: %w", err)
		}
		systemSettings.Timezone = tz
	}

	return nil
}

func sqliteBackupPathFromDSN(dsn string) (string, error) {
	value := strings.TrimSpace(dsn)
	if value == "" {
		return "", nil
	}
	if value == ":memory:" || strings.Contains(value, "mode=memory") {
		return "", fmt.Errorf("in-memory SQLite databases cannot be backed up by file copy")
	}

	if strings.HasPrefix(value, "file:") {
		value = strings.TrimPrefix(value, "file:")
	}
	if idx := strings.Index(value, "?"); idx >= 0 {
		value = value[:idx]
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("SQLite DSN does not include a file path")
	}
	if value == ":memory:" {
		return "", fmt.Errorf("in-memory SQLite databases cannot be backed up by file copy")
	}
	return value, nil
}

func (h *SettingsHandler) currentSQLiteDatabasePath(settingsValue *settings.SystemSettings) (string, bool, error) {
	driver := strings.ToLower(strings.TrimSpace(h.runtimeDBDriver))
	if driver == "" && settingsValue != nil {
		driver = strings.ToLower(strings.TrimSpace(settingsValue.DBType))
	}
	if driver == "" {
		driver = "sqlite"
	}
	if driver != "sqlite" && driver != "sqlite3" {
		return "", false, nil
	}

	for _, candidate := range []string{h.runtimeDBDSN, h.runtimeDBPath} {
		path, err := sqliteBackupPathFromDSN(candidate)
		if err != nil {
			return "", true, err
		}
		if path != "" {
			return path, true, nil
		}
	}

	if settingsValue != nil {
		path, err := sqliteBackupPathFromDSN(settingsValue.SQLitePath)
		if err != nil {
			return "", true, err
		}
		if path != "" {
			return path, true, nil
		}
	}

	return "./data/v.db", true, nil
}

// TestDatabase tests a database connection using the provided settings.
func (h *SettingsHandler) TestDatabase(c *gin.Context) {
	var req TestDatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	cfg, err := buildDatabaseTestConfig(req)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid database config", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	db, err := database.New(cfg)
	if err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("test database connection", err))
		return
	}
	defer db.Close()

	sqlDB, err := db.DB().DB()
	if err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("open database handle", err))
		return
	}
	if err := sqlDB.PingContext(c.Request.Context()); err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("ping database", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "database connection successful",
	})
}

// BackupDatabase creates a backup of the currently-running SQLite database file.
func (h *SettingsHandler) BackupDatabase(c *gin.Context) {
	settingsValue, err := h.settingsService.GetSystemSettings(c.Request.Context())
	if err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}

	sourcePath, ok, pathErr := h.currentSQLiteDatabasePath(settingsValue)
	if pathErr != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid database config", map[string]interface{}{
			"error": pathErr.Error(),
		}))
		return
	}
	if !ok {
		c.JSON(http.StatusNotImplemented, gin.H{
			"error": "Only the currently-running SQLite database can be backed up from this page",
		})
		return
	}

	if _, statErr := os.Stat(sourcePath); statErr != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("stat database file", statErr))
		return
	}

	backupDir := filepath.Join(filepath.Dir(sourcePath), "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		middleware.RespondWithError(c, errors.NewInternalError("create backup directory", err))
		return
	}

	backupName := fmt.Sprintf("vpanel_db_%s.db", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(backupDir, backupName)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		middleware.RespondWithError(c, errors.NewInternalError("open source database", err))
		return
	}
	defer sourceFile.Close()

	backupFile, err := os.Create(backupPath)
	if err != nil {
		middleware.RespondWithError(c, errors.NewInternalError("create backup file", err))
		return
	}
	defer backupFile.Close()

	if _, err := io.Copy(backupFile, sourceFile); err != nil {
		middleware.RespondWithError(c, errors.NewInternalError("copy database backup", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":        200,
		"message":     "database backup created",
		"backup_path": backupPath,
	})
}

// MigrateDatabaseRequest carries the target connection plus an explicit
// confirmation flag so a misclick can't trigger a one-way migration.
type MigrateDatabaseRequest struct {
	TestDatabaseRequest
	// Confirm must be true for the migration to run. The UI surfaces this as
	// a checkbox in the confirmation dialog.
	Confirm bool `json:"confirm"`
}

// MigrateDatabase copies every table from the currently-running database to
// the target database described in the request. It does NOT switch the
// running process to the new DB — the operator must update their config
// (V_DB_DRIVER / V_DB_DSN) and restart manually.
//
// The endpoint is intentionally synchronous: GORM's INSERTs over the wire
// are slow but the data volume in a typical V Panel install (users + traffic
// rows for a small fleet) is small enough that this completes in seconds.
// For larger installs the request will block until done; the front-end shows
// a "正在迁移..." spinner.
func (h *SettingsHandler) MigrateDatabase(c *gin.Context) {
	if h.sourceDB == nil {
		middleware.RespondWithError(c, errors.NewInternalError("source database not wired into settings handler", nil))
		return
	}

	var req MigrateDatabaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}
	if !req.Confirm {
		middleware.RespondWithError(c, errors.NewValidationError("confirmation required", map[string]interface{}{
			"confirm": "must be true to run a database migration",
		}))
		return
	}

	cfg, err := buildDatabaseTestConfig(req.TestDatabaseRequest)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid target database config", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}
	if err := h.validateMigrationTarget(cfg); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid target database config", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	tgt, err := database.New(cfg)
	if err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("open target database", err))
		return
	}
	defer tgt.Close()

	report, err := migrator.Migrate(c.Request.Context(), h.sourceDB, tgt.DB(), database.AllModels(), 500)
	if err != nil {
		h.logger.Error("database migration failed", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("migrate database", err))
		return
	}

	// Build a human-readable instruction so the operator knows how to actually
	// cut over (we never auto-switch the running process).
	envInstruction := buildCutoverInstruction(cfg)

	h.logger.Info("database migration completed",
		logger.F("driver", cfg.Driver),
		logger.F("total_rows", report.TotalRows),
		logger.F("tables", report.TableCount),
	)

	c.JSON(http.StatusOK, gin.H{
		"code":              200,
		"message":           "Data migrated to target database. Restart the panel with the new connection to start using it.",
		"report":            report,
		"cutover_env":       envInstruction,
		"target_driver":     cfg.Driver,
		"target_dsn_masked": maskDSN(cfg.DSN),
	})
}

// buildCutoverInstruction returns the env vars the operator should set on the
// next restart to actually use the target database. Returning structured data
// lets the UI render copy buttons.
func buildCutoverInstruction(cfg *database.Config) map[string]string {
	return map[string]string{
		"V_DB_DRIVER": cfg.Driver,
		"V_DB_DSN":    cfg.DSN,
	}
}

// maskDSN hides credentials in a connection string when echoing back to the
// caller. We accept some noise (the URL/keyword forms differ between drivers)
// in exchange for being defensive in all of them.
func maskDSN(dsn string) string {
	// Mask password in MySQL's user:password@tcp(...) form.
	if i := strings.Index(dsn, "@tcp("); i > 0 {
		if j := strings.Index(dsn[:i], ":"); j > 0 {
			return dsn[:j+1] + "***" + dsn[i:]
		}
	}
	// Mask password= in PostgreSQL keyword form.
	if i := strings.Index(dsn, "password="); i >= 0 {
		end := strings.Index(dsn[i:], " ")
		if end < 0 {
			return dsn[:i+len("password=")] + "***"
		}
		return dsn[:i+len("password=")] + "***" + dsn[i+end:]
	}
	return dsn
}

// TestEmailRequest represents a test email request.
type TestEmailRequest struct {
	To string `json:"to"`
}

// TestEmail sends a test email using the current SMTP configuration.
func (h *SettingsHandler) TestEmail(c *gin.Context) {
	if h.testEmailHook == nil {
		middleware.RespondWithError(c, errors.NewInternalError("test email unavailable", nil))
		return
	}

	ctx := c.Request.Context()

	var req TestEmailRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
			middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
				"error": err.Error(),
			}))
			return
		}
	}

	systemSettings, err := h.settingsService.GetSystemSettings(ctx)
	if err != nil {
		h.logger.Error("Failed to get settings for test email", logger.Err(err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}

	if err := h.testEmailHook(ctx, systemSettings, req.To); err != nil {
		h.logger.Error("Failed to send test email", logger.Err(err))
		lowerMessage := strings.ToLower(err.Error())
		if strings.Contains(lowerMessage, "smtp") ||
			strings.Contains(lowerMessage, "recipient") ||
			strings.Contains(lowerMessage, "invalid") ||
			strings.Contains(lowerMessage, "required") {
			middleware.RespondWithError(c, errors.NewValidationError("invalid email settings", map[string]interface{}{
				"error": err.Error(),
			}))
			return
		}
		middleware.RespondWithError(c, errors.NewInternalError("send test email", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "测试邮件发送成功",
	})
}

// BackupSettings creates a backup of all settings.
func (h *SettingsHandler) BackupSettings(c *gin.Context) {
	ctx := c.Request.Context()

	backup, err := h.settingsService.Backup(ctx)
	if err != nil {
		h.logger.Error("Failed to backup settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("backup settings", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "backup created",
		"data": gin.H{
			"backup": string(backup),
		},
	})
}

// RestoreSettingsRequest represents a restore settings request.
type RestoreSettingsRequest struct {
	Backup string `json:"backup" binding:"required"`
}

// RestoreSettings restores settings from a backup.
func (h *SettingsHandler) RestoreSettings(c *gin.Context) {
	ctx := c.Request.Context()

	var req RestoreSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	if err := h.settingsService.Restore(ctx, []byte(req.Backup)); err != nil {
		h.logger.Error("Failed to restore settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("restore settings", err))
		return
	}

	h.logger.Info("Settings restored from backup")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "settings restored",
	})
}

// XraySettingsRequest represents Xray settings request.
type XraySettingsRequest struct {
	AutoUpdate    bool   `json:"auto_update"`
	CustomConfig  bool   `json:"custom_config"`
	ConfigPath    string `json:"config_path"`
	CheckInterval int    `json:"check_interval"`
}

const (
	xraySettingsKey    = "xray_settings"
	xrayProtocolsKey   = "xray_protocol_settings"
	defaultCheckWindow = 24
)

func defaultXraySettings() XraySettingsRequest {
	return XraySettingsRequest{
		AutoUpdate:    false,
		CustomConfig:  false,
		ConfigPath:    "",
		CheckInterval: defaultCheckWindow,
	}
}

// GetXraySettings returns Xray-specific settings.
func (h *SettingsHandler) GetXraySettings(c *gin.Context) {
	settings := defaultXraySettings()
	if err := h.settingsService.GetTyped(c.Request.Context(), xraySettingsKey, &settings); err != nil {
		h.logger.Error("Failed to load Xray settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get xray settings", err))
		return
	}
	if settings.CheckInterval <= 0 {
		settings.CheckInterval = defaultCheckWindow
	}

	c.JSON(http.StatusOK, gin.H{
		"auto_update":    settings.AutoUpdate,
		"custom_config":  settings.CustomConfig,
		"config_path":    settings.ConfigPath,
		"check_interval": settings.CheckInterval,
	})
}

// UpdateXraySettings updates Xray-specific settings.
func (h *SettingsHandler) UpdateXraySettings(c *gin.Context) {
	var req XraySettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	if req.CheckInterval <= 0 {
		req.CheckInterval = defaultCheckWindow
	}

	data, err := json.Marshal(req)
	if err != nil {
		h.logger.Error("Failed to encode Xray settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("encode xray settings", err))
		return
	}

	if err := h.settingsService.Set(c.Request.Context(), xraySettingsKey, string(data)); err != nil {
		h.logger.Error("Failed to save Xray settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("save xray settings", err))
		return
	}

	h.logger.Info("Xray settings updated",
		logger.F("auto_update", req.AutoUpdate),
		logger.F("custom_config", req.CustomConfig))

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Xray settings updated",
		"auto_update":    req.AutoUpdate,
		"custom_config":  req.CustomConfig,
		"config_path":    req.ConfigPath,
		"check_interval": req.CheckInterval,
	})
}

// ProtocolSettingsRequest represents protocol settings request.
type ProtocolSettingsRequest struct {
	Protocols  map[string]bool `json:"protocols"`
	Transports map[string]bool `json:"transports"`
}

func defaultProtocolSettings() ProtocolSettingsRequest {
	return ProtocolSettingsRequest{
		Protocols: map[string]bool{
			"trojan":      true,
			"vmess":       true,
			"vless":       true,
			"shadowsocks": true,
			"socks":       false,
			"http":        false,
		},
		Transports: map[string]bool{
			"tcp":   true,
			"ws":    true,
			"http2": true,
			"grpc":  true,
			"quic":  false,
		},
	}
}

func mergeBoolSettings(defaults map[string]bool, overrides map[string]bool) map[string]bool {
	merged := make(map[string]bool, len(defaults))
	for key, value := range defaults {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

// GetProtocolSettings returns protocol settings.
func (h *SettingsHandler) GetProtocolSettings(c *gin.Context) {
	settings := defaultProtocolSettings()
	var stored ProtocolSettingsRequest
	if err := h.settingsService.GetTyped(c.Request.Context(), xrayProtocolsKey, &stored); err != nil {
		h.logger.Error("Failed to load protocol settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get protocol settings", err))
		return
	}
	settings.Protocols = mergeBoolSettings(settings.Protocols, stored.Protocols)
	settings.Transports = mergeBoolSettings(settings.Transports, stored.Transports)

	c.JSON(http.StatusOK, gin.H{
		"protocols":  settings.Protocols,
		"transports": settings.Transports,
	})
}

// UpdateProtocolSettings updates protocol settings.
func (h *SettingsHandler) UpdateProtocolSettings(c *gin.Context) {
	var req ProtocolSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	settings := defaultProtocolSettings()
	settings.Protocols = mergeBoolSettings(settings.Protocols, req.Protocols)
	settings.Transports = mergeBoolSettings(settings.Transports, req.Transports)

	data, err := json.Marshal(settings)
	if err != nil {
		h.logger.Error("Failed to encode protocol settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("encode protocol settings", err))
		return
	}

	if err := h.settingsService.Set(c.Request.Context(), xrayProtocolsKey, string(data)); err != nil {
		h.logger.Error("Failed to save protocol settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("save protocol settings", err))
		return
	}

	h.logger.Info("Protocol settings updated")

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Protocol settings updated",
		"protocols":  settings.Protocols,
		"transports": settings.Transports,
	})
}

// GetAutoProxySettings returns automatic proxy provisioning settings.
func (h *SettingsHandler) GetAutoProxySettings(c *gin.Context) {
	settings, err := h.settingsService.GetAutoProxySettings(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to load auto proxy settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get auto proxy settings", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"protocol_priority": settings.ProtocolPriority,
	})
}

// UpdateAutoProxySettings updates automatic proxy provisioning settings.
func (h *SettingsHandler) UpdateAutoProxySettings(c *gin.Context) {
	var req settings.AutoProxySettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid request body",
		})
		return
	}

	next, err := h.settingsService.UpdateAutoProxySettings(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.logger.Info("Auto proxy settings updated",
		logger.F("protocol_priority", strings.Join(next.ProtocolPriority, ",")))

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "Auto proxy settings updated",
		"protocol_priority": next.ProtocolPriority,
	})
}

// ApplyPanelCertificateRequest is the body for POST /settings/apply-certificate.
type ApplyPanelCertificateRequest struct {
	CertificateID int64 `json:"certificate_id" binding:"required,min=1"`
}

// ApplyCertificate materializes a DB-stored certificate to disk and writes
// its paths into the panel's settings (panel_cert_path / panel_key_path).
// The change takes effect on the next panel restart — applyStartupOverrides
// in cmd/v/main.go reads these two keys.
//
// Why we write to disk: cfg.Server.TLSCert/TLSKey expect filesystem paths.
// The certificate manager stores PEM content in the DB (Certificate.Certificate
// + Certificate.PrivateKey), so this endpoint bridges the two. /app/certs is
// mounted read-only in docker-compose, so we drop files under the writable
// data volume (/app/data/panel-certs/) instead.
func (h *SettingsHandler) ApplyCertificate(c *gin.Context) {
	if h.certRepo == nil {
		middleware.RespondWithError(c, errors.NewInternalError("certificate repo not configured", nil))
		return
	}

	var req ApplyPanelCertificateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"detail": err.Error(),
		}))
		return
	}

	ctx := c.Request.Context()
	cert, err := h.certRepo.GetByID(ctx, req.CertificateID)
	if err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("get certificate", err))
		return
	}
	if cert == nil {
		middleware.RespondWithError(c, errors.NewNotFoundError("certificate", req.CertificateID))
		return
	}

	dataDir := h.dataDir
	if dataDir == "" {
		dataDir = "/app/data"
	}

	// Resolve the actual cert/key file paths to write into settings DB.
	// Two sources to support:
	//   (a) acme.sh-issued cert — file is already on disk at cert.CertPath
	//       (relative path stored by the cert service, resolved against the
	//       container's WORKDIR /app). PEM bytes are NOT in the DB row.
	//   (b) uploaded cert — PEM content is in cert.Certificate / cert.PrivateKey,
	//       no file on disk yet, so we materialize it under /app/data/panel-certs.
	var certPath, keyPath string

	switch {
	case strings.TrimSpace(cert.CertPath) != "" && strings.TrimSpace(cert.KeyPath) != "":
		certPath = resolveCertPath(cert.CertPath)
		keyPath = resolveCertPath(cert.KeyPath)
		if _, statErr := os.Stat(certPath); statErr != nil {
			middleware.RespondWithError(c, errors.NewValidationError("certificate file not readable on disk", map[string]interface{}{
				"certificate_id": cert.ID,
				"domain":         cert.Domain,
				"cert_path":      certPath,
				"detail":         statErr.Error(),
			}))
			return
		}
		if _, statErr := os.Stat(keyPath); statErr != nil {
			middleware.RespondWithError(c, errors.NewValidationError("private key file not readable on disk", map[string]interface{}{
				"certificate_id": cert.ID,
				"domain":         cert.Domain,
				"key_path":       keyPath,
				"detail":         statErr.Error(),
			}))
			return
		}

	case strings.TrimSpace(cert.Certificate) != "" && strings.TrimSpace(cert.PrivateKey) != "":
		panelCertsDir := filepath.Join(dataDir, "panel-certs")
		if err := os.MkdirAll(panelCertsDir, 0o700); err != nil {
			middleware.RespondWithError(c, errors.NewInternalError("create panel-certs dir", err))
			return
		}
		certPath = filepath.Join(panelCertsDir, fmt.Sprintf("cert-%d.pem", cert.ID))
		keyPath = filepath.Join(panelCertsDir, fmt.Sprintf("cert-%d.key", cert.ID))
		if err := os.WriteFile(certPath, []byte(cert.Certificate), 0o600); err != nil {
			middleware.RespondWithError(c, errors.NewInternalError("write certificate file", err))
			return
		}
		if err := os.WriteFile(keyPath, []byte(cert.PrivateKey), 0o600); err != nil {
			_ = os.Remove(certPath)
			middleware.RespondWithError(c, errors.NewInternalError("write private key file", err))
			return
		}

	default:
		middleware.RespondWithError(c, errors.NewValidationError("certificate has neither files on disk nor PEM content (not yet issued?)", map[string]interface{}{
			"certificate_id": cert.ID,
			"domain":         cert.Domain,
		}))
		return
	}

	if err := h.settingsService.SetMultiple(ctx, map[string]string{
		"panel_cert_path": certPath,
		"panel_key_path":  keyPath,
	}); err != nil {
		middleware.RespondWithError(c, errors.NewDatabaseError("persist cert paths", err))
		return
	}

	h.logger.Info("panel certificate applied from manager",
		logger.F("certificate_id", cert.ID),
		logger.F("domain", cert.Domain),
		logger.F("cert_path", certPath))

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "证书已应用，重启面板后生效",
		"data": gin.H{
			"certificate_id": cert.ID,
			"domain":         cert.Domain,
			"cert_path":      certPath,
			"key_path":       keyPath,
			"expires_at":     cert.ExpiresAt,
		},
	})
}

// resolveCertPath turns the cert service's stored path into an absolute path
// usable by the panel HTTP server at startup. The cert service stores paths
// relative to the container WORKDIR (/app), e.g.
// "data/certificates/example.com/fullchain.pem"; anchor those to /app so
// startup (running with WORKDIR possibly elsewhere) can still find them.
// Absolute paths are returned unchanged.
func resolveCertPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" || filepath.IsAbs(p) {
		return p
	}
	return filepath.Join("/app", p)
}
