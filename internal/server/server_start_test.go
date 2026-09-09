package server

import (
	"net"
	"path/filepath"
	"strconv"
	"testing"

	"v/internal/config"
)

func TestPrepareListenerRejectsAddressAlreadyInUse(t *testing.T) {
	existing, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer existing.Close()
	port := existing.Addr().(*net.TCPAddr).Port

	cfg := &config.Config{}
	if listener, err := prepareListener(cfg, net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
		listener.Close()
		t.Fatal("expected occupied listen address to fail")
	}
}

func TestPrepareListenerRejectsIncompleteTLSConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.TLSCert = "cert.pem"
	if listener, err := prepareListener(cfg, "127.0.0.1:0"); err == nil {
		listener.Close()
		t.Fatal("expected incomplete TLS configuration to fail")
	}
}

func TestNotificationOutboxDirUsesPersistentDataDirectory(t *testing.T) {
	t.Setenv("VPANEL_DATA_DIR", filepath.Join(t.TempDir(), "persistent"))

	dir, err := notificationOutboxDir(&config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute outbox path, got %q", dir)
	}
	if filepath.Base(dir) != "notification-outbox" {
		t.Fatalf("expected notification-outbox suffix, got %q", dir)
	}
}

func TestNotificationOutboxDirFallsBackToDatabasePath(t *testing.T) {
	t.Setenv("VPANEL_DATA_DIR", "")
	cfg := &config.Config{}
	cfg.Database.Path = filepath.Join("var", "lib", "vpanel", "v.db")

	dir, err := notificationOutboxDir(cfg)
	if err != nil {
		t.Fatal(err)
	}
	wantSuffix := filepath.Join("var", "lib", "vpanel", "notification-outbox")
	if got := filepath.Clean(dir); len(got) < len(wantSuffix) || got[len(got)-len(wantSuffix):] != wantSuffix {
		t.Fatalf("expected suffix %q, got %q", wantSuffix, got)
	}
}
