package shadowsocks

import (
	"strings"
	"testing"
)

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
