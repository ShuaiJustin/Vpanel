// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/pkg/errors"
)

// AccessControlMiddleware checks user access based on traffic limits and expiration.
type AccessControlMiddleware struct {
	userRepo repository.UserRepository
	logger   logger.Logger
}

// NewAccessControlMiddleware creates a new access control middleware.
func NewAccessControlMiddleware(userRepo repository.UserRepository, log logger.Logger) *AccessControlMiddleware {
	return &AccessControlMiddleware{
		userRepo: userRepo,
		logger:   log,
	}
}

// CheckAccess verifies that the user has not exceeded traffic limits and account has not expired.
// This middleware should be used after authentication middleware.
func (m *AccessControlMiddleware) CheckAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		// Get user from database
		user, err := m.userRepo.GetByID(c.Request.Context(), userID.(int64))
		if err != nil {
			m.logger.Error("failed to get user for access check", logger.F("user_id", userID), logger.F("error", err))
			c.Next()
			return
		}

		// Administrative operators must be able to access management routes
		// regardless of subscription lifecycle state.
		if user.Role == "admin" {
			c.Next()
			return
		}

		// Check if user account has expired
		if user.IsExpired() {
			m.logger.Warn("access denied: account expired", logger.F("user_id", userID), logger.F("username", user.Username))
			c.AbortWithStatusJSON(http.StatusForbidden, errors.NewForbiddenError("Account has expired").ToResponse(""))
			return
		}

		// Check if user has exceeded traffic limit
		if user.IsTrafficExceeded() {
			m.logger.Warn("access denied: traffic limit exceeded",
				logger.F("user_id", userID),
				logger.F("username", user.Username),
				logger.F("traffic_used", user.TrafficUsed),
				logger.F("traffic_limit", user.TrafficLimit),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, errors.NewForbiddenError("Traffic limit exceeded").ToResponse(""))
			return
		}

		// Check if user is enabled
		if !user.Enabled {
			m.logger.Warn("access denied: account disabled", logger.F("user_id", userID), logger.F("username", user.Username))
			c.AbortWithStatusJSON(http.StatusForbidden, errors.NewForbiddenError("Account is disabled").ToResponse(""))
			return
		}

		c.Next()
	}
}

// CheckProxyAccess is a stricter check specifically for proxy-related operations.
// It uses the CanAccess() method which combines all access checks.
func (m *AccessControlMiddleware) CheckProxyAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		// Get user from database
		user, err := m.userRepo.GetByID(c.Request.Context(), userID.(int64))
		if err != nil {
			m.logger.Error("failed to get user for proxy access check", logger.F("user_id", userID), logger.F("error", err))
			c.Next()
			return
		}

		// Proxy management is also used by admins to recover and repair nodes.
		// They should not be blocked by end-user subscription checks.
		if user.Role == "admin" {
			c.Next()
			return
		}

		// Use comprehensive access check
		if !user.CanAccess() {
			var reason string
			if !user.Enabled {
				reason = "Account is disabled"
			} else if user.IsExpired() {
				reason = "Account has expired"
			} else if user.IsTrafficExceeded() {
				reason = "Traffic limit exceeded"
			} else {
				reason = "Access denied"
			}

			m.logger.Warn("proxy access denied",
				logger.F("user_id", userID),
				logger.F("username", user.Username),
				logger.F("reason", reason),
			)
			c.AbortWithStatusJSON(http.StatusForbidden, errors.NewForbiddenError(reason).ToResponse(""))
			return
		}

		c.Next()
	}
}
