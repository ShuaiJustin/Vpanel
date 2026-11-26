package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Property 3: API Input Validation
// For any API request with invalid input data, the Backend SHALL return a 400 Bad Request
// response with validation error details before any business logic is executed.
// **Validates: Requirements 3.5**

func init() {
	gin.SetMode(gin.TestMode)
}

// TestAPIInputValidation_LoginEmptyFields tests that login requests with empty required fields
// return 400 Bad Request.
func TestAPIInputValidation_LoginEmptyFields(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("login with empty username returns 400", prop.ForAll(
		func(password string) bool {
			router := gin.New()
			router.POST("/login", func(c *gin.Context) {
				var req LoginRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]string{
				"username": "",
				"password": password,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString(),
	))

	properties.Property("login with empty password returns 400", prop.ForAll(
		func(username string) bool {
			if username == "" {
				return true // Skip empty username case
			}

			router := gin.New()
			router.POST("/login", func(c *gin.Context) {
				var req LoginRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]string{
				"username": username,
				"password": "",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}

// TestAPIInputValidation_ProxyInvalidPort tests that proxy creation with invalid port
// returns 400 Bad Request.
func TestAPIInputValidation_ProxyInvalidPort(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("proxy with port < 1 returns 400", prop.ForAll(
		func(name, protocol string, port int) bool {
			if name == "" || protocol == "" {
				return true // Skip empty required fields
			}

			router := gin.New()
			router.POST("/proxy", func(c *gin.Context) {
				var req CreateProxyRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]any{
				"name":     name,
				"protocol": protocol,
				"port":     port,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxy", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Port must be between 1 and 65535
			if port < 1 || port > 65535 {
				return w.Code == http.StatusBadRequest
			}
			return w.Code == http.StatusOK
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("vmess", "vless", "trojan", "shadowsocks"),
		gen.IntRange(-1000, 70000),
	))

	properties.TestingRun(t)
}

// TestAPIInputValidation_ProxyMissingRequired tests that proxy creation with missing
// required fields returns 400 Bad Request.
func TestAPIInputValidation_ProxyMissingRequired(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("proxy without name returns 400", prop.ForAll(
		func(protocol string, port int) bool {
			if protocol == "" || port < 1 || port > 65535 {
				return true // Skip invalid cases
			}

			router := gin.New()
			router.POST("/proxy", func(c *gin.Context) {
				var req CreateProxyRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]any{
				"protocol": protocol,
				"port":     port,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxy", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.OneConstOf("vmess", "vless", "trojan", "shadowsocks"),
		gen.IntRange(1, 65535),
	))

	properties.Property("proxy without protocol returns 400", prop.ForAll(
		func(name string, port int) bool {
			if name == "" || port < 1 || port > 65535 {
				return true // Skip invalid cases
			}

			router := gin.New()
			router.POST("/proxy", func(c *gin.Context) {
				var req CreateProxyRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]any{
				"name": name,
				"port": port,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxy", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(1, 65535),
	))

	properties.Property("proxy without port returns 400", prop.ForAll(
		func(name, protocol string) bool {
			if name == "" || protocol == "" {
				return true // Skip invalid cases
			}

			router := gin.New()
			router.POST("/proxy", func(c *gin.Context) {
				var req CreateProxyRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]any{
				"name":     name,
				"protocol": protocol,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxy", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.OneConstOf("vmess", "vless", "trojan", "shadowsocks"),
	))

	properties.TestingRun(t)
}

// TestAPIInputValidation_InvalidJSON tests that requests with invalid JSON
// return 400 Bad Request.
func TestAPIInputValidation_InvalidJSON(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("invalid JSON returns 400", prop.ForAll(
		func(invalidJSON string) bool {
			// Ensure the string is not valid JSON
			var js json.RawMessage
			if json.Unmarshal([]byte(invalidJSON), &js) == nil {
				return true // Skip valid JSON
			}

			router := gin.New()
			router.POST("/test", func(c *gin.Context) {
				var req map[string]any
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBufferString(invalidJSON))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.OneConstOf(
			"{invalid}",
			"not json at all",
			"{\"key\": }",
			"[1, 2, 3",
			"{\"unclosed\": \"string",
		),
	))

	properties.TestingRun(t)
}

// TestAPIInputValidation_PasswordMinLength tests that password with less than
// minimum length returns 400 Bad Request.
func TestAPIInputValidation_PasswordMinLength(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("password shorter than 6 chars returns 400 for change password", prop.ForAll(
		func(oldPassword string, newPasswordLen int) bool {
			if oldPassword == "" || newPasswordLen < 0 || newPasswordLen >= 6 {
				return true // Skip invalid or valid cases
			}

			// Generate a password of specific length
			newPassword := ""
			for i := 0; i < newPasswordLen; i++ {
				newPassword += "a"
			}

			router := gin.New()
			router.PUT("/password", func(c *gin.Context) {
				var req ChangePasswordRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]string{
				"old_password": oldPassword,
				"new_password": newPassword,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPut, "/password", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// TestAPIInputValidation_CreateUserMinPassword tests that user creation with
// password shorter than minimum returns 400 Bad Request.
func TestAPIInputValidation_CreateUserMinPassword(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("create user with short password returns 400", prop.ForAll(
		func(username string, passwordLen int) bool {
			if username == "" || passwordLen < 0 || passwordLen >= 6 {
				return true // Skip invalid or valid cases
			}

			// Generate a password of specific length
			password := ""
			for i := 0; i < passwordLen; i++ {
				password += "a"
			}

			router := gin.New()
			router.POST("/user", func(c *gin.Context) {
				var req CreateUserRequest
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
					return
				}
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			body := map[string]string{
				"username": username,
				"password": password,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/user", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}
