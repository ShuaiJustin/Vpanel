package agent

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"v/internal/logger"
)

func TestApplyXrayConfigRestoresPreviousFileOnActivationFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	binary := filepath.Join(dir, "xray")
	old, next := []byte(`{"old":1}`), []byte(`{"new":1}`)
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	manager := &XrayManager{config: XrayManagerConfig{BinaryPath: binary, ConfigPath: path, BackupDir: dir}, logger: logger.NewNopLogger()}
	var activations int
	err := manager.applyConfig(t.Context(), next, func(_ context.Context, changed bool) error {
		activations++
		if !changed {
			t.Fatal("expected replacement")
		}
		if activations == 1 {
			return errors.New("failed to listen")
		}
		return nil
	})
	if err == nil || activations != 2 {
		t.Fatalf("expected failure followed by rollback, got %v after %d activations", err, activations)
	}
	saved, err := os.ReadFile(path)
	if err != nil || string(saved) != string(old) {
		t.Fatalf("expected old configuration after rollback, got %q, %v", saved, err)
	}
}

func TestReplaceAgentBinaryChecksTargetVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vpanel-agent")
	if err := os.WriteFile(path, []byte("original"), 0755); err != nil {
		t.Fatal(err)
	}
	newBinary := "#!/bin/sh\necho 'V Panel Node Agent new-release'\n"
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(newBinary)))
	if err := replaceAgentBinary(t.Context(), path, "new-release", strings.Repeat("0", 64), strings.NewReader(newBinary)); err == nil {
		t.Fatal("expected digest mismatch")
	}
	if err := replaceAgentBinary(t.Context(), path, "different-release", digest, strings.NewReader(newBinary)); err == nil {
		t.Fatal("expected version mismatch")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "original" {
		t.Fatalf("previous Agent changed: %q, %v", data, err)
	}
	if err := replaceAgentBinary(t.Context(), path, "new-release", digest, strings.NewReader(newBinary)); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != newBinary {
		t.Fatalf("updated Agent = %q, %v", data, err)
	}
}

func TestActivateXrayRelease(t *testing.T) {
	for _, scenario := range []struct {
		name             string
		downloaded       string
		rejectConfig     bool
		failRestart      bool
		expectedVersion  string
		expectedRestarts int
	}{
		{"switch", "26.11.9", false, false, "26.11.9", 1},
		{"wrong version", "26.11.8", false, false, "26.10.31", 0},
		{"incompatible config", "26.11.9", true, false, "26.10.31", 0},
		{"failed restart", "26.11.9", false, true, "26.10.31", 2},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			dir := t.TempDir()
			binary := filepath.Join(dir, "custom-xray")
			linked := filepath.Join(dir, "xray-link")
			if err := os.WriteFile(binary, []byte("#!/bin/sh\necho 'Xray 26.10.31'\n"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(binary, linked); err != nil {
				t.Fatal(err)
			}
			archivePath := filepath.Join(dir, "release.zip")
			file, err := os.Create(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			archive := zip.NewWriter(file)
			entry, err := archive.Create("xray")
			if err != nil {
				t.Fatal(err)
			}
			script := "#!/bin/sh\nif [ \"$1\" = version ]; then echo 'Xray " + scenario.downloaded + "'; exit 0; fi\n"
			if scenario.rejectConfig {
				script += "exit 1\n"
			}
			if _, err := entry.Write([]byte(script)); err != nil {
				t.Fatal(err)
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			restarts := 0
			version, err := activateXrayRelease(t.Context(), archivePath, "26.11.9", linked, "test-config", func(_ context.Context, target string) error {
				restarts++
				want := scenario.downloaded
				if restarts == 2 {
					want = "26.10.31"
				}
				if got := GetXrayVersion(target); got != "Xray "+want {
					t.Fatalf("restart version = %q, want %q", got, want)
				}
				if scenario.failRestart && restarts == 1 {
					return errors.New("start failed")
				}
				return nil
			})
			if (err == nil) != (scenario.name == "switch") || restarts != scenario.expectedRestarts {
				t.Fatalf("switch returned version=%q, err=%v, restarts=%d", version, err, restarts)
			}
			if got := GetXrayVersion(linked); got != "Xray "+scenario.expectedVersion {
				t.Fatalf("configured binary version = %q, want %s", got, scenario.expectedVersion)
			}
			if scenario.failRestart && !strings.Contains(err.Error(), "previous version restored") {
				t.Fatalf("missing rollback result: %v", err)
			}
		})
	}
}

// The manager must recognise the process it started. When it does not, every
// config sync takes the "Xray is managed by someone else" branch as well and
// restarts Xray a second time.
func TestManagedProcessIsReportedRunning(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start a child process: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	m := &XrayManager{cmd: cmd}
	if !m.isManagedProcessRunningLocked() {
		t.Fatal("a running child process is reported as not running")
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if m.isManagedProcessRunningLocked() {
		t.Fatal("an exited child process is reported as running")
	}
}
