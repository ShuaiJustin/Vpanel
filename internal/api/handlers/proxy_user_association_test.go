package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/proxy"
)

// Feature: project-optimization, Property 13: Proxy User Association
// *For any* proxy created by a non-admin user, the proxy's user_id SHALL be set to the
// authenticated user's ID, and *for any* proxy listing by a non-admin user, only proxies
// belonging to that user SHALL be returned.
// **Validates: Requirements 21.2, 21.3**

// mockProxyRepository is a mock implementation of ProxyRepository for testing.
type mockProxyRepository struct {
	proxies map[int64]*repository.Proxy
	nextID  int64
}

func newMockProxyRepository() *mockProxyRepository {
	return &mockProxyRepository{
		proxies: make(map[int64]*repository.Proxy),
		nextID:  1,
	}
}

func (m *mockProxyRepository) Create(ctx context.Context, proxy *repository.Proxy) error {
	proxy.ID = m.nextID
	proxy.CreatedAt = time.Now()
	proxy.UpdatedAt = time.Now()
	m.nextID++
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepository) GetByID(ctx context.Context, id int64) (*repository.Proxy, error) {
	if p, ok := m.proxies[id]; ok {
		return p, nil
	}
	return nil, nil
}

func (m *mockProxyRepository) Update(ctx context.Context, proxy *repository.Proxy) error {
	proxy.UpdatedAt = time.Now()
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepository) Delete(ctx context.Context, id int64) error {
	delete(m.proxies, id)
	return nil
}

func (m *mockProxyRepository) List(ctx context.Context, limit, offset int) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, p := range m.proxies {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockProxyRepository) GetByProtocol(ctx context.Context, protocol string) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, p := range m.proxies {
		if p.Protocol == protocol {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProxyRepository) GetEnabled(ctx context.Context) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, p := range m.proxies {
		if p.Enabled {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProxyRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, p := range m.proxies {
		if p.UserID == userID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProxyRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	for _, p := range m.proxies {
		if p.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepository) GetByPort(ctx context.Context, port int) (*repository.Proxy, error) {
	for _, p := range m.proxies {
		if p.Port == port {
			return p, nil
		}
	}
	return nil, nil
}

func (m *mockProxyRepository) EnableByUserID(ctx context.Context, userID int64) error {
	for _, p := range m.proxies {
		if p.UserID == userID {
			p.Enabled = true
		}
	}
	return nil
}

func (m *mockProxyRepository) DisableByUserID(ctx context.Context, userID int64) error {
	for _, p := range m.proxies {
		if p.UserID == userID {
			p.Enabled = false
		}
	}
	return nil
}

func (m *mockProxyRepository) DeleteByIDs(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		delete(m.proxies, id)
	}
	return nil
}

func (m *mockProxyRepository) Count(ctx context.Context) (int64, error) {
	return int64(len(m.proxies)), nil
}

func (m *mockProxyRepository) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	for _, p := range m.proxies {
		if p.Enabled {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepository) CountByProtocol(ctx context.Context) ([]*repository.ProtocolCount, error) {
	counts := make(map[string]int64)
	for _, p := range m.proxies {
		counts[p.Protocol]++
	}
	var result []*repository.ProtocolCount
	for protocol, count := range counts {
		result = append(result, &repository.ProtocolCount{
			Protocol: protocol,
			Count:    count,
		})
	}
	return result, nil
}

func (m *mockProxyRepository) GetByNodeID(ctx context.Context, nodeID int64) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, p := range m.proxies {
		if p.NodeID != nil && *p.NodeID == nodeID && p.Enabled {
			result = append(result, p)
		}
	}
	return result, nil
}

// mockProxyManager is a mock implementation of proxy.Manager for testing.
type mockProxyManager struct{}

func (m *mockProxyManager) RegisterProtocol(protocol proxy.Protocol) {}

func (m *mockProxyManager) GetProtocol(name string) (proxy.Protocol, bool) {
	return &mockProtocol{name: name}, true
}

func (m *mockProxyManager) ListProtocols() []string {
	return []string{"vmess", "vless", "trojan", "shadowsocks"}
}

func (m *mockProxyManager) CreateProxy(ctx context.Context, settings *proxy.Settings) error {
	return nil
}

func (m *mockProxyManager) UpdateProxy(ctx context.Context, settings *proxy.Settings) error {
	return nil
}

func (m *mockProxyManager) DeleteProxy(ctx context.Context, id int64) error {
	return nil
}

func (m *mockProxyManager) GetProxy(ctx context.Context, id int64) (*proxy.Settings, error) {
	return nil, nil
}

func (m *mockProxyManager) ListProxies(ctx context.Context, page, pageSize int) ([]*proxy.Settings, int64, error) {
	return nil, 0, nil
}

func (m *mockProxyManager) GetProxiesByUser(ctx context.Context, userID int64) ([]*proxy.Settings, error) {
	return nil, nil
}

func (m *mockProxyManager) GenerateLink(ctx context.Context, id int64) (string, error) {
	return "", nil
}

func (m *mockProxyManager) GenerateConfig(ctx context.Context, id int64) (json.RawMessage, error) {
	return nil, nil
}

// mockProtocol is a mock implementation of proxy.Protocol for testing.
type mockProtocol struct {
	name string
}

func (m *mockProtocol) Name() string {
	return m.name
}

func (m *mockProtocol) Validate(settings *proxy.Settings) error {
	return nil
}

func (m *mockProtocol) DefaultSettings() map[string]any {
	return map[string]any{}
}

func (m *mockProtocol) GenerateLink(settings *proxy.Settings) (string, error) {
	return "mock://link", nil
}

func (m *mockProtocol) GenerateConfig(settings *proxy.Settings) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

func (m *mockProtocol) ParseLink(link string) (*proxy.Settings, error) {
	return &proxy.Settings{}, nil
}

// setupTestRouter creates a test router with the proxy handler.
func setupTestRouter(repo *mockProxyRepository) (*gin.Engine, *ProxyHandler) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := NewProxyHandler(&mockProxyManager{}, repo, logger.NewNopLogger())

	return router, handler
}

// setUserContext sets user context in the gin context.
func setUserContext(c *gin.Context, userID int64, role string) {
	c.Set("user_id", userID)
	c.Set("role", role)
}

// TestProxyUserAssociation_CreateSetsUserID tests that when a proxy is created,
// the user_id is set to the authenticated user's ID.
func TestProxyUserAssociation_CreateSetsUserID(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("created proxy has user_id set to authenticated user", prop.ForAll(
		func(userID int64, proxyName string, port int) bool {
			if userID <= 0 || proxyName == "" || port < 1 || port > 65535 {
				return true // Skip invalid cases
			}

			repo := newMockProxyRepository()
			router, handler := setupTestRouter(repo)

			router.POST("/proxies", func(c *gin.Context) {
				setUserContext(c, userID, "user")
				handler.Create(c)
			})

			body := map[string]any{
				"name":     proxyName,
				"protocol": "vmess",
				"port":     port,
				"enabled":  true,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/proxies", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusCreated {
				return false
			}

			// Verify the proxy was created with the correct user_id
			var response ProxyResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return false
			}

			return response.UserID == userID
		},
		gen.Int64Range(1, 1000),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
		gen.IntRange(1, 65535),
	))

	properties.TestingRun(t)
}

// TestProxyUserAssociation_ListFiltersForNonAdmin tests that non-admin users
// can only see their own proxies.
func TestProxyUserAssociation_ListFiltersForNonAdmin(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("non-admin user only sees their own proxies", prop.ForAll(
		func(userID int64, otherUserID int64, numOwnProxies int, numOtherProxies int) bool {
			if userID <= 0 || otherUserID <= 0 || userID == otherUserID {
				return true // Skip invalid cases
			}
			if numOwnProxies < 0 || numOwnProxies > 10 || numOtherProxies < 0 || numOtherProxies > 10 {
				return true // Skip extreme cases
			}

			repo := newMockProxyRepository()

			// Create proxies for the current user
			for i := 0; i < numOwnProxies; i++ {
				repo.Create(context.Background(), &repository.Proxy{
					UserID:   userID,
					Name:     "own-proxy",
					Protocol: "vmess",
					Port:     10000 + i,
					Enabled:  true,
				})
			}

			// Create proxies for another user
			for i := 0; i < numOtherProxies; i++ {
				repo.Create(context.Background(), &repository.Proxy{
					UserID:   otherUserID,
					Name:     "other-proxy",
					Protocol: "vmess",
					Port:     20000 + i,
					Enabled:  true,
				})
			}

			router, handler := setupTestRouter(repo)

			router.GET("/proxies", func(c *gin.Context) {
				setUserContext(c, userID, "user") // Non-admin user
				handler.List(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				return false
			}

			var response []ProxyResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return false
			}

			// Verify only own proxies are returned
			if len(response) != numOwnProxies {
				return false
			}

			// Verify all returned proxies belong to the user
			for _, p := range response {
				if p.UserID != userID {
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 100),
		gen.Int64Range(101, 200),
		gen.IntRange(0, 5),
		gen.IntRange(0, 5),
	))

	properties.TestingRun(t)
}

// TestProxyUserAssociation_AdminSeesAllProxies tests that admin users
// can see all proxies regardless of owner.
func TestProxyUserAssociation_AdminSeesAllProxies(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("admin user sees all proxies", prop.ForAll(
		func(adminID int64, user1ID int64, user2ID int64, numUser1Proxies int, numUser2Proxies int) bool {
			if adminID <= 0 || user1ID <= 0 || user2ID <= 0 {
				return true // Skip invalid cases
			}
			if adminID == user1ID || adminID == user2ID || user1ID == user2ID {
				return true // Skip overlapping IDs
			}
			if numUser1Proxies < 0 || numUser1Proxies > 5 || numUser2Proxies < 0 || numUser2Proxies > 5 {
				return true // Skip extreme cases
			}

			repo := newMockProxyRepository()

			// Create proxies for user1
			for i := 0; i < numUser1Proxies; i++ {
				repo.Create(context.Background(), &repository.Proxy{
					UserID:   user1ID,
					Name:     "user1-proxy",
					Protocol: "vmess",
					Port:     10000 + i,
					Enabled:  true,
				})
			}

			// Create proxies for user2
			for i := 0; i < numUser2Proxies; i++ {
				repo.Create(context.Background(), &repository.Proxy{
					UserID:   user2ID,
					Name:     "user2-proxy",
					Protocol: "vmess",
					Port:     20000 + i,
					Enabled:  true,
				})
			}

			router, handler := setupTestRouter(repo)

			router.GET("/proxies", func(c *gin.Context) {
				setUserContext(c, adminID, "admin") // Admin user
				handler.List(c)
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				return false
			}

			var response []ProxyResponse
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return false
			}

			// Admin should see all proxies
			expectedTotal := numUser1Proxies + numUser2Proxies
			return len(response) == expectedTotal
		},
		gen.Int64Range(1, 50),
		gen.Int64Range(51, 100),
		gen.Int64Range(101, 150),
		gen.IntRange(0, 3),
		gen.IntRange(0, 3),
	))

	properties.TestingRun(t)
}

// TestProxyUserAssociation_NonAdminCannotAccessOthersProxy tests that non-admin users
// cannot access proxies belonging to other users.
func TestProxyUserAssociation_NonAdminCannotAccessOthersProxy(t *testing.T) {
	repo := newMockProxyRepository()

	// Create a proxy for user 1
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "user1-proxy",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	// User 2 tries to access user 1's proxy
	router.GET("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 2, "user") // User 2 (non-admin)
		handler.Get(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 403 Forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestProxyUserAssociation_AdminCanAccessAnyProxy tests that admin users
// can access any proxy regardless of owner.
func TestProxyUserAssociation_AdminCanAccessAnyProxy(t *testing.T) {
	repo := newMockProxyRepository()

	// Create a proxy for user 1
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "user1-proxy",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	// Admin tries to access user 1's proxy
	router.GET("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 999, "admin") // Admin user
		handler.Get(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestProxyUserAssociation_NonAdminCannotDeleteOthersProxy tests that non-admin users
// cannot delete proxies belonging to other users.
func TestProxyUserAssociation_NonAdminCannotDeleteOthersProxy(t *testing.T) {
	repo := newMockProxyRepository()

	// Create a proxy for user 1
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "user1-proxy",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	// User 2 tries to delete user 1's proxy
	router.DELETE("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 2, "user") // User 2 (non-admin)
		handler.Delete(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/proxies/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 403 Forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Verify proxy still exists
	p, _ := repo.GetByID(context.Background(), 1)
	assert.NotNil(t, p)
}

// TestProxyUserAssociation_UserCanDeleteOwnProxy tests that users can delete their own proxies.
func TestProxyUserAssociation_UserCanDeleteOwnProxy(t *testing.T) {
	repo := newMockProxyRepository()

	// Create a proxy for user 1
	repo.Create(context.Background(), &repository.Proxy{
		UserID:   1,
		Name:     "user1-proxy",
		Protocol: "vmess",
		Port:     10000,
		Enabled:  true,
	})

	router, handler := setupTestRouter(repo)

	// User 1 tries to delete their own proxy
	router.DELETE("/proxies/:id", func(c *gin.Context) {
		setUserContext(c, 1, "user") // User 1 (owner)
		handler.Delete(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/proxies/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 200 OK
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify proxy is deleted
	p, _ := repo.GetByID(context.Background(), 1)
	assert.Nil(t, p)
}
