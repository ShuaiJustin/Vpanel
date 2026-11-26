package agent

import "testing"

func TestShouldUseSystemdXray(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/usr/local/etc/xray/config.json", want: true},
		{path: "/etc/xray/config.json", want: true},
		{path: "/tmp/vpanel-agent/config.json", want: false},
	}

	for _, tt := range tests {
		if got := shouldUseSystemdXray(tt.path); got != tt.want {
			t.Fatalf("shouldUseSystemdXray(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestFindConfiguredXrayPID(t *testing.T) {
	psOutput := `
  101 xray /usr/local/bin/xray run -config /usr/local/etc/xray/config.json
  202 xray /usr/local/bin/xray run -c /tmp/vpanel-agent-live/config.json
  303 mihomo /usr/local/bin/mihomo -d /root/.config/mihomo
`

	if got, want := findConfiguredXrayPID(psOutput, "/tmp/vpanel-agent-live/config.json"), 202; got != want {
		t.Fatalf("expected configured xray pid %d, got %d", want, got)
	}
	if got := findConfiguredXrayPID(psOutput, "/tmp/missing.json"); got != 0 {
		t.Fatalf("expected no configured xray pid, got %d", got)
	}
}

func TestParseXrayProcessLine(t *testing.T) {
	pid, args, ok := parseXrayProcessLine(" 202 xray /usr/local/bin/xray run -c /tmp/vpanel-agent-live/config.json ")
	if !ok {
		t.Fatal("expected parse to succeed")
	}
	if pid != 202 {
		t.Fatalf("expected pid 202, got %d", pid)
	}
	if args == "" {
		t.Fatal("expected args to be captured")
	}

	if _, _, ok := parseXrayProcessLine(" 303 mihomo /usr/local/bin/mihomo -d /tmp "); ok {
		t.Fatal("expected non-xray process to be rejected")
	}
}
