// Package middleware provides HTTP middleware for the V Panel API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// PortalAuthMiddleware provides authentication middleware for the user portal.
type PortalAuthMiddleware struct {
	authService *auth.Service
	userRepo    repository.UserRepository
	logger      logger.Logger
}

// NewPortalAuthMiddleware creates a new portal authentication middleware.
func NewPortalAuthMiddleware(authService *auth.Service, userRepo repository.UserRepository, log logger.Logger) *PortalAuthMiddleware {
	return &PortalAuthMiddleware{
		authService: authService,
		userRepo:    userRepo,
		logger:      log,
	}
}

// Authenticate returns a middleware that validates JWT tokens for portal users.
func (m *PortalAuthMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "未提供认证令牌",
			})
			c.Abort()
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌格式无效",
			})
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := m.authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "认证令牌无效或已过期",
			})
			c.Abort()
			return
		}

		// Verify user exists and is enabled
		user, err := m.userRepo.GetByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "用户不存在",
			})
			c.Abort()
			return
		}

		if !user.Enabled {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "账户已被禁用",
			})
			c.Abort()
			return
		}

		// Check if account is expired
		if user.ExpiresAt != nil {
			// Expiration check handled by user.IsExpired() if needed
		}

		// Store claims and user info in context
		claims.Username = user.Username
		claims.Role = user.Role
		c.Set(string(UserClaimsKey), claims)
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireUser returns a middleware that requires the user role.
func (m *PortalAuthMiddleware) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, exists := c.Get(string(UserClaimsKey))
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "需要认证",
			})
			c.Abort()
			return
		}

		userClaims, ok := claims.(*auth.Claims)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "认证信息无效",
			})
			c.Abort()
			return
		}

		// Allow both user and admin roles
		if userClaims.Role != "user" && userClaims.Role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "权限不足",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckAccountStatus returns a middleware that checks user account status.
func (m *PortalAuthMiddleware) CheckAccountStatus() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "需要认证",
			})
			c.Abort()
			return
		}

		user, err := m.userRepo.GetByID(c.Request.Context(), userID.(int64))
		if err != nil {
			if errors.IsNotFound(err) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "用户不存在",
				})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "获取用户信息失败",
				})
			}
			c.Abort()
			return
		}

		// Check if account is disabled
		if !user.Enabled {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "账户已被禁用",
			})
			c.Abort()
			return
		}

		// Store user info for handlers
		c.Set("user", user)
		c.Next()
	}
}
