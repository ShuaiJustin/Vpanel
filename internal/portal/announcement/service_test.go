// Package announcement provides announcement services for the user portal.
package announcement

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database/repository"
)

// mockAnnouncementRepo is a mock implementation of AnnouncementRepository for testing.
type mockAnnouncementRepo struct {
	announcements map[int64]*repository.Announcement
	readStatus    map[int64]map[int64]bool // userID -> announcementID -> isRead
	nextID        int64
}

func newMockAnnouncementRepo() *mockAnnouncementRepo {
	return &mockAnnouncementRepo{
		announcements: make(map[int64]*repository.Announcement),
		readStatus:    make(map[int64]map[int64]bool),
		nextID:        1,
	}
}

func (m *mockAnnouncementRepo) Create(ctx context.Context, announcement *repository.Announcement) error {
	announcement.ID = m.nextID
	m.nextID++
	m.announcements[announcement.ID] = announcement
	return nil
}

func (m *mockAnnouncementRepo) GetByID(ctx context.Context, id int64) (*repository.Announcement, error) {
	if a, ok := m.announcements[id]; ok {
		return a, nil
	}
	return nil, &notFoundError{id: id}
}

func (m *mockAnnouncementRepo) Update(ctx context.Context, announcement *repository.Announcement) error {
	m.announcements[announcement.ID] = announcement
	return nil
}

func (m *mockAnnouncementRepo) Delete(ctx context.Context, id int64) error {
	delete(m.announcements, id)
	return nil
}

func (m *mockAnnouncementRepo) List(ctx context.Context, filter *repository.AnnouncementFilter) ([]*repository.Announcement, int64, error) {
	var results []*repository.Announcement
	for _, a := range m.announcements {
		if filter != nil && filter.IsPublished != nil && *filter.IsPublished != a.IsPublished {
			continue
		}
		if filter != nil && filter.Category != nil && *filter.Category != a.Category {
			continue
		}
		results = append(results, a)
	}
	return results, int64(len(results)), nil
}

func (m *mockAnnouncementRepo) ListPublished(ctx context.Context, limit, offset int) ([]*repository.Announcement, int64, error) {
	isPublished := true
	return m.List(ctx, &repository.AnnouncementFilter{IsPublished: &isPublished, Limit: limit, Offset: offset})
}

func (m *mockAnnouncementRepo) ListWithReadStatus(ctx context.Context, userID int64, limit, offset int) ([]*repository.AnnouncementWithReadStatus, int64, error) {
	var results []*repository.AnnouncementWithReadStatus
	for _, a := range m.announcements {
		if !a.IsPublished {
			continue
		}
		isRead := false
		if userReads, ok := m.readStatus[userID]; ok {
			isRead = userReads[a.ID]
		}
		results = append(results, &repository.AnnouncementWithReadStatus{
			Announcement: *a,
			IsRead:       isRead,
		})
	}
	return results, int64(len(results)), nil
}

func (m *mockAnnouncementRepo) Publish(ctx context.Context, id int64) error {
	if a, ok := m.announcements[id]; ok {
		a.IsPublished = true
		now := time.Now()
		a.PublishedAt = &now
	}
	return nil
}

func (m *mockAnnouncementRepo) Unpublish(ctx context.Context, id int64) error {
	if a, ok := m.announcements[id]; ok {
		a.IsPublished = false
	}
	return nil
}

func (m *mockAnnouncementRepo) MarkAsRead(ctx context.Context, userID, announcementID int64) error {
	if _, ok := m.readStatus[userID]; !ok {
		m.readStatus[userID] = make(map[int64]bool)
	}
	m.readStatus[userID][announcementID] = true
	return nil
}

func (m *mockAnnouncementRepo) IsRead(ctx context.Context, userID, announcementID int64) (bool, error) {
	if userReads, ok := m.readStatus[userID]; ok {
		return userReads[announcementID], nil
	}
	return false, nil
}

func (m *mockAnnouncementRepo) GetUnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	for _, a := range m.announcements {
		if !a.IsPublished {
			continue
		}
		if userReads, ok := m.readStatus[userID]; ok {
			if !userReads[a.ID] {
				count++
			}
		} else {
			count++
		}
	}
	return count, nil
}

func (m *mockAnnouncementRepo) GetRecent(ctx context.Context, limit int) ([]*repository.Announcement, error) {
	var results []*repository.Announcement
	for _, a := range m.announcements {
		if a.IsPublished {
			results = append(results, a)
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

type notFoundError struct {
	id int64
}

func (e *notFoundError) Error() string {
	return "not found"
}

// Unit tests

func TestService_MarkAsRead(t *testing.T) {
	repo := newMockAnnouncementRepo()
	service := NewService(repo)
	ctx := context.Background()

	// Create a published announcement
	now := time.Now()
	announcement := &repository.Announcement{
		Title:       "Test Announcement",
		Content:     "Test content",
		IsPublished: true,
		PublishedAt: &now,
	}
	repo.Create(ctx, announcement)

	// Mark as read
	err := service.MarkAsRead(ctx, 1, announcement.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify it's marked as read
	isRead, err := service.IsRead(ctx, 1, announcement.ID)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !isRead {
		t.Error("Expected announcement to be marked as read")
	}
}

func TestService_GetUnreadCount(t *testing.T) {
	repo := newMockAnnouncementRepo()
	service := NewService(repo)
	ctx := context.Background()

	// Create some published announcements
	now := time.Now()
	for i := 0; i < 5; i++ {
		announcement := &repository.Announcement{
			Title:       "Test Announcement",
			Content:     "Test content",
			IsPublished: true,
			PublishedAt: &now,
		}
		repo.Create(ctx, announcement)
	}

	// Check unread count
	count, err := service.GetUnreadCount(ctx, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 unread, got %d", count)
	}

	// Mark one as read
	service.MarkAsRead(ctx, 1, 1)

	// Check unread count again
	count, err = service.GetUnreadCount(ctx, 1)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 unread, got %d", count)
	}
}

// Feature: user-portal, Property 10: Announcement Read Status Tracking
// Validates: Requirements 9.4
// *For any* announcement marked as read by a user, subsequent queries SHALL return
// that announcement with read status true for that user.
func TestProperty_AnnouncementReadStatusTracking(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: After marking as read, IsRead returns true
	properties.Property("marking as read sets read status to true", prop.ForAll(
		func(userID, announcementID int64) bool {
			if userID <= 0 || announcementID <= 0 {
				return true
			}

			repo := newMockAnnouncementRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create a published announcement
			now := time.Now()
			announcement := &repository.Announcement{
				ID:          announcementID,
				Title:       "Test",
				Content:     "Content",
				IsPublished: true,
				PublishedAt: &now,
			}
			repo.announcements[announcementID] = announcement

			// Initially not read
			isRead, _ := service.IsRead(ctx, userID, announcementID)
			if isRead {
				return false // Should not be read initially
			}

			// Mark as read
			service.MarkAsRead(ctx, userID, announcementID)

			// Now should be read
			isRead, _ = service.IsRead(ctx, userID, announcementID)
			return isRead
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000),
	))

	// Property: Read status is user-specific
	properties.Property("read status is user-specific", prop.ForAll(
		func(user1ID, user2ID, announcementID int64) bool {
			if user1ID <= 0 || user2ID <= 0 || announcementID <= 0 || user1ID == user2ID {
				return true
			}

			repo := newMockAnnouncementRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create a published announcement
			now := time.Now()
			announcement := &repository.Announcement{
				ID:          announcementID,
				Title:       "Test",
				Content:     "Content",
				IsPublished: true,
				PublishedAt: &now,
			}
			repo.announcements[announcementID] = announcement

			// User 1 marks as read
			service.MarkAsRead(ctx, user1ID, announcementID)

			// User 1 should see it as read
			isRead1, _ := service.IsRead(ctx, user1ID, announcementID)
			// User 2 should see it as unread
			isRead2, _ := service.IsRead(ctx, user2ID, announcementID)

			return isRead1 && !isRead2
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000),
	))

	// Property: Marking as read is idempotent
	properties.Property("marking as read is idempotent", prop.ForAll(
		func(userID, announcementID int64, times int) bool {
			if userID <= 0 || announcementID <= 0 || times <= 0 || times > 10 {
				return true
			}

			repo := newMockAnnouncementRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create a published announcement
			now := time.Now()
			announcement := &repository.Announcement{
				ID:          announcementID,
				Title:       "Test",
				Content:     "Content",
				IsPublished: true,
				PublishedAt: &now,
			}
			repo.announcements[announcementID] = announcement

			// Mark as read multiple times
			for i := 0; i < times; i++ {
				service.MarkAsRead(ctx, userID, announcementID)
			}

			// Should still be read
			isRead, _ := service.IsRead(ctx, userID, announcementID)
			return isRead
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000),
		gen.IntRange(1, 10),
	))

	// Property: Unread count decreases when marking as read
	properties.Property("unread count decreases when marking as read", prop.ForAll(
		func(userID int64, numAnnouncements int) bool {
			if userID <= 0 || numAnnouncements <= 0 || numAnnouncements > 20 {
				return true
			}

			repo := newMockAnnouncementRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create published announcements
			now := time.Now()
			for i := 0; i < numAnnouncements; i++ {
				announcement := &repository.Announcement{
					Title:       "Test",
					Content:     "Content",
					IsPublished: true,
					PublishedAt: &now,
				}
				repo.Create(ctx, announcement)
			}

			// Initial unread count
			initialCount, _ := service.GetUnreadCount(ctx, userID)
			if initialCount != int64(numAnnouncements) {
				return false
			}

			// Mark first announcement as read
			service.MarkAsRead(ctx, userID, 1)

			// Unread count should decrease by 1
			newCount, _ := service.GetUnreadCount(ctx, userID)
			return newCount == initialCount-1
		},
		gen.Int64Range(1, 1000),
		gen.IntRange(1, 20),
	))

	// Property: Read status persists across queries
	properties.Property("read status persists across queries", prop.ForAll(
		func(userID, announcementID int64, numQueries int) bool {
			if userID <= 0 || announcementID <= 0 || numQueries <= 0 || numQueries > 10 {
				return true
			}

			repo := newMockAnnouncementRepo()
			service := NewService(repo)
			ctx := context.Background()

			// Create a published announcement
			now := time.Now()
			announcement := &repository.Announcement{
				ID:          announcementID,
				Title:       "Test",
				Content:     "Content",
				IsPublished: true,
				PublishedAt: &now,
			}
			repo.announcements[announcementID] = announcement

			// Mark as read
			service.MarkAsRead(ctx, userID, announcementID)

			// Query multiple times
			for i := 0; i < numQueries; i++ {
				isRead, _ := service.IsRead(ctx, userID, announcementID)
				if !isRead {
					return false
				}
			}

			return true
		},
		gen.Int64Range(1, 1000),
		gen.Int64Range(1, 1000),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

func (m *mockAnnouncementRepo) GetReadIDs(ctx context.Context, userID int64, ids []int64) ([]int64, error) {
	var read []int64
	for _, id := range ids {
		if m.readStatus[userID][id] {
			read = append(read, id)
		}
	}
	return read, nil
}
