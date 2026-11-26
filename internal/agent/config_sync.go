// Package agent provides the Node Agent functionality for V Panel.
package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"v/internal/logger"
)

// ConfigSyncConfig holds configuration for config sync behavior.
type ConfigSyncConfig struct {
	// SyncInterval is how often to check for config updates
	SyncInterval time.Duration
	// RetryInterval is how long to wait before retrying a failed sync
	RetryInterval time.Duration
	// MaxRetries is the maximum number of sync retries
	MaxRetries int
	// ValidateBeforeApply validates config before applying
	ValidateBeforeApply bool
	// BackupBeforeApply creates a backup before applying new config
	BackupBeforeApply bool
}

// DefaultConfigSyncConfig returns default config sync configuration.
func DefaultConfigSyncConfig() *ConfigSyncConfig {
	return &ConfigSyncConfig{
		SyncInterval:        5 * time.Minute,
		RetryInterval:       30 * time.Second,
		MaxRetries:          3,
		ValidateBeforeApply: true,
		BackupBeforeApply:   true,
	}
}

// ConfigSyncManager manages configuration synchronization with the Panel.
type ConfigSyncManager struct {
	mu           sync.RWMutex
	config       *ConfigSyncConfig
	agent        *Agent
	logger       logger.Logger
	lastSyncTime time.Time
	lastSyncErr  error
	syncVersion  string
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewConfigSyncManager creates a new config sync manager.
func NewConfigSyncManager(cfg *ConfigSyncConfig, agent *Agent, log logger.Logger) *ConfigSyncManager {
	if cfg == nil {
		cfg = DefaultConfigSyncConfig()
	}
	return &ConfigSyncManager{
		config: cfg,
		agent:  agent,
		logger: log,
	}
}

// Start starts the config sync manager.
func (m *ConfigSyncManager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("config sync manager is already running")
	}
	m.running = true
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.mu.Unlock()

	// Perform initial sync
	if err := m.Sync(ctx); err != nil {
		m.logger.Warn("initial config sync failed",
			logger.F("error", err.Error()))
	}

	// Start periodic sync loop
	m.wg.Add(1)
	go m.syncLoop()

	m.logger.Info("config sync manager started",
		logger.F("sync_interval", m.config.SyncInterval.String()))

	return nil
}

// Stop stops the config sync manager.
func (m *ConfigSyncManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	m.cancel()
	m.running = false
	m.mu.Unlock()

	// Wait for goroutine to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("config sync manager stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// syncLoop runs the periodic sync loop.
func (m *ConfigSyncManager) syncLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			if err := m.Sync(m.ctx); err != nil {
				m.logger.Warn("periodic config sync failed",
					logger.F("error", err.Error()))
			}
		}
	}
}

// Sync synchronizes configuration from the Panel.
func (m *ConfigSyncManager) Sync(ctx context.Context) error {
	m.agent.mu.RLock()
	if !m.agent.registered {
		m.agent.mu.RUnlock()
		return fmt.Errorf("agent not registered")
	}
	nodeID := m.agent.nodeID
	m.agent.mu.RUnlock()

	m.logger.Debug("starting config sync",
		logger.F("node_id", nodeID))

	// Fetch config from Panel
	configData, err := m.agent.panelClient.SyncConfig(ctx, nodeID)
	if err != nil {
		m.mu.Lock()
		m.lastSyncErr = err
		m.mu.Unlock()
		return fmt.Errorf("failed to fetch config: %w", err)
	}

	configVersion := configVersion(configData)
	if configVersion == "" {
		return fmt.Errorf("received empty config from panel")
	}

	// Check if config has changed
	m.mu.RLock()
	currentVersion := m.syncVersion
	m.mu.RUnlock()

	if configVersion == currentVersion {
		m.logger.Debug("config unchanged, skipping sync",
			logger.F("version", configVersion))
		return nil
	}

	// Apply the configuration
	if err := m.applyConfig(ctx, configData); err != nil {
		m.mu.Lock()
		m.lastSyncErr = err
		m.mu.Unlock()
		return fmt.Errorf("failed to apply config: %w", err)
	}

	// Update sync state
	m.mu.Lock()
	m.lastSyncTime = time.Now()
	m.lastSyncErr = nil
	m.syncVersion = configVersion
	m.mu.Unlock()

	m.logger.Info("config synced successfully",
		logger.F("version", configVersion),
		logger.F("node_id", nodeID))

	return nil
}

func configVersion(configData json.RawMessage) string {
	trimmed := bytes.TrimSpace(configData)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return ""
	}

	digest := sha256.Sum256(trimmed)
	return hex.EncodeToString(digest[:])
}

// applyConfig applies the configuration to Xray.
func (m *ConfigSyncManager) applyConfig(ctx context.Context, configData json.RawMessage) error {
	// Backup current config if enabled
	if m.config.BackupBeforeApply {
		if _, err := m.agent.xrayManager.BackupConfig(ctx); err != nil {
			m.logger.Warn("failed to backup config before sync",
				logger.F("error", err.Error()))
		}
	}

	// Validate config if enabled
	if m.config.ValidateBeforeApply {
		if err := m.agent.xrayManager.ValidateConfig(ctx, configData); err != nil {
			return fmt.Errorf("config validation failed: %w", err)
		}
	}

	// Apply the config
	if err := m.agent.xrayManager.UpdateConfig(ctx, configData); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	return nil
}

// SyncWithRetry syncs configuration with retry logic.
func (m *ConfigSyncManager) SyncWithRetry(ctx context.Context) error {
	var lastErr error

	for attempt := 0; attempt <= m.config.MaxRetries; attempt++ {
		if attempt > 0 {
			m.logger.Info("retrying config sync",
				logger.F("attempt", attempt),
				logger.F("max_retries", m.config.MaxRetries))

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.config.RetryInterval):
			}
		}

		err := m.Sync(ctx)
		if err == nil {
			return nil
		}

		lastErr = err
		m.logger.Warn("config sync attempt failed",
			logger.F("attempt", attempt+1),
			logger.F("error", err.Error()))
	}

	return fmt.Errorf("config sync failed after %d attempts: %w", m.config.MaxRetries+1, lastErr)
}

// GetLastSyncTime returns the last successful sync time.
func (m *ConfigSyncManager) GetLastSyncTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSyncTime
}

// GetLastSyncError returns the last sync error.
func (m *ConfigSyncManager) GetLastSyncError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastSyncErr
}

// GetSyncVersion returns the current sync version.
func (m *ConfigSyncManager) GetSyncVersion() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncVersion
}

// IsRunning returns whether the sync manager is running.
func (m *ConfigSyncManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// TriggerSync triggers an immediate sync.
func (m *ConfigSyncManager) TriggerSync() error {
	return m.Sync(m.ctx)
}
