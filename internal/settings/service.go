// Package settings provides system settings management.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"v/internal/database/repository"
)

const AutoProxySettingsKey = "auto_proxy_settings"

var defaultAutoProxyProtocolPriority = []string{"trojan", "vmess", "vless", "shadowsocks"}

// AutoProxySettings controls automatic proxy provisioning behavior.
type AutoProxySettings struct {
	ProtocolPriority []string `json:"protocol_priority"`
}

// RuntimeDatabaseInfo describes the database connection currently used by
// the running process. It is returned with system settings for display only;
// it is not persisted back into the settings table.
type RuntimeDatabaseInfo struct {
	Driver    string `json:"driver"`
	Path      string `json:"path,omitempty"`
	DSNMasked string `json:"dsn_masked,omitempty"`
}

// SystemSettings represents all system settings.
type SystemSettings struct {
	SiteName            string `json:"site_name"`
	SiteDescription     string `json:"site_description"`
	AllowRegistration   bool   `json:"allow_registration"`
	DefaultTrafficLimit int64  `json:"default_traffic_limit"`
	DefaultExpiryDays   int    `json:"default_expiry_days"`

	// Security settings
	SessionTimeout    int    `json:"session_timeout"`
	EnableIPWhitelist bool   `json:"enable_ip_whitelist"`
	IPWhitelist       string `json:"ip_whitelist"`
	EnableLoginLock   bool   `json:"enable_login_lock"`
	MaxLoginAttempts  int    `json:"max_login_attempts"`
	LockDuration      int    `json:"lock_duration"`

	// Panel settings
	PanelAccessIP  string `json:"panel_access_ip"`  // 面板访问 IP
	PanelPort      int    `json:"panel_port"`       // 面板监听端口
	PanelBasePath  string `json:"panel_base_path"`  // 面板基础路径
	ProxyMode      string `json:"proxy_mode"`       // 代理模式
	Timezone       string `json:"timezone"`         // 系统时区
	PanelCertPath  string `json:"panel_cert_path"`  // 面板证书公钥路径
	PanelKeyPath   string `json:"panel_key_path"`   // 面板证书私钥路径
	PanelAPIDomain string `json:"panel_api_domain"` // 面板 API 域名

	// Database settings
	DBType               string               `json:"db_type"`
	DBHost               string               `json:"db_host"`
	DBPort               int                  `json:"db_port"`
	DBName               string               `json:"db_name"`
	DBUser               string               `json:"db_user"`
	DBPassword           string               `json:"-"`
	DBPasswordConfigured bool                 `json:"db_password_configured"`
	SQLitePath           string               `json:"sqlite_path"`
	RuntimeDatabase      *RuntimeDatabaseInfo `json:"runtime_database,omitempty"`

	// Log settings
	LogLevel           string `json:"log_level"`
	LogRetentionDays   int    `json:"log_retention_days"`
	LogPath            string `json:"log_path"`
	EnableAccessLog    bool   `json:"enable_access_log"`
	EnableOperationLog bool   `json:"enable_operation_log"`

	// SMTP settings
	SMTPHost               string `json:"smtp_host"`
	SMTPPort               int    `json:"smtp_port"`
	SMTPUser               string `json:"smtp_user"`
	SMTPFrom               string `json:"smtp_from"`
	SMTPAlertEmail         string `json:"smtp_alert_email"`
	SMTPPassword           string `json:"-"` // Hidden in JSON responses
	SMTPPasswordConfigured bool   `json:"smtp_password_configured"`

	// Telegram settings
	TelegramBotToken string `json:"-"` // Hidden in JSON responses
	TelegramChatID   string `json:"telegram_chat_id"`

	// Rate limiting
	RateLimitEnabled  bool `json:"rate_limit_enabled"`
	RateLimitRequests int  `json:"rate_limit_requests"`
	RateLimitWindow   int  `json:"rate_limit_window"`

	// Payment settings
	PaymentAlipayEnabled              bool   `json:"payment_alipay_enabled"`
	PaymentAlipayAppID                string `json:"payment_alipay_app_id"`
	PaymentAlipayPrivateKey           string `json:"-"`
	PaymentAlipayPrivateKeyConfigured bool   `json:"payment_alipay_private_key_configured"`
	PaymentAlipayPublicKey            string `json:"payment_alipay_public_key"`
	PaymentAlipayNotifyURL            string `json:"payment_alipay_notify_url"`
	PaymentAlipayReturnURL            string `json:"payment_alipay_return_url"`
	PaymentAlipaySandbox              bool   `json:"payment_alipay_sandbox"`
	PaymentWeChatEnabled              bool   `json:"payment_wechat_enabled"`
	PaymentWeChatAppID                string `json:"payment_wechat_app_id"`
	PaymentWeChatMchID                string `json:"payment_wechat_mch_id"`
	PaymentWeChatAPIKey               string `json:"-"`
	PaymentWeChatAPIKeyConfigured     bool   `json:"payment_wechat_api_key_configured"`
	PaymentWeChatNotifyURL            string `json:"payment_wechat_notify_url"`
	PaymentWeChatSandbox              bool   `json:"payment_wechat_sandbox"`

	// Xray settings
	XrayConfigTemplate string `json:"xray_config_template"`
}

// DefaultSettings returns default system settings.
func DefaultSettings() *SystemSettings {
	return &SystemSettings{
		SiteName:            "V Panel",
		SiteDescription:     "Proxy Server Management Panel",
		AllowRegistration:   false,
		DefaultTrafficLimit: 0, // Unlimited
		DefaultExpiryDays:   30,
		SessionTimeout:      1440,
		EnableIPWhitelist:   false,
		EnableLoginLock:     false,
		MaxLoginAttempts:    5,
		LockDuration:        10,
		// 默认与 config.yaml.example / Dockerfile EXPOSE 对齐 (8080)。
		// 若 admin 在 UI 改了端口，会持久化到 settings 表并在重启时覆盖
		// cfg.Server.Port；保持默认就不会触发覆盖。
		PanelPort:          8080,
		PanelBasePath:      "/",
		ProxyMode:          "compatible",
		Timezone:           "Asia/Shanghai",
		DBType:             "sqlite",
		DBPort:             3306,
		DBName:             "v_panel",
		DBUser:             "root",
		SQLitePath:         "./data/v.db",
		LogLevel:           "info",
		LogRetentionDays:   30,
		LogPath:            "./logs",
		EnableAccessLog:    true,
		EnableOperationLog: true,
		SMTPPort:           587,
		RateLimitEnabled:   true,
		RateLimitRequests:  100,
		RateLimitWindow:    60, // seconds
	}
}

// DefaultAutoProxySettings returns default automatic proxy provisioning settings.
func DefaultAutoProxySettings() *AutoProxySettings {
	return &AutoProxySettings{
		ProtocolPriority: append([]string{}, defaultAutoProxyProtocolPriority...),
	}
}

// NormalizeAutoProxyProtocolPriority validates and normalizes protocol priority.
func NormalizeAutoProxyProtocolPriority(priority []string) ([]string, error) {
	if len(priority) == 0 {
		return append([]string{}, defaultAutoProxyProtocolPriority...), nil
	}

	allowed := map[string]struct{}{
		"trojan":      {},
		"vmess":       {},
		"vless":       {},
		"shadowsocks": {},
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(priority))
	for _, protocolName := range priority {
		protocolName = strings.ToLower(strings.TrimSpace(protocolName))
		if protocolName == "" {
			continue
		}
		if _, ok := allowed[protocolName]; !ok {
			return nil, fmt.Errorf("unsupported auto proxy protocol: %s", protocolName)
		}
		if _, ok := seen[protocolName]; ok {
			continue
		}
		seen[protocolName] = struct{}{}
		normalized = append(normalized, protocolName)
	}

	for _, protocolName := range defaultAutoProxyProtocolPriority {
		if _, ok := seen[protocolName]; ok {
			continue
		}
		seen[protocolName] = struct{}{}
		normalized = append(normalized, protocolName)
	}

	return normalized, nil
}

// Service provides settings management functionality.
type Service struct {
	repo    repository.SettingsRepository
	cache   *SystemSettings
	cacheMu sync.RWMutex
}

// UpdateOptions controls how system settings are persisted.
type UpdateOptions struct {
	IncludePaymentSettings bool
}

// NewService creates a new settings service.
func NewService(repo repository.SettingsRepository) *Service {
	return &Service{
		repo:  repo,
		cache: nil,
	}
}

// Get retrieves a single setting value.
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	return s.repo.Get(ctx, key)
}

// GetAll retrieves all settings as a map.
func (s *Service) GetAll(ctx context.Context) (map[string]string, error) {
	return s.repo.GetAll(ctx)
}

// GetTyped retrieves a setting and unmarshals it into the target.
func (s *Service) GetTyped(ctx context.Context, key string, target interface{}) error {
	value, err := s.repo.Get(ctx, key)
	if err != nil {
		return err
	}
	if value == "" {
		return nil
	}
	return json.Unmarshal([]byte(value), target)
}

// Set updates a single setting.
func (s *Service) Set(ctx context.Context, key, value string) error {
	err := s.repo.Set(ctx, key, value)
	if err != nil {
		return err
	}
	// Invalidate cache
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
	return nil
}

// SetMultiple updates multiple settings.
func (s *Service) SetMultiple(ctx context.Context, settings map[string]string) error {
	err := s.repo.SetMultiple(ctx, settings)
	if err != nil {
		return err
	}
	// Invalidate cache
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
	return nil
}

// IsProtocolEnabled reports whether a given proxy protocol is enabled in
// system settings (admin "协议管理" Tab). Returns true when the protocol is
// absent from the stored map so newly-introduced protocols default to ON
// rather than silently breaking. Unknown errors fall back to true so a
// transient DB hiccup never locks the admin out of creating proxies.
//
// `protocol` is normalized to lowercase. Pass exactly what proxy.Protocol
// stores, e.g. "vmess" / "vless" / "trojan" / "shadowsocks" / "socks" / "http".
func (s *Service) IsProtocolEnabled(ctx context.Context, protocol string) bool {
	if s == nil || strings.TrimSpace(protocol) == "" {
		return true
	}
	key := strings.ToLower(strings.TrimSpace(protocol))

	var stored struct {
		Protocols map[string]bool `json:"protocols"`
	}
	if err := s.GetTyped(ctx, "xray_protocol_settings", &stored); err != nil {
		return true
	}
	if stored.Protocols == nil {
		return true
	}
	if enabled, ok := stored.Protocols[key]; ok {
		return enabled
	}
	return true
}

// GetAutoProxySettings retrieves automatic proxy provisioning settings.
func (s *Service) GetAutoProxySettings(ctx context.Context) (*AutoProxySettings, error) {
	settings := DefaultAutoProxySettings()
	var stored AutoProxySettings
	if err := s.GetTyped(ctx, AutoProxySettingsKey, &stored); err != nil {
		return nil, err
	}
	if len(stored.ProtocolPriority) == 0 {
		return settings, nil
	}

	normalized, err := NormalizeAutoProxyProtocolPriority(stored.ProtocolPriority)
	if err != nil {
		return nil, err
	}
	settings.ProtocolPriority = normalized
	return settings, nil
}

// UpdateAutoProxySettings validates and persists automatic proxy provisioning settings.
func (s *Service) UpdateAutoProxySettings(ctx context.Context, next *AutoProxySettings) (*AutoProxySettings, error) {
	if next == nil {
		next = DefaultAutoProxySettings()
	}
	normalized, err := NormalizeAutoProxyProtocolPriority(next.ProtocolPriority)
	if err != nil {
		return nil, err
	}

	settings := &AutoProxySettings{ProtocolPriority: normalized}
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	if err := s.Set(ctx, AutoProxySettingsKey, string(data)); err != nil {
		return nil, err
	}
	return settings, nil
}

// GetSystemSettings retrieves all system settings as a structured object.
func (s *Service) GetSystemSettings(ctx context.Context) (*SystemSettings, error) {
	// Check cache first
	s.cacheMu.RLock()
	if s.cache != nil {
		cached := *s.cache
		s.cacheMu.RUnlock()
		return &cached, nil
	}
	s.cacheMu.RUnlock()

	// Load from database
	allSettings, err := s.repo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	settings := DefaultSettings()

	// Map database values to struct
	if v, ok := allSettings["site_name"]; ok && v != "" {
		settings.SiteName = v
	}
	if v, ok := allSettings["site_description"]; ok && v != "" {
		settings.SiteDescription = v
	}
	if v, ok := allSettings["allow_registration"]; ok {
		settings.AllowRegistration = v == "true"
	}
	if v, ok := allSettings["default_traffic_limit"]; ok && v != "" {
		var limit int64
		if json.Unmarshal([]byte(v), &limit) == nil {
			settings.DefaultTrafficLimit = limit
		}
	}
	if v, ok := allSettings["default_expiry_days"]; ok && v != "" {
		var days int
		if json.Unmarshal([]byte(v), &days) == nil {
			settings.DefaultExpiryDays = days
		}
	}
	if v, ok := allSettings["session_timeout"]; ok && v != "" {
		var timeout int
		if json.Unmarshal([]byte(v), &timeout) == nil {
			settings.SessionTimeout = timeout
		}
	}
	if v, ok := allSettings["enable_ip_whitelist"]; ok {
		settings.EnableIPWhitelist = v == "true"
	}
	if v, ok := allSettings["ip_whitelist"]; ok {
		settings.IPWhitelist = v
	}
	if v, ok := allSettings["enable_login_lock"]; ok {
		settings.EnableLoginLock = v == "true"
	}
	if v, ok := allSettings["max_login_attempts"]; ok && v != "" {
		var attempts int
		if json.Unmarshal([]byte(v), &attempts) == nil {
			settings.MaxLoginAttempts = attempts
		}
	}
	if v, ok := allSettings["lock_duration"]; ok && v != "" {
		var duration int
		if json.Unmarshal([]byte(v), &duration) == nil {
			settings.LockDuration = duration
		}
	}
	// Panel settings
	if v, ok := allSettings["panel_access_ip"]; ok {
		settings.PanelAccessIP = v
	}
	if v, ok := allSettings["panel_port"]; ok && v != "" {
		var port int
		if json.Unmarshal([]byte(v), &port) == nil {
			settings.PanelPort = port
		}
	}
	if v, ok := allSettings["panel_base_path"]; ok && v != "" {
		settings.PanelBasePath = v
	}
	if v, ok := allSettings["proxy_mode"]; ok && v != "" {
		settings.ProxyMode = v
	}
	if v, ok := allSettings["timezone"]; ok && v != "" {
		settings.Timezone = v
	}
	if v, ok := allSettings["panel_cert_path"]; ok {
		settings.PanelCertPath = v
	}
	if v, ok := allSettings["panel_key_path"]; ok {
		settings.PanelKeyPath = v
	}
	if v, ok := allSettings["panel_api_domain"]; ok {
		settings.PanelAPIDomain = v
	}
	if v, ok := allSettings["db_type"]; ok && v != "" {
		settings.DBType = v
	}
	if v, ok := allSettings["db_host"]; ok {
		settings.DBHost = v
	}
	if v, ok := allSettings["db_port"]; ok && v != "" {
		var port int
		if json.Unmarshal([]byte(v), &port) == nil {
			settings.DBPort = port
		}
	}
	if v, ok := allSettings["db_name"]; ok {
		settings.DBName = v
	}
	if v, ok := allSettings["db_user"]; ok {
		settings.DBUser = v
	}
	if v, ok := allSettings["db_password"]; ok {
		settings.DBPassword = v
		settings.DBPasswordConfigured = v != ""
	}
	if v, ok := allSettings["sqlite_path"]; ok && v != "" {
		settings.SQLitePath = v
	}
	if v, ok := allSettings["log_level"]; ok && v != "" {
		settings.LogLevel = v
	}
	if v, ok := allSettings["log_retention_days"]; ok && v != "" {
		var days int
		if json.Unmarshal([]byte(v), &days) == nil {
			settings.LogRetentionDays = days
		}
	}
	if v, ok := allSettings["log_path"]; ok && v != "" {
		settings.LogPath = v
	}
	if v, ok := allSettings["enable_access_log"]; ok {
		settings.EnableAccessLog = v == "true"
	}
	if v, ok := allSettings["enable_operation_log"]; ok {
		settings.EnableOperationLog = v == "true"
	}
	if v, ok := allSettings["smtp_host"]; ok {
		settings.SMTPHost = v
	}
	if v, ok := allSettings["smtp_port"]; ok && v != "" {
		var port int
		if json.Unmarshal([]byte(v), &port) == nil {
			settings.SMTPPort = port
		}
	}
	if v, ok := allSettings["smtp_user"]; ok {
		settings.SMTPUser = v
	}
	if v, ok := allSettings["smtp_from"]; ok {
		settings.SMTPFrom = v
	}
	if v, ok := allSettings["smtp_alert_email"]; ok {
		settings.SMTPAlertEmail = v
	}
	if v, ok := allSettings["smtp_password"]; ok {
		settings.SMTPPassword = v
		settings.SMTPPasswordConfigured = v != ""
	}
	if v, ok := allSettings["telegram_bot_token"]; ok {
		settings.TelegramBotToken = v
	}
	if v, ok := allSettings["telegram_chat_id"]; ok {
		settings.TelegramChatID = v
	}
	if v, ok := allSettings["rate_limit_enabled"]; ok {
		settings.RateLimitEnabled = v == "true"
	}
	if v, ok := allSettings["rate_limit_requests"]; ok && v != "" {
		var requests int
		if json.Unmarshal([]byte(v), &requests) == nil {
			settings.RateLimitRequests = requests
		}
	}
	if v, ok := allSettings["rate_limit_window"]; ok && v != "" {
		var window int
		if json.Unmarshal([]byte(v), &window) == nil {
			settings.RateLimitWindow = window
		}
	}
	if v, ok := allSettings["payment_alipay_enabled"]; ok {
		settings.PaymentAlipayEnabled = v == "true"
	}
	if v, ok := allSettings["payment_alipay_app_id"]; ok {
		settings.PaymentAlipayAppID = v
	}
	if v, ok := allSettings["payment_alipay_private_key"]; ok {
		settings.PaymentAlipayPrivateKey = v
		settings.PaymentAlipayPrivateKeyConfigured = v != ""
	}
	if v, ok := allSettings["payment_alipay_public_key"]; ok {
		settings.PaymentAlipayPublicKey = v
	}
	if v, ok := allSettings["payment_alipay_notify_url"]; ok {
		settings.PaymentAlipayNotifyURL = v
	}
	if v, ok := allSettings["payment_alipay_return_url"]; ok {
		settings.PaymentAlipayReturnURL = v
	}
	if v, ok := allSettings["payment_alipay_sandbox"]; ok {
		settings.PaymentAlipaySandbox = v == "true"
	}
	if v, ok := allSettings["payment_wechat_enabled"]; ok {
		settings.PaymentWeChatEnabled = v == "true"
	}
	if v, ok := allSettings["payment_wechat_app_id"]; ok {
		settings.PaymentWeChatAppID = v
	}
	if v, ok := allSettings["payment_wechat_mch_id"]; ok {
		settings.PaymentWeChatMchID = v
	}
	if v, ok := allSettings["payment_wechat_api_key"]; ok {
		settings.PaymentWeChatAPIKey = v
		settings.PaymentWeChatAPIKeyConfigured = v != ""
	}
	if v, ok := allSettings["payment_wechat_notify_url"]; ok {
		settings.PaymentWeChatNotifyURL = v
	}
	if v, ok := allSettings["payment_wechat_sandbox"]; ok {
		settings.PaymentWeChatSandbox = v == "true"
	}
	if v, ok := allSettings["xray_config_template"]; ok {
		settings.XrayConfigTemplate = v
	}

	// Update cache
	s.cacheMu.Lock()
	s.cache = settings
	s.cacheMu.Unlock()

	return settings, nil
}

// UpdateSystemSettings updates system settings from a structured object.
func (s *Service) UpdateSystemSettings(ctx context.Context, settings *SystemSettings) error {
	return s.UpdateSystemSettingsWithOptions(ctx, settings, UpdateOptions{IncludePaymentSettings: true})
}

// UpdateSystemSettingsWithOptions persists system settings with selective field groups.
func (s *Service) UpdateSystemSettingsWithOptions(ctx context.Context, settings *SystemSettings, options UpdateOptions) error {
	updates := make(map[string]string)

	updates["site_name"] = settings.SiteName
	updates["site_description"] = settings.SiteDescription
	updates["allow_registration"] = boolToString(settings.AllowRegistration)

	if data, err := json.Marshal(settings.DefaultTrafficLimit); err == nil {
		updates["default_traffic_limit"] = string(data)
	}
	if data, err := json.Marshal(settings.DefaultExpiryDays); err == nil {
		updates["default_expiry_days"] = string(data)
	}
	if data, err := json.Marshal(settings.SessionTimeout); err == nil {
		updates["session_timeout"] = string(data)
	}
	updates["enable_ip_whitelist"] = boolToString(settings.EnableIPWhitelist)
	updates["ip_whitelist"] = settings.IPWhitelist
	updates["enable_login_lock"] = boolToString(settings.EnableLoginLock)
	if data, err := json.Marshal(settings.MaxLoginAttempts); err == nil {
		updates["max_login_attempts"] = string(data)
	}
	if data, err := json.Marshal(settings.LockDuration); err == nil {
		updates["lock_duration"] = string(data)
	}

	// Panel settings
	updates["panel_access_ip"] = settings.PanelAccessIP
	if data, err := json.Marshal(settings.PanelPort); err == nil {
		updates["panel_port"] = string(data)
	}
	updates["panel_base_path"] = settings.PanelBasePath
	updates["proxy_mode"] = settings.ProxyMode
	updates["timezone"] = settings.Timezone
	updates["panel_cert_path"] = settings.PanelCertPath
	updates["panel_key_path"] = settings.PanelKeyPath
	updates["panel_api_domain"] = settings.PanelAPIDomain
	updates["db_type"] = settings.DBType
	updates["db_host"] = settings.DBHost
	if data, err := json.Marshal(settings.DBPort); err == nil {
		updates["db_port"] = string(data)
	}
	updates["db_name"] = settings.DBName
	updates["db_user"] = settings.DBUser
	if settings.DBPassword != "" {
		updates["db_password"] = settings.DBPassword
	}
	updates["sqlite_path"] = settings.SQLitePath
	updates["log_level"] = settings.LogLevel
	if data, err := json.Marshal(settings.LogRetentionDays); err == nil {
		updates["log_retention_days"] = string(data)
	}
	updates["log_path"] = settings.LogPath
	updates["enable_access_log"] = boolToString(settings.EnableAccessLog)
	updates["enable_operation_log"] = boolToString(settings.EnableOperationLog)

	updates["smtp_host"] = settings.SMTPHost
	if data, err := json.Marshal(settings.SMTPPort); err == nil {
		updates["smtp_port"] = string(data)
	}
	updates["smtp_user"] = settings.SMTPUser
	updates["smtp_from"] = settings.SMTPFrom
	updates["smtp_alert_email"] = settings.SMTPAlertEmail
	if settings.SMTPPassword != "" {
		updates["smtp_password"] = settings.SMTPPassword
	}

	if settings.TelegramBotToken != "" {
		updates["telegram_bot_token"] = settings.TelegramBotToken
	}
	updates["telegram_chat_id"] = settings.TelegramChatID

	updates["rate_limit_enabled"] = boolToString(settings.RateLimitEnabled)
	if data, err := json.Marshal(settings.RateLimitRequests); err == nil {
		updates["rate_limit_requests"] = string(data)
	}
	if data, err := json.Marshal(settings.RateLimitWindow); err == nil {
		updates["rate_limit_window"] = string(data)
	}

	if options.IncludePaymentSettings {
		updates["payment_alipay_enabled"] = boolToString(settings.PaymentAlipayEnabled)
		updates["payment_alipay_app_id"] = settings.PaymentAlipayAppID
		updates["payment_alipay_private_key"] = settings.PaymentAlipayPrivateKey
		updates["payment_alipay_public_key"] = settings.PaymentAlipayPublicKey
		updates["payment_alipay_notify_url"] = settings.PaymentAlipayNotifyURL
		updates["payment_alipay_return_url"] = settings.PaymentAlipayReturnURL
		updates["payment_alipay_sandbox"] = boolToString(settings.PaymentAlipaySandbox)
		updates["payment_wechat_enabled"] = boolToString(settings.PaymentWeChatEnabled)
		updates["payment_wechat_app_id"] = settings.PaymentWeChatAppID
		updates["payment_wechat_mch_id"] = settings.PaymentWeChatMchID
		updates["payment_wechat_api_key"] = settings.PaymentWeChatAPIKey
		updates["payment_wechat_notify_url"] = settings.PaymentWeChatNotifyURL
		updates["payment_wechat_sandbox"] = boolToString(settings.PaymentWeChatSandbox)
	}

	updates["xray_config_template"] = settings.XrayConfigTemplate

	return s.SetMultiple(ctx, updates)
}

// Backup creates a backup of all settings.
func (s *Service) Backup(ctx context.Context) ([]byte, error) {
	return s.repo.Backup(ctx)
}

// Restore restores settings from a backup.
func (s *Service) Restore(ctx context.Context, data []byte) error {
	err := s.repo.Restore(ctx, data)
	if err != nil {
		return err
	}
	// Invalidate cache
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
	return nil
}

// InvalidateCache clears the settings cache.
func (s *Service) InvalidateCache() {
	s.cacheMu.Lock()
	s.cache = nil
	s.cacheMu.Unlock()
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
