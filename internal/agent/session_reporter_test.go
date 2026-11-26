package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"v/internal/logger"
)

func TestParseXrayAccessLogLine(t *testing.T) {
	line := "2026/04/10 12:34:56.123456 from tcp:198.51.100.23:43120 accepted tcp:example.com:443 [inbound-1] email: user-39-proxy-7"

	record, seenAt, ok := parseXrayAccessLogLine(line)
	if !ok {
		t.Fatal("expected line to parse")
	}
	if record.UserID != 39 || record.ProxyID != 7 {
		t.Fatalf("unexpected record identity: %+v", record)
	}
	if record.IP != "198.51.100.23" {
		t.Fatalf("unexpected parsed ip: %+v", record)
	}
	if record.DeviceInfo != "Proxy #7 connection" {
		t.Fatalf("unexpected device info: %+v", record)
	}
	if seenAt.IsZero() {
		t.Fatalf("expected parsed timestamp, got zero time")
	}
}

func TestSessionReporterCollectRecentSessions(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "access.log")
	configPath := filepath.Join(tempDir, "config.json")

	if err := os.WriteFile(configPath, []byte(`{
		"log": {
			"access": "`+logPath+`"
		}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	now := time.Now().UTC()
	logLines := []string{
		now.Add(-2*time.Minute).Local().Format("2006/01/02 15:04:05.000000") + " from tcp:198.51.100.23:43120 accepted tcp:example.com:443 [inbound-1] email: user-39-proxy-7",
		now.Add(-1*time.Minute).Local().Format("2006/01/02 15:04:05.000000") + " from tcp:198.51.100.24:43121 accepted tcp:example.com:443 [inbound-2] email: user-39-proxy-8",
		now.Add(-30*time.Second).Local().Format("2006/01/02 15:04:05.000000") + " from tcp:198.51.100.23:43120 accepted tcp:example.com:443 [inbound-1] email: user-39-proxy-7",
	}
	if err := os.WriteFile(logPath, []byte(logLines[0]+"\n"+logLines[1]+"\n"+logLines[2]+"\n"), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	reporter := newSessionReporter(XrayConfig{ConfigPath: configPath}, logger.NewNopLogger())
	sessions, err := reporter.CollectRecentSessions(context.Background())
	if err != nil {
		t.Fatalf("CollectRecentSessions returned error: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %+v", sessions)
	}
	if sessions[0].IP != "198.51.100.23" || sessions[0].ProxyID != 7 {
		t.Fatalf("expected latest session first, got %+v", sessions)
	}
	if sessions[1].IP != "198.51.100.24" || sessions[1].ProxyID != 8 {
		t.Fatalf("unexpected second session: %+v", sessions)
	}
}
