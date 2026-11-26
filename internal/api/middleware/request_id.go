// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDKey is the context key for request ID.
	RequestIDKey ContextKey = "request_id"
	// CorrelationIDKey is the context key for correlation ID.
	CorrelationIDKey ContextKey = "correlation_id"
)

// RequestIDHeader is the HTTP header name for request ID.
const RequestIDHeader = "X-Request-ID"

// CorrelationIDHeader is the HTTP header name for correlation ID.
const CorrelationIDHeader = "X-Correlation-ID"

// RequestIDMiddleware returns a middleware that adds request ID and correlation ID to the context.
// It generates a new request ID if not provided in the header.
// Correlation ID is used for distributed tracing across services.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get or generate request ID
		requestID := c.GetHeader(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Get or use request ID as correlation ID
		correlationID := c.GetHeader(CorrelationIDHeader)
		if correlationID == "" {
			correlationID = requestID
		}

		// Set in Gin context
		c.Set(string(RequestIDKey), requestID)
		c.Set(string(CorrelationIDKey), correlationID)

		// Set in response headers
		c.Header(RequestIDHeader, requestID)
		c.Header(CorrelationIDHeader, correlationID)

		// Create a new context with the IDs for downstream use
		ctx := context.WithValue(c.Request.Context(), RequestIDKey, requestID)
		ctx = context.WithValue(ctx, CorrelationIDKey, correlationID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetRequestID retrieves the request ID from the Gin context.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(string(RequestIDKey)); exists {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}

// GetCorrelationID retrieves the correlation ID from the Gin context.
func GetCorrelationID(c *gin.Context) string {
	if id, exists := c.Get(string(CorrelationIDKey)); exists {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}

// GetRequestIDFromContext retrieves the request ID from a standard context.
func GetRequestIDFromContext(ctx context.Context) string {
	if id := ctx.Value(RequestIDKey); id != nil {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}

// GetCorrelationIDFromContext retrieves the correlation ID from a standard context.
func GetCorrelationIDFromContext(ctx context.Context) string {
	if id := ctx.Value(CorrelationIDKey); id != nil {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}

// WithRequestID adds a request ID to a context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithCorrelationID adds a correlation ID to a context.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, correlationID)
}

// RequestContext holds request-scoped information for logging and tracing.
type RequestContext struct {
	RequestID     string
	CorrelationID string
	UserID        int64
	Username      string
	IP            string
	UserAgent     string
	Path          string
	Method        string
}

// GetRequestContext extracts request context from a Gin context.
func GetRequestContext(c *gin.Context) *RequestContext {
	rc := &RequestContext{
		RequestID:     GetRequestID(c),
		CorrelationID: GetCorrelationID(c),
		IP:            c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		Path:          c.Request.URL.Path,
		Method:        c.Request.Method,
	}

	// Try to get user info if authenticated
	if userID, exists := c.Get("user_id"); exists {
		if id, ok := userID.(int64); ok {
			rc.UserID = id
		}
	}
	if username, exists := c.Get("username"); exists {
		if name, ok := username.(string); ok {
			rc.Username = name
		}
	}

	return rc
}

// ToMap converts RequestContext to a map for logging.
func (rc *RequestContext) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"request_id":     rc.RequestID,
		"correlation_id": rc.CorrelationID,
		"ip":             rc.IP,
		"user_agent":     rc.UserAgent,
		"path":           rc.Path,
		"method":         rc.Method,
	}
	if rc.UserID != 0 {
		m["user_id"] = rc.UserID
	}
	if rc.Username != "" {
		m["username"] = rc.Username
	}
	return m
}
