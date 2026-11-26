// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/config"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/monitor"
	"v/internal/node"
)

// NodeDeployHandler handles node deployment requests.
type NodeDeployHandler struct {
	deployService  *node.RemoteDeployService
	nodeService    *node.Service
	entitlementSvc *entitlement.Service
	auditService   monitor.AuditService
	config         *config.Config
	publicURL      func() string
	logger         logger.Logger
}

// NewNodeDeployHandler creates a new node deploy handler.
func NewNodeDeployHandler(
	deployService *node.RemoteDeployService,
	nodeService *node.Service,
	cfg *config.Config,
	log logger.Logger,
) *NodeDeployHandler {
	return &NodeDeployHandler{
		deployService: deployService,
		nodeService:   nodeService,
		config:        cfg,
		logger:        log,
	}
}

// WithPublicURLProvider sets the runtime panel URL source for agent deployment.
func (h *NodeDeployHandler) WithPublicURLProvider(provider func() string) *NodeDeployHandler {
	h.publicURL = provider
	return h
}

// WithEntitlementService enables proactive proxy provisioning after successful node deployment.
func (h *NodeDeployHandler) WithEntitlementService(svc *entitlement.Service) *NodeDeployHandler {
	h.entitlementSvc = svc
	return h
}

// WithAuditService wires the audit emitter for node deployment actions.
func (h *NodeDeployHandler) WithAuditService(audit monitor.AuditService) *NodeDeployHandler {
	h.auditService = audit
	return h
}

// DeployAgentRequest represents a deploy agent request.
type DeployAgentRequest struct {
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	PanelURL   string `json:"panel_url"` // Panel 服务器地址（可选，优先使用此值）
}

func resolveDeployPanelURL(c *gin.Context, publicURL, explicitURL, savedURL string) string {
	for _, candidate := range []string{explicitURL, publicURL, savedURL, c.Request.Header.Get("X-Panel-URL")} {
		if panelURL := strings.TrimSuffix(strings.TrimSpace(candidate), "/"); panelURL != "" {
			return panelURL
		}
	}

	scheme := "http"
	if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	host := c.Request.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}

	return scheme + "://" + strings.TrimSpace(host)
}

// sanitizeDeployPanelURL validates a resolved panel URL before it is embedded
// into deployment scripts / YAML configs executed on target nodes as root.
// X-Forwarded-Host and X-Panel-URL are client-controlled, so anything that is
// not a plain http(s) URL is rejected (returns empty string) rather than
// interpolated.
func sanitizeDeployPanelURL(panelURL string) string {
	if panelURL == "" {
		return ""
	}
	if err := node.ValidatePanelURL(panelURL); err != nil {
		return ""
	}
	return panelURL
}

// DeployAgent deploys the agent to a remote server.
// POST /api/admin/nodes/:id/deploy
func (h *NodeDeployHandler) DeployAgent(c *gin.Context) {
	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	var req DeployAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate authentication method
	if req.Password == "" && req.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Either password or private_key must be provided",
		})
		return
	}
	if !validNodeSSHPrivateKey(req.PrivateKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": invalidNodeSSHPrivateKeyMessage,
		})
		return
	}

	if h.deployService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "Agent 部署服务不可用"})
		return
	}
	release, ok := h.deployService.BeginInstall(nodeID)
	if !ok {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "该节点已有部署任务，请查看安装进度"})
		return
	}
	queued := false
	defer func() {
		if !queued {
			release()
		}
	}()

	// Get node info
	nodeData, err := h.nodeService.GetByID(c.Request.Context(), nodeID)
	if err != nil {
		h.logger.Error("Failed to get node",
			logger.F("node_id", nodeID),
			logger.F("error", err.Error()))
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Node not found",
		})
		return
	}

	// Generate token if not exists
	if nodeData.Token == "" {
		_, err := h.nodeService.GenerateNodeToken(c.Request.Context(), nodeID)
		if err != nil {
			h.logger.Error("Failed to generate token",
				logger.F("node_id", nodeID),
				logger.F("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to generate node token",
			})
			return
		}

		// 重新从数据库读取节点信息，确保获取最新的 token
		nodeData, err = h.nodeService.GetByID(c.Request.Context(), nodeID)
		if err != nil {
			h.logger.Error("Failed to reload node after token generation",
				logger.F("node_id", nodeID),
				logger.F("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to reload node data",
			})
			return
		}

		h.logger.Info("Generated and loaded new token for node",
			logger.F("node_id", nodeID))
	}

	panelURL := sanitizeDeployPanelURL(resolveDeployPanelURL(c, h.currentPublicURL(), req.PanelURL, nodeData.PanelURL))
	if panelURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Panel URL 无效：必须是形如 https://panel.example.com 的地址",
		})
		return
	}

	// 验证 Panel URL 不能是回环地址
	if isLoopbackOrUnspecifiedDeployURL(panelURL) {
		h.logger.Error("Invalid Panel URL for remote deployment",
			logger.F("panel_url", panelURL),
			logger.F("request_host", c.Request.Host),
			logger.F("x_forwarded_host", c.Request.Header.Get("X-Forwarded-Host")))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Panel URL 配置错误：远程部署不能使用 localhost / 127.0.0.1 / ::1 / 0.0.0.0，请配置 server.public_url 为公网 IP 或域名",
		})
		return
	}

	h.logger.Info("Using Panel URL for deployment",
		logger.F("node_id", nodeID),
		logger.F("panel_url", panelURL))

	if err := persistNodeSSHMetadata(c.Request.Context(), h.nodeService, nodeID, nodeSSHMetadataInput{
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
	}); err != nil {
		h.logger.Warn("Failed to persist node SSH metadata before deployment",
			logger.Err(err),
			logger.F("node_id", nodeID))
	}

	// Prepare deploy config
	deployConfig := &node.DeployConfig{
		NodeID:     nodeID,
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
		PanelURL:   panelURL,
		NodeToken:  nodeData.Token,
	}

	h.logger.Info("Starting agent deployment",
		logger.F("node_id", nodeID),
		logger.F("host", req.Host),
		logger.F("username", req.Username))

	h.deployService.MarkInstallQueued(nodeID, "等待开始部署 Agent")
	queued = true
	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       "node_agent_deploy",
		ResourceType: "node",
		ResourceID:   strconv.FormatInt(nodeID, 10),
		Details:      map[string]interface{}{"host": req.Host, "port": req.Port, "panel_url": panelURL},
	})
	go func() {
		defer release()
		h.runAgentDeployment(deployConfig)
	}()
	c.JSON(http.StatusAccepted, gin.H{"success": true, "status": "queued", "message": "部署任务已加入队列"})
}

// runAgentDeployment owns the bounded lifetime of a queued installation.
func (h *NodeDeployHandler) runAgentDeployment(cfg *node.DeployConfig) {
	defer func() {
		if failure := recover(); failure != nil {
			h.logger.Error("Agent deployment panicked", logger.F("node_id", cfg.NodeID), logger.F("failure", fmt.Sprint(failure)))
			h.deployService.MarkInstallFailed(cfg.NodeID, "部署任务异常退出，请查看面板日志")
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Minute)
	defer cancel()
	if _, err := h.deployService.Deploy(ctx, cfg); err != nil {
		h.logger.Error("Agent deployment failed", logger.F("node_id", cfg.NodeID), logger.Err(err))
		return
	}
	provisionNodeProxiesAfterDeploy(h.entitlementSvc, h.logger, cfg.NodeID)
}

// GetDeployScript returns the deployment script for manual installation.
// GET /api/admin/nodes/:id/deploy/script
func (h *NodeDeployHandler) GetDeployScript(c *gin.Context) {
	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid node ID",
		})
		return
	}

	// Get node info
	nodeData, err := h.nodeService.GetByID(c.Request.Context(), nodeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Node not found",
		})
		return
	}

	// Generate token if not exists
	if nodeData.Token == "" {
		_, err := h.nodeService.GenerateNodeToken(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to generate node token",
			})
			return
		}

		// 重新从数据库读取节点信息，确保获取最新的 token
		nodeData, err = h.nodeService.GetByID(c.Request.Context(), nodeID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to reload node data",
			})
			return
		}
	}

	panelURL := sanitizeDeployPanelURL(resolveDeployPanelURL(c, h.currentPublicURL(), c.Query("panel_url"), nodeData.PanelURL))
	if panelURL == "" || isLoopbackOrUnspecifiedDeployURL(panelURL) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Panel URL 无效：必须是公网可访问的 https://panel.example.com 地址，不能使用本地回环地址",
		})
		return
	}

	// Generate script
	script := h.deployService.GetDeployScript(panelURL, nodeData.Token, nodeData.Port)
	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       "node_deploy_script_download",
		ResourceType: "node",
		ResourceID:   strconv.FormatInt(nodeID, 10),
	})

	c.Header("Content-Type", "text/plain")
	c.Header("Content-Disposition", "attachment; filename=install-agent.sh")
	c.Header("Cache-Control", "no-store")
	c.String(http.StatusOK, script)
}

func isLoopbackOrUnspecifiedDeployURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0"
}

func (h *NodeDeployHandler) currentPublicURL() string {
	if h != nil && h.publicURL != nil {
		return h.publicURL()
	}
	if h == nil || h.config == nil {
		return ""
	}
	return h.config.Server.PublicURL
}

// TestConnection tests SSH connection to a remote server.
// POST /api/admin/nodes/test-connection
func (h *NodeDeployHandler) TestConnection(c *gin.Context) {
	if h.deployService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "SSH deploy service unavailable",
		})
		return
	}

	var req DeployAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate authentication method
	if req.Password == "" && req.PrivateKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Either password or private_key must be provided",
		})
		return
	}
	if req.PrivateKey != "" && !validNodeSSHPrivateKey(req.PrivateKey) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid SSH private key format",
		})
		return
	}

	deployConfig := &node.DeployConfig{
		Host:       req.Host,
		Port:       req.Port,
		Username:   req.Username,
		Password:   req.Password,
		PrivateKey: req.PrivateKey,
	}

	// Try to connect using a test deployment
	err := h.deployService.TestConnection(c.Request.Context(), deployConfig)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Connection failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Connection successful",
	})
}
