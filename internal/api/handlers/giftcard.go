// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/giftcard"
	"v/internal/logger"
)

// GiftCardHandler handles gift card related requests.
type GiftCardHandler struct {
	giftCardService *giftcard.Service
	logger          logger.Logger
}

// NewGiftCardHandler creates a new gift card handler.
func NewGiftCardHandler(giftCardService *giftcard.Service, log logger.Logger) *GiftCardHandler {
	return &GiftCardHandler{
		giftCardService: giftCardService,
		logger:          log,
	}
}

// RedeemRequest represents a gift card redemption request.
type RedeemRequest struct {
	Code string `json:"code" binding:"required"`
}

// RedeemGiftCard handles POST /api/gift-cards/redeem
func (h *GiftCardHandler) RedeemGiftCard(c *gin.Context) {
	var req RedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: code is required"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	gc, err := h.giftCardService.Redeem(c.Request.Context(), req.Code, userID.(int64))
	if err != nil {
		switch err {
		case giftcard.ErrGiftCardNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "Gift card not found"})
		case giftcard.ErrGiftCardAlreadyUsed:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Gift card has already been redeemed"})
		case giftcard.ErrGiftCardExpired:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Gift card has expired"})
		case giftcard.ErrGiftCardDisabled:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Gift card is disabled"})
		case giftcard.ErrSelfRedeem:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot redeem your own gift card"})
		default:
			h.logger.Error("Failed to redeem gift card", logger.Err(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to redeem gift card"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Gift card redeemed successfully",
		"gift_card": gc,
		"credited":  gc.Value,
	})
}

// ListUserGiftCards handles GET /api/gift-cards
func (h *GiftCardHandler) ListUserGiftCards(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	giftCards, total, err := h.giftCardService.ListByUser(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list user gift cards", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list gift cards"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"gift_cards": giftCards,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

// ValidateGiftCard handles POST /api/gift-cards/validate
func (h *GiftCardHandler) ValidateGiftCard(c *gin.Context) {
	var req RedeemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: code is required"})
		return
	}

	gc, err := h.giftCardService.GetByCode(c.Request.Context(), req.Code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gift card not found", "valid": false})
		return
	}

	// Check if valid
	valid := gc.Status == giftcard.StatusActive
	if gc.ExpiresAt != nil && time.Now().After(*gc.ExpiresAt) {
		valid = false
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":  valid,
		"value":  gc.Value,
		"status": gc.Status,
	})
}

// ==================== Admin Handlers ====================

// CreateBatchRequest represents a request to create a batch of gift cards.
type CreateBatchRequest struct {
	Count     int    `json:"count" binding:"required,min=1,max=1000"`
	Value     int64  `json:"value" binding:"required,min=1"`
	ExpiresAt string `json:"expires_at"` // Optional, ISO 8601 format
	Prefix    string `json:"prefix"`     // Optional prefix for codes
}

// AdminCreateBatch handles POST /api/admin/gift-cards/batch
func (h *GiftCardHandler) AdminCreateBatch(c *gin.Context) {
	var req CreateBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Parse expiration date if provided
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expires_at format, use ISO 8601"})
			return
		}
		expiresAt = &t
	}

	createReq := &giftcard.CreateBatchRequest{
		Count:     req.Count,
		Value:     req.Value,
		ExpiresAt: expiresAt,
		Prefix:    req.Prefix,
	}

	giftCards, batchID, err := h.giftCardService.CreateBatch(c.Request.Context(), createReq, userID.(int64))
	if err != nil {
		h.logger.Error("Failed to create gift card batch", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gift cards"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":    "Gift cards created successfully",
		"batch_id":   batchID,
		"count":      len(giftCards),
		"gift_cards": giftCards,
	})
}

// AdminListGiftCards handles GET /api/admin/gift-cards
func (h *GiftCardHandler) AdminListGiftCards(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	batchID := c.Query("batch_id")

	// 验证分页参数
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	filter := giftcard.GiftCardFilter{
		Status:  status,
		BatchID: batchID,
	}

	giftCards, total, err := h.giftCardService.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list gift cards", logger.Err(err),
			logger.F("page", page),
			logger.F("pageSize", pageSize),
			logger.F("status", status),
			logger.F("batchID", batchID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve gift card list",
			"error":   "Database query failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"gift_cards": giftCards,
			"total":      total,
			"page":       page,
			"page_size":  pageSize,
		},
	})
}

// AdminGetGiftCard handles GET /api/admin/gift-cards/:id
func (h *GiftCardHandler) AdminGetGiftCard(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gift card ID"})
		return
	}

	gc, err := h.giftCardService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Gift card not found"})
		return
	}

	c.JSON(http.StatusOK, gc)
}

// AdminSetStatus handles PUT /api/admin/gift-cards/:id/status
func (h *GiftCardHandler) AdminSetStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gift card ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.giftCardService.SetStatus(c.Request.Context(), id, req.Status); err != nil {
		h.logger.Error("Failed to set gift card status", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

// AdminDeleteGiftCard handles DELETE /api/admin/gift-cards/:id
func (h *GiftCardHandler) AdminDeleteGiftCard(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid gift card ID"})
		return
	}

	if err := h.giftCardService.Delete(c.Request.Context(), id); err != nil {
		if err == giftcard.ErrGiftCardNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Gift card not found"})
			return
		}
		h.logger.Error("Failed to delete gift card", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete gift card"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Gift card deleted successfully"})
}

// AdminGetStats handles GET /api/admin/gift-cards/stats
func (h *GiftCardHandler) AdminGetStats(c *gin.Context) {
	stats, err := h.giftCardService.GetStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get gift card stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AdminGetBatchStats handles GET /api/admin/gift-cards/batch/:batch_id/stats
func (h *GiftCardHandler) AdminGetBatchStats(c *gin.Context) {
	batchID := c.Param("batch_id")
	if batchID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Batch ID is required"})
		return
	}

	stats, err := h.giftCardService.GetBatchStats(c.Request.Context(), batchID)
	if err != nil {
		h.logger.Error("Failed to get batch stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get batch statistics"})
		return
	}

	c.JSON(http.StatusOK, stats)
}
