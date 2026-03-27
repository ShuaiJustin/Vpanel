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
