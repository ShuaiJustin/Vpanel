// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/pause"
	"v/internal/logger"
)

// PauseHandler handles subscription pause-related HTTP requests.
type PauseHandler struct {
	pauseService *pause.Service
	logger       logger.Logger
}

// NewPauseHandler creates a new pause handler.
func NewPauseHandler(pauseService *pause.Service, logger logger.Logger) *PauseHandler {
	return &PauseHandler{
		pauseService: pauseService,
		logger:       logger,
	}
}

// GetPauseStatus returns the current pause status for the authenticated user.
// GET /api/subscription/pause
func (h *PauseHandler) GetPauseStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	status, err := h.pauseService.GetPauseStatus(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get pause status", logger.Err(err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pause status"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// PauseSubscription pauses the authenticated user's subscription.
// POST /api/subscription/pause
func (h *PauseHandler) PauseSubscription(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	result, err := h.pauseService.Pause(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to pause subscription", logger.Err(err), logger.F("user_id", userID))

		// Check for validation errors
		if err.Error() == "cannot_pause" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pause subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Subscription paused successfully",
		"pause":          result.Pause,
		"auto_resume_at": result.AutoResumeAt,
		"max_duration":   result.MaxDuration,
	})
}

// ResumeSubscription resumes the authenticated user's paused subscription.
// POST /api/subscription/resume
func (h *PauseHandler) ResumeSubscription(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.pauseService.Resume(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to resume subscription", logger.Err(err), logger.F("user_id", userID))

		// Check for validation errors
		if err.Error() == "not_paused" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription is not paused"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resume subscription"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Subscription resumed successfully",
	})
}

// GetPauseHistory returns the pause history for the authenticated user.
// GET /api/subscription/pause/history
func (h *PauseHandler) GetPauseHistory(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	pauses, total, err := h.pauseService.GetPauseHistory(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get pause history", logger.Err(err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pause history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pauses":    pauses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AdminGetPauseStats returns pause statistics (admin only).
// GET /api/admin/subscription/pause/stats
func (h *PauseHandler) AdminGetPauseStats(c *gin.Context) {
	stats, err := h.pauseService.GetPauseStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get pause stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pause statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AdminTriggerAutoResume triggers an immediate auto-resume check (admin only).
// POST /api/admin/subscription/pause/auto-resume
func (h *PauseHandler) AdminTriggerAutoResume(c *gin.Context) {
	resumed, err := h.pauseService.AutoResumePaused(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to trigger auto-resume", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger auto-resume"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Auto-resume triggered",
		"resumed": resumed,
	})
}
