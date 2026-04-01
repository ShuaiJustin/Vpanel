package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
)

func setupAuthMiddlewareTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&repository.User{}); err != nil {
		t.Fatalf("failed to migrate user table: %v", err)
	}

	return db
}

func createAuthMiddlewareTestUser(t *testing.T, userRepo repository.UserRepository, authSvc *auth.Service, user *repository.User) *repository.User {
	t.Helper()

	if user.PasswordHash == "" {
		passwordHash, err := authSvc.HashPassword("password123")
		if err != nil {
			t.Fatalf("failed to hash password: %v", err)
		}
		user.PasswordHash = passwordHash
	}

	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return user
}

func TestAuthMiddleware_RejectsDisabledUserWithPreviouslyIssuedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAuthMiddlewareTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-auth-middleware-secret",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	user := createAuthMiddlewareTestUser(t, userRepo, authSvc, &repository.User{
		Username: "disabled-admin",
		Role:     "admin",
		Enabled:  true,
	})

	token, err := authSvc.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	user.Enabled = false
	if err := userRepo.Update(context.Background(), user); err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	router := gin.New()
	authMiddleware := NewAuthMiddleware(authSvc, logger.NewNopLogger()).WithUserRepository(userRepo)
	router.Use(authMiddleware.Authenticate())
	router.GET("/admin", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_UsesCurrentRoleForAdminChecks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := setupAuthMiddlewareTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-auth-middleware-secret",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	user := createAuthMiddlewareTestUser(t, userRepo, authSvc, &repository.User{
		Username: "demoted-admin",
		Role:     "admin",
		Enabled:  true,
	})

	token, err := authSvc.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	user.Role = "user"
	if err := userRepo.Update(context.Background(), user); err != nil {
		t.Fatalf("failed to demote user: %v", err)
	}

	router := gin.New()
	authMiddleware := NewAuthMiddleware(authSvc, logger.NewNopLogger()).WithUserRepository(userRepo)
	router.Use(authMiddleware.Authenticate())
	router.GET("/admin", authMiddleware.RequireRole("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}
