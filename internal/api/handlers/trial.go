// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/trial"
	"v/internal/logger"
)

// TrialHandler handles trial-related requests.
type TrialHandler struct {
	trialService *trial.Service
	logger       logger.Logger
}

// NewTrialHandler creates a new TrialHandler.
func NewTrialHandler(trialService *trial.Service, log logger.Logger) *TrialHandler {
	return &TrialHandler{
		trialService: trialService,
		logger:       log,
	}
}

// TrialResponse represents a trial in API responses.
type TrialResponse struct {
	ID               int64   `json:"id"`
	UserID           int64   `json:"user_id"`
	Status           string  `json:"status"`
	StartAt          string  `json:"start_at"`
	ExpireAt         string  `json:"expire_at"`
	TrafficUsed      int64   `json:"traffic_used"`
	TrafficLimit     int64   `json:"traffic_limit"`
	RemainingDays    int     `json:"remaining_days"`
	RemainingTraffic int64   `json:"remaining_traffic"`
	ConvertedAt      *string `json:"converted_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

// TrialStatusResponse represents the trial status for a user.
type TrialStatusResponse struct {
	HasTrial    bool           `json:"has_trial"`
	CanActivate bool           `json:"can_activate"`
	Message     string         `json:"message,omitempty"`
	Trial       *TrialResponse `json:"trial,omitempty"`
	TrialConfig *TrialConfig   `json:"trial_config"`
}

// TrialConfig represents trial configuration in API responses.
type TrialConfig struct {
	Enabled            bool  `json:"enabled"`
	Duration           int   `json:"duration"`
	TrafficLimit       int64 `json:"traffic_limit"`
	RequireEmailVerify bool  `json:"require_email_verify"`
}

// TrialStatsResponse represents trial statistics in API responses.
type TrialStatsResponse struct {
	TotalTrials     int64   `json:"total_trials"`
	ActiveTrials    int64   `json:"active_trials"`
	ExpiredTrials   int64   `json:"expired_trials"`
	ConvertedTrials int64   `json:"converted_trials"`
	ConversionRate  float64 `json:"conversion_rate"`
}

// GetTrialStatus returns the trial status for the current user.
func (h *TrialHandler) GetTrialStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := userID.(int64)
	config := h.trialService.GetConfig()

	response := TrialStatusResponse{
		TrialConfig: &TrialConfig{
			Enabled:            config.Enabled,
			Duration:           config.Duration,
			TrafficLimit:       config.TrafficLimit,
			RequireEmailVerify: config.RequireEmailVerify,
		},
	}

	// Check if user has a trial
	t, err := h.trialService.GetTrial(c.Request.Context(), uid)
	if err == nil {
		response.HasTrial = true
		response.Trial = h.toTrialResponse(t)
		response.CanActivate = false
	} else {
		response.HasTrial = false
		canActivate, message := h.trialService.CanActivateTrial(c.Request.Context(), uid)
		response.CanActivate = canActivate
		response.Message = message
	}

	c.JSON(http.StatusOK, response)
}

// ActivateTrial activates a trial for the current user.
func (h *TrialHandler) ActivateTrial(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := userID.(int64)

	t, err := h.trialService.ActivateTrial(c.Request.Context(), uid)
	if err != nil {
		switch err {
		case trial.ErrTrialDisabled:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Trial feature is disabled"})
		case trial.ErrTrialAlreadyUsed:
			c.JSON(http.StatusBadRequest, gin.H{"error": "You have already used your trial"})
		case trial.ErrEmailNotVerified:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email verification required"})
		default:
			h.logger.Error("Failed to activate trial", logger.Err(err), logger.F("user_id", uid))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to activate trial"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trial activated successfully",
		"trial":   h.toTrialResponse(t),
	})
}

// GetTrial returns the trial for the current user.
func (h *TrialHandler) GetTrial(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	uid := userID.(int64)

	t, err := h.trialService.GetTrial(c.Request.Context(), uid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No trial found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trial": h.toTrialResponse(t)})
}

// AdminListTrials lists all trials (admin only).
func (h *TrialHandler) AdminListTrials(c *gin.Context) {
	// This would need pagination and filtering in a real implementation
	// For now, we'll return stats
	stats, err := h.trialService.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get trial stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trial statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": TrialStatsResponse{
			TotalTrials:     stats.TotalTrials,
			ActiveTrials:    stats.ActiveTrials,
			ExpiredTrials:   stats.ExpiredTrials,
			ConvertedTrials: stats.ConvertedTrials,
			ConversionRate:  stats.ConversionRate,
		},
	})
}

// AdminGrantTrial grants a trial to a specific user (admin only).
func (h *TrialHandler) AdminGrantTrial(c *gin.Context) {
	var req struct {
		UserID   int64 `json:"user_id" binding:"required"`
		Duration int   `json:"duration"` // days, optional
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	duration := req.Duration
	if duration <= 0 {
		duration = h.trialService.GetConfig().Duration
	}

	t, err := h.trialService.GrantTrial(c.Request.Context(), req.UserID, duration)
	if err != nil {
		h.logger.Error("Failed to grant trial", logger.Err(err), logger.F("user_id", req.UserID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to grant trial"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Trial granted successfully",
		"trial":   h.toTrialResponse(t),
	})
}

// AdminGetTrialStats returns trial statistics (admin only).
func (h *TrialHandler) AdminGetTrialStats(c *gin.Context) {
	stats, err := h.trialService.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get trial stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get trial statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"stats": TrialStatsResponse{
			TotalTrials:     stats.TotalTrials,
			ActiveTrials:    stats.ActiveTrials,
			ExpiredTrials:   stats.ExpiredTrials,
			ConvertedTrials: stats.ConvertedTrials,
			ConversionRate:  stats.ConversionRate,
		},
	})
}

// AdminGetTrialByUser returns the trial for a specific user (admin only).
func (h *TrialHandler) AdminGetTrialByUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	t, err := h.trialService.GetTrial(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No trial found for this user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"trial": h.toTrialResponse(t)})
}

// AdminExpireTrials manually triggers trial expiration (admin only).
func (h *TrialHandler) AdminExpireTrials(c *gin.Context) {
	count, err := h.trialService.ExpireTrials(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to expire trials", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to expire trials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Trials expired successfully",
		"expired_count": count,
	})
}

// toTrialResponse converts a trial to an API response.
func (h *TrialHandler) toTrialResponse(t *trial.Trial) *TrialResponse {
	response := &TrialResponse{
		ID:               t.ID,
		UserID:           t.UserID,
		Status:           t.Status,
		StartAt:          t.StartAt.Format("2006-01-02T15:04:05Z"),
		ExpireAt:         t.ExpireAt.Format("2006-01-02T15:04:05Z"),
		TrafficUsed:      t.TrafficUsed,
		TrafficLimit:     t.TrafficLimit,
		RemainingDays:    t.RemainingDays,
		RemainingTraffic: t.RemainingTraffic,
		CreatedAt:        t.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	if t.ConvertedAt != nil {
		convertedAt := t.ConvertedAt.Format("2006-01-02T15:04:05Z")
		response.ConvertedAt = &convertedAt
	}

	return response
}
