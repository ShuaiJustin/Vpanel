package cache

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// memoryItem represents a cached item with expiration.
type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// isExpired checks if the item has expired.
func (i *memoryItem) isExpired() bool {
	if i.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(i.expiresAt)
}

// MemoryCache implements an in-memory cache with TTL support.
type MemoryCache struct {
	config    Config
	items     map[string]*memoryItem
	mu        sync.RWMutex
	stopCh    chan struct{}
	closed    bool

	// Statistics
	hits    int64
	misses  int64
	sets    int64
	deletes int64
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(config Config) *MemoryCache {
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 5 * time.Minute
	}
	if config.MaxMemoryItems <= 0 {
		config.MaxMemoryItems = 10000
	}

	mc := &MemoryCache{
		config: config,
		items:  make(map[string]*memoryItem),
		stopCh: make(chan struct{}),
	}

	// Start cleanup goroutine
	go mc.cleanup()

	return mc
}

// cleanup periodically removes expired items.
func (mc *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.removeExpired()
		case <-mc.stopCh:
			return
		}
	}
}

// removeExpired removes all expired items from the cache.
func (mc *MemoryCache) removeExpired() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key, item := range mc.items {
		if item.isExpired() {
			delete(mc.items, key)
		}
	}
}


// prefixKey adds the configured prefix to a key.
func (mc *MemoryCache) prefixKey(key string) string {
	if mc.config.KeyPrefix == "" {
		return key
	}
	return mc.config.KeyPrefix + key
}

// Get retrieves a value from the cache.
func (mc *MemoryCache) Get(ctx context.Context, key string) ([]byte, error) {
	if mc.closed {
		return nil, ErrCacheClosed
	}
	if key == "" {
		return nil, ErrInvalidKey
	}

	prefixedKey := mc.prefixKey(key)

	mc.mu.RLock()
	item, exists := mc.items[prefixedKey]
	mc.mu.RUnlock()

	if !exists || item.isExpired() {
		atomic.AddInt64(&mc.misses, 1)
		return nil, ErrCacheMiss
	}

	atomic.AddInt64(&mc.hits, 1)
	// Return a copy to prevent mutation
	result := make([]byte, len(item.value))
	copy(result, item.value)
	return result, nil
}

// Set stores a value in the cache.
func (mc *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if mc.closed {
		return ErrCacheClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	prefixedKey := mc.prefixKey(key)

	if ttl <= 0 {
		ttl = mc.config.DefaultTTL
	}

	// Make a copy of the value
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)

	item := &memoryItem{
		value:     valueCopy,
		expiresAt: time.Now().Add(ttl),
	}

	mc.mu.Lock()
	// Check if we need to evict items
	if len(mc.items) >= mc.config.MaxMemoryItems {
		mc.evictOldest()
	}
	mc.items[prefixedKey] = item
	mc.mu.Unlock()

	atomic.AddInt64(&mc.sets, 1)
	return nil
}

// evictOldest removes the oldest items when cache is full.
// Must be called with lock held.
func (mc *MemoryCache) evictOldest() {
	// Simple eviction: remove expired items first
	for key, item := range mc.items {
		if item.isExpired() {
			delete(mc.items, key)
		}
	}

	// If still over limit, remove 10% of items
	if len(mc.items) >= mc.config.MaxMemoryItems {
		toRemove := mc.config.MaxMemoryItems / 10
		count := 0
		for key := range mc.items {
			delete(mc.items, key)
			count++
			if count >= toRemove {
				break
			}
		}
	}
}

// Delete removes a key from the cache.
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	if mc.closed {
		return ErrCacheClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	prefixedKey := mc.prefixKey(key)

	mc.mu.Lock()
	delete(mc.items, prefixedKey)
	mc.mu.Unlock()

	atomic.AddInt64(&mc.deletes, 1)
	return nil
}

// Exists checks if a key exists in the cache.
func (mc *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	if mc.closed {
		return false, ErrCacheClosed
	}
	if key == "" {
		return false, ErrInvalidKey
	}

	prefixedKey := mc.prefixKey(key)

	mc.mu.RLock()
	item, exists := mc.items[prefixedKey]
	mc.mu.RUnlock()

	if !exists || item.isExpired() {
		return false, nil
	}

	return true, nil
}

// MGet retrieves multiple values from the cache.
func (mc *MemoryCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if mc.closed {
		return nil, ErrCacheClosed
	}

	result := make(map[string][]byte)

	mc.mu.RLock()
	defer mc.mu.RUnlock()

	for _, key := range keys {
		if key == "" {
			continue
		}
		prefixedKey := mc.prefixKey(key)
		item, exists := mc.items[prefixedKey]
		if exists && !item.isExpired() {
			valueCopy := make([]byte, len(item.value))
			copy(valueCopy, item.value)
			result[key] = valueCopy
			atomic.AddInt64(&mc.hits, 1)
		} else {
			atomic.AddInt64(&mc.misses, 1)
		}
	}

	return result, nil
}

// MSet stores multiple values in the cache.
func (mc *MemoryCache) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if mc.closed {
		return ErrCacheClosed
	}

	if ttl <= 0 {
		ttl = mc.config.DefaultTTL
	}

	expiresAt := time.Now().Add(ttl)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key, value := range items {
		if key == "" {
			continue
		}
		prefixedKey := mc.prefixKey(key)
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		mc.items[prefixedKey] = &memoryItem{
			value:     valueCopy,
			expiresAt: expiresAt,
		}
		atomic.AddInt64(&mc.sets, 1)
	}

	return nil
}

// InvalidatePattern removes all keys matching the pattern.
// Supports simple glob patterns with * wildcard.
func (mc *MemoryCache) InvalidatePattern(ctx context.Context, pattern string) error {
	if mc.closed {
		return ErrCacheClosed
	}

	prefixedPattern := mc.prefixKey(pattern)

	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key := range mc.items {
		if matchPattern(prefixedPattern, key) {
			delete(mc.items, key)
			atomic.AddInt64(&mc.deletes, 1)
		}
	}

	return nil
}

// matchPattern checks if a key matches a simple glob pattern.
func matchPattern(pattern, key string) bool {
	// Simple glob matching with * wildcard
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(key, prefix)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(key, suffix)
	}

	return pattern == key
}

// Ping checks if the cache is available.
func (mc *MemoryCache) Ping(ctx context.Context) error {
	if mc.closed {
		return ErrCacheClosed
	}
	return nil
}

// Close closes the cache.
func (mc *MemoryCache) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return nil
	}

	mc.closed = true
	close(mc.stopCh)
	mc.items = nil

	return nil
}

// Stats returns cache statistics.
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	itemCount := int64(len(mc.items))
	mc.mu.RUnlock()

	return CacheStats{
		Hits:      atomic.LoadInt64(&mc.hits),
		Misses:    atomic.LoadInt64(&mc.misses),
		Sets:      atomic.LoadInt64(&mc.sets),
		Deletes:   atomic.LoadInt64(&mc.deletes),
		ItemCount: itemCount,
	}
}

// Clear removes all items from the cache.
func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.items = make(map[string]*memoryItem)
}
