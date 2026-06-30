package node

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
)

// mockCache is a simple in-memory cache for testing
type mockCache struct {
	data map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{
		data: make(map[string][]byte),
	}
}

func (m *mockCache) Get(ctx context.Context, key string) ([]byte, error) {
	if val, ok := m.data[key]; ok {
		return val, nil
	}
	return nil, cache.ErrCacheMiss
}

func (m *mockCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	m.data[key] = value
	return nil
}

func (m *mockCache) Delete(ctx context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockCache) Exists(ctx context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

func (m *mockCache) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	result := make(map[string][]byte)
	for _, key := range keys {
		if val, ok := m.data[key]; ok {
			result[key] = val
		}
	}
	return result, nil
}

func (m *mockCache) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	for key, val := range items {
		m.data[key] = val
	}
	return nil
}

func (m *mockCache) InvalidatePattern(ctx context.Context, pattern string) error {
	return nil
}

func (m *mockCache) Ping(ctx context.Context) error {
	return nil
}

func (m *mockCache) Close() error {
	return nil
}

// mockNodeRepo is a mock node repository for testing
type mockNodeRepo struct {
	countByStatusCalls int
	stats              map[string]int64
	created            *repository.Node
}

func (m *mockNodeRepo) CountByStatus(ctx context.Context) (map[string]int64, error) {
	m.countByStatusCalls++
	return m.stats, nil
}

func (m *mockNodeRepo) Count(ctx context.Context, filter *repository.NodeFilter) (int64, error) {
	return 0, nil
}

func (m *mockNodeRepo) CountByAddressPort(ctx context.Context, address string, port int, excludeID int64) (int64, error) {
	return 0, nil
}

func (m *mockNodeRepo) Create(ctx context.Context, node *repository.Node) error {
	m.created = node
	node.ID = 1
	return nil
}

func (m *mockNodeRepo) GetByID(ctx context.Context, id int64) (*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetByIDs(ctx context.Context, ids []int64) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) Update(ctx context.Context, node *repository.Node) error {
	return nil
}

func (m *mockNodeRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	return nil
}

func (m *mockNodeRepo) UpdateMetrics(ctx context.Context, id int64, latency int, currentUsers int) error {
	return nil
}

func (m *mockNodeRepo) UpdateLastSeen(ctx context.Context, id int64, lastSeen time.Time) error {
	return nil
}

func (m *mockNodeRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockNodeRepo) DeleteInTx(ctx context.Context, id int64) error {
	return nil
}

func (m *mockNodeRepo) List(ctx context.Context, filter *repository.NodeFilter) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) ListWithTotal(ctx context.Context, filter *repository.NodeFilter) ([]*repository.Node, int64, error) {
	return nil, 0, nil
}

func (m *mockNodeRepo) GetByToken(ctx context.Context, token string) (*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetHealthyNodes(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetTotalUsers(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockNodeRepo) Transaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (m *mockNodeRepo) UpdateToken(ctx context.Context, id int64, token string) error {
	return nil
}

func (m *mockNodeRepo) UpdateLoadMetrics(ctx context.Context, id int64, cpuUsage, memoryUsage, diskUsage float64) error {
	return nil
}

func (m *mockNodeRepo) UpdateXrayStatus(ctx context.Context, id int64, xrayRunning bool, xrayVersion string) error {
	return nil
}

func (m *mockNodeRepo) UpdateInstallStatus(ctx context.Context, id int64, status, message, steps, logs string, startedAt, finishedAt, updatedAt *time.Time) error {
	return nil
}

func (m *mockNodeRepo) UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error {
	return nil
}

func (m *mockNodeRepo) GetPendingSync(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetByStatus(ctx context.Context, status string) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetByRegion(ctx context.Context, region string) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetHealthy(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetOnline(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) GetAvailable(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) CountByName(ctx context.Context, name string, excludeID int64) (int64, error) {
	return 0, nil
}

func (m *mockNodeRepo) GetAvailableInTx(ctx context.Context) ([]*repository.Node, error) {
	return nil, nil
}

func (m *mockNodeRepo) UpdateHeartbeatBatch(ctx context.Context, id int64, status string, lastSeen time.Time, latency int, currentUsers int, cpuUsage, memoryUsage, diskUsage float64, xrayRunning bool, xrayVersion string) error {
	return nil
}

func TestGetStatistics_WithCache(t *testing.T) {
	ctx := context.Background()
	mockCache := newMockCache()
	mockRepo := &mockNodeRepo{
		stats: map[string]int64{
			"active":   10,
			"inactive": 5,
			"offline":  2,
		},
	}

	service := NewService(mockRepo, nil, nil, logger.NewNopLogger()).WithCache(mockCache)

	// First call - should hit database and cache the result
	stats1, err := service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if mockRepo.countByStatusCalls != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.countByStatusCalls)
	}

	if stats1["active"] != 10 || stats1["inactive"] != 5 || stats1["offline"] != 2 {
		t.Errorf("Unexpected stats: %v", stats1)
	}

	// Second call - should hit cache, not database
	stats2, err := service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if mockRepo.countByStatusCalls != 1 {
		t.Errorf("Expected 1 database call (cached), got %d", mockRepo.countByStatusCalls)
	}

	if stats2["active"] != 10 || stats2["inactive"] != 5 || stats2["offline"] != 2 {
		t.Errorf("Unexpected cached stats: %v", stats2)
	}
}

func TestCreate_DefaultsTLSDomainWhenTLSEnabled(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockNodeRepo{}
	service := NewService(mockRepo, nil, nil, logger.NewNopLogger())

	created, err := service.Create(ctx, &CreateNodeRequest{
		Name:       "test-node",
		Address:    "203.0.113.10",
		TLSEnabled: true,
		Protocols:  []string{"vmess"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if created.TLSDomain != DefaultTLSDomain {
		t.Fatalf("expected returned TLS domain %q, got %q", DefaultTLSDomain, created.TLSDomain)
	}
	if mockRepo.created == nil {
		t.Fatal("expected node to be persisted")
	}
	if mockRepo.created.TLSDomain != DefaultTLSDomain {
		t.Fatalf("expected persisted TLS domain %q, got %q", DefaultTLSDomain, mockRepo.created.TLSDomain)
	}
}

func TestGetStatistics_CacheInvalidation(t *testing.T) {
	ctx := context.Background()
	mockCache := newMockCache()
	mockRepo := &mockNodeRepo{
		stats: map[string]int64{
			"active":   10,
			"inactive": 5,
		},
	}

	service := NewService(mockRepo, nil, nil, logger.NewNopLogger()).WithCache(mockCache)

	// First call - cache the result
	_, err := service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if mockRepo.countByStatusCalls != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.countByStatusCalls)
	}

	// Update status - should invalidate cache
	err = service.UpdateStatus(ctx, 1, "offline")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}

	// Verify cache was invalidated
	cacheKey := "node:stats:all"
	exists, _ := mockCache.Exists(ctx, cacheKey)
	if exists {
		t.Error("Cache should have been invalidated after UpdateStatus")
	}

	// Next call should hit database again
	_, err = service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if mockRepo.countByStatusCalls != 2 {
		t.Errorf("Expected 2 database calls after cache invalidation, got %d", mockRepo.countByStatusCalls)
	}
}

func TestGetStatistics_WithoutCache(t *testing.T) {
	ctx := context.Background()
	mockRepo := &mockNodeRepo{
		stats: map[string]int64{
			"active": 10,
		},
	}

	// Service without cache
	service := NewService(mockRepo, nil, nil, logger.NewNopLogger())

	// Each call should hit database
	_, err := service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	_, err = service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	if mockRepo.countByStatusCalls != 2 {
		t.Errorf("Expected 2 database calls without cache, got %d", mockRepo.countByStatusCalls)
	}
}

func TestGetStatistics_CacheKeyFormat(t *testing.T) {
	ctx := context.Background()
	mockCache := newMockCache()
	mockRepo := &mockNodeRepo{
		stats: map[string]int64{
			"active": 10,
		},
	}

	service := NewService(mockRepo, nil, nil, logger.NewNopLogger()).WithCache(mockCache)

	// Call GetStatistics
	_, err := service.GetStatistics(ctx)
	if err != nil {
		t.Fatalf("GetStatistics failed: %v", err)
	}

	// Verify cache key format
	cacheKey := "node:stats:all"
	data, err := mockCache.Get(ctx, cacheKey)
	if err != nil {
		t.Fatalf("Cache key not found: %v", err)
	}

	// Verify cached data is valid JSON
	var stats map[string]int64
	if err := json.Unmarshal(data, &stats); err != nil {
		t.Fatalf("Cached data is not valid JSON: %v", err)
	}

	if stats["active"] != 10 {
		t.Errorf("Cached stats incorrect: %v", stats)
	}
}
