// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type authAttempt struct {
	count     int
	resetTime time.Time
}

type authRateLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*authAttempt
	maxAttempts int
	window time.Duration
}

func newAuthRateLimiter(maxAttempts int, window time.Duration) *authRateLimiter {
	limiter := &authRateLimiter{
		attempts:    make(map[string]*authAttempt),
		maxAttempts: maxAttempts,
		window:      window,
	}
	// Start cleanup goroutine
	go limiter.cleanup()
	return limiter
}

func (l *authRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for key, attempt := range l.attempts {
			if now.After(attempt.resetTime) {
				delete(l.attempts, key)
			}
		}
		l.mu.Unlock()
	}
}

func (l *authRateLimiter) isAllowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	attempt, exists := l.attempts[key]

	if !exists || now.After(attempt.resetTime) {
		// First attempt or window expired
		l.attempts[key] = &authAttempt{
			count:     1,
			resetTime: now.Add(l.window),
		}
		return true
	}

	if attempt.count >= l.maxAttempts {
		// Rate limit exceeded
		return false
	}

	// Increment attempt count
	attempt.count++
	return true
}

func (l *authRateLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

// Global rate limiters for different auth endpoints
var (
	// Login: 5 attempts per 15 minutes per IP
	loginRateLimiter = newAuthRateLimiter(5, 15*time.Minute)
	// Register: 3 attempts per hour per IP
	registerRateLimiter = newAuthRateLimiter(3, 1*time.Hour)
	// Password reset: 3 attempts per hour per IP
	passwordResetRateLimiter = newAuthRateLimiter(3, 1*time.Hour)
)

// AuthRateLimit returns a middleware that rate limits authentication attempts
func AuthRateLimit(endpoint string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		
		// Select appropriate rate limiter
		var limiter *authRateLimiter
		var endpointName string
		
		switch endpoint {
		case "login":
			limiter = loginRateLimiter
			endpointName = "login"
		case "register":
			limiter = registerRateLimiter
			endpointName = "registration"
		case "password-reset":
			limiter = passwordResetRateLimiter
			endpointName = "password reset"
		default:
			// Unknown endpoint, allow through
			c.Next()
			return
		}

		// Check rate limit
		if !limiter.isAllowed(clientIP) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Too many " + endpointName + " attempts. Please try again later.",
			})
			return
		}

		c.Next()

		// If login was successful (status 200), reset the rate limit for this IP
		if endpoint == "login" && c.Writer.Status() == http.StatusOK {
			limiter.reset(clientIP)
		}
	}
}
