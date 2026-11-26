package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"v/internal/cache"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/portal/announcement"
)

// TestPortalAnnouncementHandlerWithCache verifies cache integration for announcement list endpoint
func TestPortalAnnouncementHandlerWithCache(t *testing.T) {
	// Setup database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Migrate schema
	err = db.AutoMigrate(&repository.Announcement{}, &repository.AnnouncementRead{}, &repository.User{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create test user
	testUser := &repository.User{
		Username: "testuser",
		Email:    "test@example.com",
		PasswordHash: "hashedpassword",
		Role:     "user",
		Enabled:  true,
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create test announcements
	announcements := []*repository.Announcement{
		{
			Title:       "Announcement 1",
			Content:     "Content 1",
			IsPublished: true,
			CreatedAt:   time.Now(),
		},
		{
			Title:       "Announcement 2",
			Content:     "Content 2",
			IsPublished: true,
			CreatedAt:   time.Now(),
		},
	}
	for _, ann := range announcements {
		if err := db.Create(ann).Error; err != nil {
			t.Fatalf("Failed to create announcement: %v", err)
		}
	}

	// Create cache
	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	// Create service with cache
	announcementRepo := repository.NewAnnouncementRepository(db)
	announcementService := announcement.NewService(announcementRepo).WithCache(testCache)

	// Create handler
	handler := NewPortalAnnouncementHandler(announcementService, logger.NewNopLogger())

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUser.ID)
		c.Next()
	})
	router.GET("/announcements", handler.ListAnnouncements)

	// First request - should hit database and cache result
	req1 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=0", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w1.Code)
	}

	var response1 map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &response1); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	announcements1 := response1["announcements"].([]interface{})
	if len(announcements1) != 2 {
		t.Errorf("Expected 2 announcements, got %d", len(announcements1))
	}

	// Verify cache was populated
	ctx := context.Background()
	cacheKeyBuilder := cache.NewCacheKeyBuilder()
	cacheKey := cacheKeyBuilder.AnnouncementListKey(0, 20)
	cachedData, err := testCache.Get(ctx, cacheKey)
	if err != nil {
		t.Errorf("Expected cache to be populated, got error: %v", err)
	}
	if cachedData == nil {
		t.Error("Expected cache to contain data")
	}

	// Second request - should hit cache
	req2 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=0", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}

	var response2 map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &response2); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	announcements2 := response2["announcements"].([]interface{})
	if len(announcements2) != 2 {
		t.Errorf("Expected 2 announcements, got %d", len(announcements2))
	}

	// Verify responses are identical
	if response1["total"] != response2["total"] {
		t.Error("Expected identical total counts from cache")
	}
}

// TestPortalAnnouncementHandlerCacheInvalidation verifies cache invalidation
func TestPortalAnnouncementHandlerCacheInvalidation(t *testing.T) {
	// Setup database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Migrate schema
	err = db.AutoMigrate(&repository.Announcement{}, &repository.AnnouncementRead{}, &repository.User{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create test user
	testUser := &repository.User{
		Username: "testuser",
		Email:    "test@example.com",
		PasswordHash: "hashedpassword",
		Role:     "user",
		Enabled:  true,
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create initial announcement
	announcement1 := &repository.Announcement{
		Title:       "Announcement 1",
		Content:     "Content 1",
		IsPublished: true,
		CreatedAt:   time.Now(),
	}
	if err := db.Create(announcement1).Error; err != nil {
		t.Fatalf("Failed to create announcement: %v", err)
	}

	// Create cache
	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	// Create service with cache
	announcementRepo := repository.NewAnnouncementRepository(db)
	announcementService := announcement.NewService(announcementRepo).WithCache(testCache)

	// Create handler
	handler := NewPortalAnnouncementHandler(announcementService, logger.NewNopLogger())

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUser.ID)
		c.Next()
	})
	router.GET("/announcements", handler.ListAnnouncements)

	// First request - cache the result
	req1 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=0", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	var response1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &response1)
	announcements1 := response1["announcements"].([]interface{})
	if len(announcements1) != 1 {
		t.Errorf("Expected 1 announcement, got %d", len(announcements1))
	}

	// Add a new announcement
	announcement2 := &repository.Announcement{
		Title:       "Announcement 2",
		Content:     "Content 2",
		IsPublished: true,
		CreatedAt:   time.Now(),
	}
	if err := db.Create(announcement2).Error; err != nil {
		t.Fatalf("Failed to create announcement: %v", err)
	}

	// Invalidate cache (simulating what would happen when admin creates announcement)
	ctx := context.Background()
	if err := announcementService.InvalidateCache(ctx); err != nil {
		t.Fatalf("Failed to invalidate cache: %v", err)
	}

	// Second request - should see new announcement
	req2 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=0", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	var response2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &response2)
	announcements2 := response2["announcements"].([]interface{})
	if len(announcements2) != 2 {
		t.Errorf("Expected 2 announcements after invalidation, got %d", len(announcements2))
	}
}

// TestPortalAnnouncementHandlerDifferentPages verifies different pages use different cache keys
func TestPortalAnnouncementHandlerDifferentPages(t *testing.T) {
	// Setup database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Migrate schema
	err = db.AutoMigrate(&repository.Announcement{}, &repository.AnnouncementRead{}, &repository.User{})
	if err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create test user
	testUser := &repository.User{
		Username: "testuser",
		Email:    "test@example.com",
		PasswordHash: "hashedpassword",
		Role:     "user",
		Enabled:  true,
	}
	if err := db.Create(testUser).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Create multiple announcements
	for i := 1; i <= 25; i++ {
		announcement := &repository.Announcement{
			Title:       "Announcement " + string(rune(i)),
			Content:     "Content " + string(rune(i)),
			IsPublished: true,
			CreatedAt:   time.Now().Add(-time.Duration(i) * time.Minute),
		}
		if err := db.Create(announcement).Error; err != nil {
			t.Fatalf("Failed to create announcement: %v", err)
		}
	}

	// Create cache
	testCache := cache.NewMemoryCache(cache.Config{
		Type:           "memory",
		DefaultTTL:     5 * time.Minute,
		MaxMemoryItems: 100,
		KeyPrefix:      "test:",
	})

	// Create service with cache
	announcementRepo := repository.NewAnnouncementRepository(db)
	announcementService := announcement.NewService(announcementRepo).WithCache(testCache)

	// Create handler
	handler := NewPortalAnnouncementHandler(announcementService, logger.NewNopLogger())

	// Setup router
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", testUser.ID)
		c.Next()
	})
	router.GET("/announcements", handler.ListAnnouncements)

	ctx := context.Background()
	cacheKeyBuilder := cache.NewCacheKeyBuilder()

	// Request page 0
	req1 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=0", nil)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	// Verify page 0 is cached
	cacheKey1 := cacheKeyBuilder.AnnouncementListKey(0, 20)
	cachedData1, err := testCache.Get(ctx, cacheKey1)
	if err != nil {
		t.Errorf("Expected page 0 to be cached, got error: %v", err)
	}
	if cachedData1 == nil {
		t.Error("Expected page 0 cache to contain data")
	}

	// Request page 1
	req2 := httptest.NewRequest(http.MethodGet, "/announcements?limit=20&offset=20", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	// Verify page 1 is cached separately
	cacheKey2 := cacheKeyBuilder.AnnouncementListKey(1, 20)
	cachedData2, err := testCache.Get(ctx, cacheKey2)
	if err != nil {
		t.Errorf("Expected page 1 to be cached, got error: %v", err)
	}
	if cachedData2 == nil {
		t.Error("Expected page 1 cache to contain data")
	}

	// Verify both cache keys exist
	exists1, _ := testCache.Exists(ctx, cacheKey1)
	exists2, _ := testCache.Exists(ctx, cacheKey2)

	if !exists1 {
		t.Error("Expected page 0 cache key to exist")
	}
	if !exists2 {
		t.Error("Expected page 1 cache key to exist")
	}
}
