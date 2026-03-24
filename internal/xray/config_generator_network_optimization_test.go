package xray

import (
	"reflect"
	"testing"

	"v/internal/node"
)

func TestBuildStreamSockoptReturnsExpectedTCPOptions(t *testing.T) {
	got := buildStreamSockopt("tcp", node.NetworkOptimizationSettings{
		EnableXraySockopt: true,
		XrayTCPFastOpen:   true,
		XrayTCPCongestion: "bbr",
	})

	want := map[string]any{
		"tcpFastOpen":   true,
		"tcpcongestion": "bbr",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected sockopt: got %#v want %#v", got, want)
	}
}

func TestBuildStreamSockoptSkipsUnsupportedNetworks(t *testing.T) {
	got := buildStreamSockopt("quic", node.NetworkOptimizationSettings{
		EnableXraySockopt: true,
		XrayTCPFastOpen:   true,
		XrayTCPCongestion: "bbr",
	})

	if got != nil {
		t.Fatalf("expected nil sockopt for quic, got %#v", got)
	}
}

func TestBuildStreamSockoptReturnsNilWhenFeatureDisabled(t *testing.T) {
	got := buildStreamSockopt("tcp", node.NetworkOptimizationSettings{
		EnableXraySockopt: false,
		XrayTCPFastOpen:   true,
		XrayTCPCongestion: "bbr",
	})

	if got != nil {
		t.Fatalf("expected nil sockopt when disabled, got %#v", got)
	}
}
