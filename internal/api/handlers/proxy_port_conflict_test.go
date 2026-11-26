package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Feature: project-optimization, Property 12: Port Conflict Detection
// *For any* proxy creation or update with a port that is already in use by another proxy,
// the operation SHALL be rejected with an error containing the conflicting proxy information.
// **Validates: Requirements 21.8, 21.9**

// TestPortConflictDetection_CreateWithConflictingPort tests that creating a proxy
// with a port already in use returns a conflict error.
func TestPortConflictDetection_CreateWithConflictingPort(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("creating proxy with existing port returns conflict", prop.ForAll(
		func(userID int64, existingPort int, newProxyName string) bool {
			if userID <= 0 || existingPort < 1 || existingPort > 65535 || newProxyName == "" {
				return true // Skip invalid cases
			}

			repo := newMockProxyRepository()

			// Create an existing proxy with the port
			repo.Create(context.Background(), &repository.Proxy{
				UserID:   userID,
				Name:     "existing-proxy",
				Protocol: "vmess",
				Port:     existingPort,
				Enabled:  true,
			})

			router, handler := setupTestRouter(repo)

			router.POST("/proxies", func(c *gin.Context) {
				setUserContext(c, userID, "user")
				handler.Create(c)
			})

			// Try to create a new proxy with the same port
			body := map[string]any{
				"name":     newProxyName,
				"protocol": "vmess",
				"port":     existingPort, // Same port as existing proxy
				"enabled":  true,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxies", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should return 409 Conflict
			if w.Code != http.StatusConflict {
				return false
			}

			// Verify error response contains conflicting proxy info
			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return false
			}

			// Check that error message indicates port conflict
			if _, ok := response["error"]; !ok {
				return false
			}

			// Check that details contain conflicting proxy info
			details, ok := response["details"].(map[string]any)
			if !ok {
				return false
			}

			// Verify conflicting proxy ID is present
			if _, ok := details["conflicting_proxy_id"]; !ok {
				return false
			}

			return true
		},
		gen.Int64Range(1, 100),
		gen.IntRange(1, 65535),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// TestPortConflictDetection_CreateWithUniquePort tests that creating a proxy
// with a unique port succeeds.
func TestPortConflictDetection_CreateWithUniquePort(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("creating proxy with unique port succeeds", prop.ForAll(
		func(userID int64, existingPort int, newPort int, newProxyName string) bool {
			if userID <= 0 || existingPort < 1 || existingPort > 65535 || newPort < 1 || newPort > 65535 {
				return true // Skip invalid cases
			}
			if existingPort == newPort || newProxyName == "" {
				return true // Skip same port case
			}

			repo := newMockProxyRepository()

			// Create an existing proxy with a different port
			repo.Create(context.Background(), &repository.Proxy{
				UserID:   userID,
				Name:     "existing-proxy",
				Protocol: "vmess",
				Port:     existingPort,
				Enabled:  true,
			})

			router, handler := setupTestRouter(repo)

			router.POST("/proxies", func(c *gin.Context) {
				setUserContext(c, userID, "user")
				handler.Create(c)
			})

			// Create a new proxy with a different port
			body := map[string]any{
				"name":     newProxyName,
				"protocol": "vmess",
				"port":     newPort, // Different port
				"enabled":  true,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxies", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should return 201 Created
			return w.Code == http.StatusCreated
		},
		gen.Int64Range(1, 100),
		gen.IntRange(1, 32767),
		gen.IntRange(32768, 65535),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
	))

	properties.TestingRun(t)
}

// TestPortConflictDetection_UpdateWithConflictingPort tests that updating a proxy
// to use a port already in use returns a conflict error.
func TestPortConflictDetection_UpdateWithConflictingPort(t *testing.T) {
	repo := newMockProxyRepository()

	// Create two proxies with different ports
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "proxy1",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "proxy2",
		Protocol: "vmess",
		Port:     20000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	router.PUT("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 1, "user")
		handler.Update(c)
	})

	// Try to update proxy2 to use proxy1's port
	body := map[string]any{
		"port": 10000, // Same port as proxy1
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/proxies/2", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 409 Conflict
	assert.Equal(t, http.StatusConflict, w.Code)

	// Verify error response contains conflicting proxy info
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response, "details")

	details := response["details"].(map[string]any)
	assert.Contains(t, details, "conflicting_proxy_id")
	assert.Equal(t, float64(1), details["conflicting_proxy_id"])
}

// TestPortConflictDetection_UpdateSamePortAllowed tests that updating a proxy
// to keep the same port succeeds.
func TestPortConflictDetection_UpdateSamePortAllowed(t *testing.T) {
	repo := newMockProxyRepository()

	// Create a proxy
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "proxy1",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	router.PUT("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 1, "user")
		handler.Update(c)
	})

	// Update proxy1 with the same port (should be allowed)
	body := map[string]any{
		"name": "updated-proxy1",
		"port": 10000, // Same port as before
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/proxies/1", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestPortConflictDetection_UpdateToUniquePort tests that updating a proxy
// to use a unique port succeeds.
func TestPortConflictDetection_UpdateToUniquePort(t *testing.T) {
	repo := newMockProxyRepository()

	// Create two proxies with different ports
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "proxy1",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "proxy2",
		Protocol: "vmess",
		Port:     20000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	router.PUT("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 1, "user")
		handler.Update(c)
	})

	// Update proxy2 to use a new unique port
	body := map[string]any{
		"port": 30000, // New unique port
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/proxies/2", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify the port was updated
	var response ProxyResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 30000, response.Port)
}

// TestPortConflictDetection_ErrorContainsConflictingProxyInfo tests that the error
// response contains detailed information about the conflicting proxy.
func TestPortConflictDetection_ErrorContainsConflictingProxyInfo(t *testing.T) {
	repo := newMockProxyRepository()

	// Create an existing proxy
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "existing-proxy",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router := gin.New()
	handler := NewProxyHandler(&mockProxyManager{}, repo, logger.NewNopLogger())

	router.POST("/proxies", func(c *gin.Context) {
		setUserContext(c, 1, "user")
		handler.Create(c)
	})

	// Try to create a new proxy with the same port
	body := map[string]any{
		"name":     "new-proxy",
		"protocol": "vmess",
		"port":     10000,
		"enabled":  true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/proxies", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify error response structure
	var response map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check error message
	assert.Equal(t, "Port is already in use", response["error"])

	// Check details
	details := response["details"].(map[string]any)
	assert.Equal(t, float64(1), details["conflicting_proxy_id"])
	assert.Equal(t, "existing-proxy", details["conflicting_proxy_name"])
	assert.Equal(t, float64(10000), details["port"])
}
