// Package node provides node management functionality for multi-server management.
package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// ConfigSync errors
var (
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrSyncFailed       = errors.New("sync failed")
	ErrNodeNotOnline    = errors.New("node is not online")
	ErrNoNodesToSync    = errors.New("no nodes to sync")
	ErrConfigValidation = errors.New("configuration validation failed")
)

// SyncStatus represents the sync status of a node.
type SyncStatus struct {
	NodeID     int64      `json:"node_id"`
	NodeName   string     `json:"node_name"`
	Status     string     `json:"status"` // synced, pending, failed
	SyncedAt   *time.Time `json:"synced_at"`
	Error      string     `json:"error,omitempty"`
	RetryCount int        `json:"retry_count"`
}

// SyncResult represents the result of a sync operation.
type SyncResult struct {
	NodeID   int64     `json:"node_id"`
	Success  bool      `json:"success"`
	Error    string    `json:"error,omitempty"`
	SyncedAt time.Time `json:"synced_at"`
	Duration int64     `json:"duration_ms"`
}

// ProxyConfig represents a proxy configuration to sync.
type ProxyConfig struct {
	ID       int64          `json:"id"`
	UserID   int64          `json:"user_id"`
	Name     string         `json:"name"`
	Protocol string         `json:"protocol"`
	Port     int            `json:"port"`
	Host     string         `json:"host"`
	Settings map[string]any `json:"settings"`
	Enabled  bool           `json:"enabled"`
}

// NodeConfig represents the full configuration to sync to a node.
type NodeConfig struct {
	Version   string        `json:"version"`
	Timestamp time.Time     `json:"timestamp"`
	Proxies   []ProxyConfig `json:"proxies"`
}

// ConfigSyncConfig holds configuration for the ConfigSync service.
type ConfigSyncConfig struct {
	MaxRetries     int           // Maximum retry attempts for failed syncs
	RetryDelay     time.Duration // Delay between retries
	SyncTimeout    time.Duration // Timeout for sync operations
	ValidateConfig bool          // Whether to validate config before sync
}

// DefaultConfigSyncConfig returns default configuration.
func DefaultConfigSyncConfig() ConfigSyncConfig {
	return ConfigSyncConfig{
		MaxRetries:     3,
		RetryDelay:     5 * time.Second,
		SyncTimeout:    30 * time.Second,
		ValidateConfig: true,
	}
}

// ConfigSyncer defines the interface for configuration synchronization.
type ConfigSyncer interface {
	// SyncToNode syncs configuration to a single node.
	SyncToNode(ctx context.Context, nodeID int64) (*SyncResult, error)

	// SyncToGroup syncs configuration to all nodes in a group.
	SyncToGroup(ctx context.Context, groupID int64) ([]*SyncResult, error)

	// SyncToAll syncs configuration to all online nodes.
	SyncToAll(ctx context.Context) ([]*SyncResult, error)

	// GetSyncStatus returns the sync status for a node.
	GetSyncStatus(ctx context.Context, nodeID int64) (*SyncStatus, error)

	// ValidateConfig validates a configuration before syncing.
	ValidateConfig(ctx context.Context, config *NodeConfig) error

	// GetPendingSyncNodes returns nodes with pending sync status.
	GetPendingSyncNodes(ctx context.Context) ([]*SyncStatus, error)
}

// configSync implements ConfigSyncer.
type configSync struct {
	mu            sync.RWMutex
	config        ConfigSyncConfig
	nodeRepo      repository.NodeRepository
	nodeGroupRepo repository.NodeGroupRepository
	proxyRepo     repository.ProxyRepository
	logger        logger.Logger
	retryTracker  map[int64]int // nodeID -> retry count
}

// NewConfigSync creates a new ConfigSync service.
func NewConfigSync(
	cfg ConfigSyncConfig,
	nodeRepo repository.NodeRepository,
	nodeGroupRepo repository.NodeGroupRepository,
	proxyRepo repository.ProxyRepository,
	log logger.Logger,
) ConfigSyncer {
	return &configSync{
		config:        cfg,
		nodeRepo:      nodeRepo,
		nodeGroupRepo: nodeGroupRepo,
		proxyRepo:     proxyRepo,
		logger:        log,
		retryTracker:  make(map[int64]int),
	}
}

// SyncToNode syncs configuration to a single node.
func (s *configSync) SyncToNode(ctx context.Context, nodeID int64) (*SyncResult, error) {
	startTime := time.Now()
	result := &SyncResult{
		NodeID:   nodeID,
		SyncedAt: startTime,
	}

	// Get node
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		result.Error = fmt.Sprintf("node not found: %v", err)
		return result, ErrNodeNotFound
	}

	// Check if node is online
	if node.Status != repository.NodeStatusOnline {
		result.Error = "node is not online"
		return result, ErrNodeNotOnline
	}

	// Build configuration
	config, err := s.buildNodeConfig(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("failed to build config: %v", err)
		return result, err
	}

	// Validate configuration if enabled
	if s.config.ValidateConfig {
		if err := s.ValidateConfig(ctx, config); err != nil {
			result.Error = fmt.Sprintf("config validation failed: %v", err)
			// Update sync status to failed
			s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusFailed, nil)
			return result, err
		}
	}

	// Perform sync with retry
	syncErr := s.syncWithRetry(ctx, node, config)
	if syncErr != nil {
		result.Error = syncErr.Error()
		s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusFailed, nil)
		s.logger.Error("Failed to sync config to node",
			logger.Err(syncErr),
			logger.F("node_id", nodeID),
			logger.F("node_name", node.Name))
		return result, syncErr
	}

	// Update sync status
	now := time.Now()
	if err := s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusSynced, &now); err != nil {
		s.logger.Error("Failed to update sync status",
			logger.Err(err),
			logger.F("node_id", nodeID))
	}

	// Clear retry counter
	s.mu.Lock()
	delete(s.retryTracker, nodeID)
	s.mu.Unlock()

	result.Success = true
	result.Duration = time.Since(startTime).Milliseconds()

	s.logger.Info("Config synced to node",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name),
		logger.F("duration_ms", result.Duration))

	return result, nil
}

// SyncToGroup syncs configuration to all nodes in a group.
func (s *configSync) SyncToGroup(ctx context.Context, groupID int64) ([]*SyncResult, error) {
	// Get nodes in group
	nodes, err := s.nodeRepo.List(ctx, &repository.NodeFilter{GroupID: &groupID})
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes in group: %w", err)
	}

	if len(nodes) == 0 {
		return nil, ErrNoNodesToSync
	}

	results := make([]*SyncResult, 0, len(nodes))
	var wg sync.WaitGroup
	resultChan := make(chan *SyncResult, len(nodes))

	// Sync to each node concurrently
	for _, node := range nodes {
		if node.Status != repository.NodeStatusOnline {
			results = append(results, &SyncResult{
				NodeID:   node.ID,
				Success:  false,
				Error:    "node is not online",
				SyncedAt: time.Now(),
			})
			continue
		}

		wg.Add(1)
		go func(n *repository.Node) {
			defer wg.Done()
			result, _ := s.SyncToNode(ctx, n.ID)
			resultChan <- result
		}(node)
	}

	// Wait for all syncs to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	s.logger.Info("Config synced to group",
		logger.F("group_id", groupID),
		logger.F("node_count", len(nodes)),
		logger.F("result_count", len(results)))

	return results, nil
}

// SyncToAll syncs configuration to all online nodes.
func (s *configSync) SyncToAll(ctx context.Context) ([]*SyncResult, error) {
	// Get all online nodes
	nodes, err := s.nodeRepo.GetOnline(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get online nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, ErrNoNodesToSync
	}

	results := make([]*SyncResult, 0, len(nodes))
	var wg sync.WaitGroup
	resultChan := make(chan *SyncResult, len(nodes))

	// Sync to each node concurrently
	for _, node := range nodes {
		wg.Add(1)
		go func(n *repository.Node) {
			defer wg.Done()
			result, _ := s.SyncToNode(ctx, n.ID)
			resultChan <- result
		}(node)
	}

	// Wait for all syncs to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		results = append(results, result)
	}

	// Count successes and failures
	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	s.logger.Info("Config synced to all nodes",
		logger.F("total_nodes", len(nodes)),
		logger.F("success_count", successCount),
		logger.F("fail_count", failCount))

	return results, nil
}

// GetSyncStatus returns the sync status for a node.
func (s *configSync) GetSyncStatus(ctx context.Context, nodeID int64) (*SyncStatus, error) {
	node, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, ErrNodeNotFound
	}

	s.mu.RLock()
	retryCount := s.retryTracker[nodeID]
	s.mu.RUnlock()

	return &SyncStatus{
		NodeID:     node.ID,
		NodeName:   node.Name,
		Status:     node.SyncStatus,
		SyncedAt:   node.SyncedAt,
		RetryCount: retryCount,
	}, nil
}

// GetPendingSyncNodes returns nodes with pending sync status.
func (s *configSync) GetPendingSyncNodes(ctx context.Context) ([]*SyncStatus, error) {
	nodes, err := s.nodeRepo.GetPendingSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending sync nodes: %w", err)
	}

	statuses := make([]*SyncStatus, len(nodes))
	for i, node := range nodes {
		s.mu.RLock()
		retryCount := s.retryTracker[node.ID]
		s.mu.RUnlock()

		statuses[i] = &SyncStatus{
			NodeID:     node.ID,
			NodeName:   node.Name,
			Status:     node.SyncStatus,
			SyncedAt:   node.SyncedAt,
			RetryCount: retryCount,
		}
	}

	return statuses, nil
}

// ValidateConfig validates a configuration before syncing.
// Property 18: Config Validation Before Sync
// For any configuration sync attempt, invalid configurations SHALL be rejected before being sent to nodes.
func (s *configSync) ValidateConfig(ctx context.Context, config *NodeConfig) error {
	if config == nil {
		return fmt.Errorf("%w: config is nil", ErrInvalidConfig)
	}

	// Validate version
	if config.Version == "" {
		return fmt.Errorf("%w: version is required", ErrInvalidConfig)
	}

	// Validate timestamp
	if config.Timestamp.IsZero() {
		return fmt.Errorf("%w: timestamp is required", ErrInvalidConfig)
	}

	// Validate each proxy configuration
	portSet := make(map[int]bool)
	for i, proxy := range config.Proxies {
		if err := s.validateProxyConfig(&proxy, i); err != nil {
			return err
		}

		// Check for duplicate ports
		if portSet[proxy.Port] {
			return fmt.Errorf("%w: duplicate port %d in proxy configurations", ErrInvalidConfig, proxy.Port)
		}
		portSet[proxy.Port] = true
	}

	return nil
}

// validateProxyConfig validates a single proxy configuration.
func (s *configSync) validateProxyConfig(proxy *ProxyConfig, index int) error {
	if proxy.ID <= 0 {
		return fmt.Errorf("%w: proxy[%d] has invalid ID", ErrInvalidConfig, index)
	}

	if proxy.Name == "" {
		return fmt.Errorf("%w: proxy[%d] name is required", ErrInvalidConfig, index)
	}

	if proxy.Protocol == "" {
		return fmt.Errorf("%w: proxy[%d] protocol is required", ErrInvalidConfig, index)
	}

	// Validate protocol type
	validProtocols := map[string]bool{
		"vmess":       true,
		"vless":       true,
		"trojan":      true,
		"shadowsocks": true,
	}
	if !validProtocols[proxy.Protocol] {
		return fmt.Errorf("%w: proxy[%d] has invalid protocol: %s", ErrInvalidConfig, index, proxy.Protocol)
	}

	// Validate port range
	if proxy.Port <= 0 || proxy.Port > 65535 {
		return fmt.Errorf("%w: proxy[%d] has invalid port: %d", ErrInvalidConfig, index, proxy.Port)
	}

	// Validate protocol-specific settings
	if err := s.validateProtocolSettings(proxy); err != nil {
		return fmt.Errorf("%w: proxy[%d] %v", ErrInvalidConfig, index, err)
	}

	return nil
}

// validateProtocolSettings validates protocol-specific settings.
func (s *configSync) validateProtocolSettings(proxy *ProxyConfig) error {
	if proxy.Settings == nil {
		return nil // Settings are optional
	}

	switch proxy.Protocol {
	case "vmess":
		return s.validateVMessSettings(proxy.Settings)
	case "vless":
		return s.validateVLESSSettings(proxy.Settings)
	case "trojan":
		return s.validateTrojanSettings(proxy.Settings)
	case "shadowsocks":
		return s.validateShadowsocksSettings(proxy.Settings)
	}

	return nil
}

// validateVMessSettings validates VMess protocol settings.
func (s *configSync) validateVMessSettings(settings map[string]any) error {
	// VMess requires UUID
	if uuid, ok := settings["uuid"]; ok {
		if uuidStr, ok := uuid.(string); !ok || uuidStr == "" {
			return fmt.Errorf("vmess uuid must be a non-empty string")
		}
	}
	return nil
}

// validateVLESSSettings validates VLESS protocol settings.
func (s *configSync) validateVLESSSettings(settings map[string]any) error {
	// VLESS requires UUID
	if uuid, ok := settings["uuid"]; ok {
		if uuidStr, ok := uuid.(string); !ok || uuidStr == "" {
			return fmt.Errorf("vless uuid must be a non-empty string")
		}
	}
	return nil
}

// validateTrojanSettings validates Trojan protocol settings.
func (s *configSync) validateTrojanSettings(settings map[string]any) error {
	// Trojan requires password
	if password, ok := settings["password"]; ok {
		if pwdStr, ok := password.(string); !ok || pwdStr == "" {
			return fmt.Errorf("trojan password must be a non-empty string")
		}
	}
	return nil
}

// validateShadowsocksSettings validates Shadowsocks protocol settings.
func (s *configSync) validateShadowsocksSettings(settings map[string]any) error {
	// Shadowsocks requires method and password
	if method, ok := settings["method"]; ok {
		if methodStr, ok := method.(string); !ok || methodStr == "" {
			return fmt.Errorf("shadowsocks method must be a non-empty string")
		}
	}
	if password, ok := settings["password"]; ok {
		if pwdStr, ok := password.(string); !ok || pwdStr == "" {
			return fmt.Errorf("shadowsocks password must be a non-empty string")
		}
	}
	return nil
}

// buildNodeConfig builds the configuration to sync to nodes.
func (s *configSync) buildNodeConfig(ctx context.Context) (*NodeConfig, error) {
	// Get all enabled proxies
	proxies, err := s.proxyRepo.GetEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled proxies: %w", err)
	}

	proxyConfigs := make([]ProxyConfig, len(proxies))
	for i, p := range proxies {
		proxyConfigs[i] = ProxyConfig{
			ID:       p.ID,
			UserID:   p.UserID,
			Name:     p.Name,
			Protocol: p.Protocol,
			Port:     p.Port,
			Host:     p.Host,
			Settings: p.Settings,
			Enabled:  p.Enabled,
		}
	}

	return &NodeConfig{
		Version:   "1.0",
		Timestamp: time.Now(),
		Proxies:   proxyConfigs,
	}, nil
}

// syncWithRetry performs sync with retry logic.
func (s *configSync) syncWithRetry(ctx context.Context, node *repository.Node, config *NodeConfig) error {
	var lastErr error

	for attempt := 0; attempt <= s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			s.logger.Info("Retrying sync",
				logger.F("node_id", node.ID),
				logger.F("attempt", attempt),
				logger.F("max_retries", s.config.MaxRetries))

			// Update retry tracker
			s.mu.Lock()
			s.retryTracker[node.ID] = attempt
			s.mu.Unlock()

			// Wait before retry
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.config.RetryDelay):
			}
		}

		// Attempt sync
		err := s.performSync(ctx, node, config)
		if err == nil {
			return nil
		}

		lastErr = err
		s.logger.Warn("Sync attempt failed",
			logger.Err(err),
			logger.F("node_id", node.ID),
			logger.F("attempt", attempt+1))
	}

	return fmt.Errorf("%w: %v after %d attempts", ErrSyncFailed, lastErr, s.config.MaxRetries+1)
}

// performSync performs the actual sync operation to a node.
func (s *configSync) performSync(ctx context.Context, node *repository.Node, config *NodeConfig) error {
	// Create a context with timeout
	syncCtx, cancel := context.WithTimeout(ctx, s.config.SyncTimeout)
	defer cancel()

	// Serialize config to JSON
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("配置序列化失败: %w", err)
	}

	// 构建 Node Agent 的配置同步 API 地址
	url := "http://" + net.JoinHostPort(node.Address, strconv.Itoa(node.Port)) + "/config/sync"

	s.logger.Debug("向节点同步配置",
		logger.F("node_id", node.ID),
		logger.F("node_address", node.Address),
		logger.F("url", url),
		logger.F("config_size", len(configJSON)))

	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(syncCtx, http.MethodPost, url, bytes.NewReader(configJSON))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", node.Token))

	// 发送请求
	client := &http.Client{
		Timeout: s.config.SyncTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("配置同步请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("配置同步失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	s.logger.Info("配置同步成功",
		logger.F("node_id", node.ID),
		logger.F("node_name", node.Name))

	return nil
}

// MarkNodeForSync marks a node as needing sync.
func (s *configSync) MarkNodeForSync(ctx context.Context, nodeID int64) error {
	return s.nodeRepo.UpdateSyncStatus(ctx, nodeID, repository.NodeSyncStatusPending, nil)
}

// MarkAllNodesForSync marks all nodes as needing sync.
func (s *configSync) MarkAllNodesForSync(ctx context.Context) error {
	nodes, err := s.nodeRepo.GetOnline(ctx)
	if err != nil {
		return fmt.Errorf("failed to get online nodes: %w", err)
	}

	for _, node := range nodes {
		if err := s.nodeRepo.UpdateSyncStatus(ctx, node.ID, repository.NodeSyncStatusPending, nil); err != nil {
			s.logger.Error("Failed to mark node for sync",
				logger.Err(err),
				logger.F("node_id", node.ID))
		}
	}

	return nil
}
