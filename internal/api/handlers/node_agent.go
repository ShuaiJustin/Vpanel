// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
	"v/internal/xray"
)

// NodeAgentHandler handles Node Agent API requests.
type NodeAgentHandler struct {
	nodeService     *node.Service
	nodeRepo        repository.NodeRepository
	configGenerator *xray.ConfigGenerator
	recoveryTracker *NodeRecoveryTracker
	logger          logger.Logger
}

// NewNodeAgentHandler creates a new NodeAgentHandler.
func NewNodeAgentHandler(
	nodeService *node.Service,
	nodeRepo repository.NodeRepository,
	configGenerator *xray.ConfigGenerator,
	recoveryTracker *NodeRecoveryTracker,
	log logger.Logger,
) *NodeAgentHandler {
	if recoveryTracker == nil {
		recoveryTracker = NewNodeRecoveryTracker(log)
	}
	return &NodeAgentHandler{
		nodeService:     nodeService,
		nodeRepo:        nodeRepo,
		configGenerator: configGenerator,
		recoveryTracker: recoveryTracker,
		logger:          log,
	}
}

// RegisterRequest represents a node registration request.
type RegisterRequest struct {
	Token   string `json:"token" binding:"required"`
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// RegisterResponse represents a node registration response.
type RegisterResponse struct {
	Success bool   `json:"success"`
	NodeID  int64  `json:"node_id"`
	Message string `json:"message"`
}

// HeartbeatRequest represents a node heartbeat request.
type HeartbeatRequest struct {
	NodeID  int64        `json:"node_id" binding:"required"`
	Token   string       `json:"token" binding:"required"`
	Metrics *NodeMetrics `json:"metrics"`
}

// NodeMetrics represents metrics from a node.
type NodeMetrics struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUsed  uint64  `json:"memory_used"`
	DiskUsage   float64 `json:"disk_usage"`
	NetworkIn   uint64  `json:"network_in"`
	NetworkOut  uint64  `json:"network_out"`
	Connections int     `json:"connections"`
	XrayRunning bool    `json:"xray_running"`
	XrayVersion string  `json:"xray_version"`
	Uptime      int64   `json:"uptime"`
	Timestamp   int64   `json:"timestamp"`
}

// HeartbeatResponse represents a node heartbeat response.
type HeartbeatResponse struct {
	Success  bool      `json:"success"`
	Message  string    `json:"message"`
	Commands []Command `json:"commands,omitempty"`
}

// Command represents a command to send to a node.
type Command struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// CommandResultRequest represents a command result from a node.
type CommandResultRequest struct {
	CommandID string `json:"command_id" binding:"required"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
}

type nodeSyncStatusUpdater interface {
	UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error
}

// Register handles node registration requests.
// POST /api/node/register
func (h *NodeAgentHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, RegisterResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate token
	nodeData, err := h.nodeService.ValidateToken(c.Request.Context(), req.Token)
	if err != nil {
		h.logger.Warn("Node registration failed: invalid token",
			logger.F("token_prefix", truncateToken(req.Token)),
			logger.F("error", err.Error()))
		c.JSON(http.StatusUnauthorized, RegisterResponse{
			Success: false,
			Message: "Invalid or revoked token",
		})
		return
	}

	// Update node status to online
	if err := h.nodeService.UpdateStatus(c.Request.Context(), nodeData.ID, repository.NodeStatusOnline); err != nil {
		h.logger.Error("Failed to update node status",
			logger.F("node_id", nodeData.ID),
			logger.F("error", err.Error()))
	}

	// Update last seen
	if err := h.nodeService.UpdateLastSeen(c.Request.Context(), nodeData.ID); err != nil {
		h.logger.Error("Failed to update node last seen",
			logger.F("node_id", nodeData.ID),
			logger.F("error", err.Error()))
	}

	h.logger.Info("Node registered successfully",
		logger.F("node_id", nodeData.ID),
		logger.F("node_name", nodeData.Name),
		logger.F("version", req.Version),
		logger.F("os", req.OS),
		logger.F("arch", req.Arch))

	c.JSON(http.StatusOK, RegisterResponse{
		Success: true,
		NodeID:  nodeData.ID,
		Message: "Registration successful",
	})
}

// Heartbeat handles node heartbeat requests.
// POST /api/node/heartbeat
func (h *NodeAgentHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, HeartbeatResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate token
	nodeData, err := h.nodeService.ValidateToken(c.Request.Context(), req.Token)
	if err != nil {
		h.logger.Warn("Heartbeat failed: invalid token",
			logger.F("node_id", req.NodeID),
			logger.F("error", err.Error()))
		c.JSON(http.StatusUnauthorized, HeartbeatResponse{
			Success: false,
			Message: "Invalid or revoked token",
		})
		return
	}

	// Verify node ID matches token
	if nodeData.ID != req.NodeID {
		h.logger.Warn("Heartbeat failed: node ID mismatch",
			logger.F("expected_node_id", nodeData.ID),
			logger.F("received_node_id", req.NodeID))
		c.JSON(http.StatusUnauthorized, HeartbeatResponse{
			Success: false,
			Message: "Node ID does not match token",
		})
		return
	}

	// Update node status to online
	if err := h.nodeService.UpdateStatus(c.Request.Context(), nodeData.ID, repository.NodeStatusOnline); err != nil {
		h.logger.Error("Failed to update node status",
			logger.F("node_id", nodeData.ID),
			logger.F("error", err.Error()))
	}

	// Update last seen
	if err := h.nodeService.UpdateLastSeen(c.Request.Context(), nodeData.ID); err != nil {
		h.logger.Error("Failed to update node last seen",
			logger.F("node_id", nodeData.ID),
			logger.F("error", err.Error()))
	}

	// Update metrics if provided
	if req.Metrics != nil {
		h.logger.Info("收到节点指标数据",
			logger.F("node_id", nodeData.ID),
			logger.F("connections", req.Metrics.Connections),
			logger.F("cpu_usage", req.Metrics.CPUUsage),
			logger.F("memory_usage", req.Metrics.MemoryUsage),
			logger.F("disk_usage", req.Metrics.DiskUsage),
			logger.F("xray_running", req.Metrics.XrayRunning))

		// 更新用户连接数
		metrics := &node.NodeMetrics{
			Latency:      0, // 延迟由健康检查服务计算
			CurrentUsers: req.Metrics.Connections,
		}
		if err := h.nodeService.UpdateMetrics(c.Request.Context(), nodeData.ID, metrics); err != nil {
			h.logger.Error("Failed to update node metrics",
				logger.F("node_id", nodeData.ID),
				logger.F("error", err.Error()))
		}

		// 更新负载信息（CPU、内存、磁盘）
		if err := h.nodeRepo.UpdateLoadMetrics(c.Request.Context(), nodeData.ID,
			req.Metrics.CPUUsage,
			req.Metrics.MemoryUsage,
			req.Metrics.DiskUsage); err != nil {
			h.logger.Error("Failed to update node load metrics",
				logger.F("node_id", nodeData.ID),
				logger.F("error", err.Error()))
		} else {
			h.logger.Info("节点负载指标已更新",
				logger.F("node_id", nodeData.ID),
				logger.F("cpu", req.Metrics.CPUUsage),
				logger.F("memory", req.Metrics.MemoryUsage),
				logger.F("disk", req.Metrics.DiskUsage))
		}

		// 更新 Xray 状态
		if err := h.nodeRepo.UpdateXrayStatus(c.Request.Context(), nodeData.ID,
			req.Metrics.XrayRunning,
			req.Metrics.XrayVersion); err != nil {
			h.logger.Error("Failed to update xray status",
				logger.F("node_id", nodeData.ID),
				logger.F("error", err.Error()))
		} else {
			h.logger.Debug("Xray 状态已更新",
				logger.F("node_id", nodeData.ID),
				logger.F("xray_running", req.Metrics.XrayRunning),
				logger.F("xray_version", req.Metrics.XrayVersion))
		}

		if req.Metrics.XrayRunning {
			h.recoveryTracker.MarkCommandRecovered(nodeData.ID, commandTypeXrayStart, "节点心跳已恢复，Xray 正在运行")
		} else {
			h.QueueXrayRecoveryCommand(nodeData.ID, "heartbeat", "heartbeat reported xray not running")
		}
	} else {
		h.logger.Warn("心跳请求中没有指标数据",
			logger.F("node_id", nodeData.ID))
	}

	// Get any pending commands for this node
	commands := h.getPendingCommands(nodeData.ID)

	c.JSON(http.StatusOK, HeartbeatResponse{
		Success:  true,
		Message:  "Heartbeat received",
		Commands: commands,
	})
}

// ReportCommandResult handles command result reports from nodes.
// POST /api/node/command/result
func (h *NodeAgentHandler) ReportCommandResult(c *gin.Context) {
	var req CommandResultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Get token from header
	token := c.GetHeader("X-Node-Token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Missing node token",
		})
		return
	}

	// Validate token
	nodeData, err := h.nodeService.ValidateToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid or revoked token",
		})
		return
	}

	commandType := h.recoveryTracker.CompleteInflightCommand(nodeData.ID, req.CommandID, req.Success, req.Message)
	h.logger.Info("Command result received",
		logger.F("node_id", nodeData.ID),
		logger.F("command_id", req.CommandID),
		logger.F("command_type", commandType),
		logger.F("success", req.Success),
		logger.F("message", req.Message))

	if err := updateNodeSyncStatusFromCommandResult(c.Request.Context(), h.nodeRepo, nodeData.ID, commandType, req.Success); err != nil {
		h.logger.Error("Failed to update node sync status from command result",
			logger.F("node_id", nodeData.ID),
			logger.F("command_id", req.CommandID),
			logger.F("command_type", commandType),
			logger.F("error", err.Error()))
	}
	if err := updateNodeXrayStatusFromCommandResult(c.Request.Context(), h.nodeRepo, nodeData.ID, commandType, req.Success, req.Data); err != nil {
		h.logger.Error("Failed to update node xray status from command result",
			logger.F("node_id", nodeData.ID),
			logger.F("command_id", req.CommandID),
			logger.F("command_type", commandType),
			logger.F("error", err.Error()))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Result received",
	})
}

type nodeXrayStatusUpdater interface {
	UpdateXrayStatus(ctx context.Context, id int64, xrayRunning bool, xrayVersion string) error
}

type commandReportedXrayStatus struct {
	Running bool   `json:"running"`
	Version string `json:"version"`
}

func updateNodeXrayStatusFromCommandResult(ctx context.Context, updater nodeXrayStatusUpdater, nodeID int64, commandType string, success bool, data any) error {
	if updater == nil || nodeID <= 0 || !success {
		return nil
	}

	switch commandType {
	case commandTypeXrayStart, commandTypeXrayRestart, commandTypeXrayStatus:
		status, ok := extractReportedXrayStatus(data)
		if !ok {
			return nil
		}
		return updater.UpdateXrayStatus(ctx, nodeID, status.Running, status.Version)
	case "xray_stop":
		return updater.UpdateXrayStatus(ctx, nodeID, false, "")
	default:
		return nil
	}
}

func extractReportedXrayStatus(data any) (*commandReportedXrayStatus, bool) {
	if data == nil {
		return nil, false
	}

	switch value := data.(type) {
	case map[string]any:
		status := &commandReportedXrayStatus{
			Version: stringValueFromAny(value["version"]),
		}
		running, ok := boolValueFromAny(value["running"])
		if !ok {
			return nil, false
		}
		status.Running = running
		return status, true
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, false
		}
		var status commandReportedXrayStatus
		if err := json.Unmarshal(raw, &status); err != nil {
			return nil, false
		}
		return &status, true
	}
}

func boolValueFromAny(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	default:
		return false, false
	}
}

func stringValueFromAny(value any) string {
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

// GetConfig returns the configuration for a node.
// GET /api/node/:id/config
func (h *NodeAgentHandler) GetConfig(c *gin.Context) {
	// Get token from header
	token := c.GetHeader("X-Node-Token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Missing node token",
		})
		return
	}

	// Validate token
	nodeData, err := h.nodeService.ValidateToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Invalid or revoked token",
		})
		return
	}

	// Get node ID from URL parameter
	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	// Verify node ID matches token
	if nodeData.ID != nodeID {
		h.logger.Warn("Config request failed: node ID mismatch",
			logger.F("expected_node_id", nodeData.ID),
			logger.F("requested_node_id", nodeID))
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "Node ID does not match token",
		})
		return
	}

	// Generate Xray configuration
	config, err := h.configGenerator.GenerateForNode(c.Request.Context(), nodeID)
	if err != nil {
		h.logger.Error("Failed to generate node config",
			logger.F("node_id", nodeID),
			logger.F("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate configuration",
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

	h.logger.Info("Node config generated",
		logger.F("node_id", nodeID),
		logger.F("inbound_count", len(config.Inbounds)))

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"node_id":   nodeID,
		"version":   "1.0",
		"timestamp": time.Now().Unix(),
		"config":    string(configJSON),
	})
}

// GetSystemInfo returns system information for the Panel.
// GET /api/node/system-info
func (h *NodeAgentHandler) GetSystemInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"os":            runtime.GOOS,
			"arch":          runtime.GOARCH,
			"go_version":    runtime.Version(),
			"num_cpu":       runtime.NumCPU(),
			"num_goroutine": runtime.NumGoroutine(),
		},
	})
}

// QueueXrayRecoveryCommand queues a lightweight recovery command for a node.
func (h *NodeAgentHandler) QueueXrayRecoveryCommand(nodeID int64, source, reason string) bool {
	if h.recoveryTracker == nil {
		return false
	}
	return h.recoveryTracker.QueueXrayRecoveryCommand(nodeID, source, reason)
}

// GetRecentRecoveryEvents returns recent recovery events for a node.
func (h *NodeAgentHandler) GetRecentRecoveryEvents(nodeID int64) []NodeRecoveryEvent {
	if h.recoveryTracker == nil {
		return []NodeRecoveryEvent{}
	}
	return h.recoveryTracker.GetRecentRecoveryEvents(nodeID)
}

// getPendingCommands returns pending commands for a node.
func (h *NodeAgentHandler) getPendingCommands(nodeID int64) []Command {
	if h.recoveryTracker == nil {
		return []Command{}
	}
	return h.recoveryTracker.GetPendingCommands(nodeID)
}

// truncateToken truncates a token for logging purposes.
func truncateToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

func updateNodeSyncStatusFromCommandResult(
	ctx context.Context,
	nodeRepo nodeSyncStatusUpdater,
	nodeID int64,
	commandType string,
	success bool,
) error {
	if nodeRepo == nil || nodeID <= 0 || commandType != commandTypeConfigSync {
		return nil
	}

	if success {
		syncedAt := time.Now()
		return nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusSynced, &syncedAt)
	}

	return nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusFailed, nil)
}
