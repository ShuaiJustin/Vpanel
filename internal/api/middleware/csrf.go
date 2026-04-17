// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	csrfTokenHeader = "X-CSRF-Token"
	csrfTokenCookie = "csrf_token"
	csrfTokenLength = 32
	csrfTokenTTL    = 24 * time.Hour
)

type csrfToken struct {
	value     string
	expiresAt time.Time
}

type csrfStore struct {
	mu     sync.RWMutex
	tokens map[string]*csrfToken
}

func newCSRFStore() *csrfStore {
	store := &csrfStore{
		tokens: make(map[string]*csrfToken),
	}
	// Start cleanup goroutine
	go store.cleanup()
	return store
}

func (s *csrfStore) cleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for key, token := range s.tokens {
			if now.After(token.expiresAt) {
				delete(s.tokens, key)
			}
		}
		s.mu.Unlock()
	}
}

func (s *csrfStore) set(sessionID, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tokens[sessionID] = &csrfToken{
		value:     token,
		expiresAt: time.Now().Add(csrfTokenTTL),
	}
}

func (s *csrfStore) get(sessionID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, exists := s.tokens[sessionID]
	if !exists || time.Now().After(token.expiresAt) {
		return "", false
	}
	return token.value, true
}

func (s *csrfStore) delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.tokens, sessionID)
}

var globalCSRFStore = newCSRFStore()

// generateCSRFToken generates a cryptographically secure random token
func generateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// CSRFProtection returns a middleware that provides CSRF protection
// It generates and validates CSRF tokens for state-changing operations
func CSRFProtection() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CSRF check for safe methods (GET, HEAD, OPTIONS)
		if c.Request.Method == http.MethodGet ||
			c.Request.Method == http.MethodHead ||
			c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Skip CSRF check for API endpoints that use token authentication
		// (tokens are not vulnerable to CSRF)
		if strings.HasPrefix(c.Request.URL.Path, "/api/auth/login") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth/register") ||
			strings.HasPrefix(c.Request.URL.Path, "/api/auth/refresh") {
			c.Next()
			return
		}

		// Get session ID from context (set by auth middleware)
		sessionID, exists := c.Get("user_id")
		if !exists {
			// No authenticated session, skip CSRF check
			c.Next()
			return
		}

		sessionIDStr := ""
		switch v := sessionID.(type) {
		case int64:
			sessionIDStr = string(rune(v))
		case string:
			sessionIDStr = v
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid session ID type",
			})
			return
		}

		// Get expected CSRF token from store
		expectedToken, exists := globalCSRFStore.get(sessionIDStr)
		if !exists {
			// Generate new token if none exists
			token, err := generateCSRFToken()
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to generate CSRF token",
				})
				return
			}
			globalCSRFStore.set(sessionIDStr, token)
			expectedToken = token
		}

		// Get provided CSRF token from header
		providedToken := c.GetHeader(csrfTokenHeader)
		if providedToken == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "CSRF token missing",
			})
			return
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(expectedToken), []byte(providedToken)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "CSRF token invalid",
			})
			return
		}

		c.Next()
	}
}

// CSRFTokenProvider returns a middleware that provides CSRF tokens to clients
func CSRFTokenProvider() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get session ID from context
		sessionID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		sessionIDStr := ""
		switch v := sessionID.(type) {
		case int64:
			sessionIDStr = string(rune(v))
		case string:
			sessionIDStr = v
		default:
			c.Next()
			return
		}

		// Get or generate CSRF token
		token, exists := globalCSRFStore.get(sessionIDStr)
		if !exists {
			var err error
			token, err = generateCSRFToken()
			if err != nil {
				c.Next()
				return
			}
			globalCSRFStore.set(sessionIDStr, token)
		}

		// Set token in response header
		c.Header(csrfTokenHeader, token)
		c.Next()
	}
}
