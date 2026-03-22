package generators

import (
	"encoding/base64"
	"strings"
	"testing"

	"v/internal/database/repository"
)

func TestV2rayNGenerator_Generate(t *testing.T) {
	generator := NewV2rayNGenerator()

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
			Name:     "VLESS Server",
			Protocol: "vless",
			Host:     "vless.example.com",
			Port:     443,
			Settings: map[string]interface{}{
				"uuid":     "12345678-1234-1234-1234-123456789012",
				"network":  "tcp",
				"security": "tls",
				"sni":      "vless.example.com",
			},
			Enabled: true,
		},
		{
			ID:       3,
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
		{
			ID:       4,
			Name:     "SS Server",
			Protocol: "shadowsocks",
			Host:     "ss.example.com",
			Port:     8388,
			Settings: map[string]interface{}{
				"method":   "aes-256-gcm",
				"password": "sspassword",
			},
			Enabled: true,
		},
	}

	result, err := generator.Generate(proxies, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Decode base64 result
	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	content := string(decoded)
	lines := strings.Split(content, "\n")

	if len(lines) != 4 {
		t.Errorf("Expected 4 links, got %d", len(lines))
	}

	// Check each link starts with correct protocol
	expectedPrefixes := []string{"vmess://", "vless://", "trojan://", "ss://"}
	for i, line := range lines {
		if !strings.HasPrefix(line, expectedPrefixes[i]) {
			t.Errorf("Line %d: expected prefix %s, got %s", i, expectedPrefixes[i], line[:10])
		}
	}
}

func TestV2rayNGenerator_GenerateVMessLink(t *testing.T) {
	generator := NewV2rayNGenerator()

	proxy := &repository.Proxy{
		Name:     "Test VMess",
		Protocol: "vmess",
		Host:     "test.example.com",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":     "12345678-1234-1234-1234-123456789012",
			"alterId":  0,
			"security": "auto",
			"network":  "ws",
			"path":     "/ws",
			"host":     "test.example.com",
			"tls":      true,
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	link := string(decoded)
	if !strings.HasPrefix(link, "vmess://") {
		t.Errorf("Expected vmess:// prefix, got: %s", link[:20])
	}

	// Decode the vmess link content
	vmessContent := strings.TrimPrefix(link, "vmess://")
	vmessJSON, err := base64.StdEncoding.DecodeString(vmessContent)
	if err != nil {
		t.Fatalf("Failed to decode vmess content: %v", err)
	}

	// Check that JSON contains expected fields
	jsonStr := string(vmessJSON)
	if !strings.Contains(jsonStr, "12345678-1234-1234-1234-123456789012") {
		t.Error("VMess link should contain UUID")
	}
	if !strings.Contains(jsonStr, "test.example.com") {
		t.Error("VMess link should contain server address")
	}
}

func TestV2rayNGenerator_GenerateVMessLinkWithTLSSecurityKeepsAutoCipher(t *testing.T) {
	generator := NewV2rayNGenerator()

	proxy := &repository.Proxy{
		Name:     "TLS VMess",
		Protocol: "vmess",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":        "12345678-1234-1234-1234-123456789012",
			"alterId":     0,
			"security":    "tls",
			"server_name": "vpn.example.com",
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	vmessContent := strings.TrimPrefix(string(decoded), "vmess://")
	vmessJSON, err := base64.StdEncoding.DecodeString(vmessContent)
	if err != nil {
		t.Fatalf("Failed to decode vmess content: %v", err)
	}

	jsonStr := string(vmessJSON)
	if !strings.Contains(jsonStr, `"scy":"auto"`) {
		t.Fatalf("expected auto vmess cipher, got %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"tls":"tls"`) {
		t.Fatalf("expected tls enabled, got %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"sni":"vpn.example.com"`) {
		t.Fatalf("expected sni vpn.example.com, got %s", jsonStr)
	}
}


func TestV2rayNGenerator_GenerateVLESSLink(t *testing.T) {
	generator := NewV2rayNGenerator()

	proxy := &repository.Proxy{
		Name:     "Test VLESS",
		Protocol: "vless",
		Host:     "vless.example.com",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":     "12345678-1234-1234-1234-123456789012",
			"network":  "tcp",
			"security": "tls",
			"sni":      "vless.example.com",
			"flow":     "xtls-rprx-vision",
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	link := string(decoded)
	if !strings.HasPrefix(link, "vless://") {
		t.Errorf("Expected vless:// prefix, got: %s", link[:20])
	}

	// Check link contains expected parts
	if !strings.Contains(link, "12345678-1234-1234-1234-123456789012") {
		t.Error("VLESS link should contain UUID")
	}
	if !strings.Contains(link, "vless.example.com:443") {
		t.Error("VLESS link should contain server:port")
	}
	if !strings.Contains(link, "flow=xtls-rprx-vision") {
		t.Error("VLESS link should contain flow parameter")
	}
}

func TestV2rayNGenerator_GenerateTrojanLink(t *testing.T) {
	generator := NewV2rayNGenerator()

	proxy := &repository.Proxy{
		Name:     "Test Trojan",
		Protocol: "trojan",
		Host:     "trojan.example.com",
		Port:     443,
		Settings: map[string]interface{}{
			"password": "mypassword123",
			"sni":      "trojan.example.com",
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	link := string(decoded)
	if !strings.HasPrefix(link, "trojan://") {
		t.Errorf("Expected trojan:// prefix, got: %s", link[:20])
	}

	// Check link contains expected parts
	if !strings.Contains(link, "mypassword123") {
		t.Error("Trojan link should contain password")
	}
	if !strings.Contains(link, "trojan.example.com:443") {
		t.Error("Trojan link should contain server:port")
	}
}

func TestV2rayNGenerator_GenerateShadowsocksLink(t *testing.T) {
	generator := NewV2rayNGenerator()

	proxy := &repository.Proxy{
		Name:     "Test SS",
		Protocol: "shadowsocks",
		Host:     "ss.example.com",
		Port:     8388,
		Settings: map[string]interface{}{
			"method":   "aes-256-gcm",
			"password": "sspassword",
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	link := string(decoded)
	if !strings.HasPrefix(link, "ss://") {
		t.Errorf("Expected ss:// prefix, got: %s", link[:10])
	}

	// Check link contains expected parts
	if !strings.Contains(link, "ss.example.com:8388") {
		t.Error("SS link should contain server:port")
	}
}

func TestV2rayNGenerator_SupportsProtocol(t *testing.T) {
	generator := NewV2rayNGenerator()

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
		{"unknown", false},
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

func TestV2rayNGenerator_ContentType(t *testing.T) {
	generator := NewV2rayNGenerator()
	expected := "text/plain; charset=utf-8"
	if generator.ContentType() != expected {
		t.Errorf("ContentType() = %s, want %s", generator.ContentType(), expected)
	}
}

func TestV2rayNGenerator_FileExtension(t *testing.T) {
	generator := NewV2rayNGenerator()
	expected := "txt"
	if generator.FileExtension() != expected {
		t.Errorf("FileExtension() = %s, want %s", generator.FileExtension(), expected)
	}
}

func TestV2rayNGenerator_EmptyProxies(t *testing.T) {
	generator := NewV2rayNGenerator()

	result, err := generator.Generate([]*repository.Proxy{}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	// Empty content should still be valid base64
	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	if string(decoded) != "" {
		t.Errorf("Expected empty content, got: %s", string(decoded))
	}
}
