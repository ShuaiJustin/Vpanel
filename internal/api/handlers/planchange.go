// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/planchange"
	"v/internal/logger"
)

// PlanChangeHandler handles plan change related HTTP requests.
type PlanChangeHandler struct {
	planChangeService *planchange.Service
	logger            logger.Logger
}

// NewPlanChangeHandler creates a new plan change handler.
func NewPlanChangeHandler(planChangeService *planchange.Service, log logger.Logger) *PlanChangeHandler {
	return &PlanChangeHandler{
		planChangeService: planChangeService,
		logger:            log,
	}
}

// CalculatePlanChangeRequest represents the request body for calculating plan change.
type CalculatePlanChangeRequest struct {
	CurrentPlanID int64 `json:"current_plan_id"`
	NewPlanID     int64 `json:"new_plan_id" binding:"required"`
}

// UpgradePlanRequest represents the request body for upgrading a plan.
type UpgradePlanRequest struct {
	CurrentPlanID int64 `json:"current_plan_id"`
	NewPlanID     int64 `json:"new_plan_id" binding:"required"`
}

// DowngradePlanRequest represents the request body for downgrading a plan.
type DowngradePlanRequest struct {
	CurrentPlanID int64 `json:"current_plan_id"`
	NewPlanID     int64 `json:"new_plan_id" binding:"required"`
}

// CalculatePlanChange calculates the price difference for a plan change.
func (h *PlanChangeHandler) GetCurrentPlan(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "User not authenticated"})
		return
	}
	result, err := h.planChangeService.GetCurrentPlan(c.Request.Context(), userID.(int64))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

// POST /api/plan-change/calculate
func (h *PlanChangeHandler) CalculatePlanChange(c *gin.Context) {
	var req CalculatePlanChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
		return
	}

	changeReq := &planchange.PlanChangeRequest{
		UserID:        userID.(int64),
		CurrentPlanID: req.CurrentPlanID,
		NewPlanID:     req.NewPlanID,
	}

	result, err := h.planChangeService.CalculateChange(c.Request.Context(), changeReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": result,
	})
}

// UpgradePlan executes an immediate plan upgrade.
// POST /api/plan-change/upgrade
func (h *PlanChangeHandler) UpgradePlan(c *gin.Context) {
	var req UpgradePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
		return
	}

	changeReq := &planchange.PlanChangeRequest{
		UserID:        userID.(int64),
		CurrentPlanID: req.CurrentPlanID,
		NewPlanID:     req.NewPlanID,
	}

	_, err := h.planChangeService.ExecuteUpgrade(c.Request.Context(), changeReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plan upgraded successfully",
	})
}

// DowngradePlan schedules a plan downgrade for the next billing cycle.
// POST /api/plan-change/downgrade
func (h *PlanChangeHandler) DowngradePlan(c *gin.Context) {
	var req DowngradePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INVALID_REQUEST",
			"message": "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
		return
	}

	changeReq := &planchange.PlanChangeRequest{
		UserID:        userID.(int64),
		CurrentPlanID: req.CurrentPlanID,
		NewPlanID:     req.NewPlanID,
	}

	err := h.planChangeService.ScheduleDowngrade(c.Request.Context(), changeReq)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Plan downgrade scheduled successfully",
	})
}

// GetPendingDowngrade retrieves the user's pending downgrade.
// GET /api/plan-change/downgrade
func (h *PlanChangeHandler) GetPendingDowngrade(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
		return
	}

	downgrade, err := h.planChangeService.GetPendingDowngrade(c.Request.Context(), userID.(int64))
	if err != nil {
		if err == planchange.ErrNoPendingDowngrade {
			c.JSON(http.StatusOK, gin.H{
				"data": nil,
			})
			return
		}
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": downgrade,
	})
}

// CancelPendingDowngrade cancels the user's pending downgrade.
// DELETE /api/plan-change/downgrade
func (h *PlanChangeHandler) CancelPendingDowngrade(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "User not authenticated",
		})
		return
	}

	err := h.planChangeService.CancelPendingDowngrade(c.Request.Context(), userID.(int64))
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Pending downgrade cancelled successfully",
	})
}

// AdminListPendingDowngrades lists all pending downgrades (admin only).
// GET /api/admin/plan-changes/downgrades
func (h *PlanChangeHandler) AdminListPendingDowngrades(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	items, total, err := h.planChangeService.ListPendingDowngrades(c.Request.Context(), page, pageSize)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":  items,
		"total": total,
		"page":  page,
		"size":  pageSize,
	})
}

// handleError handles errors and returns appropriate HTTP responses.
func (h *PlanChangeHandler) handleError(c *gin.Context, err error) {
	switch err {
	case planchange.ErrCurrentPlanUnknown:
		c.JSON(http.StatusConflict, gin.H{"code": "CURRENT_PLAN_UNKNOWN", "message": "无法从已支付订单确认当前套餐，请联系管理员核对套餐归属"})
	case planchange.ErrPlanNotFound:
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "PLAN_NOT_FOUND",
			"message": "Plan not found",
		})
	case planchange.ErrPlanInactive:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "PLAN_INACTIVE",
			"message": "Plan is not active",
		})
	case planchange.ErrUserNotFound:
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "USER_NOT_FOUND",
			"message": "User not found",
		})
	case planchange.ErrNoActiveSubscription:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "NO_ACTIVE_SUBSCRIPTION",
			"message": "User has no active subscription",
		})
	case planchange.ErrSamePlan:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "SAME_PLAN",
			"message": "Cannot change to the same plan",
		})
	case planchange.ErrDowngradeNotAllowed:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "DOWNGRADE_NOT_ALLOWED",
			"message": "This is an upgrade, not a downgrade",
		})
	case planchange.ErrUpgradeNotAllowed:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "UPGRADE_NOT_ALLOWED",
			"message": "This is a downgrade, not an upgrade",
		})
	case planchange.ErrPendingDowngrade:
		c.JSON(http.StatusConflict, gin.H{
			"code":    "PENDING_DOWNGRADE_EXISTS",
			"message": "User already has a pending downgrade",
		})
	case planchange.ErrNoPendingDowngrade:
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NO_PENDING_DOWNGRADE",
			"message": "User has no pending downgrade",
		})
	case planchange.ErrInsufficientBalance:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "INSUFFICIENT_BALANCE",
			"message": "Insufficient balance for upgrade",
		})
	default:
		h.logger.Error("Plan change error", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "INTERNAL_ERROR",
			"message": "An internal error occurred",
		})
	}
}
