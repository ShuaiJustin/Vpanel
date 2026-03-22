// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/portal/stats"
)

// PortalStatsHandler handles portal statistics requests.
type PortalStatsHandler struct {
	statsService *stats.Service
	logger       logger.Logger
}

// NewPortalStatsHandler creates a new PortalStatsHandler.
func NewPortalStatsHandler(statsService *stats.Service, log logger.Logger) *PortalStatsHandler {
	return &PortalStatsHandler{
		statsService: statsService,
		logger:       log,
	}
}

// GetTrafficStats returns traffic statistics for the current user.
func (h *PortalStatsHandler) GetTrafficStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	period := c.DefaultQuery("period", "month")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	resolvedPeriod, start, end, err := stats.ResolveRange(period, startDate, endDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trafficStats, err := h.statsService.GetTrafficStatsInRange(c.Request.Context(), userID.(int64), resolvedPeriod, start, end)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get traffic stats", logger.F("error", err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取流量统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total_upload":   trafficStats.Summary.Upload,
		"total_download": trafficStats.Summary.Download,
		"total_traffic":  trafficStats.Summary.Total,
		"daily":          trafficStats.Daily,
		"period":         trafficStats.Period,
		"start_date":     trafficStats.StartDate,
		"end_date":       trafficStats.EndDate,
	})
}

// GetUsageStats returns usage statistics by node/protocol.
func (h *PortalStatsHandler) GetUsageStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	period := c.DefaultQuery("period", "month")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	summary, byNode, byProtocol, err := h.statsService.GetUsageStats(c.Request.Context(), userID.(int64), period, startDate, endDate)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get usage stats", logger.F("error", err), logger.F("user_id", userID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取使用统计失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":     summary,
		"by_node":     byNode,
		"by_protocol": byProtocol,
	})
}

// ExportStats exports traffic statistics as CSV.
func (h *PortalStatsHandler) ExportStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Parse days parameter
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	csvData, err := h.statsService.ExportTrafficCSV(c.Request.Context(), userID.(int64), days)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to export stats", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出统计数据失败"})
		return
	}

	// Set headers for CSV download
	filename := fmt.Sprintf("traffic_stats_%s.csv", time.Now().Format("20060102"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", csvData)
}

// GetDailyTraffic returns daily traffic data.
func (h *PortalStatsHandler) GetDailyTraffic(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Parse days parameter
	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 365 {
			days = parsed
		}
	}

	daily, err := h.statsService.GetDailyTraffic(c.Request.Context(), userID.(int64), days)
	if err != nil {
		if handleRequestContextError(c, err) {
			return
		}
		h.logger.Error("failed to get daily traffic", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取每日流量失败"})
		return
	}

	// Calculate aggregate
	aggregate := stats.AggregateDaily(daily)

	c.JSON(http.StatusOK, gin.H{
		"daily":     daily,
		"aggregate": aggregate,
		"days":      days,
	})
}
