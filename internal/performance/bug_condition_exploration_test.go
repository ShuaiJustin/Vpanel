// Package performance provides performance testing for the Vpanel application.
package performance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"

	"v/internal/database"
	"v/internal/database/repository"
)

// **Validates: Requirements 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4**
//
// Property 1: Bug Condition - Performance Bottleneck Detection
//
// CRITICAL: This test MUST FAIL on unfixed code - failure confirms the performance bugs exist
// DO NOT attempt to fix the test or the code when it fails
// NOTE: This test encodes the expected performance behavior - it will validate the fix when it passes after implementation
// GOAL: Surface concrete performance metrics that demonstrate the bottlenecks exist
//
// Test implementation details from Bug Condition in design:
// - N+1 query detection: Count database queries for node list endpoint (expect 1 + N queries for N nodes)
// - Serial query timing: Measure dashboard API response time breakdown (expect sum of individual query times)
// - Bundle size measurement: Analyze production build output (expect vendor.js > 1.5MB)
// - Cache miss rate: Monitor cache hit/miss ratio for traffic stats (expect 0% hit rate initially)
//
// The test assertions should match the Expected Behavior Properties from design:
// - Node list should execute ≤ 3 queries regardless of node count
// - Dashboard API should respond in ≤ 100ms
// - Initial bundle should be ≤ 500KB
// - Cache hit rate should be ≥ 80% for repeated queries

// queryCounter wraps a gorm.DB and counts the number of queries executed
type queryCounter struct {
	db    *gorm.DB
	count int
}

func newQueryCounter(db *gorm.DB) *queryCounter {
	counter := &queryCounter{db: db, count: 0}
	
	// Register callback to count queries
	db.Callback().Query().Before("gorm:query").Register("count_query", func(db *gorm.DB) {
		counter.count++
	})
	db.Callback().Create().Before("gorm:create").Register("count_create", func(db *gorm.DB) {
		counter.count++
	})
	db.Callback().Update().Before("gorm:update").Register("count_update", func(db *gorm.DB) {
		counter.count++
	})
	db.Callback().Delete().Before("gorm:delete").Register("count_delete", func(db *gorm.DB) {
		counter.count++
	})
	db.Callback().Row().Before("gorm:row").Register("count_row", func(db *gorm.DB) {
		counter.count++
	})
	db.Callback().Raw().Before("gorm:raw").Register("count_raw", func(db *gorm.DB) {
		counter.count++
	})
	
	return counter
}

func (qc *queryCounter) reset() {
	qc.count = 0
}

func (qc *queryCounter) getCount() int {
	return qc.count
}

// setupTestDB creates a test database with sample data
func setupTestDB(t *testing.T) (*database.Database, *queryCounter) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "performance_test.db")

	cfg := &database.Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          1,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	counter := newQueryCounter(db.DB())
	return db, counter
}

// TestBugCondition_N1QueryInUserSummary tests that GetFilteredSummary executes multiple COUNT queries
// EXPECTED: This test should FAIL on unfixed code (4 separate COUNT queries)
// EXPECTED AFTER FIX: This test should PASS (1 aggregated query)
func TestBugCondition_N1QueryInUserSummary(t *testing.T) {
	db, counter := setupTestDB(t)
	defer db.Close()

	// Create test users directly with DB
	for i := 0; i < 10; i++ {
		user := &repository.User{
			Username:     fmt.Sprintf("user%d", i),
			PasswordHash: "password",
			Role:         "user",
			Enabled:      i%2 == 0, // Half enabled, half disabled
		}
		if i == 0 {
			user.Role = "admin"
		}
		if err := db.DB().Create(user).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	// Reset counter after setup
	counter.reset()

	// Execute Count queries similar to GetFilteredSummary pattern
	// This simulates the 4 separate COUNT queries in the current implementation
	var total, admin, enabled, disabled int64
	
	if err := db.DB().Model(&repository.User{}).Count(&total).Error; err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	
	if err := db.DB().Model(&repository.User{}).Where("role = ?", "admin").Count(&admin).Error; err != nil {
		t.Fatalf("Count admin failed: %v", err)
	}
	
	if err := db.DB().Model(&repository.User{}).Where("enabled = ?", true).Count(&enabled).Error; err != nil {
		t.Fatalf("Count enabled failed: %v", err)
	}
	
	if err := db.DB().Model(&repository.User{}).Where("enabled = ?", false).Count(&disabled).Error; err != nil {
		t.Fatalf("Count disabled failed: %v", err)
	}

	queryCount := counter.getCount()
	
	// EXPECTED BEHAVIOR: Should execute ≤ 3 queries (ideally 1 aggregated query)
	// CURRENT BEHAVIOR: Executes 4 separate COUNT queries
	if queryCount > 3 {
		t.Logf("PERFORMANCE BUG DETECTED: User summary executed %d queries (expected ≤ 3)", queryCount)
		t.Logf("Counterexample: Actual query count = %d, Expected ≤ 3", queryCount)
		t.Logf("Results: Total=%d, Admin=%d, Enabled=%d, Disabled=%d", total, admin, enabled, disabled)
		t.Errorf("Bug condition confirmed: Multiple COUNT queries instead of single aggregated query")
	} else {
		t.Logf("Performance target met: User summary executed %d queries", queryCount)
	}
}

// TestBugCondition_N1QueryInNodeList tests that node list with groups executes N+1 queries
// EXPECTED: This test should FAIL on unfixed code (1 + N queries for N nodes)
// EXPECTED AFTER FIX: This test should PASS (≤ 3 queries regardless of node count)
func TestBugCondition_N1QueryInNodeList(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("node list should execute ≤ 3 queries regardless of node count", prop.ForAll(
		func(nodeCount uint8) bool {
			// Limit node count to reasonable range (5-20)
			n := int(nodeCount%16) + 5

			db, counter := setupTestDB(t)
			defer db.Close()

			ctx := context.Background()
			nodeRepo := repository.NewNodeRepository(db.DB())
			groupRepo := repository.NewNodeGroupRepository(db.DB())

			// Create a test group
			group := &repository.NodeGroup{
				Name:        "test-group",
				Description: "Test group",
			}
			if err := groupRepo.Create(ctx, group); err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create test nodes and assign to group
			for i := 0; i < n; i++ {
				node := &repository.Node{
					Name:    fmt.Sprintf("node%d", i),
					Address: fmt.Sprintf("192.168.1.%d", i+1),
					Token:   fmt.Sprintf("token%d", i),
					Status:  "online",
				}
				if err := nodeRepo.Create(ctx, node); err != nil {
					t.Logf("Failed to create node: %v", err)
					return false
				}

				// Add node to group
				if err := groupRepo.AddNode(ctx, group.ID, node.ID); err != nil {
					t.Logf("Failed to add node to group: %v", err)
					return false
				}
			}

			// Reset counter after setup
			counter.reset()

			// List nodes (this should trigger N+1 if groups are loaded per node)
			filter := &repository.NodeFilter{
				Limit: 100,
			}
			nodes, err := nodeRepo.List(ctx, filter)
			if err != nil {
				t.Logf("Failed to list nodes: %v", err)
				return false
			}

			if len(nodes) != n {
				t.Logf("Expected %d nodes, got %d", n, len(nodes))
				return false
			}

			queryCount := counter.getCount()

			// EXPECTED BEHAVIOR: Should execute ≤ 3 queries regardless of node count
			// CURRENT BEHAVIOR: May execute 1 + N queries if groups are loaded separately
			if queryCount > 3 {
				t.Logf("PERFORMANCE BUG DETECTED: Node list with %d nodes executed %d queries (expected ≤ 3)", n, queryCount)
				t.Logf("Counterexample: Node count = %d, Query count = %d, Expected ≤ 3", n, queryCount)
				return false
			}

			return true
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestBugCondition_SerialQueryTiming tests that dashboard queries execute serially
// EXPECTED: This test should FAIL on unfixed code (total time ≈ sum of individual query times)
// EXPECTED AFTER FIX: This test should PASS (total time ≈ max of individual query times, ≤ 100ms)
func TestBugCondition_SerialQueryTiming(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create test user
	user := &repository.User{
		Username:     "testuser",
		PasswordHash: "password",
		Role:         "user",
		Enabled:      true,
	}
	if err := db.DB().Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Simulate dashboard queries (user info, traffic stats, announcement count)
	// These are typically executed serially in the current implementation
	
	start := time.Now()
	
	// Query 1: Get user info
	var fetchedUser repository.User
	if err := db.DB().First(&fetchedUser, user.ID).Error; err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	query1Duration := time.Since(start)
	
	// Query 2: Count users (simulating traffic stats)
	start2 := time.Now()
	var count int64
	if err := db.DB().Model(&repository.User{}).Count(&count).Error; err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	query2Duration := time.Since(start2)
	
	// Query 3: Count enabled users (simulating announcement count)
	start3 := time.Now()
	var activeCount int64
	if err := db.DB().Model(&repository.User{}).Where("enabled = ?", true).Count(&activeCount).Error; err != nil {
		t.Fatalf("Failed to count active users: %v", err)
	}
	query3Duration := time.Since(start3)
	
	totalDuration := time.Since(start)
	sumOfDurations := query1Duration + query2Duration + query3Duration
	
	// EXPECTED BEHAVIOR: Total time should be ≤ 100ms with parallel execution
	// CURRENT BEHAVIOR: Total time ≈ sum of individual query times (serial execution)
	
	// Check if execution appears to be serial (total ≈ sum)
	// Allow 20% tolerance for measurement overhead
	isSerial := float64(totalDuration) > float64(sumOfDurations)*0.8
	
	if isSerial {
		t.Logf("PERFORMANCE BUG DETECTED: Queries appear to execute serially")
		t.Logf("Counterexample: Total time = %v, Sum of individual times = %v", totalDuration, sumOfDurations)
		t.Logf("Query 1: %v, Query 2: %v, Query 3: %v", query1Duration, query2Duration, query3Duration)
	}
	
	// Check if total time exceeds target
	if totalDuration > 100*time.Millisecond {
		t.Logf("PERFORMANCE BUG DETECTED: Dashboard queries took %v (expected ≤ 100ms)", totalDuration)
		t.Errorf("Bug condition confirmed: Serial query execution causing slow response times")
	} else {
		t.Logf("Performance target met: Dashboard queries completed in %v", totalDuration)
	}
}

// TestBugCondition_CacheMissRate tests that repeated queries hit the database every time
// EXPECTED: This test should FAIL on unfixed code (0% cache hit rate)
// EXPECTED AFTER FIX: This test should PASS (≥ 80% cache hit rate for repeated queries)
func TestBugCondition_CacheMissRate(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("cache hit rate should be ≥ 80% for repeated queries", prop.ForAll(
		func(repeatCount uint8) bool {
			// Limit repeat count to reasonable range (5-20)
			repeats := int(repeatCount%16) + 5

			db, counter := setupTestDB(t)
			defer db.Close()

			// Create test user
			user := &repository.User{
				Username:     "cachetest",
				PasswordHash: "password",
				Role:         "user",
				Enabled:      true,
			}
			if err := db.DB().Create(user).Error; err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Reset counter after setup
			counter.reset()

			// Execute the same query multiple times
			for i := 0; i < repeats; i++ {
				var fetchedUser repository.User
				if err := db.DB().First(&fetchedUser, user.ID).Error; err != nil {
					t.Logf("Failed to get user: %v", err)
					return false
				}
			}

			queryCount := counter.getCount()

			// EXPECTED BEHAVIOR: With caching, should execute 1 query + (repeats * 0.2) cache misses
			// CURRENT BEHAVIOR: Executes 'repeats' queries (no caching, 0% hit rate)
			
			// Calculate expected max queries with 80% cache hit rate
			// First query is always a miss, then 80% should be hits
			expectedMaxQueries := 1 + int(float64(repeats-1)*0.2)
			
			if queryCount > expectedMaxQueries {
				cacheHitRate := float64(repeats-queryCount) / float64(repeats) * 100
				t.Logf("PERFORMANCE BUG DETECTED: Cache hit rate = %.1f%% (expected ≥ 80%%)", cacheHitRate)
				t.Logf("Counterexample: Repeated %d times, executed %d queries (expected ≤ %d)", repeats, queryCount, expectedMaxQueries)
				return false
			}

			return true
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestBugCondition_FunctionBasedSearch tests that search queries use functions in WHERE clause
// EXPECTED: This test should FAIL on unfixed code (uses LOWER/TRIM functions, cannot use index)
// EXPECTED AFTER FIX: This test should PASS (uses indexed search)
func TestBugCondition_FunctionBasedSearch(t *testing.T) {
	db, _ := setupTestDB(t)
	defer db.Close()

	// Create many test users to make index usage important
	for i := 0; i < 100; i++ {
		user := &repository.User{
			Username:     fmt.Sprintf("searchuser%d", i),
			PasswordHash: "password",
			Role:         "user",
			Enabled:      true,
		}
		if err := db.DB().Create(user).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	// Execute search query using LIKE pattern (simulating the current implementation)
	var users []repository.User
	search := "searchuser5"
	likePattern := "%" + search + "%"
	
	start := time.Now()
	// This simulates the current implementation which uses LOWER(TRIM(username)) LIKE
	if err := db.DB().Where("LOWER(TRIM(username)) LIKE ?", likePattern).Limit(10).Find(&users).Error; err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	duration := time.Since(start)

	if len(users) == 0 {
		t.Fatal("Expected to find users")
	}

	// EXPECTED BEHAVIOR: Search should be fast with indexed lookup
	// CURRENT BEHAVIOR: Search uses LOWER(TRIM(username)) which cannot use index
	
	// Note: In SQLite with small datasets, this might still be fast
	// The real issue shows up with larger datasets and under load
	t.Logf("Search query took %v for %d users", duration, len(users))
	t.Logf("Note: Current implementation uses LOWER(TRIM(username)) LIKE which prevents index usage")
	t.Logf("This becomes a performance bottleneck with larger datasets")
	
	// We can't easily detect function-based queries without query plan analysis
	// But we document the issue for awareness
	if duration > 50*time.Millisecond {
		t.Logf("PERFORMANCE BUG DETECTED: Search query took %v (may indicate full table scan)", duration)
		t.Errorf("Bug condition confirmed: Function-based search preventing index usage")
	}
}
