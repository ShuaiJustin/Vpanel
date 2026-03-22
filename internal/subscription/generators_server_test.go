package subscription

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

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
	if !strings.Contains(string(payload), `"scy":"auto"`) {
		t.Fatalf("expected vmess payload to keep auto cipher, got %s", string(payload))
	}
}

func TestGenerateClash_UsesTLSForVMessWithoutCorruptingCipher(t *testing.T) {
	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess TLS",
			Protocol: "vmess",
			Port:     443,
			Settings: map[string]any{
				"uuid":          "12345678-1234-1234-1234-123456789012",
				"security":      "tls",
				"server_name":   "vpn.example.com",
				"allowInsecure": true,
				"fingerprint":   "chrome",
			},
			Enabled: true,
		},
	}

	result, err := generateClash(proxies, nil)
	if err != nil {
		t.Fatalf("generateClash returned error: %v", err)
	}

	var config map[string]any
	if err := yaml.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse clash yaml: %v", err)
	}

	proxiesValue, ok := config["proxies"].([]any)
	if !ok || len(proxiesValue) != 1 {
		t.Fatalf("expected one clash proxy, got %#v", config["proxies"])
	}

	proxy, ok := proxiesValue[0].(map[string]any)
	if !ok {
		t.Fatalf("expected proxy map, got %#v", proxiesValue[0])
	}

	if proxy["cipher"] != "auto" {
		t.Fatalf("expected vmess clash cipher auto, got %#v", proxy["cipher"])
	}
	if proxy["tls"] != true {
		t.Fatalf("expected tls enabled, got %#v", proxy["tls"])
	}
	if proxy["servername"] != "vpn.example.com" {
		t.Fatalf("expected servername vpn.example.com, got %#v", proxy["servername"])
	}
	if proxy["skip-cert-verify"] != true {
		t.Fatalf("expected skip-cert-verify true, got %#v", proxy["skip-cert-verify"])
	}
}

func TestGenerateSurge_UsesVMessCipherAndTLSSettings(t *testing.T) {
	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess TLS",
			Protocol: "vmess",
			Host:     "vpn.example.com",
			Port:     443,
			Settings: map[string]any{
				"uuid":          "12345678-1234-1234-1234-123456789012",
				"security":      "tls",
				"server_name":   "vpn.example.com",
				"allowInsecure": true,
			},
			Enabled: true,
		},
	}

	result, err := generateSurge(proxies, nil)
	if err != nil {
		t.Fatalf("generateSurge returned error: %v", err)
	}

	line := string(result)
	if !strings.Contains(line, "encrypt-method=auto") {
		t.Fatalf("expected auto vmess cipher, got %s", line)
	}
	if !strings.Contains(line, "tls=true") {
		t.Fatalf("expected tls enabled, got %s", line)
	}
	if !strings.Contains(line, "sni=vpn.example.com") {
		t.Fatalf("expected sni vpn.example.com, got %s", line)
	}
	if !strings.Contains(line, "skip-cert-verify=true") {
		t.Fatalf("expected skip-cert-verify=true, got %s", line)
	}
}

func TestGenerateQuantumultX_UsesVMessCipherAndTLSSettings(t *testing.T) {
	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess TLS",
			Protocol: "vmess",
			Host:     "vpn.example.com",
			Port:     443,
			Settings: map[string]any{
				"uuid":          "12345678-1234-1234-1234-123456789012",
				"security":      "tls",
				"server_name":   "vpn.example.com",
				"allowInsecure": true,
			},
			Enabled: true,
		},
	}

	result, err := generateQuantumultX(proxies, nil)
	if err != nil {
		t.Fatalf("generateQuantumultX returned error: %v", err)
	}

	line := string(result)
	if !strings.Contains(line, "method=auto") {
		t.Fatalf("expected auto vmess cipher, got %s", line)
	}
	if !strings.Contains(line, "obfs=over-tls") {
		t.Fatalf("expected over-tls obfs, got %s", line)
	}
	if !strings.Contains(line, "obfs-host=vpn.example.com") {
		t.Fatalf("expected obfs-host vpn.example.com, got %s", line)
	}
	if !strings.Contains(line, "tls-verification=false") {
		t.Fatalf("expected tls-verification=false, got %s", line)
	}
}

func TestGenerateSingbox_VMessesTLSSeparateFromCipher(t *testing.T) {
	proxies := []*repository.Proxy{
		{
			ID:       1,
			Name:     "VMess TLS",
			Protocol: "vmess",
			Host:     "vpn.example.com",
			Port:     443,
			Settings: map[string]any{
				"uuid":          "12345678-1234-1234-1234-123456789012",
				"security":      "tls",
				"server_name":   "vpn.example.com",
				"allowInsecure": true,
				"fingerprint":   "chrome",
			},
			Enabled: true,
		},
	}

	result, err := generateSingbox(proxies, nil)
	if err != nil {
		t.Fatalf("generateSingbox returned error: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(result, &config); err != nil {
		t.Fatalf("failed to parse sing-box json: %v", err)
	}

	outbounds, ok := config["outbounds"].([]any)
	if !ok || len(outbounds) != 1 {
		t.Fatalf("expected one outbound, got %#v", config["outbounds"])
	}

	outbound, ok := outbounds[0].(map[string]any)
	if !ok {
		t.Fatalf("expected outbound map, got %#v", outbounds[0])
	}

	if outbound["security"] != "auto" {
		t.Fatalf("expected vmess security auto, got %#v", outbound["security"])
	}

	tls, ok := outbound["tls"].(map[string]any)
	if !ok {
		t.Fatalf("expected tls object, got %#v", outbound["tls"])
	}
	if tls["server_name"] != "vpn.example.com" {
		t.Fatalf("expected tls server_name vpn.example.com, got %#v", tls["server_name"])
	}
	if tls["insecure"] != true {
		t.Fatalf("expected tls insecure true, got %#v", tls["insecure"])
	}
}
