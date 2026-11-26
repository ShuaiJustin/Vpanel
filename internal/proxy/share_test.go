package proxy

import "testing"

func TestResolveServerAddress_KeepsExplicitIPWhenSNIIsSet(t *testing.T) {
	server := ResolveServerAddress("64.176.54.36", map[string]any{
		"server":      "64.176.54.36",
		"security":    "tls",
		"server_name": "vpn.example.com",
		"tls_domain":  "vpn.example.com",
	})

	if server != "64.176.54.36" {
		t.Fatalf("expected explicit ip to be preserved, got %q", server)
	}
}

func TestResolveServerAddress_FallsBackToSNIWhenAddressMissing(t *testing.T) {
	server := ResolveServerAddress("", map[string]any{
		"security":    "tls",
		"server_name": "vpn.example.com",
		"tls_domain":  "vpn.example.com",
	})

	if server != "vpn.example.com" {
		t.Fatalf("expected sni fallback vpn.example.com, got %q", server)
	}
}

func TestResolveSNI_NormalizesWildcardDomainForClientSNI(t *testing.T) {
	sni := ResolveSNI(map[string]any{
		"security":    "tls",
		"server_name": "*.example.com",
	})

	if sni != "www.example.com" {
		t.Fatalf("expected wildcard SNI to be normalized to www.example.com, got %q", sni)
	}
}

func TestResolveServerAddress_PrefersExplicitHostOverLegacyServerSetting(t *testing.T) {
	server := ResolveServerAddress("64.176.54.36", map[string]any{
		"server": "old.example.com",
	})

	if server != "64.176.54.36" {
		t.Fatalf("expected explicit host to win over legacy settings server, got %q", server)
	}
}

func TestResolveServerAddress_UsesExternalHostOverrideFirst(t *testing.T) {
	server := ResolveServerAddress("64.176.54.36", map[string]any{
		"external_host": "edge.example.com",
		"server":        "old.example.com",
	})

	if server != "edge.example.com" {
		t.Fatalf("expected external host override edge.example.com, got %q", server)
	}
}

func TestResolveServerPort_UsesExternalPortOverride(t *testing.T) {
	port := ResolveServerPort(20003, map[string]any{
		"external_port": "80",
	})

	if port != 80 {
		t.Fatalf("expected external port 80, got %d", port)
	}
}

func TestHasTLSSettings_RespectsExplicitSecurityNone(t *testing.T) {
	if HasTLSSettings(map[string]any{
		"security": "none",
		"tls":      true,
		"sni":      "vpn.example.com",
	}) {
		t.Fatal("expected explicit security=none to disable TLS inference")
	}
}

func TestHasTLSSettings_AcceptsLegacyTLSString(t *testing.T) {
	if !HasTLSSettings(map[string]any{"tls": "tls"}) {
		t.Fatal("expected legacy tls string to enable TLS inference")
	}
}
