// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/order"
	"v/internal/commercial/refund"
	"v/internal/logger"
)

// OrderHandler handles order-related requests.
type OrderHandler struct {
	orderService  *order.Service
	refundService *refund.Service
	logger        logger.Logger
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(orderService *order.Service, log logger.Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		logger:       log,
	}
}

// WithRefundService enables admin refund operations on orders.
func (h *OrderHandler) WithRefundService(refundService *refund.Service) *OrderHandler {
	h.refundService = refundService
	return h
}

// OrderResponse represents an order in API responses.
type OrderResponse struct {
	ID             int64   `json:"id"`
	OrderNo        string  `json:"order_no"`
	UserID         int64   `json:"user_id"`
	PlanID         int64   `json:"plan_id"`
	PlanName       string  `json:"plan_name,omitempty"`
	CouponID       *int64  `json:"coupon_id,omitempty"`
	OriginalAmount int64   `json:"original_amount"`
	DiscountAmount int64   `json:"discount_amount"`
	BalanceUsed    int64   `json:"balance_used"`
	PayAmount      int64   `json:"pay_amount"`
	Status         string  `json:"status"`
	PaymentMethod  string  `json:"payment_method,omitempty"`
	PaymentNo      string  `json:"payment_no,omitempty"`
	PaidAt         *string `json:"paid_at,omitempty"`
	ExpiredAt      string  `json:"expired_at"`
	CreatedAt      string  `json:"created_at"`
}

// CreateOrderRequest represents a request to create an order.
type CreateOrderRequest struct {
	PlanID     int64  `json:"plan_id" binding:"required,gt=0"`
	CouponCode string `json:"coupon_code"`
}

// CreateOrder creates a new order.
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid request body",
		})
		return
	}

	createReq := &order.CreateOrderRequest{
		UserID:     userID.(int64),
		PlanID:     req.PlanID,
		CouponCode: req.CouponCode,
	}

	o, err := h.orderService.Create(c.Request.Context(), createReq)
	if err != nil {
		h.logger.Error("Failed to create order", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "ORDER_ERROR",
			"message": "Failed to create order",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"order": h.toOrderResponse(o)})
}

// GetOrder returns an order by ID.
func (h *OrderHandler) GetOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid order ID",
		})
		return
	}

	o, err := h.orderService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "Order not found",
		})
		return
	}

	// Check if user owns this order (unless admin)
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "admin" && o.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Access denied",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": h.toOrderResponse(o)})
}

// GetOrderByOrderNo returns an order by order number for the current user.
func (h *OrderHandler) GetOrderByOrderNo(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid order number",
		})
		return
	}

	o, err := h.orderService.GetByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "Order not found",
		})
		return
	}

	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "admin" && o.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Access denied",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"order": h.toOrderResponse(o)})
}

// ListUserOrders returns orders for the current user.
func (h *OrderHandler) ListUserOrders(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := strings.TrimSpace(c.Query("status"))

	filter := order.OrderFilter{
		UserID: func(id int64) *int64 { return &id }(userID.(int64)),
		Status: status,
	}

	orders, total, err := h.orderService.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list orders", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list orders"})
		return
	}

	response := make([]OrderResponse, len(orders))
	for i, o := range orders {
		response[i] = h.toOrderResponse(o)
	}

	c.JSON(http.StatusOK, gin.H{"orders": response, "total": total, "page": page, "page_size": pageSize})
}

// CancelOrder cancels a pending order.
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Verify ownership
	o, err := h.orderService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "Order not found",
		})
		return
	}

	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")
	if role != "admin" && o.UserID != userID.(int64) {
		c.JSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "Access denied",
		})
		return
	}

	if err := h.orderService.Cancel(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to cancel order", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled"})
}

// ListAllOrders returns all orders (admin only).
func (h *OrderHandler) ListAllOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	filter := order.OrderFilter{Status: status}
	filter.Search = strings.TrimSpace(c.Query("search"))
	filter.PaymentMethod = strings.TrimSpace(c.Query("payment_method"))

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		parsed, err := parseOrderFilterTime(startDate, false)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date"})
			return
		}
		filter.StartDate = &parsed
	}

	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		parsed, err := parseOrderFilterTime(endDate, true)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date"})
			return
		}
		filter.EndDate = &parsed
	}

	if minAmount := strings.TrimSpace(c.Query("min_amount")); minAmount != "" {
		parsed, err := strconv.ParseInt(minAmount, 10, 64)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid min amount"})
			return
		}
		filter.MinAmount = &parsed
	}

	if maxAmount := strings.TrimSpace(c.Query("max_amount")); maxAmount != "" {
		parsed, err := strconv.ParseInt(maxAmount, 10, 64)
		if err != nil || parsed < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid max amount"})
			return
		}
		filter.MaxAmount = &parsed
	}

	orders, total, err := h.orderService.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list orders", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list orders"})
		return
	}

	response := make([]OrderResponse, len(orders))
	for i, o := range orders {
		response[i] = h.toOrderResponse(o)
	}

	c.JSON(http.StatusOK, gin.H{"orders": response, "total": total, "page": page, "page_size": pageSize})
}

// UpdateOrderStatus updates an order's status (admin only).
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.orderService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		h.logger.Error("Failed to update order status", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated"})
}

// RefundOrder processes a manual refund for an order (admin only).
func (h *OrderHandler) RefundOrder(c *gin.Context) {
	if h.refundService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Refund service not available"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var req struct {
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	result, err := h.refundService.ProcessRefund(c.Request.Context(), &refund.RefundRequest{
		OrderID: id,
		Amount:  req.Amount,
		Reason:  strings.TrimSpace(req.Reason),
	})
	if err != nil {
		h.logger.Error("Failed to refund order", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"refund": result, "message": "Refund processed"})
}

func parseOrderFilterTime(value string, inclusiveEnd bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("invalid time format")
	}

	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}

	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local); err == nil {
		return parsed, nil
	}

	if parsed, err := time.ParseInLocation("2006-01-02", value, time.Local); err == nil {
		if inclusiveEnd {
			return parsed.Add(24*time.Hour - time.Nanosecond), nil
		}
		return parsed, nil
	}

	return time.Time{}, errors.New("invalid time format")
}

func (h *OrderHandler) toOrderResponse(o *order.Order) OrderResponse {
	resp := OrderResponse{
		ID:             o.ID,
		OrderNo:        o.OrderNo,
		UserID:         o.UserID,
		PlanID:         o.PlanID,
		PlanName:       o.PlanName,
		CouponID:       o.CouponID,
		OriginalAmount: o.OriginalAmount,
		DiscountAmount: o.DiscountAmount,
		BalanceUsed:    o.BalanceUsed,
		PayAmount:      o.PayAmount,
		Status:         o.Status,
		PaymentMethod:  o.PaymentMethod,
		PaymentNo:      o.PaymentNo,
		ExpiredAt:      o.ExpiredAt.Format("2006-01-02 15:04:05"),
		CreatedAt:      o.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if o.PaidAt != nil {
		paidAt := o.PaidAt.Format("2006-01-02 15:04:05")
		resp.PaidAt = &paidAt
	}
	return resp
}
