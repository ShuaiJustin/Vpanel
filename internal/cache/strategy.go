// Package cache provides caching strategy definitions including cache key patterns,
// TTL configurations, and cache invalidation helpers for the V Panel application.
//
// Cache Strategy Design:
// - User traffic stats: 60s TTL (frequently changing data)
// - Node stats: 30s TTL (moderately volatile data)
// - Proxy stats: 30s TTL (moderately volatile data)
// - Proxy config: 5m TTL (infrequently changing data)
// - Announcements: 2m TTL (relatively stable public data)
// - Node lists: 1m TTL (occasionally changing data)
// - User summary: 30s TTL (moderately changing aggregate data)
//
// The Invalidator provides domain-specific methods to invalidate cache entries
// when the underlying data changes, ensuring data consistency across the application.
package cache

import (
	"context"
	"fmt"
	"time"
)

// Cache key patterns for different data types
const (
	// KeyUserTraffic is the cache key pattern for user traffic statistics
	// Format: user:traffic:{userID}
	KeyUserTraffic = "user:traffic:%d"

	// KeyNodeStats is the cache key pattern for node statistics
	// Format: node:stats:{nodeID}
	KeyNodeStats = "node:stats:%d"

	// KeyProxyStats is the cache key pattern for proxy statistics
	// Format: proxy:stats:{proxyID}
	KeyProxyStats = "proxy:stats:%d"

	// KeyProxyConfig is the cache key pattern for proxy configuration
	// Format: proxy:config:{proxyID}
	KeyProxyConfig = "proxy:config:%d"

	// KeyAnnouncementList is the cache key pattern for announcement lists
	// Format: announcements:list:{page}:{pageSize}
	KeyAnnouncementList = "announcements:list:%d:%d"

	// KeyNodeList is the cache key pattern for node lists
	// Format: nodes:list:{filterHash}
	KeyNodeList = "nodes:list:%s"

	// KeyUserSummary is the cache key pattern for user summary statistics
	// Format: users:summary
	KeyUserSummary = "users:summary"
)

// TTL configurations for different cache key patterns
// These values are based on data volatility and update frequency
var TTLConfig = map[string]time.Duration{
	"user:traffic":   60 * time.Second, // User traffic stats change frequently
	"node:stats":     30 * time.Second, // Node stats are moderately volatile
	"proxy:stats":    30 * time.Second, // Proxy stats are moderately volatile
	"proxy:config":   5 * time.Minute,  // Proxy config changes infrequently
	"announcements":  2 * time.Minute,  // Announcements are relatively stable
	"nodes:list":     1 * time.Minute,  // Node lists change occasionally
	"users:summary":  30 * time.Second, // User summary changes moderately
}

// GetTTL returns the TTL for a given cache key prefix
func GetTTL(keyPrefix string) time.Duration {
	if ttl, ok := TTLConfig[keyPrefix]; ok {
		return ttl
	}
	// Default TTL if not found
	return 1 * time.Minute
}

// Invalidator provides methods to invalidate cache entries
// It wraps the cache interface and provides domain-specific invalidation methods
type Invalidator struct {
	cache Cache
}

// NewInvalidator creates a new cache invalidator
func NewInvalidator(cache Cache) *Invalidator {
	return &Invalidator{
		cache: cache,
	}
}

// InvalidateUserTraffic invalidates the cache for user traffic statistics
func (i *Invalidator) InvalidateUserTraffic(ctx context.Context, userID uint) error {
	key := fmt.Sprintf(KeyUserTraffic, userID)
	return i.cache.Delete(ctx, key)
}

// InvalidateNodeStats invalidates the cache for node statistics
func (i *Invalidator) InvalidateNodeStats(ctx context.Context, nodeID uint) error {
	key := fmt.Sprintf(KeyNodeStats, nodeID)
	return i.cache.Delete(ctx, key)
}

// InvalidateProxyStats invalidates the cache for proxy statistics
func (i *Invalidator) InvalidateProxyStats(ctx context.Context, proxyID uint) error {
	key := fmt.Sprintf(KeyProxyStats, proxyID)
	return i.cache.Delete(ctx, key)
}

// InvalidateProxyConfig invalidates the cache for proxy configuration
func (i *Invalidator) InvalidateProxyConfig(ctx context.Context, proxyID uint) error {
	key := fmt.Sprintf(KeyProxyConfig, proxyID)
	return i.cache.Delete(ctx, key)
}

// InvalidateAllProxyCache invalidates both stats and config cache for a proxy
func (i *Invalidator) InvalidateAllProxyCache(ctx context.Context, proxyID uint) error {
	if err := i.InvalidateProxyStats(ctx, proxyID); err != nil {
		return fmt.Errorf("failed to invalidate proxy stats: %w", err)
	}
	if err := i.InvalidateProxyConfig(ctx, proxyID); err != nil {
		return fmt.Errorf("failed to invalidate proxy config: %w", err)
	}
	return nil
}

// InvalidateAnnouncementList invalidates the cache for announcement lists
// This invalidates all pages by using a pattern match
func (i *Invalidator) InvalidateAnnouncementList(ctx context.Context) error {
	pattern := "announcements:list:*"
	return i.cache.InvalidatePattern(ctx, pattern)
}

// InvalidateNodeList invalidates the cache for node lists
// This invalidates all filtered node lists by using a pattern match
func (i *Invalidator) InvalidateNodeList(ctx context.Context) error {
	pattern := "nodes:list:*"
	return i.cache.InvalidatePattern(ctx, pattern)
}

// InvalidateUserSummary invalidates the cache for user summary statistics
func (i *Invalidator) InvalidateUserSummary(ctx context.Context) error {
	return i.cache.Delete(ctx, KeyUserSummary)
}

// InvalidateAllNodeCache invalidates all node-related caches
// This includes node stats and node lists
func (i *Invalidator) InvalidateAllNodeCache(ctx context.Context, nodeID uint) error {
	if err := i.InvalidateNodeStats(ctx, nodeID); err != nil {
		return fmt.Errorf("failed to invalidate node stats: %w", err)
	}
	if err := i.InvalidateNodeList(ctx); err != nil {
		return fmt.Errorf("failed to invalidate node list: %w", err)
	}
	return nil
}

// InvalidateAllUserCache invalidates all user-related caches
// This includes user traffic and user summary
func (i *Invalidator) InvalidateAllUserCache(ctx context.Context, userID uint) error {
	if err := i.InvalidateUserTraffic(ctx, userID); err != nil {
		return fmt.Errorf("failed to invalidate user traffic: %w", err)
	}
	if err := i.InvalidateUserSummary(ctx); err != nil {
		return fmt.Errorf("failed to invalidate user summary: %w", err)
	}
	return nil
}

// CacheKeyBuilder provides helper methods to build cache keys
type CacheKeyBuilder struct{}

// NewCacheKeyBuilder creates a new cache key builder
func NewCacheKeyBuilder() *CacheKeyBuilder {
	return &CacheKeyBuilder{}
}

// UserTrafficKey builds a cache key for user traffic statistics
func (b *CacheKeyBuilder) UserTrafficKey(userID uint) string {
	return fmt.Sprintf(KeyUserTraffic, userID)
}

// NodeStatsKey builds a cache key for node statistics
func (b *CacheKeyBuilder) NodeStatsKey(nodeID uint) string {
	return fmt.Sprintf(KeyNodeStats, nodeID)
}

// ProxyStatsKey builds a cache key for proxy statistics
func (b *CacheKeyBuilder) ProxyStatsKey(proxyID uint) string {
	return fmt.Sprintf(KeyProxyStats, proxyID)
}

// ProxyConfigKey builds a cache key for proxy configuration
func (b *CacheKeyBuilder) ProxyConfigKey(proxyID uint) string {
	return fmt.Sprintf(KeyProxyConfig, proxyID)
}

// AnnouncementListKey builds a cache key for announcement lists
func (b *CacheKeyBuilder) AnnouncementListKey(page, pageSize int) string {
	return fmt.Sprintf(KeyAnnouncementList, page, pageSize)
}

// NodeListKey builds a cache key for node lists
func (b *CacheKeyBuilder) NodeListKey(filterHash string) string {
	return fmt.Sprintf(KeyNodeList, filterHash)
}

// UserSummaryKey builds a cache key for user summary statistics
func (b *CacheKeyBuilder) UserSummaryKey() string {
	return KeyUserSummary
}
