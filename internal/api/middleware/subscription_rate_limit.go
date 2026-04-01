// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SubscriptionRateLimiter provides rate limiting for subscription access.
// It limits requests per token/IP combination to prevent abuse.
type SubscriptionRateLimiter struct {
	mu              sync.RWMutex
	clients         map[string]*subscriptionClient
	requestsPerHour int
	maxClients      int
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type subscriptionClient struct {
	requests  int
	windowEnd time.Time
	lastSeen  time.Time
}

// NewSubscriptionRateLimiter creates a new subscription rate limiter.
// requestsPerHour specifies the maximum number of requests allowed per hour per client.
func NewSubscriptionRateLimiter(requestsPerHour int) *SubscriptionRateLimiter {
	if requestsPerHour <= 0 {
		requestsPerHour = 60 // Default: 60 requests per hour
	}

	rl := &SubscriptionRateLimiter{
		clients:         make(map[string]*subscriptionClient),
		requestsPerHour: requestsPerHour,
		maxClients:      10000,           // Limit memory usage
		cleanupInterval: 5 * time.Minute, // More frequent cleanup
		stopCh:          make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanup()

	return rl
}

// RateLimit returns a middleware that limits subscription access.
func (rl *SubscriptionRateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client identifier (token + IP)
		token := c.Param("token")
		if token == "" {
			token = c.Param("code") // For short code routes
		}
		ip := c.ClientIP()
		clientKey := token + ":" + ip

		if !rl.allow(clientKey) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "Rate limit exceeded",
				"message": "Too many subscription requests. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// allow checks if a request is allowed for the given client key.
func (rl *SubscriptionRateLimiter) allow(clientKey string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, exists := rl.clients[clientKey]

	if !exists || now.After(client.windowEnd) {
		// Check if we need to evict old entries before adding new one
		if !exists && len(rl.clients) >= rl.maxClients {
			rl.evictOldest()
		}

		// New window
		rl.clients[clientKey] = &subscriptionClient{
			requests:  1,
			windowEnd: now.Add(time.Hour),
			lastSeen:  now,
		}
		return true
	}

	if client.requests >= rl.requestsPerHour {
		client.lastSeen = now
		return false
	}

	client.requests++
	client.lastSeen = now
	return true
}

// evictOldest removes the oldest 10% of entries to prevent memory exhaustion.
func (rl *SubscriptionRateLimiter) evictOldest() {
	toRemove := len(rl.clients) / 10
	if toRemove < 100 {
		toRemove = 100
	}

	type entry struct {
		key      string
		lastSeen time.Time
	}

	entries := make([]entry, 0, len(rl.clients))
	for key, client := range rl.clients {
		entries = append(entries, entry{key: key, lastSeen: client.lastSeen})
	}

	// Sort by lastSeen (oldest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].lastSeen.Before(entries[j].lastSeen)
	})

	// Remove oldest entries
	for i := 0; i < toRemove && i < len(entries); i++ {
		delete(rl.clients, entries[i].key)
	}
}

// cleanup periodically removes expired client entries.
func (rl *SubscriptionRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
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

// cleanupExpired removes expired client entries.
func (rl *SubscriptionRateLimiter) cleanupExpired() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, client := range rl.clients {
		if now.After(client.windowEnd) {
			delete(rl.clients, key)
		}
	}
}

// Close stops the cleanup goroutine.
func (rl *SubscriptionRateLimiter) Close() {
	close(rl.stopCh)
}

// GetRemainingRequests returns the number of remaining requests for a client.
func (rl *SubscriptionRateLimiter) GetRemainingRequests(clientKey string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	client, exists := rl.clients[clientKey]
	if !exists {
		return rl.requestsPerHour
	}

	if time.Now().After(client.windowEnd) {
		return rl.requestsPerHour
	}

	remaining := rl.requestsPerHour - client.requests
	if remaining < 0 {
		return 0
	}
	return remaining
}
