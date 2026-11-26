package cache

import (
	"fmt"
	"strings"
)

// CacheType represents the type of cache.
type CacheType string

const (
	CacheTypeMemory CacheType = "memory"
	CacheTypeRedis  CacheType = "redis"
)

// New creates a new cache based on the configuration.
func New(config Config) (Cache, error) {
	cacheType := CacheType(strings.ToLower(config.Type))

	switch cacheType {
	case CacheTypeMemory, "":
		return NewMemoryCache(config), nil
	case CacheTypeRedis:
		return NewRedisCache(config)
	default:
		return nil, fmt.Errorf("unsupported cache type: %s", config.Type)
	}
}

// MustNew creates a new cache and panics on error.
func MustNew(config Config) Cache {
	cache, err := New(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create cache: %v", err))
	}
	return cache
}

// NewMemory creates a new in-memory cache with default configuration.
func NewMemory() *MemoryCache {
	return NewMemoryCache(DefaultConfig())
}

// NewRedis creates a new Redis cache with the specified address.
func NewRedis(addr string) (*RedisCache, error) {
	config := DefaultConfig()
	config.Type = string(CacheTypeRedis)
	config.RedisAddr = addr
	return NewRedisCache(config)
}

// Validate validates the cache configuration.
func (c *Config) Validate() error {
	cacheType := CacheType(strings.ToLower(c.Type))

	switch cacheType {
	case CacheTypeMemory, "":
		// Memory cache has no special requirements
		if c.MaxMemoryItems < 0 {
			return fmt.Errorf("max_memory_items must be non-negative")
		}
	case CacheTypeRedis:
		if c.RedisAddr == "" {
			return fmt.Errorf("redis_addr is required for redis cache")
		}
		if c.RedisDB < 0 || c.RedisDB > 15 {
			return fmt.Errorf("redis_db must be between 0 and 15")
		}
	default:
		return fmt.Errorf("unsupported cache type: %s", c.Type)
	}

	if c.DefaultTTL < 0 {
		return fmt.Errorf("default_ttl must be non-negative")
	}

	return nil
}

// IsMemory returns true if the cache type is memory.
func (c *Config) IsMemory() bool {
	return CacheType(strings.ToLower(c.Type)) == CacheTypeMemory || c.Type == ""
}

// IsRedis returns true if the cache type is redis.
func (c *Config) IsRedis() bool {
	return CacheType(strings.ToLower(c.Type)) == CacheTypeRedis
}
