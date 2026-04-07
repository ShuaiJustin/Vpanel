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
	"v/internal/database/repository"
	"v/internal/logger"
)

// Feature: project-optimization, Property 10: Email Validation
// For any email address provided during user creation, the email SHALL match a valid
// email format pattern, or the creation SHALL be rejected.
// **Validates: Requirements 17.14**

// Feature: project-optimization, Property 11: Username Uniqueness
// For any user update attempting to change username to an existing username, the update
// SHALL be rejected with a conflict error.
// **Validates: Requirements 17.15**

// setupValidationTestDB creates an in-memory SQLite database for testing
func setupValidationTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(&repository.User{}, &repository.LoginHistory{}, &repository.Role{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// genValidEmail generates valid email addresses
func genValidEmail() gopter.Gen {
	return gen.SliceOfN(8, gen.AlphaLowerChar()).Map(func(chars []rune) string {
		return string(chars) + "@example.com"
	})
}

// genInvalidEmail generates invalid email addresses
func genInvalidEmail() gopter.Gen {
	return gen.OneGenOf(
		// Missing @
		gen.SliceOfN(8, gen.AlphaChar()).Map(func(chars []rune) string {
			return string(chars) + "example.com"
		}),
		// Missing domain
		gen.SliceOfN(8, gen.AlphaChar()).Map(func(chars []rune) string {
			return string(chars) + "@"
		}),
		// Missing local part
		gen.Const("@example.com"),
		// Double @
		gen.SliceOfN(5, gen.AlphaChar()).Map(func(chars []rune) string {
			return string(chars) + "@@example.com"
		}),
		// Missing TLD
		gen.SliceOfN(8, gen.AlphaChar()).Map(func(chars []rune) string {
			return string(chars) + "@example"
		}),
	)
}

// genValidUsername generates valid usernames
func genValidUsername() gopter.Gen {
	return gen.SliceOfN(10, gen.AlphaChar()).Map(func(chars []rune) string {
		return string(chars)
	})
}

// TestEmailValidation_Property tests that invalid emails are rejected
func TestEmailValidation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: Valid emails are accepted
	properties.Property("valid emails are accepted during user creation", prop.ForAll(
		func(username string, email string) bool {
			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users", func(c *gin.Context) {
				c.Set("user_id", int64(1))
				handler.CreateUser(c)
			})

			body := map[string]string{
				"username": username,
				"password": "password123",
				"email":    email,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Valid email should result in successful creation
			return w.Code == http.StatusCreated
		},
		genValidUsername(),
		genValidEmail(),
	))

	// Property: Invalid emails are rejected
	properties.Property("invalid emails are rejected during user creation", prop.ForAll(
		func(username string, email string) bool {
			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users", func(c *gin.Context) {
				c.Set("user_id", int64(1))
				handler.CreateUser(c)
			})

			body := map[string]string{
				"username": username,
				"password": "password123",
				"email":    email,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Invalid email should result in bad request
			// Note: Gin's binding validation may return 400 before our custom validation
			result := w.Code == http.StatusBadRequest
			if !result {
				t.Logf("Email '%s' got status %d, expected 400. Body: %s", email, w.Code, w.Body.String())
			}
			return result
		},
		genValidUsername(),
		genInvalidEmail(),
	))

	properties.TestingRun(t)
}

// TestUsernameUniqueness_Property tests that duplicate usernames are rejected
func TestUsernameUniqueness_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 50

	properties := gopter.NewProperties(parameters)

	// Property: Creating user with existing username fails
	properties.Property("creating user with existing username fails with conflict", prop.ForAll(
		func(username string) bool {
			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create first user
			passwordHash, _ := authSvc.HashPassword("password123")
			firstUser := &repository.User{
				Username:     username,
				PasswordHash: passwordHash,
				Email:        username + "@first.com",
				Role:         "user",
				Enabled:      true,
			}
			if err := db.Create(firstUser).Error; err != nil {
				return true // Skip if creation failed
			}

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users", func(c *gin.Context) {
				c.Set("user_id", int64(999))
				handler.CreateUser(c)
			})

			// Try to create second user with same username
			body := map[string]string{
				"username": username,
				"password": "password456",
				"email":    username + "@second.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should return conflict
			return w.Code == http.StatusConflict
		},
		genValidUsername(),
	))

	// Property: Updating username to existing username fails
	properties.Property("updating username to existing username fails with conflict", prop.ForAll(
		func(username1, username2 string) bool {
			if username1 == username2 {
				return true // Skip if same username
			}

			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create first user
			passwordHash, _ := authSvc.HashPassword("password123")
			firstUser := &repository.User{
				Username:     username1,
				PasswordHash: passwordHash,
				Email:        username1 + "@first.com",
				Role:         "user",
				Enabled:      true,
			}
			if err := db.Create(firstUser).Error; err != nil {
				return true
			}

			// Create second user
			secondUser := &repository.User{
				Username:     username2,
				PasswordHash: passwordHash,
				Email:        username2 + "@second.com",
				Role:         "user",
				Enabled:      true,
			}
			if err := db.Create(secondUser).Error; err != nil {
				return true
			}

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.PUT("/users/:id", func(c *gin.Context) {
				c.Set("user_id", int64(999))
				handler.UpdateUser(c)
			})

			// Try to update second user's username to first user's username
			body := map[string]string{
				"username": username1,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(secondUser.ID, 10), bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should return conflict
			return w.Code == http.StatusConflict
		},
		genValidUsername(),
		genValidUsername(),
	))

	// Property: Updating to same username succeeds
	properties.Property("updating user with same username succeeds", prop.ForAll(
		func(username string) bool {
			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			// Create user
			passwordHash, _ := authSvc.HashPassword("password123")
			user := &repository.User{
				Username:     username,
				PasswordHash: passwordHash,
				Email:        username + "@test.com",
				Role:         "user",
				Enabled:      true,
			}
			if err := db.Create(user).Error; err != nil {
				return true
			}

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.PUT("/users/:id", func(c *gin.Context) {
				c.Set("user_id", int64(999))
				handler.UpdateUser(c)
			})

			// Update with same username (should succeed)
			body := map[string]string{
				"username": username,
				"email":    username + "@updated.com",
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(user.ID, 10), bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			// Should succeed
			return w.Code == http.StatusOK
		},
		genValidUsername(),
	))

	properties.TestingRun(t)
}

// TestUsernameUniqueness_UpdateToNewUsername tests updating to a new unique username
func TestUsernameUniqueness_UpdateToNewUsername(t *testing.T) {
	db := setupValidationTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	// Create user
	passwordHash, _ := authSvc.HashPassword("password123")
	user := &repository.User{
		Username:     "originaluser",
		PasswordHash: passwordHash,
		Email:        "original@test.com",
		Role:         "user",
		Enabled:      true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	handler := NewAuthHandler(authSvc, userRepo, nil, log)

	router := gin.New()
	router.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.UpdateUser(c)
	})

	// Update to new unique username
	body := map[string]string{
		"username": "newuniqueuser",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(user.ID, 10), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify username was updated
	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if updatedUser.Username != "newuniqueuser" {
		t.Errorf("Expected username 'newuniqueuser', got '%s'", updatedUser.Username)
	}
}

// TestEmailValidation_EmptyEmail tests that empty email is allowed
func TestEmailValidation_EmptyEmail(t *testing.T) {
	db := setupValidationTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	handler := NewAuthHandler(authSvc, userRepo, nil, log)

	router := gin.New()
	router.POST("/users", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.CreateUser(c)
	})

	// Create user without email
	body := map[string]string{
		"username": "noemailuser",
		"password": "password123",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed (email is optional)
	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEmailValidation_InvalidEmail tests that specific invalid emails are rejected
func TestEmailValidation_InvalidEmail(t *testing.T) {
	invalidEmails := []string{
		"uyfagkkk@",
		"@example.com",
		"test@example",
		"test@@example.com",
		"testexample.com",
	}

	for _, email := range invalidEmails {
		t.Run("email_"+email, func(t *testing.T) {
			db := setupValidationTestDB(t)
			authSvc := auth.NewService(auth.Config{
				JWTSecret:   "test-secret-key-for-testing-12345",
				TokenExpiry: time.Hour,
			})
			userRepo := repository.NewUserRepository(db)
			log := logger.NewNopLogger()

			handler := NewAuthHandler(authSvc, userRepo, nil, log)

			router := gin.New()
			router.POST("/users", func(c *gin.Context) {
				c.Set("user_id", int64(1))
				handler.CreateUser(c)
			})

			body := map[string]string{
				"username": "testuser",
				"password": "password123",
				"email":    email,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Email '%s' got status %d, expected 400. Body: %s", email, w.Code, w.Body.String())
			}
		})
	}
}
