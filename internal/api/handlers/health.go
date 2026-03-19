// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/xray"
)

// HealthHandler handles health check requests.
type HealthHandler struct {
	repos     *repository.Repositories
	logger    logger.Logger
	xrayMgr   xray.Manager
	diskPath  string
	minDiskGB float64
}

// HealthHandlerConfig holds configuration for the health handler.
type HealthHandlerConfig struct {
	DiskPath  string  // Path to check disk space (default: "/")
	MinDiskGB float64 // Minimum free disk space in GB (default: 1.0)
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(repos *repository.Repositories, log logger.Logger, xrayMgr xray.Manager, cfg *HealthHandlerConfig) *HealthHandler {
	diskPath := "/"
	minDiskGB := 1.0

	if cfg != nil {
		if cfg.DiskPath != "" {
			diskPath = cfg.DiskPath
		}
		if cfg.MinDiskGB > 0 {
			minDiskGB = cfg.MinDiskGB
		}
	}

	return &HealthHandler{
		repos:     repos,
		logger:    log,
		xrayMgr:   xrayMgr,
		diskPath:  diskPath,
		minDiskGB: minDiskGB,
	}
}

// HealthResponse represents a health check response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

// Health returns a simple health check response.
// This endpoint is used by load balancers and monitoring systems.
func (h *HealthHandler) Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// ReadyResponse represents a readiness check response.
type ReadyResponse struct {
	Status    string           `json:"status"`
	Timestamp string           `json:"timestamp"`
	Checks    map[string]Check `json:"checks"`
}

// Check represents a single health check result.
type Check struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// Ready returns a detailed readiness check response.
// This endpoint checks all dependencies and returns their status.
func (h *HealthHandler) Ready(c *gin.Context) {
	checks := make(map[string]Check)
	allHealthy := true

	// Check database connection
	dbCheck := h.checkDatabase()
	checks["database"] = dbCheck
	if dbCheck.Status != "ok" {
		allHealthy = false
	}

	// Check Xray process status
	xrayCheck := h.checkXray()
	checks["xray"] = xrayCheck
	if xrayCheck.Status != "ok" {
		allHealthy = false
	}

	// Check disk space
	diskCheck := h.checkDiskSpace()
	checks["disk"] = diskCheck
	if diskCheck.Status != "ok" {
		allHealthy = false
	}

	status := "ok"
	httpStatus := http.StatusOK
	if !allHealthy {
		status = "degraded"
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, ReadyResponse{
		Status:    status,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

// checkDatabase checks the database connection.
func (h *HealthHandler) checkDatabase() Check {
	start := time.Now()

	// Try to perform a simple query
	if h.repos == nil || h.repos.User == nil {
		return Check{
			Status:  "error",
			Message: "database not initialized",
		}
	}

	// Try to count users as a simple health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := h.repos.User.List(ctx, 1, 0)
	latency := time.Since(start)

	if err != nil {
		h.logger.Warn("database health check failed", logger.F("error", err))
		return Check{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	return Check{
		Status:  "ok",
		Latency: latency.String(),
	}
}

// checkXray checks the Xray process status.
func (h *HealthHandler) checkXray() Check {
	start := time.Now()

	if h.xrayMgr == nil {
		return Check{
			Status:  "ok",
			Message: "xray manager not configured",
			Latency: time.Since(start).String(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := h.xrayMgr.GetStatus(ctx)
	latency := time.Since(start)

	if err != nil {
		h.logger.Warn("xray health check failed", logger.F("error", err))
		return Check{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	if status == nil || !status.Running {
		return Check{
			Status:  "error",
			Message: "xray process not running",
			Latency: latency.String(),
		}
	}

	return Check{
		Status:  "ok",
		Message: "xray process running",
		Latency: latency.String(),
	}
}

// checkDiskSpace checks available disk space.
func (h *HealthHandler) checkDiskSpace() Check {
	start := time.Now()

	var stat syscall.Statfs_t
	err := syscall.Statfs(h.diskPath, &stat)
	latency := time.Since(start)

	if err != nil {
		h.logger.Warn("disk space check failed", logger.F("error", err), logger.F("path", h.diskPath))
		return Check{
			Status:  "error",
			Message: err.Error(),
			Latency: latency.String(),
		}
	}

	// Calculate free space in GB
	freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)

	if freeGB < h.minDiskGB {
		h.logger.Warn("low disk space",
			logger.F("free_gb", freeGB),
			logger.F("min_gb", h.minDiskGB),
			logger.F("path", h.diskPath))
		return Check{
			Status:  "warning",
			Message: "low disk space: " + formatGB(freeGB) + " free",
			Latency: latency.String(),
		}
	}

	return Check{
		Status:  "ok",
		Message: formatGB(freeGB) + " free",
		Latency: latency.String(),
	}
}

// formatGB formats a float64 as a GB string.
func formatGB(gb float64) string {
	return fmt.Sprintf("%.2f GB", gb)
}
