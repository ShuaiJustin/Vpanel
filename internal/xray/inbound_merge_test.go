package xray

import (
	"fmt"
	"testing"

	"v/internal/database/repository"
)

func testVLESSInbound(proxyID int64, port int, realityKey, uuid string) InboundConfig {
	return InboundConfig{
		Tag:      fmt.Sprintf("inbound-%d", proxyID),
		Port:     port,
		Protocol: "vless",
		Settings: map[string]any{
			"clients": []map[string]any{
				{
					"id":    uuid,
					"email": fmt.Sprintf("user-%d-proxy-%d", proxyID, proxyID),
					"level": 0,
					"flow":  "xtls-rprx-vision",
				},
			},
			"decryption": "none",
			"fallbacks":  []map[string]any{},
		},
		StreamSettings: &StreamSettings{
			Network:  "tcp",
			Security: "reality",
			RealitySettings: map[string]any{
				"privateKey": realityKey,
				"serverName": "www.cloudflare.com",
			},
		},
		Sniffing: &SniffingConfig{
			Enabled:      true,
			DestOverride: []string{"http", "tls"},
		},
	}
}

func testCandidates(inbounds ...InboundConfig) []inboundCandidate {
	out := make([]inboundCandidate, 0, len(inbounds))
	for i := range inbounds {
		out = append(out, inboundCandidate{
			proxy:   &repository.Proxy{ID: int64(i + 1)},
			inbound: inbounds[i],
		})
	}
	return out
}

func TestMergeInboundCandidates_IdenticalSharedFields(t *testing.T) {
	g := &ConfigGenerator{}
	in1 := testVLESSInbound(1, 443, "same-key", "uuid-1")
	in2 := testVLESSInbound(2, 443, "same-key", "uuid-2")
	in3 := testVLESSInbound(3, 443, "same-key", "uuid-3")

	merged := g.mergeInboundCandidates(testCandidates(in1, in2, in3))

	if len(merged) != 1 {
		t.Fatalf("expected 1 merged inbound, got %d", len(merged))
	}
	clients := extractClients(merged[0].Settings)
	if len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %d", len(clients))
	}
	for i, want := range []string{
		"user-1-proxy-1", "user-2-proxy-2", "user-3-proxy-3",
	} {
		if got, _ := clients[i]["email"].(string); got != want {
			t.Errorf("client %d email = %q, want %q", i, got, want)
		}
	}
	if got, _ := clients[2]["id"].(string); got != "uuid-3" {
		t.Errorf("third client credential lost: %q", got)
	}
	if got, _ := clients[2]["flow"].(string); got != "xtls-rprx-vision" {
		t.Errorf("per-client flow lost: %q", got)
	}
	if merged[0].Port != 443 || merged[0].Protocol != "vless" {
		t.Errorf("shared fields changed: port=%d protocol=%s", merged[0].Port, merged[0].Protocol)
	}
	if got, _ := merged[0].Settings["decryption"].(string); got != "none" {
		t.Errorf("shared settings lost: decryption=%q", got)
	}
	if merged[0].StreamSettings == nil || merged[0].StreamSettings.Security != "reality" {
		t.Error("shared stream settings lost")
	}
	if merged[0].Sniffing == nil || !merged[0].Sniffing.Enabled {
		t.Error("shared sniffing lost")
	}
	if MergedInboundCount(merged) != 1 || ClientCount(merged) != 3 {
		t.Errorf("preview counters wrong: merged=%d clients=%d",
			MergedInboundCount(merged), ClientCount(merged))
	}
	if err := ValidateMergedInbounds(merged); err != nil {
		t.Errorf("merged config failed validation: %v", err)
	}
}

func TestMergeInboundCandidates_DifferentPortsAreNotMerged(t *testing.T) {
	g := &ConfigGenerator{}
	merged := g.mergeInboundCandidates(testCandidates(
		testVLESSInbound(1, 20001, "same-key", "uuid-1"),
		testVLESSInbound(2, 20039, "same-key", "uuid-2"),
		testVLESSInbound(3, 20042, "same-key", "uuid-3"),
	))

	if len(merged) != 3 {
		t.Fatalf("ports differ: expected 3 inbounds, got %d", len(merged))
	}
	for i, inbound := range merged {
		if inbound.Tag != fmt.Sprintf("inbound-%d", i+1) {
			t.Errorf("legacy tag changed for heterogeneous group: %s", inbound.Tag)
		}
	}
}

func TestMergeInboundCandidates_DifferentRealityKeysAreNotMerged(t *testing.T) {
	g := &ConfigGenerator{}
	merged := g.mergeInboundCandidates(testCandidates(
		testVLESSInbound(1, 443, "key-a", "uuid-1"),
		testVLESSInbound(2, 443, "key-b", "uuid-2"),
	))
	if len(merged) != 2 {
		t.Fatalf("Reality keys differ: expected 2 inbounds, got %d", len(merged))
	}
}

func TestMergeInboundCandidates_DifferentProtocolsAreNotMerged(t *testing.T) {
	g := &ConfigGenerator{}
	vmess := testVLESSInbound(1, 443, "same-key", "uuid-1")
	vmess.Protocol = "vmess"
	vmess.Settings = map[string]any{
		"clients": []map[string]any{{"id": "uuid-1", "email": "user-1-proxy-1", "alterId": 0}},
	}
	merged := g.mergeInboundCandidates(testCandidates(
		testVLESSInbound(2, 443, "same-key", "uuid-2"), vmess,
	))
	if len(merged) != 2 {
		t.Fatalf("protocols differ: expected 2 inbounds, got %d", len(merged))
	}
}

func TestMergeInboundCandidates_ShadowsocksPassesThrough(t *testing.T) {
	g := &ConfigGenerator{}
	ss := InboundConfig{
		Tag:      "inbound-1",
		Port:     8388,
		Protocol: "shadowsocks",
		Settings: map[string]any{
			"method":   "aes-256-gcm",
			"password": "secret",
			"email":    "user-1-proxy-1",
		},
	}
	merged := g.mergeInboundCandidates(testCandidates(ss))
	if len(merged) != 1 || merged[0].Tag != "inbound-1" {
		t.Fatalf("shadowsocks must stay a standalone inbound: %+v", merged)
	}
}

func TestMergeInboundCandidates_DeduplicatesClients(t *testing.T) {
	g := &ConfigGenerator{}
	merged := g.mergeInboundCandidates(testCandidates(
		testVLESSInbound(1, 443, "same-key", "same-uuid"),
		testVLESSInbound(2, 443, "same-key", "same-uuid"),
	))
	if len(merged) != 1 {
		t.Fatalf("expected 1 merged inbound, got %d", len(merged))
	}
	// Xray rejects duplicated credentials. Dedup is keyed on the credential
	// (id/password), not on the email: the two proxies have different emails
	// but the same UUID, so only the first client survives.
	clients := extractClients(merged[0].Settings)
	if len(clients) != 1 {
		t.Fatalf("expected duplicated UUID to be dropped, got %d clients", len(clients))
	}
	if got, _ := clients[0]["email"].(string); got != "user-1-proxy-1" {
		t.Errorf("wrong client kept after dedup: %q", got)
	}
}

func TestValidateMergedInbounds_DuplicateEmail(t *testing.T) {
	inbound := testVLESSInbound(1, 443, "same-key", "uuid-1")
	clients := extractClients(inbound.Settings)
	duplicate := map[string]any{}
	for k, v := range clients[0] {
		duplicate[k] = v
	}
	inbound.Settings["clients"] = append(clients, duplicate)

	if err := ValidateMergedInbounds([]InboundConfig{inbound}); err == nil {
		t.Fatal("expected duplicate email to be rejected")
	}
}

func TestConfigGenerator_MergingDisabledByDefault(t *testing.T) {
	g := NewConfigGenerator(nil, nil, nil, nil)
	if g.InboundMergingEnabled() {
		t.Fatal("merging must be off by default for backwards compatibility")
	}
	if !g.WithInboundMerging(true).InboundMergingEnabled() {
		t.Fatal("WithInboundMerging(true) did not enable the merge path")
	}
}
