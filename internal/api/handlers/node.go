// Package handlers provides HTTP request handlers for the V Panel API.
package handlers

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/agent"
	"v/internal/api/middleware"
	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
	"v/internal/monitor"
	"v/internal/node"
	"v/pkg/errors"
)

// NodeHandler handles node management API requests.
type NodeHandler struct {
	nodeService     *node.Service
	groupService    *node.GroupService
	deployService   *node.RemoteDeployService
	entitlementSvc  *entitlement.Service
	recoveryTracker *NodeRecoveryTracker
	certRepo        repository.CertificateRepository
	certSvc         CertificateService
	httpClient      *http.Client
	cache           cache.Cache
	auditService    monitor.AuditService
	publicURL       func() string
	logger          logger.Logger

	// In-progress tracking for async diagnosis
	diagInProgressMu sync.RWMutex
	diagInProgress   map[int64]bool
}

type queueNodeCommandFunc func(nodeID int64, source, reason string) (Command, bool)

// NewNodeHandler creates a new node handler.
func NewNodeHandler(nodeService *node.Service, groupService *node.GroupService, deployService *node.RemoteDeployService, recoveryTracker *NodeRecoveryTracker, log logger.Logger) *NodeHandler {
	return &NodeHandler{
		nodeService:     nodeService,
		groupService:    groupService,
		deployService:   deployService,
		recoveryTracker: recoveryTracker,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
		logger:          log,
		diagInProgress:  make(map[int64]bool),
	}
}

// WithCache sets the cache for the node handler.
func (h *NodeHandler) WithCache(c cache.Cache) *NodeHandler {
	h.cache = c
	return h
}

// WithEntitlementService enables proactive proxy provisioning after successful node deployment.
func (h *NodeHandler) WithEntitlementService(svc *entitlement.Service) *NodeHandler {
	h.entitlementSvc = svc
	return h
}

// WithAuditService wires the audit emitter for state-changing node ops.
func (h *NodeHandler) WithAuditService(audit monitor.AuditService) *NodeHandler {
	h.auditService = audit
	return h
}

// WithPublicURL sets the preferred externally reachable panel URL for agent deployment.
func (h *NodeHandler) WithPublicURL(publicURL string) *NodeHandler {
	h.publicURL = func() string { return publicURL }
	return h
}

// WithPublicURLProvider sets the runtime panel URL source for agent deployment.
func (h *NodeHandler) WithPublicURLProvider(provider func() string) *NodeHandler {
	h.publicURL = provider
	return h
}

// WithCertificateAutomation enables TLS-domain based certificate matching and
// deployment for node saves.
func (h *NodeHandler) WithCertificateAutomation(repo repository.CertificateRepository, svc CertificateService) *NodeHandler {
	h.certRepo = repo
	h.certSvc = svc
	return h
}

func provisionNodeProxiesAfterDeploy(svc *entitlement.Service, log logger.Logger, nodeID int64) {
	if svc == nil || nodeID <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := svc.ProvisionNodeProxies(ctx, nodeID)
	if err != nil {
		log.Warn("failed to provision proxies for newly deployed node",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return
	}
	if result == nil {
		return
	}
	log.Info("provisioned proxies for newly deployed node",
		logger.F("node_id", result.NodeID),
		logger.F("scanned_users", result.ScannedUsers),
		logger.F("entitled_users", result.EntitledUsers),
		logger.F("created", result.Created),
		logger.F("existing", result.Existing),
		logger.F("skipped", result.Skipped))
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
	GroupID  *int64  `json:"group_id,omitempty"`
	GroupIDs []int64 `json:"group_ids,omitempty"`

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
// Token is shown only once at creation time.
type NodeWithTokenResponse struct {
	NodeResponse
	Token string `json:"token"`
}

// TrafficDiagnosticResponse represents traffic collection diagnostics for a node.
type TrafficDiagnosticResponse struct {
	NodeID           int64                         `json:"node_id"`
	NodeName         string                        `json:"node_name"`
	Address          string                        `json:"address"`
	Port             int                           `json:"port"`
	DiagnosticStatus string                        `json:"diagnostic_status"`
	Message          string                        `json:"message"`
	Traffic          *agent.TrafficCollectorStatus `json:"traffic,omitempty"`
}

// MaskedTokenResponse includes a masked token for display after initial creation.
type MaskedTokenResponse struct {
	NodeResponse
	TokenPrefix string `json:"token_prefix"`
	TokenHint   string `json:"token_hint"`
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
	TrafficLimit   int64  `json:"traffic_limit"`
	TrafficResetAt string `json:"traffic_reset_at,omitempty"`
	SpeedLimit     int64  `json:"speed_limit"`

	// 协议支持
	Protocols []string `json:"protocols"`

	// TLS 配置
	TLSEnabled bool   `json:"tls_enabled"`
	TLSDomain  string `json:"tls_domain"`

	// 节点分组
	GroupID  *int64  `json:"group_id"`
	GroupIDs []int64 `json:"group_ids"`

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

	// SSH 自动安装配置（可选）
	SSH *SSHConfig `json:"ssh,omitempty"`

	// 流量和速率
	TrafficLimit   *int64  `json:"traffic_limit"`
	TrafficResetAt *string `json:"traffic_reset_at"`
	SpeedLimit     *int64  `json:"speed_limit"`

	// 协议支持
	Protocols *[]string `json:"protocols"`

	// TLS 配置
	TLSEnabled *bool   `json:"tls_enabled"`
	TLSDomain  *string `json:"tls_domain"`

	// 节点分组
	GroupID  *int64   `json:"group_id"`
	GroupIDs *[]int64 `json:"group_ids"`

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
		GroupID:  n.GroupID,
		GroupIDs: n.Groups, // Use preloaded groups from repository

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

func parseOptionalRFC3339Time(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func normalizeNodeGroupIDs(groupIDs []int64, fallback *int64) []int64 {
	seen := make(map[int64]struct{}, len(groupIDs)+1)
	normalized := make([]int64, 0, len(groupIDs)+1)

	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, exists := seen[groupID]; exists {
			continue
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}

	if len(normalized) == 0 && fallback != nil && *fallback > 0 {
		normalized = append(normalized, *fallback)
	}

	return normalized
}

func certificateDomainCandidates(domain string) []string {
	rawDomain := strings.TrimSpace(strings.ToLower(domain))
	if rawDomain == "" {
		return nil
	}

	seen := make(map[string]struct{}, 2)
	candidates := make([]string, 0, 2)
	add := func(candidate string) {
		candidate = strings.TrimSpace(strings.ToLower(candidate))
		if candidate == "" {
			return
		}
		if _, ok := seen[candidate]; ok {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	if strings.HasPrefix(rawDomain, "*.") {
		add(rawDomain)
		add(strings.TrimPrefix(rawDomain, "*."))
		return candidates
	}

	add(strings.TrimPrefix(rawDomain, "*."))
	parts := strings.Split(strings.TrimPrefix(rawDomain, "*."), ".")
	if len(parts) >= 3 {
		add("*." + strings.Join(parts[1:], "."))
	}
	return candidates
}

func certificateHasUsableMaterial(cert *repository.Certificate) bool {
	if cert == nil {
		return false
	}
	return (strings.TrimSpace(cert.CertPath) != "" && strings.TrimSpace(cert.KeyPath) != "") ||
		(strings.TrimSpace(cert.Certificate) != "" && strings.TrimSpace(cert.PrivateKey) != "")
}

func certificateIsUsable(cert *repository.Certificate) bool {
	if cert == nil || cert.Status != "active" || !certificateHasUsableMaterial(cert) {
		return false
	}
	expiresAt := cert.ExpiresAt
	if expiresAt.IsZero() && cert.ExpireDate != nil {
		expiresAt = *cert.ExpireDate
	}
	return expiresAt.IsZero() || expiresAt.After(time.Now())
}

func (h *NodeHandler) matchCertificateForTLSDomain(ctx context.Context, tlsEnabled bool, tlsDomain string) *repository.Certificate {
	if !tlsEnabled || h.certRepo == nil {
		return nil
	}

	for _, candidate := range certificateDomainCandidates(tlsDomain) {
		cert, err := h.certRepo.GetByDomain(ctx, candidate)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			h.logger.Warn("failed to auto match node TLS certificate",
				logger.Err(err),
				logger.F("tls_domain", tlsDomain),
				logger.F("candidate", candidate))
			continue
		}
		if !certificateIsUsable(cert) {
			h.logger.Warn("auto matched certificate is not usable",
				logger.F("tls_domain", tlsDomain),
				logger.F("candidate", candidate),
				logger.F("status", cert.Status))
			continue
		}
		return cert
	}
	return nil
}

func (h *NodeHandler) resolveNodeCertificateID(ctx context.Context, tlsEnabled bool, tlsDomain string, requestedID *int64, existingID *int64) (*int64, bool) {
	if requestedID != nil && *requestedID > 0 {
		return requestedID, false
	}
	if existingID != nil && *existingID > 0 {
		return existingID, false
	}

	cert := h.matchCertificateForTLSDomain(ctx, tlsEnabled, tlsDomain)
	if cert == nil {
		return requestedID, false
	}

	certID := cert.ID
	h.logger.Info("auto matched node TLS certificate",
		logger.F("tls_domain", tlsDomain),
		logger.F("cert_id", certID),
		logger.F("cert_domain", cert.Domain))
	return &certID, true
}

func nodeCertificateIDValue(certificateID *int64) int64 {
	if certificateID == nil {
		return 0
	}
	return *certificateID
}

func (h *NodeHandler) triggerCertificateDeployAsync(certID int64, nodeID int64, reason string) {
	if certID <= 0 || h.certSvc == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if err := h.certSvc.DeployToAssignedNodes(ctx, certID); err != nil {
			h.logger.Warn("failed to deploy auto matched certificate",
				logger.Err(err),
				logger.F("cert_id", certID),
				logger.F("node_id", nodeID),
				logger.F("reason", reason))
			return
		}
		h.logger.Info("auto matched certificate deployment triggered",
			logger.F("cert_id", certID),
			logger.F("node_id", nodeID),
			logger.F("reason", reason))
	}()
}

func nodeMutationErrorResponse(err error, fallbackMessage string) (int, gin.H) {
	message := err.Error()
	for _, prefix := range []string{
		node.ErrDuplicateNode.Error() + ": ",
		node.ErrInvalidNode.Error() + ": ",
		node.ErrInvalidAddress.Error() + ": ",
	} {
		if strings.HasPrefix(message, prefix) {
			message = strings.TrimPrefix(message, prefix)
			break
		}
	}

	switch {
	case stderrors.Is(err, node.ErrDuplicateNode):
		return http.StatusConflict, gin.H{"error": message}
	case stderrors.Is(err, node.ErrInvalidAddress), stderrors.Is(err, node.ErrInvalidNode):
		return http.StatusBadRequest, gin.H{"error": message}
	case stderrors.Is(err, node.ErrNodeNotFound):
		return http.StatusNotFound, gin.H{"error": "Node not found"}
	default:
		return http.StatusInternalServerError, gin.H{"error": fallbackMessage}
	}
}

func (h *NodeHandler) populateNodeGroupIDs(ctx context.Context, resp *NodeResponse) {
	if resp == nil {
		return
	}

	// If groups are already loaded (from Preload), just normalize them
	if len(resp.GroupIDs) > 0 {
		resp.GroupIDs = normalizeNodeGroupIDs(resp.GroupIDs, resp.GroupID)
		if len(resp.GroupIDs) > 0 && resp.GroupID == nil {
			resp.GroupID = &resp.GroupIDs[0]
		}
		return
	}

	resp.GroupIDs = normalizeNodeGroupIDs(resp.GroupIDs, resp.GroupID)
	if h == nil || h.groupService == nil {
		return
	}

	// Fallback: load groups from database if not preloaded
	groups, err := h.groupService.GetGroupsForNode(ctx, resp.ID)
	if err != nil {
		return
	}

	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.ID <= 0 {
			continue
		}
		groupIDs = append(groupIDs, group.ID)
	}

	resp.GroupIDs = normalizeNodeGroupIDs(groupIDs, resp.GroupID)
	if len(resp.GroupIDs) > 0 {
		resp.GroupID = &resp.GroupIDs[0]
	}
}

func (h *NodeHandler) buildNodeResponse(ctx context.Context, n *node.Node) *NodeResponse {
	resp := toNodeResponse(n)
	h.populateNodeGroupIDs(ctx, resp)
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

func normalizeTrafficDiagnosticPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func nodeAgentTrafficStatusURL(address string, port int) (string, error) {
	host := strings.TrimSpace(address)
	if host == "" {
		return "", fmt.Errorf("node address is empty")
	}
	if port <= 0 {
		port = 18443
	}
	return fmt.Sprintf("http://%s/traffic/status", net.JoinHostPort(host, strconv.Itoa(port))), nil
}

func nodeTrafficDiagnosticStatus(
	collector *agent.TrafficCollectorStatus,
	statusCode int,
	fetchErr error,
) string {
	if fetchErr != nil {
		if statusCode == http.StatusOK {
			return agent.TrafficCollectorStatusCollectorError
		}
		return "agent_unreachable"
	}
	if statusCode == http.StatusNotFound {
		return "agent_no_traffic_endpoint"
	}
	if statusCode != http.StatusOK {
		return agent.TrafficCollectorStatusCollectorError
	}
	if collector == nil {
		return agent.TrafficCollectorStatusCollectorError
	}
	if !collector.XrayRunning {
		return "xray_not_running"
	}
	switch collector.Status {
	case agent.TrafficCollectorStatusHealthyCollecting:
		return agent.TrafficCollectorStatusHealthyCollecting
	case agent.TrafficCollectorStatusHealthyIdle:
		return agent.TrafficCollectorStatusHealthyIdle
	default:
		return agent.TrafficCollectorStatusCollectorError
	}
}

func hasTrafficConfigPathMismatch(collector *agent.TrafficCollectorStatus) bool {
	if collector == nil {
		return false
	}
	configured := normalizeTrafficDiagnosticPath(collector.ConfiguredConfigPath)
	resolved := normalizeTrafficDiagnosticPath(collector.ResolvedConfigPath)
	return configured != "" && resolved != "" && configured != resolved
}

func trafficConfigMismatchMessage(collector *agent.TrafficCollectorStatus) string {
	if !hasTrafficConfigPathMismatch(collector) {
		return ""
	}
	return fmt.Sprintf(
		"agent 配置路径与运行中的 Xray 配置路径不一致，当前使用 %s 采集流量",
		normalizeTrafficDiagnosticPath(collector.ResolvedConfigPath),
	)
}

func nodeTrafficDiagnosticMessage(
	diagnosticStatus string,
	collector *agent.TrafficCollectorStatus,
	statusCode int,
	fetchErr error,
) string {
	if fetchErr != nil {
		if statusCode == http.StatusOK {
			return fmt.Sprintf("无法解析节点返回的流量诊断数据: %v", fetchErr)
		}
		return fmt.Sprintf("无法连接到节点 Agent，请检查地址、端口和防火墙: %v", fetchErr)
	}

	if mismatch := trafficConfigMismatchMessage(collector); mismatch != "" &&
		(diagnosticStatus == agent.TrafficCollectorStatusHealthyCollecting ||
			diagnosticStatus == agent.TrafficCollectorStatusHealthyIdle ||
			diagnosticStatus == agent.TrafficCollectorStatusCollectorError) {
		return mismatch
	}

	switch diagnosticStatus {
	case "agent_no_traffic_endpoint":
		return "节点 Agent 可达，但当前版本未提供 /traffic/status 诊断接口"
	case "xray_not_running":
		return "节点 Agent 可达，但 Xray 当前未运行"
	case agent.TrafficCollectorStatusCollectorError:
		if collector != nil && strings.TrimSpace(collector.LastError) != "" {
			return collector.LastError
		}
		if statusCode != http.StatusOK && statusCode > 0 {
			return fmt.Sprintf("节点 Agent 返回异常状态码 %d", statusCode)
		}
		return "流量采集器报告异常"
	case agent.TrafficCollectorStatusHealthyIdle:
		return "流量采集器运行正常，当前没有新的用户流量记录"
	case agent.TrafficCollectorStatusHealthyCollecting:
		if collector != nil {
			return fmt.Sprintf("流量采集器运行正常，最近一次采集记录数 %d", collector.LastRecordCount)
		}
		return "流量采集器运行正常，最近一次采集已有流量记录"
	case "agent_unreachable":
		return "无法连接到节点 Agent，请检查节点地址、端口和防火墙"
	default:
		return "流量采集状态未知"
	}
}

// diagCacheKey generates the cache key for node diagnosis results.
func diagCacheKey(nodeID int64) string {
	return fmt.Sprintf("node:diag:%d", nodeID)
}

// isDiagInProgress checks if a diagnosis is currently in progress for the given node.
func (h *NodeHandler) isDiagInProgress(nodeID int64) bool {
	h.diagInProgressMu.RLock()
	defer h.diagInProgressMu.RUnlock()
	return h.diagInProgress[nodeID]
}

// setDiagInProgress sets the in-progress status for a node diagnosis.
func (h *NodeHandler) setDiagInProgress(nodeID int64, inProgress bool) {
	h.diagInProgressMu.Lock()
	defer h.diagInProgressMu.Unlock()
	if inProgress {
		h.diagInProgress[nodeID] = true
	} else {
		delete(h.diagInProgress, nodeID)
	}
}

func (h *NodeHandler) fetchTrafficCollectorStatus(
	ctx context.Context,
	n *node.Node,
) (*agent.TrafficCollectorStatus, int, error) {
	if n == nil {
		return nil, 0, fmt.Errorf("node is nil")
	}

	url, err := nodeAgentTrafficStatusURL(n.Address, n.Port)
	if err != nil {
		return nil, 0, err
	}

	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, resp.StatusCode, nil
	}

	var collector agent.TrafficCollectorStatus
	if err := json.NewDecoder(resp.Body).Decode(&collector); err != nil {
		return nil, resp.StatusCode, err
	}

	return &collector, resp.StatusCode, nil
}

func (h *NodeHandler) buildTrafficDiagnosticResponse(
	ctx context.Context,
	n *node.Node,
) *TrafficDiagnosticResponse {
	// Try to get cached result first
	if h.cache != nil {
		cacheKey := diagCacheKey(n.ID)
		if cachedData, err := h.cache.Get(ctx, cacheKey); err == nil {
			var cachedResp TrafficDiagnosticResponse
			if err := json.Unmarshal(cachedData, &cachedResp); err == nil {
				// Return cached result immediately
				return &cachedResp
			}
		}
	}

	// Trigger async update if not already in progress
	if h.cache != nil && !h.isDiagInProgress(n.ID) {
		go h.runDiagnosisAsync(n)
	}

	// Return empty/placeholder result for first request
	// or when cache is not available, fall back to synchronous call
	if h.cache == nil {
		collector, statusCode, fetchErr := h.fetchTrafficCollectorStatus(ctx, n)
		diagnosticStatus := nodeTrafficDiagnosticStatus(collector, statusCode, fetchErr)
		return &TrafficDiagnosticResponse{
			NodeID:           n.ID,
			NodeName:         n.Name,
			Address:          n.Address,
			Port:             n.Port,
			DiagnosticStatus: diagnosticStatus,
			Message:          nodeTrafficDiagnosticMessage(diagnosticStatus, collector, statusCode, fetchErr),
			Traffic:          collector,
		}
	}

	// Return placeholder response indicating diagnosis is in progress
	return &TrafficDiagnosticResponse{
		NodeID:           n.ID,
		NodeName:         n.Name,
		Address:          n.Address,
		Port:             n.Port,
		DiagnosticStatus: "pending",
		Message:          "诊断正在进行中，请稍后刷新",
		Traffic:          nil,
	}
}

// runDiagnosisAsync performs the diagnosis in the background and caches the result.
func (h *NodeHandler) runDiagnosisAsync(n *node.Node) {
	// Mark as in progress
	h.setDiagInProgress(n.ID, true)
	defer h.setDiagInProgress(n.ID, false)

	// Create a background context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch the diagnosis result
	collector, statusCode, fetchErr := h.fetchTrafficCollectorStatus(ctx, n)
	diagnosticStatus := nodeTrafficDiagnosticStatus(collector, statusCode, fetchErr)

	response := &TrafficDiagnosticResponse{
		NodeID:           n.ID,
		NodeName:         n.Name,
		Address:          n.Address,
		Port:             n.Port,
		DiagnosticStatus: diagnosticStatus,
		Message:          nodeTrafficDiagnosticMessage(diagnosticStatus, collector, statusCode, fetchErr),
		Traffic:          collector,
	}

	// Cache the result with 5-minute TTL
	if h.cache != nil {
		cacheKey := diagCacheKey(n.ID)
		if data, err := json.Marshal(response); err == nil {
			if err := h.cache.Set(ctx, cacheKey, data, 5*time.Minute); err != nil {
				h.logger.Warn("Failed to cache diagnosis result",
					logger.Err(err),
					logger.F("node_id", n.ID))
			}
		}
	}
}

// List returns all nodes with optional filtering.
// GET /api/admin/nodes
func (h *NodeHandler) List(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	region := c.Query("region")
	search := c.Query("search")

	// Cap limit to prevent database resource exhaustion
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	filter := node.NodeFilter{
		Status: status,
		Region: region,
		Search: search,
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
		response[i] = h.buildNodeResponse(c.Request.Context(), n)
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

	c.JSON(http.StatusOK, h.buildNodeResponse(c.Request.Context(), n))
}

// GetTrafficDiagnostic returns the live traffic collection diagnostic for a node.
// GET /api/admin/nodes/:id/traffic-diagnostic
func (h *NodeHandler) GetTrafficDiagnostic(c *gin.Context) {
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
		h.logger.Error("Failed to get node for traffic diagnostic", logger.Err(err), logger.F("id", id))
		middleware.HandleInternalError(c, errors.MsgNodeNotFound, err)
		return
	}

	c.JSON(http.StatusOK, h.buildTrafficDiagnosticResponse(c.Request.Context(), n))
}

// Create creates a new node.
// POST /api/admin/nodes
func (h *NodeHandler) Create(c *gin.Context) {
	var req CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.HandleBadRequest(c, errors.MsgInvalidRequest)
		return
	}

	trafficResetAt, err := parseOptionalRFC3339Time(req.TrafficResetAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid traffic_reset_at, must be RFC3339"})
		return
	}

	groupIDs := normalizeNodeGroupIDs(req.GroupIDs, req.GroupID)
	var primaryGroupID *int64
	if len(groupIDs) > 0 {
		primaryGroupID = &groupIDs[0]
	}

	certificateID, autoMatchedCertificate := h.resolveNodeCertificateID(c.Request.Context(), req.TLSEnabled, req.TLSDomain, req.CertificateID, nil)

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
		TrafficLimit:   req.TrafficLimit,
		TrafficResetAt: trafficResetAt,
		SpeedLimit:     req.SpeedLimit,

		// 协议支持
		Protocols: req.Protocols,

		// TLS 配置
		TLSEnabled: req.TLSEnabled,
		TLSDomain:  req.TLSDomain,

		// 节点分组
		GroupID: primaryGroupID,

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
		CertificateID: certificateID,
	}

	n, err := h.nodeService.Create(c.Request.Context(), createReq)
	if err != nil {
		h.logger.Error("Failed to create node", logger.Err(err))
		status, payload := nodeMutationErrorResponse(err, "Failed to create node")
		c.JSON(status, payload)
		return
	}

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeCreate,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(n.ID, 10),
		Details:      map[string]any{"name": n.Name, "address": n.Address, "region": n.Region},
	})

	if len(groupIDs) > 0 && h.groupService != nil {
		if err := h.groupService.SyncNodeGroups(c.Request.Context(), n.ID, groupIDs); err != nil {
			h.logger.Error("Failed to sync node groups after create", logger.Err(err), logger.F("node_id", n.ID))
			if cleanupErr := h.nodeService.Delete(c.Request.Context(), n.ID); cleanupErr != nil {
				h.logger.Error("Failed to cleanup node after group sync failure", logger.Err(cleanupErr), logger.F("node_id", n.ID))
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign node groups"})
			return
		}
		n, err = h.nodeService.GetByID(c.Request.Context(), n.ID)
		if err != nil {
			h.logger.Error("Failed to reload node after group sync", logger.Err(err), logger.F("node_id", n.ID))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload created node"})
			return
		}
	} else if primaryGroupID == nil && h.groupService != nil {
		// Ensure old memberships are not left behind if this endpoint is ever reused with a pre-created record.
		if err := h.groupService.SyncNodeGroups(c.Request.Context(), n.ID, nil); err != nil {
			h.logger.Warn("Failed to clear empty node groups after create", logger.Err(err), logger.F("node_id", n.ID))
		}
	}

	h.logger.Info("Node created", logger.F("node_id", n.ID), logger.F("name", n.Name))

	if req.SSH != nil {
		if err := persistNodeSSHMetadata(c.Request.Context(), h.nodeService, n.ID, nodeSSHMetadataInput{
			Host:       req.SSH.Host,
			Port:       req.SSH.Port,
			Username:   req.SSH.Username,
			Password:   req.SSH.Password,
			PrivateKey: req.SSH.PrivateKey,
		}); err != nil {
			h.logger.Warn("Failed to persist node SSH metadata after create", logger.Err(err), logger.F("node_id", n.ID))
		} else {
			n, err = h.nodeService.GetByID(c.Request.Context(), n.ID)
			if err != nil {
				h.logger.Warn("Failed to reload node after persisting SSH metadata", logger.Err(err), logger.F("node_id", n.ID))
			}
		}
	}

	deployCertificateID := nodeCertificateIDValue(certificateID)
	if deployCertificateID > 0 && req.SSH != nil && h.deployService == nil {
		h.triggerCertificateDeployAsync(deployCertificateID, n.ID, "node_create_with_ssh")
	} else if deployCertificateID > 0 && req.SSH == nil {
		reason := "node_create"
		if autoMatchedCertificate {
			reason = "node_create_auto_match"
		}
		h.triggerCertificateDeployAsync(deployCertificateID, n.ID, reason)
	}

	resp := &NodeWithTokenResponse{
		NodeResponse: *h.buildNodeResponse(c.Request.Context(), n),
		Token:        n.Token,
	}

	// 如果提供了 SSH 配置，启动后台部署并立即返回
	if req.SSH != nil && h.deployService != nil {
		h.logger.Info("Starting auto-install", logger.F("node_id", n.ID), logger.F("host", req.SSH.Host))

		publicURL := ""
		if h.publicURL != nil {
			publicURL = h.publicURL()
		}
		panelURL := resolveDeployPanelURL(c, publicURL, req.SSH.PanelURL, n.PanelURL)

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
			if deployCertificateID > 0 {
				h.triggerCertificateDeployAsync(deployCertificateID, nodeID, "node_create_after_auto_install")
			}
			provisionNodeProxiesAfterDeploy(h.entitlementSvc, h.logger, nodeID)
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

	var trafficResetAt *time.Time
	if req.TrafficResetAt != nil {
		trafficResetAt, err = parseOptionalRFC3339Time(*req.TrafficResetAt)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid traffic_reset_at, must be RFC3339"})
			return
		}
	}

	var groupIDs []int64
	if req.GroupIDs != nil || req.GroupID != nil {
		if req.GroupIDs != nil {
			groupIDs = normalizeNodeGroupIDs(*req.GroupIDs, req.GroupID)
		} else {
			groupIDs = normalizeNodeGroupIDs(nil, req.GroupID)
		}
	}
	var primaryGroupID *int64
	if len(groupIDs) > 0 {
		primaryGroupID = &groupIDs[0]
	} else if req.GroupIDs != nil {
		primaryGroupID = nil
	} else {
		primaryGroupID = req.GroupID
	}

	existingNode, err := h.nodeService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to load node before update", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node"})
		return
	}

	tlsEnabled := existingNode.TLSEnabled
	if req.TLSEnabled != nil {
		tlsEnabled = *req.TLSEnabled
	}
	tlsDomain := existingNode.TLSDomain
	if req.TLSDomain != nil {
		tlsDomain = *req.TLSDomain
	}
	certificateID, autoMatchedCertificate := h.resolveNodeCertificateID(
		c.Request.Context(),
		tlsEnabled,
		tlsDomain,
		req.CertificateID,
		existingNode.CertificateID,
	)
	previousCertificateID := nodeCertificateIDValue(existingNode.CertificateID)
	nextCertificateID := nodeCertificateIDValue(certificateID)

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
		TrafficLimit:   req.TrafficLimit,
		TrafficResetAt: trafficResetAt,
		SpeedLimit:     req.SpeedLimit,

		// 协议支持
		Protocols: req.Protocols,

		// TLS 配置
		TLSEnabled: req.TLSEnabled,
		TLSDomain:  req.TLSDomain,

		// 节点分组
		GroupID: primaryGroupID,

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
		CertificateID: certificateID,
	}

	n, err := h.nodeService.Update(c.Request.Context(), id, updateReq)
	if err != nil {
		h.logger.Error("Failed to update node", logger.Err(err), logger.F("id", id))
		status, payload := nodeMutationErrorResponse(err, "Failed to update node")
		c.JSON(status, payload)
		return
	}

	if (req.GroupIDs != nil || req.GroupID != nil) && h.groupService != nil {
		if err := h.groupService.SyncNodeGroups(c.Request.Context(), id, groupIDs); err != nil {
			h.logger.Error("Failed to sync node groups after update", logger.Err(err), logger.F("node_id", id))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update node groups"})
			return
		}
		n, err = h.nodeService.GetByID(c.Request.Context(), id)
		if err != nil {
			h.logger.Error("Failed to reload node after group sync", logger.Err(err), logger.F("node_id", id))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload updated node"})
			return
		}
	}

	if req.SSH != nil {
		if err := persistNodeSSHMetadata(c.Request.Context(), h.nodeService, id, nodeSSHMetadataInput{
			Host:       req.SSH.Host,
			Port:       req.SSH.Port,
			Username:   req.SSH.Username,
			Password:   req.SSH.Password,
			PrivateKey: req.SSH.PrivateKey,
		}); err != nil {
			h.logger.Warn("Failed to persist node SSH metadata after update", logger.Err(err), logger.F("node_id", id))
		} else {
			n, err = h.nodeService.GetByID(c.Request.Context(), id)
			if err != nil {
				h.logger.Warn("Failed to reload node after persisting SSH metadata on update", logger.Err(err), logger.F("node_id", id))
			}
		}
	}

	if nextCertificateID > 0 && nextCertificateID != previousCertificateID {
		reason := "node_update_certificate_change"
		if autoMatchedCertificate {
			reason = "node_update_auto_match"
		}
		h.triggerCertificateDeployAsync(nextCertificateID, id, reason)
	}

	h.logger.Info("Node updated", logger.F("node_id", id))

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeUpdate,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(id, 10),
		Details:      map[string]any{"name": n.Name, "address": n.Address, "region": n.Region},
	})

	c.JSON(http.StatusOK, h.buildNodeResponse(c.Request.Context(), n))
}

// Delete deletes a node.
// DELETE /api/admin/nodes/:id
func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	nodeData, err := h.nodeService.GetByID(c.Request.Context(), id)
	if err != nil {
		if err == node.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
			return
		}
		h.logger.Error("Failed to load node before delete", logger.Err(err), logger.F("id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete node"})
		return
	}

	cleanupConfig := h.buildNodeCleanupDeployConfig(nodeData)

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

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeDelete,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(id, 10),
	})

	if cleanupConfig != nil && h.deployService != nil {
		go func(nodeID int64, nodeName string, cfg *node.DeployConfig) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			if err := h.deployService.CleanupAgent(ctx, cfg); err != nil {
				h.logger.Warn("Failed to cleanup remote agent after node delete",
					logger.Err(err),
					logger.F("node_id", nodeID),
					logger.F("node_name", nodeName),
					logger.F("host", cfg.Host))
				return
			}

			h.logger.Info("Remote agent cleanup triggered after node delete",
				logger.F("node_id", nodeID),
				logger.F("node_name", nodeName),
				logger.F("host", cfg.Host))
		}(nodeData.ID, nodeData.Name, cleanupConfig)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Node deleted successfully"})
}

func (h *NodeHandler) buildNodeCleanupDeployConfig(nodeData *node.Node) *node.DeployConfig {
	if nodeData == nil {
		return nil
	}

	host := strings.TrimSpace(nodeData.SSHHost)
	if host == "" {
		host = strings.TrimSpace(nodeData.Address)
	}
	if host == "" {
		return nil
	}

	username := strings.TrimSpace(nodeData.SSHUser)
	if username == "" {
		username = "root"
	}

	port := nodeData.SSHPort
	if port == 0 {
		port = 22
	}

	password := strings.TrimSpace(nodeData.SSHPassword)
	privateKey := ""
	if keyPath := strings.TrimSpace(nodeData.SSHKeyPath); keyPath != "" {
		if data, err := os.ReadFile(keyPath); err == nil {
			privateKey = string(data)
		} else if password == "" {
			h.logger.Warn("Failed to read saved SSH private key for node cleanup",
				logger.Err(err),
				logger.F("node_id", nodeData.ID),
				logger.F("key_path", keyPath))
		}
	}

	if password == "" && privateKey == "" {
		return nil
	}

	return &node.DeployConfig{
		NodeID:     nodeData.ID,
		Host:       host,
		Port:       port,
		Username:   username,
		Password:   password,
		PrivateKey: privateKey,
	}
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

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeTokenGen,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(id, 10),
	})

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

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeTokenRot,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(id, 10),
	})

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

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionNodeTokenRev,
		ResourceType: monitor.ResourceNode,
		ResourceID:   strconv.FormatInt(id, 10),
	})

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
			"error":        "已有相同类型命令正在等待节点领取，或刚刚已加入队列；如果持续出现，请先确认节点 Agent 已注册并能连接面板",
			"command_type": commandType,
		})
		return
	}

	h.logger.Info("Node command queued by admin",
		logger.F("node_id", id),
		logger.F("command_id", cmd.ID),
		logger.F("command_type", cmd.Type))

	if cmd.Type == commandTypeConfigSync {
		if err := h.nodeService.MarkSyncPending(c.Request.Context(), id); err != nil {
			h.logger.Warn("Failed to mark node sync pending after config command queued",
				logger.Err(err),
				logger.F("node_id", id),
				logger.F("command_id", cmd.ID))
		}
	}

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

// InstallCoreVersionRequest represents a request to install a specific Xray version on a node.
type InstallCoreVersionRequest struct {
	Version string `json:"version"`
}

// InstallCoreVersion queues an Xray version install/switch command on a node.
// Empty version means "latest release".
// POST /api/admin/nodes/:id/core/install-version
func (h *NodeHandler) InstallCoreVersion(c *gin.Context) {
	var req InstallCoreVersionRequest
	_ = c.ShouldBindJSON(&req)
	targetVersion := strings.TrimSpace(req.Version)
	targetVersion = strings.TrimPrefix(targetVersion, "v")

	// Light-weight validation: allow empty (= latest) or semver-ish "x.y.z"
	if targetVersion != "" {
		for _, ch := range targetVersion {
			if (ch < '0' || ch > '9') && ch != '.' && !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && ch != '-' {
				c.JSON(http.StatusBadRequest, gin.H{"error": "版本号格式无效"})
				return
			}
		}
	}

	reason := "管理员切换节点 Xray 内核版本"
	successMsg := "版本切换命令已加入队列，节点将在下次心跳时下载并重启"
	if targetVersion == "" {
		reason = "管理员升级节点 Xray 内核到最新版本"
		successMsg = "升级命令已加入队列，节点将在下次心跳时下载最新版"
	}

	h.queueNodeCommand(
		c,
		commandTypeXrayInstallVersion,
		reason,
		successMsg,
		func(nodeID int64, source, reason string) (Command, bool) {
			return h.recoveryTracker.QueueXrayInstallVersionCommand(nodeID, source, reason, targetVersion)
		},
	)
}

func (h *NodeHandler) UpdateAgent(c *gin.Context) {
	h.queueNodeCommand(
		c,
		commandTypeAgentUpdate,
		"管理员更新节点 Agent",
		"Agent 更新命令已加入队列，节点将在下次心跳时下载并重启",
		h.recoveryTracker.QueueAgentUpdateCommand,
	)
}

// GetSSHMetadata returns SSH connection metadata for a node.
// GET /api/admin/nodes/:id/ssh-metadata
func (h *NodeHandler) GetSSHMetadata(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	node, err := h.nodeService.GetByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Node not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"host":     node.SSHHost,
		"port":     node.SSHPort,
		"username": node.SSHUser,
	})
}

// SaveSSHMetadata saves SSH connection metadata for a node.
// POST /api/admin/nodes/:id/ssh-metadata
func (h *NodeHandler) SaveSSHMetadata(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid node ID"})
		return
	}

	var input nodeSSHMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if err := persistNodeSSHMetadata(c.Request.Context(), h.nodeService, id, input); err != nil {
		h.logger.Error("Failed to save SSH metadata", logger.Err(err), logger.F("node_id", id))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save SSH metadata"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SSH metadata saved successfully"})
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
