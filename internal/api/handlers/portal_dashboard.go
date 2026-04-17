// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/portal/announcement"
	"v/internal/portal/stats"
	pkgerrors "v/pkg/errors"
)

type portalDashboardEntitlement interface {
	EvaluateAccess(ctx context.Context, userID int64) (*entitlement.AccessState, error)
}

// PortalDashboardHandler handles portal dashboard requests.
type PortalDashboardHandler struct {
	userRepo            repository.UserRepository
	statsService        *stats.Service
	announcementService *announcement.Service
	entitlement         portalDashboardEntitlement
	logger              logger.Logger
}

// NewPortalDashboardHandler creates a new PortalDashboardHandler.
func NewPortalDashboardHandler(
	userRepo repository.UserRepository,
	statsService *stats.Service,
	announcementService *announcement.Service,
	log logger.Logger,
) *PortalDashboardHandler {
	return &PortalDashboardHandler{
		userRepo:            userRepo,
		statsService:        statsService,
		announcementService: announcementService,
		logger:              log,
	}
}

// WithEntitlementService configures entitlement-aware dashboard traffic data.
func (h *PortalDashboardHandler) WithEntitlementService(entitlementService portalDashboardEntitlement) *PortalDashboardHandler {
	h.entitlement = entitlementService
	return h
}

// GetDashboard returns dashboard data for the current user.
func (h *PortalDashboardHandler) GetDashboard(c *gin.Context) {
	userID, ok := currentUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Define result types for parallel execution
	type userResult struct {
		user *repository.User
		err  error
	}
	type trafficResult struct {
		summary *stats.TrafficSummary
		err     error
	}
	type entitlementResult struct {
		accessState *entitlement.AccessState
		err         error
	}
	type announcementResult struct {
		count int64
		err   error
	}

	// Create channels for parallel execution
	userCh := make(chan userResult, 1)
	trafficCh := make(chan trafficResult, 1)
	entitlementCh := make(chan entitlementResult, 1)
	announcementCh := make(chan announcementResult, 1)

	ctx := c.Request.Context()

	// Execute queries in parallel using goroutines
	// Get user info
	go func() {
		user, err := h.userRepo.GetByID(ctx, userID)
		userCh <- userResult{user: user, err: err}
	}()

	// Get traffic summary
	go func() {
		var summary *stats.TrafficSummary
		var err error
		if h.statsService != nil {
			summary, err = h.statsService.GetTrafficSummary(ctx, userID)
		}
		trafficCh <- trafficResult{summary: summary, err: err}
	}()

	// Get entitlement access state
	go func() {
		var accessState *entitlement.AccessState
		var err error
		if h.entitlement != nil {
			accessState, err = h.entitlement.EvaluateAccess(ctx, userID)
		}
		entitlementCh <- entitlementResult{accessState: accessState, err: err}
	}()

	// Get unread announcement count
	go func() {
		var count int64
		var err error
		if h.announcementService != nil {
			count, err = h.announcementService.GetUnreadCount(ctx, userID)
		}
		announcementCh <- announcementResult{count: count, err: err}
	}()

	// Aggregate results from all goroutines
	userRes := <-userCh
	trafficRes := <-trafficCh
	entitlementRes := <-entitlementCh
	announcementRes := <-announcementCh

	// Handle user result (critical error)
	if userRes.err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}
	user := userRes.user

	// Handle traffic summary result (non-critical)
	trafficSummary := trafficRes.summary
	if trafficRes.err != nil {
		h.logger.Warn("failed to get portal dashboard traffic summary", logger.F("error", trafficRes.err), logger.F("user_id", userID))
	}

	// Handle entitlement result and calculate effective traffic
	effectiveTrafficLimit := user.TrafficLimit
	effectiveTrafficUsed := user.TrafficUsed
	if entitlementRes.accessState != nil {
		effectiveTrafficLimit = entitlementRes.accessState.EffectiveTrafficLimit
		effectiveTrafficUsed = entitlementRes.accessState.EffectiveTrafficUsed
	}
	if entitlementRes.err != nil && !pkgerrors.IsForbidden(entitlementRes.err) {
		h.logger.Warn("failed to evaluate portal dashboard entitlement",
			logger.F("user_id", userID),
			logger.F("error", entitlementRes.err),
		)
	}

	// Handle announcement result (non-critical)
	unreadCount := announcementRes.count
	if announcementRes.err != nil {
		h.logger.Warn("failed to get portal dashboard unread announcement count", logger.F("error", announcementRes.err), logger.F("user_id", userID))
	}

	// Calculate traffic percentage
	var trafficPercentage float64
	if effectiveTrafficLimit > 0 {
		trafficPercentage = float64(effectiveTrafficUsed) / float64(effectiveTrafficLimit) * 100
	}

	trafficLimitDisplay := stats.FormatBytes(effectiveTrafficLimit)
	if effectiveTrafficLimit <= 0 {
		trafficLimitDisplay = "不限流量"
	}

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                 user.ID,
			"username":           user.Username,
			"email":              user.Email,
			"enabled":            user.Enabled,
			"expires_at":         user.ExpiresAt,
			"two_factor_enabled": user.TwoFactorEnabled,
		},
		"traffic": gin.H{
			"used":       effectiveTrafficUsed,
			"limit":      effectiveTrafficLimit,
			"percentage": trafficPercentage,
			"used_str":   stats.FormatBytes(effectiveTrafficUsed),
			"limit_str":  trafficLimitDisplay,
		},
		"summary":              trafficSummary,
		"unread_announcements": unreadCount,
	})
}

// GetTrafficSummary returns traffic summary for the current user.
func (h *PortalDashboardHandler) GetTrafficSummary(c *gin.Context) {
	userID, ok := currentUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	if h.statsService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "统计服务不可用"})
		return
	}

	summary, err := h.statsService.GetTrafficSummary(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("failed to get traffic summary", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取流量统计失败"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetRecentAnnouncements returns recent announcements.
func (h *PortalDashboardHandler) GetRecentAnnouncements(c *gin.Context) {
	userID, ok := currentUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	if h.announcementService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "公告服务不可用"})
		return
	}

	announcements, _, err := h.announcementService.ListAnnouncements(c.Request.Context(), userID, 5, 0)
	if err != nil {
		h.logger.Error("failed to get announcements", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取公告失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"announcements": announcements,
	})
}
