// Package cache provides caching functionality for the V Panel application.
// It supports both in-memory and Redis-based caching with a unified interface.
package cache

import (
	"context"
	"errors"
	"time"
)

// Common errors
var (
	ErrCacheMiss    = errors.New("cache: key not found")
	ErrCacheClosed  = errors.New("cache: connection closed")
	ErrInvalidKey   = errors.New("cache: invalid key")
	ErrInvalidValue = errors.New("cache: invalid value")
)

// Cache defines the interface for cache operations.
// Both memory and Redis implementations must satisfy this interface.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns ErrCacheMiss if the key doesn't exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with the specified TTL.
	// If ttl is 0, the default TTL is used.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a key from the cache.
	// Returns nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache.
	Exists(ctx context.Context, key string) (bool, error)

	// MGet retrieves multiple values from the cache.
	// Missing keys are not included in the result map.
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)

	// MSet stores multiple values in the cache with the specified TTL.
	MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error

	// InvalidatePattern removes all keys matching the pattern.
	// Pattern syntax depends on the implementation.
	InvalidatePattern(ctx context.Context, pattern string) error

	// Ping checks if the cache is available.
	Ping(ctx context.Context) error

	// Close closes the cache connection.
	Close() error
}

// Config holds cache configuration.
type Config struct {
	// Type specifies the cache type: "memory" or "redis"
	Type string `yaml:"type" env:"V_CACHE_TYPE" default:"memory"`

	// Redis configuration
	RedisAddr     string `yaml:"redis_addr" env:"V_REDIS_ADDR" default:"localhost:6379"`
	RedisPassword string `yaml:"redis_password" env:"V_REDIS_PASSWORD" default:""`
	RedisDB       int    `yaml:"redis_db" env:"V_REDIS_DB" default:"0"`

	// Common configuration
	DefaultTTL     time.Duration `yaml:"default_ttl" env:"V_CACHE_DEFAULT_TTL" default:"5m"`
	MaxMemoryItems int           `yaml:"max_memory_items" env:"V_CACHE_MAX_ITEMS" default:"10000"`
	KeyPrefix      string        `yaml:"key_prefix" env:"V_CACHE_KEY_PREFIX" default:"vpanel:"`
}

// DefaultConfig returns the default cache configuration.
func DefaultConfig() Config {
	return Config{
		Type:           "memory",
		RedisAddr:      "localhost:6379",
		RedisPassword:  "",
		RedisDB:        0,
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 10000,
		KeyPrefix:      "vpanel:",
	}
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits       int64 `json:"hits"`
	Misses     int64 `json:"misses"`
	Sets       int64 `json:"sets"`
	Deletes    int64 `json:"deletes"`
	ItemCount  int64 `json:"item_count"`
	MemoryUsed int64 `json:"memory_used,omitempty"`
}

// StatsProvider is an optional interface for caches that provide statistics.
type StatsProvider interface {
	Stats() CacheStats
}
