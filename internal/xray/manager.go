// Package xray provides Xray-core process management.
package xray

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
	"v/pkg/errors"
)

// Status represents Xray process status.
type Status struct {
	Running     bool      `json:"running"`
	PID         int       `json:"pid,omitempty"`
	Uptime      string    `json:"uptime,omitempty"`
	Version     string    `json:"version"`
	Connections int       `json:"connections"`
	StartedAt   time.Time `json:"started_at,omitempty"`
}

// Version represents Xray version information.
type Version struct {
	Current   string `json:"current"`
	Latest    string `json:"latest,omitempty"`
	CanUpdate bool   `json:"can_update"`
}

// Manager manages Xray-core process.
type Manager interface {
	// Process management
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	GetStatus(ctx context.Context) (*Status, error)

	// Configuration management
	GetConfig(ctx context.Context) (json.RawMessage, error)
	UpdateConfig(ctx context.Context, config json.RawMessage) error
	ValidateConfig(ctx context.Context, config json.RawMessage) error
	ReloadConfig(ctx context.Context) error

	// Version management
	GetVersion(ctx context.Context) (*Version, error)

	// Backup and restore
	BackupConfig(ctx context.Context) (string, error)
	RestoreConfig(ctx context.Context, backupPath string) error
}

// manager implements Manager.
type manager struct {
	mu           sync.RWMutex
	cmd          *exec.Cmd
	configPath   string
	binaryPath   string
	backupDir    string
	startedAt    time.Time
	logger       logger.Logger
	restartCount int
	maxRestarts  int
}

// Config holds Xray manager configuration.
type Config struct {
	BinaryPath  string
	ConfigPath  string
	BackupDir   string
	MaxRestarts int
}

// NewManager creates a new Xray manager.
func NewManager(cfg Config, log logger.Logger) Manager {
	if cfg.MaxRestarts <= 0 {
		cfg.MaxRestarts = 3
	}
	if cfg.BackupDir == "" {
		cfg.BackupDir = "/tmp/xray-backups"
	}

	return &manager{
		configPath:  cfg.ConfigPath,
		binaryPath:  cfg.BinaryPath,
		backupDir:   cfg.BackupDir,
		maxRestarts: cfg.MaxRestarts,
		logger:      log,
	}
}

// Start starts the Xray process.
func (m *manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		// Check if process is still running
		if err := m.cmd.Process.Signal(os.Signal(nil)); err == nil {
			return errors.NewConflictError("xray", "status", "already running")
		}
	}

	// Validate config before starting
	if err := m.validateConfigInternal(ctx); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Start Xray process
	m.cmd = exec.CommandContext(ctx, m.binaryPath, "run", "-c", m.configPath)
	m.cmd.Stdout = io.Discard
	m.cmd.Stderr = io.Discard

	if err := m.cmd.Start(); err != nil {
		return errors.NewInternalError("failed to start xray", err)
	}

	m.startedAt = time.Now()
	m.restartCount = 0
	m.logger.Info("xray started", logger.F("pid", m.cmd.Process.Pid))

	// Start process monitor
	go m.monitorProcess(ctx)

	return nil
}

// Stop stops the Xray process.
func (m *manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd == nil || m.cmd.Process == nil {
		return nil // Already stopped
	}

	// Send SIGTERM first
	if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
		m.logger.Warn("failed to send interrupt signal", logger.F("error", err))
	}

	// Wait for graceful shutdown
	done := make(chan error, 1)
	go func() {
		done <- m.cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		// Force kill if not stopped
		if err := m.cmd.Process.Kill(); err != nil {
			m.logger.Error("failed to kill xray process", logger.F("error", err))
		}
	case err := <-done:
		if err != nil {
			m.logger.Debug("xray process exited", logger.F("error", err))
		}
	}

	m.cmd = nil
	m.logger.Info("xray stopped")
	return nil
}

// Restart restarts the Xray process.
func (m *manager) Restart(ctx context.Context) error {
	if err := m.Stop(ctx); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond) // Brief pause
	return m.Start(ctx)
}

// GetStatus returns the current Xray status.
func (m *manager) GetStatus(ctx context.Context) (*Status, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := &Status{
		Running: false,
		Version: m.getVersionString(),
	}

	if m.cmd != nil && m.cmd.Process != nil {
		// Check if process is still running
		if err := m.cmd.Process.Signal(os.Signal(nil)); err == nil {
			status.Running = true
			status.PID = m.cmd.Process.Pid
			status.StartedAt = m.startedAt
			status.Uptime = time.Since(m.startedAt).Round(time.Second).String()
		}
	}

	return status, nil
}

// GetConfig returns the current Xray configuration.
func (m *manager) GetConfig(ctx context.Context) (json.RawMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.NewNotFoundError("xray config", m.configPath)
		}
		return nil, errors.NewInternalError("failed to read config", err)
	}

	return json.RawMessage(data), nil
}

// UpdateConfig updates the Xray configuration.
func (m *manager) UpdateConfig(ctx context.Context, config json.RawMessage) error {
	// Validate new config first
	if err := m.ValidateConfig(ctx, config); err != nil {
		return err
	}

	// Backup current config
	backupPath, err := m.BackupConfig(ctx)
	if err != nil {
		m.logger.Warn("failed to backup config", logger.F("error", err))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Write new config
	if err := os.WriteFile(m.configPath, config, 0644); err != nil {
		return errors.NewInternalError("failed to write config", err)
	}

	m.logger.Info("xray config updated", logger.F("backup", backupPath))
	return nil
}

// ValidateConfig validates an Xray configuration.
func (m *manager) ValidateConfig(ctx context.Context, config json.RawMessage) error {
	// Write config to temp file
	tmpFile, err := os.CreateTemp("", "xray-config-*.json")
	if err != nil {
		return errors.NewInternalError("failed to create temp file", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(config); err != nil {
		tmpFile.Close()
		return errors.NewInternalError("failed to write temp config", err)
	}
	tmpFile.Close()

	// Run xray with test flag
	cmd := exec.CommandContext(ctx, m.binaryPath, "run", "-test", "-c", tmpFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.NewValidationError("invalid xray config: "+string(output), err)
	}

	return nil
}

// ReloadConfig reloads the Xray configuration.
func (m *manager) ReloadConfig(ctx context.Context) error {
	m.mu.RLock()
	running := m.cmd != nil && m.cmd.Process != nil
	m.mu.RUnlock()

	if !running {
		return nil // Not running, nothing to reload
	}

	// Validate config before reload
	if err := m.validateConfigInternal(ctx); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Restart to apply new config
	return m.Restart(ctx)
}

// GetVersion returns Xray version information.
func (m *manager) GetVersion(ctx context.Context) (*Version, error) {
	version := &Version{
		Current: m.getVersionString(),
	}

	// TODO: Check for latest version from GitHub releases
	// For now, just return current version
	version.CanUpdate = false

	return version, nil
}

// GetBinaryPath returns the current Xray binary path.
func (m *manager) GetBinaryPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.binaryPath
}

// SetBinaryPath updates the Xray binary path used for future starts.
func (m *manager) SetBinaryPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if path != "" {
		m.binaryPath = path
	}
}

// BackupConfig creates a backup of the current configuration.
func (m *manager) BackupConfig(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Ensure backup directory exists
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return "", errors.NewInternalError("failed to create backup directory", err)
	}

	// Read current config
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // No config to backup
		}
		return "", errors.NewInternalError("failed to read config", err)
	}

	// Create backup file with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(m.backupDir, fmt.Sprintf("xray-config-%s.json", timestamp))

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return "", errors.NewInternalError("failed to write backup", err)
	}

	m.logger.Info("config backed up", logger.F("path", backupPath))
	return backupPath, nil
}

// RestoreConfig restores configuration from a backup.
func (m *manager) RestoreConfig(ctx context.Context, backupPath string) error {
	// Read backup file
	data, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.NewNotFoundError("backup file", backupPath)
		}
		return errors.NewInternalError("failed to read backup", err)
	}

	// Validate backup config
	if err := m.ValidateConfig(ctx, data); err != nil {
		return fmt.Errorf("backup config validation failed: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Write restored config
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return errors.NewInternalError("failed to restore config", err)
	}

	m.logger.Info("config restored", logger.F("from", backupPath))
	return nil
}

// monitorProcess monitors the Xray process and restarts if needed.
func (m *manager) monitorProcess(ctx context.Context) {
	for {
		m.mu.RLock()
		cmd := m.cmd
		m.mu.RUnlock()

		if cmd == nil {
			return // Process was stopped intentionally
		}

		// Wait for process to exit
		err := cmd.Wait()
		if err == nil {
			return // Clean exit
		}

		m.mu.Lock()
		if m.cmd != cmd {
			m.mu.Unlock()
			return // Process was replaced
		}

		m.restartCount++
		if m.restartCount > m.maxRestarts {
			m.logger.Error("xray crashed too many times, giving up",
				logger.F("restarts", m.restartCount))
			m.cmd = nil
			m.mu.Unlock()
			return
		}

		m.logger.Warn("xray crashed, restarting",
			logger.F("error", err),
			logger.F("restart_count", m.restartCount))

		// Exponential backoff
		backoff := time.Duration(m.restartCount) * time.Second
		m.mu.Unlock()

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}

		// Restart
		if err := m.Start(ctx); err != nil {
			m.logger.Error("failed to restart xray", logger.F("error", err))
		}
	}
}

// validateConfigInternal validates the current config file.
func (m *manager) validateConfigInternal(ctx context.Context) error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}
	return m.ValidateConfig(ctx, data)
}

// getVersionString returns the Xray version string.
func (m *manager) getVersionString() string {
	cmd := exec.Command(m.binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	// Parse version from output (first line usually contains version)
	lines := string(output)
	if len(lines) > 0 {
		// Extract version number
		return lines[:min(50, len(lines))]
	}
	return "unknown"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
