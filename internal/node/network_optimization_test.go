package node

import "testing"

func TestNetworkOptimizationSettingsNormalizeDefaultsSockoptFields(t *testing.T) {
	input := NetworkOptimizationSettings{
		EnableBBR:         true,
		EnableTCPFastOpen: true,
		EnableXraySockopt: true,
	}

	got := input.Normalize()

	if got.XrayTCPCongestion != "bbr" {
		t.Fatalf("expected congestion control to default to bbr, got %q", got.XrayTCPCongestion)
	}
	if !got.XrayTCPFastOpen {
		t.Fatal("expected xray tcp fast open to be enabled when tcp fast open is enabled")
	}
}

func TestNetworkOptimizationSettingsNormalizeClearsSockoptChildrenWhenDisabled(t *testing.T) {
	input := NetworkOptimizationSettings{
		EnableXraySockopt: false,
		XrayTCPFastOpen:   true,
		XrayTCPCongestion: "bbr",
	}

	got := input.Normalize()

	if got.XrayTCPFastOpen {
		t.Fatal("expected xray tcp fast open to be cleared when sockopt is disabled")
	}
	if got.XrayTCPCongestion != "" {
		t.Fatalf("expected congestion control to be cleared, got %q", got.XrayTCPCongestion)
	}
}

func TestParseNetworkOptimizationSettingsReturnsZeroValueOnInvalidJSON(t *testing.T) {
	got := ParseNetworkOptimizationSettings("{invalid")

	if !got.IsEmpty() {
		t.Fatalf("expected invalid json to return empty settings, got %+v", got)
	}
}
