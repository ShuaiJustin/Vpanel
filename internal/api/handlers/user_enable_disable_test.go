package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/auth"
	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
)

// Feature: project-optimization, Property 8: User Enable/Disable
// For any disabled user account, login attempts SHALL be rejected, and for any enabled
// user account with valid credentials, login attempts SHALL succeed.
// **Validates: Requirements 17.1, 17.2, 17.3**

func init() {
	gin.SetMode(gin.TestMode)
}

// genUsername generates valid usernames (10 alphanumeric characters)
func genUsername() gopter.Gen {
	return gen.SliceOfN(10, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// genPassword generates valid passwords (10 alphanumeric characters)
func genPassword() gopter.Gen {
	return gen.SliceOfN(10, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// setupUserTestDB creates an in-memory SQLite database for testing
func setupUserTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&repository.User{}, &repository.Node{}, &repository.Proxy{}, &repository.Traffic{}, &repository.Trial{}, &repository.SubscriptionPause{}, &repository.UserNodeAssignment{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// createTestUser creates a test user with the given parameters
func createTestUser(t *testing.T, db *gorm.DB, authSvc *auth.Service, username, password string, enabled bool) *repository.User {
	passwordHash, err := authSvc.HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	user := &repository.User{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        username + "@example.com",
		Role:         "user",
		Enabled:      true, // Create with enabled=true first
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// If we need the user to be disabled, update it after creation
	// This works around GORM's handling of boolean default values
	if !enabled {
		if err := db.Model(user).Update("enabled", false).Error; err != nil {
			t.Fatalf("Failed to disable user: %v", err)
		}
		user.Enabled = false
	}

	return user
}

// TestUserEnableDisable_Property tests that disabled users cannot login
// and enabled users with valid credentials can login.
func TestUserEnableDisable_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Disabled users cannot login
	properties.Property("disabled users cannot login with valid credentials", prop.ForAll(
		func(username, password string) bool {
			db := setupUserTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a disabled user
			user := createTestUser(t, db, authSvc, username, password, false)
			if user == nil {
				t.Log("Failed to create user")
				return true // Skip if user creation failed
			}

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/login", handler.Login)

			body := map[string]string{
				"username": username,
				"password": password,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Disabled user should not be able to login
			result := w.Code == http.StatusUnauthorized || w.Code == http.StatusForbidden
			if !result {
				t.Logf("Unexpected status code: %d, body: %s", w.Code, w.Body.String())
			}
			return result
		},
		genUsername(),
		genPassword(),
	))

	// Property: Enabled users can login with valid credentials
	properties.Property("enabled users can login with valid credentials", prop.ForAll(
		func(username, password string) bool {
			db := setupUserTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create an enabled user
			createTestUser(t, db, authSvc, username, password, true)

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/login", handler.Login)

			body := map[string]string{
				"username": username,
				"password": password,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Enabled user with valid credentials should be able to login
			return w.Code == http.StatusOK
		},
		genUsername(),
		genPassword(),
	))

	properties.TestingRun(t)
}

// TestUserEnableDisable_EnableEndpoint tests the enable user endpoint
func TestUserEnableDisable_EnableEndpoint(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("enabling a disabled user changes their status to enabled", prop.ForAll(
		func(username string) bool {
			db := setupUserTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create a disabled user
			user := createTestUser(t, db, authSvc, username, "password123", false)

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users/:id/enable", func(c *gin.Context) {
				c.Set("user_id", int64(999)) // Admin user
				handler.EnableUser(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/enable", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				return false
			}

			// Verify user is now enabled
			updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
			if err != nil {
				return false
			}

			return updatedUser.Enabled == true
		},
		genUsername(),
	))

	properties.TestingRun(t)
}

// TestUserEnableDisable_DisableEndpoint tests the disable user endpoint
func TestUserEnableDisable_DisableEndpoint(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	properties.Property("disabling an enabled user changes their status to disabled", prop.ForAll(
		func(username string) bool {
			db := setupUserTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create an enabled user
			user := createTestUser(t, db, authSvc, username, "password123", true)

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users/:id/disable", func(c *gin.Context) {
				c.Set("user_id", int64(999)) // Admin user (different from target)
				handler.DisableUser(c)
			})

			req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/disable", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				return false
			}

			// Verify user is now disabled
			updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
			if err != nil {
				return false
			}

			return updatedUser.Enabled == false
		},
		genUsername(),
	))

	properties.TestingRun(t)
}

// TestUserEnableDisable_CannotDisableSelf tests that users cannot disable themselves
func TestUserEnableDisable_CannotDisableSelf(t *testing.T) {
	db := setupUserTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	// Create an enabled user
	user := createTestUser(t, db, authSvc, "selfuser", "password123", true)

	handler := NewAuthHandler(authSvc, userRepo, nil, log)

	router := gin.New()
	router.POST("/users/:id/disable", func(c *gin.Context) {
		c.Set("user_id", user.ID) // Same user trying to disable themselves
		handler.DisableUser(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/disable", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should return 400 Bad Request
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	// Verify user is still enabled
	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if !updatedUser.Enabled {
		t.Error("User should still be enabled after failed self-disable attempt")
	}
}

// TestUserEnableDisable_LoginAfterStateChange tests login behavior after enable/disable
func TestUserEnableDisable_LoginAfterStateChange(t *testing.T) {
	db := setupUserTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	password := "password123"
	user := createTestUser(t, db, authSvc, "stateuser", password, true)

	handler := NewAuthHandler(authSvc, userRepo, nil, log)

	router := gin.New()
	router.POST("/login", handler.Login)
	router.POST("/users/:id/disable", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.DisableUser(c)
	})
	router.POST("/users/:id/enable", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.EnableUser(c)
	})

	// Step 1: Login should succeed when enabled
	body := map[string]string{"username": "stateuser", "password": password}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Step 1: Expected login to succeed, got status %d", w.Code)
	}

	// Step 2: Disable the user
	req = httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/disable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Step 2: Expected disable to succeed, got status %d", w.Code)
	}

	// Step 3: Login should fail when disabled
	jsonBody, _ = json.Marshal(body)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Errorf("Step 3: Expected login to fail for disabled user, got status %d", w.Code)
	}

	// Step 4: Re-enable the user
	req = httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/enable", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Step 4: Expected enable to succeed, got status %d", w.Code)
	}

	// Step 5: Login should succeed again
	jsonBody, _ = json.Marshal(body)
	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Step 5: Expected login to succeed after re-enable, got status %d", w.Code)
	}
}

func TestUserEnableDisable_DisableEndpointCleansRuntimeResources(t *testing.T) {
	db := setupUserTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	trialRepo := repository.NewTrialRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	pauseRepo := repository.NewPauseRepository(db)
	log := logger.NewNopLogger()

	user := createTestUser(t, db, authSvc, "cleanupdisableuser", "password123", true)
	node := &repository.Node{
		Name:    "cleanup-node",
		Address: "cleanup-node.example.com",
		Token:   "cleanup-node-token",
		Status:  repository.NodeStatusOnline,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}
	nodeRef := node.ID
	if err := db.Create(&repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "cleanup-proxy",
		Protocol: "vmess",
		Port:     23001,
		Host:     "127.0.0.1",
		Enabled:  true,
	}).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("Failed to create assignment: %v", err)
	}

	trialService := trialsvc.NewService(trialRepo, userRepo, log, nil)
	entitlementService := entitlement.NewService(
		userRepo,
		trialRepo,
		proxyRepo,
		nodeRepo,
		assignmentRepo,
		trialService,
		log,
	).WithPauseRepository(pauseRepo)

	var syncedNodeID int64
	entitlementService.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	handler := NewAuthHandler(authSvc, userRepo, nil, log).WithEntitlementService(entitlementService)

	router := gin.New()
	router.POST("/users/:id/disable", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.DisableUser(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/disable", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected disable to succeed, got %d: %s", w.Code, w.Body.String())
	}

	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to reload user: %v", err)
	}
	if updatedUser.Enabled {
		t.Fatal("Expected user to be disabled")
	}

	var proxyCount int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&proxyCount).Error; err != nil {
		t.Fatalf("Failed to count proxies: %v", err)
	}
	if proxyCount != 0 {
		t.Fatalf("Expected disabled user proxies to be cleaned up, got %d", proxyCount)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to reload assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("Expected disabled user assignment cleanup, got %+v", assignment)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("Expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}
}
