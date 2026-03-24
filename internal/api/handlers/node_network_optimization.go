package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

// NodeNetworkOptimizationHandler handles node network optimization APIs.
type NodeNetworkOptimizationHandler struct {
	nodeRepo         repository.NodeRepository
	deployService    *node.RemoteDeployService
	recoveryTracker  *NodeRecoveryTracker
	logger           logger.Logger
}

// NewNodeNetworkOptimizationHandler creates a new handler.
func NewNodeNetworkOptimizationHandler(
	nodeRepo repository.NodeRepository,
	deployService *node.RemoteDeployService,
	recoveryTracker *NodeRecoveryTracker,
	log logger.Logger,
) *NodeNetworkOptimizationHandler {
	return &NodeNetworkOptimizationHandler{
		nodeRepo:        nodeRepo,
		deployService:   deployService,
		recoveryTracker: recoveryTracker,
		logger:          log,
	}
}

type nodeNetworkOptimizationSSHInput struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
}

type nodeNetworkOptimizationInspectRequest struct {
	SSH *nodeNetworkOptimizationSSHInput `json:"ssh"`
}

type nodeNetworkOptimizationApplyRequest struct {
	SSH      *nodeNetworkOptimizationSSHInput `json:"ssh"`
	Settings node.NetworkOptimizationSettings `json:"settings"`
}

type nodeNetworkOptimizationRollbackRequest struct {
	SSH *nodeNetworkOptimizationSSHInput `json:"ssh"`
}

func (h *NodeNetworkOptimizationHandler) GetProfile(c *gin.Context) {
	nodeData, err := h.getRepositoryNode(c)
	if err != nil {
		return
	}

	savedSettings := node.ParseNetworkOptimizationSettings(nodeData.NetworkOptimizationSettings)
	defaultHost, defaultPort, defaultUser := defaultNodeSSH(nodeData)

	c.JSON(http.StatusOK, gin.H{
		"node_id":              nodeData.ID,
		"has_saved_settings":   strings.TrimSpace(nodeData.NetworkOptimizationSettings) != "",
		"saved_settings":       savedSettings,
		"recommended_settings": node.RecommendedNetworkOptimizationSettings(),
		"ssh_defaults": gin.H{
			"host":             defaultHost,
			"port":             defaultPort,
			"username":         defaultUser,
			"has_saved_password": strings.TrimSpace(nodeData.SSHPassword) != "",
			"has_saved_private_key": strings.TrimSpace(nodeData.SSHKeyPath) != "",
		},
		"backup_path": node.NetworkOptimizationBackupPath,
	})
}

func (h *NodeNetworkOptimizationHandler) Inspect(c *gin.Context) {
	if h.deployService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSH deploy service unavailable"})
		return
	}

	nodeData, err := h.getRepositoryNode(c)
	if err != nil {
		return
	}

	var req nodeNetworkOptimizationInspectRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	deployConfig, err := buildNodeDeployConfig(nodeData, req.SSH)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state, logs, err := h.deployService.InspectNetworkOptimization(c.Request.Context(), deployConfig)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
			"logs":  logs,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"state":          state,
		"logs":           logs,
		"saved_settings": node.ParseNetworkOptimizationSettings(nodeData.NetworkOptimizationSettings),
	})
}

func (h *NodeNetworkOptimizationHandler) Apply(c *gin.Context) {
	if h.deployService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSH deploy service unavailable"})
		return
	}

	nodeData, err := h.getRepositoryNode(c)
	if err != nil {
		return
	}

	var req nodeNetworkOptimizationApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	settings := req.Settings.Normalize()
	if settings.IsEmpty() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少启用一项网络优化设置"})
		return
	}

	deployConfig, err := buildNodeDeployConfig(nodeData, req.SSH)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deployService.ApplyNetworkOptimization(c.Request.Context(), deployConfig, settings)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
			"logs":  resultLog(result),
		})
		return
	}

	encodedSettings, err := json.Marshal(settings)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encode network optimization settings"})
		return
	}

	nodeData.NetworkOptimizationSettings = string(encodedSettings)
	if err := h.nodeRepo.Update(c.Request.Context(), nodeData); err != nil {
		h.logger.Error("Failed to persist node network optimization settings", logger.Err(err), logger.F("node_id", nodeData.ID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "远端优化已执行，但保存面板配置失败",
			"result": result,
		})
		return
	}

	command, queued := h.queueConfigSync(nodeData.ID, "管理员应用节点网络优化后同步配置")

	c.JSON(http.StatusOK, gin.H{
		"message":       "节点网络优化已应用",
		"result":        result,
		"sync_queued":   queued,
		"command_id":    command.ID,
		"saved_settings": settings,
	})
}

func (h *NodeNetworkOptimizationHandler) Rollback(c *gin.Context) {
	if h.deployService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "SSH deploy service unavailable"})
		return
	}

	nodeData, err := h.getRepositoryNode(c)
	if err != nil {
		return
	}

	var req nodeNetworkOptimizationRollbackRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	deployConfig, err := buildNodeDeployConfig(nodeData, req.SSH)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.deployService.RollbackNetworkOptimization(c.Request.Context(), deployConfig)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": err.Error(),
			"logs":  resultLog(result),
		})
		return
	}

	nodeData.NetworkOptimizationSettings = ""
	if err := h.nodeRepo.Update(c.Request.Context(), nodeData); err != nil {
		h.logger.Error("Failed to clear node network optimization settings", logger.Err(err), logger.F("node_id", nodeData.ID))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "远端回滚已执行，但面板配置清理失败",
			"result": result,
		})
		return
	}

	command, queued := h.queueConfigSync(nodeData.ID, "管理员回滚节点网络优化后同步配置")

	c.JSON(http.StatusOK, gin.H{
		"message":     "节点网络优化已回滚",
		"result":      result,
		"sync_queued": queued,
		"command_id":  command.ID,
	})
}

func (h *NodeNetworkOptimizationHandler) getRepositoryNode(c *gin.Context) (*repository.Node, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return nil, err
	}

	nodeData, err := h.nodeRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return nil, err
	}

	return nodeData, nil
}

func (h *NodeNetworkOptimizationHandler) queueConfigSync(nodeID int64, reason string) (Command, bool) {
	if h.recoveryTracker == nil {
		return Command{}, false
	}
	return h.recoveryTracker.QueueConfigSyncCommandDetailed(nodeID, "admin", reason)
}

func buildNodeDeployConfig(nodeData *repository.Node, input *nodeNetworkOptimizationSSHInput) (*node.DeployConfig, error) {
	if nodeData == nil {
		return nil, fmt.Errorf("node data is required")
	}

	host, port, username := defaultNodeSSH(nodeData)
	password := strings.TrimSpace(nodeData.SSHPassword)
	privateKey := ""

	if strings.TrimSpace(nodeData.SSHKeyPath) != "" {
		if data, err := os.ReadFile(strings.TrimSpace(nodeData.SSHKeyPath)); err == nil {
			privateKey = string(data)
		} else if password == "" {
			return nil, fmt.Errorf("读取节点 SSH 私钥失败: %w", err)
		}
	}

	if input != nil {
		if trimmed := strings.TrimSpace(input.Host); trimmed != "" {
			host = trimmed
		}
		if input.Port > 0 {
			port = input.Port
		}
		if trimmed := strings.TrimSpace(input.Username); trimmed != "" {
			username = trimmed
		}
		if trimmed := strings.TrimSpace(input.Password); trimmed != "" {
			password = trimmed
		}
		if trimmed := strings.TrimSpace(input.PrivateKey); trimmed != "" {
			privateKey = input.PrivateKey
		}
	}

	if host == "" {
		return nil, fmt.Errorf("请提供 SSH 主机地址")
	}
	if username == "" {
		username = "root"
	}
	if port == 0 {
		port = 22
	}
	if strings.TrimSpace(password) == "" && strings.TrimSpace(privateKey) == "" {
		return nil, fmt.Errorf("请提供 SSH 密码或私钥")
	}

	return &node.DeployConfig{
		NodeID:     nodeData.ID,
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
		PanelURL:   nodeData.PanelURL,
		NodeToken:  nodeData.Token,
	}, nil
}

func defaultNodeSSH(nodeData *repository.Node) (string, int, string) {
	host := strings.TrimSpace(nodeData.SSHHost)
	if host == "" {
		host = strings.TrimSpace(nodeData.Address)
	}

	port := nodeData.SSHPort
	if port == 0 {
		port = 22
	}

	username := strings.TrimSpace(nodeData.SSHUser)
	if username == "" {
		username = "root"
	}

	return host, port, username
}

func bindOptionalJSON(c *gin.Context, target any) error {
	if c.Request.ContentLength == 0 {
		return nil
	}
	return c.ShouldBindJSON(target)
}

func resultLog(result *node.NetworkOptimizationExecutionResult) string {
	if result == nil {
		return ""
	}
	return result.Log
}
