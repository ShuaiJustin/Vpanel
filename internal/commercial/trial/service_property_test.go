// Package trial provides trial subscription management functionality.
package trial

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	err = db.AutoMigrate(&repository.Trial{}, &repository.User{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// createTestUser creates a test user in the database.
func createTestUser(db *gorm.DB, userID int64) error {
	user := &repository.User{
		ID:            userID,
		Username:      "testuser",
		Email:         "test@example.com",
		PasswordHash:  "hashedpassword",
		EmailVerified: true,
	}
	return db.Create(user).Error
}

// Feature: commercial-system, Property 15: Trial Uniqueness
// Validates: Requirements 15.3
// *For any* user, they SHALL have at most one trial record, and once used, cannot activate another trial.
func TestProperty_TrialUniqueness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("each user can only have one trial", prop.ForAll(
		func(userID int64) bool {
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			trialRepo := repository.NewTrialRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a test user
			if err := createTestUser(db, userID); err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			config := &Config{
				Enabled:            true,
				Duration:           7,
				TrafficLimit:       1073741824,
				RequireEmailVerify: false,
				AutoActivate:       false,
			}

			svc := NewService(trialRepo, userRepo, log, config)
			ctx := context.Background()

			// First activation should succeed
			trial1, err := svc.ActivateTrial(ctx, userID)
			if err != nil {
				t.Logf("First activation failed: %v", err)
				return false
			}
			if trial1 == nil {
				t.Log("First trial is nil")
				return false
			}

			// Second activation should fail with ErrTrialAlreadyUsed
			trial2, err := svc.ActivateTrial(ctx, userID)
			if err != ErrTrialAlreadyUsed {
				t.Logf("Expected ErrTrialAlreadyUsed, got: %v", err)
				return false
			}
			if trial2 != nil {
				t.Log("Second trial should be nil")
				return false
			}

			// Verify only one trial exists in database
			var count int64
			db.Model(&repository.Trial{}).Where("user_id = ?", userID).Count(&count)
			if count != 1 {
				t.Logf("Expected 1 trial, got: %d", count)
				return false
			}

			return true
		},
		gen.Int64Range(1, 1000),
	))

	properties.Property("HasUsedTrial returns true after activation", prop.ForAll(
		func(userID int64) bool {
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			trialRepo := repository.NewTrialRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a test user
			if err := createTestUser(db, userID); err != nil {
				return false
			}

			config := DefaultConfig()
			svc := NewService(trialRepo, userRepo, log, config)
			ctx := context.Background()

			// Before activation, HasUsedTrial should return false
			if svc.HasUsedTrial(ctx, userID) {
				return false
			}

			// Activate trial
			_, err := svc.ActivateTrial(ctx, userID)
			if err != nil {
				return false
			}

			// After activation, HasUsedTrial should return true
			return svc.HasUsedTrial(ctx, userID)
		},
		gen.Int64Range(1, 1000),
	))

	properties.Property("expired trial still counts as used", prop.ForAll(
		func(userID int64) bool {
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			trialRepo := repository.NewTrialRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a test user
			if err := createTestUser(db, userID); err != nil {
				return false
			}

			config := DefaultConfig()
			svc := NewService(trialRepo, userRepo, log, config)
			ctx := context.Background()

			// Create an expired trial directly in database
			expiredTrial := &repository.Trial{
				UserID:      userID,
				Status:      "expired",
				StartAt:     time.Now().AddDate(0, 0, -14),
				ExpireAt:    time.Now().AddDate(0, 0, -7),
				TrafficUsed: 0,
			}
			if err := db.Create(expiredTrial).Error; err != nil {
				return false
			}

			// HasUsedTrial should still return true
			if !svc.HasUsedTrial(ctx, userID) {
				return false
			}

			// Trying to activate should fail
			_, err := svc.ActivateTrial(ctx, userID)
			return err == ErrTrialAlreadyUsed
		},
		gen.Int64Range(1, 1000),
	))

	properties.Property("converted trial still counts as used", prop.ForAll(
		func(userID int64) bool {
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			trialRepo := repository.NewTrialRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a test user
			if err := createTestUser(db, userID); err != nil {
				return false
			}

			config := DefaultConfig()
			svc := NewService(trialRepo, userRepo, log, config)
			ctx := context.Background()

			// Create a converted trial directly in database
			now := time.Now()
			convertedTrial := &repository.Trial{
				UserID:      userID,
				Status:      "converted",
				StartAt:     now.AddDate(0, 0, -7),
				ExpireAt:    now,
				TrafficUsed: 500000000,
				ConvertedAt: &now,
			}
			if err := db.Create(convertedTrial).Error; err != nil {
				return false
			}

			// HasUsedTrial should still return true
			if !svc.HasUsedTrial(ctx, userID) {
				return false
			}

			// Trying to activate should fail
			_, err := svc.ActivateTrial(ctx, userID)
			return err == ErrTrialAlreadyUsed
		},
		gen.Int64Range(1, 1000),
	))

	properties.TestingRun(t)
}

// Feature: commercial-system, Property 15: Trial Uniqueness (Database Constraint)
// Validates: Requirements 15.3
// The database should enforce uniqueness constraint on user_id.
func TestProperty_TrialDatabaseUniqueness(t *testing.T) {
	db := setupTestDB(t)

	// Create a test user
	if err := createTestUser(db, 1); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create first trial
	trial1 := &repository.Trial{
		UserID:      1,
		Status:      "active",
		StartAt:     time.Now(),
		ExpireAt:    time.Now().AddDate(0, 0, 7),
		TrafficUsed: 0,
	}
	if err := db.Create(trial1).Error; err != nil {
		t.Fatalf("Failed to create first trial: %v", err)
	}

	// Attempt to create second trial for same user should fail due to unique constraint
	trial2 := &repository.Trial{
		UserID:      1,
		Status:      "active",
		StartAt:     time.Now(),
		ExpireAt:    time.Now().AddDate(0, 0, 7),
		TrafficUsed: 0,
	}
	err := db.Create(trial2).Error
	if err == nil {
		t.Error("Expected unique constraint violation, but got no error")
	}
}
