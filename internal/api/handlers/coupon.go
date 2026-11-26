// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/coupon"
	"v/internal/logger"
)

// CouponHandler handles coupon-related requests.
type CouponHandler struct {
	couponService *coupon.Service
	logger        logger.Logger
}

// NewCouponHandler creates a new CouponHandler.
func NewCouponHandler(couponService *coupon.Service, log logger.Logger) *CouponHandler {
	return &CouponHandler{
		couponService: couponService,
		logger:        log,
	}
}

// CouponResponse represents a coupon in API responses.
type CouponResponse struct {
	ID             int64   `json:"id"`
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Value          int64   `json:"value"`
	MinOrderAmount int64   `json:"min_order_amount"`
	MaxDiscount    int64   `json:"max_discount"`
	TotalLimit     int     `json:"total_limit"`
	PerUserLimit   int     `json:"per_user_limit"`
	UsedCount      int     `json:"used_count"`
	PlanIDs        []int64 `json:"plan_ids,omitempty"`
	StartAt        string  `json:"start_at"`
	ExpireAt       string  `json:"expire_at"`
	IsActive       bool    `json:"is_active"`
}

// ValidateCouponRequest represents a request to validate a coupon.
type ValidateCouponRequest struct {
	Code   string `json:"code" binding:"required"`
	PlanID int64  `json:"plan_id" binding:"required,gt=0"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
}

func couponValidationErrorMessage(err error) string {
	switch {
	case errors.Is(err, coupon.ErrCouponNotFound):
		return "优惠券不存在，请核对券码。"
	case errors.Is(err, coupon.ErrCouponExpired):
		return "优惠券已过期。"
	case errors.Is(err, coupon.ErrCouponInactive):
		return "优惠券当前已停用。"
	case errors.Is(err, coupon.ErrCouponNotStarted):
		return "优惠券尚未到生效时间。"
	case errors.Is(err, coupon.ErrCouponLimitReached):
		return "优惠券已被领完或达到总使用上限。"
	case errors.Is(err, coupon.ErrCouponUserLimit):
		return "您已达到该优惠券的使用次数上限。"
	case errors.Is(err, coupon.ErrCouponMinAmount):
		return "订单金额未满足该优惠券的最低消费门槛。"
	case errors.Is(err, coupon.ErrCouponPlanMismatch):
		return "该优惠券不适用于当前所选套餐。"
	default:
		return "优惠券不可用，请核对券码、有效期和适用条件。"
	}
}

// ValidateCoupon validates a coupon code.
func (h *CouponHandler) ValidateCoupon(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	var req ValidateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid request body",
		})
		return
	}

	cp, discount, err := h.couponService.Validate(c.Request.Context(), req.Code, userID.(int64), req.PlanID, req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "COUPON_ERROR",
			"message": couponValidationErrorMessage(err),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"coupon": h.toCouponResponse(cp), "discount": discount})
}

// CreateCouponRequest represents a request to create a coupon.
type CreateCouponRequest struct {
	Code           string  `json:"code"`
	Name           string  `json:"name" binding:"required"`
	Type           string  `json:"type" binding:"required"`
	Value          int64   `json:"value" binding:"required"`
	MinOrderAmount int64   `json:"min_order_amount"`
	MaxDiscount    int64   `json:"max_discount"`
	TotalLimit     int     `json:"total_limit"`
	PerUserLimit   int     `json:"per_user_limit"`
	PlanIDs        []int64 `json:"plan_ids"`
	StartAt        string  `json:"start_at" binding:"required"`
	ExpireAt       string  `json:"expire_at" binding:"required"`
	IsActive       bool    `json:"is_active"`
}

func parseCouponTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed, nil
	}
	return time.ParseInLocation("2006-01-02", value, time.Local)
}

func normalizeCouponType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "percent":
		return coupon.TypePercentage
	default:
		return strings.ToLower(strings.TrimSpace(value))
	}
}

// ListCoupons returns all coupons (admin only).
func (h *CouponHandler) ListCoupons(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	coupons, total, err := h.couponService.List(c.Request.Context(), coupon.CouponFilter{}, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list coupons", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list coupons"})
		return
	}

	response := make([]CouponResponse, len(coupons))
	for i, cp := range coupons {
		response[i] = h.toCouponResponse(cp)
	}

	c.JSON(http.StatusOK, gin.H{"coupons": response, "total": total, "page": page, "page_size": pageSize})
}

// CreateCoupon creates a new coupon (admin only).
func (h *CouponHandler) CreateCoupon(c *gin.Context) {
	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	startAt, err := parseCouponTimestamp(req.StartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_at format. Use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"})
		return
	}
	expireAt, err := parseCouponTimestamp(req.ExpireAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expire_at format. Use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"})
		return
	}

	req.Type = normalizeCouponType(req.Type)

	// Validate time range
	if expireAt.Before(startAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expire_at must be after start_at"})
		return
	}

	createReq := &coupon.CreateCouponRequest{
		Code:           req.Code,
		Name:           req.Name,
		Type:           req.Type,
		Value:          req.Value,
		MinOrderAmount: req.MinOrderAmount,
		MaxDiscount:    req.MaxDiscount,
		TotalLimit:     req.TotalLimit,
		PerUserLimit:   req.PerUserLimit,
		PlanIDs:        req.PlanIDs,
		StartAt:        startAt,
		ExpireAt:       expireAt,
		IsActive:       req.IsActive,
	}

	cp, err := h.couponService.Create(c.Request.Context(), createReq)
	if err != nil {
		h.logger.Error("Failed to create coupon", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"coupon": h.toCouponResponse(cp)})
}

// UpdateCoupon updates a coupon (admin only).
func (h *CouponHandler) UpdateCoupon(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
		return
	}

	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	startAt, err := parseCouponTimestamp(req.StartAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_at format. Use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"})
		return
	}
	expireAt, err := parseCouponTimestamp(req.ExpireAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid expire_at format. Use YYYY-MM-DD or YYYY-MM-DD HH:MM:SS"})
		return
	}

	req.Type = normalizeCouponType(req.Type)

	if expireAt.Before(startAt) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expire_at must be after start_at"})
		return
	}

	updateReq := &coupon.CreateCouponRequest{
		Code:           req.Code,
		Name:           req.Name,
		Type:           req.Type,
		Value:          req.Value,
		MinOrderAmount: req.MinOrderAmount,
		MaxDiscount:    req.MaxDiscount,
		TotalLimit:     req.TotalLimit,
		PerUserLimit:   req.PerUserLimit,
		PlanIDs:        req.PlanIDs,
		StartAt:        startAt,
		ExpireAt:       expireAt,
		IsActive:       req.IsActive,
	}

	cp, err := h.couponService.Update(c.Request.Context(), id, updateReq)
	if err != nil {
		h.logger.Error("Failed to update coupon", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"coupon": h.toCouponResponse(cp)})
}

// DeleteCoupon deletes a coupon (admin only).
func (h *CouponHandler) DeleteCoupon(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon ID"})
		return
	}

	if err := h.couponService.Delete(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete coupon", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete coupon"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Coupon deleted"})
}

// GenerateBatchCodes generates batch coupon codes (admin only).
func (h *CouponHandler) GenerateBatchCodes(c *gin.Context) {
	var req struct {
		Prefix string `json:"prefix" binding:"required"`
		Count  int    `json:"count" binding:"required,min=1,max=1000"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	codes, err := h.couponService.GenerateBatchCodes(req.Prefix, req.Count)
	if err != nil {
		h.logger.Error("Failed to generate batch codes", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate codes"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"codes": codes})
}

func (h *CouponHandler) toCouponResponse(cp *coupon.Coupon) CouponResponse {
	return CouponResponse{
		ID:             cp.ID,
		Code:           cp.Code,
		Name:           cp.Name,
		Type:           cp.Type,
		Value:          cp.Value,
		MinOrderAmount: cp.MinOrderAmount,
		MaxDiscount:    cp.MaxDiscount,
		TotalLimit:     cp.TotalLimit,
		PerUserLimit:   cp.PerUserLimit,
		UsedCount:      cp.UsedCount,
		PlanIDs:        cp.PlanIDs,
		StartAt:        cp.StartAt.Format("2006-01-02 15:04:05"),
		ExpireAt:       cp.ExpireAt.Format("2006-01-02 15:04:05"),
		IsActive:       cp.IsActive,
	}
}
