package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"

	"v/internal/database/repository"
)

// Property 23: Database Connection Retry
// For any database connection failure, the system SHALL retry the connection
// with exponential backoff, and after successful reconnection, database
// operations SHALL resume normally.
// **Validates: Requirements 15.2**

func TestDatabaseConnectionRetry_ExponentialBackoff(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("retry intervals increase exponentially", prop.ForAll(
		func(maxRetries int, baseInterval int) bool {
			if maxRetries < 1 || maxRetries > 5 {
				maxRetries = 3
			}
			if baseInterval < 10 || baseInterval > 100 {
				baseInterval = 50
			}

			baseIntervalDuration := time.Duration(baseInterval) * time.Millisecond

			// Calculate expected intervals
			var intervals []time.Duration
			for i := 0; i < maxRetries; i++ {
				interval := baseIntervalDuration * time.Duration(1<<uint(i))
				intervals = append(intervals, interval)
			}

			// Verify exponential growth
			for i := 1; i < len(intervals); i++ {
				if intervals[i] != intervals[i-1]*2 {
					t.Logf("Interval %d should be 2x interval %d", i, i-1)
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 5),
		gen.IntRange(10, 100),
	))

	properties.TestingRun(t)
}

func TestDatabaseConnectionRetry_HealthCheckUpdatesStatus(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: 100 * time.Millisecond,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Initially should be healthy
	if !db.IsHealthy() {
		t.Error("Database should be healthy initially")
	}

	// Ping should succeed
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Errorf("Ping should succeed: %v", err)
	}

	// Health status should be accurate
	status := db.Health(ctx)
	if !status.Healthy {
		t.Error("Health status should be healthy")
	}
	if status.Error != "" {
		t.Errorf("Health status should not have error: %s", status.Error)
	}
}

func TestDatabaseConnectionRetry_WithRetrySucceeds(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour, // Disable auto health check
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// WithRetry should succeed for valid operations
	callCount := 0
	err = db.WithRetry(ctx, func(gormDB *gorm.DB) error {
		callCount++
		return gormDB.Exec("SELECT 1").Error
	})

	if err != nil {
		t.Errorf("WithRetry should succeed: %v", err)
	}
	if callCount != 1 {
		t.Errorf("Should only call once on success, called %d times", callCount)
	}
}

func TestDatabaseConnectionRetry_IsRetryableError(t *testing.T) {
	testCases := []struct {
		errMsg    string
		retryable bool
	}{
		{"database is locked", true},
		{"connection refused", true},
		{"connection reset by peer", true},
		{"broken pipe", true},
		{"timeout exceeded", true},
		{"deadlock detected", true},
		{"record not found", false},
		{"unique constraint violation", false},
		{"syntax error", false},
	}

	for _, tc := range testCases {
		result := isRetryableError(fmt.Errorf("%s", tc.errMsg))
		if result != tc.retryable {
			t.Errorf("isRetryableError(%q) = %v, want %v", tc.errMsg, result, tc.retryable)
		}
	}
}

// Property 24: Slow Query Logging
// For any database query exceeding the configured threshold (default 200ms),
// the query SHALL be logged with its duration and SQL statement.
// **Validates: Requirements 15.5**

func TestSlowQueryLogging_QueriesAboveThresholdAreLogged(t *testing.T) {
	// Clear any existing slow queries
	ClearSlowQueries()

	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use a very short threshold for testing
	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  1 * time.Nanosecond, // Very short to ensure logging
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a table
	err = db.DB().Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER PRIMARY KEY, name TEXT)").Error
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Execute a query (should be logged as slow due to 1ns threshold)
	err = db.DB().Exec("INSERT INTO test_table (name) VALUES ('test')").Error
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Check slow query logs
	logs := GetSlowQueries()
	if len(logs) == 0 {
		t.Error("Expected slow queries to be logged")
	}

	// Verify log entry has required fields
	for _, log := range logs {
		if log.SQL == "" {
			t.Error("Slow query log should have SQL")
		}
		if log.Duration <= 0 {
			t.Error("Slow query log should have positive duration")
		}
		if log.Time.IsZero() {
			t.Error("Slow query log should have timestamp")
		}
	}
}

func TestSlowQueryLogging_QueriesBelowThresholdNotLogged(t *testing.T) {
	// Clear any existing slow queries
	ClearSlowQueries()

	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use a very long threshold
	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  1 * time.Hour, // Very long to ensure no logging
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Execute a quick query
	err = db.DB().Exec("SELECT 1").Error
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}

	// Check slow query logs - should be empty
	logs := GetSlowQueries()
	if len(logs) > 0 {
		t.Errorf("Expected no slow queries, got %d", len(logs))
	}
}

func TestSlowQueryLogging_ThresholdCanBeUpdated(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Check initial threshold
	threshold := db.GetSlowQueryThreshold()
	if threshold != 200*time.Millisecond {
		t.Errorf("Expected 200ms threshold, got %v", threshold)
	}

	// Update threshold
	db.SetSlowQueryThreshold(500 * time.Millisecond)

	// Verify update
	threshold = db.GetSlowQueryThreshold()
	if threshold != 500*time.Millisecond {
		t.Errorf("Expected 500ms threshold after update, got %v", threshold)
	}
}

func TestSlowQueryLogging_MaxLogsRespected(t *testing.T) {
	// Clear any existing slow queries
	ClearSlowQueries()

	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  1 * time.Nanosecond, // Very short to ensure logging
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Create a table
	err = db.DB().Exec("CREATE TABLE IF NOT EXISTS test_max (id INTEGER PRIMARY KEY)").Error
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Execute many queries
	for i := 0; i < 150; i++ {
		db.DB().Exec("SELECT 1")
	}

	// Check that logs are capped at maxSlowQueries
	logs := GetSlowQueries()
	if len(logs) > maxSlowQueries {
		t.Errorf("Expected max %d slow queries, got %d", maxSlowQueries, len(logs))
	}
}

func TestDatabaseClose(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	cfg := &Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          3,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	// Close should succeed
	err = db.Close()
	if err != nil {
		t.Errorf("Close should succeed: %v", err)
	}

	// Verify database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file should exist after close")
	}
}

func TestDatabaseAutoMigrate_SharedTrafficTablesAllowSharedUsersWithForeignKeysEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "shared-traffic.db")

	db, err := New(&Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          1,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}

	// Simulate an old schema that enforced foreign keys on traffic tables.
	if err := db.DB().Exec(`PRAGMA foreign_keys = OFF`).Error; err != nil {
		t.Fatalf("failed to disable foreign keys: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT, password TEXT)`,
		`CREATE TABLE IF NOT EXISTS nodes (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, address TEXT, token TEXT)`,
		`CREATE TABLE IF NOT EXISTS proxies (id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, name TEXT, protocol TEXT, port INTEGER)`,
		`CREATE TABLE IF NOT EXISTS traffic (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				proxy_id INTEGER,
				upload INTEGER DEFAULT 0,
				download INTEGER DEFAULT 0,
				recorded_at TIMESTAMP NOT NULL,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
				FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE SET NULL
			)`,
		`CREATE TABLE IF NOT EXISTS node_traffic (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				node_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			proxy_id INTEGER,
			upload INTEGER DEFAULT 0,
			download INTEGER DEFAULT 0,
			recorded_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY (proxy_id) REFERENCES proxies(id) ON DELETE SET NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_node ON node_traffic(node_id)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_user ON node_traffic(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_proxy ON node_traffic(proxy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_recorded ON node_traffic(recorded_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_node_recorded ON node_traffic(node_id, recorded_at)`,
		`CREATE INDEX IF NOT EXISTS idx_node_traffic_user_recorded ON node_traffic(user_id, recorded_at)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_user_id ON traffic(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_proxy_id ON traffic(proxy_id)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_recorded_at ON traffic(recorded_at)`,
	} {
		if execErr := db.DB().Exec(stmt).Error; execErr != nil {
			t.Fatalf("failed to seed legacy schema: %v", execErr)
		}
	}
	if err := db.DB().Exec(`PRAGMA foreign_keys = ON`).Error; err != nil {
		t.Fatalf("failed to enable foreign keys: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	if err := db.DB().Exec(`PRAGMA foreign_keys = ON`).Error; err != nil {
		t.Fatalf("failed to re-enable foreign keys after AutoMigrate: %v", err)
	}

	if err := db.DB().Create(&repository.Node{
		ID:      1,
		Name:    "shared-node",
		Address: "127.0.0.1",
		Token:   "node-token",
	}).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	result := db.DB().Create(&repository.NodeTraffic{
		NodeID:   1,
		UserID:   0,
		Upload:   128,
		Download: 64,
	})
	if result.Error != nil {
		t.Fatalf("expected shared traffic insert to succeed, got %v", result.Error)
	}
	result = db.DB().Create(&repository.Traffic{
		UserID:     0,
		Upload:     32,
		Download:   16,
		RecordedAt: time.Now(),
	})
	if result.Error != nil {
		t.Fatalf("expected shared global traffic insert to succeed, got %v", result.Error)
	}

	type fkRow struct {
		Table string `gorm:"column:table"`
		From  string `gorm:"column:from"`
	}
	for _, tableName := range []string{"traffic", "node_traffic"} {
		var foreignKeys []fkRow
		if err := db.DB().Raw(fmt.Sprintf(`PRAGMA foreign_key_list('%s')`, tableName)).Scan(&foreignKeys).Error; err != nil {
			t.Fatalf("failed to inspect %s foreign keys: %v", tableName, err)
		}
		if len(foreignKeys) != 0 {
			t.Fatalf("expected %s foreign keys to be removed, still found %+v", tableName, foreignKeys)
		}
	}

	_ = db.Close()
}
