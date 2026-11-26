package vless

import (
	"net/url"
	"strings"
	"testing"

	"v/internal/proxy"
)

func TestGenerateLink_IncludesEncryptionNoneAndKeepsIPWhenSNIIsSet(t *testing.T) {
	protocol := New()
	settings := &proxy.Settings{
		Name: "test",
		Host: "64.176.54.36",
		Port: 443,
		Settings: map[string]any{
			"uuid":        "12345678-1234-1234-1234-123456789012",
			"security":    "tls",
			"server":      "64.176.54.36",
			"server_name": "vpn.example.com",
		},
	}

	link, err := protocol.GenerateLink(settings)
	if err != nil {
		t.Fatalf("GenerateLink returned error: %v", err)
	}

	if !strings.HasPrefix(link, "vless://12345678-1234-1234-1234-123456789012@64.176.54.36:443") {
		t.Fatalf("expected link to keep server ip, got %s", link)
	}

	parsed, err := url.Parse(link)
	if err != nil {
		t.Fatalf("failed to parse vless link: %v", err)
	}

	query := parsed.Query()
	if query.Get("encryption") != "none" {
		t.Fatalf("expected encryption=none, got %q", query.Get("encryption"))
	}
	if query.Get("security") != "tls" {
		t.Fatalf("expected security=tls, got %q", query.Get("security"))
	}
	if query.Get("sni") != "vpn.example.com" {
		t.Fatalf("expected sni=vpn.example.com, got %q", query.Get("sni"))
	}
}
