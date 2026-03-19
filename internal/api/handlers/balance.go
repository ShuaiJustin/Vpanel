// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/commercial/balance"
	"v/internal/logger"
)

// BalanceHandler handles balance-related requests.
type BalanceHandler struct {
	balanceService *balance.Service
	logger         logger.Logger
}

// NewBalanceHandler creates a new BalanceHandler.
func NewBalanceHandler(balanceService *balance.Service, log logger.Logger) *BalanceHandler {
	return &BalanceHandler{
		balanceService: balanceService,
		logger:         log,
	}
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

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	txs, total, err := h.balanceService.GetTransactions(c.Request.Context(), userID.(int64), page, pageSize)
	if err != nil {
		h.logger.Error("Failed to get transactions", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
		return
	}

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

	c.JSON(http.StatusOK, gin.H{"transactions": response, "total": total, "page": page, "page_size": pageSize})
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

	// Get operator info from context
	operator, _ := c.Get("username")
	operatorID, _ := c.Get("user_id")
	operatorStr := ""
	if operator != nil {
		operatorStr = operator.(string)
	}

	// Log the adjustment operation
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
