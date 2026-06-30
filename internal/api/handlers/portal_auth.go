// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/monitor"
	portalauth "v/internal/portal/auth"
	"v/internal/settings"
	pkgerrors "v/pkg/errors"
)

// PortalAuthHandler handles portal authentication requests.
type PortalAuthHandler struct {
	portalAuthService *portalauth.Service
	authService       *auth.Service
	userRepo          repository.UserRepository
	roleRepo          repository.RoleRepository
	proxyRepo         repository.ProxyRepository
	entitlement       *entitlement.Service
	emailSender       portalEmailSender
	telegramSender    portalTelegramSender
	baseURL           string
	avatarStoragePath string
	rateLimiter       *portalauth.RateLimiter
	rateLimitConfig   portalauth.RateLimitConfig
	auditService      monitor.AuditService
	settingsService   *settings.Service
	logger            logger.Logger
}

type portalEmailSender interface {
	CanSendEmail() bool
	SendEmail(to, subject, body string) error
}

type portalTelegramSender interface {
	CanSendTelegram() bool
	SendTelegramTo(chatID, message string) error
}

func normalizePortalEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (h *PortalAuthHandler) updateLastLogin(ctx *gin.Context, userID int64) {
	user, err := h.userRepo.GetByID(ctx.Request.Context(), userID)
	if err != nil {
		h.logger.Warn("failed to load user for last login update", logger.F("user_id", userID), logger.F("error", err))
		return
	}

	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.LastLoginIP = ctx.ClientIP()
	if err := h.userRepo.Update(ctx.Request.Context(), user); err != nil {
		h.logger.Warn("failed to update portal user last login", logger.F("user_id", userID), logger.F("error", err))
	}
}

// NewPortalAuthHandler creates a new PortalAuthHandler.
func NewPortalAuthHandler(
	portalAuthService *portalauth.Service,
	authService *auth.Service,
	userRepo repository.UserRepository,
	proxyRepo repository.ProxyRepository,
	log logger.Logger,
) *PortalAuthHandler {
	return &PortalAuthHandler{
		portalAuthService: portalAuthService,
		authService:       authService,
		userRepo:          userRepo,
		proxyRepo:         proxyRepo,
		avatarStoragePath: "./data/avatars",
		rateLimiter:       portalauth.NewRateLimiter(),
		rateLimitConfig:   portalauth.DefaultRateLimitConfig(),
		logger:            log,
	}
}

// WithEmailSender configures a mail sender for verification and reset emails.
func (h *PortalAuthHandler) WithEmailSender(sender portalEmailSender, baseURL string) *PortalAuthHandler {
	h.emailSender = sender
	h.baseURL = strings.TrimSuffix(baseURL, "/")
	return h
}

// WithEntitlementService configures portal entitlement-aware profile data.
func (h *PortalAuthHandler) WithEntitlementService(entitlementService *entitlement.Service) *PortalAuthHandler {
	h.entitlement = entitlementService
	return h
}

// WithTelegramSender configures a Telegram sender for user binding verification.
func (h *PortalAuthHandler) WithTelegramSender(sender portalTelegramSender) *PortalAuthHandler {
	h.telegramSender = sender
	return h
}

// WithAvatarStoragePath configures where uploaded avatars are stored.
func (h *PortalAuthHandler) WithAvatarStoragePath(path string) *PortalAuthHandler {
	if strings.TrimSpace(path) != "" {
		h.avatarStoragePath = strings.TrimSpace(path)
	}
	return h
}

// WithRoleRepository enables role-based permission enrichment in login responses.
func (h *PortalAuthHandler) WithRoleRepository(roleRepo repository.RoleRepository) *PortalAuthHandler {
	h.roleRepo = roleRepo
	return h
}

// WithAuditService wires the audit emitter for portal user operations.
func (h *PortalAuthHandler) WithAuditService(audit monitor.AuditService) *PortalAuthHandler {
	h.auditService = audit
	return h
}

// WithSettingsService wires authentication integration settings.
func (h *PortalAuthHandler) WithSettingsService(settingsService *settings.Service) *PortalAuthHandler {
	h.settingsService = settingsService
	return h
}

// getRolePermissions resolves the permission list for a role name. Admins get "*".
func (h *PortalAuthHandler) getRolePermissions(c *gin.Context, roleName string) []string {
	if roleName == "" {
		return nil
	}
	if roleName == "admin" {
		return []string{"*"}
	}
	if h == nil || h.roleRepo == nil {
		return nil
	}

	role, err := h.roleRepo.GetByName(c.Request.Context(), roleName)
	if err != nil || role == nil {
		return nil
	}

	perms, err := role.GetPermissionsList()
	if err != nil {
		return nil
	}
	return perms
}

func normalizePortalHost(rawHost string) string {
	host := strings.TrimSpace(strings.Split(rawHost, ",")[0])
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	return strings.Trim(strings.ToLower(host), "[]")
}

func isLocalPortalHost(host string) bool {
	switch normalizePortalHost(host) {
	case "", "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	default:
		return false
	}
}

func isLocalPortalBaseURL(rawBaseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawBaseURL))
	if err != nil {
		return false
	}

	return isLocalPortalHost(parsed.Host)
}

func requestPortalBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	host := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Host"), ",")[0])
	if host == "" {
		host = strings.TrimSpace(c.Request.Host)
	}
	if host == "" || isLocalPortalHost(host) {
		return ""
	}

	scheme := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])
	if scheme == "" {
		if strings.EqualFold(c.GetHeader("X-Forwarded-Ssl"), "on") {
			scheme = "https"
		} else if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	return fmt.Sprintf("%s://%s", strings.ToLower(scheme), host)
}

func (h *PortalAuthHandler) resolvePortalBaseURL(c *gin.Context) string {
	configuredBaseURL := strings.TrimSuffix(strings.TrimSpace(h.baseURL), "/")
	if configuredBaseURL != "" && !isLocalPortalBaseURL(configuredBaseURL) {
		return configuredBaseURL
	}

	if requestBaseURL := requestPortalBaseURL(c); requestBaseURL != "" {
		return strings.TrimSuffix(requestBaseURL, "/")
	}

	return configuredBaseURL
}

func portalAvatarPublicPath(filename string) string {
	return "/api/portal/auth/avatar/" + filename
}

func (h *PortalAuthHandler) deleteManagedAvatarFile(rawURL string) {
	if h == nil || strings.TrimSpace(rawURL) == "" {
		return
	}
	const prefix = "/api/portal/auth/avatar/"
	if !strings.HasPrefix(rawURL, prefix) {
		return
	}
	filename := filepath.Base(strings.TrimPrefix(rawURL, prefix))
	if filename == "." || filename == "" {
		return
	}
	_ = os.Remove(filepath.Join(h.avatarStoragePath, filename))
}

func (h *PortalAuthHandler) sendVerificationEmail(c *gin.Context, email, token string) error {
	baseURL := h.resolvePortalBaseURL(c)
	if h.emailSender == nil || !h.emailSender.CanSendEmail() || email == "" || token == "" || baseURL == "" {
		return nil
	}

	verifyURL := baseURL + "/user/login?verify_email_token=" + token
	subject := "请验证您的邮箱"
	body := "欢迎注册 V Panel。\n\n请点击以下链接验证您的邮箱（24 小时内有效）：\n" + verifyURL + "\n\n如果这不是您的操作，请忽略此邮件。"

	return h.emailSender.SendEmail(email, subject, body)
}

func (h *PortalAuthHandler) sendPasswordResetEmail(c *gin.Context, email, token string) error {
	baseURL := h.resolvePortalBaseURL(c)
	if h.emailSender == nil || !h.emailSender.CanSendEmail() || email == "" || token == "" || baseURL == "" {
		return nil
	}

	resetURL := baseURL + "/user/reset-password?token=" + token
	subject := "重置您的密码"
	body := "我们收到了您的密码重置请求。\n\n请点击以下链接设置新密码（1 小时内有效）：\n" + resetURL + "\n\n如果这不是您的操作，请忽略此邮件。"

	return h.emailSender.SendEmail(email, subject, body)
}

// PortalRegisterRequest represents a registration request.
type PortalRegisterRequest struct {
	Username   string `json:"username" binding:"required"`
	Email      string `json:"email" binding:"required"`
	Password   string `json:"password" binding:"required"`
	InviteCode string `json:"invite_code,omitempty"`
}

// Register handles user registration.
func (h *PortalAuthHandler) Register(c *gin.Context) {
	var req PortalRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	registerReq := &portalauth.RegisterRequest{
		Username:   req.Username,
		Email:      req.Email,
		Password:   req.Password,
		InviteCode: req.InviteCode,
	}

	result, err := h.portalAuthService.Register(c.Request.Context(), registerReq, false, h.authService.HashPassword)
	if err != nil {
		h.handleError(c, err)
		return
	}

	emailVerificationSent := false
	if h.emailSender != nil && h.emailSender.CanSendEmail() {
		token, tokenErr := h.portalAuthService.CreateEmailVerificationToken(c.Request.Context(), result.UserID, result.Email)
		if tokenErr != nil {
			h.logger.Warn("failed to create email verification token", logger.F("user_id", result.UserID), logger.F("error", tokenErr))
		} else if err := h.sendVerificationEmail(c, result.Email, token); err != nil {
			h.logger.Warn("failed to send verification email", logger.F("user_id", result.UserID), logger.F("error", err))
		} else {
			emailVerificationSent = true
		}
	}

	if !emailVerificationSent {
		user, userErr := h.userRepo.GetByID(c.Request.Context(), result.UserID)
		if userErr != nil {
			h.logger.Warn("failed to load portal user for email verification fallback", logger.F("user_id", result.UserID), logger.F("error", userErr))
		} else if !user.EmailVerified {
			now := time.Now().UTC()
			user.EmailVerified = true
			user.EmailVerifiedAt = &now
			if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
				h.logger.Warn("failed to mark portal user email as verified by fallback", logger.F("user_id", result.UserID), logger.F("error", err))
			}
		}
	}

	if h.entitlement != nil {
		if _, _, entitlementErr := h.entitlement.GetAccessibleProxies(c.Request.Context(), result.UserID); entitlementErr != nil && !pkgerrors.IsForbidden(entitlementErr) {
			h.logger.Warn("failed to initialize portal trial entitlement after registration",
				logger.F("user_id", result.UserID),
				logger.F("error", entitlementErr),
			)
		}
	}

	h.logger.Info("user registered via portal", logger.F("user_id", result.UserID), logger.F("username", result.Username))

	c.JSON(http.StatusCreated, gin.H{
		"message":                 "注册成功",
		"need_email_verification": emailVerificationSent,
		"user": gin.H{
			"id":       result.UserID,
			"username": result.Username,
			"email":    result.Email,
		},
	})
}

// PortalLoginRequest represents a login request.
type PortalLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Remember bool   `json:"remember"`
}

// Login handles user login.
func (h *PortalAuthHandler) Login(c *gin.Context) {
	var req PortalLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	loginReq := &portalauth.LoginRequest{
		Username: req.Username,
		Password: req.Password,
		Remember: req.Remember,
	}

	result, err := h.portalAuthService.Login(
		c.Request.Context(),
		loginReq,
		c.ClientIP(),
		h.rateLimiter,
		h.rateLimitConfig,
		h.authService.VerifyPassword,
		h.authService.GenerateToken,
	)
	if err != nil {
		h.handleLoginError(c, err)
		return
	}

	// Check if 2FA is required
	if result.Requires2FA {
		c.JSON(http.StatusOK, gin.H{
			"requires_2fa": true,
			"user_id":      result.UserID,
		})
		return
	}

	h.logger.Info("user logged in via portal", logger.F("user_id", result.UserID))
	h.updateLastLogin(c, result.UserID)

	c.JSON(http.StatusOK, gin.H{
		"token": result.Token,
		"user": gin.H{
			"id":                    result.UserID,
			"username":              result.Username,
			"email":                 result.Email,
			"role":                  result.Role,
			"permissions":           h.getRolePermissions(c, result.Role),
			"force_password_change": result.ForcePasswordChange,
		},
	})
}

func (h *PortalAuthHandler) handleLoginError(c *gin.Context, err error) {
	if appErr, ok := pkgerrors.AsAppError(err); ok {
		switch appErr.Code {
		case pkgerrors.ErrCodeValidation, pkgerrors.ErrCodeBadRequest:
			c.JSON(http.StatusBadRequest, gin.H{"error": appErr.Message})
		case pkgerrors.ErrCodeUnauthorized, pkgerrors.ErrCodeNotFound:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		case pkgerrors.ErrCodeForbidden:
			c.JSON(http.StatusForbidden, gin.H{"error": appErr.Message})
		case pkgerrors.ErrCodeRateLimit:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		default:
			h.logger.Error("portal login error", logger.F("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误，请稍后重试"})
		}
		return
	}

	errStr := err.Error()
	switch {
	case contains(errStr, "validation"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
	case contains(errStr, "unauthorized"), contains(errStr, "密码错误"), contains(errStr, "invalid credentials"), contains(errStr, "not found"):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
	case contains(errStr, "forbidden"), contains(errStr, "禁用"), contains(errStr, "disabled"):
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
	case contains(errStr, "rate limit"), contains(errStr, "过于频繁"), contains(errStr, "too many"):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
	case contains(errStr, "expired"), contains(errStr, "过期"):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号已过期，请续费"})
	default:
		h.logger.Error("portal login error", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误，请稍后重试"})
	}
}

// Logout handles user logout.
func (h *PortalAuthHandler) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	c.JSON(http.StatusOK, gin.H{"message": "登出成功"})
}

// PortalForgotPasswordRequest represents a forgot password request.
type PortalForgotPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

// ForgotPassword handles password reset request.
func (h *PortalAuthHandler) ForgotPassword(c *gin.Context) {
	var req PortalForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	resetReq := &portalauth.RequestPasswordResetRequest{
		Email: req.Email,
	}

	token, err := h.portalAuthService.RequestPasswordReset(c.Request.Context(), resetReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	if token != "" {
		if err := h.sendPasswordResetEmail(c, resetReq.Email, token); err != nil {
			h.logger.Warn("failed to send password reset email", logger.F("email", resetReq.Email), logger.F("error", err))
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "如果该邮箱已注册，您将收到密码重置邮件"})
}

// PortalResetPasswordRequest represents a password reset request.
type PortalResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ResetPassword handles password reset.
func (h *PortalAuthHandler) ResetPassword(c *gin.Context) {
	var req PortalResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	resetReq := &portalauth.ResetPasswordRequest{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	}

	if err := h.portalAuthService.ExecutePasswordReset(c.Request.Context(), resetReq, h.authService.HashPassword); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "密码重置成功"})
}

// VerifyEmail handles email verification.
func (h *PortalAuthHandler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "验证令牌不能为空"})
		return
	}

	if err := h.portalAuthService.VerifyEmail(c.Request.Context(), token); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "邮箱验证成功"})
}

// GetProfile returns the current user's profile.
func (h *PortalAuthHandler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	var (
		effectiveExpiresAt    = user.ExpiresAt
		effectiveTrafficLimit = user.TrafficLimit
		effectiveTrafficUsed  = user.TrafficUsed
		availableNodes        = 0
		accessDenied          = false
		hasActiveSubscription = false
		hasActiveTrial        = false
	)

	if h.entitlement != nil {
		accessState, accessErr := h.entitlement.EvaluateAccess(c.Request.Context(), userID.(int64))
		if accessState != nil {
			effectiveExpiresAt = accessState.EffectiveExpiresAt
			effectiveTrafficLimit = accessState.EffectiveTrafficLimit
			effectiveTrafficUsed = accessState.EffectiveTrafficUsed
			hasActiveSubscription = accessState.HasActiveSubscription
			hasActiveTrial = accessState.HasActiveTrial
		}
		if accessErr == nil {
			if proxies, _, proxyErr := h.entitlement.GetAccessibleProxies(c.Request.Context(), userID.(int64)); proxyErr == nil {
				nodeIDs := make(map[int64]struct{}, len(proxies))
				for _, proxy := range proxies {
					nodeKey := proxy.ID
					if proxy.NodeID != nil && *proxy.NodeID > 0 {
						nodeKey = *proxy.NodeID
					}
					nodeIDs[nodeKey] = struct{}{}
				}
				availableNodes = len(nodeIDs)
			}
		} else if pkgerrors.IsForbidden(accessErr) {
			accessDenied = true
		} else {
			h.logger.Warn("failed to evaluate portal entitlement for profile",
				logger.F("user_id", userID.(int64)),
				logger.F("error", accessErr),
			)
		}
	}

	// Determine user status
	status := "active"
	if !user.Enabled {
		status = "disabled"
	} else if effectiveExpiresAt != nil && time.Now().After(*effectiveExpiresAt) {
		status = "expired"
	} else if accessDenied {
		status = "expired"
	}

	if h.entitlement == nil && h.proxyRepo != nil {
		proxies, err := h.proxyRepo.GetByUserID(c.Request.Context(), userID.(int64), 1000, 0)
		if err == nil && len(proxies) == 0 {
			proxies, err = h.proxyRepo.GetEnabled(c.Request.Context())
		}
		if err == nil {
			nodeIDs := map[int64]struct{}{}
			for _, proxy := range proxies {
				if proxy.Enabled {
					nodeKey := proxy.ID
					if proxy.NodeID != nil && *proxy.NodeID > 0 {
						nodeKey = *proxy.NodeID
					}
					nodeIDs[nodeKey] = struct{}{}
				}
			}
			availableNodes = len(nodeIDs)
		}
	}

	entitlementType := "none"
	switch {
	case hasActiveSubscription:
		entitlementType = "subscription"
	case hasActiveTrial:
		entitlementType = "trial"
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                      user.ID,
		"username":                user.Username,
		"email":                   user.Email,
		"email_verified":          user.EmailVerified,
		"email_verified_at":       user.EmailVerifiedAt,
		"role":                    user.Role,
		"permissions":             h.getRolePermissions(c, user.Role),
		"enabled":                 user.Enabled,
		"status":                  status,
		"display_name":            user.DisplayName,
		"avatar_url":              user.AvatarURL,
		"notify_email":            user.NotifyEmail,
		"notify_telegram":         user.NotifyTelegram,
		"notify_traffic_warning":  user.NotifyTrafficWarning,
		"notify_expiry_reminder":  user.NotifyExpiryReminder,
		"notify_announcements":    user.NotifyAnnouncements,
		"theme":                   user.Theme,
		"language":                user.Language,
		"telegram_id":             user.TelegramID,
		"telegram_bound":          strings.TrimSpace(user.TelegramID) != "",
		"force_password_change":   user.ForcePasswordChange,
		"traffic_limit":           effectiveTrafficLimit,
		"traffic_used":            effectiveTrafficUsed,
		"expires_at":              effectiveExpiresAt,
		"has_active_subscription": hasActiveSubscription,
		"has_active_trial":        hasActiveTrial,
		"entitlement_type":        entitlementType,
		"two_factor_enabled":      user.TwoFactorEnabled,
		"available_nodes":         availableNodes,
		"created_at":              user.CreatedAt,
	})
}

// PortalUpdateProfileRequest represents a profile update request.
type PortalUpdateProfileRequest struct {
	Email                *string `json:"email,omitempty"`
	DisplayName          *string `json:"display_name,omitempty"`
	AvatarURL            *string `json:"avatar_url,omitempty"`
	NotifyEmail          *bool   `json:"notify_email,omitempty"`
	NotifyTelegram       *bool   `json:"notify_telegram,omitempty"`
	NotifyTrafficWarning *bool   `json:"notify_traffic_warning,omitempty"`
	NotifyExpiryReminder *bool   `json:"notify_expiry_reminder,omitempty"`
	NotifyAnnouncements  *bool   `json:"notify_announcements,omitempty"`
	Theme                *string `json:"theme,omitempty"`
	Language             *string `json:"language,omitempty"`
}

// UpdateProfile updates the current user's profile.
func (h *PortalAuthHandler) UpdateProfile(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req PortalUpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	profileChanged := false
	emailChanged := false
	verificationEmailSent := false
	previousAvatarURL := user.AvatarURL

	if req.Email != nil {
		email := normalizePortalEmail(*req.Email)
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱不能为空"})
			return
		}
		if !portalauth.ValidateEmail(email) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "邮箱格式不正确"})
			return
		}

		if email != normalizePortalEmail(user.Email) {
			existing, lookupErr := h.userRepo.GetByEmail(c.Request.Context(), email)
			if lookupErr == nil && existing != nil && existing.ID != user.ID {
				c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被其他账号使用"})
				return
			}
			if lookupErr != nil && !pkgerrors.IsNotFound(lookupErr) {
				h.logger.Warn("failed to validate portal profile email uniqueness",
					logger.F("user_id", user.ID),
					logger.F("error", lookupErr),
				)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
				return
			}

			user.Email = email
			user.EmailVerified = false
			user.EmailVerifiedAt = nil
			profileChanged = true
			emailChanged = true
		}
	}

	if req.DisplayName != nil {
		displayName := strings.TrimSpace(*req.DisplayName)
		if len(displayName) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "显示名称不能超过 64 个字符"})
			return
		}
		if displayName != user.DisplayName {
			user.DisplayName = displayName
			profileChanged = true
		}
	}

	if req.AvatarURL != nil {
		avatarURL := strings.TrimSpace(*req.AvatarURL)
		if len(avatarURL) > 512 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "头像地址不能超过 512 个字符"})
			return
		}
		if avatarURL != user.AvatarURL {
			user.AvatarURL = avatarURL
			profileChanged = true
		}
	}

	if req.NotifyEmail != nil && user.NotifyEmail != *req.NotifyEmail {
		user.NotifyEmail = *req.NotifyEmail
		profileChanged = true
	}

	if req.NotifyTelegram != nil && user.NotifyTelegram != *req.NotifyTelegram {
		user.NotifyTelegram = *req.NotifyTelegram
		profileChanged = true
	}

	if req.NotifyTrafficWarning != nil && user.NotifyTrafficWarning != *req.NotifyTrafficWarning {
		user.NotifyTrafficWarning = *req.NotifyTrafficWarning
		profileChanged = true
	}

	if req.NotifyExpiryReminder != nil && user.NotifyExpiryReminder != *req.NotifyExpiryReminder {
		user.NotifyExpiryReminder = *req.NotifyExpiryReminder
		profileChanged = true
	}

	if req.NotifyAnnouncements != nil && user.NotifyAnnouncements != *req.NotifyAnnouncements {
		user.NotifyAnnouncements = *req.NotifyAnnouncements
		profileChanged = true
	}

	if req.Theme != nil {
		theme := strings.TrimSpace(*req.Theme)
		switch theme {
		case "", "auto", "light", "dark":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "主题设置无效"})
			return
		}
		if theme == "" {
			theme = "auto"
		}
		if theme != user.Theme {
			user.Theme = theme
			profileChanged = true
		}
	}

	if req.Language != nil {
		language := strings.TrimSpace(*req.Language)
		switch language {
		case "", "zh-CN", "en-US":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "语言设置无效"})
			return
		}
		if language == "" {
			language = "zh-CN"
		}
		if language != user.Language {
			user.Language = language
			profileChanged = true
		}
	}

	if !profileChanged {
		c.JSON(http.StatusOK, gin.H{
			"message": "资料未发生变化",
			"user": gin.H{
				"email":                  user.Email,
				"email_verified":         user.EmailVerified,
				"email_verified_at":      user.EmailVerifiedAt,
				"display_name":           user.DisplayName,
				"avatar_url":             user.AvatarURL,
				"notify_email":           user.NotifyEmail,
				"notify_telegram":        user.NotifyTelegram,
				"notify_traffic_warning": user.NotifyTrafficWarning,
				"notify_expiry_reminder": user.NotifyExpiryReminder,
				"notify_announcements":   user.NotifyAnnouncements,
				"theme":                  user.Theme,
				"language":               user.Language,
				"telegram_id":            user.TelegramID,
				"telegram_bound":         strings.TrimSpace(user.TelegramID) != "",
			},
		})
		return
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败"})
		return
	}

	if previousAvatarURL != user.AvatarURL {
		h.deleteManagedAvatarFile(previousAvatarURL)
	}

	if emailChanged && h.emailSender != nil && h.emailSender.CanSendEmail() && strings.TrimSpace(h.resolvePortalBaseURL(c)) != "" {
		token, tokenErr := h.portalAuthService.ResendVerificationEmail(c.Request.Context(), user.ID, user.Email)
		if tokenErr != nil {
			h.logger.Warn("failed to create portal verification token after email change",
				logger.F("user_id", user.ID),
				logger.F("error", tokenErr),
			)
		} else if sendErr := h.sendVerificationEmail(c, user.Email, token); sendErr != nil {
			h.logger.Warn("failed to send portal verification email after email change",
				logger.F("user_id", user.ID),
				logger.F("error", sendErr),
			)
		} else {
			verificationEmailSent = true
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":                 "更新成功",
		"need_email_verification": emailChanged,
		"verification_email_sent": verificationEmailSent,
		"user": gin.H{
			"email":                  user.Email,
			"email_verified":         user.EmailVerified,
			"email_verified_at":      user.EmailVerifiedAt,
			"display_name":           user.DisplayName,
			"avatar_url":             user.AvatarURL,
			"notify_email":           user.NotifyEmail,
			"notify_telegram":        user.NotifyTelegram,
			"notify_traffic_warning": user.NotifyTrafficWarning,
			"notify_expiry_reminder": user.NotifyExpiryReminder,
			"notify_announcements":   user.NotifyAnnouncements,
			"theme":                  user.Theme,
			"language":               user.Language,
			"telegram_id":            user.TelegramID,
			"telegram_bound":         strings.TrimSpace(user.TelegramID) != "",
		},
	})
}

// ResendVerificationEmail resends a verification email for the current user.
func (h *PortalAuthHandler) ResendVerificationEmail(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	email := normalizePortalEmail(user.Email)
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前账号未绑定邮箱"})
		return
	}
	if user.EmailVerified {
		c.JSON(http.StatusBadRequest, gin.H{"error": "当前邮箱已验证"})
		return
	}
	if h.emailSender == nil || !h.emailSender.CanSendEmail() || strings.TrimSpace(h.resolvePortalBaseURL(c)) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "邮件服务暂不可用"})
		return
	}

	token, err := h.portalAuthService.ResendVerificationEmail(c.Request.Context(), user.ID, email)
	if err != nil {
		h.handleError(c, err)
		return
	}
	if err := h.sendVerificationEmail(c, email, token); err != nil {
		h.logger.Warn("failed to resend verification email",
			logger.F("user_id", user.ID),
			logger.F("error", err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "发送验证邮件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "验证邮件已发送"})
}

type PortalBindTelegramRequest struct {
	ChatID string `json:"chat_id" binding:"required"`
}

// BindTelegram binds a Telegram chat ID to the current user after sending a verification message.
func (h *PortalAuthHandler) BindTelegram(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}
	if h.telegramSender == nil || !h.telegramSender.CanSendTelegram() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Telegram 绑定当前不可用"})
		return
	}

	var req PortalBindTelegramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	chatID := strings.TrimSpace(req.ChatID)
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram Chat ID 不能为空"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	message := fmt.Sprintf("V Panel Telegram 绑定成功\n\n用户: %s\n时间: %s", user.Username, time.Now().Format("2006-01-02 15:04:05"))
	if err := h.telegramSender.SendTelegramTo(chatID, message); err != nil {
		h.logger.Warn("failed to verify telegram binding", logger.F("user_id", user.ID), logger.F("error", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法向该 Chat ID 发送验证消息，请确认已先与机器人开始对话"})
		return
	}

	user.TelegramID = chatID
	user.NotifyTelegram = true
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "绑定 Telegram 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Telegram 绑定成功",
		"user": gin.H{
			"telegram_id":     user.TelegramID,
			"telegram_bound":  true,
			"notify_telegram": true,
		},
	})
}

// UnbindTelegram removes the Telegram binding for the current user.
func (h *PortalAuthHandler) UnbindTelegram(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	user.TelegramID = ""
	user.NotifyTelegram = false
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "解绑 Telegram 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Telegram 已解绑",
		"user": gin.H{
			"telegram_id":     "",
			"telegram_bound":  false,
			"notify_telegram": false,
		},
	})
}

// UploadAvatar uploads an avatar image for the current user.
func (h *PortalAuthHandler) UploadAvatar(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择头像文件"})
		return
	}
	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件不能为空"})
		return
	}
	if fileHeader.Size > 2*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件不能超过 2MB"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取头像文件失败"})
		return
	}
	defer src.Close()

	content, err := io.ReadAll(io.LimitReader(src, 2*1024*1024+1))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取头像文件失败"})
		return
	}
	if len(content) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "头像文件不能为空"})
		return
	}

	contentType := http.DetectContentType(content)
	allowedTypes := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/gif":  "gif",
		"image/webp": "webp",
	}
	ext, ok := allowedTypes[contentType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅支持 JPG、PNG、GIF、WEBP 格式头像"})
		return
	}

	if err := os.MkdirAll(h.avatarStoragePath, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "初始化头像目录失败"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	filename := fmt.Sprintf("avatar_%d_%d.%s", user.ID, time.Now().UnixNano(), ext)
	targetPath := filepath.Join(h.avatarStoragePath, filename)
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存头像失败"})
		return
	}

	oldAvatar := user.AvatarURL
	user.AvatarURL = portalAvatarPublicPath(filename)
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		_ = os.Remove(targetPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新头像失败"})
		return
	}

	h.deleteManagedAvatarFile(oldAvatar)

	c.JSON(http.StatusOK, gin.H{
		"message": "头像上传成功",
		"user": gin.H{
			"avatar_url": user.AvatarURL,
		},
	})
}

// GetAvatar serves a previously uploaded avatar image.
func (h *PortalAuthHandler) GetAvatar(c *gin.Context) {
	filename := filepath.Base(strings.TrimSpace(c.Param("filename")))
	if filename == "" || filename == "." || filename == "/" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的头像文件"})
		return
	}

	targetPath := filepath.Join(h.avatarStoragePath, filename)
	if _, err := os.Stat(targetPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "头像不存在"})
		return
	}

	c.File(targetPath)
}

// PortalChangePasswordRequest represents a password change request.
type PortalChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePassword changes the current user's password.
func (h *PortalAuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req PortalChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	changeReq := &portalauth.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}

	if err := h.portalAuthService.ChangePassword(
		c.Request.Context(),
		userID.(int64),
		changeReq,
		h.authService.VerifyPassword,
		h.authService.HashPassword,
	); err != nil {
		h.handleError(c, err)
		return
	}

	uid := userID.(int64)
	emitAudit(c, h.auditService, monitor.AuditEntry{
		UserID:       &uid,
		Action:       monitor.ActionPasswordChange,
		ResourceType: monitor.ResourceUser,
		ResourceID:   strconv.FormatInt(uid, 10),
		Details:      map[string]any{"source": "portal"},
	})

	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// Enable2FA initiates 2FA setup.
func (h *PortalAuthHandler) Enable2FA(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	secret, backupCodes, err := h.portalAuthService.Setup2FA(c.Request.Context(), userID.(int64), h.authService.GenerateTOTPSecret)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret":       secret,
		"backup_codes": backupCodes,
	})
}

// Portal2FAVerifyRequest represents a 2FA verification request.
type Portal2FAVerifyRequest struct {
	Code string `json:"code" binding:"required"`
}

// Verify2FA verifies and enables 2FA.
func (h *PortalAuthHandler) Verify2FA(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req Portal2FAVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.portalAuthService.Enable2FA(c.Request.Context(), userID.(int64), req.Code, h.authService.VerifyTOTP); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已启用"})
}

// Portal2FALoginRequest represents a 2FA login verification request.
type Portal2FALoginRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	Code   string `json:"code" binding:"required"`
}

// Verify2FALogin verifies 2FA code during login.
func (h *PortalAuthHandler) Verify2FALogin(c *gin.Context) {
	var req Portal2FALoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	twoFactorReq := &portalauth.TwoFactorRequest{
		UserID: req.UserID,
		Code:   req.Code,
	}

	result, err := h.portalAuthService.Verify2FA(
		c.Request.Context(),
		twoFactorReq,
		h.authService.VerifyTOTP,
		h.authService.GenerateToken,
	)
	if err != nil {
		h.handleError(c, err)
		return
	}

	h.updateLastLogin(c, result.UserID)

	c.JSON(http.StatusOK, gin.H{
		"token": result.Token,
		"user": gin.H{
			"id":                    result.UserID,
			"username":              result.Username,
			"email":                 result.Email,
			"role":                  result.Role,
			"permissions":           h.getRolePermissions(c, result.Role),
			"force_password_change": result.ForcePasswordChange,
		},
	})
}

// PortalDisable2FARequest represents a 2FA disable request.
type PortalDisable2FARequest struct {
	Password string `json:"password" binding:"required"`
}

// Disable2FA disables 2FA for the current user.
func (h *PortalAuthHandler) Disable2FA(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	var req PortalDisable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.portalAuthService.Disable2FA(c.Request.Context(), userID.(int64), req.Password, h.authService.VerifyPassword); err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "两步验证已禁用"})
}

// handleError handles errors and returns appropriate HTTP responses.
func (h *PortalAuthHandler) handleError(c *gin.Context, err error) {
	// Check error type using errors package
	if appErr, ok := pkgerrors.AsAppError(err); ok {
		switch appErr.Code {
		case pkgerrors.ErrCodeValidation, pkgerrors.ErrCodeBadRequest:
			c.JSON(http.StatusBadRequest, gin.H{"error": appErr.Message})
		case pkgerrors.ErrCodeUnauthorized:
			c.JSON(http.StatusUnauthorized, gin.H{"error": firstNonEmpty(appErr.Message, "未授权操作")})
		case pkgerrors.ErrCodeForbidden:
			c.JSON(http.StatusForbidden, gin.H{"error": appErr.Message})
		case pkgerrors.ErrCodeConflict:
			c.JSON(http.StatusConflict, gin.H{"error": appErr.Message})
		case pkgerrors.ErrCodeRateLimit:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
		case pkgerrors.ErrCodeNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "请求的资源不存在或已失效"})
		default:
			h.logger.Error("portal auth error", logger.F("error", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误，请稍后重试"})
		}
		return
	}

	// Fallback to string matching for non-AppError errors
	errStr := err.Error()
	switch {
	case contains(errStr, "validation"):
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
	case contains(errStr, "unauthorized"), contains(errStr, "密码错误"), contains(errStr, "验证码错误"), contains(errStr, "备份码错误"), contains(errStr, "invalid credentials"):
		c.JSON(http.StatusUnauthorized, gin.H{"error": errStr})
	case contains(errStr, "forbidden"), contains(errStr, "禁用"), contains(errStr, "disabled"):
		c.JSON(http.StatusForbidden, gin.H{"error": "账号已被禁用，请联系管理员"})
	case contains(errStr, "conflict"), contains(errStr, "已存在"), contains(errStr, "already exists"):
		c.JSON(http.StatusConflict, gin.H{"error": errStr})
	case contains(errStr, "rate limit"), contains(errStr, "过于频繁"), contains(errStr, "too many"):
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "登录尝试过于频繁，请稍后再试"})
	case contains(errStr, "not found"):
		c.JSON(http.StatusNotFound, gin.H{"error": "请求的资源不存在或已失效"})
	case contains(errStr, "expired"), contains(errStr, "过期"):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号已过期，请续费"})
	default:
		h.logger.Error("portal auth error", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "服务器内部错误，请稍后重试"})
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsLower(s, substr))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
