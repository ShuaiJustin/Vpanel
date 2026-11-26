// Package handlers provides HTTP request handlers.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"v/internal/commercial/currency"
	"v/internal/commercial/plan"
	"v/internal/logger"
)

// CurrencyHandler handles currency-related HTTP requests.
type CurrencyHandler struct {
	currencyService     *currency.Service
	planCurrencyService *plan.CurrencyService
	logger              logger.Logger
}

// NewCurrencyHandler creates a new currency handler.
func NewCurrencyHandler(
	currencyService *currency.Service,
	planCurrencyService *plan.CurrencyService,
	log logger.Logger,
) *CurrencyHandler {
	return &CurrencyHandler{
		currencyService:     currencyService,
		planCurrencyService: planCurrencyService,
		logger:              log,
	}
}

// GetSupportedCurrencies returns the list of supported currencies.
// GET /api/currencies
func (h *CurrencyHandler) GetSupportedCurrencies(c *gin.Context) {
	currencies := h.currencyService.GetAllCurrencyInfo()
	baseCurrency := h.currencyService.GetBaseCurrency()

	c.JSON(http.StatusOK, gin.H{
		"currencies":    currencies,
		"base_currency": baseCurrency,
	})
}

// DetectCurrency detects the user's preferred currency based on IP.
// GET /api/currencies/detect
func (h *CurrencyHandler) DetectCurrency(c *gin.Context) {
	// Get client IP
	clientIP := c.ClientIP()

	// Detect currency
	detectedCurrency := h.currencyService.DetectCurrency(c.Request.Context(), clientIP)
	currencyInfo := h.currencyService.GetCurrencyInfo(detectedCurrency)

	c.JSON(http.StatusOK, gin.H{
		"currency": detectedCurrency,
		"info":     currencyInfo,
		"ip":       clientIP,
	})
}

// GetExchangeRate returns the exchange rate between two currencies.
// GET /api/currencies/rate?from=CNY&to=USD
func (h *CurrencyHandler) GetExchangeRate(c *gin.Context) {
	from := c.Query("from")
	to := c.Query("to")

	if from == "" || to == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "from and to currencies are required",
		})
		return
	}

	rate, err := h.currencyService.GetRate(c.Request.Context(), from, to)
	if err != nil {
		h.logger.Error("Failed to get exchange rate", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"from": from,
		"to":   to,
		"rate": rate,
	})
}

// ConvertAmount converts an amount between currencies.
// POST /api/currencies/convert
func (h *CurrencyHandler) ConvertAmount(c *gin.Context) {
	var req struct {
		Amount int64  `json:"amount" binding:"required"`
		From   string `json:"from" binding:"required"`
		To     string `json:"to" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	converted, err := h.currencyService.Convert(c.Request.Context(), req.Amount, req.From, req.To)
	if err != nil {
		h.logger.Error("Failed to convert amount", logger.Err(err))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"original_amount":  req.Amount,
		"converted_amount": converted,
		"from":             req.From,
		"to":               req.To,
		"formatted":        h.currencyService.FormatPrice(converted, req.To),
	})
}

// GetPlansWithPrices returns plans with prices in user's currency.
// GET /api/plans/prices?currency=USD
func (h *CurrencyHandler) GetPlansWithPrices(c *gin.Context) {
	userCurrency := c.Query("currency")

	// If no currency specified, detect from IP
	if userCurrency == "" {
		userCurrency = h.currencyService.DetectCurrency(c.Request.Context(), c.ClientIP())
	}

	// Validate currency
	if !h.currencyService.IsCurrencySupported(userCurrency) {
		userCurrency = h.currencyService.GetBaseCurrency()
	}

	plans, err := h.planCurrencyService.ListPlansWithPrices(c.Request.Context(), userCurrency)
	if err != nil {
		h.logger.Error("Failed to list plans with prices", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get plans",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plans":    plans,
		"currency": userCurrency,
	})
}

// GetPlanPrices returns all prices for a specific plan.
// GET /api/plans/:id/prices
func (h *CurrencyHandler) GetPlanPrices(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid plan ID",
		})
		return
	}

	userCurrency := c.Query("currency")
	if userCurrency == "" {
		userCurrency = h.currencyService.DetectCurrency(c.Request.Context(), c.ClientIP())
	}

	planWithPrices, err := h.planCurrencyService.GetPlanWithPrices(c.Request.Context(), planID, userCurrency)
	if err != nil {
		h.logger.Error("Failed to get plan prices", logger.Err(err), logger.F("plan_id", planID))
		c.JSON(http.StatusNotFound, gin.H{
			"error": "plan not found",
		})
		return
	}

	c.JSON(http.StatusOK, planWithPrices)
}

// SetPlanPrices sets prices for a plan in multiple currencies (admin only).
// PUT /api/admin/plans/:id/prices
func (h *CurrencyHandler) SetPlanPrices(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid plan ID",
		})
		return
	}

	var req struct {
		Prices map[string]int64 `json:"prices" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request body",
		})
		return
	}

	if err := h.planCurrencyService.SetPricesForPlan(c.Request.Context(), planID, req.Prices); err != nil {
		h.logger.Error("Failed to set plan prices", logger.Err(err), logger.F("plan_id", planID))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "prices updated successfully",
	})
}

// DeletePlanPrice deletes a specific currency price for a plan (admin only).
// DELETE /api/admin/plans/:id/prices/:currency
func (h *CurrencyHandler) DeletePlanPrice(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid plan ID",
		})
		return
	}

	currencyCode := c.Param("currency")
	if currencyCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "currency code is required",
		})
		return
	}

	if err := h.planCurrencyService.DeletePriceInCurrency(c.Request.Context(), planID, currencyCode); err != nil {
		h.logger.Error("Failed to delete plan price", logger.Err(err), logger.F("plan_id", planID), logger.F("currency", currencyCode))
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "price deleted successfully",
	})
}

// UpdateExchangeRates triggers an update of exchange rates (admin only).
// POST /api/admin/currencies/update-rates
func (h *CurrencyHandler) UpdateExchangeRates(c *gin.Context) {
	if err := h.currencyService.UpdateRates(c.Request.Context()); err != nil {
		h.logger.Error("Failed to update exchange rates", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update exchange rates",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "exchange rates updated successfully",
	})
}
