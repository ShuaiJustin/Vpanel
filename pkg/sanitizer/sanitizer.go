// Package sanitizer provides input sanitization utilities to prevent
// SQL injection, XSS, and command injection attacks.
package sanitizer

import (
	"html"
	"regexp"
	"strings"
	"unicode"
)

// Sanitizer provides methods for sanitizing user input.
type Sanitizer struct {
	// Configuration options
	AllowHTML       bool
	AllowedHTMLTags []string
}

// New creates a new Sanitizer with default settings.
func New() *Sanitizer {
	return &Sanitizer{
		AllowHTML:       false,
		AllowedHTMLTags: []string{},
	}
}

// NewWithOptions creates a new Sanitizer with custom options.
func NewWithOptions(allowHTML bool, allowedTags []string) *Sanitizer {
	return &Sanitizer{
		AllowHTML:       allowHTML,
		AllowedHTMLTags: allowedTags,
	}
}

// SQL injection patterns to detect
var sqlPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\b(SELECT|INSERT|UPDATE|DELETE|DROP|UNION|ALTER|CREATE|TRUNCATE|EXEC|EXECUTE)\b)`),
	regexp.MustCompile(`(?i)(\b(OR|AND)\b\s+[\d\w]+\s*=\s*[\d\w]+)`),
	regexp.MustCompile(`(?i)(--\s|--$|\#\s|\#$|\/\*|\*\/)`), // More specific: -- or # followed by space or end
	regexp.MustCompile(`(?i)(\bINTO\b\s+\bOUTFILE\b)`),
	regexp.MustCompile(`(?i)(\bLOAD_FILE\b)`),
	regexp.MustCompile(`(?i)(;\s*(SELECT|INSERT|UPDATE|DELETE|DROP))`),
	regexp.MustCompile(`(?i)(\bWAITFOR\b\s+\bDELAY\b)`),
	regexp.MustCompile(`(?i)(\bBENCHMARK\b\s*\()`),
}

// XSS patterns to detect
var xssPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`),
	regexp.MustCompile(`(?i)<script[^>]*>`),
	regexp.MustCompile(`(?i)javascript\s*:`),
	regexp.MustCompile(`(?i)vbscript\s*:`),
	regexp.MustCompile(`(?i)on\w+\s*=`),
	regexp.MustCompile(`(?i)<iframe[^>]*>`),
	regexp.MustCompile(`(?i)<object[^>]*>`),
	regexp.MustCompile(`(?i)<embed[^>]*>`),
	regexp.MustCompile(`(?i)<link[^>]*>`),
	regexp.MustCompile(`(?i)<meta[^>]*>`),
	regexp.MustCompile(`(?i)expression\s*\(`),
	regexp.MustCompile(`(?i)url\s*\(\s*['"]*\s*data:`),
}


// Command injection patterns to detect
var cmdPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[;&|` + "`" + `$]`),
	regexp.MustCompile(`\$\([^)]*\)`),
	regexp.MustCompile(`\$\{[^}]*\}`),
	regexp.MustCompile(`(?i)\b(cat|ls|rm|mv|cp|chmod|chown|wget|curl|bash|sh|zsh|python|perl|ruby|php|nc|netcat|ncat)\b`),
	regexp.MustCompile(`(?i)(>|>>|<)\s*[/\w]`),
	regexp.MustCompile(`\|\s*\w`),
	regexp.MustCompile(`(?i)\b(eval|exec|system|passthru|shell_exec|popen|proc_open)\b`),
}

// SanitizeResult contains the result of sanitization.
type SanitizeResult struct {
	Original    string
	Sanitized   string
	WasModified bool
	Threats     []ThreatInfo
}

// ThreatInfo describes a detected threat.
type ThreatInfo struct {
	Type        ThreatType
	Pattern     string
	Position    int
	Description string
}

// ThreatType represents the type of security threat.
type ThreatType string

const (
	ThreatSQL     ThreatType = "SQL_INJECTION"
	ThreatXSS     ThreatType = "XSS"
	ThreatCommand ThreatType = "COMMAND_INJECTION"
)

// SanitizeString sanitizes a string input, removing potentially dangerous content.
func (s *Sanitizer) SanitizeString(input string) SanitizeResult {
	result := SanitizeResult{
		Original:    input,
		Sanitized:   input,
		WasModified: false,
		Threats:     []ThreatInfo{},
	}

	if input == "" {
		return result
	}

	// Detect and remove SQL injection patterns
	result = s.sanitizeSQL(result)

	// Detect and remove XSS patterns
	result = s.sanitizeXSS(result)

	// Detect and remove command injection patterns
	result = s.sanitizeCommand(result)

	// Final cleanup
	result.Sanitized = s.cleanupString(result.Sanitized)

	result.WasModified = result.Original != result.Sanitized

	return result
}

// sanitizeSQL removes SQL injection patterns.
func (s *Sanitizer) sanitizeSQL(result SanitizeResult) SanitizeResult {
	for _, pattern := range sqlPatterns {
		matches := pattern.FindAllStringIndex(result.Sanitized, -1)
		for _, match := range matches {
			result.Threats = append(result.Threats, ThreatInfo{
				Type:        ThreatSQL,
				Pattern:     result.Sanitized[match[0]:match[1]],
				Position:    match[0],
				Description: "SQL injection pattern detected",
			})
		}
		result.Sanitized = pattern.ReplaceAllString(result.Sanitized, "")
	}
	return result
}

// sanitizeXSS removes XSS patterns.
func (s *Sanitizer) sanitizeXSS(result SanitizeResult) SanitizeResult {
	for _, pattern := range xssPatterns {
		matches := pattern.FindAllStringIndex(result.Sanitized, -1)
		for _, match := range matches {
			result.Threats = append(result.Threats, ThreatInfo{
				Type:        ThreatXSS,
				Pattern:     result.Sanitized[match[0]:match[1]],
				Position:    match[0],
				Description: "XSS pattern detected",
			})
		}
		result.Sanitized = pattern.ReplaceAllString(result.Sanitized, "")
	}

	// HTML escape if not allowing HTML
	if !s.AllowHTML {
		result.Sanitized = html.EscapeString(result.Sanitized)
	}

	return result
}

// sanitizeCommand removes command injection patterns.
func (s *Sanitizer) sanitizeCommand(result SanitizeResult) SanitizeResult {
	for _, pattern := range cmdPatterns {
		matches := pattern.FindAllStringIndex(result.Sanitized, -1)
		for _, match := range matches {
			result.Threats = append(result.Threats, ThreatInfo{
				Type:        ThreatCommand,
				Pattern:     result.Sanitized[match[0]:match[1]],
				Position:    match[0],
				Description: "Command injection pattern detected",
			})
		}
		result.Sanitized = pattern.ReplaceAllString(result.Sanitized, "")
	}
	return result
}

// cleanupString performs final cleanup on the sanitized string.
func (s *Sanitizer) cleanupString(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Trim whitespace
	input = strings.TrimSpace(input)

	// Collapse multiple spaces
	spaceRegex := regexp.MustCompile(`\s+`)
	input = spaceRegex.ReplaceAllString(input, " ")

	return input
}


// SanitizeForSQL sanitizes input specifically for SQL context.
// Note: This should be used in addition to parameterized queries, not as a replacement.
func (s *Sanitizer) SanitizeForSQL(input string) string {
	if input == "" {
		return input
	}

	// Escape single quotes
	input = strings.ReplaceAll(input, "'", "''")

	// Remove dangerous SQL keywords
	for _, pattern := range sqlPatterns {
		input = pattern.ReplaceAllString(input, "")
	}

	return strings.TrimSpace(input)
}

// SanitizeForHTML sanitizes input for HTML context.
func (s *Sanitizer) SanitizeForHTML(input string) string {
	if input == "" {
		return input
	}

	// Remove XSS patterns first
	for _, pattern := range xssPatterns {
		input = pattern.ReplaceAllString(input, "")
	}

	// HTML escape
	return html.EscapeString(input)
}

// SanitizeForCommand sanitizes input for shell command context.
// Note: Avoid using user input in shell commands when possible.
func (s *Sanitizer) SanitizeForCommand(input string) string {
	if input == "" {
		return input
	}

	// Remove dangerous characters and patterns
	for _, pattern := range cmdPatterns {
		input = pattern.ReplaceAllString(input, "")
	}

	// Remove any remaining shell metacharacters
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "[", "]", "<", ">", "!", "\\", "\n", "\r"}
	for _, char := range dangerousChars {
		input = strings.ReplaceAll(input, char, "")
	}

	return strings.TrimSpace(input)
}

// SanitizeFilename sanitizes a filename to prevent path traversal.
func (s *Sanitizer) SanitizeFilename(input string) string {
	if input == "" {
		return input
	}

	// Remove path separators
	input = strings.ReplaceAll(input, "/", "")
	input = strings.ReplaceAll(input, "\\", "")

	// Remove path traversal patterns
	input = strings.ReplaceAll(input, "..", "")

	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Keep only safe characters
	var result strings.Builder
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// SanitizePath sanitizes a file path.
func (s *Sanitizer) SanitizePath(input string) string {
	if input == "" {
		return input
	}

	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")

	// Normalize path separators
	input = strings.ReplaceAll(input, "\\", "/")

	// Remove path traversal
	for strings.Contains(input, "..") {
		input = strings.ReplaceAll(input, "..", "")
	}

	// Remove double slashes
	for strings.Contains(input, "//") {
		input = strings.ReplaceAll(input, "//", "/")
	}

	return strings.TrimSpace(input)
}

// ContainsSQLInjection checks if input contains SQL injection patterns.
func (s *Sanitizer) ContainsSQLInjection(input string) bool {
	for _, pattern := range sqlPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// ContainsXSS checks if input contains XSS patterns.
func (s *Sanitizer) ContainsXSS(input string) bool {
	for _, pattern := range xssPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// ContainsCommandInjection checks if input contains command injection patterns.
func (s *Sanitizer) ContainsCommandInjection(input string) bool {
	for _, pattern := range cmdPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

// IsSafe checks if input is safe (contains no detected threats).
func (s *Sanitizer) IsSafe(input string) bool {
	return !s.ContainsSQLInjection(input) &&
		!s.ContainsXSS(input) &&
		!s.ContainsCommandInjection(input)
}

// DetectThreats returns all detected threats in the input without modifying it.
func (s *Sanitizer) DetectThreats(input string) []ThreatInfo {
	var threats []ThreatInfo

	// Check SQL injection
	for _, pattern := range sqlPatterns {
		matches := pattern.FindAllStringIndex(input, -1)
		for _, match := range matches {
			threats = append(threats, ThreatInfo{
				Type:        ThreatSQL,
				Pattern:     input[match[0]:match[1]],
				Position:    match[0],
				Description: "SQL injection pattern detected",
			})
		}
	}

	// Check XSS
	for _, pattern := range xssPatterns {
		matches := pattern.FindAllStringIndex(input, -1)
		for _, match := range matches {
			threats = append(threats, ThreatInfo{
				Type:        ThreatXSS,
				Pattern:     input[match[0]:match[1]],
				Position:    match[0],
				Description: "XSS pattern detected",
			})
		}
	}

	// Check command injection
	for _, pattern := range cmdPatterns {
		matches := pattern.FindAllStringIndex(input, -1)
		for _, match := range matches {
			threats = append(threats, ThreatInfo{
				Type:        ThreatCommand,
				Pattern:     input[match[0]:match[1]],
				Position:    match[0],
				Description: "Command injection pattern detected",
			})
		}
	}

	return threats
}

// Default is the default sanitizer instance.
var Default = New()

// SanitizeString sanitizes a string using the default sanitizer.
func SanitizeString(input string) SanitizeResult {
	return Default.SanitizeString(input)
}

// SanitizeForSQL sanitizes input for SQL using the default sanitizer.
func SanitizeForSQL(input string) string {
	return Default.SanitizeForSQL(input)
}

// SanitizeForHTML sanitizes input for HTML using the default sanitizer.
func SanitizeForHTML(input string) string {
	return Default.SanitizeForHTML(input)
}

// SanitizeForCommand sanitizes input for shell commands using the default sanitizer.
func SanitizeForCommand(input string) string {
	return Default.SanitizeForCommand(input)
}

// SanitizeFilename sanitizes a filename using the default sanitizer.
func SanitizeFilename(input string) string {
	return Default.SanitizeFilename(input)
}

// SanitizePath sanitizes a file path using the default sanitizer.
func SanitizePath(input string) string {
	return Default.SanitizePath(input)
}

// IsSafe checks if input is safe using the default sanitizer.
func IsSafe(input string) bool {
	return Default.IsSafe(input)
}

// DetectThreats detects threats using the default sanitizer.
func DetectThreats(input string) []ThreatInfo {
	return Default.DetectThreats(input)
}
