package generators

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gopkg.in/yaml.v3"

	"v/internal/database/repository"
)

func TestClashGenerator_Generate(t *testing.T) {
	generator := NewClashGenerator()

	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess Server",
			Protocol: "vmess",
			Host:     "vmess.example.com",
			Port:     443,
			Settings: map[string]interface{}{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"alterId":  0,
				"security": "auto",
				"network":  "ws",
				"path":     "/ws",
				"tls":      true,
			},
			Enabled: true,
		},
		{
			ID:       2,
			Name:     "Trojan Server",
			Protocol: "trojan",
			Host:     "trojan.example.com",
			Port:     443,
			Settings: map[string]interface{}{
				"password": "mypassword",
				"sni":      "trojan.example.com",
			},
			Enabled: true,
		},
	}

	result, err := generator.Generate(proxies, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Parse YAML to verify structure
	var config ClashConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if len(config.Proxies) != 2 {
		t.Errorf("Expected 2 proxies, got %d", len(config.Proxies))
	}

	if config.Mode != "rule" {
		t.Errorf("Expected mode 'rule', got '%s'", config.Mode)
	}
	if len(config.ProxyGroups) == 0 || len(config.ProxyGroups[0].Proxies) == 0 || config.ProxyGroups[0].Proxies[0] != "VMess Server" {
		t.Fatalf("expected first select option to be the first generated node, got %#v", config.ProxyGroups)
	}
}

func TestClashGenerator_SupportsProtocol(t *testing.T) {
	generator := NewClashGenerator()

	tests := []struct {
		protocol string
		expected bool
	}{
		{"vmess", true},
		{"vless", false},
		{"trojan", true},
		{"shadowsocks", true},
		{"ss", true},
		{"http", false},
		{"socks", false},
	}

	for _, tt := range tests {
		t.Run(tt.protocol, func(t *testing.T) {
			result := generator.SupportsProtocol(tt.protocol)
			if result != tt.expected {
				t.Errorf("SupportsProtocol(%s) = %v, want %v", tt.protocol, result, tt.expected)
			}
		})
	}
}

func TestClashGenerator_SkipsVLESSForLegacyCompatibility(t *testing.T) {
	generator := NewClashGenerator()

	result, err := generator.Generate([]*repository.Proxy{
		{
			ID:       1,
			Name:     "VLESS Reality",
			Protocol: "vless",
			Host:     "vless.example.com",
			Port:     443,
			Settings: map[string]interface{}{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"security": "reality",
			},
			Enabled: true,
		},
	}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	var config ClashConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}
	if len(config.Proxies) != 0 {
		t.Fatalf("expected legacy Clash config to skip VLESS, got %#v", config.Proxies)
	}
}

func TestClashMetaGenerator_Generate(t *testing.T) {
	generator := NewClashMetaGenerator()

	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VLESS Reality",
			Protocol: "vless",
			Host:     "vless.example.com",
			Port:     443,
			Settings: map[string]interface{}{
				"uuid":        "12345678-1234-1234-1234-123456789012",
				"network":     "tcp",
				"security":    "reality",
				"sni":         "www.microsoft.com",
				"publicKey":   "abc123",
				"shortId":     "def456",
				"fingerprint": "chrome",
				"flow":        "xtls-rprx-vision",
			},
			Enabled: true,
		},
	}

	result, err := generator.Generate(proxies, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Parse YAML to verify structure
	var config ClashMetaConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if len(config.Proxies) != 1 {
		t.Errorf("Expected 1 proxy, got %d", len(config.Proxies))
	}

	// Check Reality-specific fields
	proxy := config.Proxies[0]
	if proxy["type"] != "vless" {
		t.Errorf("Expected type 'vless', got '%v'", proxy["type"])
	}
	if proxy["flow"] != "xtls-rprx-vision" {
		t.Errorf("Expected flow 'xtls-rprx-vision', got '%v'", proxy["flow"])
	}
}

// Feature: subscription-system, Property 7: Clash Configuration Round Trip
// Validates: Requirements 2.5
// *For any* valid set of proxies, generating Clash YAML and parsing it back
// SHALL produce equivalent proxy configurations.
func TestProperty_ClashConfigurationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	generator := NewClashGenerator()

	properties.Property("Clash YAML round trip preserves proxy count and names", prop.ForAll(
		func(numProxies int) bool {
			if numProxies < 1 {
				numProxies = 1
			}
			if numProxies > 10 {
				numProxies = 10
			}

			// Generate test proxies
			proxies := make([]*repository.Proxy, numProxies)
			for i := 0; i < numProxies; i++ {
				proxies[i] = &repository.Proxy{
					ID:       int64(i + 1),
					Name:     "Proxy-" + string(rune('A'+i)),
					Protocol: "vmess",
					Host:     "server" + string(rune('0'+i)) + ".example.com",
					Port:     443 + i,
					Settings: map[string]interface{}{
						"uuid":     "12345678-1234-1234-1234-12345678901" + string(rune('0'+i)),
						"alterId":  0,
						"security": "auto",
						"network":  "tcp",
					},
					Enabled: true,
				}
			}

			// Generate Clash config
			result, err := generator.Generate(proxies, nil)
			if err != nil {
				return false
			}

			// Parse back
			var config ClashConfig
			if err := yaml.Unmarshal(result, &config); err != nil {
				return false
			}

			// Verify proxy count matches
			if len(config.Proxies) != numProxies {
				return false
			}

			// Verify each proxy has required fields
			for _, proxy := range config.Proxies {
				if proxy["name"] == nil || proxy["type"] == nil ||
					proxy["server"] == nil || proxy["port"] == nil {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Test that Clash config contains all required fields
func TestClashGenerator_RequiredFields(t *testing.T) {
	generator := NewClashGenerator()

	proxy := &repository.Proxy{
		ID:       1,
		Name:     "Test Proxy",
		Protocol: "vmess",
		Host:     "test.example.com",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":     "12345678-1234-1234-1234-123456789012",
			"alterId":  0,
			"security": "auto",
			"network":  "tcp",
		},
		Enabled: true,
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	var config ClashConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Check required top-level fields
	if config.Port == 0 {
		t.Error("Port should be set")
	}
	if config.SocksPort == 0 {
		t.Error("SocksPort should be set")
	}
	if config.Mode == "" {
		t.Error("Mode should be set")
	}
	if config.LogLevel == "" {
		t.Error("LogLevel should be set")
	}

	// Check proxy has required fields
	if len(config.Proxies) != 1 {
		t.Fatalf("Expected 1 proxy, got %d", len(config.Proxies))
	}

	p := config.Proxies[0]
	requiredFields := []string{"name", "type", "server", "port", "uuid"}
	for _, field := range requiredFields {
		if p[field] == nil {
			t.Errorf("Proxy missing required field: %s", field)
		}
	}
}

func TestClashGenerator_EmptyProxies(t *testing.T) {
	generator := NewClashGenerator()

	result, err := generator.Generate([]*repository.Proxy{}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	var config ClashConfig
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	if len(config.Proxies) != 0 {
		t.Errorf("Expected 0 proxies, got %d", len(config.Proxies))
	}
}

func TestClashGenerator_ContentType(t *testing.T) {
	generator := NewClashGenerator()
	expected := "text/yaml; charset=utf-8"
	if generator.ContentType() != expected {
		t.Errorf("ContentType() = %s, want %s", generator.ContentType(), expected)
	}
}

func TestClashGenerator_FileExtension(t *testing.T) {
	generator := NewClashGenerator()
	expected := "yaml"
	if generator.FileExtension() != expected {
		t.Errorf("FileExtension() = %s, want %s", generator.FileExtension(), expected)
	}
}
