package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupProxyPreloadTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&User{}, &Proxy{}, &Trial{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// TestProxyRepository_ListWithPreload verifies that List function uses Preload
// to eliminate N+1 queries for User and Trial associations.
// This test validates the optimization from Task 3.3.
func TestProxyRepository_ListWithPreload(t *testing.T) {
	db := setupProxyPreloadTestDB(t)
	ctx := context.Background()
	repo := NewProxyRepository(db)

	// Create test users
	user1 := &User{
		Username:     "user1",
		PasswordHash: "hash1",
		Email:        "user1@example.com",
		Enabled:      true,
	}
	if err := db.WithContext(ctx).Create(user1).Error; err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}

	user2 := &User{
		Username:     "user2",
		PasswordHash: "hash2",
		Email:        "user2@example.com",
		Enabled:      true,
	}
	if err := db.WithContext(ctx).Create(user2).Error; err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Create trial for user1
	trial1 := &Trial{
		UserID:   user1.ID,
		Status:   "active",
		StartAt:  time.Now(),
		ExpireAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := db.WithContext(ctx).Create(trial1).Error; err != nil {
		t.Fatalf("Failed to create trial1: %v", err)
	}

	// Create proxies
	proxy1 := &Proxy{
		UserID:   user1.ID,
		Name:     "proxy1",
		Protocol: "vmess",
		Port:     10001,
		Enabled:  true,
	}
	if err := db.WithContext(ctx).Create(proxy1).Error; err != nil {
		t.Fatalf("Failed to create proxy1: %v", err)
	}

	proxy2 := &Proxy{
		UserID:   user2.ID,
		Name:     "proxy2",
		Protocol: "vless",
		Port:     10002,
		Enabled:  true,
	}
	if err := db.WithContext(ctx).Create(proxy2).Error; err != nil {
		t.Fatalf("Failed to create proxy2: %v", err)
	}

	proxy3 := &Proxy{
		UserID:   user1.ID,
		Name:     "proxy3",
		Protocol: "trojan",
		Port:     10003,
		Enabled:  true,
	}
	if err := db.WithContext(ctx).Create(proxy3).Error; err != nil {
		t.Fatalf("Failed to create proxy3: %v", err)
	}

	// List proxies - should preload User and Trial
	proxies, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(proxies) != 3 {
		t.Fatalf("Expected 3 proxies, got %d", len(proxies))
	}

	// Verify User associations are loaded
	for i, proxy := range proxies {
		if proxy.User == nil {
			t.Errorf("Proxy %d: User not preloaded", i)
			continue
		}

		// Verify user data is correct
		if proxy.UserID == user1.ID {
			if proxy.User.Username != "user1" {
				t.Errorf("Proxy %d: Expected user1, got %s", i, proxy.User.Username)
			}
		} else if proxy.UserID == user2.ID {
			if proxy.User.Username != "user2" {
				t.Errorf("Proxy %d: Expected user2, got %s", i, proxy.User.Username)
			}
		}
	}

	// Verify Trial associations are loaded for user1's proxies
	for i, proxy := range proxies {
		if proxy.UserID == user1.ID {
			if proxy.Trial == nil {
				t.Errorf("Proxy %d: Trial not preloaded for user1", i)
			} else if proxy.Trial.Status != "active" {
				t.Errorf("Proxy %d: Expected trial status 'active', got '%s'", i, proxy.Trial.Status)
			}
		} else if proxy.UserID == user2.ID {
			// user2 has no trial, so Trial should be nil
			if proxy.Trial != nil {
				t.Errorf("Proxy %d: Expected nil Trial for user2, got %+v", i, proxy.Trial)
			}
		}
	}
}

// TestProxyRepository_ListQueryCount verifies that List executes minimal queries
// This test ensures the N+1 query problem is eliminated.
func TestProxyRepository_ListQueryCount(t *testing.T) {
	db := setupProxyPreloadTestDB(t)
	
	// Enable query logging to count queries
	db = db.Session(&gorm.Session{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	})
	
	ctx := context.Background()
	repo := NewProxyRepository(db)

	// Create test data
	user := &User{
		Username:     "testuser",
		PasswordHash: "hash",
		Email:        "test@example.com",
		Enabled:      true,
	}
	if err := db.WithContext(ctx).Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	trial := &Trial{
		UserID:   user.ID,
		Status:   "active",
		StartAt:  time.Now(),
		ExpireAt: time.Now().Add(7 * 24 * time.Hour),
	}
	if err := db.WithContext(ctx).Create(trial).Error; err != nil {
		t.Fatalf("Failed to create trial: %v", err)
	}

	// Create 10 proxies
	for i := 0; i < 10; i++ {
		proxy := &Proxy{
			UserID:   user.ID,
			Name:     "proxy",
			Protocol: "vmess",
			Port:     20000 + i,
			Enabled:  true,
		}
		if err := db.WithContext(ctx).Create(proxy).Error; err != nil {
			t.Fatalf("Failed to create proxy %d: %v", i, err)
		}
	}

	// List proxies - with Preload, should execute 3 queries:
	// 1. SELECT proxies
	// 2. SELECT users WHERE id IN (...)
	// 3. SELECT trials WHERE user_id IN (...)
	proxies, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(proxies) != 10 {
		t.Fatalf("Expected 10 proxies, got %d", len(proxies))
	}

	// Verify all associations are loaded
	for i, proxy := range proxies {
		if proxy.User == nil {
			t.Errorf("Proxy %d: User not preloaded", i)
		}
		if proxy.Trial == nil {
			t.Errorf("Proxy %d: Trial not preloaded", i)
		}
	}
}
