// Package errors provides structured error types for the V Panel application.
// It supports error codes, contextual information, and error wrapping.
package errors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ErrorCode represents an error code type.
type ErrorCode string

// Error codes
const (
	ErrCodeValidation   ErrorCode = "VALIDATION_ERROR"
	ErrCodeNotFound     ErrorCode = "NOT_FOUND"
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
	ErrCodeInternal     ErrorCode = "INTERNAL_ERROR"
	ErrCodeDatabase     ErrorCode = "DATABASE_ERROR"
	ErrCodeConfig       ErrorCode = "CONFIG_ERROR"
	ErrCodeConflict     ErrorCode = "CONFLICT"
	ErrCodeBadRequest   ErrorCode = "BAD_REQUEST"
	ErrCodeRateLimit    ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeCacheError   ErrorCode = "CACHE_ERROR"
	ErrCodeXrayError    ErrorCode = "XRAY_ERROR"
)

// AppError represents an application error with code, message, and context.
type AppError struct {
	Code       ErrorCode      `json:"code"`
	Message    string         `json:"message"`
	Details    any            `json:"details,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"` // 用户友好的建议
	Context    map[string]any `json:"context,omitempty"`
	Cause      error          `json:"-"`
	Operation  string         `json:"-"` // The operation that failed
	Entity     string         `json:"-"` // The entity involved (e.g., "user", "proxy")
	EntityID   any            `json:"-"` // The entity ID if applicable
	HTTPCode   int            `json:"-"` // Custom HTTP status code override
}

// ErrorResponse represents the standardized API error response format.
// This is the structure returned to API clients.
type ErrorResponse struct {
	Success   bool           `json:"success"`
	Error     *ErrorDetail   `json:"error"`
	RequestID string         `json:"request_id,omitempty"`
	Timestamp string         `json:"timestamp"`
}

// ErrorDetail contains detailed error information.
type ErrorDetail struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Details    any            `json:"details,omitempty"`
	Suggestion string         `json:"suggestion,omitempty"` // 用户友好的建议
}

// FieldError represents a validation error for a specific field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationErrors holds multiple field validation errors.
type ValidationErrors struct {
	Fields map[string]string `json:"fields"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	msg := fmt.Sprintf("%s: %s", e.Code, e.Message)
	if e.Operation != "" {
		msg = fmt.Sprintf("%s (operation: %s)", msg, e.Operation)
	}
	if e.Entity != "" {
		msg = fmt.Sprintf("%s (entity: %s", msg, e.Entity)
		if e.EntityID != nil {
			msg = fmt.Sprintf("%s, id: %v", msg, e.EntityID)
		}
		msg += ")"
	}
	if e.Cause != nil {
		msg = fmt.Sprintf("%s: %v", msg, e.Cause)
	}
	return msg
}

// Unwrap returns the underlying error.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target.
func (e *AppError) Is(target error) bool {
	t, ok := target.(*AppError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// HTTPStatus returns the appropriate HTTP status code for the error.
func (e *AppError) HTTPStatus() int {
	// Allow custom HTTP status code override
	if e.HTTPCode != 0 {
		return e.HTTPCode
	}
	switch e.Code {
	case ErrCodeValidation, ErrCodeBadRequest:
		return http.StatusBadRequest
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeDatabase, ErrCodeInternal, ErrCodeConfig, ErrCodeCacheError, ErrCodeXrayError:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// ToResponse converts AppError to ErrorResponse for API responses.
func (e *AppError) ToResponse(requestID string) *ErrorResponse {
	return &ErrorResponse{
		Success: false,
		Error: &ErrorDetail{
			Code:       string(e.Code),
			Message:    e.Message,
			Details:    e.Details,
			Suggestion: e.getSuggestion(),
		},
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// getSuggestion returns a user-friendly suggestion based on the error code.
func (e *AppError) getSuggestion() string {
	if e.Suggestion != "" {
		return e.Suggestion
	}
	
	switch e.Code {
	case ErrCodeUnauthorized:
		return "请重新登录后再试"
	case ErrCodeForbidden:
		return "您没有权限执行此操作，请联系管理员"
	case ErrCodeNotFound:
		return "请检查资源是否存在"
	case ErrCodeValidation, ErrCodeBadRequest:
		return "请检查输入的参数是否正确"
	case ErrCodeConflict:
		return "资源已存在，请使用其他名称"
	case ErrCodeRateLimit:
		return "请求过于频繁，请稍后再试"
	case ErrCodeInternal, ErrCodeDatabase:
		return "服务器遇到问题，请稍后重试或联系管理员"
	default:
		return "请稍后重试，如问题持续请联系管理员"
	}
}

// WithSuggestion sets a custom suggestion message.
func (e *AppError) WithSuggestion(suggestion string) *AppError {
	e.Suggestion = suggestion
	return e
}

// WithHTTPStatus sets a custom HTTP status code.
func (e *AppError) WithHTTPStatus(status int) *AppError {
	e.HTTPCode = status
	return e
}

// WithContext adds context to the error.
func (e *AppError) WithContext(key string, value any) *AppError {
	if e.Context == nil {
		e.Context = make(map[string]any)
	}
	e.Context[key] = value
	return e
}

// WithOperation sets the operation that failed.
func (e *AppError) WithOperation(op string) *AppError {
	e.Operation = op
	return e
}

// WithEntity sets the entity information.
func (e *AppError) WithEntity(entity string, id any) *AppError {
	e.Entity = entity
	e.EntityID = id
	return e
}

// HasContext checks if the error has context information.
func (e *AppError) HasContext() bool {
	return e.Operation != "" || e.Entity != "" || len(e.Context) > 0
}

// New creates a new AppError.
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Wrap wraps an error with an AppError.
func Wrap(err error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Cause:   err,
	}
}

// NewValidationError creates a validation error.
func NewValidationError(message string, details any) *AppError {
	return &AppError{
		Code:    ErrCodeValidation,
		Message: message,
		Details: details,
	}
}

// NewNotFoundError creates a not found error.
func NewNotFoundError(entity string, id any) *AppError {
	return &AppError{
		Code:     ErrCodeNotFound,
		Message:  fmt.Sprintf("%s not found", entity),
		Entity:   entity,
		EntityID: id,
	}
}

// NewUnauthorizedError creates an unauthorized error.
func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeUnauthorized,
		Message: message,
	}
}

// NewForbiddenError creates a forbidden error.
func NewForbiddenError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeForbidden,
		Message: message,
	}
}

// NewInternalError creates an internal error.
func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Code:    ErrCodeInternal,
		Message: message,
		Cause:   cause,
	}
}

// NewDatabaseError creates a database error with operation context.
func NewDatabaseError(operation string, cause error) *AppError {
	return &AppError{
		Code:      ErrCodeDatabase,
		Message:   fmt.Sprintf("database operation failed: %s", operation),
		Operation: operation,
		Cause:     cause,
	}
}

// NewDatabaseErrorWithEntity creates a database error with entity context.
func NewDatabaseErrorWithEntity(operation string, entity string, entityID any, cause error) *AppError {
	return &AppError{
		Code:      ErrCodeDatabase,
		Message:   fmt.Sprintf("database operation failed: %s", operation),
		Operation: operation,
		Entity:    entity,
		EntityID:  entityID,
		Cause:     cause,
	}
}

// NewConfigError creates a configuration error.
func NewConfigError(message string, cause error) *AppError {
	return &AppError{
		Code:    ErrCodeConfig,
		Message: message,
		Cause:   cause,
	}
}

// NewConflictError creates a conflict error.
func NewConflictError(entity, field string, value any) *AppError {
	return &AppError{
		Code:    ErrCodeConflict,
		Message: fmt.Sprintf("%s with %s '%v' already exists", entity, field, value),
		Entity:  entity,
		Details: map[string]any{
			"field": field,
			"value": value,
		},
	}
}

// NewBadRequestError creates a bad request error.
func NewBadRequestError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeBadRequest,
		Message: message,
	}
}

// NewRateLimitError creates a rate limit exceeded error.
func NewRateLimitError(message string) *AppError {
	return &AppError{
		Code:    ErrCodeRateLimit,
		Message: message,
	}
}

// NewCacheError creates a cache error.
func NewCacheError(operation string, cause error) *AppError {
	return &AppError{
		Code:      ErrCodeCacheError,
		Message:   fmt.Sprintf("cache operation failed: %s", operation),
		Operation: operation,
		Cause:     cause,
	}
}

// NewXrayError creates an Xray error.
func NewXrayError(operation string, cause error) *AppError {
	return &AppError{
		Code:      ErrCodeXrayError,
		Message:   fmt.Sprintf("xray operation failed: %s", operation),
		Operation: operation,
		Cause:     cause,
	}
}

// NewValidationErrorWithFields creates a validation error with field-specific messages.
func NewValidationErrorWithFields(message string, fields map[string]string) *AppError {
	return &AppError{
		Code:    ErrCodeValidation,
		Message: message,
		Details: &ValidationErrors{Fields: fields},
	}
}

// IsAppError checks if an error is an AppError.
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// AsAppError converts an error to an AppError if possible.
func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// GetCode returns the error code if the error is an AppError.
func GetCode(err error) ErrorCode {
	if appErr, ok := AsAppError(err); ok {
		return appErr.Code
	}
	return ErrCodeInternal
}

// IsNotFound checks if the error is a not found error.
func IsNotFound(err error) bool {
	return GetCode(err) == ErrCodeNotFound
}

// IsValidation checks if the error is a validation error.
func IsValidation(err error) bool {
	return GetCode(err) == ErrCodeValidation
}

// IsUnauthorized checks if the error is an unauthorized error.
func IsUnauthorized(err error) bool {
	return GetCode(err) == ErrCodeUnauthorized
}

// IsDatabase checks if the error is a database error.
func IsDatabase(err error) bool {
	return GetCode(err) == ErrCodeDatabase
}

// IsRateLimit checks if the error is a rate limit error.
func IsRateLimit(err error) bool {
	return GetCode(err) == ErrCodeRateLimit
}

// IsCacheError checks if the error is a cache error.
func IsCacheError(err error) bool {
	return GetCode(err) == ErrCodeCacheError
}

// IsXrayError checks if the error is an Xray error.
func IsXrayError(err error) bool {
	return GetCode(err) == ErrCodeXrayError
}

// IsForbidden checks if the error is a forbidden error.
func IsForbidden(err error) bool {
	return GetCode(err) == ErrCodeForbidden
}

// IsConflict checks if the error is a conflict error.
func IsConflict(err error) bool {
	return GetCode(err) == ErrCodeConflict
}

// IsDuplicateKey checks if the error is a database duplicate key error.
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	// Check for GORM duplicate key errors
	errStr := err.Error()
	return strings.Contains(errStr, "UNIQUE constraint failed") ||
		strings.Contains(errStr, "Duplicate entry") ||
		strings.Contains(errStr, "duplicate key value")
}

// ToErrorResponse converts any error to an ErrorResponse.
// If the error is not an AppError, it creates a generic internal error response.
func ToErrorResponse(err error, requestID string) *ErrorResponse {
	if appErr, ok := AsAppError(err); ok {
		return appErr.ToResponse(requestID)
	}
	// For non-AppError, return a sanitized internal error
	return &ErrorResponse{
		Success: false,
		Error: &ErrorDetail{
			Code:       string(ErrCodeInternal),
			Message:    "服务器内部错误",
			Suggestion: "请稍后重试，如问题持续请联系管理员",
		},
		RequestID: requestID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// GetHTTPStatus returns the HTTP status code for any error.
func GetHTTPStatus(err error) int {
	if appErr, ok := AsAppError(err); ok {
		return appErr.HTTPStatus()
	}
	return http.StatusInternalServerError
}
