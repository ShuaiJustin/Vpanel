package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 1: Error Response Consistency
// For any API error response, the response SHALL contain a valid JSON object with fields:
// `code` (non-empty string), `message` (non-empty string), `timestamp` (valid ISO 8601 format),
// and optionally `details` (object) and `request_id` (string).
// **Validates: Requirements 2.1, 2.2, 2.3, 2.5**

func TestErrorResponseConsistency(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Generate all possible error codes
	errorCodes := []ErrorCode{
		ErrCodeValidation,
		ErrCodeNotFound,
		ErrCodeUnauthorized,
		ErrCodeForbidden,
		ErrCodeInternal,
		ErrCodeDatabase,
		ErrCodeConfig,
		ErrCodeConflict,
		ErrCodeBadRequest,
		ErrCodeRateLimit,
		ErrCodeCacheError,
		ErrCodeXrayError,
	}

	properties.Property("error responses contain required fields", prop.ForAll(
		func(codeIdx int, message, requestID string) bool {
			code := errorCodes[codeIdx%len(errorCodes)]
			if message == "" {
				message = "test error"
			}

			appErr := New(code, message)
			response := appErr.ToResponse(requestID)

			if response.Success {
				t.Log("Response success should be false for errors")
				return false
			}

			if response.Error == nil {
				t.Log("Response error payload is nil")
				return false
			}

			// Check code is non-empty
			if response.Error.Code == "" {
				t.Log("Response code is empty")
				return false
			}

			// Check message is non-empty
			if response.Error.Message == "" {
				t.Log("Response message is empty")
				return false
			}

			// Check timestamp is valid ISO 8601 format
			if response.Timestamp == "" {
				t.Log("Response timestamp is empty")
				return false
			}
			_, err := time.Parse(time.RFC3339, response.Timestamp)
			if err != nil {
				t.Logf("Response timestamp is not valid RFC3339: %s", response.Timestamp)
				return false
			}

			// Check request_id matches input
			if response.RequestID != requestID {
				t.Logf("Request ID mismatch: expected %q, got %q", requestID, response.RequestID)
				return false
			}

			return true
		},
		gen.IntRange(0, len(errorCodes)-1),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestErrorResponseWithValidationDetails(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("validation errors contain field-specific details", prop.ForAll(
		func(fieldName, fieldError, requestID string) bool {
			if fieldName == "" || fieldError == "" {
				return true
			}

			fields := map[string]string{fieldName: fieldError}
			appErr := NewValidationErrorWithFields("Validation failed", fields)
			response := appErr.ToResponse(requestID)

			if response.Error == nil {
				t.Log("Response error payload is nil for validation error")
				return false
			}

			// Check that details contains the validation errors
			if response.Error.Details == nil {
				t.Log("Response details is nil for validation error")
				return false
			}

			validationErrs, ok := response.Error.Details.(*ValidationErrors)
			if !ok {
				t.Logf("Response details is not ValidationErrors: %T", response.Error.Details)
				return false
			}

			if validationErrs.Fields[fieldName] != fieldError {
				t.Logf("Field error mismatch: expected %q, got %q", fieldError, validationErrs.Fields[fieldName])
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestErrorCodeToHTTPStatusMapping(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Define expected mappings
	expectedMappings := map[ErrorCode]int{
		ErrCodeValidation:   400,
		ErrCodeBadRequest:   400,
		ErrCodeNotFound:     404,
		ErrCodeUnauthorized: 401,
		ErrCodeForbidden:    403,
		ErrCodeConflict:     409,
		ErrCodeRateLimit:    429,
		ErrCodeDatabase:     500,
		ErrCodeInternal:     500,
		ErrCodeConfig:       500,
		ErrCodeCacheError:   500,
		ErrCodeXrayError:    500,
	}

	properties.Property("error codes map to correct HTTP status codes", prop.ForAll(
		func(message string) bool {
			if message == "" {
				message = "test"
			}

			for code, expectedStatus := range expectedMappings {
				appErr := New(code, message)
				actualStatus := appErr.HTTPStatus()
				if actualStatus != expectedStatus {
					t.Logf("Code %s: expected status %d, got %d", code, expectedStatus, actualStatus)
					return false
				}
			}
			return true
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestToErrorResponseForNonAppError(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("non-AppError converts to sanitized internal error", prop.ForAll(
		func(errorMsg, requestID string) bool {
			if errorMsg == "" {
				errorMsg = "some error"
			}

			// Create a regular error (not AppError)
			regularErr := fmt.Errorf("%s", errorMsg)
			response := ToErrorResponse(regularErr, requestID)

			if response.Success {
				t.Log("Response success should be false for non-AppError")
				return false
			}

			if response.Error == nil {
				t.Log("Response error payload is nil")
				return false
			}

			// Should return internal error code
			if response.Error.Code != string(ErrCodeInternal) {
				t.Logf("Expected code %s, got %s", ErrCodeInternal, response.Error.Code)
				return false
			}

			// Message should be a generic sanitized message
			expectedMsg := "服务器内部错误"
			if response.Error.Message != expectedMsg {
				t.Logf("Expected message %q, got %q", expectedMsg, response.Error.Message)
				return false
			}

			// Should have valid timestamp
			_, err := time.Parse(time.RFC3339, response.Timestamp)
			if err != nil {
				t.Logf("Invalid timestamp: %s", response.Timestamp)
				return false
			}

			// Request ID should be preserved
			if response.RequestID != requestID {
				t.Logf("Request ID mismatch: expected %q, got %q", requestID, response.RequestID)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

// Property 2: Database Error Context
// For any database operation that fails, the returned error SHALL contain
// contextual information including the operation type and relevant entity identifiers.
// **Validates: Requirements 2.4**

func TestDatabaseErrorContext_HasOperation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("database errors contain operation context", prop.ForAll(
		func(operation string) bool {
			if operation == "" {
				return true
			}

			cause := fmt.Errorf("connection failed")
			err := NewDatabaseError(operation, cause)

			// Check that operation is set
			if err.Operation != operation {
				t.Logf("Operation not set: expected %q, got %q", operation, err.Operation)
				return false
			}

			// Check that error message contains operation
			errMsg := err.Error()
			if !strings.Contains(errMsg, operation) {
				t.Logf("Error message does not contain operation: %s", errMsg)
				return false
			}

			// Check that HasContext returns true
			if !err.HasContext() {
				t.Log("HasContext returned false")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

func TestDatabaseErrorContext_HasEntityInfo(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("database errors with entity contain entity info", prop.ForAll(
		func(operation, entity string, entityID int64) bool {
			if operation == "" || entity == "" {
				return true
			}

			cause := fmt.Errorf("record not found")
			err := NewDatabaseErrorWithEntity(operation, entity, entityID, cause)

			// Check that entity info is set
			if err.Entity != entity {
				t.Logf("Entity not set: expected %q, got %q", entity, err.Entity)
				return false
			}

			if err.EntityID != entityID {
				t.Logf("EntityID not set: expected %v, got %v", entityID, err.EntityID)
				return false
			}

			// Check that error message contains entity info
			errMsg := err.Error()
			if !strings.Contains(errMsg, entity) {
				t.Logf("Error message does not contain entity: %s", errMsg)
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("user", "proxy", "traffic", "protocol"),
		gen.Int64Range(1, 10000),
	))

	properties.TestingRun(t)
}

func TestDatabaseErrorContext_PreservesCause(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("database errors preserve the underlying cause", prop.ForAll(
		func(operation, causeMsg string) bool {
			if operation == "" || causeMsg == "" {
				return true
			}

			cause := fmt.Errorf("%s", causeMsg)
			err := NewDatabaseError(operation, cause)

			// Check that cause is preserved
			if err.Cause == nil {
				t.Log("Cause is nil")
				return false
			}

			if err.Cause.Error() != causeMsg {
				t.Logf("Cause message mismatch: expected %q, got %q", causeMsg, err.Cause.Error())
				return false
			}

			// Check that Unwrap returns the cause
			unwrapped := err.Unwrap()
			if unwrapped == nil || unwrapped.Error() != causeMsg {
				t.Log("Unwrap did not return the cause")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

func TestAppError_HTTPStatus(t *testing.T) {
	tests := []struct {
		code     ErrorCode
		expected int
	}{
		{ErrCodeValidation, 400},
		{ErrCodeBadRequest, 400},
		{ErrCodeNotFound, 404},
		{ErrCodeUnauthorized, 401},
		{ErrCodeForbidden, 403},
		{ErrCodeConflict, 409},
		{ErrCodeRateLimit, 429},
		{ErrCodeDatabase, 500},
		{ErrCodeInternal, 500},
		{ErrCodeConfig, 500},
		{ErrCodeCacheError, 500},
		{ErrCodeXrayError, 500},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			err := New(tt.code, "test")
			if status := err.HTTPStatus(); status != tt.expected {
				t.Errorf("Expected status %d, got %d", tt.expected, status)
			}
		})
	}
}

func TestAppError_HTTPStatusOverride(t *testing.T) {
	err := New(ErrCodeInternal, "test error")
	err.WithHTTPStatus(503)

	if status := err.HTTPStatus(); status != 503 {
		t.Errorf("Expected status 503, got %d", status)
	}
}

func TestAppError_WithContext(t *testing.T) {
	err := New(ErrCodeInternal, "test error")
	err.WithContext("key1", "value1").WithContext("key2", 123)

	if err.Context["key1"] != "value1" {
		t.Errorf("Expected context key1=value1, got %v", err.Context["key1"])
	}
	if err.Context["key2"] != 123 {
		t.Errorf("Expected context key2=123, got %v", err.Context["key2"])
	}
}

func TestAppError_Is(t *testing.T) {
	err1 := New(ErrCodeNotFound, "user not found")
	err2 := New(ErrCodeNotFound, "proxy not found")
	err3 := New(ErrCodeValidation, "invalid input")

	if !errors.Is(err1, err2) {
		t.Error("Expected err1 to match err2 (same code)")
	}
	if errors.Is(err1, err3) {
		t.Error("Expected err1 to not match err3 (different code)")
	}
}

func TestErrorHelpers(t *testing.T) {
	notFoundErr := NewNotFoundError("user", 123)
	if !IsNotFound(notFoundErr) {
		t.Error("Expected IsNotFound to return true")
	}

	validationErr := NewValidationError("invalid email", nil)
	if !IsValidation(validationErr) {
		t.Error("Expected IsValidation to return true")
	}

	unauthorizedErr := NewUnauthorizedError("invalid token")
	if !IsUnauthorized(unauthorizedErr) {
		t.Error("Expected IsUnauthorized to return true")
	}

	dbErr := NewDatabaseError("insert", fmt.Errorf("constraint violation"))
	if !IsDatabase(dbErr) {
		t.Error("Expected IsDatabase to return true")
	}

	rateLimitErr := NewRateLimitError("too many requests")
	if !IsRateLimit(rateLimitErr) {
		t.Error("Expected IsRateLimit to return true")
	}

	cacheErr := NewCacheError("get", fmt.Errorf("connection refused"))
	if !IsCacheError(cacheErr) {
		t.Error("Expected IsCacheError to return true")
	}

	xrayErr := NewXrayError("start", fmt.Errorf("process failed"))
	if !IsXrayError(xrayErr) {
		t.Error("Expected IsXrayError to return true")
	}

	forbiddenErr := NewForbiddenError("access denied")
	if !IsForbidden(forbiddenErr) {
		t.Error("Expected IsForbidden to return true")
	}

	conflictErr := NewConflictError("resource", "name", "test")
	if !IsConflict(conflictErr) {
		t.Error("Expected IsConflict to return true")
	}
}

func TestWrap(t *testing.T) {
	cause := fmt.Errorf("original error")
	wrapped := Wrap(cause, ErrCodeInternal, "wrapped message")

	if wrapped.Cause != cause {
		t.Error("Expected cause to be preserved")
	}
	if wrapped.Code != ErrCodeInternal {
		t.Error("Expected code to be ErrCodeInternal")
	}
	if wrapped.Message != "wrapped message" {
		t.Error("Expected message to be 'wrapped message'")
	}
}
