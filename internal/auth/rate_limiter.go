// Package auth provides authentication and authorization services.
package auth

import (
	"context"
	"sync"
	"time"

	"v/pkg/errors"
)

// RateLimiterConfig holds rate limiter configuration.
type RateLimiterConfig struct {
	MaxAttempts     int           // Maximum attempts allowed within the window
	Window          time.Duration // Time window for counting attempts
	BlockDuration   time.Duration // How long the IP remains blocked after exceeding the limit
	CleanupInterval time.Duration // Interval for cleaning up expired entries
}

// DefaultRateLimiterConfig returns the default rate limiter configuration.
// Default: 5 attempts per minute per IP
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		MaxAttempts:     5,
		Window:          time.Minute,
		BlockDuration:   time.Minute,
		CleanupInterval: 5 * time.Minute,
	}
}

// loginAttempt tracks login attempts for an IP.
type loginAttempt struct {
	attempts     int
	firstAttempt time.Time
	lastAttempt  time.Time
	blocked      bool
	blockedUntil time.Time
}

// RateLimiter implements IP-based rate limiting for login attempts.
type RateLimiter struct {
	config   RateLimiterConfig
	attempts map[string]*loginAttempt
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 5
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.BlockDuration <= 0 {
		config.BlockDuration = config.Window
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 5 * time.Minute
	}

	rl := &RateLimiter{
		config:   config,
		attempts: make(map[string]*loginAttempt),
		stopCh:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// cleanup periodically removes expired entries.
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanupExpired()
		case <-rl.stopCh:
			return
		}
	}
}

// cleanupExpired removes expired entries from the attempts map.
func (rl *RateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for ip, attempt := range rl.attempts {
		// Remove if the window has expired and not blocked
		if now.Sub(attempt.firstAttempt) > rl.config.Window && !attempt.blocked {
			delete(rl.attempts, ip)
			continue
		}
		// Remove if block has expired
		if attempt.blocked && now.After(attempt.blockedUntil) {
			delete(rl.attempts, ip)
		}
	}
}

// Stop stops the rate limiter cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// CheckRateLimit checks if the IP is rate limited.
// Returns true if the request is allowed, false if rate limited.
func (rl *RateLimiter) CheckRateLimit(ctx context.Context, ip string) (bool, error) {
	rl.mu.RLock()
	attempt, exists := rl.attempts[ip]
	rl.mu.RUnlock()

	if !exists {
		return true, nil
	}

	now := time.Now()

	// Check if blocked
	if attempt.blocked {
		if now.Before(attempt.blockedUntil) {
			return false, errors.NewRateLimitError("too many login attempts, please try again later")
		}
		// Block expired, allow the request
		return true, nil
	}

	// Check if window has expired
	if now.Sub(attempt.firstAttempt) > rl.config.Window {
		return true, nil
	}

	// Check if max attempts exceeded
	if attempt.attempts >= rl.config.MaxAttempts {
		return false, errors.NewRateLimitError("too many login attempts, please try again later")
	}

	return true, nil
}

// RecordLoginAttempt records a login attempt for the given IP.
// success indicates whether the login was successful.
func (rl *RateLimiter) RecordLoginAttempt(ctx context.Context, ip string, success bool) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	attempt, exists := rl.attempts[ip]

	if !exists {
		attempt = &loginAttempt{
			attempts:     0,
			firstAttempt: now,
		}
		rl.attempts[ip] = attempt
	}

	// If successful login, reset the counter
	if success {
		delete(rl.attempts, ip)
		return nil
	}

	// Check if window has expired, reset if so
	if now.Sub(attempt.firstAttempt) > rl.config.Window {
		attempt.attempts = 0
		attempt.firstAttempt = now
		attempt.blocked = false
	}

	// Increment attempt counter
	attempt.attempts++
	attempt.lastAttempt = now

	// Block if max attempts exceeded
	if attempt.attempts >= rl.config.MaxAttempts {
		attempt.blocked = true
		attempt.blockedUntil = now.Add(rl.config.BlockDuration)
	}

	return nil
}

// GetAttemptCount returns the current attempt count for an IP.
// Returns 0 if no attempts recorded or window expired.
func (rl *RateLimiter) GetAttemptCount(ip string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempt, exists := rl.attempts[ip]
	if !exists {
		return 0
	}

	// Check if window has expired
	if time.Since(attempt.firstAttempt) > rl.config.Window {
		return 0
	}

	return attempt.attempts
}

// IsBlocked checks if an IP is currently blocked.
func (rl *RateLimiter) IsBlocked(ip string) bool {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempt, exists := rl.attempts[ip]
	if !exists {
		return false
	}

	if !attempt.blocked {
		return false
	}

	return time.Now().Before(attempt.blockedUntil)
}

// Reset resets the rate limiter state for an IP.
func (rl *RateLimiter) Reset(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// ResetAll resets all rate limiter state.
func (rl *RateLimiter) ResetAll() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.attempts = make(map[string]*loginAttempt)
}

// GetBlockedUntil returns when the IP will be unblocked.
// Returns zero time if not blocked.
func (rl *RateLimiter) GetBlockedUntil(ip string) time.Time {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	attempt, exists := rl.attempts[ip]
	if !exists || !attempt.blocked {
		return time.Time{}
	}

	return attempt.blockedUntil
}

// RemainingAttempts returns the number of remaining attempts for an IP.
func (rl *RateLimiter) RemainingAttempts(ip string) int {
	count := rl.GetAttemptCount(ip)
	remaining := rl.config.MaxAttempts - count
	if remaining < 0 {
		return 0
	}
	return remaining
}
