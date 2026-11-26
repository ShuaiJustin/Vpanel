package dispatcher

import (
	"testing"
	"time"

	"v/internal/database/repository"
)

func TestExpiryReminderGoesOutOncePerStage(t *testing.T) {
	expires := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	at := func(daysBefore float64) time.Time {
		return expires.Add(-time.Duration(daysBefore * 24 * float64(time.Hour)))
	}
	ptr := func(v time.Time) *time.Time { return &v }

	cases := []struct {
		name     string
		now      time.Time
		lastSent *time.Time
		want     bool
	}{
		{"outside the window", at(8), nil, false},
		{"enters 7-day stage", at(6.9), nil, true},
		{"already reminded in 7-day stage", at(5), ptr(at(6.9)), false},
		{"enters 3-day stage", at(2.9), ptr(at(6.9)), true},
		{"already reminded in 3-day stage", at(2), ptr(at(2.9)), false},
		{"enters last day", at(0.9), ptr(at(2.9)), true},
		{"already reminded on last day", at(0.5), ptr(at(0.9)), false},
		{"renewed: reminder from the old period does not count", at(6), ptr(at(40)), true},
	}
	for _, tc := range cases {
		if got := expiryReminderDue(expires, tc.lastSent, tc.now); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestTrafficWarningGoesOutOncePerCycle(t *testing.T) {
	now := time.Date(2026, 12, 20, 0, 0, 0, 0, time.UTC)
	reset := now.AddDate(0, 0, -10)
	before, after := reset.Add(-time.Hour), reset.Add(time.Hour)

	if !trafficWarningDue(&repository.User{TrafficResetAt: &reset, LastTrafficWarningAt: &before}, now) {
		t.Error("a warning from the previous cycle must not suppress this cycle's")
	}
	if trafficWarningDue(&repository.User{TrafficResetAt: &reset, LastTrafficWarningAt: &after}, now) {
		t.Error("a second warning in the same cycle must be suppressed")
	}
	recent := now.AddDate(0, 0, -2)
	if trafficWarningDue(&repository.User{LastTrafficWarningAt: &recent}, now) {
		t.Error("without a cycle start the cooldown applies")
	}
}
