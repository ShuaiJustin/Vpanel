package node

import (
	"context"
	"testing"
	"time"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
)

// TestTrafficService_GetTrafficByUser_CacheIntegration tests cache integration for user traffic stats
func TestTrafficService_GetTrafficByUser_CacheIntegration(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNopLogger()

	// Create in-memory cache
	memCache := cache.NewMemoryCache(cache.Config{
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
	})

	// Create mock repository
	mockRepo := &mockNodeTrafficRepo{
		getTotalByUserInRangeFunc: func(ctx context.Context, userID int64, start, end time.Time) (int64, int64, error) {
			// Return test data
			return 1000, 2000, nil
		},
	}

	// Create traffic service with cache
	service := &TrafficService{
		nodeTrafficRepo: mockRepo,
		cache:           memCache,
		logger:          log,
	}

	userID := int64(123)
	start := time.Now().Add(-24 * time.Hour)
	end := time.Now()

	// First call - should query database and cache result
	stats1, err := service.GetTrafficByUser(ctx, userID, start, end)
	if err != nil {
		t.Fatalf("GetTrafficByUser failed: %v", err)
	}

	if stats1.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, stats1.UserID)
	}
	if stats1.Upload != 1000 {
		t.Errorf("Expected Upload 1000, got %d", stats1.Upload)
	}
	if stats1.Download != 2000 {
		t.Errorf("Expected Download 2000, got %d", stats1.Download)
	}
	if stats1.Total != 3000 {
		t.Errorf("Expected Total 3000, got %d", stats1.Total)
	}

	// Verify cache was populated
	cacheKey := "user:traffic:123"
	exists, err := memCache.Exists(ctx, cacheKey)
	if err != nil {
		t.Fatalf("Cache Exists check failed: %v", err)
	}
	if !exists {
		t.Error("Expected cache key to exist after first call")
	}

	// Second call - should return cached result without querying database
	// Change the mock to return different data to verify cache is used
	mockRepo.getTotalByUserInRangeFunc = func(ctx context.Context, userID int64, start, end time.Time) (int64, int64, error) {
		t.Error("Database should not be queried on cache hit")
		return 9999, 9999, nil
	}

	stats2, err := service.GetTrafficByUser(ctx, userID, start, end)
	if err != nil {
		t.Fatalf("GetTrafficByUser (cached) failed: %v", err)
	}

	// Should return cached values, not new values
	if stats2.Upload != 1000 {
		t.Errorf("Expected cached Upload 1000, got %d", stats2.Upload)
	}
	if stats2.Download != 2000 {
		t.Errorf("Expected cached Download 2000, got %d", stats2.Download)
	}
}

// TestTrafficService_RecordTrafficBatch_InvalidatesUserCache tests cache invalidation on traffic recording
func TestTrafficService_RecordTrafficBatch_InvalidatesUserCache(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNopLogger()

	// Create in-memory cache
	memCache := cache.NewMemoryCache(cache.Config{
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 1000,
	})

	// Pre-populate cache with user traffic data
	userID := int64(456)
	cacheKey := "user:traffic:456"
	cachedData := []byte(`{"user_id":456,"upload":5000,"download":10000,"total":15000}`)
	err := memCache.Set(ctx, cacheKey, cachedData, 60*time.Second)
	if err != nil {
		t.Fatalf("Failed to populate cache: %v", err)
	}

	// Verify cache is populated
	exists, err := memCache.Exists(ctx, cacheKey)
	if err != nil {
		t.Fatalf("Cache Exists check failed: %v", err)
	}
	if !exists {
		t.Fatal("Cache should be populated before test")
	}

	// Create mock repositories
	mockNodeTrafficRepo := &mockNodeTrafficRepo{
		createBatchFunc: func(ctx context.Context, records []*repository.NodeTraffic) error {
			return nil
		},
	}

	// Create traffic service with cache
	service := &TrafficService{
		nodeTrafficRepo: mockNodeTrafficRepo,
		cache:           memCache,
		logger:          log,
	}

	// Record traffic for the user
	records := []*TrafficRecord{
		{
			NodeID:   1,
			UserID:   userID,
			Upload:   100,
			Download: 200,
		},
	}

	err = service.RecordTrafficBatch(ctx, records)
	if err != nil {
		t.Fatalf("RecordTrafficBatch failed: %v", err)
	}

	// Verify cache was invalidated
	exists, err = memCache.Exists(ctx, cacheKey)
	if err != nil {
		t.Fatalf("Cache Exists check failed: %v", err)
	}
	if exists {
		t.Error("Cache should be invalidated after traffic recording")
	}
}

// mockNodeTrafficRepo is a mock implementation of NodeTrafficRepository for testing
type mockNodeTrafficRepo struct {
	getTotalByUserInRangeFunc func(ctx context.Context, userID int64, start, end time.Time) (int64, int64, error)
	createBatchFunc           func(ctx context.Context, records []*repository.NodeTraffic) error
}

func (m *mockNodeTrafficRepo) GetTotalByUserInRange(ctx context.Context, userID int64, start, end time.Time) (int64, int64, error) {
	if m.getTotalByUserInRangeFunc != nil {
		return m.getTotalByUserInRangeFunc(ctx, userID, start, end)
	}
	return 0, 0, nil
}

func (m *mockNodeTrafficRepo) CreateBatch(ctx context.Context, records []*repository.NodeTraffic) error {
	if m.createBatchFunc != nil {
		return m.createBatchFunc(ctx, records)
	}
	return nil
}

// Implement other required methods with no-op implementations
func (m *mockNodeTrafficRepo) Create(ctx context.Context, traffic *repository.NodeTraffic) error {
	return nil
}

func (m *mockNodeTrafficRepo) GetByID(ctx context.Context, id int64) (*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetByNodeID(ctx context.Context, nodeID int64, limit, offset int) ([]*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetByDateRange(ctx context.Context, start, end time.Time) ([]*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetByUserAndDateRange(ctx context.Context, userID int64, start, end time.Time) ([]*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetTotalTraffic(ctx context.Context, start, end time.Time) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockNodeTrafficRepo) GetTotalByNodeInRange(ctx context.Context, nodeID int64, start, end time.Time) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockNodeTrafficRepo) GetTotalByUserOnNode(ctx context.Context, userID, nodeID int64) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockNodeTrafficRepo) GetStatsByNode(ctx context.Context, start, end time.Time) ([]*repository.NodeTrafficStats, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetStatsByGroup(ctx context.Context, start, end time.Time) ([]*repository.GroupTrafficStats, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetStatsByUser(ctx context.Context, nodeID int64, start, end time.Time, limit int) ([]*repository.UserNodeTrafficStats, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) DeleteByNodeID(ctx context.Context, nodeID int64) error {
	return nil
}

func (m *mockNodeTrafficRepo) DeleteOlderThan(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func (m *mockNodeTrafficRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockNodeTrafficRepo) GetByNodeAndDateRange(ctx context.Context, nodeID int64, start, end time.Time) ([]*repository.NodeTraffic, error) {
	return nil, nil
}

func (m *mockNodeTrafficRepo) GetTotalByNode(ctx context.Context, nodeID int64) (int64, int64, error) {
	return 0, 0, nil
}

func (m *mockNodeTrafficRepo) GetTotalByUser(ctx context.Context, userID int64) (int64, int64, error) {
	return 0, 0, nil
}
