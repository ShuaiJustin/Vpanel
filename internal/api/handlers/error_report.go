package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"v/internal/logger"
)

// ErrorReportHandler handles error reporting from frontend.
type ErrorReportHandler struct {
	logger logger.Logger
}

// NewErrorReportHandler creates a new ErrorReportHandler.
func NewErrorReportHandler(log logger.Logger) *ErrorReportHandler {
	return &ErrorReportHandler{
		logger: log,
	}
}

// ReportErrors handles frontend error reports.
// POST /api/errors/report
func (h *ErrorReportHandler) ReportErrors(c *gin.Context) {
	var req struct {
		Errors     []map[string]interface{} `json:"errors"`
		BatchID    string                   `json:"batchId"`
		ReportedAt string                   `json:"reportedAt"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "invalid request",
			"error":   err.Error(),
		})
		return
	}

	// Log frontend errors
	for _, errData := range req.Errors {
		h.logger.Error("frontend error reported",
			logger.F("batch_id", req.BatchID),
			logger.F("error_id", errData["errorId"]),
			logger.F("message", errData["message"]),
			logger.F("url", errData["url"]),
			logger.F("component", errData["component"]),
			logger.F("user_agent", errData["userAgent"]),
		)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "errors reported successfully",
	})
}
