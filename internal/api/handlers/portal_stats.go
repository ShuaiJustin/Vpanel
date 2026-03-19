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

	// Parse period parameter
	period := c.DefaultQuery("period", "month")
	period = stats.ValidatePeriod(period)

	// Get daily traffic for chart
	days := 30
	if period == "week" {
		days = 7
	} else if period == "day" {
		days = 1
	} else if period == "year" {
		days = 365
	}

	daily, err := h.statsService.GetDailyTraffic(c.Request.Context(), userID.(int64), days)
	if err != nil {
		h.logger.Error("failed to get daily traffic", logger.F("error", err), logger.F("user_id", userID))
		// Return empty data instead of error to avoid frontend issues
		daily = []*stats.DailyTraffic{}
	}

	// Ensure daily is not nil
	if daily == nil {
		daily = []*stats.DailyTraffic{}
	}

	// Calculate totals
	var totalUpload, totalDownload int64
	for _, d := range daily {
		totalUpload += d.Upload
		totalDownload += d.Download
	}

	c.JSON(http.StatusOK, gin.H{
		"total_upload":   totalUpload,
		"total_download": totalDownload,
		"total_traffic":  totalUpload + totalDownload,
		"daily":          daily,
		"period":         period,
	})
}

// GetUsageStats returns usage statistics by node/protocol.
func (h *PortalStatsHandler) GetUsageStats(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未认证"})
		return
	}

	// Get traffic summary
	summary, err := h.statsService.GetTrafficSummary(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("failed to get usage stats", logger.F("error", err), logger.F("user_id", userID))
		// Return empty data instead of error
		summary = &stats.TrafficSummary{
			Upload:      0,
			Download:    0,
			Total:       0,
			UploadStr:   "0 B",
			DownloadStr: "0 B",
			TotalStr:    "0 B",
		}
	}

	// Initialize empty arrays for node and protocol usage
	byNode := []map[string]interface{}{}
	byProtocol := []map[string]interface{}{}

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
