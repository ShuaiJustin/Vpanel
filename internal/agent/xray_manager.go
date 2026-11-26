// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
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
	configMu  sync.Mutex
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
	return m.applyConfig(ctx, config, func(ctx context.Context, changed bool) error {
		if changed && m.GetStatus().Running {
			if err := m.Restart(ctx); err != nil {
				return err
			}
			return waitForXrayStartup(ctx, func() *XrayStatus { return m.GetStatus() })
		}
		return nil
	})
}

func (m *XrayManager) applyConfig(ctx context.Context, config json.RawMessage, activate func(context.Context, bool) error) error {
	m.configMu.Lock()
	defer m.configMu.Unlock()
	path, err := filepath.EvalSymlinks(m.config.ConfigPath)
	if os.IsNotExist(err) {
		path = m.config.ConfigPath
	} else if err != nil {
		return err
	}
	old, err := os.ReadFile(path)
	hadConfig := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if hadConfig && bytes.Equal(bytes.TrimSpace(old), bytes.TrimSpace(config)) {
		return activate(ctx, false)
	}
	if err := m.ValidateConfig(ctx, config); err != nil {
		return err
	}
	if hadConfig {
		if _, err := m.BackupConfig(ctx); err != nil {
			return fmt.Errorf("back up Xray config: %w", err)
		}
	}
	if err := writeXrayConfig(path, config); err != nil {
		return err
	}
	if err := activate(ctx, true); err != nil {
		if !hadConfig {
			return errors.Join(err, os.Remove(path))
		}
		if restoreErr := writeXrayConfig(path, old); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore Xray config: %w", restoreErr))
		}
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if rollbackErr := activate(rollbackCtx, true); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("activate restored Xray config: %w", rollbackErr))
		}
		return fmt.Errorf("%w; previous Xray configuration restored", err)
	}
	return nil
}

func writeXrayConfig(path string, data []byte) error {
	stage, err := os.CreateTemp(filepath.Dir(path), ".xray-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	// Xray managed by systemd may run as a different user. Preserve the
	// existing read permissions when replacing its config; a first config
	// gets the same mode the installer uses (CreateTemp alone gives 0600).
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := stage.Chmod(mode); err != nil {
		_ = stage.Close()
		return err
	}
	_, writeErr := stage.Write(data)
	if err := errors.Join(writeErr, stage.Close()); err != nil {
		return err
	}
	return os.Rename(stage.Name(), path)
}

func waitForXrayStartup(ctx context.Context, status func() *XrayStatus) error {
	first := status()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		current := status()
		if first.PID == 0 || !current.Running || current.PID != first.PID {
			return fmt.Errorf("Xray did not remain running after startup")
		}
		return nil
	}
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

func (m *XrayManager) setVersion(version string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.version = version
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
	// Signal 0 probes the process without delivering anything. A nil os.Signal
	// is rejected as "unsupported signal type", which reported every managed
	// process as dead.
	return m.cmd != nil && m.cmd.Process != nil && m.cmd.Process.Signal(syscall.Signal(0)) == nil
}

func outputWriterOrDiscard(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
