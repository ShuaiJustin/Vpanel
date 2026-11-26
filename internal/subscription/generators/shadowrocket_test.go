package generators

import (
	"encoding/base64"
	"strings"
	"testing"

	"v/internal/database/repository"
)

func TestShadowrocketGenerator_GenerateVLESSLinkIncludesEncryptionNone(t *testing.T) {
	generator := NewShadowrocketGenerator()

	proxy := &repository.Proxy{
		Name:     "Test VLESS",
		Protocol: "vless",
		Host:     "64.176.54.36",
		Port:     443,
		Settings: map[string]interface{}{
			"uuid":        "12345678-1234-1234-1234-123456789012",
			"security":    "tls",
			"server":      "64.176.54.36",
			"server_name": "vpn.example.com",
		},
	}

	result, err := generator.Generate([]*repository.Proxy{proxy}, nil)
	if err != nil {
		t.Fatalf("Failed to generate: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	link := string(decoded)
	if !strings.HasPrefix(link, "vless://") {
		t.Fatalf("expected vless:// prefix, got %s", link)
	}
	if !strings.Contains(link, "@64.176.54.36:443") || !strings.Contains(link, "encryption=none") || !strings.Contains(link, "sni=vpn.example.com") {
		t.Fatalf("unexpected shadowrocket vless link: %s", link)
	}
}
