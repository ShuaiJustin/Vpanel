package handlers

import (
	"context"
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

type logFilterRequest struct {
	Level     string `json:"level"`
	MinLevel  string `json:"min_level"`
	Source    string `json:"source"`
	UserID    *int64 `json:"user_id"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Keyword   string `json:"keyword"`
	RequestID string `json:"request_id"`
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
	filter := buildLogFilter(c, nil)

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
	var req logFilterRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid log filter"})
		return
	}

	filter := buildLogFilter(c, &req)

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
	filter := buildLogFilter(c, nil)

	format := c.DefaultQuery("format", "json")
	logs, err := h.loadLogsForExport(c.Request.Context(), filter)
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

func buildLogFilter(c *gin.Context, req *logFilterRequest) *repository.LogFilter {
	filter := &repository.LogFilter{}

	if level := resolveFilterString(c.Query("level"), reqValue(req, func(v *logFilterRequest) string { return v.Level })); level != "" {
		filter.Level = level
	}
	if minLevel := resolveFilterString(c.Query("min_level"), reqValue(req, func(v *logFilterRequest) string { return v.MinLevel })); minLevel != "" {
		filter.MinLevel = minLevel
	}
	if source := resolveFilterString(c.Query("source"), reqValue(req, func(v *logFilterRequest) string { return v.Source })); source != "" {
		filter.Source = source
	}
	if keyword := resolveFilterString(c.Query("keyword"), reqValue(req, func(v *logFilterRequest) string { return v.Keyword })); keyword != "" {
		filter.Keyword = keyword
	}
	if requestID := resolveFilterString(c.Query("request_id"), reqValue(req, func(v *logFilterRequest) string { return v.RequestID })); requestID != "" {
		filter.RequestID = requestID
	}
	if userID := reqUserID(req); userID != nil {
		filter.UserID = userID
	} else if userID := strings.TrimSpace(c.Query("user_id")); userID != "" {
		if id, err := strconv.ParseInt(userID, 10, 64); err == nil {
			filter.UserID = &id
		}
	}
	if startTime := resolveFilterString(c.Query("start_time"), reqValue(req, func(v *logFilterRequest) string { return v.StartTime })); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := resolveFilterString(c.Query("end_time"), reqValue(req, func(v *logFilterRequest) string { return v.EndTime })); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}

	return filter
}

func resolveFilterString(queryValue, requestValue string) string {
	if value := strings.TrimSpace(requestValue); value != "" {
		return value
	}
	return strings.TrimSpace(queryValue)
}

func reqValue(req *logFilterRequest, getter func(*logFilterRequest) string) string {
	if req == nil {
		return ""
	}
	return getter(req)
}

func reqUserID(req *logFilterRequest) *int64 {
	if req == nil || req.UserID == nil {
		return nil
	}
	return req.UserID
}

func (h *LogHandler) loadLogsForExport(ctx context.Context, filter *repository.LogFilter) ([]*repository.Log, error) {
	const exportBatchSize = 2000

	logs, total, err := h.service.Query(ctx, filter, exportBatchSize, 0)
	if err != nil {
		return nil, err
	}

	if total <= int64(len(logs)) {
		return logs, nil
	}

	allLogs := make([]*repository.Log, 0, total)
	allLogs = append(allLogs, logs...)

	for offset := len(allLogs); int64(offset) < total; {
		batch, _, err := h.service.Query(ctx, filter, exportBatchSize, offset)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		allLogs = append(allLogs, batch...)
		offset += len(batch)
	}

	return allLogs, nil
}
