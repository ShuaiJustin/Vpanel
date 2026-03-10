package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 1: Configuration Precedence
// For any configuration key, when both an environment variable (with V_ prefix)
// and a config file value are set, the environment variable value SHALL take
// precedence over the config file value, and both SHALL take precedence over default values.
// **Validates: Requirements 1.6, 5.1, 5.3**

func TestConfigPrecedence_EnvOverridesFile(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("env vars override config file values for server port", prop.ForAll(
		func(filePort, envPort int) bool {
			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := `server:
  port: ` + string(rune(filePort+'0')) + `
`
			// Write file with actual port value
			configContent = "server:\n  port: " + itoa(filePort) + "\n"
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Logf("Failed to write config file: %v", err)
				return false
			}

			// Set environment variable
			os.Setenv("V_SERVER_PORT", itoa(envPort))
			defer os.Unsetenv("V_SERVER_PORT")

			// Load config
			loader := NewLoader(configPath)
			cfg, err := loader.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Environment variable should take precedence
			return cfg.Server.Port == envPort
		},
		gen.IntRange(1024, 65535),
		gen.IntRange(1024, 65535),
	))

	properties.Property("env vars override config file values for log level", prop.ForAll(
		func(fileLevel, envLevel string) bool {
			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := "log:\n  level: " + fileLevel + "\n"
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Logf("Failed to write config file: %v", err)
				return false
			}

			// Set environment variable
			os.Setenv("V_LOG_LEVEL", envLevel)
			defer os.Unsetenv("V_LOG_LEVEL")

			// Load config
			loader := NewLoader(configPath)
			cfg, err := loader.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// Environment variable should take precedence
			return cfg.Log.Level == envLevel
		},
		gen.OneConstOf("debug", "info", "warn", "error"),
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	properties.TestingRun(t)
}

func TestConfigPrecedence_FileOverridesDefaults(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("config file values override defaults for server port", prop.ForAll(
		func(filePort int) bool {
			// Clear any environment variables
			os.Unsetenv("V_SERVER_PORT")

			// Create temp config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			configContent := "server:\n  port: " + itoa(filePort) + "\n"
			if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
				t.Logf("Failed to write config file: %v", err)
				return false
			}

			// Load config
			loader := NewLoader(configPath)
			cfg, err := loader.Load()
			if err != nil {
				t.Logf("Failed to load config: %v", err)
				return false
			}

			// File value should override default (8080)
			return cfg.Server.Port == filePort
		},
		gen.IntRange(1024, 65535),
	))

	properties.TestingRun(t)
}

func TestConfigPrecedence_DefaultsApplied(t *testing.T) {
	// Clear all relevant environment variables
	envVars := []string{
		"V_SERVER_HOST", "V_SERVER_PORT",
		"V_DB_PATH", "V_LOG_LEVEL", "V_LOG_FORMAT",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	// Load config without file
	loader := NewLoader("")
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Check defaults are applied
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Expected default log level info, got %s", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Expected default log format json, got %s", cfg.Log.Format)
	}
}

// Helper function to convert int to string
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}


// Property 4: Configuration Validation at Startup
// For any invalid configuration (missing required fields, invalid values),
// the Backend SHALL fail to start and log a specific error message identifying
// the invalid configuration.
// **Validates: Requirements 5.4**

func TestConfigValidation_InvalidPort(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("invalid port values are rejected", prop.ForAll(
		func(port int) bool {
			cfg := &Config{
				Server: ServerConfig{
					Host: "0.0.0.0",
					Port: port,
					Mode: "debug",
				},
				Database: DatabaseConfig{
					Driver: "sqlite",
					Path:   "/data/v.db",
				},
				Auth: AuthConfig{
					JWTSecret:          "this-is-a-very-long-secret-key-for-testing-purposes",
					AdminUsername:      "admin",
					AdminPassword:      "admin12345678",
					TokenExpiry:        24 * 60 * 60 * 1000000000,
					RefreshTokenExpiry: 168 * 60 * 60 * 1000000000,
				},
				Xray: XrayConfig{
					BinPath:    "./xray/bin",
					ConfigPath: "./xray/config.json",
				},
				Log: LogConfig{
					Level:  "info",
					Format: "json",
				},
			}

			err := cfg.Validate()

			// Port must be between 1 and 65535
			if port < 1 || port > 65535 {
				// Should return validation error
				if err == nil {
					return false
				}
				return hasValidationError(err, "server.port")
			}
			// Valid port should not cause validation error for this field
			return !hasValidationError(err, "server.port")
		},
		gen.OneGenOf(
			gen.IntRange(-1000, 0),      // Invalid: negative or zero
			gen.IntRange(65536, 100000), // Invalid: too high
			gen.IntRange(1, 65535),      // Valid range
		),
	))

	properties.TestingRun(t)
}

// hasValidationError checks if the error contains a validation error for the given field.
func hasValidationError(err error, field string) bool {
	if err == nil {
		return false
	}
	if ve, ok := err.(*ValidationError); ok {
		return ve.Field == field
	}
	if ves, ok := err.(*ValidationErrors); ok {
		for _, e := range ves.Errors {
			if e.Field == field {
				return true
			}
		}
	}
	return false
}

func TestConfigValidation_InvalidLogLevel(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error", "fatal"}
	invalidLevels := []string{"trace", "verbose", "critical", "none", "invalid"}

	// Test valid levels pass validation
	for _, level := range validLevels {
		cfg := createValidConfig()
		cfg.Log.Level = level
		if err := cfg.Validate(); err != nil {
			if hasValidationError(err, "log.level") {
				t.Errorf("Valid log level %q should not cause error, got: %v", level, err)
			}
		}
	}

	// Test invalid levels fail validation
	for _, level := range invalidLevels {
		cfg := createValidConfig()
		cfg.Log.Level = level
		err := cfg.Validate()
		if !hasValidationError(err, "log.level") {
			t.Errorf("Invalid log level %q should cause error", level)
		}
	}
}

func TestGetBaseURL_Priority(t *testing.T) {
	cfg := createValidConfig()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 8080

	cfg.Server.BaseURL = "https://base.example.com/"
	cfg.Server.PublicURL = "https://public.example.com/"
	if got := cfg.GetBaseURL(); got != "https://base.example.com" {
		t.Fatalf("expected base_url to win, got %s", got)
	}

	cfg.Server.BaseURL = ""
	if got := cfg.GetBaseURL(); got != "https://public.example.com" {
		t.Fatalf("expected public_url fallback, got %s", got)
	}
}

func TestConfigValidation_InvalidLogFormat(t *testing.T) {
	validFormats := []string{"json", "text"}
	invalidFormats := []string{"xml", "csv", "yaml", "invalid"}

	// Test valid formats pass validation
	for _, format := range validFormats {
		cfg := createValidConfig()
		cfg.Log.Format = format
		if err := cfg.Validate(); err != nil {
			if hasValidationError(err, "log.format") {
				t.Errorf("Valid log format %q should not cause error, got: %v", format, err)
			}
		}
	}

	// Test invalid formats fail validation
	for _, format := range invalidFormats {
		cfg := createValidConfig()
		cfg.Log.Format = format
		err := cfg.Validate()
		if !hasValidationError(err, "log.format") {
			t.Errorf("Invalid log format %q should cause error", format)
		}
	}
}

func TestConfigValidation_EmptyRequiredFields(t *testing.T) {
	// Test empty database path
	cfg := createValidConfig()
	cfg.Database.Path = ""
	cfg.Database.DSN = ""

	err := cfg.Validate()
	if !hasValidationError(err, "database.path") {
		t.Error("Expected validation error for empty database path")
	}

	// Test empty admin username
	cfg = createValidConfig()
	cfg.Auth.AdminUsername = ""
	err = cfg.Validate()
	if !hasValidationError(err, "auth.admin_username") {
		t.Error("Expected validation error for empty admin username")
	}

	// Test empty admin password
	cfg = createValidConfig()
	cfg.Auth.AdminPassword = ""
	err = cfg.Validate()
	if !hasValidationError(err, "auth.admin_password") {
		t.Error("Expected validation error for empty admin password")
	}
}

// createValidConfig creates a valid configuration for testing.
func createValidConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
			Mode: "debug",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			Path:   "/data/v.db",
		},
		Auth: AuthConfig{
			JWTSecret:          "this-is-a-very-long-secret-key-for-testing-purposes",
			AdminUsername:      "admin",
			AdminPassword:      "admin12345678",
			TokenExpiry:        24 * 60 * 60 * 1000000000,
			RefreshTokenExpiry: 168 * 60 * 60 * 1000000000,
		},
		Xray: XrayConfig{
			BinPath:    "./xray/bin",
			ConfigPath: "./xray/config.json",
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
	}
}


// Property 4: JWT Secret Validation
// For any JWT secret configuration, secrets with length less than 32 characters
// SHALL be rejected during configuration validation.
// **Validates: Requirements 1.7**

func TestJWTSecretValidation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("JWT secrets shorter than 32 characters are rejected", prop.ForAll(
		func(secretLen int) bool {
			secret := generateString(secretLen)
			cfg := createValidConfig()
			cfg.Auth.JWTSecret = secret

			err := cfg.Validate()

			if secretLen < MinJWTSecretLength {
				// Should be rejected
				return hasValidationError(err, "auth.jwt_secret")
			}
			// Should be accepted
			return !hasValidationError(err, "auth.jwt_secret")
		},
		gen.IntRange(1, 64),
	))

	properties.TestingRun(t)
}

func TestJWTSecretValidation_Boundary(t *testing.T) {
	// Test exactly at boundary
	testCases := []struct {
		length   int
		expected bool // true = should pass validation
	}{
		{31, false},
		{32, true},
		{33, true},
		{64, true},
	}

	for _, tc := range testCases {
		secret := generateString(tc.length)
		cfg := createValidConfig()
		cfg.Auth.JWTSecret = secret

		err := cfg.Validate()
		hasError := hasValidationError(err, "auth.jwt_secret")

		if tc.expected && hasError {
			t.Errorf("JWT secret of length %d should pass validation", tc.length)
		}
		if !tc.expected && !hasError {
			t.Errorf("JWT secret of length %d should fail validation", tc.length)
		}
	}
}

func TestValidateJWTSecret(t *testing.T) {
	// Test empty secret
	err := ValidateJWTSecret("")
	if err == nil {
		t.Error("Empty JWT secret should fail validation")
	}

	// Test short secret
	err = ValidateJWTSecret("short")
	if err == nil {
		t.Error("Short JWT secret should fail validation")
	}

	// Test valid secret
	err = ValidateJWTSecret("this-is-a-very-long-secret-key-for-testing")
	if err != nil {
		t.Errorf("Valid JWT secret should pass validation, got: %v", err)
	}
}

// Property 22: Configuration Validation
// For any application startup with missing required configuration values,
// the application SHALL fail to start with a clear error message indicating
// the missing configuration.
// **Validates: Requirements 11.1, 11.2**

func TestConfigurationValidation_MissingRequired(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("missing required fields cause validation errors", prop.ForAll(
		func(fieldToEmpty int) bool {
			cfg := createValidConfig()

			// Empty one required field based on the random number
			switch fieldToEmpty % 5 {
			case 0:
				cfg.Database.Path = ""
				cfg.Database.DSN = ""
				return hasValidationError(cfg.Validate(), "database.path")
			case 1:
				cfg.Auth.AdminUsername = ""
				return hasValidationError(cfg.Validate(), "auth.admin_username")
			case 2:
				cfg.Auth.AdminPassword = ""
				return hasValidationError(cfg.Validate(), "auth.admin_password")
			case 3:
				cfg.Xray.BinPath = ""
				return hasValidationError(cfg.Validate(), "xray.bin_path")
			case 4:
				cfg.Xray.ConfigPath = ""
				return hasValidationError(cfg.Validate(), "xray.config_path")
			}
			return true
		},
		gen.IntRange(0, 100),
	))

	properties.TestingRun(t)
}

func TestConfigurationValidation_InvalidValues(t *testing.T) {
	// Test invalid server mode
	cfg := createValidConfig()
	cfg.Server.Mode = "invalid"
	if !hasValidationError(cfg.Validate(), "server.mode") {
		t.Error("Invalid server mode should cause validation error")
	}

	// Test invalid database driver
	cfg = createValidConfig()
	cfg.Database.Driver = "invalid"
	if !hasValidationError(cfg.Validate(), "database.driver") {
		t.Error("Invalid database driver should cause validation error")
	}

	// Test negative max open conns
	cfg = createValidConfig()
	cfg.Database.MaxOpenConns = 0
	if !hasValidationError(cfg.Validate(), "database.max_open_conns") {
		t.Error("Zero max_open_conns should cause validation error")
	}

	// Test max idle > max open
	cfg = createValidConfig()
	cfg.Database.MaxOpenConns = 5
	cfg.Database.MaxIdleConns = 10
	if !hasValidationError(cfg.Validate(), "database.max_idle_conns") {
		t.Error("max_idle_conns > max_open_conns should cause validation error")
	}
}

func TestConfigurationValidation_DSNFormat(t *testing.T) {
	// Test PostgreSQL DSN validation
	cfg := createValidConfig()
	cfg.Database.Driver = "postgres"
	cfg.Database.DSN = "invalid-dsn"
	if !hasValidationError(cfg.Validate(), "database.dsn") {
		t.Error("Invalid PostgreSQL DSN should cause validation error")
	}

	// Valid PostgreSQL DSN
	cfg.Database.DSN = "postgres://user:pass@localhost:5432/dbname"
	if hasValidationError(cfg.Validate(), "database.dsn") {
		t.Error("Valid PostgreSQL DSN should not cause validation error")
	}

	// Test MySQL DSN validation
	cfg.Database.Driver = "mysql"
	cfg.Database.DSN = "invalid-dsn"
	if !hasValidationError(cfg.Validate(), "database.dsn") {
		t.Error("Invalid MySQL DSN should cause validation error")
	}

	// Valid MySQL DSN
	cfg.Database.DSN = "user:pass@tcp(localhost:3306)/dbname"
	if hasValidationError(cfg.Validate(), "database.dsn") {
		t.Error("Valid MySQL DSN should not cause validation error")
	}
}

func TestConfigurationValidation_ProductionMode(t *testing.T) {
	// Test production validation
	cfg := createValidConfig()
	cfg.Server.Mode = "release"
	cfg.Auth.JWTSecret = "development-secret-change-in-production"
	cfg.Auth.AdminPassword = "admin123"

	err := cfg.ValidateForProduction()
	if err == nil {
		t.Error("Production validation should fail with default secrets")
	}

	// Check specific errors
	if !hasValidationError(err, "auth.jwt_secret") {
		t.Error("Production validation should flag default JWT secret")
	}
	if !hasValidationError(err, "auth.admin_password") {
		t.Error("Production validation should flag default admin password")
	}
}

func TestConfigurationValidation_TLSConfig(t *testing.T) {
	// Test TLS config - both must be provided together
	cfg := createValidConfig()
	cfg.Server.TLSCert = "/path/to/cert.pem"
	cfg.Server.TLSKey = ""
	if !hasValidationError(cfg.Validate(), "server.tls") {
		t.Error("TLS cert without key should cause validation error")
	}

	cfg.Server.TLSCert = ""
	cfg.Server.TLSKey = "/path/to/key.pem"
	if !hasValidationError(cfg.Validate(), "server.tls") {
		t.Error("TLS key without cert should cause validation error")
	}

	// Both provided - should pass
	cfg.Server.TLSCert = "/path/to/cert.pem"
	cfg.Server.TLSKey = "/path/to/key.pem"
	if hasValidationError(cfg.Validate(), "server.tls") {
		t.Error("Valid TLS config should not cause validation error")
	}
}

func TestConfigurationValidation_TokenExpiry(t *testing.T) {
	// Test token expiry validation
	cfg := createValidConfig()
	cfg.Auth.TokenExpiry = 0
	if !hasValidationError(cfg.Validate(), "auth.token_expiry") {
		t.Error("Zero token expiry should cause validation error")
	}

	cfg.Auth.TokenExpiry = -1
	if !hasValidationError(cfg.Validate(), "auth.token_expiry") {
		t.Error("Negative token expiry should cause validation error")
	}

	// Refresh token must be >= token expiry
	cfg = createValidConfig()
	cfg.Auth.TokenExpiry = 24 * 60 * 60 * 1000000000     // 24h
	cfg.Auth.RefreshTokenExpiry = 1 * 60 * 60 * 1000000000 // 1h
	if !hasValidationError(cfg.Validate(), "auth.refresh_token_expiry") {
		t.Error("Refresh token expiry < token expiry should cause validation error")
	}
}

// generateString generates a string of the specified length.
func generateString(length int) string {
	if length <= 0 {
		return ""
	}
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = 'a' + byte(i%26)
	}
	return string(result)
}
