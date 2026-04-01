// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/plan"
	"v/internal/logger"
)

// PlanHandler handles plan-related requests.
type PlanHandler struct {
	planService *plan.Service
	logger      logger.Logger
}

// NewPlanHandler creates a new PlanHandler.
func NewPlanHandler(planService *plan.Service, log logger.Logger) *PlanHandler {
	return &PlanHandler{
		planService: planService,
		logger:      log,
	}
}

// PlanResponse represents a plan in API responses.
type PlanResponse struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	TrafficLimit  int64    `json:"traffic_limit"`
	Duration      int      `json:"duration"`
	Price         int64    `json:"price"`
	PlanType      string   `json:"plan_type"`
	ResetCycle    string   `json:"reset_cycle"`
	IPLimit       int      `json:"ip_limit"`
	SortOrder     int      `json:"sort_order"`
	IsActive      bool     `json:"is_active"`
	IsRecommended bool     `json:"is_recommended"`
	Features      []string `json:"features"`
	MonthlyPrice  int64    `json:"monthly_price"`
}

// CreatePlanRequest represents a request to create a plan.
type CreatePlanRequest struct {
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	TrafficLimit  int64    `json:"traffic_limit"`
	Duration      int      `json:"duration" binding:"required,min=1"`
	Price         int64    `json:"price" binding:"required,min=0"`
	PlanType      string   `json:"plan_type"`
	ResetCycle    string   `json:"reset_cycle"`
	IPLimit       int      `json:"ip_limit"`
	SortOrder     int      `json:"sort_order"`
	IsActive      bool     `json:"is_active"`
	IsRecommended bool     `json:"is_recommended"`
	Features      []string `json:"features"`
}

type UpdatePlanRequest struct {
	Name          *string   `json:"name"`
	Description   *string   `json:"description"`
	TrafficLimit  *int64    `json:"traffic_limit"`
	Duration      *int      `json:"duration"`
	Price         *int64    `json:"price"`
	PlanType      *string   `json:"plan_type"`
	ResetCycle    *string   `json:"reset_cycle"`
	IPLimit       *int      `json:"ip_limit"`
	SortOrder     *int      `json:"sort_order"`
	IsActive      *bool     `json:"is_active"`
	IsRecommended *bool     `json:"is_recommended"`
	Features      *[]string `json:"features"`
}

func (h *PlanHandler) respondPlanError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, plan.ErrPlanNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
	case errors.Is(err, plan.ErrInvalidPlan):
		message := err.Error()
		message = strings.TrimPrefix(message, plan.ErrInvalidPlan.Error()+": ")
		c.JSON(http.StatusBadRequest, gin.H{"error": message})
	default:
		h.logger.Error(fallback, logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fallback})
	}
}

// ListActivePlans returns all active plans (public endpoint).
func (h *PlanHandler) ListActivePlans(c *gin.Context) {
	plans, err := h.planService.ListActive(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list active plans", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list plans"})
		return
	}

	response := make([]PlanResponse, len(plans))
	for i, p := range plans {
		response[i] = h.toPlanResponse(p)
	}

	c.JSON(http.StatusOK, gin.H{"plans": response})
}

// GetPlan returns a plan by ID.
func (h *PlanHandler) GetPlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID"})
		return
	}

	p, err := h.planService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"plan": h.toPlanResponse(p)})
}

// CreatePlan creates a new plan (admin only).
func (h *PlanHandler) CreatePlan(c *gin.Context) {
	var req CreatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	createReq := &plan.CreatePlanRequest{
		Name:          req.Name,
		Description:   req.Description,
		TrafficLimit:  req.TrafficLimit,
		Duration:      req.Duration,
		Price:         req.Price,
		PlanType:      req.PlanType,
		ResetCycle:    req.ResetCycle,
		IPLimit:       req.IPLimit,
		SortOrder:     req.SortOrder,
		IsActive:      req.IsActive,
		IsRecommended: req.IsRecommended,
		Features:      req.Features,
	}

	p, err := h.planService.Create(c.Request.Context(), createReq)
	if err != nil {
		h.respondPlanError(c, err, "Failed to create plan")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"plan": h.toPlanResponse(p)})
}

// UpdatePlan updates a plan (admin only).
func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID"})
		return
	}

	var req UpdatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	updateReq := &plan.UpdatePlanRequest{
		Name:          req.Name,
		Description:   req.Description,
		TrafficLimit:  req.TrafficLimit,
		Duration:      req.Duration,
		Price:         req.Price,
		PlanType:      req.PlanType,
		ResetCycle:    req.ResetCycle,
		IPLimit:       req.IPLimit,
		SortOrder:     req.SortOrder,
		IsActive:      req.IsActive,
		IsRecommended: req.IsRecommended,
		Features:      req.Features,
	}

	p, err := h.planService.Update(c.Request.Context(), id, updateReq)
	if err != nil {
		h.respondPlanError(c, err, "Failed to update plan")
		return
	}

	c.JSON(http.StatusOK, gin.H{"plan": h.toPlanResponse(p)})
}

// DeletePlan deletes a plan (admin only).
func (h *PlanHandler) DeletePlan(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID"})
		return
	}

	if err := h.planService.Delete(c.Request.Context(), id); err != nil {
		h.respondPlanError(c, err, "Failed to delete plan")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plan deleted"})
}

// TogglePlanStatus toggles a plan's active status (admin only).
func (h *PlanHandler) TogglePlanStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid plan ID"})
		return
	}

	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.planService.SetActive(c.Request.Context(), id, req.IsActive); err != nil {
		h.respondPlanError(c, err, "Failed to update plan status")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Plan status updated"})
}

// ListAllPlans returns all plans (admin only).
func (h *PlanHandler) ListAllPlans(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	includeInactive := c.DefaultQuery("include_inactive", "false") == "true"

	filter := plan.PlanFilter{}
	if !includeInactive {
		active := true
		filter.IsActive = &active
	}

	plans, total, err := h.planService.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list plans", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list plans"})
		return
	}

	response := make([]PlanResponse, len(plans))
	for i, p := range plans {
		response[i] = h.toPlanResponse(p)
	}

	c.JSON(http.StatusOK, gin.H{"plans": response, "total": total, "page": page, "page_size": pageSize})
}

func (h *PlanHandler) toPlanResponse(p *plan.Plan) PlanResponse {
	return PlanResponse{
		ID:            p.ID,
		Name:          p.Name,
		Description:   p.Description,
		TrafficLimit:  p.TrafficLimit,
		Duration:      p.Duration,
		Price:         p.Price,
		PlanType:      p.PlanType,
		ResetCycle:    p.ResetCycle,
		IPLimit:       p.IPLimit,
		SortOrder:     p.SortOrder,
		IsActive:      p.IsActive,
		IsRecommended: p.IsRecommended,
		Features:      p.Features,
		MonthlyPrice:  h.planService.CalculateMonthlyPrice(p),
	}
}
