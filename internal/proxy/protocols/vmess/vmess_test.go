package vmess

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"v/internal/proxy"
)

func TestGenerateLink_UsesTLSDomainAsServerAddress(t *testing.T) {
	protocol := New()
	settings := &proxy.Settings{
		Name: "test",
		Port: 443,
		Settings: map[string]any{
			"uuid":        "12345678-1234-1234-1234-123456789012",
			"security":    "tls",
			"server_name": "vpn.example.com",
			"tls_domain":  "vpn.example.com",
		},
	}

	link, err := protocol.GenerateLink(settings)
	if err != nil {
		t.Fatalf("GenerateLink returned error: %v", err)
	}

	encoded := link[len("vmess://"):]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("failed to decode vmess link: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("failed to parse vmess payload: %v", err)
	}

	if payload["add"] != "vpn.example.com" {
		t.Fatalf("expected add to be vpn.example.com, got %v", payload["add"])
	}
	if payload["tls"] != "tls" {
		t.Fatalf("expected tls to be tls, got %v", payload["tls"])
	}
}
