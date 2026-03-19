// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

// NodeHealthHandler handles node health check API requests.
type NodeHealthHandler struct {
	healthChecker   *node.HealthChecker
	healthCheckRepo repository.HealthCheckRepository
	nodeRepo        repository.NodeRepository
	logger          logger.Logger
}

// NewNodeHealthHandler creates a new node health handler.
func NewNodeHealthHandler(
	healthChecker *node.HealthChecker,
	healthCheckRepo repository.HealthCheckRepository,
	nodeRepo repository.NodeRepository,
	log logger.Logger,
) *NodeHealthHandler {
	return &NodeHealthHandler{
		healthChecker:   healthChecker,
		healthCheckRepo: healthCheckRepo,
		nodeRepo:        nodeRepo,
		logger:          log,
	}
}

// HealthCheckResponse represents a health check result in API responses.
type HealthCheckResponse struct {
	ID        int64  `json:"id,omitempty"`
	NodeID    int64  `json:"node_id"`
	Status    string `json:"status"`
	TCPOk     bool   `json:"tcp_ok"`
	APIOk     bool   `json:"api_ok"`
	XrayOk    bool   `json:"xray_ok"`
	Latency   int    `json:"latency"`
	Message   string `json:"message"`
	CheckedAt string `json:"checked_at"`
}

// HealthCheckerStatusResponse represents the health checker status.
type HealthCheckerStatusResponse struct {
	Running            bool   `json:"running"`
	Interval           string `json:"interval"`
	UnhealthyThreshold int    `json:"unhealthy_threshold"`
	HealthyThreshold   int    `json:"healthy_threshold"`
}

// UpdateHealthConfigRequest represents a request to update health check config.
type UpdateHealthConfigRequest struct {
	Interval           *string `json:"interval"`
	UnhealthyThreshold *int    `json:"unhealthy_threshold"`
	HealthyThreshold   *int    `json:"healthy_threshold"`
}

// toHealthCheckResponse converts a health check result to API response format.
func toHealthCheckResponse(r *node.HealthCheckResult) *HealthCheckResponse {
	return &HealthCheckResponse{
		NodeID:    r.NodeID,
		Status:    r.Status,
		TCPOk:     r.TCPOk,
		APIOk:     r.APIOk,
		XrayOk:    r.XrayOk,
		Latency:   r.Latency,
		Message:   r.Message,
		CheckedAt: r.CheckedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// toHealthCheckResponseFromRepo converts a repository health check to API response format.
func toHealthCheckResponseFromRepo(hc *repository.HealthCheck) *HealthCheckResponse {
	return &HealthCheckResponse{
		ID:        hc.ID,
		NodeID:    hc.NodeID,
		Status:    hc.Status,
		TCPOk:     hc.TCPOk,
		APIOk:     hc.APIOk,
		XrayOk:    hc.XrayOk,
		Latency:   hc.Latency,
		Message:   hc.Message,
		CheckedAt: hc.CheckedAt.Format("2006-01-02T15:04:05Z"),
	}
}

// CheckNode performs a health check on a specific node.
// POST /api/admin/nodes/:id/health-check
func (h *NodeHealthHandler) CheckNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	result, err := h.healthChecker.CheckNode(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to check node health", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check node health"})
		return
	}

	c.JSON(http.StatusOK, toHealthCheckResponse(result))
}

// CheckAll performs health checks on all nodes.
// POST /api/admin/nodes/health-check
func (h *NodeHealthHandler) CheckAll(c *gin.Context) {
	results, err := h.healthChecker.CheckAll(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to check all nodes health", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check nodes health"})
		return
	}

	response := make([]*HealthCheckResponse, len(results))
	for i, r := range results {
		response[i] = toHealthCheckResponse(r)
	}

	c.JSON(http.StatusOK, gin.H{"results": response})
}

// GetHistory returns health check history for a node.
// GET /api/admin/nodes/:id/health-history
func (h *NodeHealthHandler) GetHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	history, err := h.healthChecker.GetHistory(c.Request.Context(), id, limit)
	if err != nil {
		h.logger.Error("Failed to get health check history", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health check history"})
		return
	}

	response := make([]*HealthCheckResponse, len(history))
	for i, hc := range history {
		response[i] = toHealthCheckResponseFromRepo(hc)
	}

	c.JSON(http.StatusOK, gin.H{"history": response})
}

// GetLatest returns the latest health check for a node.
// GET /api/admin/nodes/:id/health-latest
func (h *NodeHealthHandler) GetLatest(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	latest, err := h.healthCheckRepo.GetLatestByNodeID(c.Request.Context(), id)
	if err != nil {
		h.logger.Error("Failed to get latest health check", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get latest health check"})
		return
	}

	if latest == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No health checks found for this node"})
		return
	}

	c.JSON(http.StatusOK, toHealthCheckResponseFromRepo(latest))
}

// GetHealthStats returns health statistics for a node.
// GET /api/admin/nodes/:id/health-stats
func (h *NodeHealthHandler) GetHealthStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	// Default to last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	if sinceStr := c.Query("since"); sinceStr != "" {
		if parsed, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = parsed
		}
	}

	// Get success and failure counts
	successCount, err := h.healthCheckRepo.CountByStatus(c.Request.Context(), id, repository.HealthCheckStatusSuccess, since)
	if err != nil {
		h.logger.Error("Failed to count successes", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health stats"})
		return
	}

	failureCount, err := h.healthCheckRepo.CountByStatus(c.Request.Context(), id, repository.HealthCheckStatusFailed, since)
	if err != nil {
		h.logger.Error("Failed to count failures", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health stats"})
		return
	}

	// Get average latency
	avgLatency, err := h.healthCheckRepo.GetAverageLatency(c.Request.Context(), id, since)
	if err != nil {
		h.logger.Error("Failed to get average latency", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health stats"})
		return
	}

	// Get consecutive failures/successes
	consecutiveFailures := h.healthChecker.GetConsecutiveFailures(id)
	consecutiveSuccesses := h.healthChecker.GetConsecutiveSuccesses(id)

	total := successCount + failureCount
	var successRate float64
	if total > 0 {
		successRate = float64(successCount) / float64(total) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id":               id,
		"since":                 since.Format("2006-01-02T15:04:05Z"),
		"total_checks":          total,
		"success_count":         successCount,
		"failure_count":         failureCount,
		"success_rate":          successRate,
		"average_latency_ms":    avgLatency,
		"consecutive_failures":  consecutiveFailures,
		"consecutive_successes": consecutiveSuccesses,
	})
}

// GetCheckerStatus returns the health checker status.
// GET /api/admin/health-checker/status
func (h *NodeHealthHandler) GetCheckerStatus(c *gin.Context) {
	config := h.healthChecker.GetConfig()

	c.JSON(http.StatusOK, &HealthCheckerStatusResponse{
		Running:            h.healthChecker.IsRunning(),
		Interval:           config.Interval.String(),
		UnhealthyThreshold: config.UnhealthyThreshold,
		HealthyThreshold:   config.HealthyThreshold,
	})
}

// StartChecker starts the health checker.
// POST /api/admin/health-checker/start
func (h *NodeHealthHandler) StartChecker(c *gin.Context) {
	if h.healthChecker.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"message": "Health checker is already running"})
		return
	}

	if err := h.healthChecker.Start(c.Request.Context()); err != nil {
		h.logger.Error("Failed to start health checker", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start health checker"})
		return
	}

	h.logger.Info("Health checker started")
	c.JSON(http.StatusOK, gin.H{"message": "Health checker started"})
}

// StopChecker stops the health checker.
// POST /api/admin/health-checker/stop
func (h *NodeHealthHandler) StopChecker(c *gin.Context) {
	if !h.healthChecker.IsRunning() {
		c.JSON(http.StatusOK, gin.H{"message": "Health checker is not running"})
		return
	}

	if err := h.healthChecker.Stop(c.Request.Context()); err != nil {
		h.logger.Error("Failed to stop health checker", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stop health checker"})
		return
	}

	h.logger.Info("Health checker stopped")
	c.JSON(http.StatusOK, gin.H{"message": "Health checker stopped"})
}

// UpdateCheckerConfig updates the health checker configuration.
// PUT /api/admin/health-checker/config
func (h *NodeHealthHandler) UpdateCheckerConfig(c *gin.Context) {
	var req UpdateHealthConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	config := h.healthChecker.GetConfig()

	if req.Interval != nil {
		duration, err := time.ParseDuration(*req.Interval)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid interval format"})
			return
		}
		if duration < time.Second {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Interval must be at least 1 second"})
			return
		}
		config.Interval = duration
	}

	if req.UnhealthyThreshold != nil {
		if *req.UnhealthyThreshold < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unhealthy threshold must be at least 1"})
			return
		}
		config.UnhealthyThreshold = *req.UnhealthyThreshold
	}

	if req.HealthyThreshold != nil {
		if *req.HealthyThreshold < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Healthy threshold must be at least 1"})
			return
		}
		config.HealthyThreshold = *req.HealthyThreshold
	}

	h.healthChecker.UpdateConfig(config)

	h.logger.Info("Health checker config updated",
		logger.F("interval", config.Interval.String()),
		logger.F("unhealthy_threshold", config.UnhealthyThreshold),
		logger.F("healthy_threshold", config.HealthyThreshold))

	c.JSON(http.StatusOK, &HealthCheckerStatusResponse{
		Running:            h.healthChecker.IsRunning(),
		Interval:           config.Interval.String(),
		UnhealthyThreshold: config.UnhealthyThreshold,
		HealthyThreshold:   config.HealthyThreshold,
	})
}

// GetClusterHealth returns overall cluster health summary.
// GET /api/admin/nodes/cluster-health
func (h *NodeHealthHandler) GetClusterHealth(c *gin.Context) {
	// Get node counts by status
	statusCounts, err := h.nodeRepo.CountByStatus(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get node status counts", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get cluster health"})
		return
	}

	totalNodes := int64(0)
	for _, count := range statusCounts {
		totalNodes += count
	}

	onlineNodes := statusCounts[repository.NodeStatusOnline]
	offlineNodes := statusCounts[repository.NodeStatusOffline]
	unhealthyNodes := statusCounts[repository.NodeStatusUnhealthy]

	// Calculate health percentage
	var healthPercentage float64
	if totalNodes > 0 {
		healthPercentage = float64(onlineNodes) / float64(totalNodes) * 100
	}

	// Determine overall status
	var overallStatus string
	switch {
	case totalNodes == 0:
		overallStatus = "no_nodes"
	case onlineNodes == totalNodes:
		overallStatus = "healthy"
	case onlineNodes == 0:
		overallStatus = "critical"
	case unhealthyNodes > 0:
		overallStatus = "degraded"
	default:
		overallStatus = "partial"
	}

	c.JSON(http.StatusOK, gin.H{
		"overall_status":    overallStatus,
		"health_percentage": healthPercentage,
		"total_nodes":       totalNodes,
		"online_nodes":      onlineNodes,
		"offline_nodes":     offlineNodes,
		"unhealthy_nodes":   unhealthyNodes,
		"status_breakdown":  statusCounts,
	})
}
