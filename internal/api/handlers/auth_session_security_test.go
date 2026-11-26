package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"v/internal/api/middleware"
	"v/internal/database/repository"
	"v/internal/logger"
	portalauth "v/internal/portal/auth"
)

func currentTestTOTP(secret string) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(time.Now().Unix()/30))
	m := hmac.New(sha1.New, key)
	_, _ = m.Write(counter[:])
	sum := m.Sum(nil)
	offset := int(sum[len(sum)-1] & 15)
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1000000)
}

func authJSONRequest(t *testing.T, router *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestTwoFactorBothPasswordEntrypointsRequireOneTimeChallenge(t *testing.T) {
	router, portal, users := setupPortalTestRouter()
	password, err := portal.authService.HashPassword("password123")
	require.NoError(t, err)
	user := &repository.User{Username: "mfa-user", PasswordHash: password, Role: "admin", Enabled: true, TwoFactorEnabled: true}
	require.NoError(t, users.Create(context.Background(), user))
	tokens := newPortalMockAuthTokenRepo()
	secret := "JBSWY3DPEHPK3PXP"
	tokens.twoFactorSecrets[user.ID] = &repository.TwoFactorSecret{UserID: user.ID, Secret: secret, Enabled: true}
	portal.portalAuthService = portalauth.NewService(users, tokens).WithAuthService(portal.authService)
	admin := NewAuthHandler(portal.authService, users, nil, logger.NewNopLogger())
	router.POST("/api/auth/login", admin.Login)
	router.POST("/api/auth/2fa/login", portal.Verify2FALogin)
	for _, endpoint := range []string{"/api/auth/login", "/api/portal/auth/login"} {
		completionEndpoint := "/api/portal/auth/2fa/login"
		if endpoint == "/api/auth/login" {
			completionEndpoint = "/api/auth/2fa/login"
		}
		w := authJSONRequest(t, router, http.MethodPost, endpoint, map[string]any{"username": user.Username, "password": "password123"})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var first struct {
			Token       string `json:"token"`
			Requires2FA bool   `json:"requires_2fa"`
			Challenge   string `json:"challenge_token"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &first))
		require.True(t, first.Requires2FA)
		require.Empty(t, first.Token)
		require.NotEmpty(t, first.Challenge)
		bad := authJSONRequest(t, router, http.MethodPost, completionEndpoint, map[string]any{"user_id": user.ID, "code": currentTestTOTP(secret)})
		require.Equal(t, http.StatusBadRequest, bad.Code)
		body := map[string]any{"user_id": user.ID, "code": currentTestTOTP(secret), "challenge_token": first.Challenge}
		good := authJSONRequest(t, router, http.MethodPost, completionEndpoint, body)
		require.Equal(t, http.StatusOK, good.Code, good.Body.String())
		var result struct {
			Token        string `json:"token"`
			RefreshToken string `json:"refresh_token"`
		}
		require.NoError(t, json.Unmarshal(good.Body.Bytes(), &result))
		_, err := portal.authService.ValidateToken(result.Token)
		require.NoError(t, err)
		_, err = portal.authService.ValidateRefreshToken(result.RefreshToken)
		require.NoError(t, err)
		replay := authJSONRequest(t, router, http.MethodPost, completionEndpoint, body)
		require.Equal(t, http.StatusUnauthorized, replay.Code, replay.Body.String())
	}
}

func TestUserWriteCannotGrantOrTakeOverPrivilegedAccounts(t *testing.T) {
	h, users, svc := newUserManagementTestHandler(t)
	admin := createManagedTestUser(t, users, svc, &repository.User{Username: "protected-admin", Role: "admin", Enabled: true})
	user := createManagedTestUser(t, users, svc, &repository.User{Username: "normal-user", Role: "user", Enabled: true})
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("user_id", int64(999)); c.Set("role", "support"); c.Next() })
	router.POST("/users", h.CreateUser)
	router.PUT("/users/:id", h.UpdateUser)
	router.POST("/users/:id/reset-password", h.ResetPassword)
	for _, test := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/users", map[string]any{"username": "new-admin", "password": "password123", "role": "admin"}},
		{http.MethodPut, "/users/" + strconv.FormatInt(user.ID, 10), map[string]any{"role": "admin"}},
		{http.MethodPut, "/users/" + strconv.FormatInt(admin.ID, 10), map[string]any{"password": "takeover123"}},
		{http.MethodPost, "/users/" + strconv.FormatInt(admin.ID, 10) + "/reset-password", map[string]any{}},
	} {
		w := authJSONRequest(t, router, test.method, test.path, test.body)
		require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	}
	current, err := users.GetByID(context.Background(), admin.ID)
	require.NoError(t, err)
	require.Equal(t, admin.PasswordHash, current.PasswordHash)
	w := authJSONRequest(t, router, http.MethodPost, "/users", map[string]any{"username": "new-user", "password": "password123", "role": "user"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestAuthSessionEndpointsEnforcePurposeRotationAndLogout(t *testing.T) {
	router, portal, users := setupPortalTestRouter()
	user := &repository.User{Username: "session-user", PasswordHash: "stored-hash", Role: "admin", Enabled: true}
	require.NoError(t, users.Create(context.Background(), user))
	admin := NewAuthHandler(portal.authService, users, nil, logger.NewNopLogger())
	check := middleware.NewAuthMiddleware(portal.authService, logger.NewNopLogger()).WithUserRepository(users)
	router.POST("/api/auth/refresh", admin.RefreshToken)
	router.POST("/api/auth/logout", check.Authenticate(), admin.Logout)
	router.POST("/api/portal/auth/logout", check.Authenticate(), portal.Logout)
	for _, logoutPath := range []string{"/api/auth/logout", "/api/portal/auth/logout"} {
		access, refresh, err := portal.authService.GenerateTokenPair(user.ID, user.Username, user.Role, time.Hour)
		require.NoError(t, err)
		wrongPurpose := authJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{"refresh_token": access})
		require.Equal(t, http.StatusUnauthorized, wrongPurpose.Code)
		rotated := authJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{"refresh_token": refresh})
		require.Equal(t, http.StatusOK, rotated.Code, rotated.Body.String())
		var pair LoginResponse
		require.NoError(t, json.Unmarshal(rotated.Body.Bytes(), &pair))
		replay := authJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{"refresh_token": refresh})
		require.Equal(t, http.StatusUnauthorized, replay.Code)
		_, err = portal.authService.ValidateToken(access)
		require.Error(t, err)
		for _, bearer := range []string{pair.RefreshToken, pair.Token} {
			req := httptest.NewRequest(http.MethodPost, logoutPath, nil)
			req.Header.Set("Authorization", "Bearer "+bearer)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if bearer == pair.RefreshToken {
				require.Equal(t, http.StatusUnauthorized, w.Code)
			} else {
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			}
		}
		_, err = portal.authService.ValidateToken(pair.Token)
		require.Error(t, err)
		afterLogout := authJSONRequest(t, router, http.MethodPost, "/api/auth/refresh", map[string]any{"refresh_token": pair.RefreshToken})
		require.Equal(t, http.StatusUnauthorized, afterLogout.Code)
	}
}
