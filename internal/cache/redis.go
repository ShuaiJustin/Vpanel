package cache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements a Redis-based cache.
type RedisCache struct {
	client *redis.Client
	config Config
	closed bool

	// Statistics
	hits    int64
	misses  int64
	sets    int64
	deletes int64
}

// NewRedisCache creates a new Redis cache.
func NewRedisCache(config Config) (*RedisCache, error) {
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 5 * time.Minute
	}

	client := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
		config: config,
	}, nil
}

// prefixKey adds the configured prefix to a key.
func (rc *RedisCache) prefixKey(key string) string {
	if rc.config.KeyPrefix == "" {
		return key
	}
	return rc.config.KeyPrefix + key
}

// Get retrieves a value from the cache.
func (rc *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	if rc.closed {
		return nil, ErrCacheClosed
	}
	if key == "" {
		return nil, ErrInvalidKey
	}

	prefixedKey := rc.prefixKey(key)

	result, err := rc.client.Get(ctx, prefixedKey).Bytes()
	if err == redis.Nil {
		atomic.AddInt64(&rc.misses, 1)
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, err
	}

	atomic.AddInt64(&rc.hits, 1)
	return result, nil
}

// Set stores a value in the cache.
func (rc *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if rc.closed {
		return ErrCacheClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	prefixedKey := rc.prefixKey(key)

	if ttl <= 0 {
		ttl = rc.config.DefaultTTL
	}

	err := rc.client.Set(ctx, prefixedKey, value, ttl).Err()
	if err != nil {
		return err
	}

	atomic.AddInt64(&rc.sets, 1)
	return nil
}

// Delete removes a key from the cache.
func (rc *RedisCache) Delete(ctx context.Context, key string) error {
	if rc.closed {
		return ErrCacheClosed
	}
	if key == "" {
		return ErrInvalidKey
	}

	prefixedKey := rc.prefixKey(key)

	err := rc.client.Del(ctx, prefixedKey).Err()
	if err != nil {
		return err
	}

	atomic.AddInt64(&rc.deletes, 1)
	return nil
}

// Exists checks if a key exists in the cache.
func (rc *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	if rc.closed {
		return false, ErrCacheClosed
	}
	if key == "" {
		return false, ErrInvalidKey
	}

	prefixedKey := rc.prefixKey(key)

	result, err := rc.client.Exists(ctx, prefixedKey).Result()
	if err != nil {
		return false, err
	}

	return result > 0, nil
}


// MGet retrieves multiple values from the cache.
func (rc *RedisCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	if rc.closed {
		return nil, ErrCacheClosed
	}

	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	// Prefix all keys
	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = rc.prefixKey(key)
	}

	results, err := rc.client.MGet(ctx, prefixedKeys...).Result()
	if err != nil {
		return nil, err
	}

	result := make(map[string][]byte)
	for i, val := range results {
		if val != nil {
			if str, ok := val.(string); ok {
				result[keys[i]] = []byte(str)
				atomic.AddInt64(&rc.hits, 1)
			}
		} else {
			atomic.AddInt64(&rc.misses, 1)
		}
	}

	return result, nil
}

// MSet stores multiple values in the cache.
func (rc *RedisCache) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if rc.closed {
		return ErrCacheClosed
	}

	if len(items) == 0 {
		return nil
	}

	if ttl <= 0 {
		ttl = rc.config.DefaultTTL
	}

	// Use pipeline for efficiency
	pipe := rc.client.Pipeline()

	for key, value := range items {
		prefixedKey := rc.prefixKey(key)
		pipe.Set(ctx, prefixedKey, value, ttl)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	atomic.AddInt64(&rc.sets, int64(len(items)))
	return nil
}

// InvalidatePattern removes all keys matching the pattern.
func (rc *RedisCache) InvalidatePattern(ctx context.Context, pattern string) error {
	if rc.closed {
		return ErrCacheClosed
	}

	prefixedPattern := rc.prefixKey(pattern)

	// Use SCAN to find matching keys
	var cursor uint64
	var deletedCount int64

	for {
		keys, nextCursor, err := rc.client.Scan(ctx, cursor, prefixedPattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := rc.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
			deletedCount += int64(len(keys))
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	atomic.AddInt64(&rc.deletes, deletedCount)
	return nil
}

// Ping checks if the cache is available.
func (rc *RedisCache) Ping(ctx context.Context) error {
	if rc.closed {
		return ErrCacheClosed
	}
	return rc.client.Ping(ctx).Err()
}

// Close closes the cache connection.
func (rc *RedisCache) Close() error {
	if rc.closed {
		return nil
	}
	rc.closed = true
	return rc.client.Close()
}

// Stats returns cache statistics.
func (rc *RedisCache) Stats() CacheStats {
	ctx := context.Background()
	
	// Get item count from Redis
	var itemCount int64
	info, err := rc.client.DBSize(ctx).Result()
	if err == nil {
		itemCount = info
	}

	return CacheStats{
		Hits:      atomic.LoadInt64(&rc.hits),
		Misses:    atomic.LoadInt64(&rc.misses),
		Sets:      atomic.LoadInt64(&rc.sets),
		Deletes:   atomic.LoadInt64(&rc.deletes),
		ItemCount: itemCount,
	}
}

// Client returns the underlying Redis client for advanced operations.
func (rc *RedisCache) Client() *redis.Client {
	return rc.client
}
