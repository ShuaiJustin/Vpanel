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
	if payload["scy"] != "auto" {
		t.Fatalf("expected scy to be auto, got %v", payload["scy"])
	}
	if _, ok := payload["port"].(string); !ok {
		t.Fatalf("expected port to be encoded as string, got %T", payload["port"])
	}
	if _, ok := payload["aid"].(string); !ok {
		t.Fatalf("expected aid to be encoded as string, got %T", payload["aid"])
	}
}

func TestGenerateLink_KeepsIPWhenSNIIsSet(t *testing.T) {
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

	if payload["add"] != "64.176.54.36" {
		t.Fatalf("expected add to keep server ip 64.176.54.36, got %v", payload["add"])
	}
	if payload["sni"] != "vpn.example.com" {
		t.Fatalf("expected sni vpn.example.com, got %v", payload["sni"])
	}
}

func TestParseLink_AcceptsStringPortAndAid(t *testing.T) {
	protocol := New()
	payload := map[string]any{
		"v":    "2",
		"ps":   "test",
		"add":  "vpn.example.com",
		"port": "443",
		"id":   "12345678-1234-1234-1234-123456789012",
		"aid":  "0",
		"scy":  "auto",
		"net":  "tcp",
		"type": "none",
		"tls":  "tls",
		"sni":  "vpn.example.com",
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	settings, err := protocol.ParseLink("vmess://" + base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatalf("ParseLink returned error: %v", err)
	}
	if settings.Port != 443 {
		t.Fatalf("expected port 443, got %d", settings.Port)
	}
	if settings.GetInt("alterId") != 0 {
		t.Fatalf("expected alterId 0, got %d", settings.GetInt("alterId"))
	}
	if settings.GetString("scy") != "auto" {
		t.Fatalf("expected scy auto, got %q", settings.GetString("scy"))
	}
}

func TestGenerateLink_UsesExternalPortWhenProvided(t *testing.T) {
	protocol := New()
	settings := &proxy.Settings{
		Name: "test",
		Host: "180.173.123.192",
		Port: 20004,
		Settings: map[string]any{
			"uuid":          "12345678-1234-1234-1234-123456789012",
			"network":       "ws",
			"path":          "/vpws-20004",
			"external_port": 80,
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

	if payload["port"] != "80" {
		t.Fatalf("expected external port 80, got %v", payload["port"])
	}
}

func TestGenerateConfig_IncludesWSSettings(t *testing.T) {
	protocol := New()
	settings := &proxy.Settings{
		ID:   24,
		Port: 20004,
		Settings: map[string]any{
			"uuid":    "12345678-1234-1234-1234-123456789012",
			"network": "ws",
			"path":    "/vpws-20004",
			"host":    "180.173.123.192",
		},
	}

	raw, err := protocol.GenerateConfig(settings)
	if err != nil {
		t.Fatalf("GenerateConfig returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed to parse vmess config: %v", err)
	}

	streamSettings, ok := payload["streamSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected streamSettings map, got %T", payload["streamSettings"])
	}
	if streamSettings["network"] != "ws" {
		t.Fatalf("expected ws network, got %v", streamSettings["network"])
	}
	wsSettings, ok := streamSettings["wsSettings"].(map[string]any)
	if !ok {
		t.Fatalf("expected wsSettings map, got %T", streamSettings["wsSettings"])
	}
	if wsSettings["path"] != "/vpws-20004" {
		t.Fatalf("expected ws path /vpws-20004, got %v", wsSettings["path"])
	}
}
