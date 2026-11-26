package cache

import (
	"context"
	"testing"
	"time"
)

func TestGetTTL(t *testing.T) {
	tests := []struct {
		name      string
		keyPrefix string
		expected  time.Duration
	}{
		{
			name:      "user traffic TTL",
			keyPrefix: "user:traffic",
			expected:  60 * time.Second,
		},
		{
			name:      "node stats TTL",
			keyPrefix: "node:stats",
			expected:  30 * time.Second,
		},
		{
			name:      "proxy stats TTL",
			keyPrefix: "proxy:stats",
			expected:  30 * time.Second,
		},
		{
			name:      "proxy config TTL",
			keyPrefix: "proxy:config",
			expected:  5 * time.Minute,
		},
		{
			name:      "announcements TTL",
			keyPrefix: "announcements",
			expected:  2 * time.Minute,
		},
		{
			name:      "nodes list TTL",
			keyPrefix: "nodes:list",
			expected:  1 * time.Minute,
		},
		{
			name:      "users summary TTL",
			keyPrefix: "users:summary",
			expected:  30 * time.Second,
		},
		{
			name:      "unknown prefix returns default",
			keyPrefix: "unknown:prefix",
			expected:  1 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTTL(tt.keyPrefix)
			if result != tt.expected {
				t.Errorf("GetTTL(%s) = %v, want %v", tt.keyPrefix, result, tt.expected)
			}
		})
	}
}

func TestCacheKeyBuilder(t *testing.T) {
	builder := NewCacheKeyBuilder()

	tests := []struct {
		name     string
		buildFn  func() string
		expected string
	}{
		{
			name:     "user traffic key",
			buildFn:  func() string { return builder.UserTrafficKey(123) },
			expected: "user:traffic:123",
		},
		{
			name:     "node stats key",
			buildFn:  func() string { return builder.NodeStatsKey(456) },
			expected: "node:stats:456",
		},
		{
			name:     "proxy stats key",
			buildFn:  func() string { return builder.ProxyStatsKey(789) },
			expected: "proxy:stats:789",
		},
		{
			name:     "proxy config key",
			buildFn:  func() string { return builder.ProxyConfigKey(101) },
			expected: "proxy:config:101",
		},
		{
			name:     "announcement list key",
			buildFn:  func() string { return builder.AnnouncementListKey(1, 20) },
			expected: "announcements:list:1:20",
		},
		{
			name:     "node list key",
			buildFn:  func() string { return builder.NodeListKey("abc123") },
			expected: "nodes:list:abc123",
		},
		{
			name:     "user summary key",
			buildFn:  func() string { return builder.UserSummaryKey() },
			expected: "users:summary",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.buildFn()
			if result != tt.expected {
				t.Errorf("got %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestInvalidator(t *testing.T) {
	ctx := context.Background()
	cache := NewMemoryCache(DefaultConfig())
	invalidator := NewInvalidator(cache)

	t.Run("invalidate user traffic", func(t *testing.T) {
		key := "user:traffic:123"
		cache.Set(ctx, key, []byte("test"), time.Minute)

		err := invalidator.InvalidateUserTraffic(ctx, 123)
		if err != nil {
			t.Errorf("InvalidateUserTraffic failed: %v", err)
		}

		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("key should not exist after invalidation")
		}
	})

	t.Run("invalidate node stats", func(t *testing.T) {
		key := "node:stats:456"
		cache.Set(ctx, key, []byte("test"), time.Minute)

		err := invalidator.InvalidateNodeStats(ctx, 456)
		if err != nil {
			t.Errorf("InvalidateNodeStats failed: %v", err)
		}

		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("key should not exist after invalidation")
		}
	})

	t.Run("invalidate proxy stats", func(t *testing.T) {
		key := "proxy:stats:789"
		cache.Set(ctx, key, []byte("test"), time.Minute)

		err := invalidator.InvalidateProxyStats(ctx, 789)
		if err != nil {
			t.Errorf("InvalidateProxyStats failed: %v", err)
		}

		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("key should not exist after invalidation")
		}
	})

	t.Run("invalidate proxy config", func(t *testing.T) {
		key := "proxy:config:101"
		cache.Set(ctx, key, []byte("test"), time.Minute)

		err := invalidator.InvalidateProxyConfig(ctx, 101)
		if err != nil {
			t.Errorf("InvalidateProxyConfig failed: %v", err)
		}

		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("key should not exist after invalidation")
		}
	})

	t.Run("invalidate all proxy cache", func(t *testing.T) {
		statsKey := "proxy:stats:202"
		configKey := "proxy:config:202"
		cache.Set(ctx, statsKey, []byte("test"), time.Minute)
		cache.Set(ctx, configKey, []byte("test"), time.Minute)

		err := invalidator.InvalidateAllProxyCache(ctx, 202)
		if err != nil {
			t.Errorf("InvalidateAllProxyCache failed: %v", err)
		}

		statsExists, _ := cache.Exists(ctx, statsKey)
		configExists, _ := cache.Exists(ctx, configKey)
		if statsExists || configExists {
			t.Error("both keys should not exist after invalidation")
		}
	})

	t.Run("invalidate user summary", func(t *testing.T) {
		key := "users:summary"
		cache.Set(ctx, key, []byte("test"), time.Minute)

		err := invalidator.InvalidateUserSummary(ctx)
		if err != nil {
			t.Errorf("InvalidateUserSummary failed: %v", err)
		}

		exists, _ := cache.Exists(ctx, key)
		if exists {
			t.Error("key should not exist after invalidation")
		}
	})

	t.Run("invalidate announcement list pattern", func(t *testing.T) {
		cache.Set(ctx, "announcements:list:1:20", []byte("test"), time.Minute)
		cache.Set(ctx, "announcements:list:2:20", []byte("test"), time.Minute)

		err := invalidator.InvalidateAnnouncementList(ctx)
		if err != nil {
			t.Errorf("InvalidateAnnouncementList failed: %v", err)
		}

		exists1, _ := cache.Exists(ctx, "announcements:list:1:20")
		exists2, _ := cache.Exists(ctx, "announcements:list:2:20")
		if exists1 || exists2 {
			t.Error("announcement list keys should not exist after pattern invalidation")
		}
	})

	t.Run("invalidate node list pattern", func(t *testing.T) {
		cache.Set(ctx, "nodes:list:filter1", []byte("test"), time.Minute)
		cache.Set(ctx, "nodes:list:filter2", []byte("test"), time.Minute)

		err := invalidator.InvalidateNodeList(ctx)
		if err != nil {
			t.Errorf("InvalidateNodeList failed: %v", err)
		}

		exists1, _ := cache.Exists(ctx, "nodes:list:filter1")
		exists2, _ := cache.Exists(ctx, "nodes:list:filter2")
		if exists1 || exists2 {
			t.Error("node list keys should not exist after pattern invalidation")
		}
	})

	t.Run("invalidate all node cache", func(t *testing.T) {
		cache.Set(ctx, "node:stats:303", []byte("test"), time.Minute)
		cache.Set(ctx, "nodes:list:filter3", []byte("test"), time.Minute)

		err := invalidator.InvalidateAllNodeCache(ctx, 303)
		if err != nil {
			t.Errorf("InvalidateAllNodeCache failed: %v", err)
		}

		statsExists, _ := cache.Exists(ctx, "node:stats:303")
		listExists, _ := cache.Exists(ctx, "nodes:list:filter3")
		if statsExists || listExists {
			t.Error("node cache keys should not exist after invalidation")
		}
	})

	t.Run("invalidate all user cache", func(t *testing.T) {
		cache.Set(ctx, "user:traffic:404", []byte("test"), time.Minute)
		cache.Set(ctx, "users:summary", []byte("test"), time.Minute)

		err := invalidator.InvalidateAllUserCache(ctx, 404)
		if err != nil {
			t.Errorf("InvalidateAllUserCache failed: %v", err)
		}

		trafficExists, _ := cache.Exists(ctx, "user:traffic:404")
		summaryExists, _ := cache.Exists(ctx, "users:summary")
		if trafficExists || summaryExists {
			t.Error("user cache keys should not exist after invalidation")
		}
	})
}

func TestInvalidatorWithNilCache(t *testing.T) {
	// Test that invalidator handles nil cache gracefully
	// This shouldn't panic but will return errors
	ctx := context.Background()
	
	// Create a mock cache that returns errors
	cache := NewMemoryCache(DefaultConfig())
	cache.Close() // Close the cache to simulate unavailable cache
	
	invalidator := NewInvalidator(cache)
	
	// These should not panic even with closed cache
	_ = invalidator.InvalidateUserTraffic(ctx, 1)
	_ = invalidator.InvalidateNodeStats(ctx, 1)
	_ = invalidator.InvalidateProxyStats(ctx, 1)
	_ = invalidator.InvalidateProxyConfig(ctx, 1)
}
