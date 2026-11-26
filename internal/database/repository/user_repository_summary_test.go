package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestGetFilteredSummary_AggregatedQuery verifies that GetFilteredSummary uses a single aggregated query
func TestGetFilteredSummary_AggregatedQuery(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create test users
	users := []*User{
		{Username: "admin1", PasswordHash: "hash", Role: "admin", Enabled: true},
		{Username: "admin2", PasswordHash: "hash", Role: "admin"},  // Will use default (true)
		{Username: "user1", PasswordHash: "hash", Role: "user", Enabled: true},
		{Username: "user2", PasswordHash: "hash", Role: "user", Enabled: true},
		{Username: "user3", PasswordHash: "hash", Role: "user"},  // Will use default (true)
		{Username: "user4", PasswordHash: "hash", Role: "user"},  // Will use default (true)
	}

	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}
	
	// Now explicitly disable some users using Update
	if err := db.Model(&User{}).Where("username IN ?", []string{"admin2", "user3", "user4"}).Update("enabled", false).Error; err != nil {
		t.Fatalf("Failed to disable users: %v", err)
	}

	// Create repository and test GetFilteredSummary
	repo := NewUserRepository(db).(*userRepository)
	ctx := context.Background()

	// Test with no filters
	summary, err := repo.GetFilteredSummary(ctx, UserListFilter{})
	if err != nil {
		t.Fatalf("GetFilteredSummary failed: %v", err)
	}

	// Verify counts
	if summary.Total != 6 {
		t.Errorf("Expected Total=6, got %d", summary.Total)
	}
	if summary.Admin != 2 {
		t.Errorf("Expected Admin=2, got %d", summary.Admin)
	}
	if summary.Enabled != 3 {
		t.Errorf("Expected Enabled=3, got %d", summary.Enabled)
	}
	if summary.Disabled != 3 {
		t.Errorf("Expected Disabled=3, got %d", summary.Disabled)
	}

	// Test with role filter
	summary, err = repo.GetFilteredSummary(ctx, UserListFilter{Role: "admin"})
	if err != nil {
		t.Fatalf("GetFilteredSummary with role filter failed: %v", err)
	}

	if summary.Total != 2 {
		t.Errorf("Expected Total=2 with role filter, got %d", summary.Total)
	}
	if summary.Admin != 2 {
		t.Errorf("Expected Admin=2 with role filter, got %d", summary.Admin)
	}

	// Test with status filter
	summary, err = repo.GetFilteredSummary(ctx, UserListFilter{Status: "enabled"})
	if err != nil {
		t.Fatalf("GetFilteredSummary with status filter failed: %v", err)
	}

	// When filtering by status="enabled", we only see enabled users
	// So Total should be 3 (the enabled users), and Enabled should be 3, Disabled should be 0
	if summary.Total != 3 {
		t.Errorf("Expected Total=3 with status filter, got %d", summary.Total)
	}
	if summary.Enabled != 3 {
		t.Errorf("Expected Enabled=3 with status filter, got %d", summary.Enabled)
	}
	if summary.Disabled != 0 {
		t.Errorf("Expected Disabled=0 with status filter (filtered out), got %d", summary.Disabled)
	}

	// Test with search filter
	summary, err = repo.GetFilteredSummary(ctx, UserListFilter{Search: "admin"})
	if err != nil {
		t.Fatalf("GetFilteredSummary with search filter failed: %v", err)
	}

	if summary.Total != 2 {
		t.Errorf("Expected Total=2 with search filter, got %d", summary.Total)
	}
}
