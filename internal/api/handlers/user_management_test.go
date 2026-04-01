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

func TestUpdateCurrentUser_UpdatesEmailAndNormalizesCase(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "profile-user",
		Email:    "profile@example.com",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.PUT("/auth/me", func(c *gin.Context) {
		c.Set("user_id", user.ID)
		handler.UpdateCurrentUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"email": "NewAdmin@Example.com ",
	})
	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBuffer(body))
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
	if updatedUser.Email != "newadmin@example.com" {
		t.Fatalf("expected normalized email newadmin@example.com, got %q", updatedUser.Email)
	}
}

func TestUpdateCurrentUser_RejectsDuplicateEmail(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	firstUser := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "first-user",
		Email:    "first@example.com",
		Role:     "admin",
		Enabled:  true,
	})
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "second-user",
		Email:    "second@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.PUT("/auth/me", func(c *gin.Context) {
		c.Set("user_id", firstUser.ID)
		handler.UpdateCurrentUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"email": "SECOND@example.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/auth/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateUser_RejectsDuplicateEmail(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "existing-user",
		Email:    "duplicate@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/users", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.CreateUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"username": "new-user",
		"password": "password123",
		"email":    "Duplicate@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateUser_RejectsDuplicateEmail(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	firstUser := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "editable-user",
		Email:    "editable@example.com",
		Role:     "user",
		Enabled:  true,
	})
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "taken-user",
		Email:    "taken@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.UpdateUser(c)
	})

	body, _ := json.Marshal(map[string]any{
		"email": "Taken@example.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatInt(firstUser.ID, 10), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d: %s", w.Code, w.Body.String())
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

func TestListUsers_AppliesServerSideFiltersAndPagination(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "alpha-admin",
		Email:    "alpha@example.com",
		Role:     "admin",
		Enabled:  true,
	})
	disabledUser := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "beta-user",
		Email:    "beta@example.com",
		Role:     "user",
		Enabled:  false,
	})
	disabledUser.Enabled = false
	if err := userRepo.Update(context.Background(), disabledUser); err != nil {
		t.Fatalf("failed to disable beta-user: %v", err)
	}
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "gamma-user",
		Email:    "gamma@example.com",
		Role:     "user",
		Enabled:  true,
	})

	router := gin.New()
	router.GET("/users", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.ListUsers(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/users?page=1&page_size=1&search=user&role=user&status=enabled", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload struct {
		Users []UserResponse `json:"users"`
		Total int            `json:"total"`
		Page  int            `json:"page"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if payload.Total != 1 {
		t.Fatalf("expected filtered total 1, got %d", payload.Total)
	}
	if payload.Page != 1 {
		t.Fatalf("expected page 1, got %d", payload.Page)
	}
	if len(payload.Users) != 1 || payload.Users[0].Username != "gamma-user" {
		t.Fatalf("expected gamma-user only, got %+v", payload.Users)
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

func TestRefreshToken_RejectsDisabledUser(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "refresh-disabled-user",
		Email:    "refresh-disabled@example.com",
		Role:     "admin",
		Enabled:  true,
	})

	refreshToken, err := authSvc.GenerateRefreshToken(user.ID)
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	user.Enabled = false
	if err := userRepo.Update(context.Background(), user); err != nil {
		t.Fatalf("failed to disable user: %v", err)
	}

	router := gin.New()
	router.POST("/refresh", handler.RefreshToken)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	req := httptest.NewRequest(http.MethodPost, "/refresh", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePassword_ClearsForcePasswordChangeFlag(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	user := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username:            "force-change-user",
		Email:               "force-change@example.com",
		Role:                "admin",
		Enabled:             true,
		ForcePasswordChange: true,
	})

	router := gin.New()
	router.PUT("/auth/password", func(c *gin.Context) {
		c.Set("user_id", user.ID)
		handler.ChangePassword(c)
	})

	body, _ := json.Marshal(map[string]string{
		"old_password": "password123",
		"new_password": "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPut, "/auth/password", bytes.NewBuffer(body))
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
	if updatedUser.ForcePasswordChange {
		t.Fatal("expected force_password_change to be cleared after password change")
	}
	if !authSvc.VerifyPassword("newpassword123", updatedUser.PasswordHash) {
		t.Fatal("expected new password to be persisted")
	}
}

func TestListUsers_ReturnsFilteredSummaryCounts(t *testing.T) {
	handler, userRepo, authSvc := newUserManagementTestHandler(t)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "summary-admin",
		Email:    "summary-admin@example.com",
		Role:     "admin",
		Enabled:  true,
	})
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "summary-user-enabled",
		Email:    "summary-enabled@example.com",
		Role:     "user",
		Enabled:  true,
	})
	disabledUser := createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "summary-user-disabled",
		Email:    "summary-disabled@example.com",
		Role:     "user",
		Enabled:  false,
	})
	disabledUser.Enabled = false
	if err := userRepo.Update(context.Background(), disabledUser); err != nil {
		t.Fatalf("failed to disable summary user: %v", err)
	}

	router := gin.New()
	router.GET("/users", func(c *gin.Context) {
		c.Set("user_id", int64(999))
		handler.ListUsers(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/users?search=summary", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload struct {
		Total         int   `json:"total"`
		AdminTotal    int   `json:"admin_total"`
		EnabledTotal  int   `json:"enabled_total"`
		DisabledTotal int   `json:"disabled_total"`
		CurrentUserID int64 `json:"current_user_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Total != 3 || payload.AdminTotal != 1 || payload.EnabledTotal != 2 || payload.DisabledTotal != 1 {
		t.Fatalf("unexpected summary counts: %+v", payload)
	}
	if payload.CurrentUserID != 999 {
		t.Fatalf("expected current_user_id 999, got %d", payload.CurrentUserID)
	}
}
