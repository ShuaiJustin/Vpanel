package handlers

import (
	"testing"

	"v/internal/database/repository"
)

func TestResolveProxyServerAddress_PrefersExplicitServer(t *testing.T) {
	proxyModel := &repository.Proxy{
		Host: "stale.example.com",
		Settings: map[string]any{
			"server": "edge.example.com",
		},
	}
	nodeModel := &repository.Node{Address: "node.example.com"}

	if got := resolveProxyServerAddress(proxyModel, nodeModel, proxyModel.Settings); got != "edge.example.com" {
		t.Fatalf("expected explicit server to win, got %q", got)
	}
}

func TestResolveProxyServerAddress_FallsBackToNodeAddress(t *testing.T) {
	proxyModel := &repository.Proxy{
		Host:     "stale.example.com",
		Settings: map[string]any{},
	}
	nodeModel := &repository.Node{Address: "node.example.com"}

	if got := resolveProxyServerAddress(proxyModel, nodeModel, proxyModel.Settings); got != "node.example.com" {
		t.Fatalf("expected node address fallback, got %q", got)
	}
}
