// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupLogTestDB creates an in-memory SQLite database for testing.
func setupLogTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate the Log table
	err = db.AutoMigrate(&Log{})
	require.NoError(t, err)

	return db
}

// generateLogEntry creates a log entry with the given parameters.
func generateLogEntry(level, message, source, ip, userAgent, requestID string, userID *int64) *Log {
	return &Log{
		Level:     level,
		Message:   message,
		Source:    source,
		UserID:    userID,
		IP:        ip,
		UserAgent: userAgent,
		RequestID: requestID,
		Fields:    "{}",
	}
}

// validLogLevels returns the valid log levels.
func validLogLevels() []string {
	return []string{"debug", "info", "warn", "error", "fatal"}
}

// genLogLevel generates a random valid log level.
func genLogLevel() gopter.Gen {
	return gen.OneConstOf("debug", "info", "warn", "error", "fatal")
}

// genNonEmptyString generates a non-empty alphanumeric string with specified length range.
func genNonEmptyString(minLen, maxLen int) gopter.Gen {
	return gen.SliceOfN(maxLen, gen.AlphaChar()).Map(func(chars []rune) string {
		if len(chars) < minLen {
			// Pad with 'a' if too short
			for len(chars) < minLen {
				chars = append(chars, 'a')
			}
		}
		return string(chars)
	}).SuchThat(func(s string) bool {
		return len(s) >= minLen && len(s) <= maxLen
	})
}

// Feature: logging-system, Property 2: Batch Insertion Integrity
// For any batch of N log entries submitted to CreateBatch, exactly N entries SHALL be persisted
// to the database, and each entry SHALL be retrievable with its original data intact.
// Validates: Requirements 1.4
func TestLogRepository_BatchInsertionIntegrity(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("batch of N log entries results in exactly N persisted entries with intact data", prop.ForAll(
		func(batchSize int, levelIdx int, source string) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			levels := validLogLevels()
			level := levels[levelIdx%len(levels)]

			// Create batch of log entries
			logs := make([]*Log, batchSize)
			for i := 0; i < batchSize; i++ {
				logs[i] = generateLogEntry(
					level,
					"Test message "+string(rune('A'+i%26)),
					source,
					"192.168.1.1",
					"TestAgent",
					"req-"+string(rune('0'+i%10)),
					nil,
				)
			}

			// Perform batch insert
			err := repo.CreateBatch(ctx, logs)
			if err != nil {
				return false
			}

			// Count total entries
			count, err := repo.Count(ctx, nil)
			if err != nil {
				return false
			}

			// Verify exactly N entries were persisted
			if count != int64(batchSize) {
				return false
			}

			// Verify each entry is retrievable with original data
			allLogs, err := repo.List(ctx, nil, batchSize+10, 0)
			if err != nil {
				return false
			}

			if len(allLogs) != batchSize {
				return false
			}

			// Verify data integrity - all entries have correct level and source
			for _, log := range allLogs {
				if log.Level != level || log.Source != source {
					return false
				}
				if log.ID == 0 {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 50),
		gen.IntRange(0, 4),
		genNonEmptyString(3, 20),
	))

	properties.TestingRun(t)
}

// Feature: logging-system, Property 3: Unique Identifier Generation
// For any set of log entries created through the Log_Service, each entry SHALL have a unique
// identifier, and no two entries SHALL share the same ID.
// Validates: Requirements 1.5
func TestLogRepository_UniqueIdentifierGeneration(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all created log entries have unique IDs", prop.ForAll(
		func(entryCount int) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			// Create multiple log entries
			ids := make(map[int64]bool)
			for i := 0; i < entryCount; i++ {
				log := generateLogEntry(
					"info",
					"Message "+string(rune('A'+i%26)),
					"test-source",
					"192.168.1.1",
					"TestAgent",
					"req-"+string(rune('0'+i%10)),
					nil,
				)

				err := repo.Create(ctx, log)
				if err != nil {
					return false
				}

				// Check ID is non-zero
				if log.ID == 0 {
					return false
				}

				// Check ID is unique
				if ids[log.ID] {
					return false // Duplicate ID found
				}
				ids[log.ID] = true
			}

			// Verify all IDs are unique
			return len(ids) == entryCount
		},
		gen.IntRange(1, 100),
	))

	properties.TestingRun(t)
}

// Feature: logging-system, Property 4: Pagination Correctness
// For any log query with page P and page_size S, the result SHALL contain at most S entries,
// and the entries SHALL be from position (P-1)*S to P*S-1 in the ordered result set.
// Validates: Requirements 2.1
func TestLogRepository_PaginationCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("pagination returns correct subset of records", prop.ForAll(
		func(totalRecords, pageSize, page int) bool {
			if pageSize <= 0 || page < 1 {
				return true // Skip invalid inputs
			}

			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			// Create log entries with distinct timestamps
			for i := 0; i < totalRecords; i++ {
				log := &Log{
					Level:     "info",
					Message:   "Message " + string(rune('A'+i%26)),
					Source:    "test-source",
					IP:        "192.168.1.1",
					UserAgent: "TestAgent",
					RequestID: "req-" + string(rune('0'+i%10)),
					Fields:    "{}",
					CreatedAt: time.Now().Add(time.Duration(i) * time.Millisecond),
				}
				if err := repo.Create(ctx, log); err != nil {
					return false
				}
			}

			// Calculate offset from page number
			offset := (page - 1) * pageSize

			// Get paginated results
			logs, err := repo.List(ctx, nil, pageSize, offset)
			if err != nil {
				return false
			}

			// Calculate expected count
			expectedCount := pageSize
			if offset >= totalRecords {
				expectedCount = 0
			} else if offset+pageSize > totalRecords {
				expectedCount = totalRecords - offset
			}

			// Verify result count
			if len(logs) != expectedCount {
				return false
			}

			// Verify results are at most pageSize
			if len(logs) > pageSize {
				return false
			}

			return true
		},
		gen.IntRange(1, 50),
		gen.IntRange(1, 20),
		gen.IntRange(1, 10),
	))

	properties.TestingRun(t)
}

// Feature: logging-system, Property 5: Filter Correctness
// For any log query with filters (level, date range, source, keyword), all returned entries
// SHALL match ALL specified filter criteria, and no entry matching all criteria SHALL be excluded.
// Validates: Requirements 2.2, 2.3, 2.4, 2.5
func TestLogRepository_FilterCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("filter by level returns only matching entries", prop.ForAll(
		func(targetLevel string) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			// Create logs with different levels
			levels := validLogLevels()
			for _, level := range levels {
				for i := 0; i < 5; i++ {
					log := generateLogEntry(level, "Message", "source", "192.168.1.1", "Agent", "req", nil)
					if err := repo.Create(ctx, log); err != nil {
						return false
					}
				}
			}

			// Filter by target level
			filter := &LogFilter{Level: targetLevel}
			logs, err := repo.List(ctx, filter, 100, 0)
			if err != nil {
				return false
			}

			// Verify all returned logs match the filter
			for _, log := range logs {
				if log.Level != targetLevel {
					return false
				}
			}

			// Verify count matches
			count, err := repo.Count(ctx, filter)
			if err != nil {
				return false
			}

			return int64(len(logs)) == count && count == 5
		},
		genLogLevel(),
	))

	properties.Property("filter by source returns only matching entries", prop.ForAll(
		func(sourceIdx int) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			// Create logs with different sources
			sources := []string{"api", "auth", "proxy", "system", "handler"}
			targetSource := sources[sourceIdx%len(sources)]

			for _, source := range sources {
				for i := 0; i < 3; i++ {
					log := generateLogEntry("info", "Message", source, "192.168.1.1", "Agent", "req", nil)
					if err := repo.Create(ctx, log); err != nil {
						return false
					}
				}
			}

			// Filter by target source
			filter := &LogFilter{Source: targetSource}
			logs, err := repo.List(ctx, filter, 100, 0)
			if err != nil {
				return false
			}

			// Verify all returned logs match the filter
			for _, log := range logs {
				if log.Source != targetSource {
					return false
				}
			}

			// Should return exactly 3 logs for the target source
			return len(logs) == 3
		},
		gen.IntRange(0, 4),
	))

	properties.Property("filter by date range returns only entries within range", prop.ForAll(
		func(daysBack int) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			now := time.Now()

			// Create logs at different times
			for i := 0; i < 10; i++ {
				log := &Log{
					Level:     "info",
					Message:   "Message",
					Source:    "source",
					IP:        "192.168.1.1",
					UserAgent: "Agent",
					RequestID: "req",
					Fields:    "{}",
					CreatedAt: now.AddDate(0, 0, -i), // i days ago
				}
				if err := db.Create(log).Error; err != nil {
					return false
				}
			}

			// Filter by date range
			startTime := now.AddDate(0, 0, -daysBack)
			endTime := now.Add(time.Hour) // Include today
			filter := &LogFilter{
				StartTime: &startTime,
				EndTime:   &endTime,
			}

			logs, err := repo.List(ctx, filter, 100, 0)
			if err != nil {
				return false
			}

			// Verify all returned logs are within the date range
			for _, log := range logs {
				if log.CreatedAt.Before(startTime) || log.CreatedAt.After(endTime) {
					return false
				}
			}

			// Expected count: logs from day 0 to daysBack (inclusive)
			expectedCount := daysBack + 1
			if expectedCount > 10 {
				expectedCount = 10
			}

			return len(logs) == expectedCount
		},
		gen.IntRange(0, 9),
	))

	properties.Property("filter by keyword returns only entries containing the keyword", prop.ForAll(
		func(keywordIdx int) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			keywords := []string{"error", "warning", "success", "failed", "completed"}
			keyword := keywords[keywordIdx%len(keywords)]

			// Create logs with and without the keyword
			messagesWithKeyword := []string{
				"This contains " + keyword + " in the middle",
				keyword + " at the start",
				"At the end " + keyword,
			}
			messagesWithoutKeyword := []string{
				"No match here",
				"Another message",
				"Something else",
			}

			for _, msg := range messagesWithKeyword {
				log := generateLogEntry("info", msg, "source", "192.168.1.1", "Agent", "req", nil)
				if err := repo.Create(ctx, log); err != nil {
					return false
				}
			}
			for _, msg := range messagesWithoutKeyword {
				log := generateLogEntry("info", msg, "source", "192.168.1.1", "Agent", "req", nil)
				if err := repo.Create(ctx, log); err != nil {
					return false
				}
			}

			// Filter by keyword
			filter := &LogFilter{Keyword: keyword}
			logs, err := repo.List(ctx, filter, 100, 0)
			if err != nil {
				return false
			}

			// Verify all returned logs contain the keyword
			for _, log := range logs {
				if !containsSubstring(log.Message, keyword) {
					return false
				}
			}

			// Should return exactly 3 logs (the ones with keyword)
			return len(logs) == 3
		},
		gen.IntRange(0, 4),
	))

	properties.TestingRun(t)
}

// Feature: logging-system, Property 5: Filter Correctness (Combined Filters)
// Validates: Requirements 2.2, 2.3, 2.4, 2.5
func TestLogRepository_CombinedFilterCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("combined filters return only entries matching ALL criteria", prop.ForAll(
		func(levelIdx, sourceIdx int) bool {
			db := setupLogTestDB(t)
			repo := NewLogRepository(db)
			ctx := context.Background()

			allLevels := validLogLevels()
			allSources := []string{"api", "auth", "proxy", "system", "handler"}

			targetLevel := allLevels[levelIdx%len(allLevels)]
			targetSource := allSources[sourceIdx%len(allSources)]

			// Create logs with various combinations
			levels := []string{targetLevel, "debug", "error"}
			sources := []string{targetSource, "other1", "other2"}

			// Make levels and sources unique
			uniqueLevels := make(map[string]bool)
			uniqueSources := make(map[string]bool)
			for _, l := range levels {
				uniqueLevels[l] = true
			}
			for _, s := range sources {
				uniqueSources[s] = true
			}

			for level := range uniqueLevels {
				for source := range uniqueSources {
					log := generateLogEntry(level, "Message", source, "192.168.1.1", "Agent", "req", nil)
					if err := repo.Create(ctx, log); err != nil {
						return false
					}
				}
			}

			// Filter by both level and source
			filter := &LogFilter{
				Level:  targetLevel,
				Source: targetSource,
			}
			logs, err := repo.List(ctx, filter, 100, 0)
			if err != nil {
				return false
			}

			// Verify all returned logs match BOTH criteria
			for _, log := range logs {
				if log.Level != targetLevel || log.Source != targetSource {
					return false
				}
			}

			// Should return exactly 1 log (matching both criteria)
			return len(logs) == 1
		},
		gen.IntRange(0, 4),
		gen.IntRange(0, 4),
	))

	properties.TestingRun(t)
}

// containsSubstring checks if s contains substr (case-sensitive).
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Unit tests for edge cases

func TestLogRepository_CreateEmptyBatch(t *testing.T) {
	db := setupLogTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	// Empty batch should not error
	err := repo.CreateBatch(ctx, []*Log{})
	assert.NoError(t, err)

	count, err := repo.Count(ctx, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestLogRepository_GetByIDNotFound(t *testing.T) {
	db := setupLogTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, 99999)
	assert.Error(t, err)
}

func TestLogRepository_MinLevelFilter(t *testing.T) {
	db := setupLogTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	// Create logs with all levels
	for _, level := range validLogLevels() {
		log := generateLogEntry(level, "Message", "source", "192.168.1.1", "Agent", "req", nil)
		err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	// Filter by min level "warn" should return warn, error, fatal
	filter := &LogFilter{MinLevel: "warn"}
	logs, err := repo.List(ctx, filter, 100, 0)
	require.NoError(t, err)

	assert.Len(t, logs, 3)
	for _, log := range logs {
		assert.Contains(t, []string{"warn", "error", "fatal"}, log.Level)
	}
}

func TestLogRepository_DeleteOlderThan(t *testing.T) {
	db := setupLogTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	now := time.Now()

	// Create old and new logs
	oldLog := &Log{
		Level:     "info",
		Message:   "Old message",
		Source:    "source",
		CreatedAt: now.AddDate(0, 0, -10),
	}
	newLog := &Log{
		Level:     "info",
		Message:   "New message",
		Source:    "source",
		CreatedAt: now,
	}

	db.Create(oldLog)
	db.Create(newLog)

	// Delete logs older than 5 days
	cutoff := now.AddDate(0, 0, -5)
	deleted, err := repo.DeleteOlderThan(ctx, cutoff)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify only new log remains
	count, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestLogRepository_DeleteByFilter(t *testing.T) {
	db := setupLogTestDB(t)
	repo := NewLogRepository(db)
	ctx := context.Background()

	// Create logs with different levels
	for _, level := range []string{"info", "info", "error", "error", "error"} {
		log := generateLogEntry(level, "Message", "source", "192.168.1.1", "Agent", "req", nil)
		err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	// Delete only error logs
	filter := &LogFilter{Level: "error"}
	deleted, err := repo.DeleteByFilter(ctx, filter)
	require.NoError(t, err)
	assert.Equal(t, int64(3), deleted)

	// Verify only info logs remain
	count, err := repo.Count(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}
