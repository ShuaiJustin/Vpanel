package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/settings"
)

func newAuthSecurityTestHandler(t *testing.T, systemSettings *settings.SystemSettings) (*AuthHandler, repository.UserRepository, *auth.Service) {
	t.Helper()

	db := setupValidationTestDB(t)
	require.NoError(t, db.AutoMigrate(&repository.Setting{}))

	authSvc := auth.NewService(auth.Config{
		JWTSecret:          "test-secret-key-for-auth-security",
		TokenExpiry:        2 * time.Hour,
		RefreshTokenExpiry: 24 * time.Hour,
	})
	userRepo := repository.NewUserRepository(db)
	loginHistoryRepo := repository.NewLoginHistoryRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	settingsSvc := settings.NewService(settingsRepo)

	if systemSettings != nil {
		require.NoError(t, settingsSvc.UpdateSystemSettings(context.Background(), systemSettings))
	}

	handler := NewAuthHandler(authSvc, userRepo, loginHistoryRepo, logger.NewNopLogger()).
		WithSecuritySettings(settingsSvc)

	return handler, userRepo, authSvc
}

func performAdminLogin(t *testing.T, router *gin.Engine, username, password, remoteAddr string) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"username": username,
		"password": password,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestAuthHandler_LoginRateLimitedAfterConfiguredFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)

	systemSettings := settings.DefaultSettings()
	systemSettings.EnableLoginLock = true
	systemSettings.MaxLoginAttempts = 2
	systemSettings.LockDuration = 15

	handler, userRepo, authSvc := newAuthSecurityTestHandler(t, systemSettings)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "admin-rate-limit",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/auth/login", handler.Login)

	for i := 0; i < 2; i++ {
		response := performAdminLogin(t, router, "admin-rate-limit", "wrong-password", "203.0.113.10:12345")
		require.Equal(t, http.StatusUnauthorized, response.Code, response.Body.String())
	}

	blocked := performAdminLogin(t, router, "admin-rate-limit", "wrong-password", "203.0.113.10:12345")
	require.Equal(t, http.StatusTooManyRequests, blocked.Code, blocked.Body.String())
}

func TestAuthHandler_LoginRejectsNonWhitelistedIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	systemSettings := settings.DefaultSettings()
	systemSettings.EnableIPWhitelist = true
	systemSettings.IPWhitelist = "192.168.10.0/24"

	handler, userRepo, authSvc := newAuthSecurityTestHandler(t, systemSettings)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "admin-whitelist",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/auth/login", handler.Login)

	response := performAdminLogin(t, router, "admin-whitelist", "password123", "203.0.113.20:34567")
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
}

func TestAuthHandler_LoginUsesConfiguredSessionTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)

	systemSettings := settings.DefaultSettings()
	systemSettings.SessionTimeout = 45

	handler, userRepo, authSvc := newAuthSecurityTestHandler(t, systemSettings)
	createManagedTestUser(t, userRepo, authSvc, &repository.User{
		Username: "admin-session-timeout",
		Role:     "admin",
		Enabled:  true,
	})

	router := gin.New()
	router.POST("/auth/login", handler.Login)

	response := performAdminLogin(t, router, "admin-session-timeout", "password123", "198.51.100.9:45678")
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	var loginResponse LoginResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &loginResponse))
	require.Equal(t, int64(45*60), loginResponse.ExpiresIn)

	claims, err := authSvc.ValidateToken(loginResponse.Token)
	require.NoError(t, err)
	require.NotNil(t, claims.ExpiresAt)
	require.NotNil(t, claims.IssuedAt)

	tokenLifetime := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	require.InDelta(t, float64((45 * time.Minute).Seconds()), tokenLifetime.Seconds(), 2)
}
