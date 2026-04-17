package proxy

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"v/internal/cache"
	"v/internal/database/repository"
)

// mockProxyRepository is a mock implementation of ProxyRepository for testing
type mockProxyRepository struct {
	proxies      map[int64]*repository.Proxy
	getCallCount int
}

func newMockProxyRepository() *mockProxyRepository {
	return &mockProxyRepository{
		proxies: make(map[int64]*repository.Proxy),
	}
}

func (m *mockProxyRepository) Create(ctx context.Context, proxy *repository.Proxy) error {
	proxy.ID = int64(len(m.proxies) + 1)
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepository) GetByID(ctx context.Context, id int64) (*repository.Proxy, error) {
	m.getCallCount++
	if proxy, ok := m.proxies[id]; ok {
		return proxy, nil
	}
	return nil, nil
}

func (m *mockProxyRepository) Update(ctx context.Context, proxy *repository.Proxy) error {
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepository) Delete(ctx context.Context, id int64) error {
	delete(m.proxies, id)
	return nil
}

func (m *mockProxyRepository) List(ctx context.Context, limit, offset int) ([]*repository.Proxy, error) {
	return nil, nil
}

func (m *mockProxyRepository) GetByProtocol(ctx context.Context, protocol string) ([]*repository.Proxy, error) {
	return nil, nil
}

func (m *mockProxyRepository) GetEnabled(ctx context.Context) ([]*repository.Proxy, error) {
	return nil, nil
}

func (m *mockProxyRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Proxy, error) {
	return nil, nil
}

func (m *mockProxyRepository) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	return 0, nil
}

func (m *mockProxyRepository) GetByPort(ctx context.Context, port int) (*repository.Proxy, error) {
	return nil, nil
}

func (m *mockProxyRepository) EnableByUserID(ctx context.Context, userID int64) error {
	return nil
}

func (m *mockProxyRepository) DisableByUserID(ctx context.Context, userID int64) error {
	return nil
}

func (m *mockProxyRepository) DeleteByIDs(ctx context.Context, ids []int64) error {
	return nil
}

func (m *mockProxyRepository) Count(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockProxyRepository) CountEnabled(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockProxyRepository) CountByProtocol(ctx context.Context) ([]*repository.ProtocolCount, error) {
	return nil, nil
}

func (m *mockProxyRepository) GetByNodeID(ctx context.Context, nodeID int64) ([]*repository.Proxy, error) {
	return nil, nil
}

// TestProxyConfigCaching tests that proxy configurations are cached correctly
func TestProxyConfigCaching(t *testing.T) {
	ctx := context.Background()

	// Create mock repository and cache
	mockRepo := newMockProxyRepository()
	memCache := cache.NewMemoryCache(cache.Config{
		MaxMemoryItems: 100,
		DefaultTTL:     5 * time.Minute,
	})

	// Create manager with cache
	mgr := NewManagerWithCache(mockRepo, memCache).(*manager)

	// Create a test proxy in the repository
	testProxy := &repository.Proxy{
		ID:       1,
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		Host:     "example.com",
		Settings: map[string]any{"uuid": "test-uuid"},
		Enabled:  true,
		Remark:   "test proxy",
	}
	mockRepo.proxies[1] = testProxy

	// First call - should hit database
	settings1, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}
	if settings1.Name != "test-proxy" {
		t.Errorf("Expected name 'test-proxy', got '%s'", settings1.Name)
	}
	if mockRepo.getCallCount != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.getCallCount)
	}

	// Second call - should hit cache
	settings2, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}
	if settings2.Name != "test-proxy" {
		t.Errorf("Expected name 'test-proxy', got '%s'", settings2.Name)
	}
	if mockRepo.getCallCount != 1 {
		t.Errorf("Expected 1 database call (cached), got %d", mockRepo.getCallCount)
	}

	// Verify cache hit
	cacheKey := "proxy:config:1"
	data, err := memCache.Get(ctx, cacheKey)
	if err != nil {
		t.Errorf("Cache should contain the proxy config: %v", err)
	}
	if data == nil {
		t.Error("Cache data should not be nil")
	}
}

// TestProxyConfigCacheInvalidation tests that cache is invalidated on update
func TestProxyConfigCacheInvalidation(t *testing.T) {
	ctx := context.Background()

	// Create mock repository and cache
	mockRepo := newMockProxyRepository()
	memCache := cache.NewMemoryCache(cache.Config{
		MaxMemoryItems: 100,
		DefaultTTL:     5 * time.Minute,
	})

	// Create manager with cache
	mgr := NewManagerWithCache(mockRepo, memCache).(*manager)

	// Register a mock protocol for validation
	mockProtocol := &mockProtocol{name: "vmess"}
	mgr.RegisterProtocol(mockProtocol)

	// Create a test proxy in the repository
	testProxy := &repository.Proxy{
		ID:       1,
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		Host:     "example.com",
		Settings: map[string]any{"uuid": "test-uuid"},
		Enabled:  true,
		Remark:   "test proxy",
	}
	mockRepo.proxies[1] = testProxy

	// First call - populate cache
	_, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}

	// Verify cache is populated
	cacheKey := "proxy:config:1"
	_, err = memCache.Get(ctx, cacheKey)
	if err != nil {
		t.Errorf("Cache should contain the proxy config before update: %v", err)
	}

	// Update the proxy
	updatedSettings := &Settings{
		ID:       1,
		Name:     "updated-proxy",
		Protocol: "vmess",
		Port:     10087,
		Host:     "example.com",
		Settings: map[string]any{"uuid": "test-uuid"},
		Enabled:  true,
		Remark:   "updated proxy",
	}
	err = mgr.UpdateProxy(ctx, updatedSettings)
	if err != nil {
		t.Fatalf("UpdateProxy failed: %v", err)
	}

	// Verify cache is invalidated
	_, err = memCache.Get(ctx, cacheKey)
	if err != cache.ErrCacheMiss {
		t.Errorf("Cache should be invalidated after update, got error: %v", err)
	}

	// Next GetProxy should hit database again
	mockRepo.getCallCount = 0
	settings, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}
	if settings.Name != "updated-proxy" {
		t.Errorf("Expected name 'updated-proxy', got '%s'", settings.Name)
	}
	if mockRepo.getCallCount != 1 {
		t.Errorf("Expected 1 database call after cache invalidation, got %d", mockRepo.getCallCount)
	}
}

// TestProxyConfigCacheInvalidationOnDelete tests that cache is invalidated on delete
func TestProxyConfigCacheInvalidationOnDelete(t *testing.T) {
	ctx := context.Background()

	// Create mock repository and cache
	mockRepo := newMockProxyRepository()
	memCache := cache.NewMemoryCache(cache.Config{
		MaxMemoryItems: 100,
		DefaultTTL:     5 * time.Minute,
	})

	// Create manager with cache
	mgr := NewManagerWithCache(mockRepo, memCache).(*manager)

	// Create a test proxy in the repository
	testProxy := &repository.Proxy{
		ID:       1,
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		Host:     "example.com",
		Settings: map[string]any{"uuid": "test-uuid"},
		Enabled:  true,
		Remark:   "test proxy",
	}
	mockRepo.proxies[1] = testProxy

	// First call - populate cache
	_, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}

	// Verify cache is populated
	cacheKey := "proxy:config:1"
	_, err = memCache.Get(ctx, cacheKey)
	if err != nil {
		t.Errorf("Cache should contain the proxy config before delete: %v", err)
	}

	// Delete the proxy
	err = mgr.DeleteProxy(ctx, 1)
	if err != nil {
		t.Fatalf("DeleteProxy failed: %v", err)
	}

	// Verify cache is invalidated
	_, err = memCache.Get(ctx, cacheKey)
	if err != cache.ErrCacheMiss {
		t.Errorf("Cache should be invalidated after delete, got error: %v", err)
	}
}

// TestProxyConfigWithoutCache tests that manager works without cache
func TestProxyConfigWithoutCache(t *testing.T) {
	ctx := context.Background()

	// Create mock repository without cache
	mockRepo := newMockProxyRepository()

	// Create manager without cache
	mgr := NewManager(mockRepo).(*manager)

	// Create a test proxy in the repository
	testProxy := &repository.Proxy{
		ID:       1,
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		Host:     "example.com",
		Settings: map[string]any{"uuid": "test-uuid"},
		Enabled:  true,
		Remark:   "test proxy",
	}
	mockRepo.proxies[1] = testProxy

	// First call
	settings1, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}
	if settings1.Name != "test-proxy" {
		t.Errorf("Expected name 'test-proxy', got '%s'", settings1.Name)
	}
	if mockRepo.getCallCount != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.getCallCount)
	}

	// Second call - should hit database again (no cache)
	settings2, err := mgr.GetProxy(ctx, 1)
	if err != nil {
		t.Fatalf("GetProxy failed: %v", err)
	}
	if settings2.Name != "test-proxy" {
		t.Errorf("Expected name 'test-proxy', got '%s'", settings2.Name)
	}
	if mockRepo.getCallCount != 2 {
		t.Errorf("Expected 2 database calls (no cache), got %d", mockRepo.getCallCount)
	}
}

// mockProtocol is a mock implementation of Protocol for testing
type mockProtocol struct {
	name string
}

func (m *mockProtocol) Name() string {
	return m.name
}

func (m *mockProtocol) Validate(settings *Settings) error {
	return nil
}

func (m *mockProtocol) GenerateLink(settings *Settings) (string, error) {
	return "", nil
}

func (m *mockProtocol) GenerateConfig(settings *Settings) (json.RawMessage, error) {
	return nil, nil
}

func (m *mockProtocol) ParseLink(link string) (*Settings, error) {
	return nil, nil
}

func (m *mockProtocol) DefaultSettings() map[string]any {
	return make(map[string]any)
}

