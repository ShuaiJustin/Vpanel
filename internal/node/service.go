// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
)

// Common errors
var (
	ErrNodeNotFound    = errors.New("node not found")
	ErrInvalidNode     = errors.New("invalid node data")
	ErrInvalidAddress  = errors.New("invalid node address")
	ErrInvalidToken    = errors.New("invalid token")
	ErrTokenRevoked    = errors.New("token has been revoked")
	ErrNoHealthyNodes  = errors.New("no healthy nodes available")
	ErrNodeAtCapacity  = errors.New("node is at capacity")
	ErrDuplicateToken  = errors.New("duplicate token generated")
	ErrNodeUnreachable = errors.New("node is unreachable")
	ErrDuplicateNode   = errors.New("node already exists")
)

// TokenLength is the length of generated tokens in bytes (64 hex chars = 32 bytes)
const TokenLength = 32

// Node represents a node in the service layer.
type Node struct {
	ID                int64        `json:"id"`
	Name              string       `json:"name"`
	Address           string       `json:"address"`
	Port              int          `json:"port"`
	PanelURL          string       `json:"panel_url"` // Panel server URL
	Token             string       `json:"token,omitempty"`
	Status            string       `json:"status"`
	Tags              []string     `json:"tags"`
	Region            string       `json:"region"`
	Weight            int          `json:"weight"`
	MaxUsers          int          `json:"max_users"`
	CurrentUsers      int          `json:"current_users"`
	Latency           int          `json:"latency"`
	LastSeenAt        *time.Time   `json:"last_seen_at"`
	SyncStatus        string       `json:"sync_status"`
	SyncedAt          *time.Time   `json:"synced_at"`
	InstallStatus     string       `json:"install_status"`
	InstallMessage    string       `json:"install_message"`
	InstallSteps      []DeployStep `json:"install_steps"`
	InstallLogs       string       `json:"install_logs"`
	InstallStartedAt  *time.Time   `json:"install_started_at"`
	InstallFinishedAt *time.Time   `json:"install_finished_at"`
	InstallUpdatedAt  *time.Time   `json:"install_updated_at"`
	IPWhitelist       []string     `json:"ip_whitelist"`

	// 流量统计
	TrafficUp      int64      `json:"traffic_up"`
	TrafficDown    int64      `json:"traffic_down"`
	TrafficTotal   int64      `json:"traffic_total"`
	TrafficLimit   int64      `json:"traffic_limit"`
	TrafficResetAt *time.Time `json:"traffic_reset_at"`

	// 负载信息
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	NetSpeed    int64   `json:"net_speed"`

	// 速率限制
	SpeedLimit int64 `json:"speed_limit"`

	// 协议支持
	Protocols []string `json:"protocols"`

	// TLS 配置
	TLSEnabled  bool   `json:"tls_enabled"`
	TLSDomain   string `json:"tls_domain"`
	TLSCertPath string `json:"tls_cert_path,omitempty"`
	TLSKeyPath  string `json:"tls_key_path,omitempty"`

	// 节点分组
	GroupID *int64  `json:"group_id"`
	Groups  []int64 `json:"group_ids,omitempty"` // Group IDs this node belongs to

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

	// Xray 状态
	XrayRunning bool   `json:"xray_running"`
	XrayVersion string `json:"xray_version"`

	// 证书关联
	CertificateID *int64 `json:"certificate_id,omitempty"`

	SSHHost     string `json:"-"`
	SSHPort     int    `json:"-"`
	SSHUser     string `json:"-"`
	SSHPassword string `json:"-"`
	SSHKeyPath  string `json:"-"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NodeMetrics represents node metrics for updates.
type NodeMetrics struct {
	Latency      int `json:"latency"`
	CurrentUsers int `json:"current_users"`
}

// CreateNodeRequest represents a request to create a node.
type CreateNodeRequest struct {
	Name        string   `json:"name"`
	Address     string   `json:"address"`
	Port        int      `json:"port"`
	PanelURL    string   `json:"panel_url"` // Panel server URL
	Tags        []string `json:"tags"`
	Region      string   `json:"region"`
	Weight      int      `json:"weight"`
	MaxUsers    int      `json:"max_users"`
	IPWhitelist []string `json:"ip_whitelist"`

	// 流量限制
	TrafficLimit   int64      `json:"traffic_limit"`
	TrafficResetAt *time.Time `json:"traffic_reset_at"`

	// 速率限制
	SpeedLimit int64 `json:"speed_limit"`

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

	// 流量限制
	TrafficLimit   *int64     `json:"traffic_limit"`
	TrafficResetAt *time.Time `json:"traffic_reset_at"`

	// 速率限制
	SpeedLimit *int64 `json:"speed_limit"`

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

// NodeFilter defines filter options for listing nodes.
type NodeFilter struct {
	Status  string
	Region  string
	Search  string
	Tags    []string
	GroupID *int64
	Limit   int
	Offset  int
}

// Service provides node management operations.
type Service struct {
	nodeRepo       repository.NodeRepository
	assignmentRepo repository.UserNodeAssignmentRepository
	proxyRepo      repository.ProxyRepository
	logger         logger.Logger
	cache          cache.Cache
	invalidator    *cache.Invalidator
}

// NewService creates a new node service.
func NewService(
	nodeRepo repository.NodeRepository,
	assignmentRepo repository.UserNodeAssignmentRepository,
	proxyRepo repository.ProxyRepository,
	log logger.Logger,
) *Service {
	return &Service{
		nodeRepo:       nodeRepo,
		assignmentRepo: assignmentRepo,
		proxyRepo:      proxyRepo,
		logger:         log,
		cache:          nil,
		invalidator:    nil,
	}
}

// WithCache adds cache support to the service.
func (s *Service) WithCache(c cache.Cache) *Service {
	s.cache = c
	s.invalidator = cache.NewInvalidator(c)
	return s
}

// Create creates a new node.
func (s *Service) Create(ctx context.Context, req *CreateNodeRequest) (*Node, error) {
	// 基础字段验证
	if req.Name == "" {
		return nil, fmt.Errorf("%w: 节点名称不能为空", ErrInvalidNode)
	}
	if len(req.Name) > 128 {
		return nil, fmt.Errorf("%w: 节点名称过长（最多 128 个字符）", ErrInvalidNode)
	}
	if req.Address == "" {
		return nil, fmt.Errorf("%w: 节点地址不能为空", ErrInvalidNode)
	}

	// 验证 Panel URL 格式（防止 SSRF 攻击）
	if req.PanelURL != "" {
		panelURL, err := url.Parse(req.PanelURL)
		if err != nil || (panelURL.Scheme != "http" && panelURL.Scheme != "https") {
			return nil, fmt.Errorf("%w: Panel URL 格式无效，必须是 http:// 或 https:// 开头的有效 URL", ErrInvalidNode)
		}
		// 防止内网地址 SSRF
		if panelURL.Hostname() == "localhost" || panelURL.Hostname() == "127.0.0.1" || strings.HasPrefix(panelURL.Hostname(), "192.168.") || strings.HasPrefix(panelURL.Hostname(), "10.") || strings.HasPrefix(panelURL.Hostname(), "172.") {
			s.logger.Warn("Potential SSRF attempt with internal Panel URL", logger.F("panel_url", req.PanelURL))
		}
	}

	// 标准化地址（去除空格和协议前缀）
	address := strings.TrimSpace(req.Address)
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "https://")
	address = strings.TrimSuffix(address, "/")

	// 验证地址格式
	if !ValidateAddress(address) {
		return nil, fmt.Errorf("%w: 地址格式无效，请输入有效的 IP 地址或域名", ErrInvalidAddress)
	}

	// 验证端口范围
	port := req.Port
	if port <= 0 {
		port = 18443 // 默认端口
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("%w: 端口必须在 1-65535 之间", ErrInvalidNode)
	}

	// 验证权重
	weight := req.Weight
	if weight <= 0 {
		weight = 1
	}
	if weight > 100 {
		return nil, fmt.Errorf("%w: 权重必须在 1-100 之间", ErrInvalidNode)
	}

	// 验证最大用户数
	if req.MaxUsers < 0 {
		return nil, fmt.Errorf("%w: 最大用户数不能为负数", ErrInvalidNode)
	}

	// 验证流量限制
	if req.TrafficLimit < 0 {
		return nil, fmt.Errorf("%w: 流量限制不能为负数", ErrInvalidNode)
	}

	// 验证速率限制
	if req.SpeedLimit < 0 {
		return nil, fmt.Errorf("%w: 速率限制不能为负数", ErrInvalidNode)
	}

	// 验证协议列表
	if len(req.Protocols) > 0 {
		validProtocols := map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "wireguard": true, "socks": true, "http": true,
		}
		for _, protocol := range req.Protocols {
			if !validProtocols[strings.ToLower(protocol)] {
				return nil, fmt.Errorf("%w: 不支持的协议 '%s'", ErrInvalidNode, protocol)
			}
		}
	}

	// 验证 TLS 配置
	if req.TLSEnabled && req.TLSDomain == "" {
		return nil, fmt.Errorf("%w: 启用 TLS 时必须指定域名", ErrInvalidNode)
	}
	if req.TLSDomain != "" && !ValidateDomain(req.TLSDomain) {
		return nil, fmt.Errorf("%w: TLS 域名格式无效", ErrInvalidNode)
	}

	// 验证告警阈值
	if req.AlertTrafficThreshold < 0 || req.AlertTrafficThreshold > 100 {
		return nil, fmt.Errorf("%w: 流量告警阈值必须在 0-100 之间", ErrInvalidNode)
	}
	if req.AlertCPUThreshold < 0 || req.AlertCPUThreshold > 100 {
		return nil, fmt.Errorf("%w: CPU 告警阈值必须在 0-100 之间", ErrInvalidNode)
	}
	if req.AlertMemoryThreshold < 0 || req.AlertMemoryThreshold > 100 {
		return nil, fmt.Errorf("%w: 内存告警阈值必须在 0-100 之间", ErrInvalidNode)
	}

	// 验证 IP 白名单格式
	for _, ip := range req.IPWhitelist {
		ipTrimmed := strings.TrimSpace(ip)
		if ipTrimmed == "" {
			continue
		}
		// 支持 CIDR 格式
		if strings.Contains(ipTrimmed, "/") {
			_, _, err := net.ParseCIDR(ipTrimmed)
			if err != nil {
				return nil, fmt.Errorf("%w: IP 白名单中的 CIDR 格式无效: %s", ErrInvalidNode, ip)
			}
		} else if !ValidateIPv4(ipTrimmed) && !ValidateIPv6(ipTrimmed) {
			return nil, fmt.Errorf("%w: IP 白名单中的地址无效: %s", ErrInvalidNode, ip)
		}
	}

	// 检查节点名称和地址是否重复 - 使用数据库级别的高效查询
	nameCount, err := s.nodeRepo.CountByName(ctx, req.Name, 0)
	if err != nil {
		s.logger.Warn("Failed to check for duplicate node name", logger.Err(err))
		// 继续创建，不因为查询失败而阻止
	} else if nameCount > 0 {
		return nil, fmt.Errorf("%w: 节点名称 '%s' 已存在", ErrDuplicateNode, req.Name)
	}

	addressPortCount, err := s.nodeRepo.CountByAddressPort(ctx, address, port, 0)
	if err != nil {
		s.logger.Warn("Failed to check for duplicate node address", logger.Err(err))
		// 继续创建，不因为查询失败而阻止
	} else if addressPortCount > 0 {
		return nil, fmt.Errorf("%w: 节点地址 %s:%d 已存在", ErrDuplicateNode, address, port)
	}

	// 生成 token
	token, err := GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", logger.Err(err))
		return nil, fmt.Errorf("生成认证 Token 失败: %w", err)
	}

	// 记录生成的 token（用于调试）
	s.logger.Info("Generated node token",
		logger.F("token_length", len(token)),
		logger.F("token_prefix", token[:8]),
		logger.F("token_suffix", token[len(token)-8:]))

	// 序列化 JSON 字段
	tagsJSON, _ := json.Marshal(req.Tags)
	if req.Tags == nil {
		tagsJSON = []byte("[]")
	}

	ipWhitelistJSON, _ := json.Marshal(req.IPWhitelist)
	if req.IPWhitelist == nil {
		ipWhitelistJSON = []byte("[]")
	}

	protocolsJSON, _ := json.Marshal(req.Protocols)
	if req.Protocols == nil {
		protocolsJSON = []byte("[]")
	}

	// 设置默认告警阈值
	alertTrafficThreshold := req.AlertTrafficThreshold
	if alertTrafficThreshold == 0 {
		alertTrafficThreshold = 80
	}
	alertCPUThreshold := req.AlertCPUThreshold
	if alertCPUThreshold == 0 {
		alertCPUThreshold = 80
	}
	alertMemoryThreshold := req.AlertMemoryThreshold
	if alertMemoryThreshold == 0 {
		alertMemoryThreshold = 80
	}

	trafficResetAt := req.TrafficResetAt
	if trafficResetAt == nil && req.TrafficLimit > 0 {
		now := time.Now()
		trafficResetAt = &now
	}

	repoNode := &repository.Node{
		Name:        req.Name,
		Address:     address, // 使用标准化后的地址
		Port:        port,
		Token:       token,
		PanelURL:    req.PanelURL, // 保存 Panel URL
		Status:      repository.NodeStatusOffline,
		Tags:        string(tagsJSON),
		Region:      req.Region,
		Weight:      weight,
		MaxUsers:    req.MaxUsers,
		SyncStatus:  repository.NodeSyncStatusPending,
		IPWhitelist: string(ipWhitelistJSON),

		// 流量和速率
		TrafficLimit:   req.TrafficLimit,
		TrafficResetAt: trafficResetAt,
		SpeedLimit:     req.SpeedLimit,

		// 协议
		Protocols: string(protocolsJSON),

		// TLS
		TLSEnabled: req.TLSEnabled,
		TLSDomain:  req.TLSDomain,

		// 分组和排序
		GroupID:  req.GroupID,
		Priority: req.Priority,
		Sort:     req.Sort,

		// 告警
		AlertTrafficThreshold: alertTrafficThreshold,
		AlertCPUThreshold:     alertCPUThreshold,
		AlertMemoryThreshold:  alertMemoryThreshold,

		// 描述
		Description: req.Description,
		Remarks:     req.Remarks,

		// 证书关联
		CertificateID: req.CertificateID,
	}

	if err := s.nodeRepo.Create(ctx, repoNode); err != nil {
		s.logger.Error("Failed to create node", logger.Err(err))
		return nil, fmt.Errorf("创建节点失败: %w", err)
	}

	// Invalidate node statistics cache
	if s.cache != nil {
		cacheKey := "node:stats:all"
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			// Log but don't fail the request
			s.logger.Warn("Failed to invalidate node statistics cache", logger.Err(err))
		}
	}

	s.logger.Info("Node created successfully",
		logger.F("node_id", repoNode.ID),
		logger.F("name", repoNode.Name),
		logger.F("address", repoNode.Address),
		logger.F("port", repoNode.Port),
		logger.F("token_saved", repoNode.Token[:8]+"..."))

	return s.toNode(repoNode), nil
}

// GetByID retrieves a node by ID.
func (s *Service) GetByID(ctx context.Context, id int64) (*Node, error) {
	repoNode, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNodeNotFound
	}
	return s.toNode(repoNode), nil
}

// MarkSyncPending marks a node as waiting for its generated Xray config to be applied.
func (s *Service) MarkSyncPending(ctx context.Context, id int64) error {
	if err := s.nodeRepo.UpdateSyncStatus(ctx, id, repository.NodeSyncStatusPending, nil); err != nil {
		s.logger.Error("Failed to mark node sync pending", logger.Err(err), logger.F("id", id))
		return err
	}

	if s.cache != nil {
		cacheKey := "node:stats:all"
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			s.logger.Warn("Failed to invalidate node statistics cache", logger.Err(err))
		}
	}

	return nil
}

// UpdateSSHConfig persists SSH metadata for later remote operations.
func (s *Service) UpdateSSHConfig(ctx context.Context, id int64, host string, port int, username, password, keyPath string) error {
	repoNode, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		return ErrNodeNotFound
	}

	trimmedHost := strings.TrimSpace(host)
	if trimmedHost != "" {
		repoNode.SSHHost = trimmedHost
	}
	if port > 0 {
		repoNode.SSHPort = port
	}
	trimmedUser := strings.TrimSpace(username)
	if trimmedUser != "" {
		repoNode.SSHUser = trimmedUser
	}
	if password != "" {
		encryptedPassword, encryptErr := EncryptSSHPassword(password)
		if encryptErr != nil {
			return encryptErr
		}
		repoNode.SSHPassword = encryptedPassword
	}
	trimmedKeyPath := strings.TrimSpace(keyPath)
	if trimmedKeyPath != "" {
		repoNode.SSHKeyPath = trimmedKeyPath
	}

	if err := s.nodeRepo.Update(ctx, repoNode); err != nil {
		return err
	}

	return nil
}

// Update updates a node.
func (s *Service) Update(ctx context.Context, id int64, req *UpdateNodeRequest) (*Node, error) {
	repoNode, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNodeNotFound
	}

	// 验证更新请求
	if req.Name != nil {
		if *req.Name == "" {
			return nil, fmt.Errorf("%w: 节点名称不能为空", ErrInvalidNode)
		}
		if len(*req.Name) > 128 {
			return nil, fmt.Errorf("%w: 节点名称过长（最多 128 个字符）", ErrInvalidNode)
		}

		// 检查名称是否与其他节点重复 - 使用数据库级别的高效查询
		nameCount, err := s.nodeRepo.CountByName(ctx, *req.Name, id)
		if err != nil {
			s.logger.Warn("Failed to check for duplicate node name", logger.Err(err))
		} else if nameCount > 0 {
			return nil, fmt.Errorf("%w: 节点名称 '%s' 已被使用", ErrDuplicateNode, *req.Name)
		}
		repoNode.Name = *req.Name
	}

	if req.Address != nil {
		// 标准化地址
		address := strings.TrimSpace(*req.Address)
		address = strings.TrimPrefix(address, "http://")
		address = strings.TrimPrefix(address, "https://")
		address = strings.TrimSuffix(address, "/")

		if !ValidateAddress(address) {
			return nil, fmt.Errorf("%w: 地址格式无效", ErrInvalidAddress)
		}

		// 检查地址+端口是否与其他节点重复 - 使用数据库级别的高效查询
		checkPort := repoNode.Port
		if req.Port != nil {
			checkPort = *req.Port
		}

		addressPortCount, err := s.nodeRepo.CountByAddressPort(ctx, address, checkPort, id)
		if err != nil {
			s.logger.Warn("Failed to check for duplicate node address", logger.Err(err))
		} else if addressPortCount > 0 {
			return nil, fmt.Errorf("%w: 节点地址 %s:%d 已被使用", ErrDuplicateNode, address, checkPort)
		}
		repoNode.Address = address
	}

	if req.Port != nil {
		if *req.Port <= 0 || *req.Port > 65535 {
			return nil, fmt.Errorf("%w: 端口必须在 1-65535 之间", ErrInvalidNode)
		}
		repoNode.Port = *req.Port
	}

	if req.PanelURL != nil {
		// 验证 Panel URL 格式（防止 SSRF 攻击）
		if *req.PanelURL != "" {
			panelURL, err := url.Parse(*req.PanelURL)
			if err != nil || (panelURL.Scheme != "http" && panelURL.Scheme != "https") {
				return nil, fmt.Errorf("%w: Panel URL 格式无效，必须是 http:// 或 https:// 开头的有效 URL", ErrInvalidNode)
			}
			// 防止内网地址 SSRF
			if panelURL.Hostname() == "localhost" || panelURL.Hostname() == "127.0.0.1" || strings.HasPrefix(panelURL.Hostname(), "192.168.") || strings.HasPrefix(panelURL.Hostname(), "10.") || strings.HasPrefix(panelURL.Hostname(), "172.") {
				s.logger.Warn("Potential SSRF attempt with internal Panel URL", logger.F("panel_url", *req.PanelURL))
			}
		}
		repoNode.PanelURL = *req.PanelURL
	}

	if req.Tags != nil {
		tagsJSON, _ := json.Marshal(*req.Tags)
		repoNode.Tags = string(tagsJSON)
	}

	if req.Region != nil {
		repoNode.Region = *req.Region
	}

	if req.Weight != nil {
		if *req.Weight < 1 || *req.Weight > 100 {
			return nil, fmt.Errorf("%w: 权重必须在 1-100 之间", ErrInvalidNode)
		}
		repoNode.Weight = *req.Weight
	}

	if req.MaxUsers != nil {
		if *req.MaxUsers < 0 {
			return nil, fmt.Errorf("%w: 最大用户数不能为负数", ErrInvalidNode)
		}
		repoNode.MaxUsers = *req.MaxUsers
	}

	if req.IPWhitelist != nil {
		// 验证 IP 白名单
		for _, ip := range *req.IPWhitelist {
			ipTrimmed := strings.TrimSpace(ip)
			if ipTrimmed == "" {
				continue
			}
			if strings.Contains(ipTrimmed, "/") {
				_, _, err := net.ParseCIDR(ipTrimmed)
				if err != nil {
					return nil, fmt.Errorf("%w: IP 白名单中的 CIDR 格式无效: %s", ErrInvalidNode, ip)
				}
			} else if !ValidateIPv4(ipTrimmed) && !ValidateIPv6(ipTrimmed) {
				return nil, fmt.Errorf("%w: IP 白名单中的地址无效: %s", ErrInvalidNode, ip)
			}
		}
		ipWhitelistJSON, _ := json.Marshal(*req.IPWhitelist)
		repoNode.IPWhitelist = string(ipWhitelistJSON)
	}

	if req.TrafficLimit != nil {
		if *req.TrafficLimit < 0 {
			return nil, fmt.Errorf("%w: 流量限制不能为负数", ErrInvalidNode)
		}
		previousTrafficLimit := repoNode.TrafficLimit
		repoNode.TrafficLimit = *req.TrafficLimit
		if *req.TrafficLimit > 0 && req.TrafficResetAt == nil && (repoNode.TrafficResetAt == nil || previousTrafficLimit <= 0) {
			now := time.Now()
			repoNode.TrafficResetAt = &now
		}
	}
	if req.TrafficResetAt != nil {
		repoNode.TrafficResetAt = req.TrafficResetAt
	}

	if req.SpeedLimit != nil {
		if *req.SpeedLimit < 0 {
			return nil, fmt.Errorf("%w: 速率限制不能为负数", ErrInvalidNode)
		}
		repoNode.SpeedLimit = *req.SpeedLimit
	}

	if req.Protocols != nil {
		// 验证协议
		validProtocols := map[string]bool{
			"vless": true, "vmess": true, "trojan": true,
			"shadowsocks": true, "wireguard": true, "socks": true, "http": true,
		}
		for _, protocol := range *req.Protocols {
			if !validProtocols[strings.ToLower(protocol)] {
				return nil, fmt.Errorf("%w: 不支持的协议 '%s'", ErrInvalidNode, protocol)
			}
		}
		protocolsJSON, _ := json.Marshal(*req.Protocols)
		repoNode.Protocols = string(protocolsJSON)
	}

	if req.TLSEnabled != nil {
		repoNode.TLSEnabled = *req.TLSEnabled
	}

	if req.TLSDomain != nil {
		if *req.TLSDomain != "" && !ValidateDomain(*req.TLSDomain) {
			return nil, fmt.Errorf("%w: TLS 域名格式无效", ErrInvalidNode)
		}
		repoNode.TLSDomain = *req.TLSDomain
	}

	if req.GroupID != nil {
		repoNode.GroupID = req.GroupID
	}

	if req.Priority != nil {
		repoNode.Priority = *req.Priority
	}

	if req.Sort != nil {
		repoNode.Sort = *req.Sort
	}

	if req.AlertTrafficThreshold != nil {
		if *req.AlertTrafficThreshold < 0 || *req.AlertTrafficThreshold > 100 {
			return nil, fmt.Errorf("%w: 流量告警阈值必须在 0-100 之间", ErrInvalidNode)
		}
		repoNode.AlertTrafficThreshold = *req.AlertTrafficThreshold
	}

	if req.AlertCPUThreshold != nil {
		if *req.AlertCPUThreshold < 0 || *req.AlertCPUThreshold > 100 {
			return nil, fmt.Errorf("%w: CPU 告警阈值必须在 0-100 之间", ErrInvalidNode)
		}
		repoNode.AlertCPUThreshold = *req.AlertCPUThreshold
	}

	if req.AlertMemoryThreshold != nil {
		if *req.AlertMemoryThreshold < 0 || *req.AlertMemoryThreshold > 100 {
			return nil, fmt.Errorf("%w: 内存告警阈值必须在 0-100 之间", ErrInvalidNode)
		}
		repoNode.AlertMemoryThreshold = *req.AlertMemoryThreshold
	}

	if req.Description != nil {
		repoNode.Description = *req.Description
	}

	if req.Remarks != nil {
		repoNode.Remarks = *req.Remarks
	}

	if req.CertificateID != nil {
		repoNode.CertificateID = req.CertificateID
	}

	if err := s.nodeRepo.Update(ctx, repoNode); err != nil {
		s.logger.Error("Failed to update node", logger.Err(err), logger.F("id", id))
		return nil, fmt.Errorf("更新节点失败: %w", err)
	}

	// Invalidate node statistics cache
	if s.cache != nil {
		cacheKey := "node:stats:all"
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			// Log but don't fail the request
			s.logger.Warn("Failed to invalidate node statistics cache", logger.Err(err))
		}
	}

	s.logger.Info("Node updated successfully",
		logger.F("node_id", id),
		logger.F("name", repoNode.Name))

	return s.toNode(repoNode), nil
}

// Delete deletes a node and reassigns its users to other healthy nodes.
// 此操作在事务中执行，确保原子性
func (s *Service) Delete(ctx context.Context, id int64) error {
	node, err := s.nodeRepo.GetByID(ctx, id)
	if err != nil {
		return ErrNodeNotFound
	}

	deletedProxyCount := 0

	// Get users assigned to this node
	userIDs, err := s.assignmentRepo.GetUserIDsByNodeID(ctx, id)
	if err != nil {
		s.logger.Error("Failed to get users for node", logger.Err(err), logger.F("node_id", id))
		return err
	}

	// 在事务中执行用户重分配和节点删除
	err = s.nodeRepo.Transaction(ctx, func(txCtx context.Context) error {
		// Reassign users to other healthy nodes
		if len(userIDs) > 0 {
			if err := s.reassignUsersFromNodeInTx(txCtx, id, userIDs); err != nil {
				s.logger.Error("Failed to reassign users in transaction",
					logger.Err(err),
					logger.F("node_id", id))
				return fmt.Errorf("无法重分配用户: %w", err)
			}
		}

		deletedProxyCount, err = s.deleteNodeBoundProxiesInTx(txCtx, id)
		if err != nil {
			s.logger.Error("Failed to delete node-bound proxies in transaction",
				logger.Err(err),
				logger.F("node_id", id))
			return fmt.Errorf("无法清理节点代理: %w", err)
		}

		// Delete the node
		if err := s.nodeRepo.DeleteInTx(txCtx, id); err != nil {
			s.logger.Error("Failed to delete node in transaction",
				logger.Err(err),
				logger.F("id", id))
			return err
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Node deletion transaction failed",
			logger.Err(err),
			logger.F("node_id", id))
		return fmt.Errorf("节点删除失败: %w", err)
	}

	// Invalidate node statistics cache
	if s.cache != nil {
		cacheKey := "node:stats:all"
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			// Log but don't fail the request
			s.logger.Warn("Failed to invalidate node statistics cache", logger.Err(err))
		}
	}

	s.logger.Info("Node deleted successfully",
		logger.F("id", id),
		logger.F("name", node.Name),
		logger.F("reassigned_users", len(userIDs)),
		logger.F("deleted_proxies", deletedProxyCount))
	return nil
}

func (s *Service) deleteNodeBoundProxiesInTx(ctx context.Context, nodeID int64) (int, error) {
	if s == nil || s.proxyRepo == nil || nodeID <= 0 {
		return 0, nil
	}

	const batchSize = 500
	proxyIDs := make([]int64, 0)
	offset := 0

	for {
		proxies, err := s.proxyRepo.List(ctx, batchSize, offset)
		if err != nil {
			return 0, err
		}
		if len(proxies) == 0 {
			break
		}

		for _, proxyModel := range proxies {
			if proxyModel == nil || proxyModel.NodeID == nil || *proxyModel.NodeID != nodeID {
				continue
			}
			proxyIDs = append(proxyIDs, proxyModel.ID)
		}

		if len(proxies) < batchSize {
			break
		}
		offset += len(proxies)
	}

	if len(proxyIDs) == 0 {
		return 0, nil
	}

	if err := s.proxyRepo.DeleteByIDs(ctx, proxyIDs); err != nil {
		return 0, err
	}

	return len(proxyIDs), nil
}

// reassignUsersFromNode reassigns users from a node to other healthy nodes.
// 已废弃：使用 reassignUsersFromNodeInTx 在事务中执行
func (s *Service) reassignUsersFromNode(ctx context.Context, excludeNodeID int64, userIDs []int64) error {
	return s.reassignUsersFromNodeInTx(ctx, excludeNodeID, userIDs)
}

// reassignUsersFromNodeInTx reassigns users from a node to other healthy nodes within a transaction.
func (s *Service) reassignUsersFromNodeInTx(ctx context.Context, excludeNodeID int64, userIDs []int64) error {
	// Get available healthy nodes
	healthyNodes, err := s.nodeRepo.GetAvailableInTx(ctx)
	if err != nil {
		return fmt.Errorf("获取可用节点失败: %w", err)
	}

	// Filter out the node being deleted
	var availableNodes []*repository.Node
	for _, n := range healthyNodes {
		if n.ID != excludeNodeID {
			availableNodes = append(availableNodes, n)
		}
	}

	if len(availableNodes) == 0 {
		// 严重问题：没有可用节点，用户将失去服务
		s.logger.Error("No healthy nodes available for reassignment",
			logger.F("node_id", excludeNodeID),
			logger.F("user_count", len(userIDs)))
		return fmt.Errorf("没有可用的健康节点来重分配 %d 个用户", len(userIDs))
	}

	// Distribute users across available nodes using round-robin
	failedCount := 0
	failedUsers := make([]int64, 0)

	for i, userID := range userIDs {
		targetNode := availableNodes[i%len(availableNodes)]
		if err := s.assignmentRepo.ReassignInTx(ctx, userID, targetNode.ID); err != nil {
			s.logger.Error("Failed to reassign user",
				logger.Err(err),
				logger.F("user_id", userID),
				logger.F("target_node_id", targetNode.ID))
			failedCount++
			failedUsers = append(failedUsers, userID)
		}
	}

	if failedCount > 0 {
		return fmt.Errorf("重分配失败: %d/%d 用户分配失败 (用户ID: %v)",
			failedCount, len(userIDs), failedUsers)
	}

	s.logger.Info("Users reassigned successfully in transaction",
		logger.F("user_count", len(userIDs)),
		logger.F("target_nodes", len(availableNodes)))
	return nil
}

// List lists nodes with filter and pagination.
func (s *Service) List(ctx context.Context, filter NodeFilter) ([]*Node, int64, error) {
	repoFilter := &repository.NodeFilter{
		Status:  filter.Status,
		Region:  filter.Region,
		Search:  filter.Search,
		Tags:    filter.Tags,
		GroupID: filter.GroupID,
		Limit:   filter.Limit,
		Offset:  filter.Offset,
	}

	repoNodes, err := s.nodeRepo.List(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to list nodes", logger.Err(err))
		return nil, 0, err
	}

	total, err := s.nodeRepo.Count(ctx, repoFilter)
	if err != nil {
		s.logger.Error("Failed to count nodes", logger.Err(err))
		return nil, 0, err
	}

	nodes := make([]*Node, len(repoNodes))
	for i, rn := range repoNodes {
		nodes[i] = s.toNode(rn)
	}

	return nodes, total, nil
}

// UpdateStatus updates a node's status.
func (s *Service) UpdateStatus(ctx context.Context, id int64, status string) error {
	if err := s.nodeRepo.UpdateStatus(ctx, id, status); err != nil {
		s.logger.Error("Failed to update node status", logger.Err(err), logger.F("id", id), logger.F("status", status))
		return err
	}

	// Invalidate node statistics cache since status affects statistics
	if s.cache != nil {
		cacheKey := "node:stats:all"
		if err := s.cache.Delete(ctx, cacheKey); err != nil {
			// Log but don't fail the request
			s.logger.Warn("Failed to invalidate node statistics cache", logger.Err(err))
		}
	}

	return nil
}

// UpdateMetrics updates a node's metrics.
func (s *Service) UpdateMetrics(ctx context.Context, id int64, metrics *NodeMetrics) error {
	if err := s.nodeRepo.UpdateMetrics(ctx, id, metrics.Latency, metrics.CurrentUsers); err != nil {
		s.logger.Error("Failed to update node metrics", logger.Err(err), logger.F("id", id))
		return err
	}
	return nil
}

// UpdateLastSeen updates a node's last seen timestamp.
func (s *Service) UpdateLastSeen(ctx context.Context, id int64) error {
	if err := s.nodeRepo.UpdateLastSeen(ctx, id, time.Now()); err != nil {
		s.logger.Error("Failed to update node last seen", logger.Err(err), logger.F("id", id))
		return err
	}
	return nil
}

// GetHealthyNodes returns all healthy (online) nodes.
func (s *Service) GetHealthyNodes(ctx context.Context) ([]*Node, error) {
	repoNodes, err := s.nodeRepo.GetHealthy(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, len(repoNodes))
	for i, rn := range repoNodes {
		nodes[i] = s.toNode(rn)
	}
	return nodes, nil
}

// GetAvailableNodes returns nodes that are online and not at capacity.
func (s *Service) GetAvailableNodes(ctx context.Context) ([]*Node, error) {
	repoNodes, err := s.nodeRepo.GetAvailable(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, len(repoNodes))
	for i, rn := range repoNodes {
		nodes[i] = s.toNode(rn)
	}
	return nodes, nil
}

// ============================================
// Token Management
// ============================================

// GenerateToken generates a cryptographically secure random token.
func GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateNodeToken generates a new token for a node.
func (s *Service) GenerateNodeToken(ctx context.Context, nodeID int64) (string, error) {
	_, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return "", ErrNodeNotFound
	}

	token, err := GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", logger.Err(err))
		return "", err
	}

	if err := s.nodeRepo.UpdateToken(ctx, nodeID, token); err != nil {
		s.logger.Error("Failed to update node token", logger.Err(err), logger.F("node_id", nodeID))
		return "", err
	}

	s.logger.Info("Generated new token for node", logger.F("node_id", nodeID))
	return token, nil
}

// RotateToken rotates a node's token, invalidating the old one immediately.
// 警告: 旧 token 立即失效，节点需要使用新 token 重新连接
func (s *Service) RotateToken(ctx context.Context, nodeID int64) (string, error) {
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return "", ErrNodeNotFound
	}

	// Generate new token
	newToken, err := GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate new token", logger.Err(err))
		return "", err
	}

	// Update token (old token is immediately invalidated)
	if err := s.nodeRepo.UpdateToken(ctx, nodeID, newToken); err != nil {
		s.logger.Error("Failed to rotate token", logger.Err(err), logger.F("node_id", nodeID))
		return "", err
	}

	// 将节点状态设置为 offline，等待新 token 重新连接
	if err := s.nodeRepo.UpdateStatus(ctx, nodeID, repository.NodeStatusOffline); err != nil {
		s.logger.Warn("Failed to update node status after token rotation",
			logger.Err(err), logger.F("node_id", nodeID))
	}

	s.logger.Info("Rotated token for node",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name))
	return newToken, nil
}

// RevokeToken revokes a node's token by setting it to empty.
func (s *Service) RevokeToken(ctx context.Context, nodeID int64) error {
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return ErrNodeNotFound
	}

	// Set token to empty string to revoke it
	if err := s.nodeRepo.UpdateToken(ctx, nodeID, ""); err != nil {
		s.logger.Error("Failed to revoke token", logger.Err(err), logger.F("node_id", nodeID))
		return err
	}

	// Also set node status to offline since it can no longer authenticate
	if err := s.nodeRepo.UpdateStatus(ctx, nodeID, repository.NodeStatusOffline); err != nil {
		s.logger.Error("Failed to update node status after token revocation",
			logger.Err(err), logger.F("node_id", nodeID))
	}

	s.logger.Info("Revoked token for node",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name))
	return nil
}

// ValidateToken validates a token and returns the associated node.
func (s *Service) ValidateToken(ctx context.Context, token string) (*Node, error) {
	if token == "" {
		s.logger.Warn("Token validation failed: empty token")
		return nil, ErrInvalidToken
	}

	// 记录验证的 token（用于调试）
	s.logger.Debug("Validating token",
		logger.F("token_length", len(token)),
		logger.F("token_prefix", token[:min(8, len(token))]),
		logger.F("token_suffix", token[max(0, len(token)-8):]))

	repoNode, err := s.nodeRepo.GetByToken(ctx, token)
	if err != nil {
		s.logger.Debug("Token not found in database",
			logger.F("token_prefix", token[:min(8, len(token))]),
			logger.Err(err))
		return nil, ErrInvalidToken
	}

	// Check if token is revoked (empty token in DB means revoked)
	if repoNode.Token == "" {
		s.logger.Warn("Token is revoked", logger.F("node_id", repoNode.ID))
		return nil, ErrTokenRevoked
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(repoNode.Token), []byte(token)) != 1 {
		s.logger.Debug("Token mismatch (constant-time check)", logger.F("node_id", repoNode.ID))
		return nil, ErrInvalidToken
	}

	s.logger.Debug("Token validated successfully",
		logger.F("node_id", repoNode.ID),
		logger.F("node_name", repoNode.Name))

	return s.toNode(repoNode), nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ============================================
// Address Validation
// ============================================

// domainRegex matches valid domain names
var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// ValidateAddress validates if the given address is a valid IPv4, IPv6, or domain name.
func ValidateAddress(address string) bool {
	if address == "" {
		return false
	}

	// Trim whitespace
	address = strings.TrimSpace(address)

	// Check if it's a valid IP address (IPv4 or IPv6)
	if ip := net.ParseIP(address); ip != nil {
		return true
	}

	// Check if it's a valid domain name
	if domainRegex.MatchString(address) {
		return true
	}

	// Check for localhost
	if address == "localhost" {
		return true
	}

	return false
}

// ValidateIPv4 validates if the given address is a valid IPv4 address.
func ValidateIPv4(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return ip.To4() != nil
}

// ValidateIPv6 validates if the given address is a valid IPv6 address.
func ValidateIPv6(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}
	return ip.To4() == nil && ip.To16() != nil
}

// ValidateDomain validates if the given address is a valid domain name.
func ValidateDomain(address string) bool {
	if address == "" {
		return false
	}
	address = strings.TrimSpace(address)
	if address == "localhost" {
		return true
	}
	return domainRegex.MatchString(address)
}

// ============================================
// Helper Methods
// ============================================

// toNode converts a repository node to a service node.
func (s *Service) toNode(rn *repository.Node) *Node {
	var tags []string
	var ipWhitelist []string
	var protocols []string
	var installSteps []DeployStep

	if rn.Tags != "" {
		if err := json.Unmarshal([]byte(rn.Tags), &tags); err != nil {
			s.logger.Warn("Failed to unmarshal node tags", logger.Err(err), logger.F("node_id", rn.ID))
			tags = []string{}
		}
	}
	if rn.IPWhitelist != "" {
		if err := json.Unmarshal([]byte(rn.IPWhitelist), &ipWhitelist); err != nil {
			s.logger.Warn("Failed to unmarshal node IP whitelist", logger.Err(err), logger.F("node_id", rn.ID))
			ipWhitelist = []string{}
		}
	}
	if rn.Protocols != "" {
		if err := json.Unmarshal([]byte(rn.Protocols), &protocols); err != nil {
			s.logger.Warn("Failed to unmarshal node protocols", logger.Err(err), logger.F("node_id", rn.ID))
			protocols = []string{}
		}
	}
	if rn.InstallSteps != "" {
		if err := json.Unmarshal([]byte(rn.InstallSteps), &installSteps); err != nil {
			s.logger.Warn("Failed to unmarshal node install steps", logger.Err(err), logger.F("node_id", rn.ID))
			installSteps = []DeployStep{}
		}
	}

	sshPassword, err := DecryptSSHPassword(rn.SSHPassword)
	if err != nil {
		s.logger.Warn("Failed to decrypt node SSH password", logger.Err(err), logger.F("node_id", rn.ID))
	}

	// Extract group IDs from preloaded Groups
	var groupIDs []int64
	if len(rn.Groups) > 0 {
		groupIDs = make([]int64, 0, len(rn.Groups))
		for _, group := range rn.Groups {
			if group != nil {
				groupIDs = append(groupIDs, group.ID)
			}
		}
	}

	return &Node{
		ID:                rn.ID,
		Name:              rn.Name,
		Address:           rn.Address,
		Port:              rn.Port,
		PanelURL:          rn.PanelURL, // 添加 Panel URL 字段
		Token:             rn.Token,
		Status:            rn.Status,
		Tags:              tags,
		Region:            rn.Region,
		Weight:            rn.Weight,
		MaxUsers:          rn.MaxUsers,
		CurrentUsers:      rn.CurrentUsers,
		Latency:           rn.Latency,
		LastSeenAt:        rn.LastSeenAt,
		SyncStatus:        rn.SyncStatus,
		SyncedAt:          rn.SyncedAt,
		InstallStatus:     rn.InstallStatus,
		InstallMessage:    rn.InstallMessage,
		InstallSteps:      installSteps,
		InstallLogs:       rn.InstallLogs,
		InstallStartedAt:  rn.InstallStartedAt,
		InstallFinishedAt: rn.InstallFinishedAt,
		InstallUpdatedAt:  rn.InstallUpdatedAt,
		IPWhitelist:       ipWhitelist,

		// 流量统计
		TrafficUp:      rn.TrafficUp,
		TrafficDown:    rn.TrafficDown,
		TrafficTotal:   rn.TrafficTotal,
		TrafficLimit:   rn.TrafficLimit,
		TrafficResetAt: rn.TrafficResetAt,

		// 负载信息
		CPUUsage:    rn.CPUUsage,
		MemoryUsage: rn.MemoryUsage,
		DiskUsage:   rn.DiskUsage,
		NetSpeed:    rn.NetSpeed,

		// 速率限制
		SpeedLimit: rn.SpeedLimit,

		// 协议支持
		Protocols: protocols,

		// TLS 配置
		TLSEnabled:  rn.TLSEnabled,
		TLSDomain:   rn.TLSDomain,
		TLSCertPath: rn.TLSCertPath,
		TLSKeyPath:  rn.TLSKeyPath,

		// 节点分组
		GroupID: rn.GroupID,
		Groups:  groupIDs,

		// 排序和优先级
		Priority: rn.Priority,
		Sort:     rn.Sort,

		// 告警配置
		AlertTrafficThreshold: rn.AlertTrafficThreshold,
		AlertCPUThreshold:     rn.AlertCPUThreshold,
		AlertMemoryThreshold:  rn.AlertMemoryThreshold,

		// 备注和描述
		Description: rn.Description,
		Remarks:     rn.Remarks,

		// Xray 状态
		XrayRunning: rn.XrayRunning,
		XrayVersion: rn.XrayVersion,

		// 证书关联
		CertificateID: rn.CertificateID,

		SSHHost:     rn.SSHHost,
		SSHPort:     rn.SSHPort,
		SSHUser:     rn.SSHUser,
		SSHPassword: sshPassword,
		SSHKeyPath:  rn.SSHKeyPath,

		CreatedAt: rn.CreatedAt,
		UpdatedAt: rn.UpdatedAt,
	}
}

// GetStatistics returns node statistics.
func (s *Service) GetStatistics(ctx context.Context) (map[string]int64, error) {
	// Try cache first if available
	if s.cache != nil {
		cacheKey := "node:stats:all"

		// Try to get from cache
		data, err := s.cache.Get(ctx, cacheKey)
		if err == nil {
			// Cache hit - unmarshal and return
			var stats map[string]int64
			unmarshalErr := json.Unmarshal(data, &stats)
			if unmarshalErr == nil {
				return stats, nil
			}
			// If unmarshal fails, continue to query database
			s.logger.Warn("Failed to unmarshal cached node statistics", logger.Err(unmarshalErr))
		} else if !errors.Is(err, cache.ErrCacheMiss) {
			// Log cache errors but continue to database
			s.logger.Warn("Failed to get node statistics from cache", logger.Err(err))
		}
	}

	// Cache miss or cache unavailable - query database
	stats, err := s.nodeRepo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	// Store in cache if available
	if s.cache != nil {
		cacheKey := "node:stats:all"
		ttl := cache.GetTTL("node:stats")

		data, marshalErr := json.Marshal(stats)
		if marshalErr == nil {
			if cacheErr := s.cache.Set(ctx, cacheKey, data, ttl); cacheErr != nil {
				// Log cache errors but don't fail the request
				s.logger.Warn("Failed to cache node statistics", logger.Err(cacheErr))
			}
		} else {
			s.logger.Warn("Failed to marshal node statistics for caching", logger.Err(marshalErr))
		}
	}

	return stats, nil
}

// GetTotalUsers returns the total number of users across all nodes.
func (s *Service) GetTotalUsers(ctx context.Context) (int64, error) {
	return s.nodeRepo.GetTotalUsers(ctx)
}

// TestNodeConnectivity 测试节点连通性
func (s *Service) TestNodeConnectivity(ctx context.Context, address string, port int) error {
	if port <= 0 {
		port = 18443
	}

	addr := fmt.Sprintf("%s:%d", address, port)

	// 使用 context-aware 连接，尊重上下文超时
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		s.logger.Warn("节点连通性测试失败",
			logger.F("address", address),
			logger.F("port", port),
			logger.Err(err))
		return fmt.Errorf("%w: %v", ErrNodeUnreachable, err)
	}
	defer conn.Close()

	s.logger.Info("节点连通性测试成功",
		logger.F("address", address),
		logger.F("port", port))
	return nil
}

// ValidateNodeConfig 验证节点配置的完整性
func (s *Service) ValidateNodeConfig(req *CreateNodeRequest) error {
	// 基础验证
	if req.Name == "" {
		return fmt.Errorf("节点名称不能为空")
	}
	if req.Address == "" {
		return fmt.Errorf("节点地址不能为空")
	}

	// 地址格式验证
	if !ValidateAddress(req.Address) {
		return fmt.Errorf("节点地址格式无效")
	}

	// 端口验证
	if req.Port > 0 && (req.Port < 1 || req.Port > 65535) {
		return fmt.Errorf("端口必须在 1-65535 之间")
	}

	// TLS 配置验证
	if req.TLSEnabled && req.TLSDomain == "" {
		return fmt.Errorf("启用 TLS 时必须指定域名")
	}

	// 协议验证
	validProtocols := map[string]bool{
		"vless": true, "vmess": true, "trojan": true,
		"shadowsocks": true, "wireguard": true,
	}
	for _, p := range req.Protocols {
		if !validProtocols[strings.ToLower(p)] {
			return fmt.Errorf("不支持的协议: %s", p)
		}
	}

	return nil
}

// GetXrayConfig generates and returns the Xray configuration for a node.
func (s *Service) GetXrayConfig(ctx context.Context, nodeID int64) (map[string]interface{}, error) {
	// 检查节点是否存在
	node, err := s.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	// TODO: 实现 Xray 配置生成逻辑
	// 这里返回一个示例配置
	config := map[string]interface{}{
		"log": map[string]interface{}{
			"loglevel": "warning",
		},
		"inbounds": []map[string]interface{}{
			{
				"port":     node.Port,
				"protocol": "vless",
				"settings": map[string]interface{}{
					"clients":    []interface{}{},
					"decryption": "none",
				},
				"streamSettings": map[string]interface{}{
					"network":  "tcp",
					"security": "tls",
				},
			},
		},
		"outbounds": []map[string]interface{}{
			{
				"protocol": "freedom",
				"tag":      "direct",
			},
		},
	}

	return config, nil
}
