// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/commission"
	"v/internal/commercial/invite"
	"v/internal/logger"
)

// InviteHandler handles invite-related requests.
type InviteHandler struct {
	inviteService     *invite.Service
	commissionService *commission.Service
	logger            logger.Logger
}

// NewInviteHandler creates a new InviteHandler.
func NewInviteHandler(inviteService *invite.Service, commissionService *commission.Service, log logger.Logger) *InviteHandler {
	return &InviteHandler{
		inviteService:     inviteService,
		commissionService: commissionService,
		logger:            log,
	}
}

// InviteCodeResponse represents an invite code in API responses.
type InviteCodeResponse struct {
	Code        string `json:"code"`
	InviteCount int    `json:"invite_count"`
	InviteLink  string `json:"invite_link"`
}

// ReferralResponse represents a referral in API responses.
type ReferralResponse struct {
	InviteeID   int64   `json:"invitee_id"`
	Status      string  `json:"status"`
	ConvertedAt *string `json:"converted_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

// CommissionResponse represents a commission in API responses.
type CommissionResponse struct {
	ID         int64   `json:"id"`
	FromUserID int64   `json:"from_user_id"`
	OrderID    int64   `json:"order_id"`
	Amount     int64   `json:"amount"`
	Rate       float64 `json:"rate"`
	Status     string  `json:"status"`
	ConfirmAt  *string `json:"confirm_at,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// GetInviteCode returns the current user's invite code.
func (h *InviteHandler) GetInviteCode(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	code, err := h.inviteService.GetOrCreateCode(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get invite code", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invite code"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"invite": InviteCodeResponse{
			Code:        code.Code,
			InviteCount: code.InviteCount,
			InviteLink:  h.inviteService.GenerateInviteLink(code.Code),
		},
	})
}

// GetReferrals returns the current user's referrals.
func (h *InviteHandler) GetReferrals(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	referrals, total, err := h.inviteService.GetReferrals(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get referrals", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get referrals"})
		return
	}

	responses := make([]ReferralResponse, len(referrals))
	for i, r := range referrals {
		responses[i] = ReferralResponse{
			InviteeID:   r.InviteeID,
			Status:      r.Status,
			ConvertedAt: r.ConvertedAt,
			CreatedAt:   r.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"referrals": responses,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetInviteStats returns the current user's invite statistics.
func (h *InviteHandler) GetInviteStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	stats, err := h.inviteService.GetStats(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get invite stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get invite stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// GetCommissions returns the current user's commissions.
func (h *InviteHandler) GetCommissions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	commissions, total, err := h.commissionService.ListAll(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get commissions", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get commissions"})
		return
	}

	responses := make([]CommissionResponse, len(commissions))
	for i, comm := range commissions {
		responses[i] = CommissionResponse{
			ID:         comm.ID,
			FromUserID: comm.FromUserID,
			OrderID:    comm.OrderID,
			Amount:     comm.Amount,
			Rate:       comm.Rate,
			Status:     comm.Status,
			ConfirmAt:  comm.ConfirmAt,
			CreatedAt:  comm.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"commissions": responses,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
	})
}

// GetCommissionSummary returns the current user's commission summary.
func (h *InviteHandler) GetCommissionSummary(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	total, err := h.commissionService.GetTotalEarnings(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get total earnings", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get commission summary"})
		return
	}

	pending, err := h.commissionService.GetPendingEarnings(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get pending earnings", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get commission summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_earnings":   total,
		"pending_earnings": pending,
	})
}
