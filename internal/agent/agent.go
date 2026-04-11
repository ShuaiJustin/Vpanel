// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"v/internal/logger"
)

const (
	xrayWatchdogInterval = 15 * time.Second
	xraySelfHealCooldown = 20 * time.Second
	xraySelfHealTimeout  = 45 * time.Second
)

type heartbeatTrafficReporter interface {
	PrepareDelta(ctx context.Context) (*TrafficSnapshot, []TrafficRecord, error)
	Commit(snapshot *TrafficSnapshot)
}

type heartbeatTrafficStatusReporter interface {
	heartbeatTrafficReporter
	GetCollectorStatus() *TrafficCollectorStatus
}

type heartbeatTrafficStateReporter interface {
	heartbeatTrafficReporter
	ExportCommittedCounters() map[string]int64
	RestoreCommittedCounters(map[string]int64)
}

type heartbeatSessionReporter interface {
	CollectRecentSessions(ctx context.Context) ([]ProxySessionRecord, error)
}

type pendingTrafficBatch struct {
	batchID  string
	snapshot *TrafficSnapshot
	records  []TrafficRecord
}

type persistedTrafficState struct {
	CommittedCounters map[string]int64              `json:"committed_counters,omitempty"`
	PendingBatch      *persistedPendingTrafficBatch `json:"pending_batch,omitempty"`
}

type persistedPendingTrafficBatch struct {
	BatchID  string           `json:"batch_id"`
	Snapshot *TrafficSnapshot `json:"-"`
	Counters map[string]int64 `json:"counters,omitempty"`
	Records  []TrafficRecord  `json:"records,omitempty"`
}

// Agent represents the Node Agent that runs on each Xray node.
type Agent struct {
	config     *Config
	logger     logger.Logger
	httpClient *http.Client

	// Components
	xrayManager      *XrayManager
	healthServer     *HealthServer
	panelClient      *PanelClient
	metricsCollector *MetricsCollector
	commandExecutor  *CommandExecutor
	trafficReporter  heartbeatTrafficReporter
	sessionReporter  heartbeatSessionReporter

	// State
	mu                  sync.RWMutex
	running             bool
	registered          bool
	nodeID              int64
	pendingTraffic      *pendingTrafficBatch
	authFailureStop     bool
	authFailureReason   string
	lastXrayHealAttempt time.Time
	xrayHealInProgress  bool

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NodeMetrics represents metrics collected from the node.
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

// RegisterRequest represents a registration request to the Panel.
type RegisterRequest struct {
	Token   string `json:"token"`
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// RegisterResponse represents a registration response from the Panel.
type RegisterResponse struct {
	Success bool   `json:"success"`
	NodeID  int64  `json:"node_id"`
	Message string `json:"message"`
}

// HeartbeatRequest represents a heartbeat request to the Panel.
type HeartbeatRequest struct {
	NodeID         int64                `json:"node_id"`
	Token          string               `json:"token"`
	Metrics        *NodeMetrics         `json:"metrics"`
	Traffic        []TrafficRecord      `json:"traffic,omitempty"`
	ProxySessions  []ProxySessionRecord `json:"proxy_sessions,omitempty"`
	TrafficBatchID string               `json:"traffic_batch_id,omitempty"`
}

// HeartbeatResponse represents a heartbeat response from the Panel.
type HeartbeatResponse struct {
	Success  bool      `json:"success"`
	Message  string    `json:"message"`
	Commands []Command `json:"commands,omitempty"`
}

// Command represents a command from the Panel to execute.
type Command struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// CommandResult represents the result of executing a command.
type CommandResult struct {
	CommandID string `json:"command_id"`
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Data      any    `json:"data,omitempty"`
}

// New creates a new Node Agent.
func New(cfg *Config, log logger.Logger) (*Agent, error) {
	httpClient := &http.Client{
		Timeout: cfg.Panel.ConnectTimeout,
	}

	agent := &Agent{
		config:     cfg,
		logger:     log,
		httpClient: httpClient,
	}

	// Initialize Xray manager
	agent.xrayManager = NewXrayManager(XrayManagerConfig{
		BinaryPath: cfg.Xray.BinaryPath,
		ConfigPath: cfg.Xray.ConfigPath,
		BackupDir:  cfg.Xray.BackupDir,
		Stdout:     &logWriter{logger: log, prefix: "[Xray-stdout]"},
		Stderr:     &logWriter{logger: log, prefix: "[Xray-stderr]"},
	}, log)

	// Initialize health server
	agent.healthServer = NewHealthServer(HealthServerConfig{
		Host: cfg.Health.Host,
		Port: cfg.Health.Port,
	}, agent, log)

	// Initialize panel client
	agent.panelClient = NewPanelClient(PanelClientConfig{
		URL:               cfg.Panel.URL,
		Token:             cfg.Node.Token,
		TLSSkipVerify:     cfg.Panel.TLSSkipVerify,
		ConnectTimeout:    cfg.Panel.ConnectTimeout,
		ReconnectInterval: cfg.Panel.ReconnectInterval,
		MaxReconnectDelay: cfg.Panel.MaxReconnectDelay,
	}, log)

	// Initialize metrics collector
	agent.metricsCollector = NewMetricsCollector(log)

	// Initialize command executor
	agent.commandExecutor = NewCommandExecutor(agent, log)
	agent.trafficReporter = newTrafficReporter(cfg.Xray, log)
	agent.sessionReporter = newSessionReporter(cfg.Xray, log)
	agent.restoreTrafficState()

	return agent, nil
}

// Start starts the Node Agent.
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent is already running")
	}
	a.running = true
	a.ctx, a.cancel = context.WithCancel(ctx)
	a.mu.Unlock()

	// Ensure Xray is installed
	installer := NewXrayInstaller(a.logger)
	if err := installer.EnsureXrayInstalled(ctx, a.config.Xray.ConfigPath); err != nil {
		a.logger.Error("Xray 安装检查失败", logger.F("error", err.Error()))
		// Continue anyway - Xray might be installed in a custom location
	} else {
		a.logger.Info("Xray 安装检查完成")
	}

	// Start Xray service if not running
	if err := a.ensureXrayRunning(ctx); err != nil {
		a.logger.Error("Xray 启动失败", logger.F("error", err.Error()))
		// Continue anyway - Xray might be managed externally or will be started later
	}

	// Start health server
	if err := a.healthServer.Start(); err != nil {
		return fmt.Errorf("failed to start health server: %w", err)
	}

	// Register with Panel
	if err := a.register(); err != nil {
		a.logger.Warn("initial registration failed, will retry",
			logger.F("error", err.Error()))
	}

	// Start heartbeat loop
	a.wg.Add(1)
	go a.heartbeatLoop()

	// Start Xray watchdog loop
	a.wg.Add(1)
	go a.xrayWatchdogLoop()

	// Start command processor
	a.wg.Add(1)
	go a.commandProcessorLoop()

	a.logger.Info("agent started successfully")
	return nil
}

// Stop stops the Node Agent.
func (a *Agent) Stop(ctx context.Context) error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return nil
	}
	a.cancel()
	a.running = false
	a.mu.Unlock()

	// Stop health server
	if err := a.healthServer.Stop(ctx); err != nil {
		a.logger.Error("failed to stop health server", logger.F("error", err))
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("agent stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// register registers the agent with the Panel Server.
func (a *Agent) register() error {
	req := &RegisterRequest{
		Token:   a.config.Node.Token,
		Name:    a.config.Node.Name,
		Version: "1.0.0",
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}

	resp, err := a.panelClient.Register(a.ctx, req)
	if err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration rejected: %s", resp.Message)
	}

	a.mu.Lock()
	a.nodeID = resp.NodeID
	a.registered = true
	a.mu.Unlock()

	a.logger.Info("registered with panel",
		logger.F("node_id", resp.NodeID),
		logger.F("message", resp.Message))

	return nil
}

// HeartbeatConfig holds configuration for heartbeat behavior.
type HeartbeatConfig struct {
	Interval      time.Duration
	RetryInterval time.Duration
	MaxRetries    int
}

// DefaultHeartbeatConfig returns default heartbeat configuration.
func DefaultHeartbeatConfig() *HeartbeatConfig {
	return &HeartbeatConfig{
		Interval:      30 * time.Second,
		RetryInterval: 5 * time.Second,
		MaxRetries:    3,
	}
}

// heartbeatLoop sends periodic heartbeats to the Panel.
func (a *Agent) heartbeatLoop() {
	defer a.wg.Done()

	config := DefaultHeartbeatConfig()
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	// Send initial heartbeat
	a.sendHeartbeat()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat to the Panel.
func (a *Agent) sendHeartbeat() {
	a.mu.RLock()
	authFailureStop := a.authFailureStop
	a.mu.RUnlock()

	if authFailureStop {
		return
	}

	// 读取当前状态（避免长时间持有锁）
	a.mu.RLock()
	registered := a.registered
	nodeID := a.nodeID
	a.mu.RUnlock()

	if !registered {
		// Keep retrying registration with backoff. Panel outages are transient and
		// the agent must resume heartbeats automatically once the panel is back.
		if a.panelClient.ShouldReconnect() {
			if err := a.panelClient.WaitForReconnect(a.ctx); err != nil {
				return // Context cancelled
			}
			if err := a.register(); err != nil {
				if isPermanentAuthError(err) {
					a.markPermanentAuthFailure(err)
					return
				}
				a.logger.Warn("registration failed during heartbeat",
					logger.F("error", err.Error()),
					logger.F("consecutive_fails", a.panelClient.GetConsecutiveFails()))
				// 注册失败时更新状态
				a.mu.Lock()
				a.registered = false
				a.mu.Unlock()
			}
		}
		return
	}

	// Collect metrics
	metrics := a.collectMetrics()
	var trafficSnapshot *TrafficSnapshot
	var trafficRecords []TrafficRecord
	var trafficBatchID string
	var proxySessions []ProxySessionRecord
	if a.trafficReporter != nil {
		snapshot, records, batchID, err := a.prepareHeartbeatTraffic(nodeID)
		if err != nil {
			a.logger.Warn("failed to collect xray traffic stats",
				logger.F("node_id", nodeID),
				logger.F("error", err.Error()))
		} else {
			trafficSnapshot = snapshot
			trafficRecords = records
			trafficBatchID = batchID
		}
	}
	if a.sessionReporter != nil {
		sessions, err := a.sessionReporter.CollectRecentSessions(a.ctx)
		if err != nil {
			a.logger.Warn("failed to collect proxy session activity",
				logger.F("node_id", nodeID),
				logger.F("error", err.Error()))
		} else {
			proxySessions = sessions
		}
	}

	req := &HeartbeatRequest{
		NodeID:         nodeID,
		Token:          a.config.Node.Token,
		Metrics:        metrics,
		Traffic:        trafficRecords,
		ProxySessions:  proxySessions,
		TrafficBatchID: trafficBatchID,
	}

	resp, err := a.panelClient.Heartbeat(a.ctx, req)
	if err != nil {
		if isPermanentAuthError(err) {
			a.markPermanentAuthFailure(err)
			return
		}
		a.logger.Error("heartbeat failed",
			logger.F("error", err.Error()),
			logger.F("node_id", nodeID),
			logger.F("consecutive_fails", a.panelClient.GetConsecutiveFails()))

		// Check if we need to re-register
		a.mu.Lock()
		a.registered = false
		a.mu.Unlock()
		return
	}

	if !resp.Success {
		a.logger.Warn("heartbeat rejected",
			logger.F("message", resp.Message),
			logger.F("node_id", nodeID))
		return
	}

	a.logger.Debug("heartbeat sent successfully",
		logger.F("node_id", nodeID),
		logger.F("commands_received", len(resp.Commands)),
		logger.F("traffic_records", len(trafficRecords)),
		logger.F("proxy_sessions", len(proxySessions)))

	if trafficSnapshot != nil {
		a.acknowledgeHeartbeatTraffic(trafficBatchID, trafficSnapshot)
	}

	// Process any commands from the response
	if len(resp.Commands) > 0 {
		a.processCommands(resp.Commands)
	}
}

func (a *Agent) prepareHeartbeatTraffic(nodeID int64) (*TrafficSnapshot, []TrafficRecord, string, error) {
	if pending := a.loadPendingTrafficBatch(); pending != nil {
		return pending.snapshot, pending.records, pending.batchID, nil
	}

	snapshot, records, err := a.trafficReporter.PrepareDelta(a.ctx)
	if err != nil {
		return nil, nil, "", err
	}
	if len(records) == 0 {
		return snapshot, nil, "", nil
	}

	batchID := buildTrafficBatchID(nodeID, snapshot, records)
	pending := &pendingTrafficBatch{
		batchID:  batchID,
		snapshot: snapshot,
		records:  cloneTrafficRecords(records),
	}

	a.mu.Lock()
	a.pendingTraffic = pending
	a.mu.Unlock()
	a.persistTrafficState()

	return snapshot, cloneTrafficRecords(records), batchID, nil
}

func (a *Agent) acknowledgeHeartbeatTraffic(batchID string, snapshot *TrafficSnapshot) {
	if snapshot == nil || a.trafficReporter == nil {
		return
	}

	a.trafficReporter.Commit(snapshot)
	if batchID == "" {
		return
	}

	a.mu.Lock()
	if a.pendingTraffic != nil && a.pendingTraffic.batchID == batchID {
		a.pendingTraffic = nil
	}
	a.mu.Unlock()
	a.persistTrafficState()
}

func (a *Agent) loadPendingTrafficBatch() *pendingTrafficBatch {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.pendingTraffic == nil {
		return nil
	}

	return &pendingTrafficBatch{
		batchID:  a.pendingTraffic.batchID,
		snapshot: a.pendingTraffic.snapshot,
		records:  cloneTrafficRecords(a.pendingTraffic.records),
	}
}

func buildTrafficBatchID(nodeID int64, snapshot *TrafficSnapshot, records []TrafficRecord) string {
	type batchRecord struct {
		UserID   int64 `json:"user_id"`
		ProxyID  int64 `json:"proxy_id"`
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}
	type batchCounter struct {
		Name  string `json:"name"`
		Value int64  `json:"value"`
	}

	normalizedRecords := make([]batchRecord, 0, len(records))
	for _, record := range records {
		proxyID := int64(0)
		if record.ProxyID != nil {
			proxyID = *record.ProxyID
		}
		normalizedRecords = append(normalizedRecords, batchRecord{
			UserID:   record.UserID,
			ProxyID:  proxyID,
			Upload:   record.Upload,
			Download: record.Download,
		})
	}
	sort.Slice(normalizedRecords, func(i, j int) bool {
		if normalizedRecords[i].UserID != normalizedRecords[j].UserID {
			return normalizedRecords[i].UserID < normalizedRecords[j].UserID
		}
		if normalizedRecords[i].ProxyID != normalizedRecords[j].ProxyID {
			return normalizedRecords[i].ProxyID < normalizedRecords[j].ProxyID
		}
		if normalizedRecords[i].Upload != normalizedRecords[j].Upload {
			return normalizedRecords[i].Upload < normalizedRecords[j].Upload
		}
		return normalizedRecords[i].Download < normalizedRecords[j].Download
	})

	counters := make([]batchCounter, 0)
	if snapshot != nil {
		counters = make([]batchCounter, 0, len(snapshot.counters))
		for name, value := range snapshot.counters {
			counters = append(counters, batchCounter{Name: name, Value: value})
		}
		sort.Slice(counters, func(i, j int) bool {
			return counters[i].Name < counters[j].Name
		})
	}

	payload, err := json.Marshal(struct {
		NodeID   int64          `json:"node_id"`
		Records  []batchRecord  `json:"records"`
		Counters []batchCounter `json:"counters"`
	}{
		NodeID:   nodeID,
		Records:  normalizedRecords,
		Counters: counters,
	})
	if err != nil {
		payload = []byte(fmt.Sprintf("%d:%d:%d", nodeID, len(normalizedRecords), len(counters)))
	}

	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func cloneTrafficRecords(records []TrafficRecord) []TrafficRecord {
	if len(records) == 0 {
		return nil
	}

	cloned := make([]TrafficRecord, len(records))
	for i, record := range records {
		cloned[i] = record
		if record.ProxyID != nil {
			proxyID := *record.ProxyID
			cloned[i].ProxyID = &proxyID
		}
	}
	return cloned
}

func cloneTrafficSnapshot(snapshot *TrafficSnapshot) *TrafficSnapshot {
	if snapshot == nil {
		return nil
	}

	clonedCounters := make(map[string]int64, len(snapshot.counters))
	for name, value := range snapshot.counters {
		clonedCounters[name] = value
	}

	return &TrafficSnapshot{counters: clonedCounters}
}

func cloneCommittedCounters(counters map[string]int64) map[string]int64 {
	if len(counters) == 0 {
		return nil
	}

	cloned := make(map[string]int64, len(counters))
	for name, value := range counters {
		cloned[name] = value
	}
	return cloned
}

func (a *Agent) trafficStatePath() string {
	if a == nil || a.config == nil {
		return ""
	}

	stateDir := strings.TrimSpace(a.config.Xray.BackupDir)
	if stateDir == "" {
		stateDir = filepath.Dir(a.config.Xray.ConfigPath)
	}
	if strings.TrimSpace(stateDir) == "" {
		return ""
	}

	return filepath.Join(stateDir, "traffic-state.json")
}

func (a *Agent) restoreTrafficState() {
	stateReporter, ok := a.trafficReporter.(heartbeatTrafficStateReporter)
	if a == nil || !ok {
		return
	}

	path := a.trafficStatePath()
	if path == "" {
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			a.logger.Warn("failed to read persisted traffic state", logger.F("path", path), logger.F("error", err.Error()))
		}
		return
	}

	var state persistedTrafficState
	if err := json.Unmarshal(data, &state); err != nil {
		a.logger.Warn("failed to parse persisted traffic state", logger.F("path", path), logger.F("error", err.Error()))
		return
	}

	stateReporter.RestoreCommittedCounters(state.CommittedCounters)
	if state.PendingBatch != nil {
		a.pendingTraffic = &pendingTrafficBatch{
			batchID: state.PendingBatch.BatchID,
			snapshot: &TrafficSnapshot{
				counters: cloneCommittedCounters(state.PendingBatch.Counters),
			},
			records: cloneTrafficRecords(state.PendingBatch.Records),
		}
		if len(state.PendingBatch.Counters) == 0 {
			a.pendingTraffic.snapshot = nil
		}
	}

	a.logger.Info("restored persisted traffic state",
		logger.F("path", path),
		logger.F("committed_counters", len(state.CommittedCounters)),
		logger.F("has_pending_batch", state.PendingBatch != nil),
	)
}

func (a *Agent) persistTrafficState() {
	stateReporter, ok := a.trafficReporter.(heartbeatTrafficStateReporter)
	if a == nil || !ok {
		return
	}

	path := a.trafficStatePath()
	if path == "" {
		return
	}

	state := persistedTrafficState{
		CommittedCounters: cloneCommittedCounters(stateReporter.ExportCommittedCounters()),
	}

	pending := a.loadPendingTrafficBatch()
	if pending != nil {
		counters := map[string]int64(nil)
		if pending.snapshot != nil {
			counters = cloneCommittedCounters(pending.snapshot.counters)
		}
		state.PendingBatch = &persistedPendingTrafficBatch{
			BatchID:  pending.batchID,
			Counters: counters,
			Records:  cloneTrafficRecords(pending.records),
		}
	}

	if len(state.CommittedCounters) == 0 && state.PendingBatch == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			a.logger.Warn("failed to remove persisted traffic state", logger.F("path", path), logger.F("error", err.Error()))
		}
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		a.logger.Warn("failed to ensure traffic state directory", logger.F("path", path), logger.F("error", err.Error()))
		return
	}

	data, err := json.Marshal(state)
	if err != nil {
		a.logger.Warn("failed to marshal traffic state", logger.F("path", path), logger.F("error", err.Error()))
		return
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		a.logger.Warn("failed to write temporary traffic state", logger.F("path", tmpPath), logger.F("error", err.Error()))
		return
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		a.logger.Warn("failed to persist traffic state", logger.F("path", path), logger.F("error", err.Error()))
	}
}

func (a *Agent) markPermanentAuthFailure(err error) {
	if err == nil {
		return
	}

	a.mu.Lock()
	if a.authFailureStop {
		a.mu.Unlock()
		return
	}
	a.authFailureStop = true
	a.authFailureReason = err.Error()
	a.registered = false
	a.mu.Unlock()

	a.logger.Error("permanent authentication failure detected; stop retrying panel registration",
		logger.F("error", err.Error()))
}

// collectMetrics collects current node metrics.
func (a *Agent) collectMetrics() *NodeMetrics {
	metrics := a.metricsCollector.Collect()

	// Add Xray status
	status := a.currentXrayStatus()
	metrics.XrayRunning = status.Running
	metrics.XrayVersion = strings.TrimSpace(status.Version)

	if !status.Running {
		a.triggerXraySelfHeal("heartbeat metrics detected xray stopped")
	}

	return metrics
}

// commandProcessorLoop processes commands from the Panel.
func (a *Agent) commandProcessorLoop() {
	defer a.wg.Done()

	// This loop handles any async command processing
	// Commands are primarily received via heartbeat responses
	<-a.ctx.Done()
}

// processCommands processes commands received from the Panel.
func (a *Agent) processCommands(commands []Command) {
	for _, cmd := range commands {
		result := a.commandExecutor.Execute(a.ctx, &cmd)

		// Report command result back to Panel
		if err := a.panelClient.ReportCommandResult(a.ctx, result); err != nil {
			a.logger.Error("failed to report command result",
				logger.F("command_id", cmd.ID),
				logger.F("error", err.Error()))
		}
	}
}

// executeCommand executes a single command (legacy method for backward compatibility).
func (a *Agent) executeCommand(cmd Command) *CommandResult {
	return a.commandExecutor.Execute(a.ctx, &cmd)
}

// IsRunning returns whether the agent is running.
func (a *Agent) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// IsRegistered returns whether the agent is registered with the Panel.
func (a *Agent) IsRegistered() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.registered
}

// GetNodeID returns the node ID assigned by the Panel.
func (a *Agent) GetNodeID() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.nodeID
}

// GetXrayStatus returns the current Xray status.
func (a *Agent) GetXrayStatus() *XrayStatus {
	return a.currentXrayStatus()
}

// GetMetrics returns current node metrics.
func (a *Agent) GetMetrics() *NodeMetrics {
	return a.collectMetrics()
}

// GetTrafficCollectorStatus returns the current traffic collection status.
func (a *Agent) GetTrafficCollectorStatus() *TrafficCollectorStatus {
	statusReporter, ok := a.trafficReporter.(heartbeatTrafficStatusReporter)
	if !ok {
		return &TrafficCollectorStatus{
			Status:      TrafficCollectorStatusUnknown,
			XrayRunning: a.currentXrayStatus().Running,
		}
	}

	status := statusReporter.GetCollectorStatus()
	if status == nil {
		status = &TrafficCollectorStatus{Status: TrafficCollectorStatusUnknown}
	}
	status.XrayRunning = a.currentXrayStatus().Running
	return status
}

// GetXrayVersion returns the Xray version string.
func GetXrayVersion(binaryPath string) string {
	cmd := exec.Command(binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func (a *Agent) xrayWatchdogLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(xrayWatchdogInterval)
	defer ticker.Stop()

	a.triggerXraySelfHeal("agent startup watchdog")

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if !a.currentXrayStatus().Running {
				a.triggerXraySelfHeal("watchdog detected xray stopped")
			}
		}
	}
}

func (a *Agent) triggerXraySelfHeal(reason string) {
	if a.ctx == nil || a.ctx.Err() != nil {
		return
	}
	if a.currentXrayStatus().Running {
		return
	}

	a.mu.Lock()
	if a.xrayHealInProgress {
		a.mu.Unlock()
		return
	}
	if !a.lastXrayHealAttempt.IsZero() && time.Since(a.lastXrayHealAttempt) < xraySelfHealCooldown {
		a.mu.Unlock()
		return
	}
	a.xrayHealInProgress = true
	a.lastXrayHealAttempt = time.Now()
	a.mu.Unlock()

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() {
			a.mu.Lock()
			a.xrayHealInProgress = false
			a.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(a.ctx, xraySelfHealTimeout)
		defer cancel()

		a.logger.Warn("检测到 Xray 未运行，开始自动拉起/自愈", logger.F("reason", reason))
		if err := a.ensureXrayRunning(ctx); err != nil {
			a.logger.Error("Xray 自动拉起/自愈失败",
				logger.F("reason", reason),
				logger.F("error", err.Error()))
			return
		}

		status := a.currentXrayStatus()
		if status.Running {
			a.logger.Info("Xray 自动拉起/自愈成功",
				logger.F("reason", reason),
				logger.F("pid", status.PID))
		}
	}()
}

func (a *Agent) currentXrayStatus() *XrayStatus {
	status := a.xrayManager.GetStatus()
	if status == nil {
		status = &XrayStatus{Version: "unknown"}
	}
	if status.Running {
		status.Version = strings.TrimSpace(status.Version)
		return status
	}
	if !a.isXrayRunning() {
		status.Version = strings.TrimSpace(status.Version)
		return status
	}

	resolved := *status
	resolved.Running = true
	resolved.PID = a.lookupXrayPID()
	if strings.TrimSpace(resolved.Version) == "" || strings.TrimSpace(resolved.Version) == "unknown" {
		resolved.Version = GetXrayVersion(a.resolveXrayBinaryPath())
	} else {
		resolved.Version = strings.TrimSpace(resolved.Version)
	}
	return &resolved
}

// ensureXrayRunning ensures Xray service is running.
func (a *Agent) ensureXrayRunning(ctx context.Context) error {
	if a.currentXrayStatus().Running {
		a.logger.Info("Xray 服务已在运行")
		return nil
	}

	a.logger.Info("Xray 未运行，尝试启动...")

	xrayBinaryPath := a.resolveXrayBinaryPath()
	if _, err := os.Stat(xrayBinaryPath); err != nil {
		if _, lookErr := exec.LookPath(xrayBinaryPath); lookErr != nil {
			a.logger.Error("未找到 Xray 可执行文件", logger.F("binary_path", xrayBinaryPath), logger.F("error", lookErr.Error()))
			return fmt.Errorf("xray binary not found: %w", lookErr)
		}
	}

	if _, err := os.Stat(a.config.Xray.ConfigPath); err != nil {
		a.logger.Error("Xray 配置文件不存在",
			logger.F("config_path", a.config.Xray.ConfigPath),
			logger.F("error", err.Error()))
		return fmt.Errorf("xray config not found: %w", err)
	}

	if runtime.GOOS == "linux" && shouldUseSystemdXray(a.config.Xray.ConfigPath) {
		a.logger.Info("尝试通过 systemctl 启动 Xray...")
		cmd := exec.CommandContext(ctx, "systemctl", "start", "xray")
		output, err := cmd.CombinedOutput()
		if err != nil {
			a.logger.Warn("systemctl 启动失败",
				logger.F("error", err.Error()),
				logger.F("output", string(output)))
		} else {
			time.Sleep(2 * time.Second)
			if a.currentXrayStatus().Running {
				a.logger.Info("Xray 服务通过 systemctl 启动成功")
				return nil
			}
			a.logger.Warn("systemctl 启动后 Xray 仍未运行，尝试直接启动")
		}
	}

	return a.startXrayDirect(ctx)
}

func (a *Agent) restartXray(ctx context.Context) error {
	if runtime.GOOS == "linux" && shouldUseSystemdXray(a.config.Xray.ConfigPath) {
		a.logger.Info("尝试通过 systemctl 重启 Xray...")
		cmd := exec.CommandContext(ctx, "systemctl", "restart", "xray")
		output, err := cmd.CombinedOutput()
		if err != nil {
			a.logger.Warn("systemctl 重启失败",
				logger.F("error", err.Error()),
				logger.F("output", string(output)))
		} else {
			time.Sleep(2 * time.Second)
			if a.currentXrayStatus().Running {
				a.logger.Info("Xray 服务通过 systemctl 重启成功")
				return nil
			}
			a.logger.Warn("systemctl 重启完成，但 Xray 仍未运行，尝试回退到直接拉起")
		}
	}

	if err := a.xrayManager.Restart(ctx); err != nil {
		a.logger.Warn("直接重启 Xray 失败，尝试兜底拉起",
			logger.F("error", err.Error()))
	} else {
		time.Sleep(2 * time.Second)
		if a.currentXrayStatus().Running {
			a.logger.Info("Xray 直接重启成功")
			return nil
		}
		a.logger.Warn("直接重启完成，但 Xray 仍未运行，尝试兜底拉起")
	}

	return a.ensureXrayRunning(ctx)
}

func (a *Agent) resolveXrayBinaryPath() string {
	if path := strings.TrimSpace(a.config.Xray.BinaryPath); path != "" {
		return path
	}
	return "xray"
}

// isXrayRunning checks if Xray process is running.
func (a *Agent) isXrayRunning() bool {
	return a.lookupXrayPID() > 0
}

func (a *Agent) lookupXrayPID() int {
	managedStatus := a.xrayManager.GetStatus()
	if managedStatus != nil && managedStatus.Running && managedStatus.PID > 0 {
		return managedStatus.PID
	}

	output, err := exec.Command("ps", "-eo", "pid=", "-o", "comm=", "-o", "args=").Output()
	if err == nil {
		return findConfiguredXrayPID(string(output), a.config.Xray.ConfigPath)
	}
	return 0
}

func parsePIDOutput(output []byte) int {
	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

// startXrayDirect starts Xray process directly.
func (a *Agent) startXrayDirect(ctx context.Context) error {
	xrayBinaryPath := a.resolveXrayBinaryPath()
	a.logger.Info("直接启动 Xray 进程...",
		logger.F("binary_path", xrayBinaryPath),
		logger.F("config_path", a.config.Xray.ConfigPath))

	if err := a.xrayManager.Start(ctx); err != nil {
		a.logger.Error("启动 Xray 进程失败", logger.F("error", err.Error()))
		return fmt.Errorf("failed to start xray: %w", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a.currentXrayStatus().Running {
			a.logger.Info("Xray 启动成功")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	if a.currentXrayStatus().Running {
		a.logger.Info("Xray 启动成功")
		return nil
	}

	a.logger.Error("Xray 启动失败：进程未运行")
	return fmt.Errorf("xray failed to start")
}

// logWriter implements io.Writer to redirect Xray output to logger
type logWriter struct {
	logger logger.Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Info(w.prefix, logger.F("output", string(p)))
	return len(p), nil
}

func shouldUseSystemdXray(configPath string) bool {
	clean := filepath.Clean(strings.TrimSpace(configPath))
	switch clean {
	case "/etc/xray/config.json", "/usr/local/etc/xray/config.json":
		return true
	default:
		return false
	}
}

func findConfiguredXrayPID(psOutput, configPath string) int {
	cleanConfigPath := filepath.Clean(strings.TrimSpace(configPath))
	if cleanConfigPath == "" {
		return 0
	}

	var pids []int
	for _, line := range strings.Split(psOutput, "\n") {
		pid, args, ok := parseXrayProcessLine(line)
		if !ok {
			continue
		}
		if strings.Contains(args, cleanConfigPath) {
			pids = append(pids, pid)
		}
	}

	if len(pids) == 0 {
		return 0
	}
	sort.Ints(pids)
	return pids[0]
}

func parseXrayProcessLine(line string) (int, string, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 3 {
		return 0, "", false
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil || pid <= 0 {
		return 0, "", false
	}

	comm := fields[1]
	if comm != "xray" && !strings.HasSuffix(comm, "/xray") {
		return 0, "", false
	}

	return pid, strings.Join(fields[2:], " "), true
}
