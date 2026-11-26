// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/commercial/order"
	"v/internal/database/repository"
	"v/internal/logger"
)

// ReportHandler handles report-related requests.
type ReportHandler struct {
	orderService *order.Service
	repos        *repository.Repositories
	logger       logger.Logger
}

// NewReportHandler creates a new ReportHandler.
func NewReportHandler(orderService *order.Service, repos *repository.Repositories, log logger.Logger) *ReportHandler {
	return &ReportHandler{orderService: orderService, repos: repos, logger: log}
}

type reportDateRange struct {
	start time.Time
	end   time.Time
}

type commercialOverview struct {
	Start            string                      `json:"start"`
	End              string                      `json:"end"`
	Orders           commercialOrderMetrics      `json:"orders"`
	Recharges        commercialRechargeMetrics   `json:"recharges"`
	BalancePurchases commercialPurchaseMetrics   `json:"balance_purchases"`
	Adjustments      commercialAdjustmentMetrics `json:"adjustments"`
}

type commercialOrderMetrics struct {
	Revenue int64 `json:"revenue"`
	Count   int64 `json:"count"`
}

type commercialRechargeMetrics struct {
	Amount int64 `json:"amount"`
	Count  int64 `json:"count"`
}

type commercialPurchaseMetrics struct {
	Amount int64 `json:"amount"`
	Count  int64 `json:"count"`
}

type commercialAdjustmentMetrics struct {
	IncreaseAmount int64 `json:"increase_amount"`
	DecreaseAmount int64 `json:"decrease_amount"`
	NetAmount      int64 `json:"net_amount"`
	IncreaseCount  int64 `json:"increase_count"`
	DecreaseCount  int64 `json:"decrease_count"`
	Count          int64 `json:"count"`
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func endOfDay(value time.Time) time.Time {
	return startOfDay(value).Add(24*time.Hour - time.Nanosecond)
}

func (h *ReportHandler) parseDateRange(c *gin.Context) (*reportDateRange, error) {
	now := time.Now()
	location := now.Location()
	rangeValue := &reportDateRange{
		start: startOfDay(now.AddDate(0, 0, -29)),
		end:   endOfDay(now),
	}

	if startStr := c.Query("start"); startStr != "" {
		start, err := time.ParseInLocation("2006-01-02", startStr, location)
		if err != nil {
			return nil, err
		}
		rangeValue.start = startOfDay(start)
	}

	if endStr := c.Query("end"); endStr != "" {
		end, err := time.ParseInLocation("2006-01-02", endStr, location)
		if err != nil {
			return nil, err
		}
		rangeValue.end = endOfDay(end)
	}

	if rangeValue.start.After(rangeValue.end) {
		return nil, errInvalidDateRange
	}

	return rangeValue, nil
}

var errInvalidDateRange = errors.New("invalid date range")

func (h *ReportHandler) respondDateRangeError(c *gin.Context, message string, err error) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code":    400,
		"message": message,
		"error":   err.Error(),
	})
}

// GetRevenueReport returns revenue statistics (admin only).
func (h *ReportHandler) GetRevenueReport(c *gin.Context) {
	ctx := c.Request.Context()

	dateRange, err := h.parseDateRange(c)
	if err != nil {
		if err == errInvalidDateRange {
			h.respondDateRangeError(c, "Start date must be before end date", err)
			return
		}
		h.respondDateRangeError(c, "Invalid date format. Use YYYY-MM-DD format", err)
		return
	}

	if h.orderService == nil {
		h.logger.Error("Order service is not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Order service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	revenue, err := h.orderService.GetRevenueByDateRange(ctx, dateRange.start, dateRange.end)
	if err != nil {
		h.logger.Error("Failed to get revenue", logger.Err(err),
			logger.F("start", dateRange.start),
			logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "Failed to retrieve revenue data",
			"error":   "Database query failed",
		})
		return
	}

	orderCount, err := h.orderService.GetOrderCountByDateRange(ctx, dateRange.start, dateRange.end)
	if err != nil {
		h.logger.Error("Failed to get order count", logger.Err(err),
			logger.F("start", dateRange.start),
			logger.F("end", dateRange.end))
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
			"start":       dateRange.start.Format("2006-01-02"),
			"end":         dateRange.end.Format("2006-01-02"),
		},
	})
}

// GetOrderStats returns order statistics (admin only).
func (h *ReportHandler) GetOrderStats(c *gin.Context) {
	ctx := c.Request.Context()

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

	pending, _ := h.orderService.GetOrderCountByStatus(ctx, repository.OrderStatusPending)
	paid, _ := h.orderService.GetOrderCountByStatus(ctx, repository.OrderStatusPaid)
	completed, _ := h.orderService.GetOrderCountByStatus(ctx, repository.OrderStatusCompleted)
	cancelled, _ := h.orderService.GetOrderCountByStatus(ctx, repository.OrderStatusCancelled)
	refunded, _ := h.orderService.GetOrderCountByStatus(ctx, repository.OrderStatusRefunded)

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

// GetCommercialOverview returns monetization overview metrics for the selected period (admin only).
func (h *ReportHandler) GetCommercialOverview(c *gin.Context) {
	ctx := c.Request.Context()

	dateRange, err := h.parseDateRange(c)
	if err != nil {
		if err == errInvalidDateRange {
			h.respondDateRangeError(c, "Start date must be before end date", err)
			return
		}
		h.respondDateRangeError(c, "Invalid date format. Use YYYY-MM-DD format", err)
		return
	}

	if h.orderService == nil || h.repos == nil || h.repos.DB() == nil {
		h.logger.Error("Commercial report dependencies are not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "Commercial report service is not available",
			"error":   "Service initialization failed",
		})
		return
	}

	overview := commercialOverview{
		Start: dateRange.start.Format("2006-01-02"),
		End:   dateRange.end.Format("2006-01-02"),
	}

	overview.Orders.Revenue, err = h.orderService.GetRevenueByDateRange(ctx, dateRange.start, dateRange.end)
	if err != nil {
		h.logger.Error("Failed to get commercial order revenue", logger.Err(err), logger.F("start", dateRange.start), logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve commercial report overview"})
		return
	}

	overview.Orders.Count, err = h.orderService.GetOrderCountByDateRange(ctx, dateRange.start, dateRange.end)
	if err != nil {
		h.logger.Error("Failed to get commercial paid order count", logger.Err(err), logger.F("start", dateRange.start), logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve commercial report overview"})
		return
	}

	db := h.repos.DB().WithContext(ctx)

	var rechargeMetrics commercialRechargeMetrics
	if err := db.Model(&repository.BalanceRechargeOrder{}).
		Where("status = ? AND paid_at >= ? AND paid_at <= ?", repository.BalanceRechargeStatusPaid, dateRange.start, dateRange.end).
		Select("COALESCE(SUM(amount), 0) AS amount, COUNT(*) AS count").
		Scan(&rechargeMetrics).Error; err != nil {
		h.logger.Error("Failed to get commercial recharge metrics", logger.Err(err), logger.F("start", dateRange.start), logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve commercial report overview"})
		return
	}
	overview.Recharges = rechargeMetrics

	var purchaseMetrics commercialPurchaseMetrics
	if err := db.Model(&repository.BalanceTransaction{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", repository.BalanceTxTypePurchase, dateRange.start, dateRange.end).
		Select("COALESCE(SUM(ABS(amount)), 0) AS amount, COUNT(*) AS count").
		Scan(&purchaseMetrics).Error; err != nil {
		h.logger.Error("Failed to get balance purchase metrics", logger.Err(err), logger.F("start", dateRange.start), logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve commercial report overview"})
		return
	}
	overview.BalancePurchases = purchaseMetrics

	var adjustmentMetrics commercialAdjustmentMetrics
	if err := db.Model(&repository.BalanceTransaction{}).
		Where("type = ? AND created_at >= ? AND created_at <= ?", repository.BalanceTxTypeAdjustment, dateRange.start, dateRange.end).
		Select(`
			COALESCE(SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END), 0) AS increase_amount,
			COALESCE(SUM(CASE WHEN amount < 0 THEN ABS(amount) ELSE 0 END), 0) AS decrease_amount,
			COALESCE(SUM(amount), 0) AS net_amount,
			COALESCE(SUM(CASE WHEN amount > 0 THEN 1 ELSE 0 END), 0) AS increase_count,
			COALESCE(SUM(CASE WHEN amount < 0 THEN 1 ELSE 0 END), 0) AS decrease_count,
			COUNT(*) AS count
		`).
		Scan(&adjustmentMetrics).Error; err != nil {
		h.logger.Error("Failed to get balance adjustment metrics", logger.Err(err), logger.F("start", dateRange.start), logger.F("end", dateRange.end))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve commercial report overview"})
		return
	}
	overview.Adjustments = adjustmentMetrics

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    overview,
	})
}
