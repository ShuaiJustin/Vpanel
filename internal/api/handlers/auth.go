// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
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
	authService      *auth.Service
	userRepo         repository.UserRepository
	loginHistoryRepo repository.LoginHistoryRepository
	logger           logger.Logger
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
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role"`
	Status    bool   `json:"status"`
	CreatedAt string `json:"created_at"`
	LastLogin string `json:"last_login,omitempty"`
}

func buildUserResponse(user *repository.User) UserResponse {
	response := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		Status:    user.Enabled,
		CreatedAt: user.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if user.LastLoginAt != nil {
		response.LastLogin = user.LastLoginAt.Format("2006-01-02T15:04:05Z")
	}

	return response
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

func (h *AuthHandler) updateLastLogin(ctx context.Context, user *repository.User, ip string) {
	now := time.Now().UTC()
	user.LastLoginAt = &now
	user.LastLoginIP = ip

	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Warn("failed to update user last login", logger.F("user_id", user.ID), logger.F("error", err))
	}
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

	// Authenticate user
	user, err := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if err != nil {
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
		h.logger.Warn("login failed: user disabled", logger.F("username", req.Username))
		recordLogin(false)
		middleware.HandleForbidden(c, errors.MsgUserDisabled)
		return
	}

	if !h.authService.VerifyPassword(req.Password, user.PasswordHash) {
		h.logger.Warn("login failed: invalid password", logger.F("username", req.Username))
		recordLogin(false)
		middleware.HandleUnauthorized(c, errors.MsgInvalidCredentials)
		return
	}

	// Generate tokens
	token, err := h.authService.GenerateToken(user.ID, user.Username, user.Role)
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
	recordLogin(true)
	h.updateLastLogin(c.Request.Context(), user, clientIP)

	h.logger.Info("user logged in", logger.F("username", req.Username), logger.F("user_id", user.ID))

	c.JSON(http.StatusOK, LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		ExpiresIn:    3600, // 1 hour
		User:         func() *UserResponse { resp := buildUserResponse(user); return &resp }(),
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

	// Generate new tokens
	token, err := h.authService.GenerateToken(user.ID, user.Username, user.Role)
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
		ExpiresIn:    3600,
		User:         func() *UserResponse { resp := buildUserResponse(user); return &resp }(),
	})
}

// Logout handles user logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	// In a stateless JWT system, logout is handled client-side
	// For stateful sessions, we would invalidate the token here
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// GetCurrentUser returns the current authenticated user.
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		middleware.HandleUnauthorized(c, errors.MsgUnauthorized)
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
	if err != nil {
		middleware.HandleNotFound(c, "user", userID)
		return
	}

	c.JSON(http.StatusOK, buildUserResponse(user))
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

	userID, _ := c.Get("user_id")
	user, err := h.userRepo.GetByID(c.Request.Context(), userID.(int64))
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
	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		middleware.HandleInternalError(c, "密码修改失败，请稍后重试", err)
		return
	}

	h.logger.Info("password changed", logger.F("user_id", userID))
	c.JSON(http.StatusOK, gin.H{"message": "密码修改成功"})
}

// ListUsers returns all users (admin only).
func (h *AuthHandler) ListUsers(c *gin.Context) {
	total, err := h.userRepo.Count(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count users"})
		return
	}

	if total == 0 {
		c.JSON(http.StatusOK, gin.H{
			"users": []UserResponse{},
			"total": 0,
		})
		return
	}

	users, err := h.userRepo.List(c.Request.Context(), int(total), 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = buildUserResponse(user)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": response,
		"total": total,
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

	// Validate email if provided
	if req.Email != "" && !isValidEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
		return
	}

	// Check if username exists
	existing, _ := h.userRepo.GetByUsername(c.Request.Context(), req.Username)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Hash password
	passwordHash, err := h.authService.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Set default role
	role := req.Role
	if role == "" {
		role = "user"
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

	c.JSON(http.StatusCreated, buildUserResponse(user))
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

	c.JSON(http.StatusOK, buildUserResponse(user))
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
		username := strings.TrimSpace(*req.Username)
		if username == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username cannot be empty"})
			return
		}

		// Check if new username is already taken
		existing, _ := h.userRepo.GetByUsername(c.Request.Context(), username)
		if existing != nil && existing.ID != id {
			c.JSON(http.StatusConflict, errors.NewConflictError("user", "username", username).ToResponse(""))
			return
		}

		user.Username = username
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		if email != "" && !isValidEmail(email) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email format"})
			return
		}

		user.Email = email
	}
	if req.Role != nil {
		role := strings.TrimSpace(*req.Role)
		if role == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Role cannot be empty"})
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

	c.JSON(http.StatusOK, buildUserResponse(user))
}

// DeleteUser deletes a user (admin only).
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent self-deletion
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(int64) == id {
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
	currentUserID, _ := c.Get("user_id")
	if currentUserID.(int64) == id {
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

// UpdateUserExtendedRequest represents an extended update user request.
type UpdateUserExtendedRequest struct {
	Email        string  `json:"email"`
	Role         string  `json:"role"`
	Password     string  `json:"password"`
	Enabled      *bool   `json:"enabled"`
	TrafficLimit *int64  `json:"traffic_limit"`
	ExpiresAt    *string `json:"expires_at"`
}

// UpdateUserExtended updates a user with extended fields (admin only).
func (h *AuthHandler) UpdateUserExtended(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid user ID", nil).ToResponse(""))
		return
	}

	var req UpdateUserExtendedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid request body", nil).ToResponse(""))
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

	// Update fields
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" {
		if !h.ensureEnabledAdminRemains(c, user, req.Role, user.Enabled, "demote") {
			return
		}

		user.Role = req.Role
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, errors.NewValidationError("Password must be at least 6 characters", nil).ToResponse(""))
			return
		}

		passwordHash, err := h.authService.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, errors.NewInternalError("Failed to hash password", err).ToResponse(""))
			return
		}
		user.PasswordHash = passwordHash
	}
	if req.Enabled != nil {
		if !h.ensureEnabledAdminRemains(c, user, user.Role, *req.Enabled, "disable") {
			return
		}

		user.Enabled = *req.Enabled
	}
	if req.TrafficLimit != nil {
		user.TrafficLimit = *req.TrafficLimit
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			user.ExpiresAt = nil
		} else {
			expiresAt, err := time.Parse("2006-01-02T15:04:05Z", *req.ExpiresAt)
			if err != nil {
				c.JSON(http.StatusBadRequest, errors.NewValidationError("Invalid expires_at format", nil).ToResponse(""))
				return
			}
			user.ExpiresAt = &expiresAt
		}
	}

	if err := h.userRepo.Update(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusInternalServerError, errors.NewDatabaseError("Failed to update user", err).ToResponse(""))
		return
	}

	h.logger.Info("user updated (extended)", logger.F("user_id", id))

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
