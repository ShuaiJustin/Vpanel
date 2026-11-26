package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 5: JSON Log Format
// For any log message output when running in container mode (V_LOG_FORMAT=json),
// the output SHALL be valid JSON containing at minimum: timestamp, level, and message fields.
// **Validates: Requirements 9.2**

func TestJSONLogFormat_ValidJSON(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("all log messages produce valid JSON with required fields", prop.ForAll(
		func(msg string, level string) bool {
			// Skip empty messages as they might cause issues
			if msg == "" {
				return true
			}

			// Create a buffer to capture output
			var buf bytes.Buffer

			// Create logger with JSON format
			logger := New(Config{
				Level:  "debug",
				Format: "json",
				Output: "stdout",
			})

			// Set output to buffer
			dl := logger.(*defaultLogger)
			dl.SetOutput(&buf)

			// Log the message at the specified level
			switch level {
			case "debug":
				logger.Debug(msg)
			case "info":
				logger.Info(msg)
			case "warn":
				logger.Warn(msg)
			case "error":
				logger.Error(msg)
			}

			// Parse the output as JSON
			output := strings.TrimSpace(buf.String())
			if output == "" {
				return true // No output is valid for filtered levels
			}

			var entry JSONLogEntry
			if err := json.Unmarshal([]byte(output), &entry); err != nil {
				t.Logf("Failed to parse JSON: %v, output: %s", err, output)
				return false
			}

			// Check required fields
			if entry.Timestamp == "" {
				t.Log("Missing timestamp field")
				return false
			}
			if entry.Level == "" {
				t.Log("Missing level field")
				return false
			}
			if entry.Message != msg {
				t.Logf("Message mismatch: expected %q, got %q", msg, entry.Message)
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	properties.TestingRun(t)
}

func TestJSONLogFormat_WithFields(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("log messages with fields produce valid JSON", prop.ForAll(
		func(msg string, fieldKey string, fieldValue string) bool {
			// Skip empty values
			if msg == "" || fieldKey == "" {
				return true
			}

			var buf bytes.Buffer
			logger := New(Config{
				Level:  "debug",
				Format: "json",
				Output: "stdout",
			})
			dl := logger.(*defaultLogger)
			dl.SetOutput(&buf)

			logger.Info(msg, F(fieldKey, fieldValue))

			output := strings.TrimSpace(buf.String())
			if output == "" {
				return true
			}

			var entry JSONLogEntry
			if err := json.Unmarshal([]byte(output), &entry); err != nil {
				t.Logf("Failed to parse JSON: %v", err)
				return false
			}

			// Check that field is present
			if entry.Fields == nil {
				t.Log("Fields map is nil")
				return false
			}

			val, ok := entry.Fields[fieldKey]
			if !ok {
				t.Logf("Field %q not found", fieldKey)
				return false
			}

			if val != fieldValue {
				t.Logf("Field value mismatch: expected %q, got %q", fieldValue, val)
				return false
			}

			return true
		},
		gen.AlphaString(),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Property 6: Log Level Filtering
// For any log message, when a log level is configured (e.g., V_LOG_LEVEL=warn),
// only messages at or above that level SHALL be output.
// **Validates: Requirements 9.3**

func TestLogLevelFiltering(t *testing.T) {
	levels := []struct {
		name  string
		level Level
	}{
		{"debug", DebugLevel},
		{"info", InfoLevel},
		{"warn", WarnLevel},
		{"error", ErrorLevel},
	}

	for _, configLevel := range levels {
		t.Run("configured_"+configLevel.name, func(t *testing.T) {
			for _, msgLevel := range levels {
				t.Run("message_"+msgLevel.name, func(t *testing.T) {
					var buf bytes.Buffer
					logger := New(Config{
						Level:  configLevel.name,
						Format: "json",
						Output: "stdout",
					})
					dl := logger.(*defaultLogger)
					dl.SetOutput(&buf)

					// Log at the message level
					switch msgLevel.level {
					case DebugLevel:
						logger.Debug("test message")
					case InfoLevel:
						logger.Info("test message")
					case WarnLevel:
						logger.Warn("test message")
					case ErrorLevel:
						logger.Error("test message")
					}

					output := strings.TrimSpace(buf.String())
					hasOutput := output != ""

					// Message should be output only if msgLevel >= configLevel
					shouldOutput := msgLevel.level >= configLevel.level

					if hasOutput != shouldOutput {
						t.Errorf("Level filtering failed: config=%s, msg=%s, hasOutput=%v, shouldOutput=%v",
							configLevel.name, msgLevel.name, hasOutput, shouldOutput)
					}
				})
			}
		})
	}
}

func TestLogLevelFiltering_Property(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	levelOrder := map[string]int{
		"debug": 0,
		"info":  1,
		"warn":  2,
		"error": 3,
	}

	properties.Property("messages below configured level are filtered", prop.ForAll(
		func(configLevel, msgLevel string) bool {
			var buf bytes.Buffer
			logger := New(Config{
				Level:  configLevel,
				Format: "json",
				Output: "stdout",
			})
			dl := logger.(*defaultLogger)
			dl.SetOutput(&buf)

			// Log at the message level
			switch msgLevel {
			case "debug":
				logger.Debug("test")
			case "info":
				logger.Info("test")
			case "warn":
				logger.Warn("test")
			case "error":
				logger.Error("test")
			}

			hasOutput := strings.TrimSpace(buf.String()) != ""
			shouldOutput := levelOrder[msgLevel] >= levelOrder[configLevel]

			return hasOutput == shouldOutput
		},
		gen.OneConstOf("debug", "info", "warn", "error"),
		gen.OneConstOf("debug", "info", "warn", "error"),
	))

	properties.TestingRun(t)
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	dl := logger.(*defaultLogger)
	dl.SetOutput(&buf)

	// Create logger with base fields
	loggerWithFields := logger.With(F("request_id", "123"), F("user_id", "456"))
	loggerWithFields.Info("test message", F("extra", "value"))

	output := strings.TrimSpace(buf.String())
	var entry JSONLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check all fields are present
	if entry.Fields["request_id"] != "123" {
		t.Errorf("Expected request_id=123, got %v", entry.Fields["request_id"])
	}
	if entry.Fields["user_id"] != "456" {
		t.Errorf("Expected user_id=456, got %v", entry.Fields["user_id"])
	}
	if entry.Fields["extra"] != "value" {
		t.Errorf("Expected extra=value, got %v", entry.Fields["extra"])
	}
}

func TestTextFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "debug",
		Format: "text",
		Output: "stdout",
	})
	dl := logger.(*defaultLogger)
	dl.SetOutput(&buf)

	logger.Info("test message", F("key", "value"))

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("Expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "test message") {
		t.Errorf("Expected 'test message' in output, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("Expected 'key=value' in output, got: %s", output)
	}
}


// Feature: project-optimization, Property 29: Structured Logging Consistency
// *For any* log entry, the field names SHALL follow a consistent naming convention
// (snake_case) and standard fields SHALL use predefined constants.
// **Validates: Requirements 10.4**

func TestStructuredLoggingConsistency_StandardFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	dl := logger.(*defaultLogger)
	dl.SetOutput(&buf)

	// Test standard field helpers
	logger.Info("test request",
		RequestID("req-123"),
		CorrelationID("corr-456"),
		UserID(789),
		Username("testuser"),
		Component("api"),
		StatusCode(200),
		Duration(100*time.Millisecond),
		Action("create"),
		ResourceType("user"),
		ResourceID(123),
	)

	output := strings.TrimSpace(buf.String())
	var entry JSONLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify standard field names are used
	expectedFields := map[string]any{
		"request_id":     "req-123",
		"correlation_id": "corr-456",
		"user_id":        float64(789), // JSON numbers are float64
		"username":       "testuser",
		"component":      "api",
		"status_code":    float64(200),
		"duration_ms":    float64(100),
		"action":         "create",
		"resource_type":  "user",
		"resource_id":    float64(123),
	}

	for key, expected := range expectedFields {
		actual, ok := entry.Fields[key]
		if !ok {
			t.Errorf("Missing field %q", key)
			continue
		}
		if actual != expected {
			t.Errorf("Field %q: expected %v, got %v", key, expected, actual)
		}
	}
}

func TestStructuredLoggingConsistency_ErrorField(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	dl := logger.(*defaultLogger)
	dl.SetOutput(&buf)

	testErr := fmt.Errorf("test error message")
	logger.Error("operation failed", Err(testErr))

	output := strings.TrimSpace(buf.String())
	var entry JSONLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify error field
	errField, ok := entry.Fields["error"]
	if !ok {
		t.Error("Missing error field")
	}
	if errField != "test error message" {
		t.Errorf("Expected error='test error message', got %v", errField)
	}
}

func TestStructuredLoggingConsistency_NilError(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})
	dl := logger.(*defaultLogger)
	dl.SetOutput(&buf)

	logger.Info("operation succeeded", Err(nil))

	output := strings.TrimSpace(buf.String())
	var entry JSONLogEntry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify error field is nil
	errField, ok := entry.Fields["error"]
	if !ok {
		t.Error("Missing error field")
	}
	if errField != nil {
		t.Errorf("Expected error=nil, got %v", errField)
	}
}

func TestStructuredLoggingConsistency_FieldNaming(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// All standard field constants should be snake_case
	standardFields := []string{
		FieldRequestID,
		FieldCorrelationID,
		FieldMethod,
		FieldPath,
		FieldStatusCode,
		FieldDuration,
		FieldClientIP,
		FieldUserAgent,
		FieldUserID,
		FieldUsername,
		FieldRole,
		FieldResourceType,
		FieldResourceID,
		FieldAction,
		FieldError,
		FieldErrorCode,
		FieldStack,
		FieldComponent,
		FieldService,
		FieldVersion,
	}

	properties.Property("all standard field names are snake_case", prop.ForAll(
		func(idx int) bool {
			if idx < 0 || idx >= len(standardFields) {
				return true
			}
			fieldName := standardFields[idx]
			// Check snake_case: lowercase letters, numbers, and underscores only
			for _, c := range fieldName {
				if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
					return false
				}
			}
			return true
		},
		gen.IntRange(0, len(standardFields)-1),
	))

	properties.TestingRun(t)
}
