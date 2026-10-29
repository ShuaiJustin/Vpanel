package node

import (
	"fmt"
	"strings"
	"testing"
)

func TestNetworkOptimizationSettingsNormalizePreservesExplicitSockoptChoices(t *testing.T) {
	input := NetworkOptimizationSettings{EnableBBR: true, EnableTCPFastOpen: true, EnableXraySockopt: true}
	got := input.Normalize()
	if got.XrayTCPCongestion != "" || got.XrayTCPFastOpen {
		t.Fatalf("normalization must preserve disabled Xray TFO and unset congestion, got %+v", got)
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

func TestAdjustAppliedNetworkOptimizationSettingsSkipsBBR(t *testing.T) {
	input := NetworkOptimizationSettings{
		EnableBBR:         true,
		EnableFQ:          true,
		EnableTCPFastOpen: true,
	}

	got := adjustAppliedNetworkOptimizationSettings(input, "当前内核不支持 BBR，已跳过 BBR，继续应用其他优化项")
	if got.EnableBBR {
		t.Fatal("expected BBR to be disabled after skip log")
	}
	if !got.EnableFQ || !got.EnableTCPFastOpen {
		t.Fatalf("expected other settings to remain enabled, got %+v", got)
	}
}

func TestAdjustAppliedNetworkOptimizationSettingsKeepsBBRWithoutSkipLog(t *testing.T) {
	input := NetworkOptimizationSettings{EnableBBR: true, EnableFQ: true}
	got := adjustAppliedNetworkOptimizationSettings(input, "已应用节点网络优化")
	if !got.EnableBBR {
		t.Fatal("expected BBR to remain enabled without skip log")
	}
}

func TestApplyNetworkOptimizationScriptDegradesMissingBBR(t *testing.T) {
	script := fmt.Sprintf(applyNetworkOptimizationScriptTemplate, NetworkOptimizationBackupPath, networkOptimizationSysctlPath, 1, 1, 1)
	if !strings.Contains(script, "已跳过 BBR") {
		t.Fatal("expected apply script to degrade when BBR is unavailable")
	}
	if strings.Contains(script, "请升级内核或加载 tcp_bbr 模块") {
		t.Fatal("expected hard-fail BBR message to be removed")
	}
}

func TestAdjustAppliedNetworkOptimizationSettingsSkipsUnsupportedXrayBBR(t *testing.T) {
	got := adjustAppliedNetworkOptimizationSettings(RecommendedNetworkOptimizationSettings(), "当前内核不支持 BBR，已跳过 BBR，继续应用其他优化项")
	if got.XrayTCPCongestion != "" {
		t.Fatalf("unsupported bbr must not be sent to Xray: %+v", got)
	}
	if got.Normalize().XrayTCPCongestion != "" {
		t.Fatal("normalization must not restore unsupported bbr")
	}
}
