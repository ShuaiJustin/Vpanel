// Package auth provides authentication and authorization services.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// TokenBlacklistConfig holds token blacklist configuration.
type TokenBlacklistConfig struct {
	CleanupInterval time.Duration // Interval for cleaning up expired entries
}

// DefaultTokenBlacklistConfig returns the default token blacklist configuration.
func DefaultTokenBlacklistConfig() TokenBlacklistConfig {
	return TokenBlacklistConfig{
		CleanupInterval: 15 * time.Minute,
	}
}

// blacklistedToken represents a revoked token.
type blacklistedToken struct {
	tokenHash string
	expiresAt time.Time
	revokedAt time.Time
}

// TokenBlacklist manages revoked JWT tokens.
// It stores token hashes to prevent reuse of revoked tokens.
type TokenBlacklist struct {
	config TokenBlacklistConfig
	tokens map[string]*blacklistedToken
	mu     sync.RWMutex
	stopCh chan struct{}
}

// NewTokenBlacklist creates a new token blacklist with the given configuration.
func NewTokenBlacklist(config TokenBlacklistConfig) *TokenBlacklist {
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = 15 * time.Minute
	}

	tb := &TokenBlacklist{
		config: config,
		tokens: make(map[string]*blacklistedToken),
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go tb.cleanup()

	return tb
}

// cleanup periodically removes expired entries.
func (tb *TokenBlacklist) cleanup() {
	ticker := time.NewTicker(tb.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tb.cleanupExpired()
		case <-tb.stopCh:
			return
		}
	}
}

// cleanupExpired removes expired tokens from the blacklist.
func (tb *TokenBlacklist) cleanupExpired() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	for hash, token := range tb.tokens {
		if now.After(token.expiresAt) {
			delete(tb.tokens, hash)
		}
	}
}

// Stop stops the token blacklist cleanup goroutine.
func (tb *TokenBlacklist) Stop() {
	close(tb.stopCh)
}

// hashToken creates a SHA-256 hash of the token.
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}


// RevokeToken adds a token to the blacklist.
// expiresAt should be the token's original expiration time.
func (tb *TokenBlacklist) RevokeToken(ctx context.Context, token string, expiresAt time.Time) error {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	hash := hashToken(token)
	tb.tokens[hash] = &blacklistedToken{
		tokenHash: hash,
		expiresAt: expiresAt,
		revokedAt: time.Now(),
	}

	return nil
}

// IsRevoked checks if a token has been revoked.
func (tb *TokenBlacklist) IsRevoked(ctx context.Context, token string) bool {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	hash := hashToken(token)
	bt, exists := tb.tokens[hash]
	if !exists {
		return false
	}

	// If the token has expired, it's effectively not in the blacklist anymore
	// (it would be invalid anyway due to expiration)
	if time.Now().After(bt.expiresAt) {
		return false
	}

	return true
}

// GetRevokedCount returns the number of tokens currently in the blacklist.
func (tb *TokenBlacklist) GetRevokedCount() int {
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	return len(tb.tokens)
}

// Clear removes all tokens from the blacklist.
func (tb *TokenBlacklist) Clear() {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	tb.tokens = make(map[string]*blacklistedToken)
}

// GetRevokedAt returns when a token was revoked.
// Returns zero time if the token is not in the blacklist.
func (tb *TokenBlacklist) GetRevokedAt(token string) time.Time {
	tb.mu.RLock()
	defer tb.mu.RUnlock()

	hash := hashToken(token)
	bt, exists := tb.tokens[hash]
	if !exists {
		return time.Time{}
	}

	return bt.revokedAt
}

// RemoveToken removes a specific token from the blacklist.
// This is mainly useful for testing.
func (tb *TokenBlacklist) RemoveToken(token string) {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	hash := hashToken(token)
	delete(tb.tokens, hash)
}

// TokenBlacklistStore defines the interface for persistent token blacklist storage.
// This can be implemented with database or Redis for distributed deployments.
type TokenBlacklistStore interface {
	// Add adds a token hash to the blacklist.
	Add(ctx context.Context, tokenHash string, expiresAt time.Time) error
	// Exists checks if a token hash exists in the blacklist.
	Exists(ctx context.Context, tokenHash string) (bool, error)
	// Remove removes a token hash from the blacklist.
	Remove(ctx context.Context, tokenHash string) error
	// CleanupExpired removes expired entries.
	CleanupExpired(ctx context.Context) error
}

// PersistentTokenBlacklist wraps TokenBlacklist with persistent storage.
type PersistentTokenBlacklist struct {
	*TokenBlacklist
	store TokenBlacklistStore
}

// NewPersistentTokenBlacklist creates a new persistent token blacklist.
func NewPersistentTokenBlacklist(config TokenBlacklistConfig, store TokenBlacklistStore) *PersistentTokenBlacklist {
	return &PersistentTokenBlacklist{
		TokenBlacklist: NewTokenBlacklist(config),
		store:          store,
	}
}

// RevokeToken adds a token to both in-memory and persistent blacklist.
func (ptb *PersistentTokenBlacklist) RevokeToken(ctx context.Context, token string, expiresAt time.Time) error {
	// Add to in-memory blacklist
	if err := ptb.TokenBlacklist.RevokeToken(ctx, token, expiresAt); err != nil {
		return err
	}

	// Add to persistent store if available
	if ptb.store != nil {
		hash := hashToken(token)
		if err := ptb.store.Add(ctx, hash, expiresAt); err != nil {
			return err
		}
	}

	return nil
}

// IsRevoked checks if a token has been revoked in either in-memory or persistent blacklist.
func (ptb *PersistentTokenBlacklist) IsRevoked(ctx context.Context, token string) bool {
	// Check in-memory first (faster)
	if ptb.TokenBlacklist.IsRevoked(ctx, token) {
		return true
	}

	// Check persistent store if available
	if ptb.store != nil {
		hash := hashToken(token)
		exists, err := ptb.store.Exists(ctx, hash)
		if err == nil && exists {
			return true
		}
	}

	return false
}
