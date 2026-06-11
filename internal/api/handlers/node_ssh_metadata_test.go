package handlers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistNodeSSHPrivateKeyWritesKeyFile(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("VPANEL_DATA_DIR", dataDir)

	keyPath, err := persistNodeSSHPrivateKey(42, "  PRIVATE KEY DATA\n\n")
	if err != nil {
		t.Fatalf("expected private key to be persisted, got %v", err)
	}

	expectedPath := filepath.Join(dataDir, "node-ssh-keys", "node-42.key")
	if keyPath != expectedPath {
		t.Fatalf("unexpected key path: got %q want %q", keyPath, expectedPath)
	}

	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read persisted private key: %v", err)
	}
	if string(data) != "PRIVATE KEY DATA\n" {
		t.Fatalf("unexpected key content: %q", string(data))
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat persisted private key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected key permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestPersistNodeSSHPrivateKeyIgnoresEmptyKey(t *testing.T) {
	if keyPath, err := persistNodeSSHPrivateKey(42, " \n\t "); err != nil || keyPath != "" {
		t.Fatalf("expected empty key to be ignored, got path=%q err=%v", keyPath, err)
	}
}
