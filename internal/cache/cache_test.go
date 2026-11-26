package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 6: Cache Consistency
// For any cached data item, after the corresponding database record is updated,
// the cache entry SHALL be invalidated, and subsequent reads SHALL return the updated data.
// **Validates: Requirements 4.1, 4.2, 4.6**

func TestCacheConsistency_SetThenGet(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("set then get returns the same value", prop.ForAll(
		func(key string, value []byte) bool {
			if key == "" || len(value) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set value
			err := cache.Set(ctx, key, value, 0)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Get value
			retrieved, err := cache.Get(ctx, key)
			if err != nil {
				t.Logf("Get failed: %v", err)
				return false
			}

			// Values should match
			if len(retrieved) != len(value) {
				t.Logf("Length mismatch: expected %d, got %d", len(value), len(retrieved))
				return false
			}

			for i := range value {
				if retrieved[i] != value[i] {
					t.Logf("Value mismatch at index %d", i)
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestCacheConsistency_UpdateInvalidatesOldValue(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("updating a key returns the new value on subsequent reads", prop.ForAll(
		func(key string, oldValue, newValue []byte) bool {
			if key == "" || len(oldValue) == 0 || len(newValue) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set old value
			err := cache.Set(ctx, key, oldValue, 0)
			if err != nil {
				t.Logf("Set old value failed: %v", err)
				return false
			}

			// Update with new value
			err = cache.Set(ctx, key, newValue, 0)
			if err != nil {
				t.Logf("Set new value failed: %v", err)
				return false
			}

			// Get should return new value
			retrieved, err := cache.Get(ctx, key)
			if err != nil {
				t.Logf("Get failed: %v", err)
				return false
			}

			// Should match new value
			if len(retrieved) != len(newValue) {
				t.Logf("Length mismatch: expected %d, got %d", len(newValue), len(retrieved))
				return false
			}

			for i := range newValue {
				if retrieved[i] != newValue[i] {
					t.Logf("Value mismatch at index %d", i)
					return false
				}
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestCacheConsistency_DeleteInvalidatesEntry(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("delete removes the entry and subsequent get returns miss", prop.ForAll(
		func(key string, value []byte) bool {
			if key == "" || len(value) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set value
			err := cache.Set(ctx, key, value, 0)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Verify it exists
			exists, err := cache.Exists(ctx, key)
			if err != nil || !exists {
				t.Logf("Key should exist after set")
				return false
			}

			// Delete
			err = cache.Delete(ctx, key)
			if err != nil {
				t.Logf("Delete failed: %v", err)
				return false
			}

			// Get should return miss
			_, err = cache.Get(ctx, key)
			if err != ErrCacheMiss {
				t.Logf("Expected cache miss after delete, got: %v", err)
				return false
			}

			// Exists should return false
			exists, err = cache.Exists(ctx, key)
			if err != nil || exists {
				t.Logf("Key should not exist after delete")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestCacheConsistency_InvalidatePatternRemovesMatchingKeys(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("invalidate pattern removes all matching keys", prop.ForAll(
		func(prefix string, suffixes []string) bool {
			if prefix == "" || len(suffixes) == 0 {
				return true
			}

			// Limit suffixes to avoid too many keys
			if len(suffixes) > 10 {
				suffixes = suffixes[:10]
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set multiple keys with same prefix
			for _, suffix := range suffixes {
				if suffix == "" {
					continue
				}
				key := prefix + ":" + suffix
				err := cache.Set(ctx, key, []byte("value"), 0)
				if err != nil {
					t.Logf("Set failed for key %s: %v", key, err)
					return false
				}
			}

			// Set a key with different prefix
			otherKey := "other:" + prefix
			err := cache.Set(ctx, otherKey, []byte("other"), 0)
			if err != nil {
				t.Logf("Set other key failed: %v", err)
				return false
			}

			// Invalidate pattern
			pattern := prefix + ":*"
			err = cache.InvalidatePattern(ctx, pattern)
			if err != nil {
				t.Logf("InvalidatePattern failed: %v", err)
				return false
			}

			// All matching keys should be gone
			for _, suffix := range suffixes {
				if suffix == "" {
					continue
				}
				key := prefix + ":" + suffix
				_, err := cache.Get(ctx, key)
				if err != ErrCacheMiss {
					t.Logf("Key %s should be invalidated", key)
					return false
				}
			}

			// Other key should still exist
			_, err = cache.Get(ctx, otherKey)
			if err != nil {
				t.Logf("Other key should still exist: %v", err)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOfN(5, gen.Identifier()),
	))

	properties.TestingRun(t)
}


// Property 7: Cache TTL Expiration
// For any cached item with a configured TTL, after the TTL expires,
// the cache SHALL return a miss, and the data SHALL be fetched from the database.
// **Validates: Requirements 4.3**

func TestCacheTTLExpiration_ItemExpiresAfterTTL(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("item expires after TTL", prop.ForAll(
		func(key string, value []byte) bool {
			if key == "" || len(value) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     100 * time.Millisecond,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set with short TTL
			ttl := 50 * time.Millisecond
			err := cache.Set(ctx, key, value, ttl)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Should exist immediately
			retrieved, err := cache.Get(ctx, key)
			if err != nil {
				t.Logf("Get immediately after set failed: %v", err)
				return false
			}
			if len(retrieved) != len(value) {
				t.Logf("Value mismatch immediately after set")
				return false
			}

			// Wait for TTL to expire
			time.Sleep(60 * time.Millisecond)

			// Should be expired now
			_, err = cache.Get(ctx, key)
			if err != ErrCacheMiss {
				t.Logf("Expected cache miss after TTL, got: %v", err)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestCacheTTLExpiration_DefaultTTLApplied(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("default TTL is applied when TTL is 0", prop.ForAll(
		func(key string, value []byte) bool {
			if key == "" || len(value) == 0 {
				return true
			}

			defaultTTL := 50 * time.Millisecond
			config := Config{
				Type:           "memory",
				DefaultTTL:     defaultTTL,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set with TTL=0 (should use default)
			err := cache.Set(ctx, key, value, 0)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Should exist immediately
			_, err = cache.Get(ctx, key)
			if err != nil {
				t.Logf("Get immediately after set failed: %v", err)
				return false
			}

			// Wait for default TTL to expire
			time.Sleep(60 * time.Millisecond)

			// Should be expired now
			_, err = cache.Get(ctx, key)
			if err != ErrCacheMiss {
				t.Logf("Expected cache miss after default TTL, got: %v", err)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestCacheTTLExpiration_ExistsReturnsFalseAfterExpiry(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Exists returns false after TTL expires", prop.ForAll(
		func(key string, value []byte) bool {
			if key == "" || len(value) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     100 * time.Millisecond,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Set with short TTL
			ttl := 50 * time.Millisecond
			err := cache.Set(ctx, key, value, ttl)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Should exist immediately
			exists, err := cache.Exists(ctx, key)
			if err != nil || !exists {
				t.Logf("Should exist immediately after set")
				return false
			}

			// Wait for TTL to expire
			time.Sleep(60 * time.Millisecond)

			// Should not exist after expiry
			exists, err = cache.Exists(ctx, key)
			if err != nil {
				t.Logf("Exists check failed: %v", err)
				return false
			}
			if exists {
				t.Logf("Should not exist after TTL expires")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

// Additional cache tests for MGet and MSet

func TestCacheConsistency_MSetThenMGet(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("MSet then MGet returns all values", prop.ForAll(
		func(keys []string) bool {
			if len(keys) == 0 {
				return true
			}

			// Filter empty keys and limit count
			validKeys := make([]string, 0)
			seen := make(map[string]bool)
			for _, k := range keys {
				if k != "" && !seen[k] {
					validKeys = append(validKeys, k)
					seen[k] = true
				}
				if len(validKeys) >= 5 {
					break
				}
			}
			if len(validKeys) == 0 {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Create items map
			items := make(map[string][]byte)
			for _, key := range validKeys {
				items[key] = []byte("value-" + key)
			}

			// MSet
			err := cache.MSet(ctx, items, 0)
			if err != nil {
				t.Logf("MSet failed: %v", err)
				return false
			}

			// MGet
			retrieved, err := cache.MGet(ctx, validKeys)
			if err != nil {
				t.Logf("MGet failed: %v", err)
				return false
			}

			// All keys should be present
			if len(retrieved) != len(validKeys) {
				t.Logf("Expected %d keys, got %d", len(validKeys), len(retrieved))
				return false
			}

			// Values should match
			for key, expectedValue := range items {
				retrievedValue, ok := retrieved[key]
				if !ok {
					t.Logf("Key %s not found in retrieved", key)
					return false
				}
				if string(retrievedValue) != string(expectedValue) {
					t.Logf("Value mismatch for key %s", key)
					return false
				}
			}

			return true
		},
		gen.SliceOfN(10, gen.Identifier()),
	))

	properties.TestingRun(t)
}

func TestCacheConsistency_PingReturnsNilWhenHealthy(t *testing.T) {
	config := Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
		KeyPrefix:      "test:",
	}
	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	err := cache.Ping(ctx)
	if err != nil {
		t.Errorf("Ping should return nil for healthy cache, got: %v", err)
	}
}

func TestCacheConsistency_OperationsFailAfterClose(t *testing.T) {
	config := Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
		KeyPrefix:      "test:",
	}
	cache := NewMemoryCache(config)

	ctx := context.Background()

	// Close the cache
	err := cache.Close()
	if err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// All operations should fail
	_, err = cache.Get(ctx, "key")
	if err != ErrCacheClosed {
		t.Errorf("Get should return ErrCacheClosed after close, got: %v", err)
	}

	err = cache.Set(ctx, "key", []byte("value"), 0)
	if err != ErrCacheClosed {
		t.Errorf("Set should return ErrCacheClosed after close, got: %v", err)
	}

	err = cache.Delete(ctx, "key")
	if err != ErrCacheClosed {
		t.Errorf("Delete should return ErrCacheClosed after close, got: %v", err)
	}

	_, err = cache.Exists(ctx, "key")
	if err != ErrCacheClosed {
		t.Errorf("Exists should return ErrCacheClosed after close, got: %v", err)
	}

	err = cache.Ping(ctx)
	if err != ErrCacheClosed {
		t.Errorf("Ping should return ErrCacheClosed after close, got: %v", err)
	}
}

func TestCacheConsistency_InvalidKeyReturnsError(t *testing.T) {
	config := Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
		KeyPrefix:      "test:",
	}
	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// Empty key should return error
	_, err := cache.Get(ctx, "")
	if err != ErrInvalidKey {
		t.Errorf("Get with empty key should return ErrInvalidKey, got: %v", err)
	}

	err = cache.Set(ctx, "", []byte("value"), 0)
	if err != ErrInvalidKey {
		t.Errorf("Set with empty key should return ErrInvalidKey, got: %v", err)
	}

	err = cache.Delete(ctx, "")
	if err != ErrInvalidKey {
		t.Errorf("Delete with empty key should return ErrInvalidKey, got: %v", err)
	}

	_, err = cache.Exists(ctx, "")
	if err != ErrInvalidKey {
		t.Errorf("Exists with empty key should return ErrInvalidKey, got: %v", err)
	}
}

// Test JSON serialization round-trip for cached repository data
func TestCacheConsistency_JSONRoundTrip(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	type TestUser struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		Role     string `json:"role"`
		Enabled  bool   `json:"enabled"`
	}

	properties.Property("JSON serialization round-trip preserves data", prop.ForAll(
		func(id int64, username, email, role string, enabled bool) bool {
			if username == "" {
				return true
			}

			config := Config{
				Type:           "memory",
				DefaultTTL:     5 * time.Minute,
				MaxMemoryItems: 1000,
				KeyPrefix:      "test:",
			}
			cache := NewMemoryCache(config)
			defer cache.Close()

			ctx := context.Background()

			// Create user
			user := TestUser{
				ID:       id,
				Username: username,
				Email:    email,
				Role:     role,
				Enabled:  enabled,
			}

			// Serialize
			data, err := json.Marshal(user)
			if err != nil {
				t.Logf("Marshal failed: %v", err)
				return false
			}

			// Store in cache
			key := "user:" + username
			err = cache.Set(ctx, key, data, 0)
			if err != nil {
				t.Logf("Set failed: %v", err)
				return false
			}

			// Retrieve from cache
			retrieved, err := cache.Get(ctx, key)
			if err != nil {
				t.Logf("Get failed: %v", err)
				return false
			}

			// Deserialize
			var retrievedUser TestUser
			err = json.Unmarshal(retrieved, &retrievedUser)
			if err != nil {
				t.Logf("Unmarshal failed: %v", err)
				return false
			}

			// Compare
			if retrievedUser.ID != user.ID ||
				retrievedUser.Username != user.Username ||
				retrievedUser.Email != user.Email ||
				retrievedUser.Role != user.Role ||
				retrievedUser.Enabled != user.Enabled {
				t.Logf("User mismatch: expected %+v, got %+v", user, retrievedUser)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.Identifier(),
		gen.Identifier(),
		gen.OneConstOf("admin", "user", "viewer"),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

func TestCacheStats(t *testing.T) {
	config := Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
		KeyPrefix:      "test:",
	}
	cache := NewMemoryCache(config)
	defer cache.Close()

	ctx := context.Background()

	// Initial stats
	stats := cache.Stats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Sets != 0 || stats.Deletes != 0 {
		t.Error("Initial stats should be zero")
	}

	// Set a value
	cache.Set(ctx, "key1", []byte("value1"), 0)
	stats = cache.Stats()
	if stats.Sets != 1 {
		t.Errorf("Expected 1 set, got %d", stats.Sets)
	}

	// Get existing key (hit)
	cache.Get(ctx, "key1")
	stats = cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}

	// Get non-existing key (miss)
	cache.Get(ctx, "nonexistent")
	stats = cache.Stats()
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	// Delete
	cache.Delete(ctx, "key1")
	stats = cache.Stats()
	if stats.Deletes != 1 {
		t.Errorf("Expected 1 delete, got %d", stats.Deletes)
	}

	// Item count
	cache.Set(ctx, "key2", []byte("value2"), 0)
	cache.Set(ctx, "key3", []byte("value3"), 0)
	stats = cache.Stats()
	if stats.ItemCount != 2 {
		t.Errorf("Expected 2 items, got %d", stats.ItemCount)
	}
}
