package subscription

import (
	"strings"
	"testing"

	"v/internal/database/repository"
	subgenerators "v/internal/subscription/generators"
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

	info := subgenerators.ExtractProxyInfo(proxyModel)
	if info.Name != "VMess · 64.176.54.36:20001" {
		t.Fatalf("unexpected proxy name: %q", info.Name)
	}
	if info.Server != "64.176.54.36" {
		t.Fatalf("unexpected server: %q", info.Server)
	}
	if info.Port != 20001 {
		t.Fatalf("unexpected port: %d", info.Port)
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

	info := subgenerators.ExtractProxyInfo(proxyModel)
	if strings.TrimSpace(info.Name) != "日本 01" {
		t.Fatalf("expected custom remark to win, got %q", info.Name)
	}
}
