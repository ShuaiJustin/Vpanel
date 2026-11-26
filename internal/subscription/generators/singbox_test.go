package generators

import (
	"encoding/json"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database/repository"
)

func TestSingboxGenerator_Generate(t *testing.T) {
	generator := NewSingboxGenerator()

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

	// Parse JSON to verify structure
	var config SingboxConfig
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(config.Outbounds) != 2 {
		t.Errorf("Expected 2 outbounds, got %d", len(config.Outbounds))
	}
}

func TestSingboxGenerator_SupportsProtocol(t *testing.T) {
	generator := NewSingboxGenerator()

	tests := []struct {
		protocol string
		expected bool
	}{
		{"vmess", true},
		{"vless", true},
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

func TestSingboxGenerator_ContentType(t *testing.T) {
	generator := NewSingboxGenerator()
	expected := "application/json; charset=utf-8"
	if generator.ContentType() != expected {
		t.Errorf("ContentType() = %s, want %s", generator.ContentType(), expected)
	}
}

func TestSingboxGenerator_FileExtension(t *testing.T) {
	generator := NewSingboxGenerator()
	expected := "json"
	if generator.FileExtension() != expected {
		t.Errorf("FileExtension() = %s, want %s", generator.FileExtension(), expected)
	}
}

// Feature: subscription-system, Property 8: Sing-box Configuration Round Trip
// Validates: Requirements 2.5
// *For any* valid set of proxies, generating Sing-box JSON and parsing it back
// SHALL produce equivalent proxy configurations.
func TestProperty_SingboxConfigurationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	generator := NewSingboxGenerator()

	properties.Property("Sing-box JSON round trip preserves outbound count and tags", prop.ForAll(
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

			// Generate Sing-box config
			result, err := generator.Generate(proxies, nil)
			if err != nil {
				return false
			}

			// Parse back
			var config SingboxConfig
			if err := json.Unmarshal(result, &config); err != nil {
				return false
			}

			// Verify outbound count matches
			if len(config.Outbounds) != numProxies {
				return false
			}

			// Verify each outbound has required fields
			for _, outbound := range config.Outbounds {
				if outbound["tag"] == nil || outbound["type"] == nil ||
					outbound["server"] == nil || outbound["server_port"] == nil {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

func TestSingboxGenerator_EmptyProxies(t *testing.T) {
	generator := NewSingboxGenerator()

	result, err := generator.Generate([]*repository.Proxy{}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	var config SingboxConfig
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(config.Outbounds) != 0 {
		t.Errorf("Expected 0 outbounds, got %d", len(config.Outbounds))
	}
}

func TestSingboxGenerator_RequiredFields(t *testing.T) {
	generator := NewSingboxGenerator()

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

	var config SingboxConfig
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if len(config.Outbounds) != 1 {
		t.Fatalf("Expected 1 outbound, got %d", len(config.Outbounds))
	}

	outbound := config.Outbounds[0]
	requiredFields := []string{"tag", "type", "server", "server_port", "uuid"}
	for _, field := range requiredFields {
		if outbound[field] == nil {
			t.Errorf("Outbound missing required field: %s", field)
		}
	}
}
