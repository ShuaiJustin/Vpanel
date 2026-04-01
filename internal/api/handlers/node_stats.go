// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/node"
)

// NodeStatsHandler handles node traffic statistics API requests.
type NodeStatsHandler struct {
	trafficService *node.TrafficService
	nodeService    *node.Service
	groupService   *node.GroupService
	logger         logger.Logger
}

// NewNodeStatsHandler creates a new node stats handler.
func NewNodeStatsHandler(
	trafficService *node.TrafficService,
	nodeService *node.Service,
	groupService *node.GroupService,
	log logger.Logger,
) *NodeStatsHandler {
	return &NodeStatsHandler{
		trafficService: trafficService,
		nodeService:    nodeService,
		groupService:   groupService,
		logger:         log,
	}
}

// TrafficStatsResponse represents traffic statistics in API responses.
type TrafficStatsResponse struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// NodeTrafficStatsResponse represents traffic statistics for a node.
type NodeTrafficStatsResponse struct {
	NodeID   int64 `json:"node_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// UserTrafficStatsResponse represents traffic statistics for a user.
type UserTrafficStatsResponse struct {
	UserID   int64 `json:"user_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// GroupTrafficStatsResponse represents traffic statistics for a group.
type GroupTrafficStatsResponse struct {
	GroupID  int64 `json:"group_id"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
	Total    int64 `json:"total"`
}

// RecordTrafficRequest represents a request to record traffic.
type RecordTrafficRequest struct {
	NodeID   int64  `json:"node_id" binding:"required"`
	UserID   int64  `json:"user_id" binding:"required"`
	ProxyID  *int64 `json:"proxy_id"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// RecordTrafficBatchRequest represents a request to record multiple traffic entries.
type RecordTrafficBatchRequest struct {
	Records []RecordTrafficRequest `json:"records" binding:"required"`
}

// parseTimeRange parses start and end time from query parameters.
func parseTimeRange(c *gin.Context) (time.Time, time.Time) {
	// Default to last 24 hours
	end := time.Now()
	start := end.Add(-24 * time.Hour)

	if startStr := c.Query("start"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			start = parsed
		}
	}

	if endStr := c.Query("end"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			end = parsed
		}
	}

	return start, end
}

// GetTotalTraffic returns total traffic across all nodes.
// GET /api/admin/nodes/traffic/total
func (h *NodeStatsHandler) GetTotalTraffic(c *gin.Context) {
	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTotalTraffic(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get total traffic", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total traffic"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": &TrafficStatsResponse{
			Upload:   stats.Upload,
			Download: stats.Download,
			Total:    stats.Total,
		},
	})
}

// GetTrafficByNode returns traffic statistics for a specific node.
// GET /api/admin/nodes/:id/traffic
func (h *NodeStatsHandler) GetTrafficByNode(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTrafficByNode(c.Request.Context(), id, start, end)
	if err != nil {
		h.logger.Error("Failed to get traffic by node", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": &NodeTrafficStatsResponse{
			NodeID:   stats.NodeID,
			Upload:   stats.Upload,
			Download: stats.Download,
			Total:    stats.Total,
		},
	})
}

// GetTrafficByUser returns traffic statistics for a specific user.
// GET /api/admin/users/:id/node-traffic
func (h *NodeStatsHandler) GetTrafficByUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTrafficByUser(c.Request.Context(), id, start, end)
	if err != nil {
		h.logger.Error("Failed to get traffic by user", logger.Err(err), logger.F("user_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": &UserTrafficStatsResponse{
			UserID:   stats.UserID,
			Upload:   stats.Upload,
			Download: stats.Download,
			Total:    stats.Total,
		},
	})
}

// GetUserTrafficBreakdown returns traffic breakdown by node for a user.
// GET /api/admin/users/:id/node-traffic/breakdown
func (h *NodeStatsHandler) GetUserTrafficBreakdown(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	start, end := parseTimeRange(c)

	breakdown, err := h.trafficService.GetUserTrafficBreakdownByNode(c.Request.Context(), id, start, end)
	if err != nil {
		h.logger.Error("Failed to get user traffic breakdown", logger.Err(err), logger.F("user_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic breakdown"})
		return
	}

	response := make([]gin.H, len(breakdown))
	for i, b := range breakdown {
		response[i] = gin.H{
			"user_id":  b.UserID,
			"node_id":  b.NodeID,
			"upload":   b.Upload,
			"download": b.Download,
			"total":    b.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"start":     start.Format("2006-01-02T15:04:05Z"),
		"end":       end.Format("2006-01-02T15:04:05Z"),
		"breakdown": response,
	})
}

// GetTrafficByGroup returns traffic statistics for a specific group.
// GET /api/admin/node-groups/:id/traffic
func (h *NodeStatsHandler) GetTrafficByGroup(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group ID"})
		return
	}

	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTrafficByGroup(c.Request.Context(), id, start, end)
	if err != nil {
		h.logger.Error("Failed to get traffic by group", logger.Err(err), logger.F("group_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": &GroupTrafficStatsResponse{
			GroupID:  stats.GroupID,
			Upload:   stats.Upload,
			Download: stats.Download,
			Total:    stats.Total,
		},
	})
}

// GetTrafficStatsByNode returns traffic statistics grouped by node.
// GET /api/admin/nodes/traffic/by-node
func (h *NodeStatsHandler) GetTrafficStatsByNode(c *gin.Context) {
	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTrafficStatsByNode(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get traffic stats by node", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic stats"})
		return
	}

	response := make([]*NodeTrafficStatsResponse, len(stats))
	for i, s := range stats {
		response[i] = &NodeTrafficStatsResponse{
			NodeID:   s.NodeID,
			Upload:   s.Upload,
			Download: s.Download,
			Total:    s.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": response,
	})
}

// GetTrafficStatsByGroup returns traffic statistics grouped by node group.
// GET /api/admin/nodes/traffic/by-group
func (h *NodeStatsHandler) GetTrafficStatsByGroup(c *gin.Context) {
	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTrafficStatsByGroup(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get traffic stats by group", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get traffic stats"})
		return
	}

	response := make([]*GroupTrafficStatsResponse, len(stats))
	for i, s := range stats {
		response[i] = &GroupTrafficStatsResponse{
			GroupID:  s.GroupID,
			Upload:   s.Upload,
			Download: s.Download,
			Total:    s.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"start": start.Format("2006-01-02T15:04:05Z"),
		"end":   end.Format("2006-01-02T15:04:05Z"),
		"stats": response,
	})
}

// GetTopUsersByTraffic returns top users by traffic on a specific node.
// GET /api/admin/nodes/:id/traffic/top-users
func (h *NodeStatsHandler) GetTopUsersByTraffic(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetTopUsersByTraffic(c.Request.Context(), id, start, end, limit)
	if err != nil {
		h.logger.Error("Failed to get top users by traffic", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get top users"})
		return
	}

	response := make([]gin.H, len(stats))
	for i, s := range stats {
		response[i] = gin.H{
			"user_id":  s.UserID,
			"node_id":  s.NodeID,
			"upload":   s.Upload,
			"download": s.Download,
			"total":    s.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"start":     start.Format("2006-01-02T15:04:05Z"),
		"end":       end.Format("2006-01-02T15:04:05Z"),
		"top_users": response,
	})
}

// GetAggregatedStats returns comprehensive aggregated traffic statistics.
// GET /api/admin/nodes/traffic/aggregated
func (h *NodeStatsHandler) GetAggregatedStats(c *gin.Context) {
	start, end := parseTimeRange(c)

	stats, err := h.trafficService.GetAggregatedStats(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get aggregated stats", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get aggregated stats"})
		return
	}

	// Convert node stats
	nodeStats := make([]*NodeTrafficStatsResponse, len(stats.ByNode))
	for i, s := range stats.ByNode {
		nodeStats[i] = &NodeTrafficStatsResponse{
			NodeID:   s.NodeID,
			Upload:   s.Upload,
			Download: s.Download,
			Total:    s.Total,
		}
	}

	// Convert group stats
	groupStats := make([]*GroupTrafficStatsResponse, len(stats.ByGroup))
	for i, s := range stats.ByGroup {
		groupStats[i] = &GroupTrafficStatsResponse{
			GroupID:  s.GroupID,
			Upload:   s.Upload,
			Download: s.Download,
			Total:    s.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"start":          start.Format("2006-01-02T15:04:05Z"),
		"end":            end.Format("2006-01-02T15:04:05Z"),
		"total_upload":   stats.TotalUpload,
		"total_download": stats.TotalDownload,
		"total":          stats.Total,
		"by_node":        nodeStats,
		"by_group":       groupStats,
	})
}

// RecordTraffic records a traffic entry.
// POST /api/admin/nodes/traffic
func (h *NodeStatsHandler) RecordTraffic(c *gin.Context) {
	var req RecordTrafficRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	record := &node.TrafficRecord{
		NodeID:   req.NodeID,
		UserID:   req.UserID,
		ProxyID:  req.ProxyID,
		Upload:   req.Upload,
		Download: req.Download,
	}

	if err := h.trafficService.RecordTraffic(c.Request.Context(), record); err != nil {
		h.logger.Error("Failed to record traffic", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record traffic"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Traffic recorded successfully"})
}

// RecordTrafficBatch records multiple traffic entries.
// POST /api/admin/nodes/traffic/batch
func (h *NodeStatsHandler) RecordTrafficBatch(c *gin.Context) {
	var req RecordTrafficBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	records := make([]*node.TrafficRecord, len(req.Records))
	for i, r := range req.Records {
		records[i] = &node.TrafficRecord{
			NodeID:   r.NodeID,
			UserID:   r.UserID,
			ProxyID:  r.ProxyID,
			Upload:   r.Upload,
			Download: r.Download,
		}
	}

	if err := h.trafficService.RecordTrafficBatch(c.Request.Context(), records); err != nil {
		h.logger.Error("Failed to record traffic batch", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record traffic"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Traffic recorded successfully",
		"count":   len(records),
	})
}

// CleanupOldRecords deletes old traffic records.
// POST /api/admin/nodes/traffic/cleanup
func (h *NodeStatsHandler) CleanupOldRecords(c *gin.Context) {
	// Default retention: 30 days
	retentionDays, _ := strconv.Atoi(c.DefaultQuery("retention_days", "30"))
	retention := time.Duration(retentionDays) * 24 * time.Hour

	deleted, err := h.trafficService.CleanupOldRecords(c.Request.Context(), retention)
	if err != nil {
		h.logger.Error("Failed to cleanup old traffic records", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cleanup records"})
		return
	}

	h.logger.Info("Cleaned up old traffic records",
		logger.F("deleted", deleted),
		logger.F("retention_days", retentionDays))

	c.JSON(http.StatusOK, gin.H{
		"message":        "Cleanup completed",
		"deleted_count":  deleted,
		"retention_days": retentionDays,
	})
}

// GetRealTimeStats returns real-time traffic monitoring data.
// GET /api/admin/nodes/traffic/realtime
func (h *NodeStatsHandler) GetRealTimeStats(c *gin.Context) {
	// Get traffic from last 5 minutes for "real-time" view
	end := time.Now()
	start := end.Add(-5 * time.Minute)

	// Get total traffic
	totalStats, err := h.trafficService.GetTotalTraffic(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get real-time total traffic", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get real-time stats"})
		return
	}

	// Get traffic by node
	nodeStats, err := h.trafficService.GetTrafficStatsByNode(c.Request.Context(), start, end)
	if err != nil {
		h.logger.Error("Failed to get real-time node traffic", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get real-time stats"})
		return
	}

	// Get node statistics
	nodeStatusStats, err := h.nodeService.GetStatistics(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get node statistics", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get real-time stats"})
		return
	}

	totalUsers, err := h.nodeService.GetTotalUsers(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get total users", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get real-time stats"})
		return
	}

	// Convert node stats
	nodeTrafficResponse := make([]*NodeTrafficStatsResponse, len(nodeStats))
	for i, s := range nodeStats {
		nodeTrafficResponse[i] = &NodeTrafficStatsResponse{
			NodeID:   s.NodeID,
			Upload:   s.Upload,
			Download: s.Download,
			Total:    s.Total,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"timestamp": end.Format("2006-01-02T15:04:05Z"),
		"window":    "5m",
		"traffic": gin.H{
			"upload":   totalStats.Upload,
			"download": totalStats.Download,
			"total":    totalStats.Total,
		},
		"nodes": gin.H{
			"by_status":   nodeStatusStats,
			"total_users": totalUsers,
		},
		"traffic_by_node": nodeTrafficResponse,
	})
}
