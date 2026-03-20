// Package auth provides authentication services for the user portal.
package auth

import (
	"context"
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"v/internal/database/repository"
	"v/pkg/errors"
)

// Service provides authentication operations for the user portal.
type Service struct {
	userRepo      repository.UserRepository
	authTokenRepo repository.AuthTokenRepository
}

// NewService creates a new portal auth service.
func NewService(userRepo repository.UserRepository, authTokenRepo repository.AuthTokenRepository) *Service {
	return &Service{
		userRepo:      userRepo,
		authTokenRepo: authTokenRepo,
	}
}

// Email validation regex (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail validates an email address format.
// Returns true if the email is valid according to RFC 5322 (simplified).
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}
	email = normalizeEmail(email)
	if len(email) > 254 {
		return false
	}
	return emailRegex.MatchString(email)
}

// ValidatePassword validates password strength.
// Password must be at least 8 characters and contain at least one letter and one number.
func ValidatePassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	hasLetter := false
	hasNumber := false

	for _, char := range password {
		if unicode.IsLetter(char) {
			hasLetter = true
		}
		if unicode.IsDigit(char) {
			hasNumber = true
		}
		if hasLetter && hasNumber {
			return true
		}
	}

	return hasLetter && hasNumber
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// RegisterRequest represents a user registration request.
type RegisterRequest struct {
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"password"`
	InviteCode string `json:"invite_code,omitempty"`
}

// Validate validates the registration request.
func (r *RegisterRequest) Validate() []ValidationError {
	var errs []ValidationError

	// Validate username
	r.Username = strings.TrimSpace(r.Username)
	if r.Username == "" {
		errs = append(errs, ValidationError{Field: "username", Message: "用户名不能为空"})
	} else if len(r.Username) < 3 || len(r.Username) > 50 {
		errs = append(errs, ValidationError{Field: "username", Message: "用户名长度必须在3-50个字符之间"})
	}

	// Validate email
	r.Email = normalizeEmail(r.Email)
	if r.Email == "" {
		errs = append(errs, ValidationError{Field: "email", Message: "邮箱不能为空"})
	} else if !ValidateEmail(r.Email) {
		errs = append(errs, ValidationError{Field: "email", Message: "邮箱格式不正确"})
	}

	// Validate password
	if r.Password == "" {
		errs = append(errs, ValidationError{Field: "password", Message: "密码不能为空"})
	} else if !ValidatePassword(r.Password) {
		errs = append(errs, ValidationError{Field: "password", Message: "密码必须至少8个字符，包含字母和数字"})
	}

	return errs
}

// LoginRequest represents a user login request.
type LoginRequest struct {
	Username string `json:"username"` // Can be username or email
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// Validate validates the login request.
func (r *LoginRequest) Validate() []ValidationError {
	var errs []ValidationError

	r.Username = strings.TrimSpace(r.Username)
	if ValidateEmail(r.Username) {
		r.Username = normalizeEmail(r.Username)
	}
	if r.Username == "" {
		errs = append(errs, ValidationError{Field: "username", Message: "用户名或邮箱不能为空"})
	}

	if r.Password == "" {
		errs = append(errs, ValidationError{Field: "password", Message: "密码不能为空"})
	}

	return errs
}

// ChangePasswordRequest represents a password change request.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// Validate validates the change password request.
func (r *ChangePasswordRequest) Validate() []ValidationError {
	var errs []ValidationError

	if r.CurrentPassword == "" {
		errs = append(errs, ValidationError{Field: "current_password", Message: "当前密码不能为空"})
	}

	if r.NewPassword == "" {
		errs = append(errs, ValidationError{Field: "new_password", Message: "新密码不能为空"})
	} else if !ValidatePassword(r.NewPassword) {
		errs = append(errs, ValidationError{Field: "new_password", Message: "新密码必须至少8个字符，包含字母和数字"})
	}

	return errs
}

// ResetPasswordRequest represents a password reset request.
type ResetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

// Validate validates the reset password request.
func (r *ResetPasswordRequest) Validate() []ValidationError {
	var errs []ValidationError

	if r.Token == "" {
		errs = append(errs, ValidationError{Field: "token", Message: "重置令牌不能为空"})
	}

	if r.NewPassword == "" {
		errs = append(errs, ValidationError{Field: "new_password", Message: "新密码不能为空"})
	} else if !ValidatePassword(r.NewPassword) {
		errs = append(errs, ValidationError{Field: "new_password", Message: "新密码必须至少8个字符，包含字母和数字"})
	}

	return errs
}

// CheckUsernameExists checks if a username already exists.
func (s *Service) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	_, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RegisterResult represents the result of a registration.
type RegisterResult struct {
	UserID            int64  `json:"user_id"`
	Username          string `json:"username"`
	Email             string `json:"email"`
	VerificationToken string `json:"-"` // For sending verification email
}

// Register registers a new user.
func (s *Service) Register(ctx context.Context, req *RegisterRequest, inviteRequired bool, hashPassword func(string) (string, error)) (*RegisterResult, error) {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return nil, errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Check if username exists
	exists, err := s.CheckUsernameExists(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.NewConflictError("user", "username", req.Username)
	}

	// Check if email exists
	existingUser, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if existingUser != nil {
		return nil, errors.New(errors.ErrCodeConflict, "该邮箱已被注册")
	}

	// Validate invite code if required
	if inviteRequired && req.InviteCode != "" {
		inviteCode, err := s.authTokenRepo.GetInviteCodeByCode(ctx, req.InviteCode)
		if err != nil {
			if errors.IsNotFound(err) {
				return nil, errors.NewValidationError("invite_code", "邀请码无效")
			}
			return nil, err
		}
		if !inviteCode.IsValid() {
			return nil, errors.NewValidationError("invite_code", "邀请码已过期或已用完")
		}
	} else if inviteRequired {
		return nil, errors.NewValidationError("invite_code", "需要邀请码才能注册")
	}

	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		return nil, errors.NewInternalError("failed to hash password", err)
	}

	// Create user
	user := &repository.User{
		Username:     req.Username,
		PasswordHash: hashedPassword,
		Email:        req.Email,
		Role:         "user",
		Enabled:      true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// Use invite code if provided
	if req.InviteCode != "" {
		if err := s.authTokenRepo.UseInviteCode(ctx, req.InviteCode, user.ID); err != nil {
			// Log error but don't fail registration
		}
	}

	// Generate email verification token
	verificationToken := generateSecureToken(32)

	return &RegisterResult{
		UserID:            user.ID,
		Username:          user.Username,
		Email:             user.Email,
		VerificationToken: verificationToken,
	}, nil
}

// generateSecureToken generates a cryptographically secure random token.
func generateSecureToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// CreateEmailVerificationToken creates an email verification token for a user.
func (s *Service) CreateEmailVerificationToken(ctx context.Context, userID int64, email string) (string, error) {
	token := generateSecureToken(32)
	expiresAt := time.Now().Add(24 * time.Hour) // Token expires in 24 hours

	verificationToken := &repository.EmailVerificationToken{
		UserID:    userID,
		Email:     email,
		Token:     token,
		ExpiresAt: expiresAt,
	}

	if err := s.authTokenRepo.CreateEmailVerificationToken(ctx, verificationToken); err != nil {
		return "", err
	}

	return token, nil
}

// VerifyEmail verifies an email using the verification token.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	// Get the verification token
	verificationToken, err := s.authTokenRepo.GetEmailVerificationTokenByToken(ctx, token)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewValidationError("token", "验证令牌无效")
		}
		return err
	}

	// Check if already verified
	if verificationToken.IsVerified() {
		return errors.NewValidationError("token", "邮箱已验证")
	}

	// Check if expired
	if verificationToken.IsExpired() {
		return errors.NewValidationError("token", "验证令牌已过期")
	}

	// Mark token as verified
	if err := s.authTokenRepo.MarkEmailVerified(ctx, verificationToken.ID); err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, verificationToken.UserID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	user.EmailVerified = true
	user.EmailVerifiedAt = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	return nil
}

// ResendVerificationEmail creates a new verification token for resending.
func (s *Service) ResendVerificationEmail(ctx context.Context, userID int64, email string) (string, error) {
	// Create new verification token
	return s.CreateEmailVerificationToken(ctx, userID, email)
}

// LoginResult represents the result of a login attempt.
type LoginResult struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Requires2FA  bool   `json:"requires_2fa,omitempty"`
}

// RateLimiter provides rate limiting functionality.
type RateLimiter struct {
	attempts map[string]*loginAttempts
	mu       sync.RWMutex
}

type loginAttempts struct {
	count    int
	firstAt  time.Time
	lockedAt *time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string]*loginAttempts),
	}
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	MaxAttempts   int           // Maximum attempts before lockout (default: 5)
	WindowPeriod  time.Duration // Time window for counting attempts (default: 15 minutes)
	LockoutPeriod time.Duration // How long to lock out after max attempts (default: 15 minutes)
}

// DefaultRateLimitConfig returns the default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MaxAttempts:   5,
		WindowPeriod:  15 * time.Minute,
		LockoutPeriod: 15 * time.Minute,
	}
}

// CheckRateLimit checks if the IP is rate limited.
// Returns (allowed, remainingAttempts, error)
func (r *RateLimiter) CheckRateLimit(ip string, config RateLimitConfig) (bool, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	attempts, exists := r.attempts[ip]
	if !exists {
		return true, config.MaxAttempts, nil
	}

	// Check if locked out
	if attempts.lockedAt != nil {
		if time.Since(*attempts.lockedAt) < config.LockoutPeriod {
			return false, 0, nil
		}
		// Lockout expired, reset
		delete(r.attempts, ip)
		return true, config.MaxAttempts, nil
	}

	// Check if window expired
	if time.Since(attempts.firstAt) > config.WindowPeriod {
		delete(r.attempts, ip)
		return true, config.MaxAttempts, nil
	}

	remaining := config.MaxAttempts - attempts.count
	return remaining > 0, remaining, nil
}

// RecordFailedAttempt records a failed login attempt.
func (r *RateLimiter) RecordFailedAttempt(ip string, config RateLimitConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	attempts, exists := r.attempts[ip]
	if !exists {
		r.attempts[ip] = &loginAttempts{
			count:   1,
			firstAt: time.Now(),
		}
		return
	}

	// Check if window expired
	if time.Since(attempts.firstAt) > config.WindowPeriod {
		r.attempts[ip] = &loginAttempts{
			count:   1,
			firstAt: time.Now(),
		}
		return
	}

	attempts.count++
	if attempts.count >= config.MaxAttempts {
		now := time.Now()
		attempts.lockedAt = &now
	}
}

// ResetAttempts resets the attempts for an IP after successful login.
func (r *RateLimiter) ResetAttempts(ip string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, ip)
}

// Login authenticates a user and returns a token.
func (s *Service) Login(ctx context.Context, req *LoginRequest, ip string, rateLimiter *RateLimiter, config RateLimitConfig, verifyPassword func(password, hash string) bool, generateToken func(userID int64, username, role string) (string, error)) (*LoginResult, error) {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return nil, errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Check rate limit
	if rateLimiter != nil {
		allowed, remaining, _ := rateLimiter.CheckRateLimit(ip, config)
		if !allowed {
			return nil, errors.NewRateLimitError("登录尝试次数过多，请稍后再试")
		}
		_ = remaining
	}

	// Find user by username or email
	var user *repository.User
	var err error

	if ValidateEmail(req.Username) {
		req.Username = normalizeEmail(req.Username)
		user, err = s.userRepo.GetByEmail(ctx, req.Username)
	} else {
		req.Username = strings.TrimSpace(req.Username)
		user, err = s.userRepo.GetByUsername(ctx, req.Username)
	}

	if err != nil {
		if errors.IsNotFound(err) {
			if rateLimiter != nil {
				rateLimiter.RecordFailedAttempt(ip, config)
			}
			return nil, errors.NewNotFoundError("user", req.Username)
		}
		return nil, err
	}

	// Check if user is enabled
	if !user.Enabled {
		return nil, errors.NewForbiddenError("账户已被禁用")
	}

	if !user.EmailVerified && strings.TrimSpace(user.Email) != "" {
		pendingTokens, err := s.authTokenRepo.CountPendingEmailVerificationTokensByUser(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		if pendingTokens > 0 {
			return nil, errors.NewForbiddenError("请先验证邮箱后再登录")
		}
	}

	// Verify password
	if !verifyPassword(req.Password, user.PasswordHash) {
		if rateLimiter != nil {
			rateLimiter.RecordFailedAttempt(ip, config)
		}
		return nil, errors.NewUnauthorizedError("用户名或密码错误")
	}

	// Check if 2FA is enabled
	if user.TwoFactorEnabled {
		return &LoginResult{
			UserID:      user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Role:        user.Role,
			Requires2FA: true,
		}, nil
	}

	// Generate token
	token, err := generateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, errors.NewInternalError("failed to generate token", err)
	}

	// Reset rate limit on successful login
	if rateLimiter != nil {
		rateLimiter.ResetAttempts(ip)
	}

	return &LoginResult{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		Token:    token,
	}, nil
}

// TwoFactorRequest represents a 2FA verification request.
type TwoFactorRequest struct {
	UserID int64  `json:"user_id"`
	Code   string `json:"code"`
}

// Validate validates the 2FA request.
func (r *TwoFactorRequest) Validate() []ValidationError {
	var errs []ValidationError

	if r.UserID == 0 {
		errs = append(errs, ValidationError{Field: "user_id", Message: "用户ID不能为空"})
	}

	if r.Code == "" {
		errs = append(errs, ValidationError{Field: "code", Message: "验证码不能为空"})
	} else if len(r.Code) != 6 && len(r.Code) != 8 {
		// 6 digits for TOTP, 8 characters for backup code
		errs = append(errs, ValidationError{Field: "code", Message: "验证码格式不正确"})
	}

	return errs
}

// Verify2FA verifies a 2FA code and completes login.
func (s *Service) Verify2FA(ctx context.Context, req *TwoFactorRequest, verifyTOTP func(secret, code string) bool, generateToken func(userID int64, username, role string) (string, error)) (*LoginResult, error) {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return nil, errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	// Check if 2FA is enabled
	if !user.TwoFactorEnabled {
		return nil, errors.NewValidationError("2fa", "两步验证未启用")
	}

	// Get 2FA secret
	secret, err := s.authTokenRepo.GetTwoFactorSecretByUserID(ctx, req.UserID)
	if err != nil {
		return nil, err
	}

	// Try TOTP verification first (6 digits)
	if len(req.Code) == 6 {
		if !verifyTOTP(secret.Secret, req.Code) {
			return nil, errors.NewUnauthorizedError("验证码错误")
		}
	} else {
		// Try backup code (8 characters)
		valid, err := s.authTokenRepo.VerifyBackupCode(ctx, req.UserID, req.Code)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, errors.NewUnauthorizedError("备份码错误或已使用")
		}
	}

	// Generate token
	token, err := generateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, errors.NewInternalError("failed to generate token", err)
	}

	return &LoginResult{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		Token:    token,
	}, nil
}

// Setup2FA generates a new 2FA secret for a user.
func (s *Service) Setup2FA(ctx context.Context, userID int64, generateSecret func() (string, error)) (string, []string, error) {
	// Generate TOTP secret
	secret, err := generateSecret()
	if err != nil {
		return "", nil, errors.NewInternalError("failed to generate 2FA secret", err)
	}

	// Generate backup codes
	backupCodes := make([]string, 10)
	for i := range backupCodes {
		backupCodes[i] = generateSecureToken(8)
	}

	// Store 2FA secret
	twoFactorSecret := &repository.TwoFactorSecret{
		UserID:      userID,
		Secret:      secret,
		BackupCodes: strings.Join(backupCodes, ","),
		Enabled:     false, // Not enabled until verified
	}

	if err := s.authTokenRepo.CreateTwoFactorSecret(ctx, twoFactorSecret); err != nil {
		return "", nil, err
	}

	return secret, backupCodes, nil
}

// Enable2FA enables 2FA for a user after verification.
func (s *Service) Enable2FA(ctx context.Context, userID int64, code string, verifyTOTP func(secret, code string) bool) error {
	// Get 2FA secret
	secret, err := s.authTokenRepo.GetTwoFactorSecretByUserID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify the code
	if !verifyTOTP(secret.Secret, code) {
		return errors.NewUnauthorizedError("验证码错误")
	}

	// Enable 2FA
	if err := s.authTokenRepo.EnableTwoFactor(ctx, userID); err != nil {
		return err
	}

	return nil
}

// Disable2FA disables 2FA for a user.
func (s *Service) Disable2FA(ctx context.Context, userID int64, password string, verifyPassword func(password, hash string) bool) error {
	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify password
	if !verifyPassword(password, user.PasswordHash) {
		return errors.NewUnauthorizedError("密码错误")
	}

	// Delete 2FA secret
	if err := s.authTokenRepo.DeleteTwoFactorSecret(ctx, userID); err != nil {
		return err
	}

	return nil
}

// RequestPasswordResetRequest represents a password reset request.
type RequestPasswordResetRequest struct {
	Email string `json:"email"`
}

// Validate validates the request.
func (r *RequestPasswordResetRequest) Validate() []ValidationError {
	var errs []ValidationError

	r.Email = normalizeEmail(r.Email)
	if r.Email == "" {
		errs = append(errs, ValidationError{Field: "email", Message: "邮箱不能为空"})
	} else if !ValidateEmail(r.Email) {
		errs = append(errs, ValidationError{Field: "email", Message: "邮箱格式不正确"})
	}

	return errs
}

// RequestPasswordReset creates a password reset token and returns it.
// The caller is responsible for sending the reset email.
func (s *Service) RequestPasswordReset(ctx context.Context, req *RequestPasswordResetRequest) (string, error) {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return "", errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Find user by email
	user, err := s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		if errors.IsNotFound(err) {
			// Don't reveal if email exists - return success anyway
			// but don't create a token
			return "", nil
		}
		return "", err
	}

	// Check rate limit - max 3 reset requests per hour
	count, err := s.authTokenRepo.CountPasswordResetTokensByUser(ctx, user.ID, time.Now().Add(-1*time.Hour))
	if err != nil {
		return "", err
	}
	if count >= 3 {
		return "", errors.NewRateLimitError("密码重置请求过于频繁，请稍后再试")
	}

	// Generate reset token
	token := generateSecureToken(32)
	expiresAt := time.Now().Add(1 * time.Hour) // Token expires in 1 hour

	resetToken := &repository.PasswordResetToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
	}

	if err := s.authTokenRepo.CreatePasswordResetToken(ctx, resetToken); err != nil {
		return "", err
	}

	return token, nil
}

// ExecutePasswordReset resets the password using a reset token.
func (s *Service) ExecutePasswordReset(ctx context.Context, req *ResetPasswordRequest, hashPassword func(string) (string, error)) error {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Get the reset token
	resetToken, err := s.authTokenRepo.GetPasswordResetTokenByToken(ctx, req.Token)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewValidationError("token", "重置令牌无效")
		}
		return err
	}

	// Check if already used
	if resetToken.IsUsed() {
		return errors.NewValidationError("token", "重置令牌已使用")
	}

	// Check if expired
	if resetToken.IsExpired() {
		return errors.NewValidationError("token", "重置令牌已过期")
	}

	// Hash new password
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		return errors.NewInternalError("failed to hash password", err)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, resetToken.UserID)
	if err != nil {
		return err
	}

	// Update password
	user.PasswordHash = hashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	// Mark token as used
	if err := s.authTokenRepo.MarkPasswordResetTokenUsed(ctx, resetToken.ID); err != nil {
		return err
	}

	// Note: Session invalidation would be handled by the caller
	// by invalidating all JWT tokens for this user

	return nil
}

// ChangePassword changes the password for a logged-in user.
func (s *Service) ChangePassword(ctx context.Context, userID int64, req *ChangePasswordRequest, verifyPassword func(password, hash string) bool, hashPassword func(string) (string, error)) error {
	// Validate request
	if errs := req.Validate(); len(errs) > 0 {
		return errors.NewValidationError("validation failed", errs[0].Message)
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify current password
	if !verifyPassword(req.CurrentPassword, user.PasswordHash) {
		return errors.NewUnauthorizedError("当前密码错误")
	}

	// Hash new password
	hashedPassword, err := hashPassword(req.NewPassword)
	if err != nil {
		return errors.NewInternalError("failed to hash password", err)
	}

	// Update password
	user.PasswordHash = hashedPassword
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	return nil
}
