// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/config"
	"v/internal/logger"
	"v/internal/node"
)

// NodeDeployHandler handles node deployment requests.
type NodeDeployHandler struct {
	deployService *node.RemoteDeployService
	nodeService   *node.Service
	config        *config.Config
	logger        logger.Logger
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

// DeployAgentRequest represents a deploy agent request.
type DeployAgentRequest struct {
	Host       string `json:"host" binding:"required"`
	Port       int    `json:"port"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	PanelURL   string `json:"panel_url"` // Panel 服务器地址（可选，优先使用此值）
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
			logger.F("node_id", nodeID),
			logger.F("token_length", len(nodeData.Token)),
			logger.F("token_prefix", nodeData.Token[:min(8, len(nodeData.Token))]))
	}

	// Get panel URL - 优先级：节点保存的 PanelURL > 请求参数 > 配置文件 > 请求头 > 自动检测
	panelURL := nodeData.PanelURL // 优先使用节点创建时保存的 Panel URL
	if panelURL == "" {
		panelURL = req.PanelURL
	}
	if panelURL == "" {
		panelURL = h.config.Server.PublicURL
	}
	if panelURL == "" {
		panelURL = c.Request.Header.Get("X-Panel-URL")
	}
	if panelURL == "" {
		// 自动从请求中获取（使用 X-Forwarded-Host 或 Host）
		scheme := "http"
		if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}

		// 优先使用 X-Forwarded-Host（反向代理场景）
		host := c.Request.Header.Get("X-Forwarded-Host")
		if host == "" {
			host = c.Request.Host
		}

		panelURL = scheme + "://" + host

		h.logger.Info("Auto-detected Panel URL from request",
			logger.F("panel_url", panelURL),
			logger.F("request_host", c.Request.Host),
			logger.F("x_forwarded_host", c.Request.Header.Get("X-Forwarded-Host")),
			logger.F("x_forwarded_proto", c.Request.Header.Get("X-Forwarded-Proto")))
	}

	// 验证 Panel URL 不能是 localhost 或 127.0.0.1
	if strings.Contains(panelURL, "localhost") || strings.Contains(panelURL, "127.0.0.1") {
		h.logger.Error("Invalid Panel URL for remote deployment",
			logger.F("panel_url", panelURL),
			logger.F("request_host", c.Request.Host),
			logger.F("x_forwarded_host", c.Request.Header.Get("X-Forwarded-Host")))
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Panel URL 配置错误：远程部署不能使用 localhost 或 127.0.0.1，请配置 server.public_url 为公网 IP 或域名",
		})
		return
	}

	h.logger.Info("Using Panel URL for deployment",
		logger.F("node_id", nodeID),
		logger.F("panel_url", panelURL))

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

	// Deploy agent
	result, err := h.deployService.Deploy(c.Request.Context(), deployConfig)
	if err != nil {
		h.logger.Error("Agent deployment failed",
			logger.F("node_id", nodeID),
			logger.F("error", err.Error()))

		// 返回 200 状态码，但 success 为 false，这样前端可以正确显示错误信息和日志
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": result.Message,
			"steps":   result.Steps,
			"logs":    result.Logs,
		})
		return
	}

	h.logger.Info("Agent deployed successfully",
		logger.F("node_id", nodeID),
		logger.F("host", req.Host))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": result.Message,
		"steps":   result.Steps,
		"logs":    result.Logs,
	})
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

	// Get panel URL from config or query parameter
	panelURL := h.config.Server.PublicURL
	if queryURL := c.Query("panel_url"); queryURL != "" {
		panelURL = queryURL
	}
	if panelURL == "" {
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		panelURL = scheme + "://" + c.Request.Host
	}

	// Generate script
	script := h.deployService.GetDeployScript(panelURL, nodeData.Token, nodeData.Port)

	c.Header("Content-Type", "text/plain")
	c.Header("Content-Disposition", "attachment; filename=install-agent.sh")
	c.String(http.StatusOK, script)
}

// TestConnection tests SSH connection to a remote server.
// POST /api/admin/nodes/test-connection
func (h *NodeDeployHandler) TestConnection(c *gin.Context) {
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
