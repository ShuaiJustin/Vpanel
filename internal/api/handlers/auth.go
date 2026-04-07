// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/settings"
	"v/pkg/errors"
)

// emailRegex is a simple regex for email validation.
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// isValidEmail validates email format.
func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// AuthHandler handles authentication-related requests.
type AuthHandler struct {
	authService              *auth.Service
	userRepo                 repository.UserRepository
	roleRepo                 repository.RoleRepository
	loginHistoryRepo         repository.LoginHistoryRepository
	entitlementService       *entitlement.Service
	logger                   logger.Logger
	settingsService          *settings.Service
	loginRateLimiter         *auth.RateLimiter
	loginRateLimiterConfig   auth.RateLimiterConfig
	loginRateLimiterConfigMu sync.Mutex
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *auth.Service, userRepo repository.UserRepository, loginHistoryRepo repository.LoginHistoryRepository, log logger.Logger) *AuthHandler {
	return &AuthHandler{
		authService:      authService,
		userRepo:         userRepo,
		loginHistoryRepo: loginHistoryRepo,
		logger:           log,
	}
}

// WithSecuritySettings enables login security controls driven by persisted settings.
func (h *AuthHandler) WithSecuritySettings(settingsService *settings.Service) *AuthHandler {
	h.settingsService = settingsService
	return h
}

// WithEntitlementService enables runtime cleanup reconciliation after admin access changes.
func (h *AuthHandler) WithEntitlementService(entitlementService *entitlement.Service) *AuthHandler {
	h.entitlementService = entitlementService
	return h
}

func (h *AuthHandler) reconcileRevokedUserRuntime(ctx context.Context, userID int64) {
	if h == nil || h.entitlementService == nil || userID <= 0 {
		return
	}
	if _, err := h.entitlementService.EvaluateExistingAccess(ctx, userID); err != nil && !errors.IsForbidden(err) {
		h.logger.Warn("failed to reconcile revoked user runtime resources",
			logger.Err(err),
			logger.UserID(userID),
		)
	}
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token        string        `json:"token"`
	RefreshToken string        `json:"refresh_token"`
	ExpiresIn    int64         `json:"expires_in"`
	User         *UserResponse `json:"user"`
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	ID                  int64    `json:"id"`
	Username            string   `json:"username"`
	Email               string   `json:"email,omitempty"`
	Role                string   `json:"role"`
	Permissions         []string `json:"permissions,omitempty"`
	Status              bool     `json:"status"`
	CreatedAt           string   `json:"created_at"`
	LastLogin           string   `json:"last_login,omitempty"`
	ForcePasswordChange bool     `json:"force_password_change"`
}

func buildUserResponse(user *repository.User) UserResponse {
	response := UserResponse{
		ID:                  user.ID,
		Username:            user.Username,
		Email:               user.Email,
		Role:                user.Role,
		Status:              user.Enabled,
		CreatedAt:           user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		ForcePasswordChange: user.ForcePasswordChange,
	}

	if user.LastLoginAt != nil {
		response.LastLogin = user.LastLoginAt.Format("2006-01-02T15:04:05Z")
	}

	return response
}

// WithRoleRepository enables role validation and permission enrichment.
func (h *AuthHandler) WithRoleRepository(roleRepo repository.RoleRepository) *AuthHandler {
	h.roleRepo = roleRepo
	return h
}

func (h *AuthHandler) getRolePermissions(ctx context.Context, roleName string) []string {
	if roleName == "" {
		return nil
	}
	if roleName == "admin" {
		return []string{"*"}
	}
	if h == nil || h.roleRepo == nil {
		return nil
	}

	role, err := h.roleRepo.GetByName(ctx, roleName)
	if err != nil || role == nil {
		return nil
	}

	perms, err := role.GetPermissionsList()
	if err != nil {
		return nil
	}
	return perms
}

func (h *AuthHandler) userResponse(ctx context.Context, user *repository.User) UserResponse {
	response := buildUserResponse(user)
	response.Permissions = h.getRolePermissions(ctx, user.Role)
	return response
}

func (h *AuthHandler) roleExists(ctx context.Context, roleName string) (bool, error) {
	roleName = strings.TrimSpace(roleName)
	if roleName == "" {
		return false, nil
	}
	if h == nil || h.roleRepo == nil {
		switch roleName {
		case "admin", "user", "viewer":
			return true, nil
		default:
			return false, nil
		}
	}

	role, err := h.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return false, err
	}
	return role != nil, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

func (h *AuthHandler) emailBelongsToAnotherUser(ctx context.Context, email string, userID int64) (bool, error) {
	if email == "" {
		return false, nil
	}

	existing, err := h.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return existing.ID != userID, nil
}

func (h *AuthHandler) listAllUsers(ctx context.Context) ([]*repository.User, error) {
	total, err := h.userRepo.Count(ctx)
	if err != nil {
		return nil, err
	}
	if total == 0 {
		return []*repository.User{}, nil
	}
	return h.userRepo.List(ctx, int(total), 0)
}

type filteredUserRepository interface {
	ListFiltered(ctx context.Context, filter repository.UserListFilter) ([]*repository.User, int64, error)
}

type filteredUserSummaryRepository interface {
	GetFilteredSummary(ctx context.Context, filter repository.UserListFilter) (repository.UserListSummary, error)
}

func currentUserIDFromContext(c *gin.Context) (int64, bool) {
	if c == nil {
		return 0, false
	}

	value, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	userID, ok := value.(int64)
	if !ok || userID <= 0 {
		return 0, false
	}

	return userID, true
}

func (h *AuthHandler) countEnabledAdmins(ctx context.Context) (int, error) {
	users, err := h.listAllUsers(ctx)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, user := range users {
		if user.Enabled && user.Role == "admin" {
			count++
		}
	}

	return count, nil
}

func (h *AuthHandler) ensureEnabledAdminRemains(ctx *gin.Context, user *repository.User, nextRole string, nextEnabled bool, action string) bool {
	if user.Role != "admin" || !user.Enabled || (nextRole == "admin" && nextEnabled) {
		return true
	}

	adminCount, err := h.countEnabledAdmins(ctx.Request.Context())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify admin protection"})
		return false
	}

	if adminCount <= 1 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Cannot " + action + " the last enabled admin"})
		return false
	}

	return true
}

func (h *AuthHandler) reconcileRestoredUserRuntime(ctx context.Context, userID int64) {
	if h == nil || h.entitlementService == nil || userID <= 0 {
		return
	}

	if _, _, err := h.entitlementService.GetAccessibleProxies(ctx, userID); err != nil && !errors.IsForbidden(err) {
		h.logger.Warn("failed to reconcile restored user runtime resources",
			logger.Err(err),
			logger.UserID(userID),
		)
	}
}

func (h *AuthHandler) updateLastLogin(ctx context.Context, user *repository.User, ip string) {
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.LastLoginIP = ip

	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Warn("failed to update user last login", logger.F("user_id", user.ID), logger.F("error", err))
	}
}

type authSecurityPolicy struct {
	tokenExpiry       time.Duration
	enableIPWhitelist bool
	ipWhitelist       string
	enableLoginLock   bool
	maxLoginAttempts  int
	lockDuration      time.Duration
}

func (h *AuthHandler) loadSecurityPolicy(ctx context.Context) authSecurityPolicy {
	policy := authSecurityPolicy{
		tokenExpiry:      h.authService.TokenExpiry(),
		enableLoginLock:  false,
		maxLoginAttempts: 5,
		lockDuration:     10 * time.Minute,
	}

	if h.settingsService == nil {
		return policy
	}

	systemSettings, err := h.settingsService.GetSystemSettings(ctx)
	if err != nil {
		h.logger.Warn("failed to load auth security settings", logger.Err(err))
		return policy
	}

	if systemSettings.SessionTimeout > 0 {
		policy.tokenExpiry = time.Duration(systemSettings.SessionTimeout) * time.Minute
	}
	policy.enableIPWhitelist = systemSettings.EnableIPWhitelist
	policy.ipWhitelist = systemSettings.IPWhitelist
	policy.enableLoginLock = systemSettings.EnableLoginLock
	if systemSettings.MaxLoginAttempts > 0 {
		policy.maxLoginAttempts = systemSettings.MaxLoginAttempts
	}
	if systemSettings.LockDuration > 0 {
		policy.lockDuration = time.Duration(systemSettings.LockDuration) * time.Minute
	}

	return policy
}

func (h *AuthHandler) getLoginRateLimiter(policy authSecurityPolicy) *auth.RateLimiter {
	h.loginRateLimiterConfigMu.Lock()
	defer h.loginRateLimiterConfigMu.Unlock()

	if !policy.enableLoginLock {
		if h.loginRateLimiter != nil {
			h.loginRateLimiter.Stop()
			h.loginRateLimiter = nil
			h.loginRateLimiterConfig = auth.RateLimiterConfig{}
		}
		return nil
	}

	config := auth.RateLimiterConfig{
		MaxAttempts:     policy.maxLoginAttempts,
		Window:          time.Minute,
		BlockDuration:   policy.lockDuration,
		CleanupInterval: maxDuration(5*time.Minute, policy.lockDuration),
	}

	if h.loginRateLimiter == nil || h.loginRateLimiterConfig != config {
		if h.loginRateLimiter != nil {
			h.loginRateLimiter.Stop()
		}
		h.loginRateLimiter = auth.NewRateLimiter(config)
		h.loginRateLimiterConfig = config
	}

	return h.loginRateLimiter
}

func (h *AuthHandler) recordRateLimitedLoginAttempt(ctx context.Context, limiter *auth.RateLimiter, ip string, success bool) {
	if limiter == nil {
		return
	}
	if err := limiter.RecordLoginAttempt(ctx, ip, success); err != nil {
		h.logger.Warn("failed to record login rate limit attempt", logger.F("ip", ip), logger.Err(err))
	}
}

func (h *AuthHandler) issueAccessToken(user *repository.User, expiry time.Duration) (string, int64, error) {
	token, err := h.authService.GenerateTokenWithExpiry(user.ID, user.Username, user.Role, expiry)
	if err != nil {
		return "", 0, err
	}

	return token, int64(expiry.Seconds()), nil
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, "请求参数无效，请检查用户名和密码")
		return
	}

	// Get client info for login history
	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()
	policy := h.loadSecurityPolicy(c.Request.Context())

	if policy.enableIPWhitelist && !isIPAllowedByWhitelist(clientIP, policy.ipWhitelist) {
		h.logger.Warn("login rejected by IP whitelist", logger.F("ip", clientIP), logger.F("username", req.Username))
		middleware.HandleForbidden(c, "当前 IP 不在管理后台白名单中")
		return
	}

	loginRateLimiter := h.getLoginRateLimiter(policy)
	if loginRateLimiter != nil {
		allowed, err := loginRateLimiter.CheckRateLimit(c.Request.Context(), clientIP)
		if err != nil {
			h.logger.Warn("login rejected by rate limiter", logger.F("ip", clientIP), logger.F("username", req.Username), logger.Err(err))
			middleware.HandleError(c, err)
			return
		}
		if !allowed {
			rateLimitErr := errors.NewRateLimitError("登录尝试次数过多，请稍后再试")
			h.logger.Warn("login rejected by rate limiter", logger.F("ip", clientIP), logger.F("username", req.Username))
			middleware.HandleError(c, rateLimitErr)
			return
		}
	}

	// Authenticate user
	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
		h.recordRateLimitedLoginAttempt(c.Request.Context(), loginRateLimiter, clientIP, false)
		h.logger.Warn("login failed: user not found", logger.F("username", req.Username))
		middleware.HandleUnauthorized(c, errors.MsgInvalidCredentials)
		return
	}

	// Helper to record login attempt
	recordLogin := func(success bool) {
		if h.loginHistoryRepo != nil {
			history := &repository.LoginHistory{
				UserID:    user.ID,
				IP:        clientIP,
				UserAgent: userAgent,
				Success:   success,
			}
			if err := h.loginHistoryRepo.Create(c.Request.Context(), history); err != nil {
				h.logger.Warn("failed to record login history", logger.F("error", err))
			}
		}
	}

	// Check if user is enabled
	if !user.Enabled {
		h.recordRateLimitedLoginAttempt(c.Request.Context(), loginRateLimiter, clientIP, false)
		h.logger.Warn("login failed: user disabled", logger.F("username", req.Username))
		recordLogin(false)
		middleware.HandleForbidden(c, errors.MsgUserDisabled)
		return
	}

	if !h.authService.VerifyPassword(req.Password, user.PasswordHash) {
		h.recordRateLimitedLoginAttempt(c.Request.Context(), loginRateLimiter, clientIP, false)
		h.logger.Warn("login failed: invalid password", logger.F("username", req.Username))
		recordLogin(false)
		middleware.HandleUnauthorized(c, errors.MsgInvalidCredentials)
		return
	}

	// Generate tokens
	token, expiresIn, err := h.issueAccessToken(user, policy.tokenExpiry)
	if err != nil {
		h.logger.Error("failed to generate token", logger.F("error", err))
		middleware.HandleInternalError(c, "登录失败，请稍后重试", err)
		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(user.ID)
	if err != nil {
		h.logger.Error("failed to generate refresh token", logger.F("error", err))
		middleware.HandleInternalError(c, "登录失败，请稍后重试", err)
		return
	}

	// Record successful login
	h.recordRateLimitedLoginAttempt(c.Request.Context(), loginRateLimiter, clientIP, true)
	recordLogin(true)
	h.updateLastLogin(c.Request.Context(), user, clientIP)

	h.logger.Info("user logged in", logger.F("username", req.Username), logger.F("user_id", user.ID))

	c.JSON(http.StatusOK, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User:         func() *UserResponse { resp := h.userResponse(c.Request.Context(), user); return &resp }(),
	})
}

// RefreshTokenRequest represents a refresh token request.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken handles token refresh.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
		return
	}

	policy := h.loadSecurityPolicy(c.Request.Context())
	if policy.enableIPWhitelist && !isIPAllowedByWhitelist(c.ClientIP(), policy.ipWhitelist) {
		h.logger.Warn("refresh token rejected by IP whitelist", logger.F("ip", c.ClientIP()))
		middleware.HandleForbidden(c, "当前 IP 不在管理后台白名单中")
		return
	}

	// Validate refresh token
	claims, err := h.authService.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		middleware.HandleUnauthorized(c, errors.MsgRefreshTokenExpired)
		return
	}

	// Get user
	user, err := h.userRepo.GetByID(c.Request.Context(), claims.UserID)
	if err != nil {
		middleware.HandleUnauthorized(c, errors.MsgUserNotFound)
		return
	}
	if !user.Enabled {
		middleware.HandleForbidden(c, errors.MsgUserDisabled)
		return
	}

	// Generate new tokens
	token, expiresIn, err := h.issueAccessToken(user, policy.tokenExpiry)
	if err != nil {
		middleware.HandleInternalError(c, "刷新令牌失败，请重新登录", err)
		return
	}

	refreshToken, err := h.authService.GenerateRefreshToken(user.ID)
	if err != nil {
		middleware.HandleInternalError(c, "刷新令牌失败，请重新登录", err)
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User:         func() *UserResponse { resp := h.userResponse(c.Request.Context(), user); return &resp }(),
	})
}

func maxDuration(values ...time.Duration) time.Duration {
	var max time.Duration
	for _, value := range values {
		if value > max {
			max = value
		}
	}
	return max
}

// Logout handles user logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	// For stateful sessions, we would invalidate the token here
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetCurrentUser returns the current authenticated user.
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, ok := currentUserIDFromContext(c)
	if !ok {
		middleware.HandleUnauthorized(c, errors.MsgUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		middleware.HandleNotFound(c, "user", userID)
		return
	}

	c.JSON(http.StatusOK, h.userResponse(c.Request.Context(), user))
}

// UpdateCurrentUserRequest represents a current-user profile update request.
type UpdateCurrentUserRequest struct {
	Email *string `json:"email"`
}

// UpdateCurrentUser updates the current authenticated user's profile.
func (h *AuthHandler) UpdateCurrentUser(c *gin.Context) {
	var req UpdateCurrentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, "请求参数无效")
		return
	}

	userID, ok := currentUserIDFromContext(c)
	if !ok {
		middleware.HandleUnauthorized(c, errors.MsgUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		middleware.HandleNotFound(c, "user", userID)
		return
	}

	if req.Email != nil {
		email := normalizeEmail(*req.Email)
		if email != "" && !isValidEmail(email) {
			middleware.HandleBadRequest(c, "请输入正确的邮箱地址")
			return
		}

		inUse, err := h.emailBelongsToAnotherUser(c.Request.Context(), email, user.ID)
		if err != nil {
			middleware.HandleInternalError(c, "更新个人资料失败，请稍后重试", err)
			return
		}
		if inUse {
			c.JSON(http.StatusConflict, gin.H{"error": "该邮箱已被其他账号使用"})
			return
		}

		if email != normalizeEmail(user.Email) {
			user.EmailVerified = false
			user.EmailVerifiedAt = nil
		}
		user.Email = email
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		middleware.HandleInternalError(c, "更新个人资料失败，请稍后重试", err)
		return
	}

	h.logger.Info("current user updated", logger.F("user_id", user.ID))
	c.JSON(http.StatusOK, h.userResponse(c.Request.Context(), user))
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

// ChangePassword handles password change.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, "请求参数无效，密码长度至少6位")
		return
	}

	userID, ok := currentUserIDFromContext(c)
	if !ok {
		middleware.HandleUnauthorized(c, errors.MsgUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		middleware.HandleNotFound(c, "user", userID)
		return
	}

	// Verify old password
	if !h.authService.VerifyPassword(req.OldPassword, user.PasswordHash) {
		middleware.HandleBadRequest(c, errors.MsgOldPasswordIncorrect)
		return
	}

	// Hash new password
	newHash, err := h.authService.HashPassword(req.NewPassword)
	if err != nil {
		middleware.HandleInternalError(c, "密码修改失败，请稍后重试", err)
		return
	}

	// Update password
	user.PasswordHash = newHash
	user.ForcePasswordChange = false
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		middleware.HandleInternalError(c, "密码修改失败，请稍后重试", err)
		return
	}

	h.logger.Info("password changed", logger.F("user_id", userID))
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// ListUsers returns all users (admin only).
func (h *AuthHandler) ListUsers(c *gin.Context) {
	page := 1
	pageSize := 20
	if value := c.Query("page"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if value := c.Query("page_size"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 200 {
			pageSize = parsed
		}
	} else if value := c.Query("pageSize"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 && parsed <= 200 {
			pageSize = parsed
		}
	}

	search := strings.ToLower(normalizeUsername(c.Query("search")))
	roleFilter := strings.TrimSpace(c.Query("role"))
	statusFilter := strings.TrimSpace(c.Query("status"))

	clampPage := func(total int64) int {
		if total <= 0 {
			return 1
		}
		maxPage := int((total + int64(pageSize) - 1) / int64(pageSize))
		if maxPage < 1 {
			maxPage = 1
		}
		if page > maxPage {
			return maxPage
		}
		return page
	}

	filteredUserCount := 0
	enabledUserCount := 0
	disabledUserCount := 0
	adminUserCount := 0

	var (
		pagedUsers []*repository.User
		total      int64
		err        error
	)

	if repo, ok := h.userRepo.(filteredUserRepository); ok {
		filter := repository.UserListFilter{
			Search: search,
			Role:   roleFilter,
			Status: statusFilter,
			Limit:  pageSize,
			Offset: (page - 1) * pageSize,
		}
		pagedUsers, total, err = repo.ListFiltered(c.Request.Context(), filter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
			return
		}
		if adjustedPage := clampPage(total); adjustedPage != page {
			page = adjustedPage
			filter.Offset = (page - 1) * pageSize
			pagedUsers, total, err = repo.ListFiltered(c.Request.Context(), filter)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
				return
			}
		}
		if summaryRepo, ok := h.userRepo.(filteredUserSummaryRepository); ok {
			summary, summaryErr := summaryRepo.GetFilteredSummary(c.Request.Context(), repository.UserListFilter{
				Search: search,
				Role:   roleFilter,
				Status: statusFilter,
			})
			if summaryErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to summarize users"})
				return
			}
			filteredUserCount = int(summary.Total)
			enabledUserCount = int(summary.Enabled)
			disabledUserCount = int(summary.Disabled)
			adminUserCount = int(summary.Admin)
		} else {
			filteredUserCount = int(total)
			for _, user := range pagedUsers {
				if user == nil {
					continue
				}
				if user.Enabled {
					enabledUserCount++
				} else {
					disabledUserCount++
				}
				if user.Role == "admin" {
					adminUserCount++
				}
			}
		}
	} else {
		users, listErr := h.listAllUsers(c.Request.Context())
		if listErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
			return
		}

		filteredUsers := make([]*repository.User, 0, len(users))
		for _, user := range users {
			if user == nil {
				continue
			}
			if roleFilter != "" && user.Role != roleFilter {
				continue
			}
			if statusFilter == "enabled" && !user.Enabled {
				continue
			}
			if statusFilter == "disabled" && user.Enabled {
				continue
			}
			if search != "" {
				username := strings.ToLower(normalizeUsername(user.Username))
				email := strings.ToLower(strings.TrimSpace(user.Email))
				if !strings.Contains(username, search) && !strings.Contains(email, search) {
					continue
				}
			}
			filteredUsers = append(filteredUsers, user)
		}

		sort.Slice(filteredUsers, func(i, j int) bool {
			if filteredUsers[i].CreatedAt.Equal(filteredUsers[j].CreatedAt) {
				return filteredUsers[i].ID > filteredUsers[j].ID
			}
			return filteredUsers[i].CreatedAt.After(filteredUsers[j].CreatedAt)
		})

		for _, user := range filteredUsers {
			filteredUserCount++
			if user.Enabled {
				enabledUserCount++
			} else {
				disabledUserCount++
			}
			if user.Role == "admin" {
				adminUserCount++
			}
		}

		total = int64(filteredUserCount)
		page = clampPage(total)
		start := (page - 1) * pageSize
		if start > filteredUserCount {
			start = filteredUserCount
		}
		end := start + pageSize
		if end > filteredUserCount {
			end = filteredUserCount
		}
		pagedUsers = filteredUsers[start:end]
	}

	response := make([]UserResponse, len(pagedUsers))
	for i, user := range pagedUsers {
		response[i] = h.userResponse(c.Request.Context(), user)
	}

	currentUserID, _ := currentUserIDFromContext(c)
	c.JSON(http.StatusOK, gin.H{
		"users":           response,
		"total":           total,
		"page":            page,
		"page_size":       pageSize,
		"admin_total":     adminUserCount,
		"enabled_total":   enabledUserCount,
		"disabled_total":  disabledUserCount,
		"current_user_id": currentUserID,
	})
}

// CreateUserRequest represents a create user request.
type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// CreateUser creates a new user (admin only).
func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	req.Username = normalizeUsername(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username cannot be empty"})
		return
	}
	if len(req.Username) < 3 || len(req.Username) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username length must be between 3 and 50 characters"})
		return
	}

	req.Email = normalizeEmail(req.Email)

	// Validate email if provided
	if req.Email != "" && !isValidEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	if req.Email != "" {
		inUse, err := h.emailBelongsToAnotherUser(c.Request.Context(), req.Email, 0)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate email uniqueness"})
			return
		}
		if inUse {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}
	}

	// Check if username exists
	existing, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}
	if err != nil && !errors.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate username uniqueness"})
		return
	}

	// Hash password
	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Set default role
	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "user"
	}
	roleFound, roleErr := h.roleExists(c.Request.Context(), role)
	if roleErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate role"})
		return
	}
	if !roleFound {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not exist"})
		return
	}

	// Create user
	user := &repository.User{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Role:         role,
	}

	if err := h.userRepo.Create(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	h.logger.Info("user created", logger.F("username", req.Username), logger.F("user_id", user.ID))

	c.JSON(http.StatusCreated, h.userResponse(c.Request.Context(), user))
}

// GetUser returns a user by ID (admin only).
func (h *AuthHandler) GetUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	c.JSON(http.StatusOK, h.userResponse(c.Request.Context(), user))
}

// UpdateUserRequest represents an update user request.
type UpdateUserRequest struct {
	Username *string `json:"username"`
	Email    *string `json:"email"`
	Role     *string `json:"role"`
	Password *string `json:"password"`
}

// UpdateUser updates a user (admin only).
func (h *AuthHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	// Update fields
	if req.Username != nil {
		username := normalizeUsername(*req.Username)
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username cannot be empty"})
			return
		}
		if len(username) < 3 || len(username) > 50 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username length must be between 3 and 50 characters"})
			return
		}

		// Check if new username is already taken
		existing, err := h.userRepo.GetByUsername(c.Request.Context(), username)
		if err == nil && existing != nil && existing.ID != id {
			c.JSON(http.StatusConflict, errors.NewConflictError("user", "username", username).ToResponse(""))
			return
		}
		if err != nil && !errors.IsNotFound(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate username uniqueness"})
			return
		}

		user.Username = username
	}
	if req.Email != nil {
		email := normalizeEmail(*req.Email)
		if email != "" && !isValidEmail(email) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
			return
		}

		inUse, err := h.emailBelongsToAnotherUser(c.Request.Context(), email, user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate email uniqueness"})
			return
		}
		if inUse {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
			return
		}

		if email != normalizeEmail(user.Email) {
			user.EmailVerified = false
			user.EmailVerifiedAt = nil
		}
		user.Email = email
	}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if role == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role cannot be empty"})
			return
		}
		roleFound, roleErr := h.roleExists(c.Request.Context(), role)
		if roleErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to validate role"})
			return
		}
		if !roleFound {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role does not exist"})
			return
		}
		if currentUserID, ok := currentUserIDFromContext(c); ok && currentUserID == user.ID && role != user.Role {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot change your own role"})
			return
		}
		if !h.ensureEnabledAdminRemains(c, user, role, user.Enabled, "demote") {
			return
		}

		user.Role = role
	}
	if req.Password != nil && *req.Password != "" {
		if len(*req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 6 characters"})
			return
		}

		passwordHash, err := h.authService.HashPassword(*req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
			return
		}
		user.PasswordHash = passwordHash
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	h.logger.Info("user updated", logger.F("user_id", id))

	c.JSON(http.StatusOK, h.userResponse(c.Request.Context(), user))
}

// DeleteUser deletes a user (admin only).
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent self-deletion
	currentUserID, ok := currentUserIDFromContext(c)
	if ok && currentUserID == id {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete yourself"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user"})
		return
	}

	if !h.ensureEnabledAdminRemains(c, user, user.Role, false, "delete") {
		return
	}

	if err := h.userRepo.Delete(c.Request.Context(), id); err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	h.logger.Info("user deleted", logger.F("user_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// EnableUser enables a user account (admin only).
func (h *AuthHandler) EnableUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	if user.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "User is already enabled"})
		return
	}

	user.Enabled = true
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to enable user", err).ToResponse(""))
		return
	}
	h.reconcileRestoredUserRuntime(c.Request.Context(), id)

	h.logger.Info("user enabled", logger.F("user_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "User enabled successfully"})
}

// DisableUser disables a user account (admin only).
func (h *AuthHandler) DisableUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	// Prevent self-disable
	currentUserID, ok := currentUserIDFromContext(c)
	if ok && currentUserID == id {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Cannot disable yourself", nil).ToResponse(""))
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	if !user.Enabled {
		c.JSON(http.StatusOK, gin.H{"message": "User is already disabled"})
		return
	}

	if !h.ensureEnabledAdminRemains(c, user, user.Role, false, "disable") {
		return
	}

	user.Enabled = false
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to disable user", err).ToResponse(""))
		return
	}
	h.reconcileRevokedUserRuntime(c.Request.Context(), id)

	h.logger.Info("user disabled", logger.F("user_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "User disabled successfully"})
}

// ResetPasswordResponse represents a password reset response.
type ResetPasswordResponse struct {
	TemporaryPassword string `json:"temporary_password"`
	Message           string `json:"message"`
}

// ResetPassword resets a user's password (admin only).
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	// Generate temporary password
	tempPassword := h.authService.GenerateTemporaryPassword()

	// Hash the temporary password
	passwordHash, err := h.authService.HashPassword(tempPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewInternalError("Failed to hash password", err).ToResponse(""))
		return
	}

	// Update user
	user.PasswordHash = passwordHash
	user.ForcePasswordChange = true
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to reset password", err).ToResponse(""))
		return
	}

	h.logger.Info("password reset", logger.F("user_id", id))
	c.JSON(http.StatusOK, ResetPasswordResponse{
		TemporaryPassword: tempPassword,
		Message:           "Password reset successfully. User must change password on next login.",
	})
}

// ExtendedUserResponse includes all user fields for admin views.
type ExtendedUserResponse struct {
	ID                  int64   `json:"id"`
	Username            string  `json:"username"`
	Email               string  `json:"email,omitempty"`
	Role                string  `json:"role"`
	Enabled             bool    `json:"enabled"`
	TrafficLimit        int64   `json:"traffic_limit"`
	TrafficUsed         int64   `json:"traffic_used"`
	ExpiresAt           *string `json:"expires_at,omitempty"`
	ForcePasswordChange bool    `json:"force_password_change"`
	CreatedAt           string  `json:"created_at"`
	UpdatedAt           string  `json:"updated_at"`
}

// GetUserExtended returns extended user info (admin only).
func (h *AuthHandler) GetUserExtended(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	response := ExtendedUserResponse{
		ID:                  user.ID,
		Username:            user.Username,
		Email:               user.Email,
		Role:                user.Role,
		Enabled:             user.Enabled,
		TrafficLimit:        user.TrafficLimit,
		TrafficUsed:         user.TrafficUsed,
		ForcePasswordChange: user.ForcePasswordChange,
		CreatedAt:           user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:           user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if user.ExpiresAt != nil {
		expiresAt := user.ExpiresAt.Format("2006-01-02T15:04:05Z")
		response.ExpiresAt = &expiresAt
	}

	c.JSON(http.StatusOK, response)
}

// LoginHistoryResponse represents a login history entry in API responses.
type LoginHistoryResponse struct {
	ID        int64  `json:"id"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Success   bool   `json:"success"`
	CreatedAt string `json:"created_at"`
}

// LoginHistoryListResponse represents a paginated list of login history.
type LoginHistoryListResponse struct {
	Items []LoginHistoryResponse `json:"items"`
	Total int64                  `json:"total"`
}

// GetLoginHistory returns login history for a user (admin only).
func (h *AuthHandler) GetLoginHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	// Check if user exists
	_, err = h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	// Get pagination params
	limit := 50
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get login history
	histories, err := h.loginHistoryRepo.GetByUserID(c.Request.Context(), id, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get login history", err).ToResponse(""))
		return
	}

	// Get total count
	total, err := h.loginHistoryRepo.Count(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to count login history", err).ToResponse(""))
		return
	}

	// Build response
	items := make([]LoginHistoryResponse, len(histories))
	for i, hist := range histories {
		items[i] = LoginHistoryResponse{
			ID:        hist.ID,
			IP:        hist.IP,
			UserAgent: hist.UserAgent,
			Success:   hist.Success,
			CreatedAt: hist.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, LoginHistoryListResponse{
		Items: items,
		Total: total,
	})
}

// ClearLoginHistory clears login history for a user (admin only).
func (h *AuthHandler) ClearLoginHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	// Check if user exists
	_, err = h.userRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, errors.NewNotFoundError("user", id).ToResponse(""))
			return
		}
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to get user", err).ToResponse(""))
		return
	}

	// Delete login history
	if err := h.loginHistoryRepo.DeleteByUserID(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to clear login history", err).ToResponse(""))
		return
	}

	h.logger.Info("login history cleared", logger.F("user_id", id))
	c.JSON(http.StatusOK, gin.H{"message": "Login history cleared successfully"})
}
