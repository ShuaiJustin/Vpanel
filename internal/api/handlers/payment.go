// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/payment"
	"v/internal/logger"
)

// PaymentHandler handles payment-related requests.
type PaymentHandler struct {
	paymentService *payment.Service
	retryService   *payment.RetryService
	logger         logger.Logger
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(paymentService *payment.Service, log logger.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		logger:         log,
	}
}

// NewPaymentHandlerWithRetry creates a new PaymentHandler with retry service.
func NewPaymentHandlerWithRetry(paymentService *payment.Service, retryService *payment.RetryService, log logger.Logger) *PaymentHandler {
	return &PaymentHandler{
		paymentService: paymentService,
		retryService:   retryService,
		logger:         log,
	}
}

// CreatePaymentRequest represents a request to create a payment.
type CreatePaymentRequest struct {
	OrderNo string `json:"order_no" binding:"required"`
	Method  string `json:"method" binding:"required"`
}

// PaymentResponse represents a payment in API responses.
type PaymentResponse struct {
	PaymentURL string `json:"payment_url,omitempty"`
	QRCodeURL  string `json:"qrcode_url,omitempty"`
	QRCodeData string `json:"qrcode_data,omitempty"`
	ExpireTime string `json:"expire_time"`
}

// CreatePayment creates a payment for an order.
func (h *PaymentHandler) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid request body",
		})
		return
	}

	result, err := h.paymentService.CreatePayment(c.Request.Context(), req.OrderNo, req.Method, c.ClientIP())
	if err != nil {
		h.logger.Error("Failed to create payment", logger.Err(err))
		message := "Failed to create payment"
		if errors.Is(err, payment.ErrGatewayNotFound) {
			message = "Selected payment method is not available"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "PAYMENT_ERROR",
			"message": message,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"payment": PaymentResponse{
			PaymentURL: result.PaymentURL,
			QRCodeURL:  result.QRCodeURL,
			QRCodeData: result.QRCodeData,
			ExpireTime: result.ExpireTime.Format("2006-01-02 15:04:05"),
		},
	})
}

// HandleCallback handles payment callbacks from payment gateways.
func (h *PaymentHandler) HandleCallback(c *gin.Context) {
	method := c.Param("method")

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Error("Failed to read callback body", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	signature := c.GetHeader("X-Signature")
	if signature == "" {
		signature = c.Query("sign")
	}

	if err := h.paymentService.HandleCallback(c.Request.Context(), method, body, signature); err != nil {
		h.logger.Error("Failed to handle payment callback", logger.Err(err), logger.F("method", method))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return success response based on payment method
	switch method {
	case "alipay":
		c.String(http.StatusOK, "success")
	case "wechat":
		c.XML(http.StatusOK, gin.H{"return_code": "SUCCESS", "return_msg": "OK"})
	default:
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// GetPaymentStatus returns the payment status for an order.
func (h *PaymentHandler) GetPaymentStatus(c *gin.Context) {
	orderNo := c.Param("orderNo")

	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Order number is required",
		})
		return
	}

	status, err := h.paymentService.GetPaymentStatus(c.Request.Context(), orderNo)
	if err != nil {
		h.logger.Error("Failed to get payment status", logger.Err(err))
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "Order not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

// SwitchPaymentMethodRequest represents a request to switch payment method.
type SwitchPaymentMethodRequest struct {
	OrderID int64  `json:"order_id" binding:"required"`
	Method  string `json:"method" binding:"required"`
}

// RetryPaymentRequest represents a request to retry a payment.
type RetryPaymentRequest struct {
	OrderID int64  `json:"order_id" binding:"required"`
	Method  string `json:"method"` // Optional, uses original method if not provided
}

// SwitchPaymentMethod switches the payment method for a failed order.
func (h *PaymentHandler) SwitchPaymentMethod(c *gin.Context) {
	if h.retryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry service not available"})
		return
	}

	var req SwitchPaymentMethodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.retryService.SwitchPaymentMethod(c.Request.Context(), req.OrderID, req.Method); err != nil {
		h.logger.Error("Failed to switch payment method",
			logger.Err(err),
			logger.F("orderID", req.OrderID),
			logger.F("method", req.Method))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment method switched successfully"})
}

// RetryPayment retries a failed payment.
func (h *PaymentHandler) RetryPayment(c *gin.Context) {
	if h.retryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry service not available"})
		return
	}

	var req RetryPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := h.retryService.ExecuteRetry(c.Request.Context(), req.OrderID, req.Method); err != nil {
		h.logger.Error("Failed to retry payment",
			logger.Err(err),
			logger.F("orderID", req.OrderID))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Payment retry initiated"})
}

// GetRetryInfo returns retry information for an order.
func (h *PaymentHandler) GetRetryInfo(c *gin.Context) {
	if h.retryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry service not available"})
		return
	}

	orderIDStr := c.Param("orderID")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	retryInfo := h.retryService.GetRetryInfo(orderID)
	if retryInfo == nil {
		c.JSON(http.StatusOK, gin.H{
			"retry_info": nil,
			"can_retry":  true,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"retry_info": retryInfo,
		"can_retry":  h.retryService.CanRetry(orderID),
	})
}

// GetFailedPaymentStats returns statistics about failed payments (admin only).
func (h *PaymentHandler) GetFailedPaymentStats(c *gin.Context) {
	if h.retryService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Retry service not available"})
		return
	}

	stats, err := h.retryService.GetFailedPaymentStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get failed payment stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

// ListAvailablePaymentMethods returns available payment methods.
func (h *PaymentHandler) ListAvailablePaymentMethods(c *gin.Context) {
	methods := h.paymentService.ListGateways()
	c.JSON(http.StatusOK, gin.H{"methods": methods})
}
