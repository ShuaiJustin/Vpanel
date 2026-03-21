package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/log"
	"v/internal/logger"
)

// LogHandler handles log-related API requests.
type LogHandler struct {
	service *log.Service
	logger  logger.Logger
}

type cleanupLogsRequest struct {
	RetentionDays *int `json:"retention_days"`
	Days          *int `json:"days"`
}

// NewLogHandler creates a new log handler.
func NewLogHandler(service *log.Service, log logger.Logger) *LogHandler {
	return &LogHandler{
		service: service,
		logger:  log,
	}
}

// ListLogs retrieves logs with filtering and pagination.
// GET /api/logs
func (h *LogHandler) ListLogs(c *gin.Context) {
	filter := &repository.LogFilter{}

	if level := c.Query("level"); level != "" {
		filter.Level = level
	}
	if minLevel := c.Query("min_level"); minLevel != "" {
		filter.MinLevel = minLevel
	}
	if source := c.Query("source"); source != "" {
		filter.Source = source
	}
	if userID := c.Query("user_id"); userID != "" {
		if id, err := strconv.ParseInt(userID, 10, 64); err == nil {
			filter.UserID = &id
		}
	}
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}
	if keyword := c.Query("keyword"); keyword != "" {
		filter.Keyword = keyword
	}
	if requestID := c.Query("request_id"); requestID != "" {
		filter.RequestID = requestID
	}

	// Support both page/page_size and limit/offset
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	// Also support limit/offset for backward compatibility
	if limit := c.Query("limit"); limit != "" {
		pageSize, _ = strconv.Atoi(limit)
	}
	if offset := c.Query("offset"); offset != "" {
		offsetVal, _ := strconv.Atoi(offset)
		if pageSize > 0 {
			page = (offsetVal / pageSize) + 1
		}
	}

	// Ensure page is at least 1
	if page < 1 {
		page = 1
	}

	// Cap page_size at 1000
	if pageSize > 1000 {
		pageSize = 1000
	}
	if pageSize < 1 {
		pageSize = 50
	}

	offset := (page - 1) * pageSize

	logs, total, err := h.service.Query(c.Request.Context(), filter, pageSize, offset)
	if err != nil {
		h.logger.Error("failed to query logs", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"limit":     pageSize,
		"offset":    offset,
	})
}

// GetLog retrieves a single log entry by ID.
// GET /api/logs/:id
func (h *LogHandler) GetLog(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log ID"})
		return
	}

	log, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("failed to get log", logger.F("error", err), logger.F("id", id))
		c.JSON(http.StatusNotFound, gin.H{"error": "Log not found"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// DeleteLogs deletes logs matching the filter.
// DELETE /api/logs
func (h *LogHandler) DeleteLogs(c *gin.Context) {
	filter := &repository.LogFilter{}

	if level := c.Query("level"); level != "" {
		filter.Level = level
	}
	if source := c.Query("source"); source != "" {
		filter.Source = source
	}
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}

	deleted, err := h.service.Delete(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("failed to delete logs", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete logs"})
		return
	}

	h.logger.Info("logs deleted", logger.F("count", deleted))
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// Cleanup deletes logs older than retention period.
// POST /api/logs/cleanup
func (h *LogHandler) Cleanup(c *gin.Context) {
	days, err := parseCleanupRetentionDays(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deleted, err := h.service.Cleanup(c.Request.Context(), days)
	if err != nil {
		h.logger.Error("failed to cleanup logs", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup logs"})
		return
	}

	h.logger.Info("logs cleaned up",
		logger.F("deleted", deleted),
		logger.F("retention_days", days),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":        "Cleanup completed",
		"deleted_count":  deleted,
		"retention_days": days,
		"deleted":        deleted,
		"days":           days,
	})
}

// ExportLogs exports logs in JSON or CSV format.
// GET /api/logs/export
func (h *LogHandler) ExportLogs(c *gin.Context) {
	filter := &repository.LogFilter{}

	if level := c.Query("level"); level != "" {
		filter.Level = level
	}
	if source := c.Query("source"); source != "" {
		filter.Source = source
	}
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}

	format := c.DefaultQuery("format", "json")
	limit := 10000

	logs, _, err := h.service.Query(c.Request.Context(), filter, limit, 0)
	if err != nil {
		h.logger.Error("failed to export logs", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to export logs"})
		return
	}

	switch format {
	case "json":
		c.Header("Content-Disposition", "attachment; filename=logs.json")
		c.JSON(http.StatusOK, logs)
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=logs.csv")

		writer := csv.NewWriter(c.Writer)
		// Write header
		header := []string{"ID", "Level", "Message", "Source", "UserID", "IP", "UserAgent", "RequestID", "Fields", "CreatedAt"}
		if err := writer.Write(header); err != nil {
			h.logger.Error("failed to write CSV header", logger.F("error", err))
			return
		}

		// Write data rows
		for _, log := range logs {
			userID := ""
			if log.UserID != nil {
				userID = fmt.Sprintf("%d", *log.UserID)
			}
			row := []string{
				fmt.Sprintf("%d", log.ID),
				log.Level,
				log.Message,
				log.Source,
				userID,
				log.IP,
				log.UserAgent,
				log.RequestID,
				log.Fields,
				log.CreatedAt.Format(time.RFC3339),
			}
			if err := writer.Write(row); err != nil {
				h.logger.Error("failed to write CSV row", logger.F("error", err))
				return
			}
		}
		writer.Flush()
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported format. Use 'json' or 'csv'"})
	}
}

func parseCleanupRetentionDays(c *gin.Context) (int, error) {
	const defaultRetentionDays = 30

	var req cleanupLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		return 0, fmt.Errorf("invalid cleanup request")
	}

	if req.RetentionDays != nil {
		return validateCleanupRetentionDays(*req.RetentionDays)
	}
	if req.Days != nil {
		return validateCleanupRetentionDays(*req.Days)
	}

	if value := strings.TrimSpace(c.Query("retention_days")); value != "" {
		return parseCleanupRetentionDaysValue(value)
	}
	if value := strings.TrimSpace(c.Query("days")); value != "" {
		return parseCleanupRetentionDaysValue(value)
	}

	return defaultRetentionDays, nil
}

func parseCleanupRetentionDaysValue(value string) (int, error) {
	days, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("retention_days must be a positive integer")
	}

	return validateCleanupRetentionDays(days)
}

func validateCleanupRetentionDays(days int) (int, error) {
	if days < 1 {
		return 0, fmt.Errorf("retention_days must be greater than 0")
	}

	return days, nil
}
