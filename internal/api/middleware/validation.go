// Package middleware provides HTTP middleware for the API.
package middleware

import (
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"v/pkg/errors"
)

// Validator is the global validator instance.
var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register custom tag name function to use json tags
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// Register custom validators
	_ = validate.RegisterValidation("notblank", notBlank)
}

// notBlank validates that a string is not blank (not empty and not just whitespace).
func notBlank(fl validator.FieldLevel) bool {
	return strings.TrimSpace(fl.Field().String()) != ""
}

// ValidationError represents a validation error for API responses.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Tag     string `json:"tag,omitempty"`
	Value   any    `json:"value,omitempty"`
}

// ValidateRequest validates a request body against struct tags.
// It binds the JSON body to the provided struct and validates it.
func ValidateRequest[T any](c *gin.Context) (*T, *errors.AppError) {
	var req T

	// Bind JSON body
	if err := c.ShouldBindJSON(&req); err != nil {
		return nil, errors.NewValidationError("Invalid request body", map[string]string{
			"body": "Failed to parse JSON: " + err.Error(),
		})
	}

	// Validate struct
	if err := validate.Struct(&req); err != nil {
		validationErrors := extractValidationErrors(err)
		return nil, errors.NewValidationErrorWithFields("Validation failed", validationErrors)
	}

	return &req, nil
}


// extractValidationErrors extracts field-specific error messages from validation errors.
func extractValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)

	if validationErrs, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrs {
			field := e.Field()
			fieldErrors[field] = getValidationMessage(e)
		}
	} else {
		fieldErrors["_"] = err.Error()
	}

	return fieldErrors
}

// getValidationMessage returns a human-readable message for a validation error.
func getValidationMessage(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Value is too short (minimum: " + e.Param() + ")"
	case "max":
		return "Value is too long (maximum: " + e.Param() + ")"
	case "gte":
		return "Value must be greater than or equal to " + e.Param()
	case "lte":
		return "Value must be less than or equal to " + e.Param()
	case "gt":
		return "Value must be greater than " + e.Param()
	case "lt":
		return "Value must be less than " + e.Param()
	case "len":
		return "Value must have exactly " + e.Param() + " characters"
	case "oneof":
		return "Value must be one of: " + e.Param()
	case "url":
		return "Invalid URL format"
	case "uuid":
		return "Invalid UUID format"
	case "alphanum":
		return "Value must contain only alphanumeric characters"
	case "numeric":
		return "Value must be numeric"
	case "notblank":
		return "This field cannot be blank"
	case "ip":
		return "Invalid IP address format"
	case "ipv4":
		return "Invalid IPv4 address format"
	case "ipv6":
		return "Invalid IPv6 address format"
	default:
		return "Invalid value"
	}
}

// ValidateQuery validates query parameters against struct tags.
func ValidateQuery[T any](c *gin.Context) (*T, *errors.AppError) {
	var req T

	// Bind query parameters
	if err := c.ShouldBindQuery(&req); err != nil {
		return nil, errors.NewValidationError("Invalid query parameters", map[string]string{
			"query": "Failed to parse query parameters: " + err.Error(),
		})
	}

	// Validate struct
	if err := validate.Struct(&req); err != nil {
		validationErrors := extractValidationErrors(err)
		return nil, errors.NewValidationErrorWithFields("Validation failed", validationErrors)
	}

	return &req, nil
}

// ValidatePathParam validates a path parameter.
func ValidatePathParam(c *gin.Context, name string, validators ...string) (string, *errors.AppError) {
	value := c.Param(name)
	if value == "" {
		return "", errors.NewValidationError("Missing path parameter", map[string]string{
			name: "Path parameter is required",
		})
	}

	// Apply validators
	for _, v := range validators {
		if err := validate.Var(value, v); err != nil {
			return "", errors.NewValidationError("Invalid path parameter", map[string]string{
				name: "Invalid value for " + name,
			})
		}
	}

	return value, nil
}

// ValidateHeader validates a header value.
func ValidateHeader(c *gin.Context, name string, required bool, validators ...string) (string, *errors.AppError) {
	value := c.GetHeader(name)
	if value == "" && required {
		return "", errors.NewValidationError("Missing header", map[string]string{
			name: "Header is required",
		})
	}

	if value != "" {
		for _, v := range validators {
			if err := validate.Var(value, v); err != nil {
				return "", errors.NewValidationError("Invalid header", map[string]string{
					name: "Invalid value for header " + name,
				})
			}
		}
	}

	return value, nil
}

// ValidationMiddleware returns a middleware that validates requests based on registered schemas.
func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// This middleware can be extended to support JSON Schema validation
		// For now, it just ensures the request ID is set for error responses
		c.Next()
	}
}

// RespondWithError sends an error response with proper formatting.
func RespondWithError(c *gin.Context, err *errors.AppError) {
	requestID := c.GetString("request_id")
	response := err.ToResponse(requestID)
	c.JSON(err.HTTPStatus(), response)
}

// RespondWithValidationError is a convenience function for validation errors.
func RespondWithValidationError(c *gin.Context, message string, fields map[string]string) {
	err := errors.NewValidationErrorWithFields(message, fields)
	RespondWithError(c, err)
}

// GetValidator returns the global validator instance for custom configuration.
func GetValidator() *validator.Validate {
	return validate
}

// RegisterCustomValidation registers a custom validation function.
func RegisterCustomValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}

// PaginationParams represents common pagination parameters.
type PaginationParams struct {
	Page     int `form:"page" validate:"omitempty,gte=1"`
	PageSize int `form:"page_size" validate:"omitempty,gte=1,lte=100"`
}

// GetOffset returns the offset for database queries.
func (p *PaginationParams) GetOffset() int {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 {
		p.PageSize = 20
	}
	return (p.Page - 1) * p.PageSize
}

// GetLimit returns the limit for database queries.
func (p *PaginationParams) GetLimit() int {
	if p.PageSize <= 0 {
		return 20
	}
	if p.PageSize > 100 {
		return 100
	}
	return p.PageSize
}

// SortParams represents common sorting parameters.
type SortParams struct {
	SortBy    string `form:"sort_by" validate:"omitempty,alphanum"`
	SortOrder string `form:"sort_order" validate:"omitempty,oneof=asc desc"`
}

// GetSortOrder returns the sort order, defaulting to "asc".
func (s *SortParams) GetSortOrder() string {
	if s.SortOrder == "" {
		return "asc"
	}
	return s.SortOrder
}

// DateRangeParams represents common date range parameters.
type DateRangeParams struct {
	StartDate string `form:"start_date" validate:"omitempty"`
	EndDate   string `form:"end_date" validate:"omitempty"`
}

// ParseDates parses the date strings into time.Time values.
func (d *DateRangeParams) ParseDates() (start, end time.Time, err error) {
	if d.StartDate != "" {
		start, err = time.Parse("2006-01-02", d.StartDate)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	if d.EndDate != "" {
		end, err = time.Parse("2006-01-02", d.EndDate)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	return start, end, nil
}
