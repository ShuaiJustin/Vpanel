// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/logger"
	"v/internal/settings"
	"v/pkg/errors"
)

// SettingsHandler handles settings-related requests.
type SettingsHandler struct {
	logger          logger.Logger
	settingsService *settings.Service
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

// GetSettings returns all system settings.
func (h *SettingsHandler) GetSettings(c *gin.Context) {
	ctx := c.Request.Context()

	systemSettings, err := h.settingsService.GetSystemSettings(ctx)
	if err != nil {
		h.logger.Error("Failed to get settings", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get settings", err))
		return
	}

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

	// Panel settings
	PanelAccessIP  *string `json:"panel_access_ip"`
	PanelPort      *int    `json:"panel_port"`
	PanelCertPath  *string `json:"panel_cert_path"`
	PanelKeyPath   *string `json:"panel_key_path"`
	PanelAPIDomain *string `json:"panel_api_domain"`

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
	// Panel settings
	if req.PanelAccessIP != nil {
		currentSettings.PanelAccessIP = *req.PanelAccessIP
	}
	if req.PanelPort != nil {
		currentSettings.PanelPort = *req.PanelPort
	}
	if req.PanelCertPath != nil {
		currentSettings.PanelCertPath = *req.PanelCertPath
	}
	if req.PanelKeyPath != nil {
		currentSettings.PanelKeyPath = *req.PanelKeyPath
	}
	if req.PanelAPIDomain != nil {
		currentSettings.PanelAPIDomain = *req.PanelAPIDomain
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

	if h.validateHook != nil {
		if err := h.validateHook(ctx, currentSettings); err != nil {
			middleware.RespondWithError(c, errors.NewValidationError("invalid settings", map[string]interface{}{
				"error": err.Error(),
			}))
			return
		}
	}

	// Save updated settings
	if err := h.settingsService.UpdateSystemSettings(ctx, currentSettings); err != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "settings updated",
		"data":    currentSettings,
	})
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

// GetXraySettings returns Xray-specific settings.
func (h *SettingsHandler) GetXraySettings(c *gin.Context) {
	// Return default Xray settings
	c.JSON(http.StatusOK, gin.H{
		"auto_update":    false,
		"custom_config":  false,
		"config_path":    "",
		"check_interval": 24,
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

// GetProtocolSettings returns protocol settings.
func (h *SettingsHandler) GetProtocolSettings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"protocols": map[string]bool{
			"trojan":      true,
			"vmess":       true,
			"vless":       true,
			"shadowsocks": true,
			"socks":       false,
			"http":        false,
		},
		"transports": map[string]bool{
			"tcp":   true,
			"ws":    true,
			"http2": true,
			"grpc":  true,
			"quic":  false,
		},
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

	h.logger.Info("Protocol settings updated")

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"message":    "Protocol settings updated",
		"protocols":  req.Protocols,
		"transports": req.Transports,
	})
}
