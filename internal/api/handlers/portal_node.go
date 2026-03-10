// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/logger"
	"v/internal/portal/node"
	"v/pkg/errors"
)

// PortalNodeHandler handles portal node requests.
type PortalNodeHandler struct {
	nodeService *node.Service
	logger      logger.Logger
}

// NewPortalNodeHandler creates a new PortalNodeHandler.
func NewPortalNodeHandler(nodeService *node.Service, log logger.Logger) *PortalNodeHandler {
	return &PortalNodeHandler{
		nodeService: nodeService,
		logger:      log,
	}
}

// ListNodes returns available nodes for the current user.
func (h *PortalNodeHandler) ListNodes(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		middleware.HandleUnauthorized(c, errors.MsgUnauthorized)
		return
	}

	// Parse filter parameters
	filter := &node.NodeFilter{
		Region:   c.Query("region"),
		Protocol: c.Query("protocol"),
	}

	// Get nodes
	nodes, err := h.nodeService.ListNodes(c.Request.Context(), userID.(int64), filter)
	if err != nil {
		h.logger.Error("failed to list nodes", logger.F("error", err))
		middleware.HandleInternalError(c, "获取节点列表失败", err)
		return
	}

	// Apply sorting if specified
	sortField := c.Query("sort")
	sortOrder := c.Query("order")
	if sortField != "" {
		sortOpt := &node.SortOption{
			Field: sortField,
			Order: sortOrder,
		}
		nodes = node.SortNodes(nodes, sortOpt)
	}

	// Get available regions and protocols for filtering
	regions := node.GetAvailableRegions(nodes)
	protocols := node.GetAvailableProtocols(nodes)

	c.JSON(http.StatusOK, gin.H{
		"nodes":     nodes,
		"total":     len(nodes),
		"regions":   regions,
		"protocols": protocols,
	})
}

// GetNode returns a single node by ID.
func (h *PortalNodeHandler) GetNode(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.HandleBadRequest(c, "无效的节点ID")
		return
	}

	nodeInfo, err := h.nodeService.GetNode(c.Request.Context(), id)
	if err != nil {
		middleware.HandleNotFound(c, "node", id)
		return
	}

	c.JSON(http.StatusOK, nodeInfo)
}

// TestLatencyRequest represents a latency test request.
type TestLatencyRequest struct {
	NodeIDs []int64 `json:"node_ids"`
}

// TestLatency tests TCP latency to a node host:port.
func (h *PortalNodeHandler) TestLatency(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的节点ID"})
		return
	}

	// Get node to verify it exists
	nodeInfo, err := h.nodeService.GetNode(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "节点不存在"})
		return
	}

	if nodeInfo.Host == "" || nodeInfo.Port <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"node_id": nodeInfo.ID,
			"host":    nodeInfo.Host,
			"latency": -1,
			"message": "节点地址无效",
		})
		return
	}

	target := fmt.Sprintf("%s:%d", nodeInfo.Host, nodeInfo.Port)
	start := time.Now()
	conn, dialErr := net.DialTimeout("tcp", target, 3*time.Second)
	if dialErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"node_id": nodeInfo.ID,
			"host":    nodeInfo.Host,
			"latency": -1,
			"message": "连接失败",
		})
		return
	}
	_ = conn.Close()
	latency := int(time.Since(start).Milliseconds())
	if latency < 1 {
		latency = 1
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id": nodeInfo.ID,
		"host":    nodeInfo.Host,
		"latency": latency,
		"message": "ok",
	})
}
