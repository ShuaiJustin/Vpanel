package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/logger"
	"v/internal/settings"
)

type basicAuthSettingsRepo struct {
	values map[string]string
}

func newBasicAuthSettingsRepo() *basicAuthSettingsRepo {
	return &basicAuthSettingsRepo{values: map[string]string{}}
}

func (r *basicAuthSettingsRepo) Get(ctx context.Context, key string) (string, error) {
	return r.values[key], nil
}

func (r *basicAuthSettingsRepo) GetAll(ctx context.Context) (map[string]string, error) {
	values := make(map[string]string, len(r.values))
	for key, value := range r.values {
		values[key] = value
	}
	return values, nil
}

func (r *basicAuthSettingsRepo) Set(ctx context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *basicAuthSettingsRepo) SetMultiple(ctx context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *basicAuthSettingsRepo) Delete(ctx context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func (r *basicAuthSettingsRepo) Backup(ctx context.Context) ([]byte, error) {
	return json.Marshal(r.values)
}

func (r *basicAuthSettingsRepo) Restore(ctx context.Context, data []byte) error {
	return json.Unmarshal(data, &r.values)
}

func TestBasicAuthGate_DisabledAllowsRequest(t *testing.T) {
	service := settings.NewService(newBasicAuthSettingsRepo())
	router := newBasicAuthTestRouter(service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestBasicAuthGate_RequiresConfiguredCredentials(t *testing.T) {
	service := settings.NewService(newBasicAuthSettingsRepo())
	enableBasicAuth(t, service, "edge", "secret")
	router := newBasicAuthTestRouter(service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Contains(t, recorder.Header().Get("WWW-Authenticate"), `Basic realm="V Panel"`)
}

func TestBasicAuthGate_AllowsMatchingCredentials(t *testing.T) {
	service := settings.NewService(newBasicAuthSettingsRepo())
	enableBasicAuth(t, service, "edge", "secret")
	router := newBasicAuthTestRouter(service)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.SetBasicAuth("edge", "secret")
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Contains(t, recorder.Header().Get("Set-Cookie"), basicAuthCookieName)
}

func TestBasicAuthGate_AllowsValidCookieWithBearerHeader(t *testing.T) {
	service := settings.NewService(newBasicAuthSettingsRepo())
	enableBasicAuth(t, service, "edge", "secret")
	router := newBasicAuthTestRouter(service)

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodGet, "/admin", nil)
	loginRequest.SetBasicAuth("edge", "secret")
	router.ServeHTTP(loginRecorder, loginRequest)
	require.Equal(t, http.StatusNoContent, loginRecorder.Code)
	require.NotEmpty(t, loginRecorder.Result().Cookies())

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	request.Header.Set("Authorization", "Bearer jwt-token")
	for _, cookie := range loginRecorder.Result().Cookies() {
		request.AddCookie(cookie)
	}
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
}

func TestBasicAuthGate_SkipsPublicMachineEndpoints(t *testing.T) {
	service := settings.NewService(newBasicAuthSettingsRepo())
	enableBasicAuth(t, service, "edge", "secret")
	router := newBasicAuthTestRouter(service)

	publicPaths := []string{
		"/health",
		"/api/node/register",
		"/api/subscription/token",
		"/s/shortcode",
		"/api/payments/callback/alipay",
		"/api/admin/nodes/agent/download",
	}

	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			router.ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusNoContent, recorder.Code)
		})
	}
}

func newBasicAuthTestRouter(service *settings.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BasicAuthGate(service, logger.NewNopLogger()))
	router.NoRoute(func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func enableBasicAuth(t *testing.T, service *settings.Service, username, password string) {
	t.Helper()

	systemSettings := settings.DefaultSettings()
	systemSettings.Auth.BasicAuth.Enabled = true
	systemSettings.Auth.BasicAuth.Username = username
	systemSettings.Auth.BasicAuth.Password = password
	require.NoError(t, service.UpdateSystemSettings(context.Background(), systemSettings))
}
