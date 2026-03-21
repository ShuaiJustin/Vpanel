// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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
	trafficReporter  *trafficReporter

	// State
	mu                  sync.RWMutex
	running             bool
	registered          bool
	nodeID              int64
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
	NodeID  int64           `json:"node_id"`
	Token   string          `json:"token"`
	Metrics *NodeMetrics    `json:"metrics"`
	Traffic []TrafficRecord `json:"traffic,omitempty"`
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
	// 读取当前状态（避免长时间持有锁）
	a.mu.RLock()
	registered := a.registered
	nodeID := a.nodeID
	a.mu.RUnlock()

	if !registered {
		// Try to register first with reconnection logic
		if a.panelClient.ShouldReconnect() {
			if err := a.panelClient.WaitForReconnect(a.ctx); err != nil {
				return // Context cancelled
			}
			if err := a.register(); err != nil {
				a.logger.Warn("registration failed during heartbeat",
					logger.F("error", err.Error()),
					logger.F("consecutive_fails", a.panelClient.GetConsecutiveFails()))
				// 注册失败时更新状态
				a.mu.Lock()
				a.registered = false
				a.mu.Unlock()
			}
		} else {
			a.logger.Error("max reconnection attempts reached, giving up",
				logger.F("consecutive_fails", a.panelClient.GetConsecutiveFails()))
		}
		return
	}

	// Collect metrics
	metrics := a.collectMetrics()
	var trafficSnapshot *TrafficSnapshot
	var trafficRecords []TrafficRecord
	if a.trafficReporter != nil {
		snapshot, records, err := a.trafficReporter.PrepareDelta(a.ctx)
		if err != nil {
			a.logger.Warn("failed to collect xray traffic stats",
				logger.F("node_id", nodeID),
				logger.F("error", err.Error()))
		} else {
			trafficSnapshot = snapshot
			trafficRecords = records
		}
	}

	req := &HeartbeatRequest{
		NodeID:  nodeID,
		Token:   a.config.Node.Token,
		Metrics: metrics,
		Traffic: trafficRecords,
	}

	resp, err := a.panelClient.Heartbeat(a.ctx, req)
	if err != nil {
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
		logger.F("traffic_records", len(trafficRecords)))

	if trafficSnapshot != nil {
		a.trafficReporter.Commit(trafficSnapshot)
	}

	// Process any commands from the response
	if len(resp.Commands) > 0 {
		a.processCommands(resp.Commands)
	}
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
	if a.isXrayRunning() {
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

	if runtime.GOOS == "linux" {
		a.logger.Info("尝试通过 systemctl 启动 Xray...")
		cmd := exec.CommandContext(ctx, "systemctl", "start", "xray")
		output, err := cmd.CombinedOutput()
		if err != nil {
			a.logger.Warn("systemctl 启动失败",
				logger.F("error", err.Error()),
				logger.F("output", string(output)))
		} else {
			time.Sleep(2 * time.Second)
			if a.isXrayRunning() {
				a.logger.Info("Xray 服务通过 systemctl 启动成功")
				return nil
			}
			a.logger.Warn("systemctl 启动后 Xray 仍未运行，尝试直接启动")
		}
	}

	return a.startXrayDirect(ctx)
}

func (a *Agent) restartXray(ctx context.Context) error {
	if runtime.GOOS == "linux" {
		a.logger.Info("尝试通过 systemctl 重启 Xray...")
		cmd := exec.CommandContext(ctx, "systemctl", "restart", "xray")
		output, err := cmd.CombinedOutput()
		if err != nil {
			a.logger.Warn("systemctl 重启失败",
				logger.F("error", err.Error()),
				logger.F("output", string(output)))
		} else {
			time.Sleep(2 * time.Second)
			if a.isXrayRunning() {
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
		if a.isXrayRunning() {
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
	if runtime.GOOS == "linux" {
		cmd := exec.Command("systemctl", "is-active", "xray")
		if err := cmd.Run(); err == nil {
			return true
		}
	}

	cmd := exec.Command("pgrep", "-x", "xray")
	if err := cmd.Run(); err == nil {
		return true
	}

	return false
}

func (a *Agent) lookupXrayPID() int {
	if runtime.GOOS == "linux" {
		output, err := exec.Command("systemctl", "show", "-p", "MainPID", "--value", "xray").Output()
		if err == nil {
			if pid := parsePIDOutput(output); pid > 0 {
				return pid
			}
		}
	}

	output, err := exec.Command("pgrep", "-o", "-x", "xray").Output()
	if err == nil {
		return parsePIDOutput(output)
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

	configData, err := os.ReadFile(a.config.Xray.ConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read xray config: %w", err)
	}
	if err := a.xrayManager.ValidateConfig(ctx, configData); err != nil {
		a.logger.Error("Xray 配置验证失败", logger.F("error", err.Error()))
		return fmt.Errorf("xray config validation failed: %w", err)
	}
	a.logger.Info("Xray 配置验证通过")

	cmd := exec.Command(xrayBinaryPath, "run", "-c", a.config.Xray.ConfigPath)
	cmd.Stdout = &logWriter{logger: a.logger, prefix: "[Xray-stdout]"}
	cmd.Stderr = &logWriter{logger: a.logger, prefix: "[Xray-stderr]"}

	if err := cmd.Start(); err != nil {
		a.logger.Error("启动 Xray 进程失败", logger.F("error", err.Error()))
		return fmt.Errorf("failed to start xray: %w", err)
	}

	a.logger.Info("Xray 进程已启动", logger.F("pid", cmd.Process.Pid))

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-a.ctx.Done():
			if cmd.Process != nil {
				a.logger.Info("Agent 停止，终止 Xray 进程")
				_ = cmd.Process.Kill()
			}
			return
		case err := <-done:
			if err != nil {
				a.logger.Error("Xray 进程异常退出", logger.F("error", err.Error()))
			} else {
				a.logger.Warn("Xray 进程正常退出")
			}
			a.triggerXraySelfHeal("xray process exited")
		}
	}()

	time.Sleep(3 * time.Second)

	if a.isXrayRunning() {
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
