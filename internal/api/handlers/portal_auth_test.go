package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"v/internal/auth"
	"v/internal/database/repository"
	"v/internal/logger"
	portalauth "v/internal/portal/auth"
	"v/pkg/errors"
)

// portalMockUserRepo is a mock implementation of UserRepository for portal testing.
type portalMockUserRepo struct {
	users      map[int64]*repository.User
	byUsername map[string]*repository.User
	byEmail    map[string]*repository.User
	nextID     int64
}

func newPortalMockUserRepo() *portalMockUserRepo {
	return &portalMockUserRepo{
		users:      make(map[int64]*repository.User),
		byUsername: make(map[string]*repository.User),
		byEmail:    make(map[string]*repository.User),
		nextID:     1,
	}
}

func (m *portalMockUserRepo) Create(ctx context.Context, user *repository.User) error {
	user.ID = m.nextID
	m.nextID++
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	m.byUsername[user.Username] = user
	if user.Email != "" {
		m.byEmail[strings.ToLower(user.Email)] = user
	}
	return nil
}

func (m *portalMockUserRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	if user, ok := m.users[id]; ok {
		return user, nil
	}
	return nil, errors.NewNotFoundError("user", id)
}

func (m *portalMockUserRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	if user, ok := m.byUsername[username]; ok {
		return user, nil
	}
	return nil, errors.NewNotFoundError("user", username)
}

func (m *portalMockUserRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	if user, ok := m.byEmail[strings.ToLower(strings.TrimSpace(email))]; ok {
		return user, nil
	}
	return nil, errors.NewNotFoundError("user", email)
}

func (m *portalMockUserRepo) Update(ctx context.Context, user *repository.User) error {
	user.UpdatedAt = time.Now()
	m.users[user.ID] = user
	m.byUsername[user.Username] = user
	if user.Email != "" {
		m.byEmail[strings.ToLower(user.Email)] = user
	}
	return nil
}

func (m *portalMockUserRepo) Delete(ctx context.Context, id int64) error {
	if user, ok := m.users[id]; ok {
		delete(m.byUsername, user.Username)
		delete(m.byEmail, strings.ToLower(user.Email))
		delete(m.users, id)
	}
	return nil
}

func (m *portalMockUserRepo) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	users := make([]*repository.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, u)
	}
	return users, nil
}

func (m *portalMockUserRepo) Count(ctx context.Context) (int64, error) {
	return int64(len(m.users)), nil
}

func (m *portalMockUserRepo) CountActive(ctx context.Context) (int64, error) {
	count := int64(0)
	for _, u := range m.users {
		if u.Enabled {
			count++
		}
	}
	return count, nil
}

// portalMockAuthTokenRepo is a mock implementation of AuthTokenRepository.
type portalMockAuthTokenRepo struct {
	passwordResetTokens     map[string]*repository.PasswordResetToken
	emailVerificationTokens map[string]*repository.EmailVerificationToken
	inviteCodes             map[string]*repository.InviteCode
	twoFactorSecrets        map[int64]*repository.TwoFactorSecret
}

func newPortalMockAuthTokenRepo() *portalMockAuthTokenRepo {
	return &portalMockAuthTokenRepo{
		passwordResetTokens:     make(map[string]*repository.PasswordResetToken),
		emailVerificationTokens: make(map[string]*repository.EmailVerificationToken),
		inviteCodes:             make(map[string]*repository.InviteCode),
		twoFactorSecrets:        make(map[int64]*repository.TwoFactorSecret),
	}
}

func (m *portalMockAuthTokenRepo) CreatePasswordResetToken(ctx context.Context, token *repository.PasswordResetToken) error {
	m.passwordResetTokens[token.Token] = token
	return nil
}

func (m *portalMockAuthTokenRepo) GetPasswordResetTokenByToken(ctx context.Context, token string) (*repository.PasswordResetToken, error) {
	if t, ok := m.passwordResetTokens[token]; ok {
		return t, nil
	}
	return nil, errors.NewNotFoundError("password_reset_token", token)
}

func (m *portalMockAuthTokenRepo) MarkPasswordResetTokenUsed(ctx context.Context, id int64) error {
	for _, t := range m.passwordResetTokens {
		if t.ID == id {
			now := time.Now()
			t.UsedAt = &now
		}
	}
	return nil
}

func (m *portalMockAuthTokenRepo) DeleteExpiredPasswordResetTokens(ctx context.Context) (int64, error) {
	count := int64(0)
	for token, t := range m.passwordResetTokens {
		if t.IsExpired() {
			delete(m.passwordResetTokens, token)
			count++
		}
	}
	return count, nil
}

func (m *portalMockAuthTokenRepo) CountPasswordResetTokensByUser(ctx context.Context, userID int64, since time.Time) (int64, error) {
	count := int64(0)
	for _, t := range m.passwordResetTokens {
		if t.UserID == userID && t.CreatedAt.After(since) {
			count++
		}
	}
	return count, nil
}

func (m *portalMockAuthTokenRepo) CreateEmailVerificationToken(ctx context.Context, token *repository.EmailVerificationToken) error {
	m.emailVerificationTokens[token.Token] = token
	return nil
}

func (m *portalMockAuthTokenRepo) GetEmailVerificationTokenByToken(ctx context.Context, token string) (*repository.EmailVerificationToken, error) {
	if t, ok := m.emailVerificationTokens[token]; ok {
		return t, nil
	}
	return nil, errors.NewNotFoundError("email_verification_token", token)
}

func (m *portalMockAuthTokenRepo) MarkEmailVerified(ctx context.Context, id int64) error {
	for _, t := range m.emailVerificationTokens {
		if t.ID == id {
			now := time.Now()
			t.VerifiedAt = &now
		}
	}
	return nil
}

func (m *portalMockAuthTokenRepo) CountPendingEmailVerificationTokensByUser(ctx context.Context, userID int64) (int64, error) {
	count := int64(0)
	for _, t := range m.emailVerificationTokens {
		if t.UserID == userID && !t.IsVerified() && !t.IsExpired() {
			count++
		}
	}
	return count, nil
}

func (m *portalMockAuthTokenRepo) DeleteExpiredEmailVerificationTokens(ctx context.Context) (int64, error) {
	count := int64(0)
	for token, t := range m.emailVerificationTokens {
		if t.IsExpired() {
			delete(m.emailVerificationTokens, token)
			count++
		}
	}
	return count, nil
}

func (m *portalMockAuthTokenRepo) CreateInviteCode(ctx context.Context, code *repository.InviteCode) error {
	m.inviteCodes[code.Code] = code
	return nil
}

func (m *portalMockAuthTokenRepo) GetInviteCodeByCode(ctx context.Context, code string) (*repository.InviteCode, error) {
	if c, ok := m.inviteCodes[code]; ok {
		return c, nil
	}
	return nil, errors.NewNotFoundError("invite_code", code)
}

func (m *portalMockAuthTokenRepo) UseInviteCode(ctx context.Context, code string, userID int64) error {
	if c, ok := m.inviteCodes[code]; ok {
		c.UsedCount++
		c.UsedBy = &userID
		now := time.Now()
		c.UsedAt = &now
	}
	return nil
}

func (m *portalMockAuthTokenRepo) ListInviteCodes(ctx context.Context, limit, offset int) ([]*repository.InviteCode, int64, error) {
	codes := make([]*repository.InviteCode, 0, len(m.inviteCodes))
	for _, c := range m.inviteCodes {
		codes = append(codes, c)
	}
	return codes, int64(len(codes)), nil
}

func (m *portalMockAuthTokenRepo) DeleteInviteCode(ctx context.Context, id int64) error {
	for code, c := range m.inviteCodes {
		if c.ID == id {
			delete(m.inviteCodes, code)
			return nil
		}
	}
	return errors.NewNotFoundError("invite_code", id)
}

func (m *portalMockAuthTokenRepo) CreateTwoFactorSecret(ctx context.Context, secret *repository.TwoFactorSecret) error {
	m.twoFactorSecrets[secret.UserID] = secret
	return nil
}

func (m *portalMockAuthTokenRepo) GetTwoFactorSecretByUserID(ctx context.Context, userID int64) (*repository.TwoFactorSecret, error) {
	if s, ok := m.twoFactorSecrets[userID]; ok {
		return s, nil
	}
	return nil, errors.NewNotFoundError("two_factor_secret", userID)
}

func (m *portalMockAuthTokenRepo) UpdateTwoFactorSecret(ctx context.Context, secret *repository.TwoFactorSecret) error {
	m.twoFactorSecrets[secret.UserID] = secret
	return nil
}

func (m *portalMockAuthTokenRepo) EnableTwoFactor(ctx context.Context, userID int64) error {
	if s, ok := m.twoFactorSecrets[userID]; ok {
		s.Enabled = true
		now := time.Now()
		s.EnabledAt = &now
	}
	return nil
}

func (m *portalMockAuthTokenRepo) DeleteTwoFactorSecret(ctx context.Context, userID int64) error {
	delete(m.twoFactorSecrets, userID)
	return nil
}

func (m *portalMockAuthTokenRepo) VerifyBackupCode(ctx context.Context, userID int64, code string) (bool, error) {
	return false, nil
}

func setupPortalTestRouter() (*gin.Engine, *PortalAuthHandler, *portalMockUserRepo) {
	gin.SetMode(gin.TestMode)

	userRepo := newPortalMockUserRepo()
	authTokenRepo := newPortalMockAuthTokenRepo()

	authService := auth.NewService(auth.Config{
		JWTSecret:          "test-secret-key-for-testing",
		TokenExpiry:        time.Hour,
		RefreshTokenExpiry: 24 * time.Hour,
	})

	portalAuthService := portalauth.NewService(userRepo, authTokenRepo)

	proxyRepo := &portalMockProxyRepo{}
	handler := NewPortalAuthHandler(portalAuthService, authService, userRepo, proxyRepo, &portalMockLogger{})

	router := gin.New()

	// Setup routes
	portal := router.Group("/api/portal")
	{
		auth := portal.Group("/auth")
		{
			auth.POST("/register", handler.Register)
			auth.POST("/login", handler.Login)
			auth.POST("/2fa/login", handler.Verify2FALogin)
			auth.POST("/forgot-password", handler.ForgotPassword)
			auth.POST("/reset-password", handler.ResetPassword)
			auth.GET("/verify-email", handler.VerifyEmail)
		}
	}

	return router, handler, userRepo
}

type portalMockProxyRepo struct{}

func (m *portalMockProxyRepo) Create(ctx context.Context, proxy *repository.Proxy) error { return nil }
func (m *portalMockProxyRepo) GetByID(ctx context.Context, id int64) (*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) Update(ctx context.Context, proxy *repository.Proxy) error { return nil }
func (m *portalMockProxyRepo) Delete(ctx context.Context, id int64) error                { return nil }
func (m *portalMockProxyRepo) List(ctx context.Context, limit, offset int) ([]*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) GetByProtocol(ctx context.Context, protocol string) ([]*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) GetEnabled(ctx context.Context) ([]*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) GetByUserID(ctx context.Context, userID int64, limit, offset int) ([]*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) CountByUserID(ctx context.Context, userID int64) (int64, error) {
	return 0, nil
}
func (m *portalMockProxyRepo) GetByPort(ctx context.Context, port int) (*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) GetByNodeID(ctx context.Context, nodeID int64) ([]*repository.Proxy, error) {
	return nil, nil
}
func (m *portalMockProxyRepo) EnableByUserID(ctx context.Context, userID int64) error  { return nil }
func (m *portalMockProxyRepo) DisableByUserID(ctx context.Context, userID int64) error { return nil }
func (m *portalMockProxyRepo) DeleteByIDs(ctx context.Context, ids []int64) error      { return nil }
func (m *portalMockProxyRepo) Count(ctx context.Context) (int64, error)                { return 0, nil }
func (m *portalMockProxyRepo) CountEnabled(ctx context.Context) (int64, error)         { return 0, nil }
func (m *portalMockProxyRepo) CountByProtocol(ctx context.Context) ([]*repository.ProtocolCount, error) {
	return nil, nil
}

type portalMockLogger struct{}

func (m *portalMockLogger) Debug(msg string, fields ...logger.Field)  {}
func (m *portalMockLogger) Info(msg string, fields ...logger.Field)   {}
func (m *portalMockLogger) Warn(msg string, fields ...logger.Field)   {}
func (m *portalMockLogger) Error(msg string, fields ...logger.Field)  {}
func (m *portalMockLogger) Fatal(msg string, fields ...logger.Field)  {}
func (m *portalMockLogger) With(fields ...logger.Field) logger.Logger { return m }
func (m *portalMockLogger) SetLevel(level logger.Level)               {}
func (m *portalMockLogger) GetLevel() logger.Level                    { return logger.InfoLevel }

type portalMockEmailSender struct {
	sent []portalSentEmail
}

type portalSentEmail struct {
	to      string
	subject string
	body    string
}

func (m *portalMockEmailSender) CanSendEmail() bool { return true }

func (m *portalMockEmailSender) SendEmail(to, subject, body string) error {
	m.sent = append(m.sent, portalSentEmail{
		to:      to,
		subject: subject,
		body:    body,
	})
	return nil
}

func TestPortalAuthHandler_Register(t *testing.T) {
	router, _, userRepo := setupPortalTestRouter()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "successful registration",
			body: map[string]interface{}{
				"username": "testuser",
				"email":    "test@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusCreated,
		},
		{
			name: "missing username",
			body: map[string]interface{}{
				"email":    "test@example.com",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "invalid email",
			body: map[string]interface{}{
				"username": "testuser2",
				"email":    "invalid-email",
				"password": "password123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "weak password",
			body: map[string]interface{}{
				"username": "testuser3",
				"email":    "test3@example.com",
				"password": "weak",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Register() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}

	// Verify user was created
	if len(userRepo.users) != 1 {
		t.Errorf("Expected 1 user to be created, got %d", len(userRepo.users))
	}
}

func TestPortalAuthHandler_RegisterUsesRequestHostForVerificationEmailWhenBaseURLIsLocal(t *testing.T) {
	router, handler, _ := setupPortalTestRouter()
	emailSender := &portalMockEmailSender{}
	handler.WithEmailSender(emailSender, "http://localhost:8080")

	body := map[string]interface{}{
		"username": "verifyuser",
		"email":    "verify@example.com",
		"password": "password123",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(jsonBody))
	req.Host = "shcrystal.top:13212"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-Proto", "http")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", w.Code)
	}

	if len(emailSender.sent) != 1 {
		t.Fatalf("expected one verification email, got %d", len(emailSender.sent))
	}

	if !strings.Contains(emailSender.sent[0].body, "http://shcrystal.top:13212/user/login?verify_email_token=") {
		t.Fatalf("expected verification link to use request host, got %q", emailSender.sent[0].body)
	}
}

func TestPortalAuthHandler_Login(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
		wantBody   string
	}{
		{
			name: "successful login",
			body: map[string]interface{}{
				"username": "testuser",
				"password": "password123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "wrong password",
			body: map[string]interface{}{
				"username": "testuser",
				"password": "wrongpassword",
			},
			wantStatus: http.StatusUnauthorized,
			wantBody:   "密码错误，请重新输入",
		},
		{
			name: "non-existent user",
			body: map[string]interface{}{
				"username": "nonexistent",
				"password": "password123",
			},
			wantStatus: http.StatusNotFound,
			wantBody:   "账号不存在，请检查邮箱/用户名是否正确",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Login() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}

			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("Login() body = %q, want to contain %q", w.Body.String(), tt.wantBody)
			}

			// Verify token is returned on successful login
			if tt.wantStatus == http.StatusOK {
				var response map[string]interface{}
				json.Unmarshal(w.Body.Bytes(), &response)
				if _, ok := response["token"]; !ok {
					t.Error("Expected token in response")
				}
			}
		})
	}
}

func TestPortalAuthHandler_ForgotPassword(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "existing email",
			body: map[string]interface{}{
				"email": "test@example.com",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "non-existent email - should still return OK for security",
			body: map[string]interface{}{
				"email": "nonexistent@example.com",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid email format",
			body: map[string]interface{}{
				"email": "invalid-email",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/forgot-password", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ForgotPassword() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

func TestPortalAuthHandler_DuplicateRegistration(t *testing.T) {
	router, _, _ := setupPortalTestRouter()

	// First registration
	body1, _ := json.Marshal(map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
		"password": "password123",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("First registration failed: %s", w1.Body.String())
	}

	// Duplicate registration with same username
	body2, _ := json.Marshal(map[string]interface{}{
		"username": "testuser",
		"email":    "different@example.com",
		"password": "password123",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Expected conflict for duplicate username, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestPortalAuthHandler_DuplicateEmailRegistration(t *testing.T) {
	router, _, _ := setupPortalTestRouter()

	body1, _ := json.Marshal(map[string]interface{}{
		"username": "emailowner",
		"email":    "owner@example.com",
		"password": "password123",
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("First registration failed: %s", w1.Body.String())
	}

	body2, _ := json.Marshal(map[string]interface{}{
		"username": "anotheruser",
		"email":    "Owner@example.com",
		"password": "password123",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("Expected conflict for duplicate email, got %d: %s", w2.Code, w2.Body.String())
	}

	if !strings.Contains(w2.Body.String(), "该邮箱已被注册") {
		t.Errorf("Expected duplicate email message, got %s", w2.Body.String())
	}
}

// TestPortalAuthHandler_ResetPassword tests the complete password reset flow.
// Validates: Requirements 3.1-3.6
func TestPortalAuthHandler_ResetPassword(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("oldpassword123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "resetuser",
		Email:        "reset@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	// Create a valid reset token
	resetToken := &repository.PasswordResetToken{
		ID:        1,
		UserID:    1,
		Token:     "valid-reset-token-12345678901234567890",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
	authTokenRepo.passwordResetTokens[resetToken.Token] = resetToken

	// Update the handler's service to use our mock auth token repo
	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "successful password reset",
			body: map[string]interface{}{
				"token":        "valid-reset-token-12345678901234567890",
				"new_password": "newpassword123",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid token",
			body: map[string]interface{}{
				"token":        "invalid-token",
				"new_password": "newpassword123",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "weak new password",
			body: map[string]interface{}{
				"token":        "valid-reset-token-12345678901234567890",
				"new_password": "weak",
			},
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "missing token",
			body: map[string]interface{}{
				"new_password": "newpassword123",
			},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/reset-password", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("ResetPassword() status = %v, want %v, body = %s", w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestPortalAuthHandler_ResetPasswordTokenExpired tests that expired tokens are rejected.
// Validates: Requirements 3.2
func TestPortalAuthHandler_ResetPasswordTokenExpired(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("oldpassword123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "expireduser",
		Email:        "expired@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	// Create an expired reset token
	expiredToken := &repository.PasswordResetToken{
		ID:        1,
		UserID:    1,
		Token:     "expired-reset-token-1234567890123456",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	authTokenRepo.passwordResetTokens[expiredToken.Token] = expiredToken

	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	body, _ := json.Marshal(map[string]interface{}{
		"token":        "expired-reset-token-1234567890123456",
		"new_password": "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/reset-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest for expired token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPortalAuthHandler_ResetPasswordTokenSingleUse tests that tokens can only be used once.
// Validates: Requirements 3.3
func TestPortalAuthHandler_ResetPasswordTokenSingleUse(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("oldpassword123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "singleuseuser",
		Email:        "singleuse@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	// Create a used reset token
	usedAt := time.Now().Add(-30 * time.Minute)
	usedToken := &repository.PasswordResetToken{
		ID:        1,
		UserID:    1,
		Token:     "used-reset-token-12345678901234567890",
		ExpiresAt: time.Now().Add(30 * time.Minute),
		UsedAt:    &usedAt,
		CreatedAt: time.Now().Add(-1 * time.Hour),
	}
	authTokenRepo.passwordResetTokens[usedToken.Token] = usedToken

	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	body, _ := json.Marshal(map[string]interface{}{
		"token":        "used-reset-token-12345678901234567890",
		"new_password": "newpassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/reset-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest for used token, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPortalAuthHandler_LoginDisabledAccount tests that disabled accounts cannot login.
// Validates: Requirements 2.2, 2.3
func TestPortalAuthHandler_LoginDisabledAccount(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()

	// Create a disabled user
	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "disableduser",
		Email:        "disabled@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      false, // Account is disabled
	})

	body, _ := json.Marshal(map[string]interface{}{
		"username": "disableduser",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected Forbidden for disabled account, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPortalAuthHandler_LoginWithEmail tests login using email instead of username.
// Validates: Requirements 2.1
func TestPortalAuthHandler_LoginWithEmail(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()

	// Create a test user
	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "emailuser",
		Email:        "emaillogin@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"username": "emaillogin@example.com", // Using email as username
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected OK for email login, got %d: %s", w.Code, w.Body.String())
	}

	// Verify token is returned
	var response map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &response)
	if _, ok := response["token"]; !ok {
		t.Error("Expected token in response for email login")
	}
}

func TestPortalAuthHandler_LoginWithEmailIsCaseInsensitive(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()

	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "mixedemailuser",
		Email:        "justinshuai36@gmail.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"username": "Justinshuai36@gmail.com",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected OK for case-insensitive email login, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPortalAuthHandler_LoginRequiresVerifiedEmailWhenPendingTokenExists(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	hashedPassword, _ := handler.authService.HashPassword("password123")
	user := &repository.User{
		Username:        "pendingverifyuser",
		Email:           "pending@example.com",
		PasswordHash:    hashedPassword,
		Role:            "user",
		Enabled:         true,
		EmailVerified:   false,
		EmailVerifiedAt: nil,
	}
	userRepo.Create(context.Background(), user)

	authTokenRepo.emailVerificationTokens["pending-token"] = &repository.EmailVerificationToken{
		ID:        1,
		UserID:    user.ID,
		Email:     user.Email,
		Token:     "pending-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	body, _ := json.Marshal(map[string]interface{}{
		"username": "pendingverifyuser",
		"password": "password123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected Forbidden for unverified email, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "请先验证邮箱后再登录") {
		t.Fatalf("Expected email verification message, got %s", w.Body.String())
	}
}

func TestPortalAuthHandler_VerifyEmailUpdatesUserStatus(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	hashedPassword, _ := handler.authService.HashPassword("password123")
	user := &repository.User{
		Username:      "verifyuser",
		Email:         "verify@example.com",
		PasswordHash:  hashedPassword,
		Role:          "user",
		Enabled:       true,
		EmailVerified: false,
	}
	userRepo.Create(context.Background(), user)

	authTokenRepo.emailVerificationTokens["verify-token"] = &repository.EmailVerificationToken{
		ID:        1,
		UserID:    user.ID,
		Email:     user.Email,
		Token:     "verify-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/portal/auth/verify-email?token=verify-token", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected OK for verify email, got %d: %s", w.Code, w.Body.String())
	}

	updatedUser, err := userRepo.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("Expected user lookup to succeed: %v", err)
	}
	if !updatedUser.EmailVerified {
		t.Fatalf("Expected user email to be marked verified")
	}
	if updatedUser.EmailVerifiedAt == nil {
		t.Fatalf("Expected user email verification timestamp to be set")
	}
}

func TestPortalAuthHandler_Verify2FALoginRejectsInvalidCode(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	authTokenRepo := newPortalMockAuthTokenRepo()

	hashedPassword, _ := handler.authService.HashPassword("password123")
	user := &repository.User{
		Username:         "twofauser",
		Email:            "twofa@example.com",
		PasswordHash:     hashedPassword,
		Role:             "user",
		Enabled:          true,
		TwoFactorEnabled: true,
	}
	userRepo.Create(context.Background(), user)

	authTokenRepo.twoFactorSecrets[user.ID] = &repository.TwoFactorSecret{
		UserID:  user.ID,
		Secret:  "JBSWY3DPEHPK3PXP",
		Enabled: true,
	}
	handler.portalAuthService = portalauth.NewService(userRepo, authTokenRepo)

	body, _ := json.Marshal(map[string]interface{}{
		"user_id": user.ID,
		"code":    "ABCDEF",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/2fa/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for invalid 2FA code, got %d: %s", w.Code, w.Body.String())
	}

	if !strings.Contains(w.Body.String(), "验证码错误") {
		t.Fatalf("expected invalid 2FA code message, got %s", w.Body.String())
	}
}

func TestPortalAuthHandler_ForgotPasswordSendsToNormalizedEmail(t *testing.T) {
	router, handler, userRepo := setupPortalTestRouter()
	emailSender := &portalMockEmailSender{}
	handler.WithEmailSender(emailSender, "http://shcrystal.top:13212")

	hashedPassword, _ := handler.authService.HashPassword("password123")
	userRepo.Create(context.Background(), &repository.User{
		Username:     "resetmailuser",
		Email:        "test@example.com",
		PasswordHash: hashedPassword,
		Role:         "user",
		Enabled:      true,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"email": "  TEST@example.com  ",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/forgot-password", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected OK for forgot password, got %d: %s", w.Code, w.Body.String())
	}

	if len(emailSender.sent) != 1 {
		t.Fatalf("expected one reset email, got %d", len(emailSender.sent))
	}

	if got := emailSender.sent[0].to; got != "test@example.com" {
		t.Fatalf("expected normalized recipient email, got %q", got)
	}
}

// TestPortalAuthHandler_RegistrationFlow tests the complete registration flow.
// Validates: Requirements 1.1-1.10
func TestPortalAuthHandler_RegistrationFlow(t *testing.T) {
	router, _, userRepo := setupPortalTestRouter()

	// Step 1: Register a new user
	registerBody, _ := json.Marshal(map[string]interface{}{
		"username": "flowuser",
		"email":    "flow@example.com",
		"password": "flowpassword123",
	})
	registerReq := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(registerBody))
	registerReq.Header.Set("Content-Type", "application/json")

	registerW := httptest.NewRecorder()
	router.ServeHTTP(registerW, registerReq)

	if registerW.Code != http.StatusCreated {
		t.Fatalf("Registration failed: %s", registerW.Body.String())
	}

	// Verify response contains user info
	var registerResponse map[string]interface{}
	json.Unmarshal(registerW.Body.Bytes(), &registerResponse)

	if registerResponse["message"] != "注册成功" {
		t.Errorf("Expected success message, got %v", registerResponse["message"])
	}

	userInfo, ok := registerResponse["user"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected user info in response")
	}

	if userInfo["username"] != "flowuser" {
		t.Errorf("Expected username 'flowuser', got %v", userInfo["username"])
	}

	// Step 2: Verify user was created in repository
	if len(userRepo.users) != 1 {
		t.Errorf("Expected 1 user in repository, got %d", len(userRepo.users))
	}

	// Step 3: Login with the new user
	loginBody, _ := json.Marshal(map[string]interface{}{
		"username": "flowuser",
		"password": "flowpassword123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/portal/auth/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginW := httptest.NewRecorder()
	router.ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusOK {
		t.Fatalf("Login failed after registration: %s", loginW.Body.String())
	}

	// Verify token is returned
	var loginResponse map[string]interface{}
	json.Unmarshal(loginW.Body.Bytes(), &loginResponse)

	if _, ok := loginResponse["token"]; !ok {
		t.Error("Expected token in login response")
	}
}

// TestPortalAuthHandler_PasswordValidation tests various password validation scenarios.
// Validates: Requirements 1.3, 3.4
func TestPortalAuthHandler_PasswordValidation(t *testing.T) {
	router, _, _ := setupPortalTestRouter()

	tests := []struct {
		name       string
		password   string
		wantStatus int
	}{
		{
			name:       "valid password with letters and numbers",
			password:   "password123",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "too short",
			password:   "pass1",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "only letters",
			password:   "passwordonly",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "only numbers",
			password:   "12345678",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid with special characters",
			password:   "pass@word123!",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "exactly 8 characters valid",
			password:   "pass1234",
			wantStatus: http.StatusCreated,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"username": "pwduser" + string(rune('a'+i)),
				"email":    "pwd" + string(rune('a'+i)) + "@example.com",
				"password": tt.password,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Password validation for %q: status = %v, want %v, body = %s", tt.password, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}

// TestPortalAuthHandler_EmailValidation tests various email validation scenarios.
// Validates: Requirements 1.2
func TestPortalAuthHandler_EmailValidation(t *testing.T) {
	router, _, _ := setupPortalTestRouter()

	tests := []struct {
		name       string
		email      string
		wantStatus int
	}{
		{
			name:       "valid email",
			email:      "valid@example.com",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid email with subdomain",
			email:      "user@mail.example.com",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid email with plus",
			email:      "user+tag@example.com",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid - no @",
			email:      "invalidemail.com",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid - no domain",
			email:      "invalid@",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid - no local part",
			email:      "@example.com",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid - spaces",
			email:      "invalid email@example.com",
			wantStatus: http.StatusBadRequest,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]interface{}{
				"username": "emailuser" + string(rune('a'+i)),
				"email":    tt.email,
				"password": "password123",
			})
			req := httptest.NewRequest(http.MethodPost, "/api/portal/auth/register", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Email validation for %q: status = %v, want %v, body = %s", tt.email, w.Code, tt.wantStatus, w.Body.String())
			}
		})
	}
}
