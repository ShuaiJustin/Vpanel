// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/commercial/balance"
	"v/internal/commercial/payment"
	"v/internal/logger"
)

// BalanceHandler handles balance-related requests.
type BalanceHandler struct {
	balanceService *balance.Service
	paymentService *payment.Service
	logger         logger.Logger
}

// NewBalanceHandler creates a new BalanceHandler.
func NewBalanceHandler(balanceService *balance.Service, log logger.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
		logger:         log,
	}
}

// WithPaymentService enables online recharge payment creation.
func (h *BalanceHandler) WithPaymentService(paymentService *payment.Service) *BalanceHandler {
	h.paymentService = paymentService
	return h
}

// TransactionResponse represents a balance transaction in API responses.
type TransactionResponse struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Amount      int64  `json:"amount"`
	Balance     int64  `json:"balance"`
	OrderID     *int64 `json:"order_id,omitempty"`
	Description string `json:"description"`
	Operator    string `json:"operator,omitempty"`
	CreatedAt   string `json:"created_at"`
}

// AdminRechargeOrderResponse represents a recharge order in admin API responses.
type AdminRechargeOrderResponse struct {
	ID        int64   `json:"id"`
	OrderNo   string  `json:"order_no"`
	UserID    int64   `json:"user_id"`
	Username  string  `json:"username,omitempty"`
	Amount    int64   `json:"amount"`
	Method    string  `json:"method"`
	Status    string  `json:"status"`
	PaymentNo string  `json:"payment_no,omitempty"`
	PaidAt    *string `json:"paid_at,omitempty"`
	ExpiredAt string  `json:"expired_at"`
	CreatedAt string  `json:"created_at"`
}

func buildTransactionResponses(txs []*balance.Transaction) []TransactionResponse {
	response := make([]TransactionResponse, len(txs))
	for i, tx := range txs {
		response[i] = TransactionResponse{
			ID:          tx.ID,
			Type:        tx.Type,
			Amount:      tx.Amount,
			Balance:     tx.Balance,
			OrderID:     tx.OrderID,
			Description: tx.Description,
			Operator:    tx.Operator,
			CreatedAt:   tx.CreatedAt,
		}
	}
	return response
}

func parseTransactionFilter(c *gin.Context, userID *int64) (balance.TransactionFilter, int, int, bool) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	filter := balance.TransactionFilter{
		UserID: userID,
		Type:   strings.TrimSpace(c.Query("type")),
	}

	if startDate := strings.TrimSpace(c.Query("start_date")); startDate != "" {
		parsed, err := parseOrderFilterTime(startDate, false)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date"})
			return balance.TransactionFilter{}, 0, 0, false
		}
		filter.StartDate = &parsed
	}

	if endDate := strings.TrimSpace(c.Query("end_date")); endDate != "" {
		parsed, err := parseOrderFilterTime(endDate, true)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date"})
			return balance.TransactionFilter{}, 0, 0, false
		}
		filter.EndDate = &parsed
	}

	return filter, page, pageSize, true
}

// GetBalance returns the current user's balance.
func (h *BalanceHandler) GetBalance(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	bal, err := h.balanceService.GetBalance(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to get balance", logger.Err(err))
		middleware.HandleInternalError(c, "获取余额失败", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"balance": bal})
}

// GetTransactions returns the current user's transaction history.
func (h *BalanceHandler) GetTransactions(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	uid := userID.(int64)
	filter, page, pageSize, ok := parseTransactionFilter(c, &uid)
	if !ok {
		return
	}

	txs, total, err := h.balanceService.ListTransactions(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get transactions", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": buildTransactionResponses(txs), "total": total, "page": page, "page_size": pageSize})
}

// CreateRecharge creates an online recharge payment.
func (h *BalanceHandler) CreateRecharge(c *gin.Context) {
	if h.paymentService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "RECHARGE_UNAVAILABLE",
			"message": "当前未开启在线充值，请联系管理员或使用礼品卡充值。",
		})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	var req struct {
		Amount int64  `json:"amount" binding:"required,gt=0"`
		Method string `json:"method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "请求参数不正确，请检查充值金额和支付方式。",
		})
		return
	}

	method := strings.TrimSpace(req.Method)
	if method == "" || method == "balance" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "PAYMENT_METHOD_UNAVAILABLE",
			"message": "当前支付方式暂不可用，请选择其他支付方式。",
		})
		return
	}

	rechargeOrder, err := h.balanceService.CreateRechargeOrder(c.Request.Context(), userID.(int64), req.Amount, method)
	if err != nil {
		h.logger.Error("Failed to create recharge order", logger.Err(err))
		switch {
		case err == balance.ErrRechargeUnavailable:
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    "RECHARGE_UNAVAILABLE",
				"message": "当前未开启在线充值，请联系管理员或使用礼品卡充值。",
			})
		case err == balance.ErrInvalidRechargeMethod:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "PAYMENT_METHOD_UNAVAILABLE",
				"message": "当前支付方式暂不可用，请选择其他支付方式。",
			})
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "PAYMENT_ERROR",
				"message": "创建充值订单失败，请稍后重试。",
			})
		}
		return
	}

	paymentOrder := &payment.PaymentOrder{
		OrderNo:     rechargeOrder.OrderNo,
		Amount:      rechargeOrder.Amount,
		Subject:     fmt.Sprintf("账户余额充值 ¥%.2f", float64(rechargeOrder.Amount)/100),
		Description: fmt.Sprintf("账户余额充值订单 %s", rechargeOrder.OrderNo),
		ClientIP:    c.ClientIP(),
	}

	result, err := h.paymentService.CreateGatewayPayment(method, paymentOrder)
	if err != nil {
		h.logger.Error("Failed to create recharge payment", logger.Err(err), logger.F("orderNo", rechargeOrder.OrderNo))
		status, payload := getCreatePaymentErrorResponse(err)
		if code, ok := payload["code"].(string); ok && code == "PAYMENT_ERROR" {
			payload["message"] = "创建充值支付失败，请稍后重试。"
		}
		c.JSON(status, payload)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_no": rechargeOrder.OrderNo,
		"payment": PaymentResponse{
			PaymentURL: result.PaymentURL,
			QRCodeURL:  result.QRCodeURL,
			QRCodeData: result.QRCodeData,
			ExpireTime: result.ExpireTime.Format("2006-01-02 15:04:05"),
		},
	})
}

// GetRechargeStatus returns the current recharge order status for the signed-in user.
func (h *BalanceHandler) GetRechargeStatus(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Authentication required",
		})
		return
	}

	orderNo := strings.TrimSpace(c.Param("orderNo"))
	if orderNo == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "充值订单号不能为空。",
		})
		return
	}

	rechargeOrder, err := h.balanceService.GetRechargeOrderByOrderNo(c.Request.Context(), orderNo)
	if err != nil {
		status := http.StatusNotFound
		message := "充值订单不存在或已失效。"
		if err == balance.ErrRechargeUnavailable {
			status = http.StatusServiceUnavailable
			message = "当前未开启在线充值，请联系管理员或使用礼品卡充值。"
		}
		c.JSON(status, gin.H{
			"code":    "NOT_FOUND",
			"message": message,
		})
		return
	}

	if rechargeOrder.UserID != userID.(int64) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    "NOT_FOUND",
			"message": "充值订单不存在或已失效。",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   rechargeOrder.Status,
		"order_no": rechargeOrder.OrderNo,
	})
}

// ListAdminRechargeOrders returns recharge orders (admin only).
func (h *BalanceHandler) ListAdminRechargeOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := balance.RechargeOrderFilter{
		Search: strings.TrimSpace(c.Query("search")),
		Status: strings.TrimSpace(c.Query("status")),
		Method: strings.TrimSpace(c.Query("method")),
	}

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

	orders, total, err := h.balanceService.ListRechargeOrders(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list recharge orders", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list recharge orders"})
		return
	}

	response := make([]AdminRechargeOrderResponse, len(orders))
	for i, order := range orders {
		var paidAt *string
		if order.PaidAt != nil {
			formatted := order.PaidAt.Format("2006-01-02 15:04:05")
			paidAt = &formatted
		}

		response[i] = AdminRechargeOrderResponse{
			ID:        order.ID,
			OrderNo:   order.OrderNo,
			UserID:    order.UserID,
			Username:  order.Username,
			Amount:    order.Amount,
			Method:    order.Method,
			Status:    order.Status,
			PaymentNo: order.PaymentNo,
			PaidAt:    paidAt,
			ExpiredAt: order.ExpiredAt.Format("2006-01-02 15:04:05"),
			CreatedAt: order.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	c.JSON(http.StatusOK, gin.H{"orders": response, "total": total, "page": page, "page_size": pageSize})
}

// AdminGetUserBalance returns a user's current balance (admin only).
func (h *BalanceHandler) AdminGetUserBalance(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	bal, err := h.balanceService.GetBalance(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user balance", logger.Err(err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user balance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user_id": userID, "balance": bal})
}

// AdminGetUserTransactions returns a user's balance transactions (admin only).
func (h *BalanceHandler) AdminGetUserTransactions(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	filter, page, pageSize, ok := parseTransactionFilter(c, &userID)
	if !ok {
		return
	}

	txs, total, err := h.balanceService.ListTransactions(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get user transactions", logger.Err(err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": buildTransactionResponses(txs), "total": total, "page": page, "page_size": pageSize})
}

// AdjustBalance adjusts a user's balance (admin only).
func (h *BalanceHandler) AdjustBalance(c *gin.Context) {
	var req struct {
		UserID int64  `json:"user_id" binding:"required,gt=0"`
		Amount int64  `json:"amount" binding:"required"`
		Reason string `json:"reason" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "VALIDATION_ERROR",
			"message": "Invalid request body",
		})
		return
	}

	operator, _ := c.Get("username")
	operatorID, _ := c.Get("user_id")
	operatorStr := ""
	if operator != nil {
		operatorStr = operator.(string)
	}

	h.logger.Info("Balance adjustment requested",
		logger.F("target_user_id", req.UserID),
		logger.F("operator", operatorStr),
		logger.F("operator_id", operatorID),
		logger.F("amount", req.Amount),
		logger.F("reason", req.Reason))

	if err := h.balanceService.Adjust(c.Request.Context(), req.UserID, req.Amount, req.Reason, operatorStr); err != nil {
		h.logger.Error("Failed to adjust balance",
			logger.Err(err),
			logger.F("target_user_id", req.UserID),
			logger.F("operator", operatorStr))
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "BALANCE_ERROR",
			"message": "Failed to adjust balance",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Balance adjusted successfully"})
}
