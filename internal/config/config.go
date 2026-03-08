// Package config provides configuration management for the V Panel application.
// It supports loading configuration from YAML files and environment variables,
// with environment variables taking precedence over file values.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration.
type Config struct {
	Version     string            `yaml:"-"`
	Server      ServerConfig      `yaml:"server"`
	Database    DatabaseConfig    `yaml:"database"`
	Auth        AuthConfig        `yaml:"auth"`
	Xray        XrayConfig        `yaml:"xray"`
	Log         LogConfig         `yaml:"log"`
	Certificate CertificateConfig `yaml:"certificate"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Host            string        `yaml:"host" env:"V_SERVER_HOST" default:"0.0.0.0"`
	Port            int           `yaml:"port" env:"V_SERVER_PORT" default:"8080"`
	PublicURL       string        `yaml:"public_url" env:"V_SERVER_PUBLIC_URL" default:""`
	Mode            string        `yaml:"mode" env:"V_SERVER_MODE" default:"debug"`
	BaseURL         string        `yaml:"base_url" env:"V_SERVER_BASE_URL" default:""`
	ReadTimeout     time.Duration `yaml:"read_timeout" env:"V_SERVER_READ_TIMEOUT" default:"30s"`
	WriteTimeout    time.Duration `yaml:"write_timeout" env:"V_SERVER_WRITE_TIMEOUT" default:"10m"`
	IdleTimeout     time.Duration `yaml:"idle_timeout" env:"V_SERVER_IDLE_TIMEOUT" default:"60s"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout" env:"V_SERVER_SHUTDOWN_TIMEOUT" default:"30s"`
	TLSCert         string        `yaml:"tls_cert" env:"V_SERVER_TLS_CERT" default:""`
	TLSKey          string        `yaml:"tls_key" env:"V_SERVER_TLS_KEY" default:""`
	CORSOrigins     []string      `yaml:"cors_origins" env:"V_SERVER_CORS_ORIGINS"`
	StaticPath      string        `yaml:"static_path" env:"V_SERVER_STATIC_PATH" default:"./web/dist"`
}

// DatabaseConfig contains database connection settings.
type DatabaseConfig struct {
	Driver          string `yaml:"driver" env:"V_DB_DRIVER" default:"sqlite"`
	DSN             string `yaml:"dsn" env:"V_DB_DSN" default:"./data/v.db"`
	Path            string `yaml:"path" env:"V_DB_PATH" default:"./data/v.db"`
	MaxOpenConns    int    `yaml:"max_open_conns" env:"V_DB_MAX_OPEN_CONNS" default:"10"`
	MaxIdleConns    int    `yaml:"max_idle_conns" env:"V_DB_MAX_IDLE_CONNS" default:"5"`
	ConnMaxLifetime string `yaml:"conn_max_lifetime" env:"V_DB_CONN_MAX_LIFETIME" default:"1h"`
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	JWTSecret          string        `yaml:"jwt_secret" env:"V_JWT_SECRET" default:""`
	TokenExpiry        time.Duration `yaml:"token_expiry" env:"V_TOKEN_EXPIRY" default:"24h"`
	RefreshTokenExpiry time.Duration `yaml:"refresh_token_expiry" env:"V_REFRESH_TOKEN_EXPIRY" default:"168h"`
	AdminUsername      string        `yaml:"admin_username" env:"V_ADMIN_USER" default:"admin"`
	AdminPassword      string        `yaml:"admin_password" env:"V_ADMIN_PASS" default:"admin123"`
}

// XrayConfig contains Xray-core settings.
type XrayConfig struct {
	BinaryPath string `yaml:"binary_path" env:"V_XRAY_BINARY_PATH" default:"xray"`
	BinPath    string `yaml:"bin_path" env:"V_XRAY_BIN_PATH" default:"./xray/bin"`
	ConfigPath string `yaml:"config_path" env:"V_XRAY_CONFIG_PATH" default:"./xray/config.json"`
	BackupDir  string `yaml:"backup_dir" env:"V_XRAY_BACKUP_DIR" default:"/tmp/xray-backups"`
	Version    string `yaml:"version" env:"V_XRAY_VERSION" default:"latest"`
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level           string        `yaml:"level" env:"V_LOG_LEVEL" default:"info"`
	Format          string        `yaml:"format" env:"V_LOG_FORMAT" default:"json"`
	Output          string        `yaml:"output" env:"V_LOG_OUTPUT" default:"stdout"`
	DatabaseEnabled bool          `yaml:"database_enabled" env:"V_LOG_DB_ENABLED" default:"true"`
	DatabaseLevel   string        `yaml:"database_level" env:"V_LOG_DB_LEVEL" default:"info"`
	RetentionDays   int           `yaml:"retention_days" env:"V_LOG_RETENTION_DAYS" default:"30"`
	BufferSize      int           `yaml:"buffer_size" env:"V_LOG_BUFFER_SIZE" default:"1000"`
	BatchSize       int           `yaml:"batch_size" env:"V_LOG_BATCH_SIZE" default:"100"`
	FlushInterval   time.Duration `yaml:"flush_interval" env:"V_LOG_FLUSH_INTERVAL" default:"5s"`
}

// CertificateConfig contains certificate management settings.
type CertificateConfig struct {
	StoragePath      string        `yaml:"storage_path" env:"V_CERT_STORAGE_PATH" default:"./data/certificates"`
	AutoRenewEnabled bool          `yaml:"auto_renew_enabled" env:"V_CERT_AUTO_RENEW_ENABLED" default:"true"`
	CheckInterval    time.Duration `yaml:"check_interval" env:"V_CERT_CHECK_INTERVAL" default:"24h"`
	RenewThreshold   int           `yaml:"renew_threshold" env:"V_CERT_RENEW_THRESHOLD" default:"30"` // 提前多少天续期
}

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation error: %s - %s", e.Field, e.Message)
}

// ValidationErrors represents multiple configuration validation errors.
type ValidationErrors struct {
	Errors []ValidationError
}

func (e *ValidationErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no validation errors"
	}
	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return strings.Join(msgs, "; ")
}

// Add adds a validation error.
func (e *ValidationErrors) Add(field, message string) {
	e.Errors = append(e.Errors, ValidationError{Field: field, Message: message})
}

// HasErrors returns true if there are validation errors.
func (e *ValidationErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// MinJWTSecretLength is the minimum required length for JWT secrets.
const MinJWTSecretLength = 32

// Loader handles configuration loading from various sources.
type Loader struct {
	configPath string
}

// NewLoader creates a new configuration loader.
func NewLoader(configPath string) *Loader {
	return &Loader{configPath: configPath}
}

// Load loads configuration from file and environment variables.
// Environment variables take precedence over file values.
func (l *Loader) Load() (*Config, error) {
	cfg := &Config{}

	// Apply defaults first
	if err := applyDefaults(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	// Load from file if exists
	if l.configPath != "" {
		if err := l.loadFromFile(cfg); err != nil {
			// File not found is not an error, we'll use defaults
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load config file: %w", err)
			}
		}
	}

	// Override with environment variables (highest precedence)
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, fmt.Errorf("failed to apply env overrides: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from a YAML file.
func (l *Loader) loadFromFile(cfg *Config) error {
	data, err := os.ReadFile(l.configPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	return nil
}

// Validate validates the configuration and returns any errors.
func (cfg *Config) Validate() error {
	errs := &ValidationErrors{}

	// Validate server config
	cfg.validateServer(errs)

	// Validate database config
	cfg.validateDatabase(errs)

	// Validate auth config
	cfg.validateAuth(errs)

	// Validate xray config
	cfg.validateXray(errs)

	// Validate log config
	cfg.validateLog(errs)

	// Validate certificate config
	cfg.validateCertificate(errs)

	if errs.HasErrors() {
		return errs
	}

	return nil
}

// validateServer validates server configuration.
func (cfg *Config) validateServer(errs *ValidationErrors) {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		errs.Add("server.port", "must be between 1 and 65535")
	}

	validModes := map[string]bool{"debug": true, "release": true, "test": true}
	if !validModes[strings.ToLower(cfg.Server.Mode)] {
		errs.Add("server.mode", "must be one of: debug, release, test")
	}

	if cfg.Server.ReadTimeout < 0 {
		errs.Add("server.read_timeout", "must be non-negative")
	}

	if cfg.Server.WriteTimeout < 0 {
		errs.Add("server.write_timeout", "must be non-negative")
	}

	// Validate TLS configuration
	if (cfg.Server.TLSCert != "" && cfg.Server.TLSKey == "") ||
		(cfg.Server.TLSCert == "" && cfg.Server.TLSKey != "") {
		errs.Add("server.tls", "both tls_cert and tls_key must be provided together")
	}
}

// validateDatabase validates database configuration.
func (cfg *Config) validateDatabase(errs *ValidationErrors) {
	if cfg.Database.Path == "" && cfg.Database.DSN == "" {
		errs.Add("database.path", "database path or DSN must not be empty")
	}

	validDrivers := map[string]bool{"sqlite": true, "postgres": true, "mysql": true}
	if !validDrivers[strings.ToLower(cfg.Database.Driver)] {
		errs.Add("database.driver", "must be one of: sqlite, postgres, mysql")
	}

	// Validate DSN format based on driver
	if cfg.Database.DSN != "" {
		if err := cfg.validateDSN(); err != nil {
			errs.Add("database.dsn", err.Error())
		}
	}

	if cfg.Database.MaxOpenConns < 1 {
		errs.Add("database.max_open_conns", "must be at least 1")
	}

	if cfg.Database.MaxIdleConns < 0 {
		errs.Add("database.max_idle_conns", "must be non-negative")
	}

	if cfg.Database.MaxIdleConns > cfg.Database.MaxOpenConns {
		errs.Add("database.max_idle_conns", "must not exceed max_open_conns")
	}
}

// validateDSN validates the database connection string format.
func (cfg *Config) validateDSN() error {
	dsn := cfg.Database.DSN
	driver := strings.ToLower(cfg.Database.Driver)

	switch driver {
	case "sqlite":
		// SQLite DSN is just a file path
		if dsn == "" {
			return fmt.Errorf("SQLite DSN (file path) must not be empty")
		}
	case "postgres":
		// PostgreSQL DSN format: postgres://user:password@host:port/dbname?sslmode=disable
		// or: host=localhost port=5432 user=postgres password=secret dbname=mydb
		if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
			if !strings.Contains(dsn, "host=") && !strings.Contains(dsn, "dbname=") {
				return fmt.Errorf("invalid PostgreSQL DSN format")
			}
		}
	case "mysql":
		// MySQL DSN format: user:password@tcp(host:port)/dbname?charset=utf8mb4
		if !strings.Contains(dsn, "@") || !strings.Contains(dsn, "/") {
			return fmt.Errorf("invalid MySQL DSN format, expected: user:password@tcp(host:port)/dbname")
		}
	}

	return nil
}

// validateAuth validates authentication configuration.
func (cfg *Config) validateAuth(errs *ValidationErrors) {
	// JWT secret validation - must be at least 32 characters in production
	if cfg.Auth.JWTSecret == "" {
		// Generate a warning but don't fail - use a default for development
		cfg.Auth.JWTSecret = "development-secret-change-in-production"
	} else if len(cfg.Auth.JWTSecret) < MinJWTSecretLength {
		errs.Add("auth.jwt_secret", fmt.Sprintf("must be at least %d characters for security", MinJWTSecretLength))
	}

	if cfg.Auth.AdminUsername == "" {
		errs.Add("auth.admin_username", "must not be empty")
	}

	if cfg.Auth.AdminPassword == "" {
		errs.Add("auth.admin_password", "must not be empty")
	}

	// Validate password strength for admin
	if len(cfg.Auth.AdminPassword) < 8 {
		errs.Add("auth.admin_password", "must be at least 8 characters")
	}

	if cfg.Auth.TokenExpiry <= 0 {
		errs.Add("auth.token_expiry", "must be positive")
	}

	if cfg.Auth.RefreshTokenExpiry <= 0 {
		errs.Add("auth.refresh_token_expiry", "must be positive")
	}

	if cfg.Auth.RefreshTokenExpiry < cfg.Auth.TokenExpiry {
		errs.Add("auth.refresh_token_expiry", "must be greater than or equal to token_expiry")
	}
}

// validateXray validates Xray configuration.
func (cfg *Config) validateXray(errs *ValidationErrors) {
	if cfg.Xray.BinPath == "" {
		errs.Add("xray.bin_path", "must not be empty")
	}

	if cfg.Xray.ConfigPath == "" {
		errs.Add("xray.config_path", "must not be empty")
	}
}

// validateLog validates logging configuration.
func (cfg *Config) validateLog(errs *ValidationErrors) {
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "fatal": true}
	if !validLevels[strings.ToLower(cfg.Log.Level)] {
		errs.Add("log.level", "must be one of: debug, info, warn, error, fatal")
	}

	validFormats := map[string]bool{"json": true, "text": true}
	if !validFormats[strings.ToLower(cfg.Log.Format)] {
		errs.Add("log.format", "must be one of: json, text")
	}

	validOutputs := map[string]bool{"stdout": true, "stderr": true, "file": true}
	if cfg.Log.Output != "" && !validOutputs[strings.ToLower(cfg.Log.Output)] {
		// Allow file paths
		if !strings.HasPrefix(cfg.Log.Output, "/") && !strings.HasPrefix(cfg.Log.Output, "./") {
			errs.Add("log.output", "must be one of: stdout, stderr, or a file path")
		}
	}
}

// validateCertificate validates certificate configuration.
func (cfg *Config) validateCertificate(errs *ValidationErrors) {
	if cfg.Certificate.StoragePath == "" {
		errs.Add("certificate.storage_path", "must not be empty")
	}

	if cfg.Certificate.CheckInterval <= 0 {
		errs.Add("certificate.check_interval", "must be positive")
	}

	if cfg.Certificate.RenewThreshold < 1 || cfg.Certificate.RenewThreshold > 90 {
		errs.Add("certificate.renew_threshold", "must be between 1 and 90 days")
	}
}

// ValidateJWTSecret validates a JWT secret string.
func ValidateJWTSecret(secret string) error {
	if secret == "" {
		return &ValidationError{Field: "jwt_secret", Message: "must not be empty"}
	}
	if len(secret) < MinJWTSecretLength {
		return &ValidationError{
			Field:   "jwt_secret",
			Message: fmt.Sprintf("must be at least %d characters for security", MinJWTSecretLength),
		}
	}
	return nil
}

// IsProductionMode returns true if the server is in production mode.
func (cfg *Config) IsProductionMode() bool {
	return strings.ToLower(cfg.Server.Mode) == "release"
}

// GetBaseURL returns the base URL for the server.
// If BaseURL is configured, it returns that. Otherwise, it constructs one from Host and Port.
func (cfg *Config) GetBaseURL() string {
	if cfg.Server.BaseURL != "" {
		return strings.TrimSuffix(cfg.Server.BaseURL, "/")
	}
	
	scheme := "http"
	if cfg.Server.TLSCert != "" && cfg.Server.TLSKey != "" {
		scheme = "https"
	}
	
	host := cfg.Server.Host
	if host == "0.0.0.0" {
		host = "localhost"
	}
	
	return fmt.Sprintf("%s://%s:%d", scheme, host, cfg.Server.Port)
}

// ValidateForProduction performs stricter validation for production environments.
func (cfg *Config) ValidateForProduction() error {
	errs := &ValidationErrors{}

	// Basic validation first
	if err := cfg.Validate(); err != nil {
		if ve, ok := err.(*ValidationErrors); ok {
			errs.Errors = append(errs.Errors, ve.Errors...)
		} else {
			return err
		}
	}

	// Production-specific validations
	if cfg.Auth.JWTSecret == "development-secret-change-in-production" {
		errs.Add("auth.jwt_secret", "must be set to a secure value in production")
	}

	if cfg.Auth.AdminPassword == "admin123" {
		errs.Add("auth.admin_password", "must be changed from default in production")
	}

	if cfg.Server.Mode != "release" {
		errs.Add("server.mode", "should be 'release' in production")
	}

	if errs.HasErrors() {
		return errs
	}

	return nil
}

// applyDefaults applies default values to the configuration.
func applyDefaults(cfg *Config) error {
	return applyDefaultsToStruct(reflect.ValueOf(cfg).Elem())
}

func applyDefaultsToStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if field.Kind() == reflect.Struct {
			if err := applyDefaultsToStruct(field); err != nil {
				return err
			}
			continue
		}

		defaultTag := fieldType.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		if err := setFieldValue(field, defaultTag); err != nil {
			return fmt.Errorf("failed to set default for %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the configuration.
func applyEnvOverrides(cfg *Config) error {
	return applyEnvOverridesToStruct(reflect.ValueOf(cfg).Elem())
}

func applyEnvOverridesToStruct(v reflect.Value) error {
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		if field.Kind() == reflect.Struct {
			if err := applyEnvOverridesToStruct(field); err != nil {
				return err
			}
			continue
		}

		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		envValue := os.Getenv(envTag)
		if envValue == "" {
			continue
		}

		if err := setFieldValue(field, envValue); err != nil {
			return fmt.Errorf("failed to set env var %s: %w", envTag, err)
		}
	}

	return nil
}

// setFieldValue sets a field value from a string.
func setFieldValue(field reflect.Value, value string) error {
	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Int, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			d, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.Set(reflect.ValueOf(d))
		} else {
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			field.SetInt(i)
		}
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(b)
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}
	return nil
}

// GetConfigPath returns the default configuration file path.
func GetConfigPath() string {
	// Check environment variable first
	if path := os.Getenv("V_CONFIG_PATH"); path != "" {
		return path
	}

	// Check common locations
	paths := []string{
		"./configs/config.yaml",
		"./config.yaml",
		"/app/configs/config.yaml",
		"/etc/v/config.yaml",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return "./configs/config.yaml"
}

// EnsureDataDir ensures the data directory exists.
func EnsureDataDir(dbPath string) error {
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, 0755)
}


// Load loads configuration from the specified path.
func Load(configPath string) (*Config, error) {
	loader := NewLoader(configPath)
	cfg, err := loader.Load()
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Ensure data directory exists
	if err := EnsureDataDir(cfg.Database.Path); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return cfg, nil
}
