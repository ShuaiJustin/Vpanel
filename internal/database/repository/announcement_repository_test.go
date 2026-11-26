package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"v/pkg/errors"
)

func setupAnnouncementTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&User{}, &Announcement{}, &AnnouncementRead{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

func createAnnouncementTestUser(t *testing.T, db *gorm.DB, username string) int64 {
	user := &User{
		Username:     username,
		PasswordHash: "hashedpassword",
		Email:        username + "@example.com",
		Role:         "user",
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return user.ID
}

func TestAnnouncementRepository_Create(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	announcement := &Announcement{
		Title:       "Test Announcement",
		Content:     "This is a test announcement",
		Category:    AnnouncementCategoryGeneral,
		IsPublished: false,
	}

	err := repo.Create(ctx, announcement)
	if err != nil {
		t.Fatalf("Failed to create announcement: %v", err)
	}

	if announcement.ID == 0 {
		t.Error("Expected announcement ID to be set after creation")
	}
}

func TestAnnouncementRepository_GetByID(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	announcement := &Announcement{
		Title:       "Test Announcement",
		Content:     "This is a test announcement",
		Category:    AnnouncementCategoryGeneral,
		IsPublished: true,
	}
	repo.Create(ctx, announcement)

	found, err := repo.GetByID(ctx, announcement.ID)
	if err != nil {
		t.Fatalf("Failed to get announcement: %v", err)
	}

	if found.Title != announcement.Title {
		t.Errorf("Expected title %s, got %s", announcement.Title, found.Title)
	}

	_, err = repo.GetByID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestAnnouncementRepository_Publish(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	announcement := &Announcement{
		Title:       "Test Announcement",
		Content:     "This is a test announcement",
		Category:    AnnouncementCategoryGeneral,
		IsPublished: false,
	}
	repo.Create(ctx, announcement)

	err := repo.Publish(ctx, announcement.ID)
	if err != nil {
		t.Fatalf("Failed to publish announcement: %v", err)
	}

	found, _ := repo.GetByID(ctx, announcement.ID)
	if !found.IsPublished {
		t.Error("Expected announcement to be published")
	}
	if found.PublishedAt == nil {
		t.Error("Expected published_at to be set")
	}
}

func TestAnnouncementRepository_MarkAsRead(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	userID := createAnnouncementTestUser(t, db, "testuser")

	announcement := &Announcement{
		Title:       "Test Announcement",
		Content:     "This is a test announcement",
		Category:    AnnouncementCategoryGeneral,
		IsPublished: true,
	}
	repo.Create(ctx, announcement)

	// Mark as read
	err := repo.MarkAsRead(ctx, userID, announcement.ID)
	if err != nil {
		t.Fatalf("Failed to mark as read: %v", err)
	}

	// Check if read
	isRead, err := repo.IsRead(ctx, userID, announcement.ID)
	if err != nil {
		t.Fatalf("Failed to check read status: %v", err)
	}
	if !isRead {
		t.Error("Expected announcement to be marked as read")
	}

	// Mark as read again (should not error)
	err = repo.MarkAsRead(ctx, userID, announcement.ID)
	if err != nil {
		t.Fatalf("Failed to mark as read again: %v", err)
	}
}

func TestAnnouncementRepository_GetUnreadCount(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	userID := createAnnouncementTestUser(t, db, "testuser")

	// Create published announcements
	for i := 0; i < 5; i++ {
		now := time.Now()
		announcement := &Announcement{
			Title:       "Announcement " + string(rune('1'+i)),
			Content:     "Content",
			Category:    AnnouncementCategoryGeneral,
			IsPublished: true,
			PublishedAt: &now,
		}
		repo.Create(ctx, announcement)
	}

	// Check unread count
	count, err := repo.GetUnreadCount(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get unread count: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 unread, got %d", count)
	}

	// Mark some as read
	announcements, _, _ := repo.ListPublished(ctx, 10, 0)
	repo.MarkAsRead(ctx, userID, announcements[0].ID)
	repo.MarkAsRead(ctx, userID, announcements[1].ID)

	// Check unread count again
	count, _ = repo.GetUnreadCount(ctx, userID)
	if count != 3 {
		t.Errorf("Expected 3 unread, got %d", count)
	}
}

// Feature: user-portal, Property 10: Announcement Read Status Tracking
// Validates: Requirements 9.4
func TestProperty_AnnouncementReadStatusTracking(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	repo := NewAnnouncementRepository(db)
	ctx := context.Background()

	userID := createAnnouncementTestUser(t, db, "testuser")

	// Create announcement
	now := time.Now()
	announcement := &Announcement{
		Title:       "Test Announcement",
		Content:     "Content",
		Category:    AnnouncementCategoryGeneral,
		IsPublished: true,
		PublishedAt: &now,
	}
	repo.Create(ctx, announcement)

	// Initially not read
	isRead, _ := repo.IsRead(ctx, userID, announcement.ID)
	if isRead {
		t.Error("Expected announcement to be unread initially")
	}

	// Mark as read
	repo.MarkAsRead(ctx, userID, announcement.ID)

	// Should be read now
	isRead, _ = repo.IsRead(ctx, userID, announcement.ID)
	if !isRead {
		t.Error("Expected announcement to be read after marking")
	}

	// Subsequent queries should still return read status
	for i := 0; i < 10; i++ {
		isRead, _ = repo.IsRead(ctx, userID, announcement.ID)
		if !isRead {
			t.Errorf("Read status not persisted on query %d", i)
		}
	}
}

func TestAnnouncementReadIDsAreScopedToUserAndPage(t *testing.T) {
	db := setupAnnouncementTestDB(t)
	r := NewAnnouncementRepository(db)
	ctx := context.Background()
	u1 := createAnnouncementTestUser(t, db, "batch-one")
	u2 := createAnnouncementTestUser(t, db, "batch-two")
	for i := int64(1); i <= 3; i++ {
		if err := r.Create(ctx, &Announcement{ID: i, Title: "batch", Content: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.MarkAsRead(ctx, u1, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkAsRead(ctx, u1, 3); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkAsRead(ctx, u2, 2); err != nil {
		t.Fatal(err)
	}
	ids, err := r.GetReadIDs(ctx, u1, []int64{1, 2})
	if err != nil || len(ids) != 1 || ids[0] != 1 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	ids, err = r.GetReadIDs(ctx, u2, []int64{1, 2})
	if err != nil || len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("ids=%v err=%v", ids, err)
	}
	ids, err = r.GetReadIDs(ctx, u1, nil)
	if err != nil || len(ids) != 0 {
		t.Fatalf("empty page: %v %v", ids, err)
	}
}
