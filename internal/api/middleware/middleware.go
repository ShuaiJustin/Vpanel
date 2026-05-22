// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	logservice "v/internal/log"
	"v/internal/logger"
)

// Recovery returns a middleware that recovers from panics.
func Recovery(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				log.Error("panic recovered",
					logger.F("error", err),
					logger.F("stack", stack),
					logger.F("path", c.Request.URL.Path),
					logger.F("method", c.Request.Method),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// Logger returns a middleware that logs requests.
func Logger(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []logger.Field{
			logger.F("status", status),
			logger.F("method", c.Request.Method),
			logger.F("path", path),
			logger.F("latency", latency.String()),
			logger.F("ip", c.ClientIP()),
			logger.F("user_agent", c.Request.UserAgent()),
		}

		if query != "" {
			fields = append(fields, logger.F("query", query))
		}

		if requestID := c.GetString("request_id"); requestID != "" {
			fields = append(fields, logger.F("request_id", requestID))
		}

		if len(c.Errors) > 0 {
			fields = append(fields, logger.F("errors", c.Errors.String()))
		}

		if shouldLogNodeAuthFailureAsDebug(path, status) {
			log.Debug("request completed", fields...)
		} else if status >= 500 {
			log.Error("request completed", fields...)
		} else if status >= 400 {
			log.Warn("request completed", fields...)
		} else {
			log.Info("request completed", fields...)
		}
	}
}

// LoggerWithService returns a middleware that logs requests to both console and database.
func LoggerWithService(log logger.Logger, logService *logservice.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		requestID := c.GetString("request_id")

		// Console logging fields
		fields := []logger.Field{
			logger.F("status", status),
			logger.F("method", c.Request.Method),
			logger.F("path", path),
			logger.F("latency", latency.String()),
			logger.F("ip", c.ClientIP()),
		}

		// Only log user agent for errors to reduce log noise
		if status >= 400 {
			fields = append(fields, logger.F("user_agent", c.Request.UserAgent()))
		}

		if query != "" && status >= 400 {
			// Only log query params on errors to avoid logging sensitive data
			fields = append(fields, logger.F("query", query))
		}

		if requestID != "" {
			fields = append(fields, logger.F("request_id", requestID))
		}

		if len(c.Errors) > 0 {
			fields = append(fields, logger.F("errors", c.Errors.String()))
		}

		// Determine log level based on status
		var level string
		if shouldLogNodeAuthFailureAsDebug(path, status) {
			level = "debug"
			log.Debug("request completed", fields...)
		} else if status >= 500 {
			level = "error"
			log.Error("request completed", fields...)
		} else if status >= 400 {
			level = "warn"
			log.Warn("request completed", fields...)
		} else {
			level = "info"
			log.Info("request completed", fields...)
		}

		// Log to database if service is available AND access log persistence
		// is enabled in settings (admin can disable it to reduce DB write load).
		if logService != nil && logService.AccessLogEnabled() && shouldPersistHTTPRequestLog(c.Request.Method, path, status) {
			// Get user ID from context if available
			var userID *int64
			if uid, exists := c.Get("user_id"); exists {
				if id, ok := uid.(int64); ok {
					userID = &id
				}
			}

			// Build extra fields for database
			extraFields := map[string]interface{}{
				"status":  status,
				"method":  c.Request.Method,
				"latency": latency.Milliseconds(),
			}

			if query != "" {
				extraFields["query"] = query
			}

			if len(c.Errors) > 0 {
				extraFields["errors"] = c.Errors.String()
			}

			// Add context fields
			if userID != nil {
				extraFields["user_id"] = *userID
			}
			extraFields["ip"] = c.ClientIP()
			extraFields["user_agent"] = c.Request.UserAgent()
			extraFields["request_id"] = requestID

			// Log asynchronously (non-blocking)
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = logService.Log(ctx, level, "request completed: "+c.Request.Method+" "+path, "http", extraFields)
			}()

		}
	}
}

func shouldPersistHTTPRequestLog(method, requestPath string, status int) bool {
	if status >= 400 {
		return true
	}

	normalizedMethod := strings.ToUpper(strings.TrimSpace(method))
	switch normalizedMethod {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		// High-frequency read traffic is already visible in structured stdout logs.
		// Skip DB persistence to avoid turning dashboards and polling pages into write amplification.
		return false
	}

	normalizedPath := strings.TrimSpace(requestPath)
	if normalizedPath == "" {
		return false
	}

	if normalizedPath == "/health" {
		return false
	}
	if normalizedPath == "/favicon.ico" || normalizedPath == "/favicon.svg" {
		return false
	}
	if strings.HasPrefix(normalizedPath, "/assets/") {
		return false
	}
	if strings.HasPrefix(normalizedPath, "/api/sse/") {
		return false
	}
	if normalizedPath == "/api/node/heartbeat" || normalizedPath == "/api/node/register" {
		return false
	}

	ext := strings.ToLower(path.Ext(normalizedPath))
	switch ext {
	case ".js", ".css", ".map", ".png", ".jpg", ".jpeg", ".svg", ".ico", ".webp", ".woff", ".woff2", ".ttf":
		return false
	}

	return true
}

func shouldLogNodeAuthFailureAsDebug(requestPath string, status int) bool {
	if status != http.StatusUnauthorized {
		return false
	}

	normalizedPath := strings.TrimSpace(requestPath)
	return normalizedPath == "/api/node/register" || normalizedPath == "/api/node/heartbeat"
}

// CORS returns a middleware that handles CORS.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Same-origin requests don't send an Origin header (browsers omit it
		// for navigation/server-side fetches). They are always allowed —
		// only cross-origin requests with an Origin header are subject to
		// the allowlist.
		//
		// Browsers DO send Origin for module scripts, fetch(), preload, etc.
		// even on same-origin requests. Treat those as same-origin too by
		// comparing the Origin host(:port) with the request Host header;
		// otherwise the panel would 403 its own JS/CSS whenever the host
		// (LAN IP, public IP, alt domain) isn't in V_SERVER_CORS_ORIGINS.
		if origin == "" || isSameOrigin(origin, c.Request.Host) {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Max-Age", "86400")
			if c.Request.Method == "OPTIONS" {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		// Check if origin is allowed
		allowed := false
		for _, o := range allowedOrigins {
			if o == "*" {
				// Only allow * in development
				allowed = true
				c.Header("Access-Control-Allow-Origin", "*")
				break
			} else if o == origin {
				allowed = true
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}

		// Empty allowlist = allow all (legacy/dev behavior). Set explicitly
		// in production via V_SERVER_CORS_ORIGINS.
		if !allowed && len(allowedOrigins) == 0 {
			allowed = true
			c.Header("Access-Control-Allow-Origin", origin)
		}

		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "FORBIDDEN",
				"message": "Origin not allowed",
			})
			return
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isSameOrigin reports whether the Origin header value targets the same
// host:port as the request itself. Browsers attach Origin to same-origin
// module-script / fetch / preload requests, and rejecting those with 403
// would 白屏 the panel whenever the host isn't explicitly whitelisted.
func isSameOrigin(origin, requestHost string) bool {
	if origin == "" || requestHost == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, requestHost)
}

// RequestID returns a middleware that adds a request ID to the context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// RateLimit returns a simple rate limiting middleware.
func RateLimit(requestsPerSecond int) gin.HandlerFunc {
	// Simple token bucket implementation with memory limit
	type client struct {
		tokens    float64
		lastCheck time.Time
	}
	clients := make(map[string]*client)
	rate := float64(requestsPerSecond)
	maxClients := 10000 // Prevent memory exhaustion

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		// Evict old entries if map is too large
		if len(clients) >= maxClients {
			for k, v := range clients {
				if now.Sub(v.lastCheck) > 5*time.Minute {
					delete(clients, k)
					if len(clients) < maxClients*9/10 {
						break
					}
				}
			}
		}

		cl, exists := clients[ip]
		if !exists {
			cl = &client{tokens: rate, lastCheck: now}
			clients[ip] = cl
		}

		// Refill tokens
		elapsed := now.Sub(cl.lastCheck).Seconds()
		cl.tokens += elapsed * rate
		if cl.tokens > rate {
			cl.tokens = rate
		}
		cl.lastCheck = now

		if cl.tokens < 1 {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "Too many requests, please try again later",
			})
			return
		}

		cl.tokens--
		c.Next()
	}
}

// SecureHeaders returns a middleware that adds security headers.
func SecureHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}

// ContentType returns a middleware that validates content type.
func ContentType(contentTypes ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "GET" || c.Request.Method == "DELETE" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		ct := c.ContentType()
		for _, allowed := range contentTypes {
			if strings.HasPrefix(ct, allowed) {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
			"error": "Unsupported content type",
		})
	}
}
