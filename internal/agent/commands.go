// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"v/internal/logger"
)

// CommandType defines the type of command.
type CommandType string

const (
	// Xray commands
	CommandXrayStart          CommandType = "xray_start"
	CommandXrayStop           CommandType = "xray_stop"
	CommandXrayRestart        CommandType = "xray_restart"
	CommandXrayStatus         CommandType = "xray_status"
	CommandXrayInstallVersion CommandType = "xray_install_version"

	// Configuration commands
	CommandConfigSync   CommandType = "config_sync"
	CommandConfigGet    CommandType = "config_get"
	CommandConfigBackup CommandType = "config_backup"

	// System commands
	CommandSystemInfo    CommandType = "system_info"
	CommandSystemMetrics CommandType = "system_metrics"
	CommandSystemReboot  CommandType = "system_reboot"

	// Agent commands
	CommandAgentUpdate  CommandType = "agent_update"
	CommandAgentRestart CommandType = "agent_restart"
)

// CommandExecutor handles command execution.
type CommandExecutor struct {
	mu          sync.RWMutex
	agent       *Agent
	logger      logger.Logger
	pendingCmds map[string]*Command
	resultChan  chan *CommandResult
}

// NewCommandExecutor creates a new command executor.
func NewCommandExecutor(agent *Agent, log logger.Logger) *CommandExecutor {
	return &CommandExecutor{
		agent:       agent,
		logger:      log,
		pendingCmds: make(map[string]*Command),
		resultChan:  make(chan *CommandResult, 100),
	}
}

// Execute executes a command and returns the result.
func (e *CommandExecutor) Execute(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{
		CommandID: cmd.ID,
	}

	e.logger.Info("executing command",
		logger.F("command_id", cmd.ID),
		logger.F("type", cmd.Type))

	startTime := time.Now()

	switch CommandType(cmd.Type) {
	// Xray commands
	case CommandXrayStart:
		result = e.executeXrayStart(ctx, cmd)
	case CommandXrayStop:
		result = e.executeXrayStop(ctx, cmd)
	case CommandXrayRestart:
		result = e.executeXrayRestart(ctx, cmd)
	case CommandXrayStatus:
		result = e.executeXrayStatus(ctx, cmd)
	case CommandXrayInstallVersion:
		result = e.executeXrayInstallVersion(ctx, cmd)

	// Configuration commands
	case CommandConfigSync:
		result = e.executeConfigSync(ctx, cmd)
	case CommandConfigGet:
		result = e.executeConfigGet(ctx, cmd)
	case CommandConfigBackup:
		result = e.executeConfigBackup(ctx, cmd)

	// System commands
	case CommandSystemInfo:
		result = e.executeSystemInfo(ctx, cmd)
	case CommandSystemMetrics:
		result = e.executeSystemMetrics(ctx, cmd)

	// Agent commands
	case CommandAgentUpdate:
		result = e.executeAgentUpdate(ctx, cmd)
	case CommandAgentRestart:
		result = e.executeAgentRestart(ctx, cmd)

	default:
		result.Success = false
		result.Message = fmt.Sprintf("unknown command type: %s", cmd.Type)
	}

	duration := time.Since(startTime)
	e.logger.Info("command executed",
		logger.F("command_id", cmd.ID),
		logger.F("type", cmd.Type),
		logger.F("success", result.Success),
		logger.F("duration_ms", duration.Milliseconds()))

	return result
}

// executeXrayStart starts the Xray process.
func (e *CommandExecutor) executeXrayStart(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	err := e.agent.ensureXrayRunning(ctx)
	if err == nil {
		err = waitForXrayStartup(ctx, e.agent.currentXrayStatus)
	}
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to start Xray: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Xray started successfully"
	result.Data = e.agent.currentXrayStatus()
	return result
}

// executeXrayStop stops the Xray process.
func (e *CommandExecutor) executeXrayStop(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	err := e.agent.xrayManager.Stop(ctx)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to stop Xray: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Xray stopped successfully"
	return result
}

// executeXrayRestart restarts the Xray process.
func (e *CommandExecutor) executeXrayRestart(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	err := e.agent.restartXray(ctx)
	if err == nil {
		err = waitForXrayStartup(ctx, e.agent.currentXrayStatus)
	}
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to restart Xray: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Xray restarted successfully"
	result.Data = e.agent.currentXrayStatus()
	return result
}

// executeXrayStatus returns the Xray status.
func (e *CommandExecutor) executeXrayStatus(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	status := e.agent.currentXrayStatus()
	result.Success = true
	result.Message = "Status retrieved"
	result.Data = status
	return result
}

// executeXrayInstallVersion downloads and installs a specific Xray version then restarts.
// Payload: {"version": "1.8.4"} — empty means latest release.
func (e *CommandExecutor) executeXrayInstallVersion(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	var payload struct {
		Version string `json:"version"`
	}
	if len(bytes.TrimSpace(cmd.Payload)) > 0 && string(bytes.TrimSpace(cmd.Payload)) != "null" {
		if err := json.Unmarshal(cmd.Payload, &payload); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("invalid payload: %v", err)
			return result
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()
	a := e.agent
	a.mu.Lock()
	if a.xrayHealInProgress || a.xraySwitchInProgress {
		a.mu.Unlock()
		result.Message = "Xray 正在恢复或切换版本，请稍后重试"
		return result
	}
	a.xraySwitchInProgress = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.xraySwitchInProgress = false
		a.mu.Unlock()
	}()
	installer := NewXrayInstaller(e.logger)
	version, err := installer.SwitchXrayVersion(ctx, payload.Version, a.resolveXrayBinaryPath(), a.config.Xray.ConfigPath, a.restartXrayForVersionSwitch)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to install xray version %q: %v", payload.Version, err)
		return result
	}

	a.xrayManager.setVersion(version)
	result.Success = true
	result.Message = fmt.Sprintf("Xray 版本已切换至 %s", strings.Fields(version)[1])
	result.Data = a.currentXrayStatus()
	return result
}

// executeConfigSync syncs configuration from the Panel.
func (e *CommandExecutor) executeConfigSync(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	var config json.RawMessage
	if len(bytes.TrimSpace(cmd.Payload)) > 0 && string(bytes.TrimSpace(cmd.Payload)) != "null" {
		if err := json.Unmarshal(cmd.Payload, &config); err != nil {
			e.logger.Warn("config_sync payload parse failed, falling back to panel sync",
				logger.F("command_id", cmd.ID),
				logger.F("error", err.Error()))
			config = nil
		}
	}

	if !isLikelyXrayConfig(config) {
		if len(bytes.TrimSpace(cmd.Payload)) > 0 && string(bytes.TrimSpace(cmd.Payload)) != "null" {
			e.logger.Warn("config_sync payload is not a valid xray config, falling back to panel sync",
				logger.F("command_id", cmd.ID))
		}

		e.agent.mu.RLock()
		nodeID := e.agent.nodeID
		e.agent.mu.RUnlock()

		fetchedConfig, err := e.agent.panelClient.SyncConfig(ctx, nodeID)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("failed to fetch config from Panel: %v", err)
			return result
		}
		if !isLikelyXrayConfig(fetchedConfig) {
			result.Success = false
			result.Message = "panel returned invalid xray config payload"
			return result
		}
		config = fetchedConfig
	}

	err := e.agent.xrayManager.applyConfig(ctx, config, func(ctx context.Context, changed bool) error {
		if changed && e.agent.currentXrayStatus().Running {
			if err := e.agent.restartXray(ctx); err != nil {
				return err
			}
		} else if !e.agent.currentXrayStatus().Running {
			if err := e.agent.ensureXrayRunning(ctx); err != nil {
				return err
			}
		} else {
			return nil
		}
		return waitForXrayStartup(ctx, e.agent.currentXrayStatus)
	})
	if err != nil {
		result.Message = fmt.Sprintf("failed to apply Xray config: %v", err)
		return result
	}
	applied, err := e.agent.xrayManager.GetConfig(ctx)
	if err != nil || configVersion(applied) != configVersion(config) {
		result.Message = "Xray 配置应用后校验失败"
		return result
	}

	result.Success = true
	result.Message = "Configuration synced successfully"
	result.Data = map[string]string{"config_version": configVersion(applied)}
	return result
}

func shouldRestartExternalXrayAfterConfigSync(managedRunningBefore, observedRunningBefore bool) bool {
	return observedRunningBefore && !managedRunningBefore
}

func isLikelyXrayConfig(config json.RawMessage) bool {
	trimmed := bytes.TrimSpace(config)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return false
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &cfg); err != nil {
		return false
	}

	_, hasInbounds := cfg["inbounds"]
	_, hasOutbounds := cfg["outbounds"]
	return hasInbounds && hasOutbounds
}

// executeConfigGet returns the current configuration.
func (e *CommandExecutor) executeConfigGet(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	config, err := e.agent.xrayManager.GetConfig(ctx)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to get config: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Configuration retrieved"
	result.Data = config
	return result
}

// executeConfigBackup creates a configuration backup.
func (e *CommandExecutor) executeConfigBackup(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	backupPath, err := e.agent.xrayManager.BackupConfig(ctx)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to backup config: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Configuration backed up"
	result.Data = map[string]string{"backup_path": backupPath}
	return result
}

// executeSystemInfo returns system information.
func (e *CommandExecutor) executeSystemInfo(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	metrics := e.agent.collectMetrics()
	result.Success = true
	result.Message = "System info retrieved"
	result.Data = metrics
	return result
}

// executeSystemMetrics returns current system metrics.
func (e *CommandExecutor) executeSystemMetrics(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	metrics := e.agent.collectMetrics()
	result.Success = true
	result.Message = "Metrics retrieved"
	result.Data = metrics
	return result
}

// executeAgentUpdate downloads and installs a new agent binary.
func (e *CommandExecutor) executeAgentUpdate(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}
	var target struct {
		Version string `json:"version"`
	}
	if len(cmd.Payload) != 0 {
		if err := json.Unmarshal(cmd.Payload, &target); err != nil {
			result.Message = "Agent 更新目标无效"
			return result
		}
	}

	panelURL := e.agent.config.Panel.URL
	if panelURL == "" {
		result.Success = false
		result.Message = "panel URL not configured"
		return result
	}

	exePath, err := os.Executable()
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to get executable path: %v", err)
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	downloadURL := fmt.Sprintf("%s/api/admin/nodes/agent/download?arch=%s", strings.TrimRight(panelURL, "/"), runtime.GOARCH)

	e.logger.Info("downloading agent update", logger.F("url", downloadURL))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		result.Message = fmt.Sprintf("prepare download: %v", err)
		return result
	}
	if request.URL.Scheme != "https" {
		result.Message = "Agent 在线更新需要使用 HTTPS 面板地址"
		return result
	}
	client := *e.agent.httpClient
	client.Timeout = 2 * time.Minute
	checkRedirect := client.CheckRedirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.Host != request.URL.Host || len(via) >= 10 {
			return fmt.Errorf("Agent update redirect left the configured HTTPS origin")
		}
		if checkRedirect != nil {
			return checkRedirect(req, via)
		}
		return nil
	}
	resp, err := client.Do(request)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("download failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		result.Success = false
		result.Message = fmt.Sprintf("download failed: HTTP %d", resp.StatusCode)
		return result
	}

	if err := replaceAgentBinary(ctx, exePath, target.Version, resp.Header.Get("X-Agent-SHA256"), resp.Body); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Message = "下载 Agent 超时，请检查节点到面板的下载连接后重试"
			return result
		}
		result.Message = err.Error()
		return result
	}
	result.Success = true
	result.Message = "Agent 文件已替换，等待新进程心跳"
	return result
}

func replaceAgentBinary(ctx context.Context, exePath, targetVersion, expectedSHA256 string, source io.Reader) error {
	expected, err := hex.DecodeString(strings.TrimSpace(expectedSHA256))
	if err != nil || len(expected) != sha256.Size {
		return fmt.Errorf("Agent 下载缺少有效的 SHA-256 校验值")
	}
	tmpFile, err := os.CreateTemp(filepath.Dir(exePath), ".vpanel-agent-update-*")
	if err != nil {
		return binaryWriteError("Agent", exePath, err)
	}
	defer os.Remove(tmpFile.Name())
	digest := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmpFile, digest), source)
	closeErr := tmpFile.Close()
	if err != nil {
		return fmt.Errorf("write Agent binary: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close staged Agent binary: %w", closeErr)
	}
	if !bytes.Equal(expected, digest.Sum(nil)) {
		return fmt.Errorf("Agent 下载文件 SHA-256 校验失败")
	}
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("set Agent binary permissions: %w", err)
	}
	output, err := exec.CommandContext(ctx, tmpFile.Name(), "-version").Output()
	if err != nil || !strings.HasPrefix(string(output), "V Panel Node Agent ") {
		return fmt.Errorf("downloaded file is not a runnable V Panel Agent")
	}
	firstLine, _, _ := strings.Cut(string(output), "\n")
	downloadedVersion := strings.TrimSpace(strings.TrimPrefix(firstLine, "V Panel Node Agent "))
	if targetVersion != "" && downloadedVersion != targetVersion {
		return fmt.Errorf("downloaded Agent version %q does not match %q", downloadedVersion, targetVersion)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(tmpFile.Name(), exePath); err != nil {
		return binaryWriteError("Agent", exePath, err)
	}
	return nil
}

func binaryWriteError(name, path string, err error) error {
	if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EROFS) {
		return fmt.Errorf("%s 更新无法写入 %s，请检查文件及目录的不可变属性、挂载权限和 systemd 写入限制: %w", name, path, err)
	}
	return fmt.Errorf("write %s binary %s: %w", name, path, err)
}

// executeAgentRestart restarts the agent.
func (e *CommandExecutor) executeAgentRestart(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	// Schedule restart after returning result
	go func() {
		time.Sleep(2 * time.Second)
		e.logger.Info("agent restart requested, exiting...")
		// In production, this would trigger a process restart via systemd or similar
	}()

	result.Success = true
	result.Message = "Agent restart scheduled"
	return result
}

// AddPendingCommand adds a command to the pending queue.
func (e *CommandExecutor) AddPendingCommand(cmd *Command) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.pendingCmds[cmd.ID] = cmd
}

// GetPendingCommand retrieves a pending command by ID.
func (e *CommandExecutor) GetPendingCommand(id string) *Command {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pendingCmds[id]
}

// RemovePendingCommand removes a command from the pending queue.
func (e *CommandExecutor) RemovePendingCommand(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.pendingCmds, id)
}

// GetPendingCount returns the number of pending commands.
func (e *CommandExecutor) GetPendingCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.pendingCmds)
}
