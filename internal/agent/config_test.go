package agent

import "testing"

func TestApplyEnvOverridesOverridesHealthPort(t *testing.T) {
	t.Setenv("AGENT_HEALTH_PORT", "18444")
	t.Setenv("AGENT_HEALTH_HOST", "127.0.0.1")

	cfg := DefaultConfig()
	applyEnvOverrides(cfg)

	if got, want := cfg.Health.Port, 18444; got != want {
		t.Fatalf("expected health port %d, got %d", want, got)
	}
	if got, want := cfg.Health.Host, "127.0.0.1"; got != want {
		t.Fatalf("expected health host %q, got %q", want, got)
	}
}
