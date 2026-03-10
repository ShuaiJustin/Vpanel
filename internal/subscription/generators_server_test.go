package subscription

import (
	"encoding/base64"
	"strings"
	"testing"

	"v/internal/database/repository"
)

func TestGenerateV2rayN_UsesTLSDomainWhenHostMissing(t *testing.T) {
	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess TLS",
			Protocol: "vmess",
			Port:     443,
			Settings: map[string]any{
				"uuid":        "12345678-1234-1234-1234-123456789012",
				"security":    "tls",
				"server_name": "vpn.example.com",
				"tls_domain":  "vpn.example.com",
			},
			Enabled: true,
		},
	}

	result, err := generateV2rayN(proxies, nil)
	if err != nil {
		t.Fatalf("generateV2rayN returned error: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(string(result))
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(string(decoded), "vmess://"))
	if err != nil {
		t.Fatalf("failed to decode vmess payload: %v", err)
	}

	if !strings.Contains(string(payload), "vpn.example.com") {
		t.Fatalf("expected vmess payload to contain vpn.example.com, got %s", string(payload))
	}
	if !strings.Contains(string(payload), `"tls":"tls"`) {
		t.Fatalf("expected vmess payload to mark tls, got %s", string(payload))
	}
}
