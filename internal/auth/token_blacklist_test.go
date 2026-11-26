package auth

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 5: Token Revocation
// For any revoked JWT token, subsequent API requests using that token
// SHALL be rejected with an unauthorized error.
// **Validates: Requirements 1.6**

func TestTokenRevocation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("revoked tokens are marked as revoked", prop.ForAll(
		func(token string) bool {
			if token == "" {
				return true
			}

			config := DefaultTokenBlacklistConfig()
			config.CleanupInterval = time.Hour // Long interval to avoid cleanup during test
			tb := NewTokenBlacklist(config)
			defer tb.Stop()

			ctx := context.Background()
			expiresAt := time.Now().Add(time.Hour)

			// Token should not be revoked initially
			if tb.IsRevoked(ctx, token) {
				t.Log("Token should not be revoked initially")
				return false
			}

			// Revoke the token
			err := tb.RevokeToken(ctx, token, expiresAt)
			if err != nil {
				t.Logf("Failed to revoke token: %v", err)
				return false
			}

			// Token should now be revoked
			if !tb.IsRevoked(ctx, token) {
				t.Log("Token should be revoked after RevokeToken")
				return false
			}

			return true
		},
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestTokenRevocation_DifferentTokensIndependent(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("revoking one token does not affect others", prop.ForAll(
		func(token1, token2 string) bool {
			if token1 == "" || token2 == "" || token1 == token2 {
				return true
			}

			config := DefaultTokenBlacklistConfig()
			config.CleanupInterval = time.Hour
			tb := NewTokenBlacklist(config)
			defer tb.Stop()

			ctx := context.Background()
			expiresAt := time.Now().Add(time.Hour)

			// Revoke token1
			tb.RevokeToken(ctx, token1, expiresAt)

			// token1 should be revoked
			if !tb.IsRevoked(ctx, token1) {
				t.Log("token1 should be revoked")
				return false
			}

			// token2 should NOT be revoked
			if tb.IsRevoked(ctx, token2) {
				t.Log("token2 should not be revoked")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}


func TestTokenRevocation_ExpiredTokensNotRevoked(t *testing.T) {
	config := DefaultTokenBlacklistConfig()
	config.CleanupInterval = time.Hour
	tb := NewTokenBlacklist(config)
	defer tb.Stop()

	ctx := context.Background()
	token := "test-token-123"

	// Revoke with past expiration
	expiresAt := time.Now().Add(-time.Hour)
	tb.RevokeToken(ctx, token, expiresAt)

	// Token should NOT be considered revoked (it's already expired)
	if tb.IsRevoked(ctx, token) {
		t.Error("Expired token should not be considered revoked")
	}
}

func TestTokenRevocation_RevokedAt(t *testing.T) {
	config := DefaultTokenBlacklistConfig()
	config.CleanupInterval = time.Hour
	tb := NewTokenBlacklist(config)
	defer tb.Stop()

	ctx := context.Background()
	token := "test-token-123"
	expiresAt := time.Now().Add(time.Hour)

	// Before revocation
	revokedAt := tb.GetRevokedAt(token)
	if !revokedAt.IsZero() {
		t.Error("RevokedAt should be zero before revocation")
	}

	// Revoke
	beforeRevoke := time.Now()
	tb.RevokeToken(ctx, token, expiresAt)
	afterRevoke := time.Now()

	// After revocation
	revokedAt = tb.GetRevokedAt(token)
	if revokedAt.IsZero() {
		t.Error("RevokedAt should not be zero after revocation")
	}
	if revokedAt.Before(beforeRevoke) || revokedAt.After(afterRevoke) {
		t.Error("RevokedAt should be between before and after revoke times")
	}
}

func TestTokenRevocation_Count(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("revoked count increases correctly", prop.ForAll(
		func(tokens []string) bool {
			// Deduplicate tokens
			seen := make(map[string]bool)
			uniqueTokens := make([]string, 0)
			for _, t := range tokens {
				if t != "" && !seen[t] {
					seen[t] = true
					uniqueTokens = append(uniqueTokens, t)
				}
			}

			if len(uniqueTokens) == 0 {
				return true
			}

			config := DefaultTokenBlacklistConfig()
			config.CleanupInterval = time.Hour
			tb := NewTokenBlacklist(config)
			defer tb.Stop()

			ctx := context.Background()
			expiresAt := time.Now().Add(time.Hour)

			// Initial count should be 0
			if tb.GetRevokedCount() != 0 {
				return false
			}

			// Revoke tokens
			for _, token := range uniqueTokens {
				tb.RevokeToken(ctx, token, expiresAt)
			}

			// Count should match unique tokens
			if tb.GetRevokedCount() != len(uniqueTokens) {
				return false
			}

			return true
		},
		gen.SliceOf(gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestTokenRevocation_Clear(t *testing.T) {
	config := DefaultTokenBlacklistConfig()
	config.CleanupInterval = time.Hour
	tb := NewTokenBlacklist(config)
	defer tb.Stop()

	ctx := context.Background()
	expiresAt := time.Now().Add(time.Hour)

	// Revoke some tokens
	tokens := []string{"token1", "token2", "token3"}
	for _, token := range tokens {
		tb.RevokeToken(ctx, token, expiresAt)
	}

	if tb.GetRevokedCount() != 3 {
		t.Errorf("Expected 3 revoked tokens, got %d", tb.GetRevokedCount())
	}

	// Clear
	tb.Clear()

	if tb.GetRevokedCount() != 0 {
		t.Errorf("Expected 0 revoked tokens after clear, got %d", tb.GetRevokedCount())
	}

	// Tokens should no longer be revoked
	for _, token := range tokens {
		if tb.IsRevoked(ctx, token) {
			t.Errorf("Token %s should not be revoked after clear", token)
		}
	}
}

func TestTokenRevocation_RemoveToken(t *testing.T) {
	config := DefaultTokenBlacklistConfig()
	config.CleanupInterval = time.Hour
	tb := NewTokenBlacklist(config)
	defer tb.Stop()

	ctx := context.Background()
	token := "test-token-123"
	expiresAt := time.Now().Add(time.Hour)

	// Revoke
	tb.RevokeToken(ctx, token, expiresAt)
	if !tb.IsRevoked(ctx, token) {
		t.Error("Token should be revoked")
	}

	// Remove
	tb.RemoveToken(token)
	if tb.IsRevoked(ctx, token) {
		t.Error("Token should not be revoked after removal")
	}
}

func TestTokenRevocation_HashConsistency(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("same token always produces same hash", prop.ForAll(
		func(token string) bool {
			if token == "" {
				return true
			}

			hash1 := hashToken(token)
			hash2 := hashToken(token)

			return hash1 == hash2
		},
		gen.Identifier(),
	))

	properties.Property("different tokens produce different hashes", prop.ForAll(
		func(token1, token2 string) bool {
			if token1 == "" || token2 == "" || token1 == token2 {
				return true
			}

			hash1 := hashToken(token1)
			hash2 := hashToken(token2)

			return hash1 != hash2
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}
