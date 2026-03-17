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

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
)

func newUserManagementTestHandler(t *testing.T) (*AuthHandler, repository.UserRepository, *auth.Service) {
	t.Helper()

	db := setupValidationTestDB(t)
	authSvc := auth.NewService(auth.Config{
		JWTSecret:   "test-secret-key-for-testing-12345",
		TokenExpiry: time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	log := logger.NewNopLogger()

	return NewAuthHandler(authSvc, userRepo, nil, log), userRepo, authSvc
}

func createManagedTestUser(t *testing.T, userRepo repository.UserRepository, authSvc *auth.Service, user *repository.User) *repository.User {
	t.Helper()

	passwordHash, err := authSvc.HashPassword("password123")
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if user.PasswordHash == "" {
		user.PasswordHash = passwordHash
	}

	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

func TestUpdateUser_AllowsClearingEmail(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "editable-user",
		Email:    "editable@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.UpdateUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"username": user.Username,
		"email":    "",
		"role":     user.Role,
	})
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(user.ID, 10), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updatedUser.Email != "" {
		t.Fatalf("expected email to be cleared, got %q", updatedUser.Email)
	}
}

func TestUpdateUser_PreventsDemotingLastEnabledAdmin(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	admin := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "sole-admin",
		Email:    "admin@example.com",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.UpdateUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"username": admin.Username,
		"email":    admin.Email,
		"role":     "user",
	})
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(admin.ID, 10), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	updatedUser, err := userRepo.GetByID(context.Background(), admin.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updatedUser.Role != "admin" {
		t.Fatalf("expected user to remain admin, got %q", updatedUser.Role)
	}
}

func TestDeleteUser_PreventsDeletingLastEnabledAdmin(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	admin := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "delete-protected-admin",
		Email:    "protected@example.com",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.DELETE("/users/:id", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.DeleteUser(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/users/"+strconv.FormatInt(admin.ID, 10), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := userRepo.GetByID(context.Background(), admin.ID); err != nil {
		t.Fatalf("expected admin to remain present, got error: %v", err)
	}
}

func TestResetPassword_UpdatesHashAndForcesPasswordChange(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "reset-user",
		Email:    "reset@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/users/:id/reset-password", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.ResetPassword(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatInt(user.ID, 10)+"/reset-password", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response ResetPasswordResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.TemporaryPassword == "" {
		t.Fatal("expected temporary password in response")
	}

	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !updatedUser.ForcePasswordChange {
		t.Fatal("expected force_password_change to be enabled")
	}
	if !authSvc.VerifyPassword(response.TemporaryPassword, updatedUser.PasswordHash) {
		t.Fatal("expected temporary password to match updated hash")
	}
}

func TestListUsers_ReturnsTotalAndLastLogin(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	lastLoginAt := time.Date(2026, 3, 17, 10, 30, 0, 0, time.UTC)

	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username:    "user-one",
		Email:       "one@example.com",
		Role:        "user",
		Enabled:     true,
		LastLoginAt: &lastLoginAt,
	})
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "user-two",
		Email:    "two@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.GET("/users", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.ListUsers(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Users []UserResponse `json:"users"`
		Total int            `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if response.Total != 2 {
		t.Fatalf("expected total 2, got %d", response.Total)
	}

	found := false
	for _, user := range response.Users {
		if user.Username == "user-one" {
			found = true
			if user.LastLogin == "" {
				t.Fatal("expected last_login to be returned for user-one")
			}
		}
	}
	if !found {
		t.Fatal("expected user-one to be present in response")
	}
}

func TestLogin_UpdatesLastLoginMetadata(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "login-tracked-user",
		Email:    "login@example.com",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/login", handler.Login)

	body, _ := json.Marshal(map[string]string{
		"username": user.Username,
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.10:4567"
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updatedUser.LastLoginAt == nil {
		t.Fatal("expected last_login_at to be updated")
	}
	if updatedUser.LastLoginIP != "203.0.113.10" {
		t.Fatalf("expected last_login_ip to be updated, got %q", updatedUser.LastLoginIP)
	}

	var response LoginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if response.User == nil || response.User.LastLogin == "" {
		t.Fatal("expected login response to include last_login")
	}
}
