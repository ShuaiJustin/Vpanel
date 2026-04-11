// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/ip"
	"v/internal/logger"
	"v/internal/node"
	"v/internal/xray"
)

const (
	defaultTrafficBatchTTL         = 6 * time.Hour
	trafficBatchDedupPruneInterval = time.Minute
)

type nodeAgentTrafficRecorder interface {
	RecordTrafficBatch(ctx context.Context, records []*node.TrafficRecord) error
}

// NodeAgentHandler handles Node Agent API requests.
type NodeAgentHandler struct {
	nodeService     *node.Service
	trafficService  nodeAgentTrafficRecorder
	nodeRepo        repository.NodeRepository
	configGenerator *xray.ConfigGenerator
	recoveryTracker *NodeRecoveryTracker
	trafficDeduper  *trafficBatchDeduper
	ipService       *ip.Service
	logger          logger.Logger
}

// NewNodeAgentHandler creates a new NodeAgentHandler.
func NewNodeAgentHandler(
	nodeService *node.Service,
	trafficService nodeAgentTrafficRecorder,
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
		trafficService:  trafficService,
		nodeRepo:        nodeRepo,
		configGenerator: configGenerator,
		recoveryTracker: recoveryTracker,
		trafficDeduper:  newTrafficBatchDeduper(defaultTrafficBatchTTL),
		logger:          log,
	}
}

// WithIPService injects IP tracking for node-reported proxy sessions.
func (h *NodeAgentHandler) WithIPService(ipService *ip.Service) *NodeAgentHandler {
	h.ipService = ipService
	return h
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
	NodeID         int64                `json:"node_id" binding:"required"`
	Token          string               `json:"token" binding:"required"`
	Metrics        *NodeMetrics         `json:"metrics"`
	Traffic        []TrafficRecord      `json:"traffic,omitempty"`
	ProxySessions  []ProxySessionRecord `json:"proxy_sessions,omitempty"`
	TrafficBatchID string               `json:"traffic_batch_id,omitempty"`
}

// TrafficRecord represents per-user traffic reported by the node agent.
type TrafficRecord struct {
	UserID   int64  `json:"user_id"`
	ProxyID  *int64 `json:"proxy_id,omitempty"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// ProxySessionRecord represents a recently active client IP observed by the node agent.
type ProxySessionRecord struct {
	UserID     int64  `json:"user_id"`
	ProxyID    int64  `json:"proxy_id"`
	IP         string `json:"ip"`
	LastSeen   int64  `json:"last_seen"`
	DeviceInfo string `json:"device_info,omitempty"`
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
		h.logger.Debug("Node registration failed: invalid token",
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
		h.logger.Debug("Heartbeat failed: invalid token",
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

	// Preserve unhealthy state until the health checker confirms recovery.
	newStatus := nodeData.Status
	if shouldPromoteNodeOnlineFromHeartbeat(nodeData.Status, req.Metrics) {
		newStatus = repository.NodeStatusOnline
	} else {
		h.logger.Debug("Skip heartbeat status promotion for unhealthy node",
			logger.F("node_id", nodeData.ID),
			logger.F("node_name", nodeData.Name),
			logger.F("has_metrics", req.Metrics != nil))
	}

	// Batch update all heartbeat fields in a single atomic query to prevent race conditions
	var cpuUsage, memoryUsage, diskUsage float64
	var connections int
	var xrayRunning bool
	var xrayVersion string
	if req.Metrics != nil {
		cpuUsage = req.Metrics.CPUUsage
		memoryUsage = req.Metrics.MemoryUsage
		diskUsage = req.Metrics.DiskUsage
		connections = req.Metrics.Connections
		xrayRunning = req.Metrics.XrayRunning
		xrayVersion = req.Metrics.XrayVersion

		h.logger.Info("收到节点指标数据",
			logger.F("node_id", nodeData.ID),
			logger.F("connections", connections),
			logger.F("cpu_usage", cpuUsage),
			logger.F("memory_usage", memoryUsage),
			logger.F("disk_usage", diskUsage),
			logger.F("xray_running", xrayRunning))
	} else {
		h.logger.Warn("心跳请求中没有指标数据",
			logger.F("node_id", nodeData.ID))
	}

	if err := h.nodeRepo.UpdateHeartbeatBatch(c.Request.Context(), nodeData.ID,
		newStatus, time.Now(), 0, connections,
		cpuUsage, memoryUsage, diskUsage,
		xrayRunning, xrayVersion); err != nil {
		h.logger.Error("Failed to update heartbeat batch",
			logger.F("node_id", nodeData.ID),
			logger.F("error", err.Error()))
	}

	// Handle Xray recovery
	if req.Metrics != nil {
		if req.Metrics.XrayRunning {
			h.recoveryTracker.MarkCommandRecovered(nodeData.ID, commandTypeXrayStart, "节点心跳已恢复，Xray 正在运行")
		} else {
			h.QueueXrayRecoveryCommand(nodeData.ID, "heartbeat", "heartbeat reported xray not running")
		}
	}

	if len(req.Traffic) > 0 && h.trafficService != nil {
		recordTraffic, doneTrafficBatch := h.beginTrafficBatch(nodeData.ID, req.TrafficBatchID)
		trafficRecorded := false
		defer func() {
			doneTrafficBatch(trafficRecorded)
		}()

		records := make([]*node.TrafficRecord, 0, len(req.Traffic))
		for _, traffic := range req.Traffic {
			if traffic.UserID < 0 {
				continue
			}
			records = append(records, &node.TrafficRecord{
				NodeID:   nodeData.ID,
				UserID:   traffic.UserID,
				ProxyID:  traffic.ProxyID,
				Upload:   traffic.Upload,
				Download: traffic.Download,
			})
		}

		if recordTraffic && len(records) > 0 {
			if err := h.trafficService.RecordTrafficBatch(c.Request.Context(), records); err != nil {
				h.logger.Error("Failed to record traffic from heartbeat",
					logger.F("node_id", nodeData.ID),
					logger.F("record_count", len(records)),
					logger.F("error", err.Error()))
				c.JSON(http.StatusInternalServerError, HeartbeatResponse{
					Success: false,
					Message: "Failed to record traffic",
				})
				return
			}
		} else if !recordTraffic {
			h.logger.Debug("Skip duplicate heartbeat traffic batch",
				logger.F("node_id", nodeData.ID),
				logger.F("traffic_batch_id", req.TrafficBatchID),
				logger.F("record_count", len(records)))
		}

		trafficRecorded = true
	}

	if len(req.ProxySessions) > 0 && h.ipService != nil {
		activities := make([]ip.ProxySessionActivity, 0, len(req.ProxySessions))
		for _, session := range req.ProxySessions {
			if session.UserID <= 0 || strings.TrimSpace(session.IP) == "" {
				continue
			}

			seenAt := time.Unix(session.LastSeen, 0).UTC()
			if session.LastSeen <= 0 {
				seenAt = time.Now().UTC()
			}

			activities = append(activities, ip.ProxySessionActivity{
				UserID:     uint(session.UserID),
				ProxyID:    session.ProxyID,
				IP:         strings.TrimSpace(session.IP),
				LastSeen:   seenAt,
				DeviceInfo: strings.TrimSpace(session.DeviceInfo),
			})
		}

		if len(activities) > 0 {
			if err := h.ipService.RecordProxySessions(c.Request.Context(), activities); err != nil {
				h.logger.Warn("failed to record proxy session activity from heartbeat",
					logger.F("node_id", nodeData.ID),
					logger.F("session_count", len(activities)),
					logger.F("error", err.Error()))
			}
		}
	}

	// Get any pending commands for this node
	commands := h.getPendingCommands(nodeData.ID)

	c.JSON(http.StatusOK, HeartbeatResponse{
		Success:  true,
		Message:  "Heartbeat received",
		Commands: commands,
	})
}

func shouldPromoteNodeOnlineFromHeartbeat(currentStatus string, metrics *NodeMetrics) bool {
	if currentStatus == repository.NodeStatusUnhealthy {
		return false
	}

	return true
}

func (h *NodeAgentHandler) beginTrafficBatch(nodeID int64, batchID string) (bool, func(bool)) {
	if h == nil || h.trafficDeduper == nil {
		return true, func(bool) {}
	}
	return h.trafficDeduper.begin(nodeID, batchID)
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
		"version":   hashConfigVersion(configJSON),
		"timestamp": time.Now().Unix(),
		"config":    string(configJSON),
	})
}

func hashConfigVersion(config []byte) string {
	digest := sha256.Sum256(config)
	return hex.EncodeToString(digest[:])
}

type trafficBatchDeduper struct {
	mu         sync.Mutex
	ttl        time.Duration
	lastPruned time.Time
	processed  map[string]time.Time
	inflight   map[string]struct{}
}

func newTrafficBatchDeduper(ttl time.Duration) *trafficBatchDeduper {
	if ttl <= 0 {
		ttl = defaultTrafficBatchTTL
	}
	return &trafficBatchDeduper{
		ttl:       ttl,
		processed: make(map[string]time.Time),
		inflight:  make(map[string]struct{}),
	}
}

func (d *trafficBatchDeduper) begin(nodeID int64, batchID string) (bool, func(bool)) {
	trimmedBatchID := strings.TrimSpace(batchID)
	if d == nil || nodeID <= 0 || trimmedBatchID == "" {
		return true, func(bool) {}
	}

	now := time.Now().UTC()
	key := strconv.FormatInt(nodeID, 10) + ":" + trimmedBatchID

	d.mu.Lock()
	if d.lastPruned.IsZero() || now.Sub(d.lastPruned) >= trafficBatchDedupPruneInterval {
		for existingKey, expiresAt := range d.processed {
			if !expiresAt.After(now) {
				delete(d.processed, existingKey)
			}
		}
		d.lastPruned = now
	}
	if expiresAt, exists := d.processed[key]; exists && expiresAt.After(now) {
		d.mu.Unlock()
		return false, func(bool) {}
	}
	if _, exists := d.inflight[key]; exists {
		d.mu.Unlock()
		return false, func(bool) {}
	}
	d.inflight[key] = struct{}{}
	d.mu.Unlock()

	return true, func(success bool) {
		d.mu.Lock()
		defer d.mu.Unlock()

		delete(d.inflight, key)
		if success {
			d.processed[key] = now.Add(d.ttl)
		}
	}
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
