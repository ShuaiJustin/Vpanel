// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
	"v/internal/xray"
)

// NodeConfigTestHandler handles node configuration testing.
type NodeConfigTestHandler struct {
	configGenerator *xray.ConfigGenerator
	logger          logger.Logger
}

// NewNodeConfigTestHandler creates a new node config test handler.
func NewNodeConfigTestHandler(
	configGenerator *xray.ConfigGenerator,
	log logger.Logger,
) *NodeConfigTestHandler {
	return &NodeConfigTestHandler{
		configGenerator: configGenerator,
		logger:          log,
	}
}

// PreviewConfig generates and returns the Xray config for a node (for testing/preview).
// GET /api/admin/nodes/:id/config/preview
func (h *NodeConfigTestHandler) PreviewConfig(c *gin.Context) {
	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	// Generate config
	config, err := h.configGenerator.GenerateForNode(c.Request.Context(), nodeID)
	if err != nil {
		h.logger.Error("Failed to generate config preview",
			logger.F("node_id", nodeID),
			logger.F("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate configuration: " + err.Error(),
		})
		return
	}

	// Convert to JSON
	configJSON, err := config.ToJSON()
	if err != nil {
		h.logger.Error("Failed to serialize config",
			logger.F("node_id", nodeID),
			logger.F("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to serialize configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":              true,
		"node_id":              nodeID,
		"inbound_count":        len(config.Inbounds),
		// client_count is the authoritative user count: a merged inbound
		// carries many clients, so inbound_count alone under-reports users.
		"client_count":         xray.ClientCount(config.Inbounds),
		"merged_inbound_count": xray.MergedInboundCount(config.Inbounds),
		"merge_enabled":        h.configGenerator.InboundMergingEnabled(),
		"config":               string(configJSON),
	})
}
