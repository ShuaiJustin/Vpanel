// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/logger"
	"v/internal/node"
	"v/pkg/errors"
)

// NodeHandler handles node management API requests.
type NodeHandler struct {
	nodeService     *node.Service
	deployService   *node.RemoteDeployService
	recoveryTracker *NodeRecoveryTracker
	logger          logger.Logger
}

type queueNodeCommandFunc func(nodeID int64, source, reason string) (Command, bool)

// NewNodeHandler creates a new node handler.
func NewNodeHandler(nodeService *node.Service, deployService *node.RemoteDeployService, recoveryTracker *NodeRecoveryTracker, log logger.Logger) *NodeHandler {
	return &NodeHandler{
		nodeService:     nodeService,
		deployService:   deployService,
		recoveryTracker: recoveryTracker,
		logger:          log,
	}
}

// NodeResponse represents a node in API responses.
type NodeResponse struct {
	ID                int64    `json:"id"`
	Name              string   `json:"name"`
	Address           string   `json:"address"`
	Port              int      `json:"port"`
	PanelURL          string   `json:"panel_url"` // Panel server URL
	Status            string   `json:"status"`
	Tags              []string `json:"tags"`
	Region            string   `json:"region"`
	Weight            int      `json:"weight"`
	MaxUsers          int      `json:"max_users"`
	CurrentUsers      int      `json:"current_users"`
	Latency           int      `json:"latency"`
	LastSeenAt        string   `json:"last_seen_at,omitempty"`
	SyncStatus        string   `json:"sync_status"`
	SyncedAt          string   `json:"synced_at,omitempty"`
	InstallStatus     string   `json:"install_status"`
	InstallMessage    string   `json:"install_message,omitempty"`
	InstallStartedAt  string   `json:"install_started_at,omitempty"`
	InstallFinishedAt string   `json:"install_finished_at,omitempty"`
	InstallUpdatedAt  string   `json:"install_updated_at,omitempty"`
	IPWhitelist       []string `json:"ip_whitelist,omitempty"`

	// 流量统计
	TrafficUp      int64  `json:"traffic_up"`
	TrafficDown    int64  `json:"traffic_down"`
	TrafficTotal   int64  `json:"traffic_total"`
	TrafficLimit   int64  `json:"traffic_limit"`
	TrafficResetAt string `json:"traffic_reset_at,omitempty"`

	// 负载信息
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetSpeed    int64   `json:"net_speed"`

	// 速率限制
	SpeedLimit int64 `json:"speed_limit"`

	// 协议支持
	Protocols []string `json:"protocols,omitempty"`

	// TLS 配置
	TLSEnabled bool   `json:"tls_enabled"`
	TLSDomain  string `json:"tls_domain,omitempty"`

	// 节点分组
	GroupID *int64 `json:"group_id,omitempty"`

	// 排序和优先级
	Priority int `json:"priority"`
	Sort     int `json:"sort"`

	// 告警配置
	AlertTrafficThreshold float64 `json:"alert_traffic_threshold"`
	AlertCPUThreshold     float64 `json:"alert_cpu_threshold"`
	AlertMemoryThreshold  float64 `json:"alert_memory_threshold"`

	// 备注和描述
	Description string `json:"description,omitempty"`
	Remarks     string `json:"remarks,omitempty"`

	// Xray 状态
	XrayRunning bool   `json:"xray_running"`
	XrayVersion string `json:"xray_version,omitempty"`

	// 恢复记录
	LastRecoveryStatus   string              `json:"last_recovery_status,omitempty"`
	LastRecoveryMessage  string              `json:"last_recovery_message,omitempty"`
	LastRecoverySource   string              `json:"last_recovery_source,omitempty"`
	LastRecoveryAt       string              `json:"last_recovery_at,omitempty"`
	RecentRecoveryEvents []NodeRecoveryEvent `json:"recent_recovery_events,omitempty"`

	// 证书关联
	CertificateID *int64 `json:"certificate_id,omitempty"`

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// NodeWithTokenResponse includes the token (only returned on create).
type NodeWithTokenResponse struct {
	NodeResponse
	Token string `json:"token"`
}

// CreateNodeRequest represents a request to create a node.
type CreateNodeRequest struct {
	Name        string   `json:"name" binding:"required"`
	Address     string   `json:"address" binding:"required"`
	Port        int      `json:"port"`
	PanelURL    string   `json:"panel_url"` // Panel server URL
	Tags        []string `json:"tags"`
	Region      string   `json:"region"`
	Weight      int      `json:"weight"`
	MaxUsers    int      `json:"max_users"`
	IPWhitelist []string `json:"ip_whitelist"`

	// SSH 自动安装配置（可选）
	SSH *SSHConfig `json:"ssh,omitempty"`

	// 流量和速率
	TrafficLimit int64 `json:"traffic_limit"`
	SpeedLimit   int64 `json:"speed_limit"`

	// 协议支持
	Protocols []string `json:"protocols"`

	// TLS 配置
	TLSEnabled bool   `json:"tls_enabled"`
	TLSDomain  string `json:"tls_domain"`

	// 节点分组
	GroupID *int64 `json:"group_id"`

	// 排序和优先级
	Priority int `json:"priority"`
	Sort     int `json:"sort"`

	// 告警配置
	AlertTrafficThreshold float64 `json:"alert_traffic_threshold"`
	AlertCPUThreshold     float64 `json:"alert_cpu_threshold"`
	AlertMemoryThreshold  float64 `json:"alert_memory_threshold"`

	// 备注和描述
	Description string `json:"description"`
	Remarks     string `json:"remarks"`

	// 证书关联
	CertificateID *int64 `json:"certificate_id"`
}

// SSHConfig SSH 连接配置
type SSHConfig struct {
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	PanelURL   string `json:"panel_url"` // Panel 服务器地址
}

// UpdateNodeRequest represents a request to update a node.
type UpdateNodeRequest struct {
	Name        *string   `json:"name"`
	Address     *string   `json:"address"`
	Port        *int      `json:"port"`
	PanelURL    *string   `json:"panel_url"` // Panel server URL
	Tags        *[]string `json:"tags"`
	Region      *string   `json:"region"`
	Weight      *int      `json:"weight"`
	MaxUsers    *int      `json:"max_users"`
	IPWhitelist *[]string `json:"ip_whitelist"`

	// 流量和速率
	TrafficLimit *int64 `json:"traffic_limit"`
	SpeedLimit   *int64 `json:"speed_limit"`

	// 协议支持
	Protocols *[]string `json:"protocols"`

	// TLS 配置
	TLSEnabled *bool   `json:"tls_enabled"`
	TLSDomain  *string `json:"tls_domain"`

	// 节点分组
	GroupID *int64 `json:"group_id"`

	// 排序和优先级
	Priority *int `json:"priority"`
	Sort     *int `json:"sort"`

	// 告警配置
	AlertTrafficThreshold *float64 `json:"alert_traffic_threshold"`
	AlertCPUThreshold     *float64 `json:"alert_cpu_threshold"`
	AlertMemoryThreshold  *float64 `json:"alert_memory_threshold"`

	// 备注和描述
	Description *string `json:"description"`
	Remarks     *string `json:"remarks"`

	// 证书关联
	CertificateID *int64 `json:"certificate_id"`
}

// toNodeResponse converts a node to API response format.
func toNodeResponse(n *node.Node) *NodeResponse {
	resp := &NodeResponse{
		ID:             n.ID,
		Name:           n.Name,
		Address:        n.Address,
		Port:           n.Port,
		PanelURL:       n.PanelURL, // 添加 Panel URL 字段
		Status:         n.Status,
		Tags:           n.Tags,
		Region:         n.Region,
		Weight:         n.Weight,
		MaxUsers:       n.MaxUsers,
		CurrentUsers:   n.CurrentUsers,
		Latency:        n.Latency,
		SyncStatus:     n.SyncStatus,
		InstallStatus:  n.InstallStatus,
		InstallMessage: n.InstallMessage,
		IPWhitelist:    n.IPWhitelist,

		// 流量统计
		TrafficUp:    n.TrafficUp,
		TrafficDown:  n.TrafficDown,
		TrafficTotal: n.TrafficTotal,
		TrafficLimit: n.TrafficLimit,

		// 负载信息
		CPUUsage:    n.CPUUsage,
		MemoryUsage: n.MemoryUsage,
		DiskUsage:   n.DiskUsage,
		NetSpeed:    n.NetSpeed,

		// 速率限制
		SpeedLimit: n.SpeedLimit,

		// 协议支持
		Protocols: n.Protocols,

		// TLS 配置
		TLSEnabled: n.TLSEnabled,
		TLSDomain:  n.TLSDomain,

		// 节点分组
		GroupID: n.GroupID,

		// 排序和优先级
		Priority: n.Priority,
		Sort:     n.Sort,

		// 告警配置
		AlertTrafficThreshold: n.AlertTrafficThreshold,
		AlertCPUThreshold:     n.AlertCPUThreshold,
		AlertMemoryThreshold:  n.AlertMemoryThreshold,

		// 备注和描述
		Description: n.Description,
		Remarks:     n.Remarks,

		// Xray 状态
		XrayRunning: n.XrayRunning,
		XrayVersion: n.XrayVersion,

		// 证书关联
		CertificateID: n.CertificateID,

		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: n.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if n.LastSeenAt != nil {
		resp.LastSeenAt = n.LastSeenAt.Format("2006-01-02T15:04:05Z")
	}
	if n.SyncedAt != nil {
		resp.SyncedAt = n.SyncedAt.Format("2006-01-02T15:04:05Z")
	}
	if n.InstallStartedAt != nil {
		resp.InstallStartedAt = n.InstallStartedAt.Format("2006-01-02T15:04:05Z")
	}
	if n.InstallFinishedAt != nil {
		resp.InstallFinishedAt = n.InstallFinishedAt.Format("2006-01-02T15:04:05Z")
	}
	if n.InstallUpdatedAt != nil {
		resp.InstallUpdatedAt = n.InstallUpdatedAt.Format("2006-01-02T15:04:05Z")
	}
	if n.TrafficResetAt != nil {
		resp.TrafficResetAt = n.TrafficResetAt.Format("2006-01-02T15:04:05Z")
	}
	if resp.Tags == nil {
		resp.Tags = []string{}
	}
	if resp.IPWhitelist == nil {
		resp.IPWhitelist = []string{}
	}
	if resp.Protocols == nil {
		resp.Protocols = []string{}
	}
	return resp
}

func (h *NodeHandler) buildNodeResponse(n *node.Node) *NodeResponse {
	resp := toNodeResponse(n)
	if h == nil || h.recoveryTracker == nil || resp == nil {
		return resp
	}

	events := h.recoveryTracker.GetRecentRecoveryEvents(n.ID)
	if len(events) == 0 {
		resp.RecentRecoveryEvents = []NodeRecoveryEvent{}
		return resp
	}

	resp.RecentRecoveryEvents = events
	resp.LastRecoveryStatus = events[0].Status
	resp.LastRecoveryMessage = events[0].Message
	resp.LastRecoverySource = events[0].Source
	if events[0].UpdatedAt != "" {
		resp.LastRecoveryAt = events[0].UpdatedAt
	} else {
		resp.LastRecoveryAt = events[0].CreatedAt
	}
	return resp
}

// List returns all nodes with optional filtering.
// GET /api/admin/nodes
func (h *NodeHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	region := c.Query("region")

	filter := node.NodeFilter{
		Status: status,
		Region: region,
		Limit:  limit,
		Offset: offset,
	}

	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err == nil {
			filter.GroupID = &groupID
		}
	}

	nodes, total, err := h.nodeService.List(c.Request.Context(), filter)
	if err != nil {
		h.logger.Error("Failed to list nodes", logger.Err(err))
		middleware.HandleInternalError(c, errors.MsgNodeCreateFailed, err)
		return
	}

	response := make([]*NodeResponse, len(nodes))
	for i, n := range nodes {
		response[i] = h.buildNodeResponse(n)
	}

	c.JSON(http.StatusOK, gin.H{
		"nodes": response,
		"total": total,
	})
}

// GetInstallStatus returns async install status for a node.
// GET /api/admin/nodes/:id/install-status
func (h *NodeHandler) GetInstallStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.HandleBadRequest(c, errors.MsgFieldInvalidFormat)
		return
	}

	if _, err := h.nodeService.GetByID(c.Request.Context(), id); err != nil {
		if err == node.ErrNodeNotFound {
			middleware.HandleNotFound(c, "node", id)
			return
		}
		h.logger.Error("Failed to get node install status", logger.Err(err), logger.F("id", id))
		middleware.HandleInternalError(c, errors.MsgNodeNotFound, err)
		return
	}

	if status, ok := h.deployService.GetInstallStatus(id); ok {
		c.JSON(http.StatusOK, status)
		return
	}

	if nodeData, err := h.nodeService.GetByID(c.Request.Context(), id); err == nil {
		status := nodeData.InstallStatus
		if status == "" {
			status = "idle"
		}
		message := nodeData.InstallMessage
		if message == "" && status == "idle" {
			message = "当前没有正在进行的自动安装任务"
		}
		response := gin.H{
			"node_id":    id,
			"status":     status,
			"message":    message,
			"steps":      nodeData.InstallSteps,
			"logs":       nodeData.InstallLogs,
			"updated_at": time.Now().Format(time.RFC3339),
		}
		if nodeData.InstallStartedAt != nil {
			response["started_at"] = nodeData.InstallStartedAt.Format(time.RFC3339)
		}
		if nodeData.InstallFinishedAt != nil {
			response["finished_at"] = nodeData.InstallFinishedAt.Format(time.RFC3339)
		}
		if nodeData.InstallUpdatedAt != nil {
			response["updated_at"] = nodeData.InstallUpdatedAt.Format(time.RFC3339)
		}
		c.JSON(http.StatusOK, response)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"node_id":    id,
		"status":     "idle",
		"message":    "当前没有正在进行的自动安装任务",
		"steps":      []node.DeployStep{},
		"logs":       "",
		"updated_at": time.Now().Format(time.RFC3339),
	})
}

// Get returns a single node by ID.
// GET /api/admin/nodes/:id
func (h *NodeHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		middleware.HandleBadRequest(c, errors.MsgFieldInvalidFormat)
		return
	}

	n, err := h.nodeService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			middleware.HandleNotFound(c, "node", id)
			return
		}
		h.logger.Error("Failed to get node", logger.Err(err), logger.F("id", id))
		middleware.HandleInternalError(c, errors.MsgNodeNotFound, err)
		return
	}

	c.JSON(http.StatusOK, h.buildNodeResponse(n))
}

// Create creates a new node.
// POST /api/admin/nodes
func (h *NodeHandler) Create(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
		return
	}

	createReq := &node.CreateNodeRequest{
		Name:        req.Name,
		Address:     req.Address,
		Port:        req.Port,
		PanelURL:    req.PanelURL, // 保存 Panel URL
		Tags:        req.Tags,
		Region:      req.Region,
		Weight:      req.Weight,
		MaxUsers:    req.MaxUsers,
		IPWhitelist: req.IPWhitelist,

		// 流量和速率
		TrafficLimit: req.TrafficLimit,
		SpeedLimit:   req.SpeedLimit,

		// 协议支持
		Protocols: req.Protocols,

		// TLS 配置
		TLSEnabled: req.TLSEnabled,
		TLSDomain:  req.TLSDomain,

		// 节点分组
		GroupID: req.GroupID,

		// 排序和优先级
		Priority: req.Priority,
		Sort:     req.Sort,

		// 告警配置
		AlertTrafficThreshold: req.AlertTrafficThreshold,
		AlertCPUThreshold:     req.AlertCPUThreshold,
		AlertMemoryThreshold:  req.AlertMemoryThreshold,

		// 备注和描述
		Description: req.Description,
		Remarks:     req.Remarks,

		// 证书关联
		CertificateID: req.CertificateID,
	}

	n, err := h.nodeService.Create(c.Request.Context(), createReq)
	if err != nil {
		if err == node.ErrInvalidAddress {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node address format"})
			return
		}
		if err == node.ErrInvalidNode {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node data"})
			return
		}
		h.logger.Error("Failed to create node", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create node"})
		return
	}

	h.logger.Info("Node created", logger.F("node_id", n.ID), logger.F("name", n.Name))

	resp := &NodeWithTokenResponse{
		NodeResponse: *h.buildNodeResponse(n),
		Token:        n.Token,
	}

	// 如果提供了 SSH 配置，启动后台部署并立即返回
	if req.SSH != nil && h.deployService != nil {
		h.logger.Info("Starting auto-install", logger.F("node_id", n.ID), logger.F("host", req.SSH.Host))

		// 获取 Panel URL - 优先使用数据库中保存的值，其次使用前端传递的，最后使用请求 Host
		panelURL := n.PanelURL
		if panelURL == "" && req.SSH.PanelURL != "" {
			panelURL = req.SSH.PanelURL
		}
		if panelURL == "" {
			panelURL = c.Request.Header.Get("X-Panel-URL")
		}
		if panelURL == "" {
			scheme := "http"
			if c.Request.TLS != nil {
				scheme = "https"
			}
			panelURL = scheme + "://" + c.Request.Host
		}

		h.logger.Info("Using Panel URL for deployment",
			logger.F("panel_url", panelURL),
			logger.F("node_id", n.ID))

		deployConfig := &node.DeployConfig{
			NodeID:     n.ID,
			Host:       req.SSH.Host,
			Port:       req.SSH.Port,
			Username:   req.SSH.Username,
			Password:   req.SSH.Password,
			PrivateKey: req.SSH.PrivateKey,
			PanelURL:   panelURL,
			NodeToken:  n.Token,
		}

		if deployConfig.Port == 0 {
			deployConfig.Port = 22
		}

		h.logger.Info("Queueing agent deployment in background", logger.F("node_id", n.ID))
		h.deployService.MarkInstallQueued(n.ID, "节点创建成功，等待开始自动安装")

		go func(nodeID int64, host string, cfg *node.DeployConfig) {
			result, err := h.deployService.Deploy(context.Background(), cfg)
			if err != nil {
				message := ""
				if result != nil {
					message = result.Message
				}
				h.logger.Error("Auto-install failed",
					logger.Err(err),
					logger.F("node_id", nodeID),
					logger.F("message", message))
				return
			}

			h.logger.Info("Auto-install completed successfully",
				logger.F("node_id", nodeID),
				logger.F("host", host))
		}(n.ID, req.SSH.Host, deployConfig)

		c.JSON(http.StatusCreated, struct {
			*NodeWithTokenResponse
			Installing bool   `json:"installing"`
			Success    bool   `json:"success"`
			Message    string `json:"message"`
		}{
			NodeWithTokenResponse: resp,
			Installing:            true,
			Success:               true,
			Message:               "节点创建成功，后台自动安装已开始",
		})
		return
	}

	// 没有自动安装，返回普通响应（包含 Token）
	c.JSON(http.StatusCreated, resp)
}

// Update updates an existing node.
// PUT /api/admin/nodes/:id
func (h *NodeHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	var req UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	updateReq := &node.UpdateNodeRequest{
		Name:        req.Name,
		Address:     req.Address,
		Port:        req.Port,
		PanelURL:    req.PanelURL, // 添加 Panel URL
		Tags:        req.Tags,
		Region:      req.Region,
		Weight:      req.Weight,
		MaxUsers:    req.MaxUsers,
		IPWhitelist: req.IPWhitelist,

		// 流量和速率
		TrafficLimit: req.TrafficLimit,
		SpeedLimit:   req.SpeedLimit,

		// 协议支持
		Protocols: req.Protocols,

		// TLS 配置
		TLSEnabled: req.TLSEnabled,
		TLSDomain:  req.TLSDomain,

		// 节点分组
		GroupID: req.GroupID,

		// 排序和优先级
		Priority: req.Priority,
		Sort:     req.Sort,

		// 告警配置
		AlertTrafficThreshold: req.AlertTrafficThreshold,
		AlertCPUThreshold:     req.AlertCPUThreshold,
		AlertMemoryThreshold:  req.AlertMemoryThreshold,

		// 备注和描述
		Description: req.Description,
		Remarks:     req.Remarks,

		// 证书关联
		CertificateID: req.CertificateID,
	}

	n, err := h.nodeService.Update(c.Request.Context(), id, updateReq)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		if err == node.ErrInvalidAddress {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node address format"})
			return
		}
		h.logger.Error("Failed to update node", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	h.logger.Info("Node updated", logger.F("node_id", id))

	c.JSON(http.StatusOK, h.buildNodeResponse(n))
}

// Delete deletes a node.
// DELETE /api/admin/nodes/:id
func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := h.nodeService.Delete(c.Request.Context(), id); err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to delete node", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node"})
		return
	}

	h.logger.Info("Node deleted", logger.F("node_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "Node deleted successfully"})
}

// GenerateToken generates a new token for a node.
// POST /api/admin/nodes/:id/token
func (h *NodeHandler) GenerateToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	token, err := h.nodeService.GenerateNodeToken(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to generate token", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	h.logger.Info("Token generated for node", logger.F("node_id", id))

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// RotateToken rotates a node's token.
// POST /api/admin/nodes/:id/token/rotate
func (h *NodeHandler) RotateToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	token, err := h.nodeService.RotateToken(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to rotate token", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rotate token"})
		return
	}

	h.logger.Info("Token rotated for node", logger.F("node_id", id))

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// RevokeToken revokes a node's token.
// POST /api/admin/nodes/:id/token/revoke
func (h *NodeHandler) RevokeToken(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if err := h.nodeService.RevokeToken(c.Request.Context(), id); err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to revoke token", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	h.logger.Info("Token revoked for node", logger.F("node_id", id))

	c.JSON(http.StatusOK, gin.H{"message": "Token revoked successfully"})
}

// GetStatistics returns node statistics.
// GET /api/admin/nodes/statistics
func (h *NodeHandler) GetStatistics(c *gin.Context) {
	stats, err := h.nodeService.GetStatistics(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get node statistics", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	totalUsers, err := h.nodeService.GetTotalUsers(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get total users", logger.Err(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get statistics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"by_status":   stats,
		"total_users": totalUsers,
	})
}

// UpdateStatus updates a node's status.
// PUT /api/admin/nodes/:id/status
func (h *NodeHandler) UpdateStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate status
	validStatuses := map[string]bool{
		"online":    true,
		"offline":   true,
		"unhealthy": true,
	}
	if !validStatuses[req.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status. Must be one of: online, offline, unhealthy"})
		return
	}

	if err := h.nodeService.UpdateStatus(c.Request.Context(), id, req.Status); err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to update node status", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	h.logger.Info("Node status updated", logger.F("node_id", id), logger.F("status", req.Status))

	c.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

func (h *NodeHandler) queueNodeCommand(c *gin.Context, commandType, reason, successMessage string, queueFunc queueNodeCommandFunc) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	if _, err := h.nodeService.GetByID(c.Request.Context(), id); err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to get node for command dispatch", logger.Err(err), logger.F("id", id), logger.F("command_type", commandType))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get node"})
		return
	}

	if h.recoveryTracker == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Node command queue is unavailable"})
		return
	}

	cmd, queued := queueFunc(id, "admin", reason)
	if !queued {
		c.JSON(http.StatusConflict, gin.H{
			"error":        "A similar command is already pending or was queued recently",
			"command_type": commandType,
		})
		return
	}

	h.logger.Info("Node command queued by admin",
		logger.F("node_id", id),
		logger.F("command_id", cmd.ID),
		logger.F("command_type", cmd.Type))

	c.JSON(http.StatusAccepted, gin.H{
		"success":      true,
		"queued":       true,
		"node_id":      id,
		"command_id":   cmd.ID,
		"command_type": cmd.Type,
		"message":      successMessage,
	})
}

// StartCore queues a node-local Xray start command.
// POST /api/admin/nodes/:id/core/start
func (h *NodeHandler) StartCore(c *gin.Context) {
	h.queueNodeCommand(
		c,
		commandTypeXrayStart,
		"管理员手动启动节点 Xray 内核",
		"启动命令已加入队列，将在节点下一次心跳时执行",
		func(nodeID int64, source, reason string) (Command, bool) {
			return h.recoveryTracker.QueueXrayStartCommand(nodeID, source, reason)
		},
	)
}

// RestartCore queues a node-local Xray restart command.
// POST /api/admin/nodes/:id/core/restart
func (h *NodeHandler) RestartCore(c *gin.Context) {
	h.queueNodeCommand(
		c,
		commandTypeXrayRestart,
		"管理员手动重启节点 Xray 内核",
		"重启命令已加入队列，将在节点下一次心跳时执行",
		func(nodeID int64, source, reason string) (Command, bool) {
			return h.recoveryTracker.QueueXrayRestartCommand(nodeID, source, reason)
		},
	)
}

// SyncCoreConfig queues a config_sync command for a node.
// POST /api/admin/nodes/:id/core/sync-config
func (h *NodeHandler) SyncCoreConfig(c *gin.Context) {
	h.queueNodeCommand(
		c,
		commandTypeConfigSync,
		"管理员手动同步节点配置",
		"配置同步命令已加入队列，将在节点下一次心跳时执行",
		func(nodeID int64, source, reason string) (Command, bool) {
			return h.recoveryTracker.QueueConfigSyncCommandDetailed(nodeID, source, reason)
		},
	)
}

// GetXrayConfig returns the Xray configuration for a node.
// GET /api/admin/nodes/:id/xray/config
func (h *NodeHandler) GetXrayConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	config, err := h.nodeService.GetXrayConfig(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to get Xray config", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get Xray configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"config": config,
	})
}
