package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"v/internal/logger"
)

func TestTrafficReporterResolveAPIPortFallsBackToRunningProcessConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "xray-running.json")
	if err := os.WriteFile(configPath, []byte(`{
		"api": {"tag": "api"},
		"inbounds": [{"tag": "api", "port": 43123}]
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reporter := &trafficReporter{
		configPath: "/missing/config.json",
		logger:     logger.NewNopLogger(),
		readFile:   os.ReadFile,
		processList: func() (string, error) {
			return " 101 xray /usr/local/bin/xray run -config " + configPath + "\n", nil
		},
		lastCommitted: make(map[string]int64),
	}

	resolution, err := reporter.resolveAPIPort()
	if err != nil {
		t.Fatalf("resolveAPIPort returned error: %v", err)
	}
	if resolution == nil || resolution.APIPort != 43123 {
		t.Fatalf("expected port 43123, got %+v", resolution)
	}
}

func TestTrafficReporterResolveAPIPortFallsBackToCanonicalPaths(t *testing.T) {
	configs := map[string][]byte{
		"/etc/xray/config.json": []byte(`{
			"api": {"tag": "stats-api"},
			"inbounds": [{"tag": "stats-api", "port": 52001}]
		}`),
	}

	reporter := &trafficReporter{
		configPath: "/missing/config.json",
		logger:     logger.NewNopLogger(),
		readFile: func(path string) ([]byte, error) {
			data, ok := configs[path]
			if !ok {
				return nil, os.ErrNotExist
			}
			return data, nil
		},
		processList: func() (string, error) {
			return "", errors.New("ps unavailable")
		},
		lastCommitted: make(map[string]int64),
	}

	resolution, err := reporter.resolveAPIPort()
	if err != nil {
		t.Fatalf("resolveAPIPort returned error: %v", err)
	}
	if resolution == nil || resolution.APIPort != 52001 {
		t.Fatalf("expected port 52001, got %+v", resolution)
	}
}

func TestExtractXrayConfigPathFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args string
		want string
	}{
		{
			name: "long flag",
			args: "/usr/local/bin/xray run -config /etc/xray/config.json",
			want: "/etc/xray/config.json",
		},
		{
			name: "short flag",
			args: "/usr/local/bin/xray run -c /usr/local/etc/xray/config.json",
			want: "/usr/local/etc/xray/config.json",
		},
		{
			name: "equals syntax",
			args: "/usr/local/bin/xray run -config=/opt/xray/config.json",
			want: "/opt/xray/config.json",
		},
	}

	for _, tt := range tests {
		if got := extractXrayConfigPathFromArgs(tt.args); got != tt.want {
			t.Fatalf("%s: expected %q, got %q", tt.name, tt.want, got)
		}
	}
}

func TestTrafficReporterPrepareDeltaMarksHealthyIdleWithoutRecords(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "xray.json")
	if err := os.WriteFile(configPath, []byte(`{
		"api": {"tag": "api"},
		"inbounds": [{"tag": "api", "port": 43123}]
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	reporter := &trafficReporter{
		binaryPath: "/bin/echo",
		configPath: configPath,
		logger:     logger.NewNopLogger(),
		readFile:   os.ReadFile,
		processList: func() (string, error) {
			return "", nil
		},
		lastCommitted: make(map[string]int64),
		status: &TrafficCollectorStatus{
			Status:               TrafficCollectorStatusUnknown,
			ConfiguredConfigPath: configPath,
		},
	}

	originalExecCommandContext := execCommandContext
	execCommandContext = func(ctx context.Context, name string, arg ...string) combinedOutputRunner {
		return combinedOutputFunc(func() ([]byte, error) {
			return []byte(`{"stat":[]}`), nil
		})
	}
	defer func() {
		execCommandContext = originalExecCommandContext
	}()

	snapshot, records, err := reporter.PrepareDelta(context.Background())
	if err != nil {
		t.Fatalf("PrepareDelta returned error: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if len(records) != 0 {
		t.Fatalf("expected no traffic records, got %+v", records)
	}

	status := reporter.GetCollectorStatus()
	if status == nil {
		t.Fatal("expected collector status")
	}
	if status.Status != TrafficCollectorStatusHealthyIdle {
		t.Fatalf("expected healthy idle status, got %q", status.Status)
	}
	if status.LastSuccessAt == nil || status.LastCollectionAt == nil {
		t.Fatalf("expected success timestamps, got %+v", status)
	}
}

func TestTrafficReporterPrepareDeltaRecordsCollectionError(t *testing.T) {
	reporter := &trafficReporter{
		binaryPath: "/usr/local/bin/xray",
		configPath: "/missing/config.json",
		logger:     logger.NewNopLogger(),
		readFile: func(path string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
		processList: func() (string, error) {
			return "", nil
		},
		lastCommitted: make(map[string]int64),
		status: &TrafficCollectorStatus{
			Status:               TrafficCollectorStatusUnknown,
			ConfiguredConfigPath: "/missing/config.json",
		},
	}

	_, _, err := reporter.PrepareDelta(context.Background())
	if err == nil {
		t.Fatal("expected PrepareDelta to fail")
	}

	status := reporter.GetCollectorStatus()
	if status == nil {
		t.Fatal("expected collector status")
	}
	if status.Status != TrafficCollectorStatusCollectorError {
		t.Fatalf("expected collector error status, got %q", status.Status)
	}
	if status.LastError == "" {
		t.Fatal("expected last error to be recorded")
	}
	if status.LastErrorAt == nil || status.LastCollectionAt == nil {
		t.Fatalf("expected error timestamps, got %+v", status)
	}
}
