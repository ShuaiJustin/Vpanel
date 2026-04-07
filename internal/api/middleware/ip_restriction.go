// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"v/internal/ip"
	"v/internal/logger"
)

// GetUserID extracts user ID from gin context.
// It looks for common context keys used for storing user ID.
func GetUserID(c *gin.Context) int64 {
	// Try different common keys
	if userID, exists := c.Get("user_id"); exists {
		switch v := userID.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case uint:
			return int64(v)
		case uint64:
			return int64(v)
		}
	}

	if userID, exists := c.Get("userID"); exists {
		switch v := userID.(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case uint:
			return int64(v)
		case uint64:
			return int64(v)
		}
	}

	return 0
}

// IPRestrictionMiddleware provides IP restriction checking middleware.
type IPRestrictionMiddleware struct {
	ipService *ip.Service
	logger    logger.Logger
}

// NewIPRestrictionMiddleware creates a new IPRestrictionMiddleware.
func NewIPRestrictionMiddleware(ipService *ip.Service, log logger.Logger) *IPRestrictionMiddleware {
	return &IPRestrictionMiddleware{
		ipService: ipService,
		logger:    log,
	}
}

// CheckIPRestriction returns a middleware that checks IP restrictions.
func (m *IPRestrictionMiddleware) CheckIPRestriction(getUserMaxIPs func(userID int64) int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get user ID from context
		userID := GetUserID(c)
		if userID == 0 {
			// No user ID, skip IP restriction check
			c.Next()
			return
		}

		// Get client IP
		clientIP := c.ClientIP()

		// Get user's max concurrent IPs
		maxIPs := -1 // Use default
		if getUserMaxIPs != nil {
			maxIPs = getUserMaxIPs(userID)
		}

		// Check access
		result, err := m.ipService.CheckAccess(ctx, uint(userID), clientIP, ip.AccessTypeAPI, maxIPs)
		if err != nil {
			m.logger.Error("IP restriction check failed",
				logger.F("error", err),
				logger.F("user_id", userID),
				logger.F("ip", clientIP))
			// Fail closed: deny access on error for security
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "SERVICE_UNAVAILABLE",
				"message": "IP restriction service temporarily unavailable",
			})
			return
		}

		if !result.Allowed {
			m.logger.Warn("IP access denied",
				logger.F("user_id", userID),
				logger.F("ip", clientIP),
				logger.F("reason", result.Reason),
				logger.F("code", result.Code))

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    result.Code,
				"message": result.Reason,
				"details": gin.H{
					"remaining_slots": result.RemainingSlots,
					"online_ips":      result.OnlineIPs,
				},
			})
			return
		}

		c.Next()

		// Persisting IP activity may perform geolocation enrichment; keep it off the response path.
		userAgent := c.GetHeader("User-Agent")
		go func(userID int64, clientIP, userAgent string) {
			recordCtx, cancel := newBackgroundTaskContext(ctx)
			defer cancel()
			if err := m.ipService.RecordActivity(recordCtx, uint(userID), clientIP, userAgent, ip.AccessTypeAPI); err != nil {
				m.logger.Error("Failed to record IP activity",
					logger.F("error", err),
					logger.F("user_id", userID),
					logger.F("ip", clientIP))
			}
		}(userID, clientIP, userAgent)
	}
}

// CheckSubscriptionIPRestriction returns a middleware for subscription access.
func (m *IPRestrictionMiddleware) CheckSubscriptionIPRestriction(getSubscriptionLimit func(subscriptionID uint) int) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Get subscription ID from context or path
		subscriptionID, exists := c.Get("subscription_id")
		if !exists {
			c.Next()
			return
		}

		subID, ok := subscriptionID.(uint)
		if !ok {
			c.Next()
			return
		}

		// Get client IP
		clientIP := c.ClientIP()

		// Get subscription IP limit
		limit := 0 // No limit by default
		if getSubscriptionLimit != nil {
			limit = getSubscriptionLimit(subID)
		}

		if limit <= 0 {
			c.Next()
			return
		}

		// Create subscription IP service
		subIPService := ip.NewSubscriptionIPService(m.ipService.Tracker().GetDB(), m.ipService.GeoService())

		// Check IP limit
		result, err := subIPService.CheckIPLimit(ctx, subID, clientIP, limit)
		if err != nil {
			m.logger.Error("Subscription IP check failed",
				logger.F("error", err),
				logger.F("subscription_id", subID),
				logger.F("ip", clientIP))
			// Fail closed: deny access on error for security
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"code":    "SERVICE_UNAVAILABLE",
				"message": "IP restriction service temporarily unavailable",
			})
			return
		}

		if !result.Allowed {
			m.logger.Warn("Subscription IP access denied",
				logger.F("subscription_id", subID),
				logger.F("ip", clientIP),
				logger.F("reason", result.Reason))

			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    result.Code,
				"message": "Subscription IP limit reached. Please contact support.",
			})
			return
		}

		c.Next()

		// Recording subscription IP access is best-effort and should not block delivery.
		userAgent := c.GetHeader("User-Agent")
		go func(subscriptionID uint, clientIP, userAgent string) {
			recordCtx, cancel := newBackgroundTaskContext(ctx)
			defer cancel()
			if err := subIPService.RecordAccess(recordCtx, subscriptionID, clientIP, userAgent); err != nil {
				m.logger.Error("Failed to record subscription IP access",
					logger.F("error", err),
					logger.F("subscription_id", subscriptionID),
					logger.F("ip", clientIP))
			}
		}(subID, clientIP, userAgent)
	}
}

// RecordFailedAttempt records a failed access attempt.
func (m *IPRestrictionMiddleware) RecordFailedAttempt(reason string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Check if the request failed with 401 or 403
		if c.Writer.Status() == http.StatusUnauthorized || c.Writer.Status() == http.StatusForbidden {
			ctx := c.Request.Context()
			clientIP := c.ClientIP()

			recordCtx, cancel := newBackgroundTaskContext(ctx)
			defer cancel()
			if err := m.ipService.RecordFailedAttempt(recordCtx, clientIP, reason); err != nil {
				m.logger.Error("Failed to record failed attempt",
					logger.F("error", err),
					logger.F("ip", clientIP))
			}

			// Check if should auto-blacklist
			blacklisted, err := m.ipService.CheckAutoBlacklist(ctx, clientIP)
			if err != nil {
				m.logger.Error("Failed to check auto-blacklist",
					logger.F("error", err),
					logger.F("ip", clientIP))
			} else if blacklisted {
				m.logger.Warn("IP auto-blacklisted",
					logger.F("ip", clientIP),
					logger.F("reason", reason))
			}
		}
	}
}
