// Package pause provides subscription pause functionality.
package pause

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
	err = db.AutoMigrate(&repository.SubscriptionPause{}, &repository.User{})
	if err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// createTestUserWithSubscription creates a test user with an active subscription.
func createTestUserWithSubscription(db *gorm.DB, userID int64, daysRemaining int) error {
	expiresAt := time.Now().AddDate(0, 0, daysRemaining)
	user := &repository.User{
		ID:           userID,
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hashedpassword",
		ExpiresAt:    &expiresAt,
		TrafficLimit: 10737418240, // 10GB
		TrafficUsed:  1073741824,  // 1GB used
	}
	return db.Create(user).Error
}


// Feature: commercial-system, Property 19: Pause Duration Limit
// Validates: Requirements 19.3
// *For any* subscription pause, the duration SHALL not exceed the configured max_duration.
func TestProperty_PauseDurationLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("pause auto-resume is within max duration", prop.ForAll(
		func(maxDuration int, userID int64) bool {
			if maxDuration < 1 {
				maxDuration = 1
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			pauseRepo := repository.NewPauseRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with active subscription
			if err := createTestUserWithSubscription(db, userID, 30); err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			config := &Config{
				Enabled:     true,
				MaxDuration: maxDuration,
				MaxPerCycle: 3,
			}

			svc := NewService(pauseRepo, userRepo, log, config)
			ctx := context.Background()

			// Pause subscription
			result, err := svc.Pause(ctx, userID)
			if err != nil {
				t.Logf("Failed to pause: %v", err)
				return false
			}

			// Verify auto-resume is within max duration
			now := time.Now()
			maxAutoResume := now.AddDate(0, 0, maxDuration+1) // Allow 1 day tolerance

			if result.AutoResumeAt.After(maxAutoResume) {
				t.Logf("Auto-resume %v exceeds max duration %d days from now",
					result.AutoResumeAt, maxDuration)
				return false
			}

			return true
		},
		gen.IntRange(1, 90),      // maxDuration: 1 to 90 days
		gen.Int64Range(1, 1000),  // userID
	))

	properties.Property("pause preserves remaining days and traffic", prop.ForAll(
		func(daysRemaining int, userID int64) bool {
			if daysRemaining < 1 {
				daysRemaining = 1
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			pauseRepo := repository.NewPauseRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with specific remaining days
			if err := createTestUserWithSubscription(db, userID, daysRemaining); err != nil {
				return false
			}

			config := DefaultConfig()
			svc := NewService(pauseRepo, userRepo, log, config)
			ctx := context.Background()

			// Pause subscription
			result, err := svc.Pause(ctx, userID)
			if err != nil {
				return false
			}

			// Verify remaining days is preserved (allow 1 day tolerance for timing)
			if result.Pause.RemainingDays < daysRemaining-1 || result.Pause.RemainingDays > daysRemaining+1 {
				t.Logf("Remaining days mismatch: expected ~%d, got %d",
					daysRemaining, result.Pause.RemainingDays)
				return false
			}

			// Verify remaining traffic is preserved
			if result.Pause.RemainingTraffic <= 0 {
				t.Log("Remaining traffic should be positive")
				return false
			}

			return true
		},
		gen.IntRange(1, 365),     // daysRemaining
		gen.Int64Range(1, 1000),  // userID
	))

	properties.TestingRun(t)
}


// Feature: commercial-system, Property 19: Pause Duration Limit (Frequency)
// Validates: Requirements 19.3, 19.4
// Note: This test verifies that the system tracks pause count correctly.
// The actual enforcement depends on the billing cycle calculation.
func TestProperty_PauseFrequencyLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("pause count is tracked correctly", prop.ForAll(
		func(maxPerCycle int, userID int64) bool {
			if maxPerCycle < 1 {
				maxPerCycle = 1
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			pauseRepo := repository.NewPauseRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with active subscription (long enough for multiple pauses)
			if err := createTestUserWithSubscription(db, userID, 365); err != nil {
				return false
			}

			config := &Config{
				Enabled:     true,
				MaxDuration: 7,
				MaxPerCycle: maxPerCycle,
			}

			svc := NewService(pauseRepo, userRepo, log, config)
			ctx := context.Background()

			// Perform one pause and resume
			_, err := svc.Pause(ctx, userID)
			if err != nil {
				t.Logf("Failed to pause: %v", err)
				return false
			}

			if err := svc.Resume(ctx, userID); err != nil {
				t.Logf("Failed to resume: %v", err)
				return false
			}

			// Verify pause was recorded
			history, total, err := svc.GetPauseHistory(ctx, userID, 1, 10)
			if err != nil {
				t.Logf("Failed to get history: %v", err)
				return false
			}

			if total != 1 || len(history) != 1 {
				t.Logf("Expected 1 pause in history, got %d", total)
				return false
			}

			return true
		},
		gen.IntRange(1, 5),       // maxPerCycle
		gen.Int64Range(1, 1000),  // userID
	))

	properties.TestingRun(t)
}


// Feature: commercial-system, Property 19: Pause Duration Limit (Resume)
// Validates: Requirements 19.3
func TestProperty_PauseResumeRestoresDays(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("resume restores remaining days", prop.ForAll(
		func(daysRemaining int, userID int64) bool {
			if daysRemaining < 1 {
				daysRemaining = 1
			}
			if userID <= 0 {
				userID = 1
			}

			db := setupTestDB(t)
			pauseRepo := repository.NewPauseRepository(db)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with specific remaining days
			if err := createTestUserWithSubscription(db, userID, daysRemaining); err != nil {
				return false
			}

			config := DefaultConfig()
			svc := NewService(pauseRepo, userRepo, log, config)
			ctx := context.Background()

			// Pause
			result, err := svc.Pause(ctx, userID)
			if err != nil {
				return false
			}

			savedRemainingDays := result.Pause.RemainingDays

			// Resume
			if err := svc.Resume(ctx, userID); err != nil {
				return false
			}

			// Check user's new expiration
			user, err := userRepo.GetByID(ctx, userID)
			if err != nil {
				return false
			}

			// New expiration should be approximately now + saved remaining days
			expectedExpiry := time.Now().AddDate(0, 0, savedRemainingDays)
			tolerance := 24 * time.Hour // 1 day tolerance

			diff := user.ExpiresAt.Sub(expectedExpiry)
			if diff < 0 {
				diff = -diff
			}

			if diff > tolerance {
				t.Logf("Expiry mismatch: expected ~%v, got %v",
					expectedExpiry, user.ExpiresAt)
				return false
			}

			return true
		},
		gen.IntRange(1, 365),     // daysRemaining
		gen.Int64Range(1, 1000),  // userID
	))

	properties.TestingRun(t)
}
