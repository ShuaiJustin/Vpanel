package generators

import (
	"testing"

	"v/internal/database/repository"
)

func TestExtractProxyInfo_UsesExternalPortOverride(t *testing.T) {
	proxyModel := &repository.Proxy{
		Name:     "上海-WS",
		Protocol: "vmess",
		Port:     20004,
		Host:     "180.173.123.192",
		Settings: map[string]any{
			"server":        "180.173.123.192",
			"external_port": 80,
		},
	}

	info := ExtractProxyInfo(proxyModel)
	if info.Port != 80 {
		t.Fatalf("expected external port 80, got %d", info.Port)
	}
}

func TestExtractProxyInfo_PrefersExternalHostOverride(t *testing.T) {
	proxyModel := &repository.Proxy{
		Name:     "大阪-TLS",
		Protocol: "vmess",
		Port:     20001,
		Host:     "64.176.54.36",
		Settings: map[string]any{
			"server":        "old.example.com",
			"external_host": "edge.example.com",
		},
	}

	info := ExtractProxyInfo(proxyModel)
	if info.Server != "edge.example.com" {
		t.Fatalf("expected external host edge.example.com, got %q", info.Server)
	}
	if proxyModel.Settings["server"] != "old.example.com" {
		t.Fatalf("expected original settings to remain unchanged, got %#v", proxyModel.Settings["server"])
	}
}

func TestMakeUniqueNames_UsesNumericSuffixesBeyondNine(t *testing.T) {
	proxies := make([]*ProxyInfo, 11)
	for i := range proxies {
		proxies[i] = &ProxyInfo{Name: "Shared"}
	}

	MakeUniqueNames(proxies)

	if proxies[9].Name != "Shared-10" {
		t.Fatalf("expected tenth duplicate suffix Shared-10, got %q", proxies[9].Name)
	}
	if proxies[10].Name != "Shared-11" {
		t.Fatalf("expected eleventh duplicate suffix Shared-11, got %q", proxies[10].Name)
	}
}
