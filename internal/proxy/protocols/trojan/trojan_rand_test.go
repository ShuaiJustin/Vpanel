package trojan

import (
	"strings"
	"testing"

	"v/internal/proxy"
)

func TestIPv6LinkRoundTrip(t *testing.T) {
	const host = "2001:db8::123"
	p := &Protocol{}
	link, err := p.GenerateLink(&proxy.Settings{Host: host, Port: 443, Settings: map[string]any{"password": "test-password"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "@["+host+"]:443") {
		t.Fatal("expected a bracketed IPv6 endpoint")
	}
	settings, err := p.ParseLink(link)
	if err != nil {
		t.Fatal(err)
	}
	if settings.Host != host || settings.Port != 443 {
		t.Fatalf("unexpected endpoint %s, %d", settings.Host, settings.Port)
	}
}

func TestGenerateRandomPasswordIsRandom(t *testing.T) {
	a := generateRandomPassword()
	b := generateRandomPassword()
	if a == "" || b == "" {
		t.Fatalf("empty password: %q %q", a, b)
	}
	if a == b {
		t.Fatalf("non-random passwords: %q == %q", a, b)
	}
	if len(a) != 32 || len(b) != 32 {
		t.Fatalf("unexpected length: %d %d", len(a), len(b))
	}
	if strings.HasPrefix(a, "abcdefghijklmnop") {
		t.Fatalf("password regressed to deterministic fallback: %q", a)
	}
}
