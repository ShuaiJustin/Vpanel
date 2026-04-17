package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
)

// mockTrafficRepository is a mock implementation for testing
type mockTrafficRepository struct {
	traffic map[int64]*repository.Traffic
}

func newMockTrafficRepository() *mockTrafficRepository {
	return &mockTrafficRepository{
		traffic: make(map[int64]*repository.Traffic),
	}
}

func (m *mockTrafficRepository) Create(ctx context.Context, traffic *repository.Traffic) error {
	m.traffic[traffic.ProxyID] = traffic
	return nil
}

func (m *mockTrafficRepository) GetTotalByProxy(ctx context.Context, proxyID int64) (int64, int64, error) {
	if t, ok := m.traffic[proxyID]; ok {
		return t.Upload, t.Download, nil
	}
	return 0, 0, nil
}

func (m *mockTrafficRepository) GetByProxyID(ctx context.Context, proxyID int64, limit, offset int) ([]*repository.Traffic, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Traffic, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetByDateRange(ctx context.Context, start, end time.Time) ([]*repository.Traffic, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetTotalByUser(ctx context.Context, userID int64) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockTrafficRepository) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func (m *mockTrafficRepository) GetTotalTraffic(ctx context.Context) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockTrafficRepository) GetTotalTrafficByPeriod(ctx context.Context, start, end time.Time) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockTrafficRepository) GetTrafficByProtocol(ctx context.Context, start, end time.Time) ([]*repository.ProtocolTrafficStats, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetTrafficByUser(ctx context.Context, start, end time.Time, limit int) ([]*repository.UserTrafficStats, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetTrafficTimeline(ctx context.Context, start, end time.Time, interval string) ([]*repository.TrafficTimelinePoint, error) {
	return nil, nil
}

func (m *mockTrafficRepository) GetTrafficTimelineByUser(ctx context.Context, userID int64, start, end time.Time, interval string) ([]*repository.TrafficTimelinePoint, error) {
	return nil, nil
}

func TestProxyHandler_GetStats_CacheHit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock repositories
	proxyRepo := newMockProxyRepository()
	trafficRepo := newMockTrafficRepository()

	// Create test proxy
	proxy := &repository.Proxy{
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		UserID:   1,
		Enabled:  true,
	}
	if err := proxyRepo.Create(context.Background(), proxy); err != nil {
		t.Fatalf("failed to create test proxy: %v", err)
	}

	// Create test traffic
	traffic := &repository.Traffic{
		UserID:   1,
		ProxyID:  proxy.ID,
		Upload:   1000,
		Download: 2000,
	}
	if err := trafficRepo.Create(context.Background(), traffic); err != nil {
		t.Fatalf("failed to create test traffic: %v", err)
	}

	// Create cache
	testCache := cache.NewMemoryCache(cache.Config{
		DefaultTTL:     30 * time.Second,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	// Create handler with cache
	handler := NewProxyHandlerWithTraffic(nil, proxyRepo, trafficRepo, logger.NewNopLogger()).
		WithCache(testCache)

	router := gin.New()
	router.GET("/api/proxies/:id/stats", func(c *gin.Context) {
		// Mock authentication - set user as admin
		c.Set("user_id", int64(1))
		c.Set("role", "admin")
		handler.GetStats(c)
	})

	// First request - should query database and cache result
	req1 := httptest.NewRequest(http.MethodGet, "/api/proxies/"+strconv.FormatInt(proxy.ID, 10)+"/stats", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w1.Code, w1.Body.String())
	}

	var stats1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &stats1); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify stats
	if stats1["upload"].(float64) != 1000 {
		t.Errorf("expected upload 1000, got %v", stats1["upload"])
	}
	if stats1["download"].(float64) != 2000 {
		t.Errorf("expected download 2000, got %v", stats1["download"])
	}

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "/api/proxies/"+strconv.FormatInt(proxy.ID, 10)+"/stats", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w2.Code)
	}

	var stats2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &stats2); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify cached stats match
	if stats2["upload"].(float64) != stats1["upload"].(float64) {
		t.Errorf("cached upload doesn't match: expected %v, got %v", stats1["upload"], stats2["upload"])
	}
	if stats2["download"].(float64) != stats1["download"].(float64) {
		t.Errorf("cached download doesn't match: expected %v, got %v", stats1["download"], stats2["download"])
	}
}

func TestProxyHandler_GetStats_CacheFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create mock repositories
	proxyRepo := newMockProxyRepository()
	trafficRepo := newMockTrafficRepository()

	// Create test proxy
	proxy := &repository.Proxy{
		Name:     "test-proxy",
		Protocol: "vmess",
		Port:     10086,
		UserID:   1,
		Enabled:  true,
	}
	if err := proxyRepo.Create(context.Background(), proxy); err != nil {
		t.Fatalf("failed to create test proxy: %v", err)
	}

	// Create handler without cache (cache unavailable scenario)
	handler := NewProxyHandlerWithTraffic(nil, proxyRepo, trafficRepo, logger.NewNopLogger())

	router := gin.New()
	router.GET("/api/proxies/:id/stats", func(c *gin.Context) {
		// Mock authentication - set user as admin
		c.Set("user_id", int64(1))
		c.Set("role", "admin")
		handler.GetStats(c)
	})

	// Request should work even without cache
	req := httptest.NewRequest(http.MethodGet, "/api/proxies/"+strconv.FormatInt(proxy.ID, 10)+"/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Verify stats are returned (even if zero)
	if _, ok := stats["upload"]; !ok {
		t.Error("expected upload field in response")
	}
	if _, ok := stats["download"]; !ok {
		t.Error("expected download field in response")
	}
}
