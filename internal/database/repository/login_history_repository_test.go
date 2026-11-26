// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupLoginHistoryTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate the required tables
	err = db.AutoMigrate(&User{}, &LoginHistory{})
	require.NoError(t, err)

	return db
}

// createTestUser creates a test user and returns its ID.
func createTestUser(t *testing.T, db *gorm.DB, username string) int64 {
	user := &User{
		Username:     username,
		PasswordHash: "test_hash",
		Email:        username + "@test.com",
		Role:         "user",
		Enabled:      true,
	}
	err := db.Create(user).Error
	require.NoError(t, err)
	return user.ID
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.12
func TestLoginHistoryRecording(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("login attempts are recorded with all required fields", prop.ForAll(
		func(ip string, userAgent string, success bool) bool {
			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := createTestUser(t, db, "testuser_"+ip[:min(len(ip), 8)])

			// Record login attempt
			history := &LoginHistory{
				UserID:    userID,
				IP:        ip,
				UserAgent: userAgent,
				Success:   success,
			}
			err := repo.Create(ctx, history)
			if err != nil {
				return false
			}

			// Verify the record was created with all fields
			histories, err := repo.GetByUserID(ctx, userID, 10, 0)
			if err != nil || len(histories) != 1 {
				return false
			}

			recorded := histories[0]
			return recorded.UserID == userID &&
				recorded.IP == ip &&
				recorded.UserAgent == userAgent &&
				recorded.Success == success &&
				!recorded.CreatedAt.IsZero()
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) <= 50 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) <= 255 }),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.12
func TestLoginHistoryRecording_MultipleAttempts(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("multiple login attempts are all recorded", prop.ForAll(
		func(attemptCount int) bool {
			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := createTestUser(t, db, "multiuser")

			// Record multiple login attempts
			for i := 0; i < attemptCount; i++ {
				history := &LoginHistory{
					UserID:    userID,
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					Success:   i%2 == 0, // Alternate success/failure
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
			}

			// Verify count matches
			count, err := repo.Count(ctx, userID)
			if err != nil {
				return false
			}

			return count == int64(attemptCount)
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.12
func TestLoginHistoryRecording_OrderedByTime(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("login history is returned in descending order by time", prop.ForAll(
		func(attemptCount int) bool {
			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := createTestUser(t, db, "orderuser")

			// Record multiple login attempts with small delays
			for i := 0; i < attemptCount; i++ {
				history := &LoginHistory{
					UserID:    userID,
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					Success:   true,
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
				// Small delay to ensure different timestamps
				time.Sleep(time.Millisecond)
			}

			// Get history
			histories, err := repo.GetByUserID(ctx, userID, attemptCount, 0)
			if err != nil || len(histories) != attemptCount {
				return false
			}

			// Verify descending order (most recent first)
			for i := 1; i < len(histories); i++ {
				if histories[i-1].CreatedAt.Before(histories[i].CreatedAt) {
					return false
				}
			}

			return true
		},
		gen.IntRange(2, 10),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.13
func TestLoginHistoryRecording_ClearHistory(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("clearing login history removes all records for user", prop.ForAll(
		func(attemptCount int) bool {
			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := createTestUser(t, db, "clearuser")

			// Record multiple login attempts
			for i := 0; i < attemptCount; i++ {
				history := &LoginHistory{
					UserID:    userID,
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					Success:   true,
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
			}

			// Verify records exist
			count, err := repo.Count(ctx, userID)
			if err != nil || count != int64(attemptCount) {
				return false
			}

			// Clear history
			if err := repo.DeleteByUserID(ctx, userID); err != nil {
				return false
			}

			// Verify all records are deleted
			count, err = repo.Count(ctx, userID)
			if err != nil {
				return false
			}

			return count == 0
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.12
func TestLoginHistoryRecording_UserIsolation(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("login history is isolated per user", prop.ForAll(
		func(user1Attempts, user2Attempts int) bool {
			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create two test users
			userID1 := createTestUser(t, db, "user1")
			userID2 := createTestUser(t, db, "user2")

			// Record attempts for user 1
			for i := 0; i < user1Attempts; i++ {
				history := &LoginHistory{
					UserID:    userID1,
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					Success:   true,
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
			}

			// Record attempts for user 2
			for i := 0; i < user2Attempts; i++ {
				history := &LoginHistory{
					UserID:    userID2,
					IP:        "192.168.1.2",
					UserAgent: "TestAgent2",
					Success:   false,
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
			}

			// Verify counts are isolated
			count1, err := repo.Count(ctx, userID1)
			if err != nil || count1 != int64(user1Attempts) {
				return false
			}

			count2, err := repo.Count(ctx, userID2)
			if err != nil || count2 != int64(user2Attempts) {
				return false
			}

			// Verify clearing one user doesn't affect the other
			if err := repo.DeleteByUserID(ctx, userID1); err != nil {
				return false
			}

			count1, _ = repo.Count(ctx, userID1)
			count2, _ = repo.Count(ctx, userID2)

			return count1 == 0 && count2 == int64(user2Attempts)
		},
		gen.IntRange(1, 10),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 26: Login History Recording
// Validates: Requirements 17.11
func TestLoginHistoryRecording_Pagination(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("pagination returns correct subset of records", prop.ForAll(
		func(totalRecords, limit, offset int) bool {
			if offset >= totalRecords {
				// Skip invalid offset cases
				return true
			}

			db := setupLoginHistoryTestDB(t)
			repo := NewLoginHistoryRepository(db)
			ctx := context.Background()

			// Create a test user
			userID := createTestUser(t, db, "pageuser")

			// Record multiple login attempts
			for i := 0; i < totalRecords; i++ {
				history := &LoginHistory{
					UserID:    userID,
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					Success:   true,
				}
				if err := repo.Create(ctx, history); err != nil {
					return false
				}
				time.Sleep(time.Millisecond) // Ensure different timestamps
			}

			// Get paginated results
			histories, err := repo.GetByUserID(ctx, userID, limit, offset)
			if err != nil {
				return false
			}

			// Calculate expected count
			expectedCount := min(limit, totalRecords-offset)
			if expectedCount < 0 {
				expectedCount = 0
			}

			return len(histories) == expectedCount
		},
		gen.IntRange(5, 20),
		gen.IntRange(1, 10),
		gen.IntRange(0, 15),
	))

	properties.TestingRun(t)
}

// Unit test for specific edge cases
func TestLoginHistoryRecording_EmptyUserAgent(t *testing.T) {
	db := setupLoginHistoryTestDB(t)
	repo := NewLoginHistoryRepository(db)
	ctx := context.Background()

	userID := createTestUser(t, db, "emptyagent")

	history := &LoginHistory{
		UserID:    userID,
		IP:        "192.168.1.1",
		UserAgent: "",
		Success:   true,
	}
	err := repo.Create(ctx, history)
	assert.NoError(t, err)

	histories, err := repo.GetByUserID(ctx, userID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, histories, 1)
	assert.Empty(t, histories[0].UserAgent)
}

// Unit test for specific edge cases
func TestLoginHistoryRecording_FailedLogin(t *testing.T) {
	db := setupLoginHistoryTestDB(t)
	repo := NewLoginHistoryRepository(db)
	ctx := context.Background()

	userID := createTestUser(t, db, "failedlogin")

	history := &LoginHistory{
		UserID:    userID,
		IP:        "192.168.1.1",
		UserAgent: "TestAgent",
		Success:   false,
	}
	err := repo.Create(ctx, history)
	assert.NoError(t, err)

	histories, err := repo.GetByUserID(ctx, userID, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, histories, 1)
	assert.False(t, histories[0].Success)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
