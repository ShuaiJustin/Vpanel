// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"v/internal/logger"
)

// CommandType defines the type of command.
type CommandType string

const (
	// Xray commands
	CommandXrayStart   CommandType = "xray_start"
	CommandXrayStop    CommandType = "xray_stop"
	CommandXrayRestart CommandType = "xray_restart"
	CommandXrayStatus  CommandType = "xray_status"

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
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to start Xray: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Xray started successfully"
	result.Data = e.agent.GetXrayStatus()
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

	err := e.agent.xrayManager.Restart(ctx)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to restart Xray: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Xray restarted successfully"
	result.Data = e.agent.xrayManager.GetStatus()
	return result
}

// executeXrayStatus returns the Xray status.
func (e *CommandExecutor) executeXrayStatus(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	status := e.agent.GetXrayStatus()
	result.Success = true
	result.Message = "Status retrieved"
	result.Data = status
	return result
}

// executeConfigSync syncs configuration from the Panel.
func (e *CommandExecutor) executeConfigSync(ctx context.Context, cmd *Command) *CommandResult {
	result := &CommandResult{CommandID: cmd.ID}

	// Parse config from payload
	var config json.RawMessage
	if cmd.Payload != nil {
		if err := json.Unmarshal(cmd.Payload, &config); err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("invalid config payload: %v", err)
			return result
		}
	}

	// If no config in payload, fetch from Panel
	if config == nil {
		e.agent.mu.RLock()
		nodeID := e.agent.nodeID
		e.agent.mu.RUnlock()

		fetchedConfig, err := e.agent.panelClient.SyncConfig(ctx, nodeID)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("failed to fetch config from Panel: %v", err)
			return result
		}
		config = fetchedConfig
	}

	// Update Xray configuration
	err := e.agent.xrayManager.UpdateConfig(ctx, config)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("failed to update config: %v", err)
		return result
	}

	result.Success = true
	result.Message = "Configuration synced successfully"
	return result
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
