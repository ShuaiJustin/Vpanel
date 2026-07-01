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

func TestRuntimeDatabaseInfoUsesRuntimeSQLitePath(t *testing.T) {
	handler := NewSettingsHandler(logger.NewNopLogger(), nil).
		WithRuntimeDatabaseConfig("sqlite", "/runtime/v.db", "/runtime/path-only.db")

	got := handler.runtimeDatabaseInfo()
	if got.Driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %q", got.Driver)
	}
	if got.Path != "/runtime/v.db" {
		t.Fatalf("expected runtime sqlite path, got %q", got.Path)
	}
	if got.DSNMasked != "" {
		t.Fatalf("did not expect masked DSN for sqlite, got %q", got.DSNMasked)
	}
}

func TestRuntimeDatabaseInfoMasksNonSQLiteDSN(t *testing.T) {
	handler := NewSettingsHandler(logger.NewNopLogger(), nil).
		WithRuntimeDatabaseConfig("mysql", "user:secret@tcp(db:3306)/vpanel", "")

	got := handler.runtimeDatabaseInfo()
	if got.Driver != "mysql" {
		t.Fatalf("expected mysql driver, got %q", got.Driver)
	}
	if got.DSNMasked != "user:***@tcp(db:3306)/vpanel" {
		t.Fatalf("expected masked DSN, got %q", got.DSNMasked)
	}
	if strings.Contains(got.DSNMasked, "secret") {
		t.Fatalf("masked DSN leaked password: %q", got.DSNMasked)
	}
}

func TestRuntimePanelInfoReportsDockerPublishPort(t *testing.T) {
	t.Setenv("VPANEL_PUBLISH_PORT", "13212")
	handler := NewSettingsHandler(logger.NewNopLogger(), nil)

	got := handler.runtimePanelInfo(&settings.SystemSettings{
		PanelAccessIP: "0.0.0.0",
		PanelPort:     8080,
		PublicURL:     "https://panel.shcrystal.top:13212",
	})

	if got.ListenPort != 8080 {
		t.Fatalf("expected internal listen port 8080, got %d", got.ListenPort)
	}
	if got.PublishPort != 13212 {
		t.Fatalf("expected publish port 13212, got %d", got.PublishPort)
	}
	if got.PublicPort != 13212 {
		t.Fatalf("expected public port 13212, got %d", got.PublicPort)
	}
}

func TestPublicSystemSettingsDoesNotMutateAuthSecrets(t *testing.T) {
	systemSettings := settings.DefaultSettings()
	systemSettings.Auth.BasicAuth.Password = "basic-secret"
	github := systemSettings.Auth.OAuth.Providers["github"]
	github.ClientSecret = "github-secret"
	systemSettings.Auth.OAuth.Providers["github"] = github

	publicSettings := publicSystemSettings(systemSettings)
	if publicSettings.Auth.BasicAuth.Password != "" {
		t.Fatal("expected public basic auth password to be hidden")
	}
	if publicSettings.Auth.OAuth.Providers["github"].ClientSecret != "" {
		t.Fatal("expected public oauth client secret to be hidden")
	}
	if systemSettings.Auth.BasicAuth.Password != "basic-secret" {
		t.Fatal("expected source basic auth password to remain intact")
	}
	if systemSettings.Auth.OAuth.Providers["github"].ClientSecret != "github-secret" {
		t.Fatal("expected source oauth client secret to remain intact")
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
