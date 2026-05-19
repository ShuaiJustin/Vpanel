package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"v/pkg/errors"
)

func setupSubscriptionTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate - User first (for foreign key), then Subscription
	if err := db.AutoMigrate(&User{}, &Subscription{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// createSubscriptionTestUser creates a test user and returns its ID.
func createSubscriptionTestUser(t *testing.T, db *gorm.DB, username string) int64 {
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

func TestSubscriptionRepository_Create(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}

	err := repo.Create(ctx, subscription)
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	if subscription.ID == 0 {
		t.Error("Expected subscription ID to be set after creation")
	}
}

func TestSubscriptionRepository_GetByID(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	// Get by ID
	found, err := repo.GetByID(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("Failed to get subscription: %v", err)
	}

	if found.Token != subscription.Token {
		t.Errorf("Expected token %s, got %s", subscription.Token, found.Token)
	}

	// Test not found
	_, err = repo.GetByID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_GetByToken(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "unique-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	found, err := repo.GetByToken(ctx, "unique-token-12345678901234567890")
	if err != nil {
		t.Fatalf("Failed to get subscription by token: %v", err)
	}

	if found.ID != subscription.ID {
		t.Errorf("Expected subscription ID %d, got %d", subscription.ID, found.ID)
	}

	// Test not found
	_, err = repo.GetByToken(ctx, "nonexistent-token")
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_GetByShortCode(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "short123",
	}
	repo.Create(ctx, subscription)

	found, err := repo.GetByShortCode(ctx, "short123")
	if err != nil {
		t.Fatalf("Failed to get subscription by short code: %v", err)
	}

	if found.ID != subscription.ID {
		t.Errorf("Expected subscription ID %d, got %d", subscription.ID, found.ID)
	}

	// Test not found
	_, err = repo.GetByShortCode(ctx, "nonexist")
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_GetByUserID(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	found, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to get subscription by user ID: %v", err)
	}

	if found.Token != subscription.Token {
		t.Errorf("Expected token %s, got %s", subscription.Token, found.Token)
	}

	// Test not found
	_, err = repo.GetByUserID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_Update(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	// Update subscription
	subscription.Token = "new-token-12345678901234567890"
	err := repo.Update(ctx, subscription)
	if err != nil {
		t.Fatalf("Failed to update subscription: %v", err)
	}

	// Verify update
	found, _ := repo.GetByID(ctx, subscription.ID)
	if found.Token != "new-token-12345678901234567890" {
		t.Errorf("Expected token new-token-12345678901234567890, got %s", found.Token)
	}
}

func TestSubscriptionRepository_Delete(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	// Delete subscription
	err := repo.Delete(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("Failed to delete subscription: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, subscription.ID)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error after deletion, got: %v", err)
	}

	// Test delete non-existent
	err = repo.Delete(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error for non-existent subscription, got: %v", err)
	}
}

func TestSubscriptionRepository_UniqueConstraint_UserID(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	// Create first subscription
	subscription1 := &Subscription{
		UserID:    userID,
		Token:     "token-1-12345678901234567890",
		ShortCode: "short001",
	}
	err := repo.Create(ctx, subscription1)
	if err != nil {
		t.Fatalf("Failed to create first subscription: %v", err)
	}

	// Try to create second subscription for same user - should fail
	subscription2 := &Subscription{
		UserID:    userID,
		Token:     "token-2-12345678901234567890",
		ShortCode: "short002",
	}
	err = repo.Create(ctx, subscription2)
	if err == nil {
		t.Error("Expected error when creating duplicate subscription for same user")
	}
}

func TestSubscriptionRepository_UniqueConstraint_Token(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID1 := createSubscriptionTestUser(t, db, "testuser1")
	userID2 := createSubscriptionTestUser(t, db, "testuser2")

	// Create first subscription
	subscription1 := &Subscription{
		UserID:    userID1,
		Token:     "same-token-12345678901234567890",
		ShortCode: "short001",
	}
	err := repo.Create(ctx, subscription1)
	if err != nil {
		t.Fatalf("Failed to create first subscription: %v", err)
	}

	// Try to create second subscription with same token - should fail
	subscription2 := &Subscription{
		UserID:    userID2,
		Token:     "same-token-12345678901234567890",
		ShortCode: "short002",
	}
	err = repo.Create(ctx, subscription2)
	if err == nil {
		t.Error("Expected error when creating subscription with duplicate token")
	}
}

func TestSubscriptionRepository_UpdateAccessStats(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:      userID,
		Token:       "test-token-12345678901234567890",
		ShortCode:   "abc12345",
		AccessCount: 0,
	}
	repo.Create(ctx, subscription)

	// Update access stats
	err := repo.UpdateAccessStats(ctx, subscription.ID, "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("Failed to update access stats: %v", err)
	}

	// Verify update
	found, _ := repo.GetByID(ctx, subscription.ID)
	if found.AccessCount != 1 {
		t.Errorf("Expected access count 1, got %d", found.AccessCount)
	}
	if found.LastIP != "192.168.1.1" {
		t.Errorf("Expected last IP 192.168.1.1, got %s", found.LastIP)
	}
	if found.LastUA != "Mozilla/5.0" {
		t.Errorf("Expected last UA Mozilla/5.0, got %s", found.LastUA)
	}
	if found.LastAccessAt == nil {
		t.Error("Expected last access time to be set")
	}

	// Update again
	err = repo.UpdateAccessStats(ctx, subscription.ID, "192.168.1.2", "Chrome/100")
	if err != nil {
		t.Fatalf("Failed to update access stats second time: %v", err)
	}

	found, _ = repo.GetByID(ctx, subscription.ID)
	if found.AccessCount != 2 {
		t.Errorf("Expected access count 2, got %d", found.AccessCount)
	}

	// Test not found
	err = repo.UpdateAccessStats(ctx, 99999, "192.168.1.1", "Mozilla/5.0")
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_ListAll(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	// Create multiple users and subscriptions
	for i := 0; i < 15; i++ {
		userID := createSubscriptionTestUser(t, db, "listuser"+string(rune('a'+i)))
		subscription := &Subscription{
			UserID:      userID,
			Token:       "token-" + string(rune('a'+i)) + "-12345678901234567890",
			ShortCode:   "short" + string(rune('a'+i)) + "12",
			AccessCount: int64(i),
		}
		repo.Create(ctx, subscription)
	}

	// Test list all without filter
	subscriptions, total, err := repo.ListAll(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list subscriptions: %v", err)
	}
	if total != 15 {
		t.Errorf("Expected total 15, got %d", total)
	}
	if len(subscriptions) != 15 {
		t.Errorf("Expected 15 subscriptions, got %d", len(subscriptions))
	}

	// Test pagination
	filter := &SubscriptionFilter{
		Limit:  10,
		Offset: 0,
	}
	subscriptions, total, err = repo.ListAll(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list subscriptions with pagination: %v", err)
	}
	if total != 15 {
		t.Errorf("Expected total 15, got %d", total)
	}
	if len(subscriptions) != 10 {
		t.Errorf("Expected 10 subscriptions on page 1, got %d", len(subscriptions))
	}

	// Test page 2
	filter.Offset = 10
	subscriptions, _, err = repo.ListAll(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list subscriptions page 2: %v", err)
	}
	if len(subscriptions) != 5 {
		t.Errorf("Expected 5 subscriptions on page 2, got %d", len(subscriptions))
	}

	// Test filter by min access count
	minCount := int64(10)
	filter = &SubscriptionFilter{
		MinAccessCount: &minCount,
	}
	subscriptions, total, err = repo.ListAll(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list subscriptions with min access count filter: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected 5 subscriptions with access count >= 10, got %d", total)
	}
}

func TestSubscriptionRepository_ResetAccessStats(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:      userID,
		Token:       "test-token-12345678901234567890",
		ShortCode:   "abc12345",
		AccessCount: 100,
		LastIP:      "192.168.1.1",
		LastUA:      "Mozilla/5.0",
	}
	now := time.Now()
	subscription.LastAccessAt = &now
	repo.Create(ctx, subscription)

	// Reset access stats
	err := repo.ResetAccessStats(ctx, subscription.ID)
	if err != nil {
		t.Fatalf("Failed to reset access stats: %v", err)
	}

	// Verify reset
	found, _ := repo.GetByID(ctx, subscription.ID)
	if found.AccessCount != 0 {
		t.Errorf("Expected access count 0, got %d", found.AccessCount)
	}
	if found.LastIP != "" {
		t.Errorf("Expected empty last IP, got %s", found.LastIP)
	}
	if found.LastUA != "" {
		t.Errorf("Expected empty last UA, got %s", found.LastUA)
	}

	// Test not found
	err = repo.ResetAccessStats(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestSubscriptionRepository_DeleteByUserID(t *testing.T) {
	db := setupSubscriptionTestDB(t)
	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	userID := createSubscriptionTestUser(t, db, "testuser")

	subscription := &Subscription{
		UserID:    userID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)

	// Delete by user ID
	err := repo.DeleteByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to delete subscription by user ID: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByUserID(ctx, userID)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error after deletion, got: %v", err)
	}

	// Test delete non-existent
	err = repo.DeleteByUserID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error for non-existent user, got: %v", err)
	}
}

func TestSubscriptionRepository_CascadeDelete(t *testing.T) {
	// Create a fresh database with proper foreign key setup
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Enable foreign key constraints for SQLite
	db.Exec("PRAGMA foreign_keys = ON")

	// Create tables with proper foreign key cascade
	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username VARCHAR(50) NOT NULL UNIQUE,
		password VARCHAR(255) NOT NULL,
		email VARCHAR(100),
		role VARCHAR(20) DEFAULT 'user',
		enabled BOOLEAN DEFAULT true,
		traffic_limit INTEGER DEFAULT 0,
		traffic_used INTEGER DEFAULT 0,
		expires_at DATETIME,
		force_password_change BOOLEAN DEFAULT false,
		email_verified BOOLEAN DEFAULT false,
		email_verified_at DATETIME,
		two_factor_enabled BOOLEAN DEFAULT false,
		last_login_at DATETIME,
		last_login_ip VARCHAR(45),
		avatar_url VARCHAR(512),
		display_name VARCHAR(64),
		notify_email BOOLEAN DEFAULT true,
		notify_telegram BOOLEAN DEFAULT false,
		notify_traffic_warning BOOLEAN DEFAULT true,
		notify_expiry_reminder BOOLEAN DEFAULT true,
		notify_announcements BOOLEAN DEFAULT true,
		last_traffic_warning_at DATETIME,
		last_expiry_reminder_at DATETIME,
		theme VARCHAR(16) DEFAULT 'auto',
		language VARCHAR(8) DEFAULT 'zh-CN',
		telegram_id VARCHAR(50),
		balance INTEGER DEFAULT 0,
		created_at DATETIME,
		updated_at DATETIME
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL UNIQUE,
		token VARCHAR(64) NOT NULL UNIQUE,
		short_code VARCHAR(16) UNIQUE,
		created_at DATETIME,
		updated_at DATETIME,
		last_access_at DATETIME,
		access_count INTEGER DEFAULT 0,
		last_ip VARCHAR(45),
		last_ua VARCHAR(256),
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)

	repo := NewSubscriptionRepository(db)
	ctx := context.Background()

	// Create user directly
	user := &User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		Email:        "test@example.com",
		Role:         "user",
		Enabled:      true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	subscription := &Subscription{
		UserID:    user.ID,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	repo.Create(ctx, subscription)
	subscriptionID := subscription.ID

	// Delete the user - subscription should be cascade deleted
	if err := db.Exec("DELETE FROM users WHERE id = ?", user.ID).Error; err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify subscription is also deleted
	_, err = repo.GetByID(ctx, subscriptionID)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected subscription to be cascade deleted, got: %v", err)
	}
}

// Property-based tests for subscription repository

// Feature: subscription-system, Property 21: User Subscription Uniqueness
// Validates: Requirements 10.4
// *For any* user, there SHALL be at most one subscription record in the database.
func TestProperty_UserSubscriptionUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Counter for unique usernames
	counter := 0

	properties.Property("each user can have at most one subscription", prop.ForAll(
		func(seed int) bool {
			db := setupSubscriptionTestDB(t)
			repo := NewSubscriptionRepository(db)
			ctx := context.Background()

			// Generate unique username using counter
			counter++
			username := "user" + string(rune('a'+counter%26)) + string(rune('0'+counter/26%10))

			// Create a test user
			userID := createSubscriptionTestUser(t, db, username)

			// Generate tokens based on seed
			token1 := "token1-" + string(rune('a'+seed%26)) + "-12345678901234567890"
			token2 := "token2-" + string(rune('a'+seed%26)) + "-12345678901234567890"

			// Create first subscription - should succeed
			subscription1 := &Subscription{
				UserID:    userID,
				Token:     token1,
				ShortCode: "short001",
			}
			err := repo.Create(ctx, subscription1)
			if err != nil {
				// If first creation fails, it's a test setup issue
				return false
			}

			// Try to create second subscription for same user - should fail
			subscription2 := &Subscription{
				UserID:    userID,
				Token:     token2,
				ShortCode: "short002",
			}
			err = repo.Create(ctx, subscription2)

			// The second creation MUST fail due to unique constraint on user_id
			return err != nil
		},
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t)
}
