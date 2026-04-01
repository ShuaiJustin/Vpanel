package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/subscription"
	"v/pkg/errors"
)

// Mock repositories for testing
type mockSubscriptionRepo struct {
	subscriptions map[string]*repository.Subscription
	byUserID      map[int64]*repository.Subscription
}

func newMockSubscriptionRepo() *mockSubscriptionRepo {
	return &mockSubscriptionRepo{
		subscriptions: make(map[string]*repository.Subscription),
		byUserID:      make(map[int64]*repository.Subscription),
	}
}

func (m *mockSubscriptionRepo) Create(ctx context.Context, sub *repository.Subscription) error {
	sub.ID = int64(len(m.subscriptions) + 1)
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	m.subscriptions[sub.Token] = sub
	m.byUserID[sub.UserID] = sub
	return nil
}

func (m *mockSubscriptionRepo) GetByID(ctx context.Context, id int64) (*repository.Subscription, error) {
	for _, sub := range m.subscriptions {
		if sub.ID == id {
			return sub, nil
		}
	}
	return nil, newNotFoundError("subscription", id)
}

func (m *mockSubscriptionRepo) GetByToken(ctx context.Context, token string) (*repository.Subscription, error) {
	if sub, ok := m.subscriptions[token]; ok {
		return sub, nil
	}
	return nil, newNotFoundError("subscription", token)
}

func (m *mockSubscriptionRepo) GetByShortCode(ctx context.Context, shortCode string) (*repository.Subscription, error) {
	for _, sub := range m.subscriptions {
		if sub.ShortCode == shortCode {
			return sub, nil
		}
	}
	return nil, newNotFoundError("subscription", shortCode)
}

func (m *mockSubscriptionRepo) GetByUserID(ctx context.Context, userID int64) (*repository.Subscription, error) {
	if sub, ok := m.byUserID[userID]; ok {
		return sub, nil
	}
	return nil, newNotFoundError("subscription", userID)
}

func (m *mockSubscriptionRepo) Update(ctx context.Context, sub *repository.Subscription) error {
	for token, existing := range m.subscriptions {
		if existing.ID == sub.ID && token != sub.Token {
			delete(m.subscriptions, token)
			break
		}
	}
	sub.UpdatedAt = time.Now()
	m.subscriptions[sub.Token] = sub
	m.byUserID[sub.UserID] = sub
	return nil
}

func (m *mockSubscriptionRepo) Delete(ctx context.Context, id int64) error {
	for token, sub := range m.subscriptions {
		if sub.ID == id {
			delete(m.subscriptions, token)
			delete(m.byUserID, sub.UserID)
			return nil
		}
	}
	return newNotFoundError("subscription", id)
}

func (m *mockSubscriptionRepo) DeleteByUserID(ctx context.Context, userID int64) error {
	if sub, ok := m.byUserID[userID]; ok {
		delete(m.subscriptions, sub.Token)
		delete(m.byUserID, userID)
		return nil
	}
	return newNotFoundError("subscription", userID)
}

func (m *mockSubscriptionRepo) UpdateAccessStats(ctx context.Context, id int64, ip string, userAgent string) error {
	for _, sub := range m.subscriptions {
		if sub.ID == id {
			sub.AccessCount++
			now := time.Now()
			sub.LastAccessAt = &now
			sub.LastIP = ip
			sub.LastUA = userAgent
			return nil
		}
	}
	return newNotFoundError("subscription", id)
}

func (m *mockSubscriptionRepo) ListAll(ctx context.Context, filter *repository.SubscriptionFilter) ([]*repository.Subscription, int64, error) {
	var result []*repository.Subscription
	for _, sub := range m.subscriptions {
		result = append(result, sub)
	}
	return result, int64(len(result)), nil
}

func (m *mockSubscriptionRepo) ResetAccessStats(ctx context.Context, id int64) error {
	for _, sub := range m.subscriptions {
		if sub.ID == id {
			sub.AccessCount = 0
			sub.LastAccessAt = nil
			sub.LastIP = ""
			sub.LastUA = ""
			return nil
		}
	}
	return newNotFoundError("subscription", id)
}

func newNotFoundError(resource string, id interface{}) error {
	return errors.NewNotFoundError(resource, id)
}

type mockUserRepo struct {
	users map[int64]*repository.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users: make(map[int64]*repository.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, user *repository.User) error {
	user.ID = int64(len(m.users) + 1)
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, newNotFoundError("user", id)
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	for _, user := range m.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, newNotFoundError("user", username)
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, newNotFoundError("user", email)
}

func (m *mockUserRepo) Update(ctx context.Context, user *repository.User) error {
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.users[id]; ok {
		delete(m.users, id)
		return nil
	}
	return newNotFoundError("user", id)
}

func (m *mockUserRepo) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	var result []*repository.User
	for _, user := range m.users {
		result = append(result, user)
	}
	return result, nil
}

func (m *mockUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *mockUserRepo) CountActive(ctx context.Context) (int64, error) {
	var count int64
	for _, user := range m.users {
		if user.Enabled {
			count++
		}
	}
	return count, nil
}

type mockProxyRepo struct {
	proxies map[int64]*repository.Proxy
}

func newMockProxyRepo() *mockProxyRepo {
	return &mockProxyRepo{
		proxies: make(map[int64]*repository.Proxy),
	}
}

func (m *mockProxyRepo) Create(ctx context.Context, proxy *repository.Proxy) error {
	proxy.ID = int64(len(m.proxies) + 1)
	proxy.CreatedAt = time.Now()
	proxy.UpdatedAt = time.Now()
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepo) GetByID(ctx context.Context, id int64) (*repository.Proxy, error) {
	if proxy, ok := m.proxies[id]; ok {
		return proxy, nil
	}
	return nil, newNotFoundError("proxy", id)
}

func (m *mockProxyRepo) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepo) GetByPort(ctx context.Context, port int) (*repository.Proxy, error) {
	for _, proxy := range m.proxies {
		if proxy.Port == port {
			return proxy, nil
		}
	}
	return nil, newNotFoundError("proxy", port)
}

func (m *mockProxyRepo) Update(ctx context.Context, proxy *repository.Proxy) error {
	proxy.UpdatedAt = time.Now()
	m.proxies[proxy.ID] = proxy
	return nil
}

func (m *mockProxyRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.proxies[id]; ok {
		delete(m.proxies, id)
		return nil
	}
	return newNotFoundError("proxy", id)
}

func (m *mockProxyRepo) List(ctx context.Context, limit, offset int) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, proxy := range m.proxies {
		result = append(result, proxy)
	}
	return result, nil
}

func (m *mockProxyRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.proxies)), nil
}

func (m *mockProxyRepo) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	var count int64
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepo) GetByProtocol(ctx context.Context, protocol string) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, proxy := range m.proxies {
		if proxy.Protocol == protocol {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepo) GetEnabled(ctx context.Context) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, proxy := range m.proxies {
		if proxy.Enabled {
			result = append(result, proxy)
		}
	}
	return result, nil
}

func (m *mockProxyRepo) EnableByUserID(ctx context.Context, userID int64) error {
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			proxy.Enabled = true
		}
	}
	return nil
}

func (m *mockProxyRepo) DisableByUserID(ctx context.Context, userID int64) error {
	for _, proxy := range m.proxies {
		if proxy.UserID == userID {
			proxy.Enabled = false
		}
	}
	return nil
}

func (m *mockProxyRepo) DeleteByIDs(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		delete(m.proxies, id)
	}
	return nil
}

func (m *mockProxyRepo) CountEnabled(ctx context.Context) (int64, error) {
	var count int64
	for _, proxy := range m.proxies {
		if proxy.Enabled {
			count++
		}
	}
	return count, nil
}

func (m *mockProxyRepo) CountByProtocol(ctx context.Context) ([]*repository.ProtocolCount, error) {
	counts := make(map[string]int64)
	for _, proxy := range m.proxies {
		counts[proxy.Protocol]++
	}
	var result []*repository.ProtocolCount
	for protocol, count := range counts {
		result = append(result, &repository.ProtocolCount{Protocol: protocol, Count: count})
	}
	return result, nil
}

func (m *mockProxyRepo) GetByNodeID(ctx context.Context, nodeID int64) ([]*repository.Proxy, error) {
	var result []*repository.Proxy
	for _, proxy := range m.proxies {
		if proxy.NodeID != nil && *proxy.NodeID == nodeID && proxy.Enabled {
			result = append(result, proxy)
		}
	}
	return result, nil
}

// Property 12: Invalid Token Returns 404
// For any token that does not exist in the database, accessing the subscription SHALL return HTTP 404.
// **Validates: Requirements 4.1**
func TestSubscriptionHandlerDetectFormatAliases(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewSubscriptionHandler(nil, logger.NewNopLogger())

	tests := []struct {
		name      string
		format    string
		userAgent string
		expected  subscription.ClientFormat
	}{
		{name: "v2ray alias", format: "v2ray", userAgent: "Clash/1.0", expected: subscription.FormatV2rayN},
		{name: "base64 alias", format: "base64", userAgent: "sing-box/1.0", expected: subscription.FormatV2rayN},
		{name: "raw alias", format: "raw", userAgent: "Surge/5.0", expected: subscription.FormatV2rayN},
		{name: "existing sing-box alias", format: "sing-box", userAgent: "Mozilla/5.0", expected: subscription.FormatSingbox},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			req := httptest.NewRequest(http.MethodGet, "/api/subscription/test-token", nil)
			query := req.URL.Query()
			query.Set("format", tt.format)
			req.URL.RawQuery = query.Encode()
			if tt.userAgent != "" {
				req.Header.Set("User-Agent", tt.userAgent)
			}
			c.Request = req

			got, err := handler.detectFormat(c)
			if err != nil {
				t.Fatalf("detectFormat() returned error: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("detectFormat() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestProperty12_InvalidTokenReturns404(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("invalid token returns 404", prop.ForAll(
		func(token string) bool {
			if len(token) < 8 {
				return true // Skip very short tokens
			}

			// Setup
			subRepo := newMockSubscriptionRepo()
			userRepo := newMockUserRepo()
			proxyRepo := newMockProxyRepo()
			log := logger.NewNopLogger()

			service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
			handler := NewSubscriptionHandler(service, log)

			router := gin.New()
			router.GET("/api/subscription/:token", handler.GetContent)

			req := httptest.NewRequest(http.MethodGet, "/api/subscription/"+token, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusNotFound
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) >= 8 }),
	))

	properties.TestingRun(t)
}

// Property 13: Disabled User Access Denied
// For any user whose account is disabled, accessing their subscription SHALL return HTTP 403.
// **Validates: Requirements 4.2**
func TestProperty13_DisabledUserAccessDenied(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("disabled user returns 403", prop.ForAll(
		func(userID int64) bool {
			if userID <= 0 {
				userID = 1
			}

			// Setup
			subRepo := newMockSubscriptionRepo()
			userRepo := newMockUserRepo()
			proxyRepo := newMockProxyRepo()
			log := logger.NewNopLogger()

			// Create disabled user
			user := &repository.User{
				ID:       userID,
				Username: "testuser",
				Enabled:  false,
			}
			userRepo.users[userID] = user

			// Create subscription for user
			sub := &repository.Subscription{
				ID:        1,
				UserID:    userID,
				Token:     "test-token-12345678901234567890",
				ShortCode: "abc12345",
			}
			subRepo.subscriptions[sub.Token] = sub
			subRepo.byUserID[userID] = sub

			service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
			handler := NewSubscriptionHandler(service, log)

			router := gin.New()
			router.GET("/api/subscription/:token", handler.GetContent)

			req := httptest.NewRequest(http.MethodGet, "http://panel.example.com/api/subscription/"+sub.Token+"?format=clash", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusForbidden
		},
		gen.Int64Range(1, 1000),
	))

	properties.TestingRun(t)
}

// Property 14: Traffic Exceeded Access Denied
// For any user whose traffic usage exceeds their limit, accessing their subscription SHALL return HTTP 403.
// **Validates: Requirements 4.3**
func TestProperty14_TrafficExceededAccessDenied(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("traffic exceeded returns 403", prop.ForAll(
		func(trafficLimit int64) bool {
			if trafficLimit <= 0 {
				return true // Skip no-limit case
			}

			// Setup
			subRepo := newMockSubscriptionRepo()
			userRepo := newMockUserRepo()
			proxyRepo := newMockProxyRepo()
			log := logger.NewNopLogger()

			// Create user with exceeded traffic
			user := &repository.User{
				ID:           1,
				Username:     "testuser",
				Enabled:      true,
				TrafficLimit: trafficLimit,
				TrafficUsed:  trafficLimit + 1, // Exceeded
			}
			userRepo.users[1] = user

			// Create subscription for user
			sub := &repository.Subscription{
				ID:        1,
				UserID:    1,
				Token:     "test-token-12345678901234567890",
				ShortCode: "abc12345",
			}
			subRepo.subscriptions[sub.Token] = sub
			subRepo.byUserID[1] = sub

			service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
			handler := NewSubscriptionHandler(service, log)

			router := gin.New()
			router.GET("/api/subscription/:token", handler.GetContent)

			req := httptest.NewRequest(http.MethodGet, "/api/subscription/"+sub.Token, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusForbidden
		},
		gen.Int64Range(1, 1000000),
	))

	properties.TestingRun(t)
}

// Property 15: Expired User Access Denied
// For any user whose account has expired, accessing their subscription SHALL return HTTP 403.
// **Validates: Requirements 4.4**
func TestProperty15_ExpiredUserAccessDenied(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("expired user returns 403", prop.ForAll(
		func(daysExpired int) bool {
			if daysExpired <= 0 {
				daysExpired = 1
			}

			// Setup
			subRepo := newMockSubscriptionRepo()
			userRepo := newMockUserRepo()
			proxyRepo := newMockProxyRepo()
			log := logger.NewNopLogger()

			// Create expired user
			expiredAt := time.Now().AddDate(0, 0, -daysExpired)
			user := &repository.User{
				ID:        1,
				Username:  "testuser",
				Enabled:   true,
				ExpiresAt: &expiredAt,
			}
			userRepo.users[1] = user

			// Create subscription for user
			sub := &repository.Subscription{
				ID:        1,
				UserID:    1,
				Token:     "test-token-12345678901234567890",
				ShortCode: "abc12345",
			}
			subRepo.subscriptions[sub.Token] = sub
			subRepo.byUserID[1] = sub

			service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
			handler := NewSubscriptionHandler(service, log)

			router := gin.New()
			router.GET("/api/subscription/:token", handler.GetContent)

			req := httptest.NewRequest(http.MethodGet, "/api/subscription/"+sub.Token, nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			return w.Code == http.StatusForbidden
		},
		gen.IntRange(1, 365),
	))

	properties.TestingRun(t)
}

// Property 16: Response Headers Presence
// For any successful subscription content response, it SHALL include required headers.
// **Validates: Requirements 6.1, 6.2, 6.3, 6.7**
func TestProperty16_ResponseHeadersPresence(t *testing.T) {
	// Setup
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	// Create active user
	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
	user := &repository.User{
		ID:           1,
		Username:     "testuser",
		Enabled:      true,
		TrafficLimit: 100 * 1024 * 1024 * 1024,
		TrafficUsed:  25 * 1024 * 1024 * 1024,
		ExpiresAt:    &expiresAt,
	}
	userRepo.users[1] = user

	// Create subscription for user
	sub := &repository.Subscription{
		ID:        1,
		UserID:    1,
		Token:     "test-token-12345678901234567890",
		ShortCode: "abc12345",
	}
	subRepo.subscriptions[sub.Token] = sub
	subRepo.byUserID[1] = sub

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log)

	router := gin.New()
	router.GET("/api/subscription/:token", handler.GetContent)

	req := httptest.NewRequest(http.MethodGet, "http://panel.example.com/api/subscription/"+sub.Token+"?format=clash", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check response headers
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
		return
	}

	requiredHeaders := []string{
		"Content-Disposition",
		"Profile-Update-Interval",
		"Subscription-Userinfo",
		"Profile-Title",
		"Profile-Web-Page-URL",
		"Support-URL",
	}

	for _, header := range requiredHeaders {
		if w.Header().Get(header) == "" {
			t.Errorf("missing required header: %s", header)
		}
	}

	expectedDisposition := "attachment; filename=panel-example-com-clash.yaml"
	if got := w.Header().Get("Content-Disposition"); got != expectedDisposition {
		t.Errorf("unexpected Content-Disposition header: got %q want %q", got, expectedDisposition)
	}

	expectedTitle := "panel.example.com - Clash"
	if got := w.Header().Get("Profile-Title"); got != expectedTitle {
		t.Errorf("unexpected Profile-Title header: got %q want %q", got, expectedTitle)
	}

	expectedProfileWebPageURL := "http://panel.example.com/user/subscription"
	if got := w.Header().Get("Profile-Web-Page-URL"); got != expectedProfileWebPageURL {
		t.Errorf("unexpected Profile-Web-Page-URL header: got %q want %q", got, expectedProfileWebPageURL)
	}

	expectedSupportURL := "http://panel.example.com/user/help"
	if got := w.Header().Get("Support-URL"); got != expectedSupportURL {
		t.Errorf("unexpected Support-URL header: got %q want %q", got, expectedSupportURL)
	}

	expectedUserinfo := "upload=0; download=26843545600; total=107374182400; expire=" + strconv.FormatInt(expiresAt.Unix(), 10)
	if got := w.Header().Get("Subscription-Userinfo"); got != expectedUserinfo {
		t.Errorf("unexpected Subscription-Userinfo header: got %q want %q", got, expectedUserinfo)
	}
}

func TestSubscriptionHandler_ConfiguredProfileUpdateInterval(t *testing.T) {
	// Setup
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	user := &repository.User{
		ID:       1,
		Username: "testuser",
		Enabled:  true,
	}
	userRepo.users[1] = user

	sub := &repository.Subscription{
		ID:        1,
		UserID:    1,
		Token:     "test-token-profile-interval",
		ShortCode: "cfg12345",
	}
	subRepo.subscriptions[sub.Token] = sub
	subRepo.byUserID[1] = sub

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log, 12)

	router := gin.New()
	router.GET("/api/subscription/:token", handler.GetContent)

	req := httptest.NewRequest(http.MethodGet, "/api/subscription/"+sub.Token, nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if got := w.Header().Get("Profile-Update-Interval"); got != "12" {
		t.Fatalf("unexpected Profile-Update-Interval header: got %q want %q", got, "12")
	}
}

// TestGetLink tests the GetLink endpoint
func TestGetLink(t *testing.T) {
	// Setup
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	// Create user
	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
	user := &repository.User{
		ID:           1,
		Username:     "testuser",
		Enabled:      true,
		TrafficLimit: 100 * 1024 * 1024 * 1024,
		TrafficUsed:  25 * 1024 * 1024 * 1024,
		ExpiresAt:    &expiresAt,
	}
	userRepo.users[1] = user

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log)

	router := gin.New()
	router.GET("/api/subscription/link", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.GetLink(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/subscription/link", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
		return
	}

	var response SubscriptionLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
		return
	}

	if response.Token == "" {
		t.Error("expected token to be set")
	}

	if response.Link == "" {
		t.Error("expected link to be set")
	}
}

func TestGetContent_InvalidExplicitFormatReturnsBadRequest(t *testing.T) {
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	expiresAt := time.Now().Add(30 * 24 * time.Hour).UTC()
	userRepo.users[1] = &repository.User{
		ID:           1,
		Username:     "format-user",
		Enabled:      true,
		TrafficLimit: 100,
		TrafficUsed:  0,
		ExpiresAt:    &expiresAt,
	}

	sub := &repository.Subscription{ID: 1, UserID: 1, Token: "token-format-12345678901234567890", ShortCode: "fmt12345"}
	subRepo.subscriptions[sub.Token] = sub
	subRepo.byUserID[1] = sub

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log)

	router := gin.New()
	router.GET("/api/subscription/:token", handler.GetContent)

	req := httptest.NewRequest(http.MethodGet, "/api/subscription/"+sub.Token+"?format=unknown-client", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAdminResetStats_MissingSubscriptionReturnsNotFound(t *testing.T) {
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log)

	router := gin.New()
	router.POST("/api/admin/subscriptions/:user_id/reset-stats", handler.AdminResetStats)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/subscriptions/42/reset-stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if len(subRepo.subscriptions) != 0 {
		t.Fatalf("expected no subscription to be created, got %d", len(subRepo.subscriptions))
	}
}

// TestRegenerate tests the Regenerate endpoint
func TestRegenerate(t *testing.T) {
	// Setup
	subRepo := newMockSubscriptionRepo()
	userRepo := newMockUserRepo()
	proxyRepo := newMockProxyRepo()
	log := logger.NewNopLogger()

	// Create user
	user := &repository.User{
		ID:       1,
		Username: "testuser",
		Enabled:  true,
	}
	userRepo.users[1] = user

	// Create initial subscription
	sub := &repository.Subscription{
		ID:        1,
		UserID:    1,
		Token:     "old-token-12345678901234567890",
		ShortCode: "oldcode1",
	}
	subRepo.subscriptions[sub.Token] = sub
	subRepo.byUserID[1] = sub

	service := subscription.NewService(subRepo, userRepo, proxyRepo, log, "http://localhost:8080")
	handler := NewSubscriptionHandler(service, log)

	router := gin.New()
	router.POST("/api/subscription/regenerate", func(c *gin.Context) {
		c.Set("user_id", int64(1))
		handler.Regenerate(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/subscription/regenerate", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
		return
	}

	var response subscription.SubscriptionInfo
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("failed to unmarshal response: %v", err)
		return
	}

	// Token should be different from old token
	if response.Token == "old-token-12345678901234567890" {
		t.Error("expected token to be regenerated")
	}
}
