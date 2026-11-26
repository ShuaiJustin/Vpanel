package auth

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/pkg/errors"
)

// Property 2: Rate Limiting Enforcement
// For any IP address making login attempts, after 5 failed attempts within 1 minute,
// subsequent login attempts SHALL be rejected with a rate limit error until the window expires.
// **Validates: Requirements 1.2**

func TestRateLimitingEnforcement(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("after max attempts, subsequent requests are blocked", prop.ForAll(
		func(ip string) bool {
			if ip == "" {
				return true
			}

			config := RateLimiterConfig{
				MaxAttempts:     5,
				Window:          time.Minute,
				CleanupInterval: time.Hour, // Long interval to avoid cleanup during test
			}
			rl := NewRateLimiter(config)
			defer rl.Stop()

			ctx := context.Background()

			// First 5 attempts should be allowed
			for i := 0; i < 5; i++ {
				allowed, err := rl.CheckRateLimit(ctx, ip)
				if !allowed || err != nil {
					t.Logf("Attempt %d should be allowed, got allowed=%v, err=%v", i+1, allowed, err)
					return false
				}
				// Record failed attempt
				rl.RecordLoginAttempt(ctx, ip, false)
			}

			// 6th attempt should be blocked
			allowed, err := rl.CheckRateLimit(ctx, ip)
			if allowed {
				t.Log("6th attempt should be blocked")
				return false
			}
			if err == nil || !errors.IsRateLimit(err) {
				t.Logf("Expected rate limit error, got %v", err)
				return false
			}

			return true
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestRateLimitingEnforcement_SuccessfulLoginResets(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("successful login resets the counter", prop.ForAll(
		func(ip string, failedAttempts int) bool {
			if ip == "" {
				return true
			}
			// Limit failed attempts to less than max
			failedAttempts = failedAttempts % 4
			if failedAttempts < 0 {
				failedAttempts = 0
			}

			config := RateLimiterConfig{
				MaxAttempts:     5,
				Window:          time.Minute,
				CleanupInterval: time.Hour,
			}
			rl := NewRateLimiter(config)
			defer rl.Stop()

			ctx := context.Background()

			// Record some failed attempts
			for i := 0; i < failedAttempts; i++ {
				rl.RecordLoginAttempt(ctx, ip, false)
			}

			// Verify attempts were recorded
			if rl.GetAttemptCount(ip) != failedAttempts {
				t.Logf("Expected %d attempts, got %d", failedAttempts, rl.GetAttemptCount(ip))
				return false
			}

			// Successful login should reset
			rl.RecordLoginAttempt(ctx, ip, true)

			// Counter should be reset
			if rl.GetAttemptCount(ip) != 0 {
				t.Logf("Expected 0 attempts after success, got %d", rl.GetAttemptCount(ip))
				return false
			}

			// Should be allowed again
			allowed, err := rl.CheckRateLimit(ctx, ip)
			if !allowed || err != nil {
				t.Log("Should be allowed after successful login")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}


func TestRateLimitingEnforcement_DifferentIPsIndependent(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("different IPs have independent rate limits", prop.ForAll(
		func(ip1, ip2 string) bool {
			if ip1 == "" || ip2 == "" || ip1 == ip2 {
				return true
			}

			config := RateLimiterConfig{
				MaxAttempts:     5,
				Window:          time.Minute,
				CleanupInterval: time.Hour,
			}
			rl := NewRateLimiter(config)
			defer rl.Stop()

			ctx := context.Background()

			// Block ip1
			for i := 0; i < 5; i++ {
				rl.RecordLoginAttempt(ctx, ip1, false)
			}

			// ip1 should be blocked
			allowed1, _ := rl.CheckRateLimit(ctx, ip1)
			if allowed1 {
				t.Log("ip1 should be blocked")
				return false
			}

			// ip2 should still be allowed
			allowed2, err := rl.CheckRateLimit(ctx, ip2)
			if !allowed2 || err != nil {
				t.Log("ip2 should be allowed")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestRateLimitingEnforcement_RemainingAttempts(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("remaining attempts decreases correctly", prop.ForAll(
		func(ip string, attempts int) bool {
			if ip == "" {
				return true
			}
			attempts = attempts % 6
			if attempts < 0 {
				attempts = 0
			}

			config := RateLimiterConfig{
				MaxAttempts:     5,
				Window:          time.Minute,
				CleanupInterval: time.Hour,
			}
			rl := NewRateLimiter(config)
			defer rl.Stop()

			ctx := context.Background()

			// Record attempts
			for i := 0; i < attempts; i++ {
				rl.RecordLoginAttempt(ctx, ip, false)
			}

			remaining := rl.RemainingAttempts(ip)
			expected := 5 - attempts
			if expected < 0 {
				expected = 0
			}

			if remaining != expected {
				t.Logf("Expected %d remaining, got %d", expected, remaining)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

func TestRateLimitingEnforcement_BlockedStatus(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ctx := context.Background()
	ip := "192.168.1.1"

	// Initially not blocked
	if rl.IsBlocked(ip) {
		t.Error("Should not be blocked initially")
	}

	// Record 5 failed attempts
	for i := 0; i < 5; i++ {
		rl.RecordLoginAttempt(ctx, ip, false)
	}

	// Should be blocked now
	if !rl.IsBlocked(ip) {
		t.Error("Should be blocked after 5 failed attempts")
	}

	// BlockedUntil should be set
	blockedUntil := rl.GetBlockedUntil(ip)
	if blockedUntil.IsZero() {
		t.Error("BlockedUntil should be set")
	}

	// Should be blocked for approximately 1 minute
	expectedUnblock := time.Now().Add(time.Minute)
	if blockedUntil.Before(expectedUnblock.Add(-time.Second)) || blockedUntil.After(expectedUnblock.Add(time.Second)) {
		t.Errorf("BlockedUntil should be approximately 1 minute from now, got %v", blockedUntil)
	}
}

func TestRateLimitingEnforcement_Reset(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ctx := context.Background()
	ip := "192.168.1.1"

	// Record some attempts
	for i := 0; i < 3; i++ {
		rl.RecordLoginAttempt(ctx, ip, false)
	}

	if rl.GetAttemptCount(ip) != 3 {
		t.Errorf("Expected 3 attempts, got %d", rl.GetAttemptCount(ip))
	}

	// Reset
	rl.Reset(ip)

	if rl.GetAttemptCount(ip) != 0 {
		t.Errorf("Expected 0 attempts after reset, got %d", rl.GetAttemptCount(ip))
	}
}

func TestRateLimitingEnforcement_ResetAll(t *testing.T) {
	config := RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		CleanupInterval: time.Hour,
	}
	rl := NewRateLimiter(config)
	defer rl.Stop()

	ctx := context.Background()

	// Record attempts for multiple IPs
	ips := []string{"192.168.1.1", "192.168.1.2", "192.168.1.3"}
	for _, ip := range ips {
		rl.RecordLoginAttempt(ctx, ip, false)
	}

	// Verify attempts recorded
	for _, ip := range ips {
		if rl.GetAttemptCount(ip) != 1 {
			t.Errorf("Expected 1 attempt for %s, got %d", ip, rl.GetAttemptCount(ip))
		}
	}

	// Reset all
	rl.ResetAll()

	// Verify all reset
	for _, ip := range ips {
		if rl.GetAttemptCount(ip) != 0 {
			t.Errorf("Expected 0 attempts for %s after reset, got %d", ip, rl.GetAttemptCount(ip))
		}
	}
}
