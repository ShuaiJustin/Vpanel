package handlers

import (
	"strings"
	"testing"

	"v/internal/database"
	"v/internal/logger"
	"v/internal/settings"
)

func TestShouldPersistPaymentSettings(t *testing.T) {
	t.Run("skip when request and store both lack payment fields", func(t *testing.T) {
		if shouldPersistPaymentSettings(&UpdateSettingsRequest{}, map[string]string{}) {
			t.Fatal("expected payment settings to be skipped")
		}
	})

	t.Run("persist when request includes payment field", func(t *testing.T) {
		enabled := true
		if !shouldPersistPaymentSettings(&UpdateSettingsRequest{PaymentAlipayEnabled: &enabled}, map[string]string{}) {
			t.Fatal("expected payment settings to be persisted when request touches them")
		}
	})

	t.Run("persist when store already contains payment field", func(t *testing.T) {
		if !shouldPersistPaymentSettings(&UpdateSettingsRequest{}, map[string]string{"payment_wechat_enabled": "true"}) {
			t.Fatal("expected existing persisted payment settings to be kept")
		}
	})
}

func TestBuildCutoverInstructionUsesRuntimeEnvNames(t *testing.T) {
	got := buildCutoverInstruction(&database.Config{
		Driver: "mysql",
		DSN:    "user:pass@tcp(db:3306)/vpanel",
	})

	if got["V_DB_DRIVER"] != "mysql" {
		t.Fatalf("expected V_DB_DRIVER, got %#v", got)
	}
	if got["V_DB_DSN"] != "user:pass@tcp(db:3306)/vpanel" {
		t.Fatalf("expected V_DB_DSN, got %#v", got)
	}
	if _, ok := got["V_DATABASE_DRIVER"]; ok {
		t.Fatalf("unexpected legacy env key in %#v", got)
	}
	if _, ok := got["V_DATABASE_DSN"]; ok {
		t.Fatalf("unexpected legacy env key in %#v", got)
	}
}

func TestCurrentSQLiteDatabasePathPrefersRuntimeConfig(t *testing.T) {
	handler := NewSettingsHandler(logger.NewNopLogger(), nil).
		WithRuntimeDatabaseConfig("sqlite", "/runtime/v.db", "/runtime/path-only.db")

	path, ok, err := handler.currentSQLiteDatabasePath(&settings.SystemSettings{
		DBType:     "sqlite",
		SQLitePath: "/target/from-settings.db",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected sqlite backup to be supported")
	}
	if path != "/runtime/v.db" {
		t.Fatalf("expected runtime DSN path, got %q", path)
	}
}

func TestCurrentSQLiteDatabasePathRejectsNonSQLiteRuntime(t *testing.T) {
	handler := NewSettingsHandler(logger.NewNopLogger(), nil).
		WithRuntimeDatabaseConfig("mysql", "user:pass@tcp(db:3306)/vpanel", "")

	if path, ok, err := handler.currentSQLiteDatabasePath(&settings.SystemSettings{
		DBType:     "sqlite",
		SQLitePath: "/target/from-settings.db",
	}); err != nil || ok || path != "" {
		t.Fatalf("expected non-sqlite runtime to be unsupported, path=%q ok=%v err=%v", path, ok, err)
	}
}

func TestBuildDatabaseTestConfigRequiresExplicitSQLitePath(t *testing.T) {
	if _, err := buildDatabaseTestConfig(TestDatabaseRequest{DBType: "sqlite"}); err == nil {
		t.Fatal("expected sqlite target path to be required")
	}
}

func TestValidateMigrationTargetRejectsCurrentSQLiteDatabase(t *testing.T) {
	current := t.TempDir() + "/v.db"
	handler := NewSettingsHandler(logger.NewNopLogger(), nil).
		WithRuntimeDatabaseConfig("sqlite", current, "")

	err := handler.validateMigrationTarget(&database.Config{
		Driver: "sqlite",
		DSN:    current,
	})
	if err == nil {
		t.Fatal("expected migration to current sqlite database to be rejected")
	}
	if !strings.Contains(err.Error(), "different") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizePanelBasePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "empty is root", input: "", want: "/"},
		{name: "adds leading slash", input: "vpanel", want: "/vpanel"},
		{name: "trims trailing slash", input: "/vpanel/", want: "/vpanel"},
		{name: "rejects full url", input: "https://example.com/vpanel", wantErr: true},
		{name: "rejects repeated slash", input: "/vp//admin", wantErr: true},
		{name: "rejects query", input: "/vpanel?x=1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizePanelBasePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestValidatePanelSettingsRejectsPartialTLS(t *testing.T) {
	err := validatePanelSettings(&settings.SystemSettings{
		PanelPort:     8080,
		PanelBasePath: "/",
		PanelCertPath: "/app/certs/fullchain.pem",
		PanelKeyPath:  "",
		Timezone:      "Asia/Shanghai",
	})
	if err == nil {
		t.Fatal("expected partial TLS config to be rejected")
	}
}
