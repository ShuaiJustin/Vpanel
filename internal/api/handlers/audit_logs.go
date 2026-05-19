// Package handlers — audit_logs.go exposes a read-only admin endpoint for
// querying the audit_logs table. The companion writer is AuditService
// invoked from auth.go / settings.go on each auditable operation.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
)

// AuditLogHandler serves the admin "操作日志" list view.
type AuditLogHandler struct {
	repo   repository.AuditLogRepository
	logger logger.Logger
}

// NewAuditLogHandler constructs the audit log handler.
func NewAuditLogHandler(repo repository.AuditLogRepository, log logger.Logger) *AuditLogHandler {
	return &AuditLogHandler{repo: repo, logger: log}
}

// List returns paginated audit log entries. Query params:
//
//	page, page_size — pagination (default 1, 50; page_size capped at 200)
//
// The endpoint is intentionally minimal: filtering by action / user / date
// range can be added later if the table grows enough to need it. For now the
// admin's browser-side search-on-page is plenty.
func (h *AuditLogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	logs, err := h.repo.List(c.Request.Context(), pageSize, offset)
	if err != nil {
		h.logger.Error("failed to list audit logs", logger.F("error", err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list audit logs"})
		return
	}

	total, err := h.repo.Count(c.Request.Context())
	if err != nil {
		h.logger.Error("failed to count audit logs", logger.F("error", err))
		// Continue with logs but unknown total — better to show records than 500.
		total = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
