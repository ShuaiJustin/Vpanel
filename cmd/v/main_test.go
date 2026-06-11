package main

import (
	"context"
	"encoding/json"
	"testing"

	"v/internal/config"
	"v/internal/logger"
	"v/internal/settings"
)

type startupSettingsRepo struct {
	values map[string]string
}

func newStartupSettingsService(values map[string]string) *settings.Service {
	return settings.NewService(&startupSettingsRepo{values: values})
}

func (r *startupSettingsRepo) Get(ctx context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *startupSettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(r.values))
	for key, value := range r.values {
		out[key] = value
	}
	return out, nil
}

func (r *startupSettingsRepo) Set(ctx context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *startupSettingsRepo) SetMultiple(ctx context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *startupSettingsRepo) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func (r *startupSettingsRepo) Backup(ctx context.Context) ([]byte, error) {
	return json.Marshal(r.values)
}

func (r *startupSettingsRepo) Restore(ctx context.Context, data []byte) error {
	return json.Unmarshal(data, &r.values)
}

func TestApplyStartupOverridesAllowsClearingPanelBasePath(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.BasePath = "/old"

	applyStartupOverridesFromSettings(cfg, newStartupSettingsService(map[string]string{
		"panel_base_path": "/",
	}), logger.NewNopLogger())

	if cfg.Server.BasePath != "" {
		t.Fatalf("expected base path to be cleared, got %q", cfg.Server.BasePath)
	}
}

func TestApplyStartupOverridesNormalizesPanelBasePath(t *testing.T) {
	cfg := &config.Config{}

	applyStartupOverridesFromSettings(cfg, newStartupSettingsService(map[string]string{
		"panel_base_path": "vpanel/",
	}), logger.NewNopLogger())

	if cfg.Server.BasePath != "/vpanel" {
		t.Fatalf("expected normalized base path, got %q", cfg.Server.BasePath)
	}
}

func TestApplyStartupOverridesAllowsClearingPanelTLS(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.TLSCert = "/env/cert.pem"
	cfg.Server.TLSKey = "/env/key.pem"
	cfg.Server.PublicURL = "http://panel.example.com"
	cfg.Server.CORSOrigins = []string{"http://panel.example.com"}

	applyStartupOverridesFromSettings(cfg, newStartupSettingsService(map[string]string{
		"panel_cert_path": "",
		"panel_key_path":  "",
	}), logger.NewNopLogger())

	if cfg.Server.TLSCert != "" || cfg.Server.TLSKey != "" {
		t.Fatalf("expected TLS paths to be cleared, got cert=%q key=%q", cfg.Server.TLSCert, cfg.Server.TLSKey)
	}
	if cfg.Server.PublicURL != "http://panel.example.com" {
		t.Fatalf("expected public URL to remain HTTP when TLS is cleared, got %q", cfg.Server.PublicURL)
	}
	if cfg.Server.CORSOrigins[0] != "http://panel.example.com" {
		t.Fatalf("expected CORS origin to remain HTTP when TLS is cleared, got %q", cfg.Server.CORSOrigins[0])
	}
}

func TestApplyStartupOverridesEnablesPanelTLSAndUpgradesURLs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.PublicURL = "http://panel.example.com"
	cfg.Server.CORSOrigins = []string{"http://panel.example.com", "https://admin.example.com"}

	applyStartupOverridesFromSettings(cfg, newStartupSettingsService(map[string]string{
		"panel_cert_path": "/data/cert.pem",
		"panel_key_path":  "/data/key.pem",
	}), logger.NewNopLogger())

	if cfg.Server.TLSCert != "/data/cert.pem" || cfg.Server.TLSKey != "/data/key.pem" {
		t.Fatalf("expected TLS paths to be applied, got cert=%q key=%q", cfg.Server.TLSCert, cfg.Server.TLSKey)
	}
	if cfg.Server.PublicURL != "https://panel.example.com" {
		t.Fatalf("expected public URL to upgrade to HTTPS, got %q", cfg.Server.PublicURL)
	}
	if cfg.Server.CORSOrigins[0] != "https://panel.example.com" || cfg.Server.CORSOrigins[1] != "https://admin.example.com" {
		t.Fatalf("expected CORS origins to be HTTPS-normalized, got %#v", cfg.Server.CORSOrigins)
	}
}
