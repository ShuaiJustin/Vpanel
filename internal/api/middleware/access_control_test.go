package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Feature: project-optimization, Property 9: User Access Control
// For any user whose traffic exceeds their limit OR whose account has expired,
// proxy access SHALL be denied.
// **Validates: Requirements 17.9, 17.10**

func init() {
	gin.SetMode(gin.TestMode)
}

// setupAccessControlTestDB creates an in-memory SQLite database for testing
func setupAccessControlTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&repository.User{}, &repository.Proxy{}, &repository.Traffic{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// createAccessControlTestUser creates a test user with specific access control settings
func createAccessControlTestUser(t *testing.T, db *gorm.DB, username string, enabled bool, trafficLimit, trafficUsed int64, expiresAt *time.Time) *repository.User {
	user := &repository.User{
		Username:     username,
		PasswordHash: "hash",
		Email:        username + "@example.com",
		Role:         "user",
		Enabled:      true, // Create with enabled=true first
		TrafficLimit: trafficLimit,
		TrafficUsed:  trafficUsed,
		ExpiresAt:    expiresAt,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Update enabled status if needed (workaround for GORM default value handling)
	if !enabled {
		if err := db.Model(user).Update("enabled", false).Error; err != nil {
			t.Fatalf("Failed to update user enabled status: %v", err)
		}
		user.Enabled = false
	}

	return user
}

// genUsername generates valid usernames
func genAccessControlUsername() gopter.Gen {
	return gen.SliceOfN(10, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// TestUserAccessControl_TrafficExceeded tests that users who exceed traffic limits are denied access
func TestUserAccessControl_TrafficExceeded(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("users who exceed traffic limit are denied proxy access", prop.ForAll(
		func(username string, trafficLimit int64) bool {
			if trafficLimit <= 0 {
				return true // Skip - no limit means no restriction
			}

			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with traffic exceeded (trafficUsed >= trafficLimit)
			trafficUsed := trafficLimit + 1000 // Exceed the limit
			user := createAccessControlTestUser(t, db, username, true, trafficLimit, trafficUsed, nil)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be forbidden
			return w.Code == http.StatusForbidden
		},
		genAccessControlUsername(),
		gen.Int64Range(1, 1000000), // Traffic limit > 0
	))

	properties.TestingRun(t)
}

// TestUserAccessControl_AccountExpired tests that users with expired accounts are denied access
func TestUserAccessControl_AccountExpired(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("users with expired accounts are denied proxy access", prop.ForAll(
		func(username string, daysExpired int) bool {
			if daysExpired <= 0 {
				return true // Skip - not expired
			}

			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with expired account
			expiredTime := time.Now().AddDate(0, 0, -daysExpired) // Expired daysExpired days ago
			user := createAccessControlTestUser(t, db, username, true, 0, 0, &expiredTime)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be forbidden
			return w.Code == http.StatusForbidden
		},
		genAccessControlUsername(),
		gen.IntRange(1, 365), // Days expired (1-365)
	))

	properties.TestingRun(t)
}

// TestUserAccessControl_ValidUser tests that valid users can access proxies
func TestUserAccessControl_ValidUser(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("valid users can access proxies", prop.ForAll(
		func(username string) bool {
			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create valid user (enabled, no traffic limit, not expired)
			user := createAccessControlTestUser(t, db, username, true, 0, 0, nil)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be allowed
			return w.Code == http.StatusOK
		},
		genAccessControlUsername(),
	))

	properties.TestingRun(t)
}

// TestUserAccessControl_DisabledUser tests that disabled users are denied access
func TestUserAccessControl_DisabledUser(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("disabled users are denied proxy access", prop.ForAll(
		func(username string) bool {
			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create disabled user
			user := createAccessControlTestUser(t, db, username, false, 0, 0, nil)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be forbidden
			return w.Code == http.StatusForbidden
		},
		genAccessControlUsername(),
	))

	properties.TestingRun(t)
}

// TestUserAccessControl_TrafficWithinLimit tests that users within traffic limit can access
func TestUserAccessControl_TrafficWithinLimit(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("users within traffic limit can access proxies", prop.ForAll(
		func(username string, trafficLimit int64, usagePercent int) bool {
			if trafficLimit <= 0 {
				return true // Skip - no limit
			}

			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Calculate traffic used (0-99% of limit)
			trafficUsed := (trafficLimit * int64(usagePercent)) / 100

			user := createAccessControlTestUser(t, db, username, true, trafficLimit, trafficUsed, nil)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be allowed
			return w.Code == http.StatusOK
		},
		genAccessControlUsername(),
		gen.Int64Range(1000, 1000000), // Traffic limit
		gen.IntRange(0, 99),           // Usage percent (0-99%)
	))

	properties.TestingRun(t)
}

// TestUserAccessControl_NotExpiredYet tests that users with future expiration can access
func TestUserAccessControl_NotExpiredYet(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("users with future expiration can access proxies", prop.ForAll(
		func(username string, daysUntilExpiry int) bool {
			if daysUntilExpiry <= 0 {
				return true // Skip
			}

			db := setupAccessControlTestDB(t)
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user with future expiration
			futureTime := time.Now().AddDate(0, 0, daysUntilExpiry)
			user := createAccessControlTestUser(t, db, username, true, 0, 0, &futureTime)

			middleware := NewAccessControlMiddleware(userRepo, log)

			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", user.ID)
				c.Next()
			})
			router.Use(middleware.CheckProxyAccess())
			router.GET("/proxies", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			})

			req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should be allowed
			return w.Code == http.StatusOK
		},
		genAccessControlUsername(),
		gen.IntRange(1, 365), // Days until expiry
	))

	properties.TestingRun(t)
}

// TestUserModel_CanAccess tests the User.CanAccess() method directly
func TestUserModel_CanAccess(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("CanAccess returns false when any access condition fails", prop.ForAll(
		func(enabled bool, trafficLimit, trafficUsed int64, daysOffset int) bool {
			var expiresAt *time.Time
			if daysOffset != 0 {
				t := time.Now().AddDate(0, 0, daysOffset)
				expiresAt = &t
			}

			user := &repository.User{
				Enabled:      enabled,
				TrafficLimit: trafficLimit,
				TrafficUsed:  trafficUsed,
				ExpiresAt:    expiresAt,
			}

			canAccess := user.CanAccess()

			// Calculate expected result
			isExpired := expiresAt != nil && time.Now().After(*expiresAt)
			isTrafficExceeded := trafficLimit > 0 && trafficUsed >= trafficLimit

			expectedCanAccess := enabled && !isExpired && !isTrafficExceeded

			return canAccess == expectedCanAccess
		},
		gen.Bool(),                    // enabled
		gen.Int64Range(0, 1000000),    // trafficLimit
		gen.Int64Range(0, 2000000),    // trafficUsed
		gen.IntRange(-30, 30),         // daysOffset (negative = expired, positive = future)
	))

	properties.TestingRun(t)
}

// Unit test for edge case: exactly at traffic limit
func TestUserAccessControl_ExactlyAtTrafficLimit(t *testing.T) {
	db := setupAccessControlTestDB(t)
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	// Create user with traffic exactly at limit
	trafficLimit := int64(1000000)
	user := createAccessControlTestUser(t, db, "exactlimit", true, trafficLimit, trafficLimit, nil)

	middleware := NewAccessControlMiddleware(userRepo, log)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Next()
	})
	router.Use(middleware.CheckProxyAccess())
	router.GET("/proxies", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should be forbidden (at limit = exceeded)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// Unit test for edge case: expiring today
func TestUserAccessControl_ExpiringToday(t *testing.T) {
	db := setupAccessControlTestDB(t)
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	// Create user expiring 1 second ago
	expiredTime := time.Now().Add(-time.Second)
	user := createAccessControlTestUser(t, db, "expiringtoday", true, 0, 0, &expiredTime)

	middleware := NewAccessControlMiddleware(userRepo, log)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Next()
	})
	router.Use(middleware.CheckProxyAccess())
	router.GET("/proxies", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should be forbidden
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

// Unit test: admins should be able to manage proxies even if their own
// subscription metadata is expired or over limit.
func TestUserAccessControl_AdminBypassesProxyAccessChecks(t *testing.T) {
	db := setupAccessControlTestDB(t)
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	expiredTime := time.Now().Add(-time.Hour)
	user := createAccessControlTestUser(t, db, "adminexpired", true, 1024, 2048, &expiredTime)
	if err := db.Model(user).Update("role", "admin").Error; err != nil {
		t.Fatalf("Failed to update user role: %v", err)
	}
	user.Role = "admin"

	middleware := NewAccessControlMiddleware(userRepo, log)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Next()
	})
	router.Use(middleware.CheckProxyAccess())
	router.GET("/proxies", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin, got %d", w.Code)
	}
}

// Unit test: no user_id in context should pass through
func TestUserAccessControl_NoUserID(t *testing.T) {
	db := setupAccessControlTestDB(t)
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	middleware := NewAccessControlMiddleware(userRepo, log)

	router := gin.New()
	// Don't set user_id
	router.Use(middleware.CheckProxyAccess())
	router.GET("/proxies", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req := httptest.NewRequest(http.MethodGet, "/proxies", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should pass through (no user_id means auth middleware didn't run or public route)
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
