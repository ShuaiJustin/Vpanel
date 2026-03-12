// Package agent provides the Node Agent functionality for V Panel.
// The Node Agent runs on each Xray node server and communicates with the Panel Server.
package agent

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete agent configuration.
type Config struct {
	Node   NodeConfig   `yaml:"node"`
	Panel  PanelConfig  `yaml:"panel"`
	Xray   XrayConfig   `yaml:"xray"`
	Health HealthConfig `yaml:"health"`
	Log    LogConfig    `yaml:"log"`
}

// NodeConfig contains node identification settings.
type NodeConfig struct {
	Name  string `yaml:"name" env:"AGENT_NODE_NAME"`
	Token string `yaml:"token" env:"AGENT_NODE_TOKEN"`
}

// PanelConfig contains Panel Server connection settings.
type PanelConfig struct {
	URL               string        `yaml:"url" env:"AGENT_PANEL_URL"`
	TLSSkipVerify     bool          `yaml:"tls_skip_verify" env:"AGENT_PANEL_TLS_SKIP_VERIFY"`
	ConnectTimeout    time.Duration `yaml:"connect_timeout" env:"AGENT_PANEL_CONNECT_TIMEOUT"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval" env:"AGENT_PANEL_RECONNECT_INTERVAL"`
	MaxReconnectDelay time.Duration `yaml:"max_reconnect_delay" env:"AGENT_PANEL_MAX_RECONNECT_DELAY"`
}

// XrayConfig contains Xray-core settings.
type XrayConfig struct {
	BinaryPath string `yaml:"binary_path" env:"AGENT_XRAY_BINARY_PATH"`
	ConfigPath string `yaml:"config_path" env:"AGENT_XRAY_CONFIG_PATH"`
	BackupDir  string `yaml:"backup_dir" env:"AGENT_XRAY_BACKUP_DIR"`
}

// HealthConfig contains health check endpoint settings.
type HealthConfig struct {
	Port int    `yaml:"port" env:"AGENT_HEALTH_PORT"`
	Host string `yaml:"host" env:"AGENT_HEALTH_HOST"`
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level  string `yaml:"level" env:"AGENT_LOG_LEVEL"`
	Format string `yaml:"format" env:"AGENT_LOG_FORMAT"`
	Output string `yaml:"output" env:"AGENT_LOG_OUTPUT"`
}

// DefaultConfig returns the default agent configuration.
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			Name:  "",
			Token: "",
		},
		Panel: PanelConfig{
			URL:               "",
			TLSSkipVerify:     false,
			ConnectTimeout:    10 * time.Second,
			ReconnectInterval: 5 * time.Second,
			MaxReconnectDelay: 5 * time.Minute,
		},
		Xray: XrayConfig{
			BinaryPath: "xray",
			ConfigPath: "/usr/local/etc/xray/config.json",
			BackupDir:  "/tmp/xray-backups",
		},
		Health: HealthConfig{
			Port: 18443,
			Host: "0.0.0.0",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Load from file if exists
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
			// File not found, use defaults and env vars
		} else {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}
		}
	}

	// Override with environment variables
	applyEnvOverrides(cfg)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func applyEnvOverrides(cfg *Config) {
	// Node config
	if v := os.Getenv("AGENT_NODE_NAME"); v != "" {
		cfg.Node.Name = v
	}
	if v := os.Getenv("AGENT_NODE_TOKEN"); v != "" {
		cfg.Node.Token = v
	}

	// Panel config
	if v := os.Getenv("AGENT_PANEL_URL"); v != "" {
		cfg.Panel.URL = v
	}
	if v := os.Getenv("AGENT_PANEL_TLS_SKIP_VERIFY"); v == "true" {
		cfg.Panel.TLSSkipVerify = true
	}

	// Xray config
	if v := os.Getenv("AGENT_XRAY_BINARY_PATH"); v != "" {
		cfg.Xray.BinaryPath = v
	}
	if v := os.Getenv("AGENT_XRAY_CONFIG_PATH"); v != "" {
		cfg.Xray.ConfigPath = v
	}
	if v := os.Getenv("AGENT_XRAY_BACKUP_DIR"); v != "" {
		cfg.Xray.BackupDir = v
	}

	// Health config
	if v := os.Getenv("AGENT_HEALTH_HOST"); v != "" {
		cfg.Health.Host = v
	}

	// Log config
	if v := os.Getenv("AGENT_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("AGENT_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("AGENT_LOG_OUTPUT"); v != "" {
		cfg.Log.Output = v
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.Node.Token == "" {
		return fmt.Errorf("node token is required")
	}

	if c.Panel.URL == "" {
		return fmt.Errorf("panel URL is required")
	}
	if isLoopbackPanelURL(c.Panel.URL) && !isTrue(os.Getenv("AGENT_ALLOW_LOCAL_PANEL")) {
		return fmt.Errorf("panel URL cannot be localhost/127.0.0.1 in production node mode")
	}

	if c.Health.Port <= 0 || c.Health.Port > 65535 {
		return fmt.Errorf("health port must be between 1 and 65535")
	}

	return nil
}

func isLoopbackPanelURL(raw string) bool {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	return strings.Contains(normalized, "localhost") || strings.Contains(normalized, "127.0.0.1")
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}
