// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
	proxylib "v/internal/proxy"
)

// HealthCheckConfig holds configuration for the health checker.
type HealthCheckConfig struct {
	// Interval is the time between health checks (default: 30 seconds)
	Interval time.Duration
	// Timeout is the timeout for each health check (default: 10 seconds)
	Timeout time.Duration
	// UnhealthyThreshold is the number of consecutive failures before marking unhealthy
	UnhealthyThreshold int
	// HealthyThreshold is the number of consecutive successes before marking healthy
	HealthyThreshold int
	// RetentionDays is how many days to keep health check history
	RetentionDays int
	// MaxConcurrentChecks is the maximum number of concurrent health checks (default: 10)
	MaxConcurrentChecks int
}

// DefaultHealthCheckConfig returns the default health check configuration.
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		Interval:            30 * time.Second,
		Timeout:             10 * time.Second,
		UnhealthyThreshold:  3,
		HealthyThreshold:    2,
		RetentionDays:       7,
		MaxConcurrentChecks: 10,
	}
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	NodeID    int64
	Status    string // healthy, unhealthy
	TCPOk     bool
	APIOk     bool
	XrayOk    bool
	Latency   int // milliseconds
	Message   string
	CheckedAt time.Time
}

// HealthChecker performs periodic health checks on nodes.
type HealthChecker struct {
	config          *HealthCheckConfig
	nodeRepo        repository.NodeRepository
	proxyRepo       repository.ProxyRepository
	certRepo        repository.CertificateRepository
	healthCheckRepo repository.HealthCheckRepository
	logger          logger.Logger
	httpClient      *http.Client
	notificationSvc *notification.Service

	// State tracking for consecutive failures/successes
	consecutiveFailures  map[int64]int
	consecutiveSuccesses map[int64]int
	stateMu              sync.RWMutex

	// Control
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   bool
	runningMu sync.Mutex

	// Notification callback
	onStatusChange func(nodeID int64, oldStatus, newStatus string)
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(
	config *HealthCheckConfig,
	nodeRepo repository.NodeRepository,
	proxyRepo repository.ProxyRepository,
	certRepo repository.CertificateRepository,
	healthCheckRepo repository.HealthCheckRepository,
	log logger.Logger,
) *HealthChecker {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}

	return &HealthChecker{
		config:               config,
		nodeRepo:             nodeRepo,
		proxyRepo:            proxyRepo,
		certRepo:             certRepo,
		healthCheckRepo:      healthCheckRepo,
		logger:               log,
		httpClient:           &http.Client{Timeout: config.Timeout},
		consecutiveFailures:  make(map[int64]int),
		consecutiveSuccesses: make(map[int64]int),
	}
}

// SetOnStatusChange sets the callback for node status changes.
func (hc *HealthChecker) SetOnStatusChange(callback func(nodeID int64, oldStatus, newStatus string)) {
	hc.onStatusChange = callback
}

// SetNotificationService sets the notification service for sending alerts.
func (hc *HealthChecker) SetNotificationService(svc *notification.Service) {
	hc.notificationSvc = svc
}

// Start starts the health checker.
func (hc *HealthChecker) Start(ctx context.Context) error {
	hc.runningMu.Lock()
	defer hc.runningMu.Unlock()

	if hc.running {
		return fmt.Errorf("health checker is already running")
	}

	hc.ctx, hc.cancel = context.WithCancel(ctx)
	hc.running = true

	hc.wg.Add(1)
	go hc.runLoop()

	hc.logger.Info("Health checker started",
		logger.F("interval", hc.config.Interval.String()),
		logger.F("unhealthy_threshold", hc.config.UnhealthyThreshold),
		logger.F("healthy_threshold", hc.config.HealthyThreshold))

	return nil
}

// Stop stops the health checker.
func (hc *HealthChecker) Stop(ctx context.Context) error {
	hc.runningMu.Lock()
	if !hc.running {
		hc.runningMu.Unlock()
		return nil
	}
	hc.cancel()
	hc.running = false
	hc.runningMu.Unlock()

	// Wait for goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		hc.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		hc.logger.Info("Health checker stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsRunning returns whether the health checker is running.
func (hc *HealthChecker) IsRunning() bool {
	hc.runningMu.Lock()
	defer hc.runningMu.Unlock()
	return hc.running
}

// runLoop is the main loop that performs periodic health checks.
func (hc *HealthChecker) runLoop() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	// Run initial check immediately
	hc.checkAllNodes()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.checkAllNodes()
		}
	}
}

// checkAllNodes performs health checks on all registered nodes.
// Uses a worker pool to limit concurrent checks and prevent resource exhaustion.
func (hc *HealthChecker) checkAllNodes() {
	nodes, err := hc.nodeRepo.List(hc.ctx, nil)
	if err != nil {
		hc.logger.Error("Failed to list nodes for health check", logger.Err(err))
		return
	}

	if len(nodes) == 0 {
		return
	}

	// Create a semaphore to limit concurrent checks
	maxConcurrent := hc.config.MaxConcurrentChecks
	if maxConcurrent <= 0 {
		maxConcurrent = 10 // Default fallback
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	hc.logger.Debug("Starting health checks",
		logger.F("node_count", len(nodes)),
		logger.F("max_concurrent", maxConcurrent))

	for _, node := range nodes {
		wg.Add(1)

		// Acquire semaphore
		select {
		case sem <- struct{}{}:
			// Got a slot, proceed
		case <-hc.ctx.Done():
			// Context cancelled, stop spawning new checks
			wg.Done()
			continue
		}

		go func(n *repository.Node) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			// Check for context cancellation
			select {
			case <-hc.ctx.Done():
				return
			default:
				hc.checkNode(n)
			}
		}(node)
	}

	wg.Wait()

	// Cleanup old health check records
	hc.cleanupOldRecords()
}

// checkNode performs a health check on a single node.
func (hc *HealthChecker) checkNode(node *repository.Node) {
	result := hc.performCheck(node)

	// Save health check record
	check := &repository.HealthCheck{
		NodeID:    node.ID,
		Status:    result.Status,
		Latency:   result.Latency,
		Message:   result.Message,
		TCPOk:     result.TCPOk,
		APIOk:     result.APIOk,
		XrayOk:    result.XrayOk,
		CheckedAt: result.CheckedAt,
	}

	if err := hc.healthCheckRepo.Create(hc.ctx, check); err != nil {
		hc.logger.Error("Failed to save health check record",
			logger.Err(err),
			logger.F("node_id", node.ID))
	}

	// Update consecutive counters and handle status transitions
	hc.handleCheckResult(node, result)
}

// performCheck performs the actual health check on a node.
func (hc *HealthChecker) performCheck(node *repository.Node) *HealthCheckResult {
	result := &HealthCheckResult{
		NodeID:    node.ID,
		CheckedAt: time.Now(),
	}

	tcpStart := time.Now()
	result.TCPOk = hc.checkTCP(node.Address, node.Port)
	result.Latency = int(time.Since(tcpStart).Milliseconds())
	if !result.TCPOk {
		result.Latency = -1
	}

	// Check API responsiveness
	if result.TCPOk {
		result.APIOk = hc.checkAPI(node.Address, node.Port)
	}

	// Check Xray status (via API)
	if result.APIOk {
		result.XrayOk = hc.checkXray(node.Address, node.Port)
	}

	// Check certificate expiration if node has a certificate
	certWarning := hc.checkCertificateExpiration(node)
	heartbeatHealthy := shouldTrustRecentHeartbeat(node, hc.config.Interval, time.Now())
	proxyHealth := hc.checkReachableProxyEndpoint(node)

	if nodeTrafficLimitExceeded(node) {
		result.Status = repository.HealthCheckStatusFailed
		result.Message = "Node traffic limit exceeded"
		return result
	}

	// Determine overall status
	if result.TCPOk && result.APIOk && result.XrayOk && shouldDeferProxyEndpointFailureForConfigSync(node, proxyHealth) {
		result.Status = repository.HealthCheckStatusSuccess
		result.Message = sampledProxyEndpointConfigPendingMessage(proxyHealth)
		if certWarning != "" {
			result.Message = fmt.Sprintf("%s. %s", result.Message, certWarning)
		}
	} else if result.TCPOk && result.APIOk && result.XrayOk && sampledProxyEndpointsHealthyForPrimary(proxyHealth) {
		result.Status = repository.HealthCheckStatusSuccess
		if certWarning != "" {
			result.Message = fmt.Sprintf("All checks passed. %s", certWarning)
		} else {
			result.Message = "All checks passed"
		}
	} else if result.TCPOk && result.APIOk && result.XrayOk && proxyHealth.HasSampled && !proxyHealth.AllReachable {
		if proxyHealth.AnyReachable {
			result.Status = repository.HealthCheckStatusSuccess
			result.Message = sampledProxyEndpointWarningMessage(proxyHealth)
		} else {
			result.Status = repository.HealthCheckStatusFailed
			result.Message = sampledProxyEndpointFailureMessage(proxyHealth)
		}
		if certWarning != "" {
			result.Message = fmt.Sprintf("%s. %s", result.Message, certWarning)
		}
	} else if shouldAcceptHeartbeatFallback(heartbeatHealthy, proxyHealth.HasSampled, proxyHealth.AnyReachable) {
		result.Status = repository.HealthCheckStatusSuccess
		result.Message = heartbeatFallbackMessage(proxyHealth.HasSampled)
		if certWarning != "" {
			result.Message = fmt.Sprintf("%s. %s", result.Message, certWarning)
		}
	} else {
		result.Status = repository.HealthCheckStatusFailed
		if heartbeatHealthy && proxyHealth.HasSampled && !proxyHealth.AnyReachable {
			result.Message = "Recent heartbeat confirms Xray is running, but sampled proxy endpoints are unreachable from the panel"
		} else {
			result.Message = hc.buildFailureMessage(result)
		}
		if certWarning != "" {
			result.Message = fmt.Sprintf("%s. %s", result.Message, certWarning)
		}
	}

	return result
}

func shouldDeferProxyEndpointFailureForConfigSync(node *repository.Node, proxyHealth sampledProxyEndpointHealth) bool {
	if node == nil || node.SyncStatus != repository.NodeSyncStatusPending {
		return false
	}
	return proxyHealth.HasSampled && !proxyHealth.AnyReachable
}

type sampledProxyEndpointHealth struct {
	HasSampled             bool
	AnyReachable           bool
	AllReachable           bool
	CheckedCount           int
	FirstUnreachableTarget string
}

func sampledProxyEndpointsHealthyForPrimary(proxyHealth sampledProxyEndpointHealth) bool {
	return !proxyHealth.HasSampled || proxyHealth.AllReachable
}

func sampledProxyEndpointFailureMessage(proxyHealth sampledProxyEndpointHealth) string {
	if proxyHealth.FirstUnreachableTarget != "" {
		return fmt.Sprintf("Agent and Xray checks passed, but sampled proxy endpoint %s is unreachable from the panel", proxyHealth.FirstUnreachableTarget)
	}
	return "Agent and Xray checks passed, but sampled proxy endpoints are unreachable from the panel"
}

func sampledProxyEndpointConfigPendingMessage(proxyHealth sampledProxyEndpointHealth) string {
	if proxyHealth.FirstUnreachableTarget != "" {
		return fmt.Sprintf("配置同步未完成，代理端口 %s 暂未从面板侧连通；等待 Agent 同步并重启 Xray 后复查", proxyHealth.FirstUnreachableTarget)
	}
	return "配置同步未完成，代理端口暂未从面板侧连通；等待 Agent 同步并重启 Xray 后复查"
}

func sampledProxyEndpointWarningMessage(proxyHealth sampledProxyEndpointHealth) string {
	if proxyHealth.FirstUnreachableTarget != "" {
		return fmt.Sprintf("Agent and Xray checks passed, but sampled proxy endpoint %s is unreachable while at least one sampled proxy endpoint is reachable", proxyHealth.FirstUnreachableTarget)
	}
	return "Agent and Xray checks passed, but some sampled proxy endpoints are unreachable"
}

// checkTCP checks TCP connectivity to the node.
func (hc *HealthChecker) checkTCP(address string, port int) bool {
	addr := fmt.Sprintf("%s:%d", address, port)
	conn, err := net.DialTimeout("tcp", addr, hc.config.Timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// checkAPI checks the node agent API responsiveness.
func (hc *HealthChecker) checkAPI(address string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/health", address, port)
	resp, err := hc.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// checkXray checks the Xray process status via the node agent.
func (hc *HealthChecker) checkXray(address string, port int) bool {
	url := fmt.Sprintf("http://%s:%d/xray/status", address, port)
	resp, err := hc.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func shouldTrustRecentHeartbeat(node *repository.Node, interval time.Duration, now time.Time) bool {
	if node == nil || node.LastSeenAt == nil || !node.XrayRunning {
		return false
	}

	freshnessWindow := interval * 2
	if freshnessWindow <= 0 {
		freshnessWindow = time.Minute
	}

	return now.Sub(*node.LastSeenAt) <= freshnessWindow
}

func shouldAcceptHeartbeatFallback(heartbeatHealthy, hasSampledProxy, proxyReachable bool) bool {
	if !heartbeatHealthy {
		return false
	}
	if !hasSampledProxy {
		return true
	}
	return proxyReachable
}

func heartbeatFallbackMessage(hasSampledProxy bool) string {
	if hasSampledProxy {
		return "Recent heartbeat confirms Xray is running and at least one sampled proxy endpoint is reachable"
	}
	return "Recent heartbeat confirms Xray is running"
}

func (hc *HealthChecker) checkReachableProxyEndpoint(node *repository.Node) sampledProxyEndpointHealth {
	if hc == nil || hc.proxyRepo == nil || node == nil || node.ID <= 0 {
		return sampledProxyEndpointHealth{}
	}

	ctx := hc.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	proxies, err := hc.proxyRepo.GetByNodeID(ctx, node.ID)
	if err != nil {
		hc.logger.Warn("Failed to load node proxies during heartbeat fallback health check",
			logger.Err(err),
			logger.F("node_id", node.ID))
		return sampledProxyEndpointHealth{}
	}
	if len(proxies) == 0 {
		return sampledProxyEndpointHealth{}
	}

	checkedTargets := map[string]struct{}{}
	health := sampledProxyEndpointHealth{AllReachable: true}
	const maxSampledProxyTargets = 3

	for _, proxyModel := range proxies {
		if proxyModel == nil || proxyModel.Port <= 0 {
			continue
		}
		host := resolveHealthCheckProxyHost(node, proxyModel)
		if host == "" {
			continue
		}
		target := fmt.Sprintf("%s:%d", host, proxyModel.Port)
		if _, exists := checkedTargets[target]; exists {
			continue
		}
		checkedTargets[target] = struct{}{}
		health.CheckedCount++
		if hc.checkTCP(host, proxyModel.Port) {
			health.AnyReachable = true
		} else {
			health.AllReachable = false
			if health.FirstUnreachableTarget == "" {
				health.FirstUnreachableTarget = target
			}
		}
		if health.CheckedCount >= maxSampledProxyTargets {
			break
		}
	}

	health.HasSampled = health.CheckedCount > 0
	if !health.HasSampled {
		health.AllReachable = false
	}
	return health
}

func resolveHealthCheckProxyHost(node *repository.Node, proxyModel *repository.Proxy) string {
	if proxyModel == nil {
		return ""
	}

	if proxyModel.Settings != nil {
		if explicitServer, ok := proxyModel.Settings["server"].(string); ok {
			if normalized := proxylib.NormalizeShareHost(explicitServer); normalized != "" {
				return normalized
			}
		}
	}

	if strings.EqualFold(strings.TrimSpace(proxyModel.Remark), "auto provisioned") && node != nil {
		if normalized := proxylib.NormalizeShareHost(node.Address); normalized != "" {
			return normalized
		}
	}

	if normalized := proxylib.ResolveServerAddress(proxyModel.Host, proxyModel.Settings); normalized != "" {
		return normalized
	}
	if node != nil {
		if normalized := proxylib.NormalizeShareHost(node.Address); normalized != "" {
			return normalized
		}
	}
	return ""
}

// buildFailureMessage builds a failure message based on check results.
func (hc *HealthChecker) buildFailureMessage(result *HealthCheckResult) string {
	var failures []string
	if !result.TCPOk {
		failures = append(failures, "TCP connection failed")
	}
	if !result.APIOk {
		failures = append(failures, "API not responding")
	}
	if !result.XrayOk {
		failures = append(failures, "Xray not running")
	}
	if len(failures) == 0 {
		return "Unknown failure"
	}
	return fmt.Sprintf("Health check failed: %v", failures)
}

// handleCheckResult handles the result of a health check and updates node status.
func (hc *HealthChecker) handleCheckResult(node *repository.Node, result *HealthCheckResult) {
	hc.stateMu.Lock()
	defer hc.stateMu.Unlock()

	oldStatus := node.Status

	// 总是更新延迟（无论健康检查是否成功）
	// 延迟反映的是网络连通性，即使服务不健康也应该记录
	if err := hc.nodeRepo.UpdateMetrics(hc.ctx, node.ID, result.Latency, node.CurrentUsers); err != nil {
		hc.logger.Error("Failed to update node metrics",
			logger.Err(err),
			logger.F("node_id", node.ID))
	} else {
		hc.logger.Debug("Updated node metrics",
			logger.F("node_id", node.ID),
			logger.F("latency", result.Latency),
			logger.F("current_users", node.CurrentUsers))
	}

	if result.Status == repository.HealthCheckStatusSuccess {
		// Reset failure counter, increment success counter
		hc.consecutiveFailures[node.ID] = 0
		hc.consecutiveSuccesses[node.ID]++

		// Check if we should transition to healthy
		if node.Status == repository.NodeStatusUnhealthy &&
			hc.consecutiveSuccesses[node.ID] >= hc.config.HealthyThreshold {
			hc.transitionToHealthy(node, oldStatus)
		} else if node.Status == repository.NodeStatusOffline {
			// Node came online
			hc.transitionToOnline(node, oldStatus)
		}
	} else {
		// Reset success counter, increment failure counter
		hc.consecutiveSuccesses[node.ID] = 0
		hc.consecutiveFailures[node.ID]++

		// Check if we should transition to unhealthy
		if node.Status == repository.NodeStatusOnline &&
			hc.consecutiveFailures[node.ID] >= hc.config.UnhealthyThreshold {
			hc.transitionToUnhealthy(node, oldStatus, result.Message)
		}
	}
}

// transitionToHealthy transitions a node to healthy (online) status.
func (hc *HealthChecker) transitionToHealthy(node *repository.Node, oldStatus string) {
	if err := hc.nodeRepo.UpdateStatus(hc.ctx, node.ID, repository.NodeStatusOnline); err != nil {
		hc.logger.Error("Failed to update node status to online",
			logger.Err(err),
			logger.F("node_id", node.ID))
		return
	}

	hc.logger.Info("Node recovered to healthy",
		logger.F("node_id", node.ID),
		logger.F("node_name", node.Name),
		logger.F("old_status", oldStatus))

	// Send notification
	hc.sendStatusChangeNotification(node, oldStatus, repository.NodeStatusOnline, "Node recovered after consecutive successful health checks")

	// Trigger notification callback
	if hc.onStatusChange != nil {
		hc.onStatusChange(node.ID, oldStatus, repository.NodeStatusOnline)
	}
}

// transitionToOnline transitions a node to online status.
func (hc *HealthChecker) transitionToOnline(node *repository.Node, oldStatus string) {
	if err := hc.nodeRepo.UpdateStatus(hc.ctx, node.ID, repository.NodeStatusOnline); err != nil {
		hc.logger.Error("Failed to update node status to online",
			logger.Err(err),
			logger.F("node_id", node.ID))
		return
	}

	hc.logger.Info("Node came online",
		logger.F("node_id", node.ID),
		logger.F("node_name", node.Name),
		logger.F("old_status", oldStatus))

	// Send notification
	hc.sendStatusChangeNotification(node, oldStatus, repository.NodeStatusOnline, "Node came online")

	// Trigger notification callback
	if hc.onStatusChange != nil {
		hc.onStatusChange(node.ID, oldStatus, repository.NodeStatusOnline)
	}
}

// transitionToUnhealthy transitions a node to unhealthy status.
func (hc *HealthChecker) transitionToUnhealthy(node *repository.Node, oldStatus string, reason string) {
	if err := hc.nodeRepo.UpdateStatus(hc.ctx, node.ID, repository.NodeStatusUnhealthy); err != nil {
		hc.logger.Error("Failed to update node status to unhealthy",
			logger.Err(err),
			logger.F("node_id", node.ID))
		return
	}

	hc.logger.Warn("Node became unhealthy",
		logger.F("node_id", node.ID),
		logger.F("node_name", node.Name),
		logger.F("old_status", oldStatus),
		logger.F("reason", reason),
		logger.F("consecutive_failures", hc.consecutiveFailures[node.ID]))

	// Send notification
	hc.sendStatusChangeNotification(node, oldStatus, repository.NodeStatusUnhealthy, reason)

	// Trigger notification callback
	if hc.onStatusChange != nil {
		hc.onStatusChange(node.ID, oldStatus, repository.NodeStatusUnhealthy)
	}
}

// cleanupOldRecords removes health check records older than retention period.
func (hc *HealthChecker) cleanupOldRecords() {
	cutoff := time.Now().AddDate(0, 0, -hc.config.RetentionDays)
	deleted, err := hc.healthCheckRepo.DeleteOlderThan(hc.ctx, cutoff)
	if err != nil {
		hc.logger.Error("Failed to cleanup old health check records", logger.Err(err))
		return
	}
	if deleted > 0 {
		hc.logger.Debug("Cleaned up old health check records",
			logger.F("deleted", deleted),
			logger.F("cutoff", cutoff))
	}
}

// CheckNode performs a manual health check on a specific node.
func (hc *HealthChecker) CheckNode(ctx context.Context, nodeID int64) (*HealthCheckResult, error) {
	node, err := hc.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	result := hc.performCheck(node)

	// Save health check record
	check := &repository.HealthCheck{
		NodeID:    node.ID,
		Status:    result.Status,
		Latency:   result.Latency,
		Message:   result.Message,
		TCPOk:     result.TCPOk,
		APIOk:     result.APIOk,
		XrayOk:    result.XrayOk,
		CheckedAt: result.CheckedAt,
	}

	if err := hc.healthCheckRepo.Create(ctx, check); err != nil {
		hc.logger.Error("Failed to save health check record",
			logger.Err(err),
			logger.F("node_id", node.ID))
	}

	return result, nil
}

// CheckAll performs manual health checks on all nodes.
// Uses a worker pool to limit concurrent checks.
func (hc *HealthChecker) CheckAll(ctx context.Context) ([]*HealthCheckResult, error) {
	nodes, err := hc.nodeRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return []*HealthCheckResult{}, nil
	}

	results := make([]*HealthCheckResult, 0, len(nodes))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a semaphore to limit concurrent checks
	maxConcurrent := hc.config.MaxConcurrentChecks
	if maxConcurrent <= 0 {
		maxConcurrent = 10
	}
	sem := make(chan struct{}, maxConcurrent)

	for _, node := range nodes {
		wg.Add(1)

		// Acquire semaphore
		select {
		case sem <- struct{}{}:
			// Got a slot
		case <-ctx.Done():
			wg.Done()
			continue
		}

		go func(n *repository.Node) {
			defer wg.Done()
			defer func() { <-sem }()

			// Check for context cancellation
			select {
			case <-ctx.Done():
				return
			default:
			}

			result := hc.performCheck(n)

			// Save health check record
			check := &repository.HealthCheck{
				NodeID:    n.ID,
				Status:    result.Status,
				Latency:   result.Latency,
				Message:   result.Message,
				TCPOk:     result.TCPOk,
				APIOk:     result.APIOk,
				XrayOk:    result.XrayOk,
				CheckedAt: result.CheckedAt,
			}

			if err := hc.healthCheckRepo.Create(ctx, check); err != nil {
				hc.logger.Error("Failed to save health check record",
					logger.Err(err),
					logger.F("node_id", n.ID))
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(node)
	}

	wg.Wait()
	return results, nil
}

// GetHistory retrieves health check history for a node.
func (hc *HealthChecker) GetHistory(ctx context.Context, nodeID int64, limit int) ([]*repository.HealthCheck, error) {
	return hc.healthCheckRepo.GetByNodeID(ctx, nodeID, limit)
}

// GetConsecutiveFailures returns the current consecutive failure count for a node.
func (hc *HealthChecker) GetConsecutiveFailures(nodeID int64) int {
	hc.stateMu.RLock()
	defer hc.stateMu.RUnlock()
	return hc.consecutiveFailures[nodeID]
}

// GetConsecutiveSuccesses returns the current consecutive success count for a node.
func (hc *HealthChecker) GetConsecutiveSuccesses(nodeID int64) int {
	hc.stateMu.RLock()
	defer hc.stateMu.RUnlock()
	return hc.consecutiveSuccesses[nodeID]
}

// UpdateConfig updates the health checker configuration.
func (hc *HealthChecker) UpdateConfig(config *HealthCheckConfig) {
	hc.runningMu.Lock()
	defer hc.runningMu.Unlock()
	hc.config = config
	hc.httpClient.Timeout = config.Timeout
}

// GetConfig returns the current health checker configuration.
func (hc *HealthChecker) GetConfig() *HealthCheckConfig {
	hc.runningMu.Lock()
	defer hc.runningMu.Unlock()
	return hc.config
}

// sendStatusChangeNotification sends a notification when node status changes.
func (hc *HealthChecker) sendStatusChangeNotification(node *repository.Node, oldStatus, newStatus, reason string) {
	if hc.notificationSvc == nil {
		return
	}

	data := notification.NodeStatusChangeData{
		NodeID:    node.ID,
		NodeName:  node.Name,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Reason:    reason,
		Timestamp: time.Now(),
	}

	if err := hc.notificationSvc.NotifyNodeStatusChange(data); err != nil {
		hc.logger.Error("Failed to send node status change notification",
			logger.Err(err),
			logger.F("node_id", node.ID),
			logger.F("old_status", oldStatus),
			logger.F("new_status", newStatus))
	}
}

// checkCertificateExpiration 检查节点关联证书的过期状态
func (hc *HealthChecker) checkCertificateExpiration(node *repository.Node) string {
	// 如果节点没有关联证书，跳过检查
	if node.CertificateID == nil {
		return ""
	}

	// 获取证书信息
	cert, err := hc.certRepo.GetByID(hc.ctx, *node.CertificateID)
	if err != nil {
		hc.logger.Warn("Failed to get certificate for health check",
			logger.F("node_id", node.ID),
			logger.F("cert_id", *node.CertificateID),
			logger.Err(err))
		return ""
	}

	// 检查证书过期时间
	if cert.ExpiresAt.IsZero() {
		return ""
	}

	now := time.Now()
	timeUntilExpiry := cert.ExpiresAt.Sub(now)
	daysLeft := int(timeUntilExpiry.Hours() / 24)

	// 证书已过期
	if daysLeft < 0 {
		warning := fmt.Sprintf("证书已过期 %d 天", -daysLeft)
		hc.logger.Error("Certificate expired",
			logger.F("node_id", node.ID),
			logger.F("node_name", node.Name),
			logger.F("domain", cert.Domain),
			logger.F("days_expired", -daysLeft))

		// TODO: 发送告警通知（需要实现通知服务）

		return warning
	}

	// 证书即将过期（30天内）
	if daysLeft <= 30 {
		warning := fmt.Sprintf("证书将在 %d 天后过期", daysLeft)

		// 仅在7天内记录警告日志
		if daysLeft <= 7 {
			hc.logger.Warn("Certificate expiring soon",
				logger.F("node_id", node.ID),
				logger.F("node_name", node.Name),
				logger.F("domain", cert.Domain),
				logger.F("days_left", daysLeft))

			// TODO: 发送告警通知（需要实现通知服务）
		}

		return warning
	}

	return ""
}
