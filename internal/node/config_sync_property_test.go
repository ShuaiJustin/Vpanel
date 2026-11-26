// Package node provides node management functionality.
package node

import (
	"context"
	"testing"
	"testing/quick"
	"time"
)

// Feature: multi-server-management, Property 18: Config Validation Before Sync
// Validates: Requirements 7.7
// For any configuration sync attempt, invalid configurations SHALL be rejected before being sent to nodes.

// TestProperty_ConfigValidation_NilConfigRejected tests that nil configs are rejected.
func TestProperty_ConfigValidation_NilConfigRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	err := syncer.ValidateConfig(context.Background(), nil)
	if err == nil {
		t.Error("Expected nil config to be rejected")
	}
}

// TestProperty_ConfigValidation_EmptyVersionRejected tests that configs with empty version are rejected.
func TestProperty_ConfigValidation_EmptyVersionRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	config := &NodeConfig{
		Version:   "",
		Timestamp: time.Now(),
		Proxies:   []ProxyConfig{},
	}

	err := syncer.ValidateConfig(context.Background(), config)
	if err == nil {
		t.Error("Expected config with empty version to be rejected")
	}
}

// TestProperty_ConfigValidation_ZeroTimestampRejected tests that configs with zero timestamp are rejected.
func TestProperty_ConfigValidation_ZeroTimestampRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	config := &NodeConfig{
		Version:   "1.0",
		Timestamp: time.Time{},
		Proxies:   []ProxyConfig{},
	}

	err := syncer.ValidateConfig(context.Background(), config)
	if err == nil {
		t.Error("Expected config with zero timestamp to be rejected")
	}
}

// TestProperty_ConfigValidation_ValidConfigAccepted tests that valid configs are accepted.
func TestProperty_ConfigValidation_ValidConfigAccepted(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	// Property: For any valid config, validation should pass
	f := func(version string, proxyCount uint8) bool {
		if version == "" {
			return true // Skip empty versions
		}

		// Create valid proxies
		proxies := make([]ProxyConfig, int(proxyCount%10)) // 0-9 proxies
		for i := range proxies {
			proxies[i] = ProxyConfig{
				ID:       int64(i + 1),
				UserID:   1,
				Name:     "test-proxy",
				Protocol: "vmess",
				Port:     10000 + i,
				Enabled:  true,
				Settings: map[string]any{"uuid": "test-uuid"},
			}
		}

		config := &NodeConfig{
			Version:   version,
			Timestamp: time.Now(),
			Proxies:   proxies,
		}

		err := syncer.ValidateConfig(context.Background(), config)
		return err == nil
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ConfigValidation_InvalidPortRejected tests that invalid ports are rejected.
func TestProperty_ConfigValidation_InvalidPortRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	// Property: For any port outside valid range (1-65535), validation should fail
	f := func(port int) bool {
		// Only test invalid ports
		if port > 0 && port <= 65535 {
			return true // Skip valid ports
		}

		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies: []ProxyConfig{
				{
					ID:       1,
					UserID:   1,
					Name:     "test",
					Protocol: "vmess",
					Port:     port,
					Enabled:  true,
				},
			},
		}

		err := syncer.ValidateConfig(context.Background(), config)
		return err != nil // Should be rejected
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ConfigValidation_DuplicatePortsRejected tests that duplicate ports are rejected.
func TestProperty_ConfigValidation_DuplicatePortsRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	// Property: For any config with duplicate ports, validation should fail
	f := func(port uint16) bool {
		if port == 0 {
			return true // Skip invalid port
		}

		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies: []ProxyConfig{
				{
					ID:       1,
					UserID:   1,
					Name:     "proxy1",
					Protocol: "vmess",
					Port:     int(port),
					Enabled:  true,
				},
				{
					ID:       2,
					UserID:   1,
					Name:     "proxy2",
					Protocol: "vless",
					Port:     int(port), // Same port
					Enabled:  true,
				},
			},
		}

		err := syncer.ValidateConfig(context.Background(), config)
		return err != nil // Should be rejected
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ConfigValidation_InvalidProtocolRejected tests that invalid protocols are rejected.
func TestProperty_ConfigValidation_InvalidProtocolRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	invalidProtocols := []string{
		"",
		"invalid",
		"http",
		"socks",
		"VMESS", // Case sensitive
		"Vless",
	}

	for _, protocol := range invalidProtocols {
		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies: []ProxyConfig{
				{
					ID:       1,
					UserID:   1,
					Name:     "test",
					Protocol: protocol,
					Port:     10000,
					Enabled:  true,
				},
			},
		}

		err := syncer.ValidateConfig(context.Background(), config)
		if err == nil {
			t.Errorf("Expected protocol %q to be rejected", protocol)
		}
	}
}

// TestProperty_ConfigValidation_ValidProtocolsAccepted tests that valid protocols are accepted.
func TestProperty_ConfigValidation_ValidProtocolsAccepted(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	validProtocols := []string{
		"vmess",
		"vless",
		"trojan",
		"shadowsocks",
	}

	for i, protocol := range validProtocols {
		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies: []ProxyConfig{
				{
					ID:       int64(i + 1),
					UserID:   1,
					Name:     "test",
					Protocol: protocol,
					Port:     10000 + i,
					Enabled:  true,
				},
			},
		}

		err := syncer.ValidateConfig(context.Background(), config)
		if err != nil {
			t.Errorf("Expected protocol %q to be accepted, got error: %v", protocol, err)
		}
	}
}

// TestProperty_ConfigValidation_EmptyProxyNameRejected tests that empty proxy names are rejected.
func TestProperty_ConfigValidation_EmptyProxyNameRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	config := &NodeConfig{
		Version:   "1.0",
		Timestamp: time.Now(),
		Proxies: []ProxyConfig{
			{
				ID:       1,
				UserID:   1,
				Name:     "", // Empty name
				Protocol: "vmess",
				Port:     10000,
				Enabled:  true,
			},
		},
	}

	err := syncer.ValidateConfig(context.Background(), config)
	if err == nil {
		t.Error("Expected empty proxy name to be rejected")
	}
}

// TestProperty_ConfigValidation_InvalidProxyIDRejected tests that invalid proxy IDs are rejected.
func TestProperty_ConfigValidation_InvalidProxyIDRejected(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	// Property: For any proxy with ID <= 0, validation should fail
	f := func(id int64) bool {
		if id > 0 {
			return true // Skip valid IDs
		}

		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies: []ProxyConfig{
				{
					ID:       id,
					UserID:   1,
					Name:     "test",
					Protocol: "vmess",
					Port:     10000,
					Enabled:  true,
				},
			},
		}

		err := syncer.ValidateConfig(context.Background(), config)
		return err != nil // Should be rejected
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ConfigValidation_UniquePortsAccepted tests that configs with unique ports are accepted.
func TestProperty_ConfigValidation_UniquePortsAccepted(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	// Property: For any config with unique ports, validation should pass
	f := func(basePort uint16, count uint8) bool {
		if basePort == 0 || basePort > 65000 {
			return true // Skip edge cases
		}

		proxyCount := int(count%10) + 1 // 1-10 proxies
		if int(basePort)+proxyCount > 65535 {
			return true // Skip if ports would overflow
		}

		proxies := make([]ProxyConfig, proxyCount)
		for i := range proxies {
			proxies[i] = ProxyConfig{
				ID:       int64(i + 1),
				UserID:   1,
				Name:     "test-proxy",
				Protocol: "vmess",
				Port:     int(basePort) + i, // Unique ports
				Enabled:  true,
			}
		}

		config := &NodeConfig{
			Version:   "1.0",
			Timestamp: time.Now(),
			Proxies:   proxies,
		}

		err := syncer.ValidateConfig(context.Background(), config)
		return err == nil
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_ConfigValidation_EmptyProxiesAccepted tests that configs with no proxies are accepted.
func TestProperty_ConfigValidation_EmptyProxiesAccepted(t *testing.T) {
	syncer := &configSync{
		config: DefaultConfigSyncConfig(),
	}

	config := &NodeConfig{
		Version:   "1.0",
		Timestamp: time.Now(),
		Proxies:   []ProxyConfig{},
	}

	err := syncer.ValidateConfig(context.Background(), config)
	if err != nil {
		t.Errorf("Expected empty proxies to be accepted, got error: %v", err)
	}
}
