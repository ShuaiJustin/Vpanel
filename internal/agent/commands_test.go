package agent

import "testing"

func TestShouldRestartExternalXrayAfterConfigSync(t *testing.T) {
	tests := []struct {
		name                  string
		managedRunningBefore  bool
		observedRunningBefore bool
		want                  bool
	}{
		{
			name:                  "external xray requires restart",
			managedRunningBefore:  false,
			observedRunningBefore: true,
			want:                  true,
		},
		{
			name:                  "agent managed xray already restarted by update",
			managedRunningBefore:  true,
			observedRunningBefore: true,
			want:                  false,
		},
		{
			name:                  "stopped xray does not need restart",
			managedRunningBefore:  false,
			observedRunningBefore: false,
			want:                  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRestartExternalXrayAfterConfigSync(tt.managedRunningBefore, tt.observedRunningBefore)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
