package api

import (
	"testing"

	"v/internal/settings"
)

func TestValidateEmailSettingsAllowsDefaultPortWithoutSMTP(t *testing.T) {
	router := &Router{}
	cfg := settings.DefaultSettings()
	if err := router.validateEmailSettings(cfg); err != nil {
		t.Fatalf("SMTP is not configured: %v", err)
	}
	cfg.SMTPUser = "sender@example.test"
	if err := router.validateEmailSettings(cfg); err == nil {
		t.Fatal("expected partially configured SMTP to fail validation")
	}
}
