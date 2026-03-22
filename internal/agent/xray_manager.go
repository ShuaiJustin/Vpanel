// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"v/internal/logger"
)

// XrayManagerConfig holds configuration for the Xray manager.
type XrayManagerConfig struct {
	BinaryPath string
	ConfigPath string
	BackupDir  string
	Stdout     io.Writer
	Stderr     io.Writer
}

// XrayStatus represents the current status of Xray.
type XrayStatus struct {
	Running   bool      `json:"running"`
	PID       int       `json:"pid,omitempty"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

// XrayManager manages the local Xray process.
type XrayManager struct {
	mu        sync.RWMutex
	config    XrayManagerConfig
	logger    logger.Logger
	cmd       *exec.Cmd
	waitDone  chan error
	startedAt time.Time
	version   string
}

// NewXrayManager creates a new Xray manager.
func NewXrayManager(cfg XrayManagerConfig, log logger.Logger) *XrayManager {
	if cfg.BackupDir == "" {
		cfg.BackupDir = "/tmp/xray-backups"
	}

	m := &XrayManager{
		config: cfg,
		logger: log,
	}

	// Get version
	m.version = m.getVersionString()

	return m
}

// Start starts the Xray process.
func (m *XrayManager) Start(ctx context.Context) error {
	// Validate config before starting
	if err := m.validateConfig(ctx); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isManagedProcessRunningLocked() {
		return fmt.Errorf("xray is already running")
	}

	// Start Xray process
	cmd := exec.Command(m.config.BinaryPath, "run", "-c", m.config.ConfigPath)
	cmd.Stdout = outputWriterOrDiscard(m.config.Stdout)
	cmd.Stderr = outputWriterOrDiscard(m.config.Stderr)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start xray: %w", err)
	}

	done := make(chan error, 1)
	m.cmd = cmd
	m.waitDone = done
	m.startedAt = time.Now()
	m.logger.Info("xray started", logger.F("pid", cmd.Process.Pid))

	// Start process monitor
	go m.monitorProcess(cmd, done)

	return nil
}

// Stop stops the Xray process.
func (m *XrayManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	cmd := m.cmd
	done := m.waitDone
	if cmd == nil || cmd.Process == nil {
		m.mu.Unlock()
		return nil // Already stopped
	}
	m.mu.Unlock()

	// Send SIGTERM first
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		m.logger.Warn("failed to send interrupt signal", logger.F("error", err))
	}

	select {
	case err := <-done:
		if err != nil {
			m.logger.Debug("xray process exited", logger.F("error", err))
		}
	case <-time.After(5 * time.Second):
		// Force kill if not stopped
		if err := cmd.Process.Kill(); err != nil {
			m.logger.Error("failed to kill xray process", logger.F("error", err))
		}
		select {
		case err := <-done:
			if err != nil {
				m.logger.Debug("xray process exited after kill", logger.F("error", err))
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	m.mu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
		m.waitDone = nil
	}
	m.mu.Unlock()

	m.logger.Info("xray stopped")
	return nil
}

// Restart restarts the Xray process.
func (m *XrayManager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return m.Start(ctx)
}

// GetStatus returns the current Xray status.
func (m *XrayManager) GetStatus() *XrayStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &XrayStatus{
		Running: false,
		Version: m.version,
	}

	if m.isManagedProcessRunningLocked() {
		status.Running = true
		status.PID = m.cmd.Process.Pid
		status.StartedAt = m.startedAt
		status.Uptime = time.Since(m.startedAt).Round(time.Second).String()
	}

	return status
}

// GetConfig returns the current Xray configuration.
func (m *XrayManager) GetConfig(ctx context.Context) (json.RawMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	return json.RawMessage(data), nil
}

// UpdateConfig updates the Xray configuration.
func (m *XrayManager) UpdateConfig(ctx context.Context, config json.RawMessage) error {
	// Validate new config first
	if err := m.ValidateConfig(ctx, config); err != nil {
		return err
	}

	// Backup current config
	if _, err := m.BackupConfig(ctx); err != nil {
		m.logger.Warn("failed to backup config", logger.F("error", err))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Write new config
	if err := os.WriteFile(m.config.ConfigPath, config, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	m.logger.Info("xray config updated")

	// Restart Xray if running
	if m.cmd != nil && m.cmd.Process != nil {
		m.mu.Unlock()
		err := m.Restart(ctx)
		m.mu.Lock()
		if err != nil {
			return fmt.Errorf("failed to restart xray after config update: %w", err)
		}
	}

	return nil
}

// ValidateConfig validates an Xray configuration.
func (m *XrayManager) ValidateConfig(ctx context.Context, config json.RawMessage) error {
	// Write config to temp file
	tmpFile, err := os.CreateTemp("", "xray-config-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(config); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp config: %w", err)
	}
	tmpFile.Close()

	// Run xray with test flag
	cmd := exec.CommandContext(ctx, m.config.BinaryPath, "run", "-test", "-c", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("invalid xray config: %s", string(output))
	}

	return nil
}

// BackupConfig creates a backup of the current configuration.
func (m *XrayManager) BackupConfig(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Ensure backup directory exists
	if err := os.MkdirAll(m.config.BackupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Read current config
	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No config to backup
		}
		return "", fmt.Errorf("failed to read config: %w", err)
	}

	// Create backup file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(m.config.BackupDir, fmt.Sprintf("xray-config-%s.json", timestamp))

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write backup: %w", err)
	}

	m.logger.Info("config backed up", logger.F("path", backupPath))
	return backupPath, nil
}

// RestoreConfig restores configuration from a backup.
func (m *XrayManager) RestoreConfig(ctx context.Context, backupPath string) error {
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Validate backup config
	if err := m.ValidateConfig(ctx, data); err != nil {
		return fmt.Errorf("backup config validation failed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Write restored config
	if err := os.WriteFile(m.config.ConfigPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}

	m.logger.Info("config restored", logger.F("from", backupPath))
	return nil
}

// validateConfig validates the current config file.
func (m *XrayManager) validateConfig(ctx context.Context) error {
	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		return err
	}
	return m.ValidateConfig(ctx, data)
}

// getVersionString returns the Xray version string.
func (m *XrayManager) getVersionString() string {
	cmd := exec.Command(m.config.BinaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return string(output)
}

// monitorProcess monitors the Xray process.
func (m *XrayManager) monitorProcess(cmd *exec.Cmd, done chan error) {
	// Wait for process to exit
	err := cmd.Wait()
	select {
	case done <- err:
	default:
	}
	if err != nil {
		m.logger.Warn("xray process exited", logger.F("error", err))
	}

	m.mu.Lock()
	if m.cmd == cmd {
		m.cmd = nil
		m.waitDone = nil
	}
	m.mu.Unlock()
}

func (m *XrayManager) isManagedProcessRunningLocked() bool {
	return m.cmd != nil && m.cmd.Process != nil && m.cmd.Process.Signal(os.Signal(nil)) == nil
}

func outputWriterOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
