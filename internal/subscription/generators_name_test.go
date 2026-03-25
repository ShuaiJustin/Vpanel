package subscription

import (
	"strings"
	"testing"

	"v/internal/database/repository"
)

func TestExtractProxyInfo_AutoProvisionedUsesReadableName(t *testing.T) {
	proxyModel := &repository.Proxy{
		Name:     "node-64-176-54-36-103738-vmess",
		Remark:   "auto provisioned",
		Protocol: "vmess",
		Host:     "64.176.54.36",
		Port:     20001,
		Settings: map[string]any{},
	}

	name, server, port, _ := extractProxyInfo(proxyModel)
	if name != "VMess · 64.176.54.36:20001" {
		t.Fatalf("unexpected proxy name: %q", name)
	}
	if server != "64.176.54.36" {
		t.Fatalf("unexpected server: %q", server)
	}
	if port != 20001 {
		t.Fatalf("unexpected port: %d", port)
	}
}

func TestExtractProxyInfo_CustomRemarkStillWins(t *testing.T) {
	proxyModel := &repository.Proxy{
		Name:     "node-64-176-54-36-103738-vmess",
		Remark:   "日本 01",
		Protocol: "vmess",
		Host:     "64.176.54.36",
		Port:     20001,
		Settings: map[string]any{},
	}

	name, _, _, _ := extractProxyInfo(proxyModel)
	if strings.TrimSpace(name) != "日本 01" {
		t.Fatalf("expected custom remark to win, got %q", name)
	}
}
