// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// AgentDownloadRateLimiter gates the public /api/admin/nodes/agent/download
// endpoint by client IP. The endpoint stays publicly reachable so remote
// deployment scripts can curl the agent binary, but without a limit any
// unauthenticated client can repeatedly pull the ~20MB binary and burn panel
// bandwidth. Per-hour limits match the cadence of legitimate node deployments
// (typically one-off) without degrading that flow.
type AgentDownloadRateLimiter struct {
	mu              sync.Mutex
	clients         map[string]*agentDownloadClient
	requestsPerHour int
	maxClients      int
	cleanupInterval time.Duration
	stopCh          chan struct{}
}

type agentDownloadClient struct {
	requests  int
	windowEnd time.Time
}

// NewAgentDownloadRateLimiter creates a new download rate limiter.
func NewAgentDownloadRateLimiter(requestsPerHour int) *AgentDownloadRateLimiter {
	if requestsPerHour <= 0 {
		requestsPerHour = 12
	}

	rl := &AgentDownloadRateLimiter{
		clients:         make(map[string]*agentDownloadClient),
		requestsPerHour: requestsPerHour,
		maxClients:      5000,
		cleanupInterval: 10 * time.Minute,
		stopCh:          make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// RateLimit returns the gin middleware.
func (rl *AgentDownloadRateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "下载频率过高，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

func (rl *AgentDownloadRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	client, exists := rl.clients[ip]

	if !exists || now.After(client.windowEnd) {
		if !exists && len(rl.clients) >= rl.maxClients {
			for key, c := range rl.clients {
				if now.After(c.windowEnd) {
					delete(rl.clients, key)
				}
			}
		}
		rl.clients[ip] = &agentDownloadClient{
			requests:  1,
			windowEnd: now.Add(time.Hour),
		}
		return true
	}

	if client.requests >= rl.requestsPerHour {
		return false
	}
	client.requests++
	return true
}

func (rl *AgentDownloadRateLimiter) cleanup() {
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
func (rl *AgentDownloadRateLimiter) Close() {
	close(rl.stopCh)
}
