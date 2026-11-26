package handlers

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"v/internal/logger"
)

const (
	maxErrorReportBatchSize  = 20
	maxErrorReportPerIPMin   = 20
	errorReportRateWindow    = time.Minute
)

// ErrorReportHandler handles error reporting from frontend.
type ErrorReportHandler struct {
	logger   logger.Logger
	mu       sync.Mutex
	attempts map[string][]time.Time
}

// NewErrorReportHandler creates a new ErrorReportHandler.
func NewErrorReportHandler(log logger.Logger) *ErrorReportHandler {
	return &ErrorReportHandler{
		logger:   log,
		attempts: make(map[string][]time.Time),
	}
}

func (h *ErrorReportHandler) allowClient(ip string) bool {
	now := time.Now()
	cutoff := now.Add(-errorReportRateWindow)

	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.attempts) > 4096 {
		for k, times := range h.attempts {
			if len(times) == 0 || times[len(times)-1].Before(cutoff) {
				delete(h.attempts, k)
			}
		}
	}

	recent := h.attempts[ip]
	kept := recent[:0]
	for _, ts := range recent {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= maxErrorReportPerIPMin {
		h.attempts[ip] = kept
		return false
	}
	h.attempts[ip] = append(kept, now)
	return true
}

// ReportErrors handles frontend error reports.
// POST /api/errors/report
func (h *ErrorReportHandler) ReportErrors(c *gin.Context) {
	if !h.allowClient(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": "too many error reports",
		})
		return
	}

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

	// Log frontend errors (bounded batch size to prevent log flooding).
	items := req.Errors
	if len(items) > maxErrorReportBatchSize {
		items = items[:maxErrorReportBatchSize]
	}
	for _, errData := range items {
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
