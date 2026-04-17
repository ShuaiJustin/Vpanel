package announcement

import (
	"context"
	"testing"
	"time"

	"v/internal/cache"
	"v/internal/database/repository"
)

// mockAnnouncementRepository is a mock implementation for testing
type mockAnnouncementRepository struct {
	listWithReadStatusCalls int
	announcements           []*repository.AnnouncementWithReadStatus
	total                   int64
	err                     error
}

func (m *mockAnnouncementRepository) ListWithReadStatus(ctx context.Context, userID int64, limit, offset int) ([]*repository.AnnouncementWithReadStatus, int64, error) {
	m.listWithReadStatusCalls++
	return m.announcements, m.total, m.err
}

func (m *mockAnnouncementRepository) IsRead(ctx context.Context, userID, announcementID int64) (bool, error) {
	return false, nil
}

func (m *mockAnnouncementRepository) Create(ctx context.Context, announcement *repository.Announcement) error {
	return nil
}

func (m *mockAnnouncementRepository) GetByID(ctx context.Context, id int64) (*repository.Announcement, error) {
	return nil, nil
}

func (m *mockAnnouncementRepository) Update(ctx context.Context, announcement *repository.Announcement) error {
	return nil
}

func (m *mockAnnouncementRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAnnouncementRepository) List(ctx context.Context, filter *repository.AnnouncementFilter) ([]*repository.Announcement, int64, error) {
	return nil, 0, nil
}

func (m *mockAnnouncementRepository) ListPublished(ctx context.Context, limit, offset int) ([]*repository.Announcement, int64, error) {
	return nil, 0, nil
}

func (m *mockAnnouncementRepository) Publish(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAnnouncementRepository) Unpublish(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAnnouncementRepository) MarkAsRead(ctx context.Context, userID, announcementID int64) error {
	return nil
}

func (m *mockAnnouncementRepository) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	return 0, nil
}

func (m *mockAnnouncementRepository) GetRecent(ctx context.Context, limit int) ([]*repository.Announcement, error) {
	return nil, nil
}

// TestListAnnouncementsWithCache verifies that announcement lists are cached
func TestListAnnouncementsWithCache(t *testing.T) {
	ctx := context.Background()

	// Create test announcements
	testAnnouncements := []*repository.AnnouncementWithReadStatus{
		{
			Announcement: repository.Announcement{
				ID:          1,
				Title:       "Test Announcement 1",
				Content:     "Content 1",
				IsPublished: true,
			},
			IsRead: false,
		},
		{
			Announcement: repository.Announcement{
				ID:          2,
				Title:       "Test Announcement 2",
				Content:     "Content 2",
				IsPublished: true,
			},
			IsRead: false,
		},
	}

	mockRepo := &mockAnnouncementRepository{
		announcements: testAnnouncements,
		total:         2,
	}

	// Create cache
	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	// Create service with cache
	service := NewService(mockRepo).WithCache(testCache)

	// First call - should hit database and cache the result
	results1, total1, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if len(results1) != 2 {
		t.Errorf("Expected 2 announcements, got %d", len(results1))
	}

	if total1 != 2 {
		t.Errorf("Expected total 2, got %d", total1)
	}

	if mockRepo.listWithReadStatusCalls != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.listWithReadStatusCalls)
	}

	// Second call with same parameters - should hit cache
	results2, total2, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if len(results2) != 2 {
		t.Errorf("Expected 2 announcements, got %d", len(results2))
	}

	if total2 != 2 {
		t.Errorf("Expected total 2, got %d", total2)
	}

	// Should still be 1 database call (cache hit)
	if mockRepo.listWithReadStatusCalls != 1 {
		t.Errorf("Expected 1 database call (cache hit), got %d", mockRepo.listWithReadStatusCalls)
	}

	// Third call with different user - should hit cache for announcements but check read status
	results3, total3, err := service.ListAnnouncements(ctx, 2, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if len(results3) != 2 {
		t.Errorf("Expected 2 announcements, got %d", len(results3))
	}

	if total3 != 2 {
		t.Errorf("Expected total 2, got %d", total3)
	}

	// Should still be 1 database call for list (cache hit for public data)
	if mockRepo.listWithReadStatusCalls != 1 {
		t.Errorf("Expected 1 database call (cache hit), got %d", mockRepo.listWithReadStatusCalls)
	}
}

// TestListAnnouncementsWithoutCache verifies service works without cache
func TestListAnnouncementsWithoutCache(t *testing.T) {
	ctx := context.Background()

	testAnnouncements := []*repository.AnnouncementWithReadStatus{
		{
			Announcement: repository.Announcement{
				ID:          1,
				Title:       "Test Announcement 1",
				Content:     "Content 1",
				IsPublished: true,
			},
			IsRead: false,
		},
	}

	mockRepo := &mockAnnouncementRepository{
		announcements: testAnnouncements,
		total:         1,
	}

	// Create service without cache
	service := NewService(mockRepo)

	// First call
	results1, total1, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if len(results1) != 1 {
		t.Errorf("Expected 1 announcement, got %d", len(results1))
	}

	if total1 != 1 {
		t.Errorf("Expected total 1, got %d", total1)
	}

	if mockRepo.listWithReadStatusCalls != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.listWithReadStatusCalls)
	}

	// Second call - should hit database again (no cache)
	results2, total2, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if len(results2) != 1 {
		t.Errorf("Expected 1 announcement, got %d", len(results2))
	}

	if total2 != 1 {
		t.Errorf("Expected total 1, got %d", total2)
	}

	// Should be 2 database calls (no cache)
	if mockRepo.listWithReadStatusCalls != 2 {
		t.Errorf("Expected 2 database calls (no cache), got %d", mockRepo.listWithReadStatusCalls)
	}
}

// TestInvalidateCache verifies cache invalidation works
func TestInvalidateCache(t *testing.T) {
	ctx := context.Background()

	testAnnouncements := []*repository.AnnouncementWithReadStatus{
		{
			Announcement: repository.Announcement{
				ID:          1,
				Title:       "Test Announcement",
				Content:     "Content",
				IsPublished: true,
			},
			IsRead: false,
		},
	}

	mockRepo := &mockAnnouncementRepository{
		announcements: testAnnouncements,
		total:         1,
	}

	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	service := NewService(mockRepo).WithCache(testCache)

	// First call - cache the result
	_, _, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if mockRepo.listWithReadStatusCalls != 1 {
		t.Errorf("Expected 1 database call, got %d", mockRepo.listWithReadStatusCalls)
	}

	// Invalidate cache
	err = service.InvalidateCache(ctx)
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	// Second call - should hit database again (cache invalidated)
	_, _, err = service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	if mockRepo.listWithReadStatusCalls != 2 {
		t.Errorf("Expected 2 database calls (cache invalidated), got %d", mockRepo.listWithReadStatusCalls)
	}
}

// TestCacheKeyDifferentPages verifies different pages use different cache keys
func TestCacheKeyDifferentPages(t *testing.T) {
	ctx := context.Background()

	testAnnouncements := []*repository.AnnouncementWithReadStatus{
		{
			Announcement: repository.Announcement{
				ID:          1,
				Title:       "Test Announcement",
				Content:     "Content",
				IsPublished: true,
			},
			IsRead: false,
		},
	}

	mockRepo := &mockAnnouncementRepository{
		announcements: testAnnouncements,
		total:         1,
	}

	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	service := NewService(mockRepo).WithCache(testCache)

	// Call with page 0 (offset 0, limit 20)
	_, _, err := service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	// Call with page 1 (offset 20, limit 20) - different cache key
	_, _, err = service.ListAnnouncements(ctx, 1, 20, 20)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	// Should be 2 database calls (different pages)
	if mockRepo.listWithReadStatusCalls != 2 {
		t.Errorf("Expected 2 database calls (different pages), got %d", mockRepo.listWithReadStatusCalls)
	}

	// Call with page 0 again - should hit cache
	_, _, err = service.ListAnnouncements(ctx, 1, 20, 0)
	if err != nil {
		t.Fatalf("ListAnnouncements failed: %v", err)
	}

	// Should still be 2 database calls (cache hit for page 0)
	if mockRepo.listWithReadStatusCalls != 2 {
		t.Errorf("Expected 2 database calls (cache hit), got %d", mockRepo.listWithReadStatusCalls)
	}
}
