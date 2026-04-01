// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/portal/announcement"
	"v/internal/portal/stats"
)

// PortalDashboardHandler handles portal dashboard requests.
type PortalDashboardHandler struct {
	userRepo            repository.UserRepository
	statsService        *stats.Service
	announcementService *announcement.Service
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

// GetDashboard returns dashboard data for the current user.
func (h *PortalDashboardHandler) GetDashboard(c *gin.Context) {
	userID, ok := currentUserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Get user info
	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "用户不存在"})
		return
	}

	// Get traffic summary
	var trafficSummary *stats.TrafficSummary
	if h.statsService != nil {
		trafficSummary, err = h.statsService.GetTrafficSummary(c.Request.Context(), userID)
		if err != nil {
			h.logger.Warn("failed to get portal dashboard traffic summary", logger.F("error", err), logger.F("user_id", userID))
		}
	}

	// Get unread announcement count
	var unreadCount int64
	if h.announcementService != nil {
		unreadCount, err = h.announcementService.GetUnreadCount(c.Request.Context(), userID)
		if err != nil {
			h.logger.Warn("failed to get portal dashboard unread announcement count", logger.F("error", err), logger.F("user_id", userID))
		}
	}

	// Calculate traffic percentage
	var trafficPercentage float64
	if user.TrafficLimit > 0 {
		trafficPercentage = float64(user.TrafficUsed) / float64(user.TrafficLimit) * 100
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
			"used":       user.TrafficUsed,
			"limit":      user.TrafficLimit,
			"percentage": trafficPercentage,
			"used_str":   stats.FormatBytes(user.TrafficUsed),
			"limit_str":  stats.FormatBytes(user.TrafficLimit),
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
