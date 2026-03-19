// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/order"
	"v/internal/logger"
)

// ReportHandler handles report-related requests.
type ReportHandler struct {
	orderService *order.Service
	logger       logger.Logger
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(orderService *order.Service, log logger.Logger) *ReportHandler {
	return &ReportHandler{orderService: orderService, logger: log}
}

// GetRevenueReport returns revenue statistics (admin only).
func (h *ReportHandler) GetRevenueReport(c *gin.Context) {
	ctx := c.Request.Context()

	startStr := c.Query("start")
	endStr := c.Query("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse("2006-01-02", startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Invalid start date format. Use YYYY-MM-DD format",
				"error":   err.Error(),
			})
			return
		}
	} else {
		start = time.Now().AddDate(0, -1, 0) // Default: last month
	}

	if endStr != "" {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "Invalid end date format. Use YYYY-MM-DD format",
				"error":   err.Error(),
			})
			return
		}
	} else {
		end = time.Now()
	}

	// Validate date range
	if start.After(end) {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "Start date must be before end date",
			"error":   "Invalid date range",
		})
		return
	}

	// Check if order service is available
	if h.orderService == nil {
		h.logger.Error("Order service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Order service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	revenue, err := h.orderService.GetRevenueByDateRange(ctx, start, end)
	if err != nil {
		h.logger.Error("Failed to get revenue", logger.Err(err),
			logger.F("start", start),
			logger.F("end", end))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve revenue data",
			"error":   "Database query failed",
		})
		return
	}

	orderCount, err := h.orderService.GetOrderCountByDateRange(ctx, start, end)
	if err != nil {
		h.logger.Error("Failed to get order count", logger.Err(err),
			logger.F("start", start),
			logger.F("end", end))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve order count",
			"error":   "Database query failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"revenue":     revenue,
			"order_count": orderCount,
			"start":       start.Format("2006-01-02"),
			"end":         end.Format("2006-01-02"),
		},
	})
}

// GetOrderStats returns order statistics (admin only).
func (h *ReportHandler) GetOrderStats(c *gin.Context) {
	ctx := c.Request.Context()

	// Check if order service is available
	if h.orderService == nil {
		h.logger.Error("Order service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Order service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	total, err := h.orderService.GetOrderCount(ctx)
	if err != nil {
		h.logger.Error("Failed to get total order count", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve order statistics",
			"error":   "Database query failed",
		})
		return
	}

	// Get counts by status - don't fail if individual queries fail
	pending, _ := h.orderService.GetOrderCountByStatus(ctx, "pending")
	paid, _ := h.orderService.GetOrderCountByStatus(ctx, "paid")
	completed, _ := h.orderService.GetOrderCountByStatus(ctx, "completed")
	cancelled, _ := h.orderService.GetOrderCountByStatus(ctx, "cancelled")
	refunded, _ := h.orderService.GetOrderCountByStatus(ctx, "refunded")

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data": gin.H{
			"total":     total,
			"pending":   pending,
			"paid":      paid,
			"completed": completed,
			"cancelled": cancelled,
			"refunded":  refunded,
		},
	})
}
