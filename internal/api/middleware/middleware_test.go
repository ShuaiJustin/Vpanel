package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/logger"
)

// Property 7: Error Logging with Request Context
// For any error that occurs during request processing, the error log SHALL contain
// the request ID, HTTP method, path, and error details.
// **Validates: Requirements 9.5**

func init() {
	gin.SetMode(gin.TestMode)
}

type testLogger struct {
	output *bytes.Buffer
	level  logger.Level
}

func newTestLogger() *testLogger {
	return &testLogger{output: &bytes.Buffer{}, level: logger.DebugLevel}
}

func (l *testLogger) Debug(msg string, fields ...logger.Field)  { l.log("debug", msg, fields...) }
func (l *testLogger) Info(msg string, fields ...logger.Field)   { l.log("info", msg, fields...) }
func (l *testLogger) Warn(msg string, fields ...logger.Field)   { l.log("warn", msg, fields...) }
func (l *testLogger) Error(msg string, fields ...logger.Field)  { l.log("error", msg, fields...) }
func (l *testLogger) Fatal(msg string, fields ...logger.Field)  { l.log("fatal", msg, fields...) }
func (l *testLogger) With(fields ...logger.Field) logger.Logger { return l }
func (l *testLogger) SetLevel(level logger.Level)               { l.level = level }
func (l *testLogger) GetLevel() logger.Level                    { return l.level }

func (l *testLogger) log(level, msg string, fields ...logger.Field) {
	entry := map[string]any{"level": level, "message": msg}
	if len(fields) > 0 {
		fieldsMap := make(map[string]any)
		for _, f := range fields {
			fieldsMap[f.Key] = f.Value
		}
		entry["fields"] = fieldsMap
	}
	data, _ := json.Marshal(entry)
	l.output.Write(data)
	l.output.WriteString("\n")
}

func (l *testLogger) getOutput() string { return l.output.String() }

func TestErrorLoggingWithRequestContext(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("error logs contain request context fields", prop.ForAll(
		func(method, path string) bool {
			if path == "" || !strings.HasPrefix(path, "/") {
				return true
			}

			testLog := newTestLogger()
			router := gin.New()
			router.Use(RequestID())
			router.Use(Logger(testLog))

			router.Handle(method, path, func(c *gin.Context) {
				c.Error(&gin.Error{Err: http.ErrBodyNotAllowed, Type: gin.ErrorTypePrivate})
				c.JSON(http.StatusInternalServerError, gin.H{"error": "test error"})
			})

			req := httptest.NewRequest(method, path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			output := testLog.getOutput()
			var logEntry map[string]any
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 0 {
				return false
			}

			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &logEntry); err != nil {
				return false
			}

			fields, ok := logEntry["fields"].(map[string]any)
			if !ok {
				return false
			}

			if _, ok := fields["request_id"]; !ok {
				return false
			}
			if m, ok := fields["method"]; !ok || m != method {
				return false
			}
			if p, ok := fields["path"]; !ok || p != path {
				return false
			}

			return true
		},
		gen.OneConstOf("GET", "POST", "PUT", "DELETE"),
		gen.OneConstOf("/api/test", "/api/users", "/api/proxies", "/health"),
	))

	properties.TestingRun(t)
}

func TestErrorLoggingWithRequestContext_ErrorsIncluded(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("error details are included in logs", prop.ForAll(
		func(errorMsg string) bool {
			if errorMsg == "" {
				return true
			}

			testLog := newTestLogger()
			router := gin.New()
			router.Use(RequestID())
			router.Use(Logger(testLog))

			router.GET("/test", func(c *gin.Context) {
				c.Error(&gin.Error{Err: http.ErrBodyNotAllowed, Meta: errorMsg, Type: gin.ErrorTypePrivate})
				c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			output := testLog.getOutput()
			var logEntry map[string]any
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 0 {
				return false
			}

			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &logEntry); err != nil {
				return false
			}

			fields, ok := logEntry["fields"].(map[string]any)
			if !ok {
				return false
			}

			if _, ok := fields["errors"]; !ok {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

func TestErrorLoggingWithRequestContext_RequestIDPropagated(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("request ID is propagated and logged", prop.ForAll(
		func(customRequestID string) bool {
			if customRequestID == "" {
				return true
			}

			testLog := newTestLogger()
			router := gin.New()
			router.Use(RequestID())
			router.Use(Logger(testLog))

			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-Request-ID", customRequestID)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Header().Get("X-Request-ID") != customRequestID {
				return false
			}

			output := testLog.getOutput()
			var logEntry map[string]any
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 0 {
				return false
			}

			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &logEntry); err != nil {
				return false
			}

			fields, ok := logEntry["fields"].(map[string]any)
			if !ok {
				return false
			}

			if rid, ok := fields["request_id"]; !ok || rid != customRequestID {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

func TestErrorLoggingWithRequestContext_LogLevelBasedOnStatus(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("log level matches HTTP status severity", prop.ForAll(
		func(status int) bool {
			if status < 200 || status > 599 {
				return true
			}

			testLog := newTestLogger()
			router := gin.New()
			router.Use(RequestID())
			router.Use(Logger(testLog))

			router.GET("/test", func(c *gin.Context) {
				c.Status(status)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			output := testLog.getOutput()
			var logEntry map[string]any
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) == 0 {
				return false
			}

			if err := json.Unmarshal([]byte(lines[len(lines)-1]), &logEntry); err != nil {
				return false
			}

			level, ok := logEntry["level"].(string)
			if !ok {
				return false
			}

			if status >= 500 && level != "error" {
				return false
			}
			if status >= 400 && status < 500 && level != "warn" {
				return false
			}
			if status < 400 && level != "info" {
				return false
			}

			return true
		},
		gen.IntRange(200, 599),
	))

	properties.TestingRun(t)
}

func TestShouldPersistHTTPRequestLog(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
		want   bool
	}{
		{name: "health is skipped", method: "HEAD", path: "/health", status: http.StatusOK, want: false},
		{name: "heartbeat is skipped", method: "POST", path: "/api/node/heartbeat", status: http.StatusOK, want: false},
		{name: "node register is skipped", method: "POST", path: "/api/node/register", status: http.StatusOK, want: false},
		{name: "sse is skipped", method: "GET", path: "/api/sse/xray-events", status: http.StatusOK, want: false},
		{name: "assets are skipped", method: "GET", path: "/assets/js/index.js", status: http.StatusOK, want: false},
		{name: "favicon is skipped", method: "GET", path: "/favicon.svg", status: http.StatusOK, want: false},
		{name: "successful reads skip db persistence", method: "GET", path: "/api/logs", status: http.StatusOK, want: false},
		{name: "successful admin reads skip db persistence", method: "GET", path: "/api/admin/nodes", status: http.StatusOK, want: false},
		{name: "mutating requests are kept", method: "POST", path: "/api/auth/login", status: http.StatusOK, want: true},
		{name: "errors are always kept", method: "GET", path: "/assets/js/index.js", status: http.StatusInternalServerError, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPersistHTTPRequestLog(tc.method, tc.path, tc.status)
			if got != tc.want {
				t.Fatalf("shouldPersistHTTPRequestLog(%q, %q, %d) = %v, want %v", tc.method, tc.path, tc.status, got, tc.want)
			}
		})
	}
}

// Property 28: Correlation ID Propagation
// For any API request, a unique correlation ID SHALL be generated and included
// in all log entries and error responses related to that request.
// **Validates: Requirements 10.3**

func TestCorrelationIDPropagation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("correlation ID is generated when not provided", prop.ForAll(
		func(path string) bool {
			if path == "" || !strings.HasPrefix(path, "/") {
				return true
			}

			router := gin.New()
			router.Use(RequestIDMiddleware())

			var capturedRequestID, capturedCorrelationID string
			router.GET(path, func(c *gin.Context) {
				capturedRequestID = GetRequestID(c)
				capturedCorrelationID = GetCorrelationID(c)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Request ID should be generated
			if capturedRequestID == "" {
				t.Log("Request ID was not generated")
				return false
			}

			// Correlation ID should equal request ID when not provided
			if capturedCorrelationID != capturedRequestID {
				t.Logf("Correlation ID (%s) should equal Request ID (%s) when not provided",
					capturedCorrelationID, capturedRequestID)
				return false
			}

			// Response headers should contain both IDs
			if w.Header().Get(RequestIDHeader) != capturedRequestID {
				t.Log("Request ID not in response header")
				return false
			}
			if w.Header().Get(CorrelationIDHeader) != capturedCorrelationID {
				t.Log("Correlation ID not in response header")
				return false
			}

			return true
		},
		gen.OneConstOf("/api/test", "/api/users", "/api/proxies", "/health"),
	))

	properties.TestingRun(t)
}

func TestCorrelationIDPropagation_CustomIDs(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("custom request and correlation IDs are preserved", prop.ForAll(
		func(requestID, correlationID string) bool {
			router := gin.New()
			router.Use(RequestIDMiddleware())

			var capturedRequestID, capturedCorrelationID string
			router.GET("/test", func(c *gin.Context) {
				capturedRequestID = GetRequestID(c)
				capturedCorrelationID = GetCorrelationID(c)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(RequestIDHeader, requestID)
			req.Header.Set(CorrelationIDHeader, correlationID)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Custom request ID should be preserved
			if capturedRequestID != requestID {
				t.Logf("Request ID not preserved: expected %s, got %s", requestID, capturedRequestID)
				return false
			}

			// Custom correlation ID should be preserved
			if capturedCorrelationID != correlationID {
				t.Logf("Correlation ID not preserved: expected %s, got %s", correlationID, capturedCorrelationID)
				return false
			}

			// Response headers should contain the custom IDs
			if w.Header().Get(RequestIDHeader) != requestID {
				t.Log("Custom Request ID not in response header")
				return false
			}
			if w.Header().Get(CorrelationIDHeader) != correlationID {
				t.Log("Custom Correlation ID not in response header")
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.Identifier(),
	))

	properties.TestingRun(t)
}

func TestCorrelationIDPropagation_InContext(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("correlation ID is available in request context", prop.ForAll(
		func(correlationID string) bool {
			if correlationID == "" {
				return true
			}

			router := gin.New()
			router.Use(RequestIDMiddleware())

			var contextRequestID, contextCorrelationID string
			router.GET("/test", func(c *gin.Context) {
				// Get from standard context
				contextRequestID = GetRequestIDFromContext(c.Request.Context())
				contextCorrelationID = GetCorrelationIDFromContext(c.Request.Context())
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(CorrelationIDHeader, correlationID)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Request ID should be in context
			if contextRequestID == "" {
				t.Log("Request ID not in context")
				return false
			}

			// Correlation ID should be in context
			if contextCorrelationID != correlationID {
				t.Logf("Correlation ID not in context: expected %s, got %s", correlationID, contextCorrelationID)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

func TestRequestContextExtraction(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("request context contains all expected fields", prop.ForAll(
		func(requestID, userAgent string) bool {
			if requestID == "" || userAgent == "" {
				return true
			}

			router := gin.New()
			router.Use(RequestIDMiddleware())

			var rc *RequestContext
			router.GET("/test", func(c *gin.Context) {
				rc = GetRequestContext(c)
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(RequestIDHeader, requestID)
			req.Header.Set("User-Agent", userAgent)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if rc == nil {
				t.Log("RequestContext is nil")
				return false
			}

			// Check all fields
			if rc.RequestID != requestID {
				t.Logf("RequestID mismatch: expected %s, got %s", requestID, rc.RequestID)
				return false
			}
			if rc.CorrelationID != requestID { // Should equal request ID when not provided separately
				t.Logf("CorrelationID mismatch: expected %s, got %s", requestID, rc.CorrelationID)
				return false
			}
			if rc.UserAgent != userAgent {
				t.Logf("UserAgent mismatch: expected %s, got %s", userAgent, rc.UserAgent)
				return false
			}
			if rc.Path != "/test" {
				t.Logf("Path mismatch: expected /test, got %s", rc.Path)
				return false
			}
			if rc.Method != "GET" {
				t.Logf("Method mismatch: expected GET, got %s", rc.Method)
				return false
			}

			// Check ToMap conversion
			m := rc.ToMap()
			if m["request_id"] != requestID {
				t.Log("ToMap request_id mismatch")
				return false
			}
			if m["correlation_id"] != requestID {
				t.Log("ToMap correlation_id mismatch")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 100 }),
	))

	properties.TestingRun(t)
}
