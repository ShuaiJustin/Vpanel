// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AgentRateLimiter provides rate limiting for node agent endpoints.
// Limits requests per IP to prevent brute-force attacks on token-based auth.
type AgentRateLimiter struct {
	mu              sync.Mutex
	clients         map[string]*agentClient
	requestsPerMin  int
	maxClients      int
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type agentClient struct {
	requests  int
	windowEnd time.Time
}

// NewAgentRateLimiter creates a new agent rate limiter.
// requestsPerMin specifies the maximum requests per minute per IP.
func NewAgentRateLimiter(requestsPerMin int) *AgentRateLimiter {
	if requestsPerMin <= 0 {
		requestsPerMin = 30
	}

	rl := &AgentRateLimiter{
		clients:         make(map[string]*agentClient),
		requestsPerMin:  requestsPerMin,
		maxClients:      5000,
		cleanupInterval: 2 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// RateLimit returns a gin middleware that limits agent API access.
func (rl *AgentRateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		if !rl.allow(ip) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

func (rl *AgentRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, exists := rl.clients[ip]

	if !exists || now.After(client.windowEnd) {
		if !exists && len(rl.clients) >= rl.maxClients {
			// Evict expired entries
			for key, c := range rl.clients {
				if now.After(c.windowEnd) {
					delete(rl.clients, key)
				}
			}
		}

		rl.clients[ip] = &agentClient{
			requests:  1,
			windowEnd: now.Add(time.Minute),
		}
		return true
	}

	if client.requests >= rl.requestsPerMin {
		return false
	}

	client.requests++
	return true
}

func (rl *AgentRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, client := range rl.clients {
				if now.After(client.windowEnd) {
					delete(rl.clients, key)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

// Close stops the cleanup goroutine.
func (rl *AgentRateLimiter) Close() {
	close(rl.stopCh)
}

// MaxBodySize returns a middleware that limits request body size.
func MaxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}
