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

func TestResolveServerPort_UsesExternalPortOverride(t *testing.T) {
	port := ResolveServerPort(20003, map[string]any{
		"external_port": "80",
	})

	if port != 80 {
		t.Fatalf("expected external port 80, got %d", port)
	}
}
