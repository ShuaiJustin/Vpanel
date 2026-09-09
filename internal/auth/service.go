// Package auth provides authentication and authorization services.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"v/internal/database/repository"
	"v/pkg/errors"
)

// Claims represents JWT claims.
type Claims struct {
	TokenType string `json:"token_type"`
	UserID    int64  `json:"user_id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	jwt.RegisteredClaims
}

// RefreshClaims represents refresh token claims.
type RefreshClaims struct {
	TokenType string `json:"token_type"`
	UserID    int64  `json:"user_id"`
	jwt.RegisteredClaims
}

// Config holds authentication configuration.
type Config struct {
	JWTSecret          string
	TokenExpiry        time.Duration
	RefreshTokenExpiry time.Duration
}

// Service provides authentication operations.
type Service struct {
	config         Config
	tokenBlacklist TokenBlacklistInterface
	sessions       SessionStore
	users          repository.UserRepository
}

// TokenBlacklistInterface defines the interface for token blacklist operations.
type TokenBlacklistInterface interface {
	IsRevoked(ctx context.Context, token string) bool
	RevokeToken(ctx context.Context, token string, expiresAt time.Time) error
}

// NewService creates a new authentication service.
func NewService(cfg Config) *Service {
	if cfg.TokenExpiry == 0 {
		cfg.TokenExpiry = 24 * time.Hour
	}
	if cfg.RefreshTokenExpiry == 0 {
		cfg.RefreshTokenExpiry = 7 * 24 * time.Hour
	}
	return &Service{config: cfg, sessions: &memorySessionStore{sessions: make(map[string]Session)}}
}

// WithTokenBlacklist adds token blacklist support to the service.
func (s *Service) WithTokenBlacklist(blacklist TokenBlacklistInterface) *Service {
	s.tokenBlacklist = blacklist
	return s
}

// IsTokenBlacklisted checks if a token has been revoked.
func (s *Service) IsTokenBlacklisted(ctx context.Context, token string) bool {
	if s.tokenBlacklist == nil {
		return false
	}
	return s.tokenBlacklist.IsRevoked(ctx, token)
}

// RevokeToken removes the durable session shared by its access/refresh tokens.
func (s *Service) RevokeToken(ctx context.Context, token string, expiresAt time.Time) error {
	claims, err := s.ValidateToken(token)
	if err != nil {
		return err
	}
	return s.consumeSession(ctx, claims.ID)
}

// GenerateToken generates a JWT token for a user.
func (s *Service) GenerateToken(userID int64, username, role string) (string, error) {
	return s.GenerateTokenWithExpiry(userID, username, role, s.config.TokenExpiry)
}

// GenerateTokenWithExpiry generates a JWT token for a user with a custom expiry.
func (s *Service) GenerateTokenWithExpiry(userID int64, username, role string, expiry time.Duration, expectedPassword ...string) (string, error) {
	if expiry <= 0 {
		expiry = s.config.TokenExpiry
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := s.newSession(ctx, userID, "login", expiry, expectedPassword...)
	if err != nil {
		return "", err
	}
	return s.signAccessToken(session.ID, userID, username, role, expiry, "access")
}

func (s *Service) signAccessToken(sessionID string, userID int64, username, role string, expiry time.Duration, tokenType string) (string, error) {
	claims := &Claims{
		TokenType: tokenType,
		UserID:    userID,
		Username:  username,
		Role:      role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// TokenExpiry returns the configured access token expiry duration.
func (s *Service) TokenExpiry() time.Duration {
	return s.config.TokenExpiry
}

// GenerateRefreshToken generates a refresh token for a user.
func (s *Service) GenerateRefreshToken(userID int64) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := s.newSession(ctx, userID, "login", s.config.RefreshTokenExpiry)
	if err != nil {
		return "", err
	}
	return s.signRefreshToken(session.ID, userID)
}

func (s *Service) signRefreshToken(sessionID string, userID int64) (string, error) {
	claims := &RefreshClaims{
		TokenType: "refresh",
		UserID:    userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        sessionID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.RefreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// ValidateToken validates a JWT token and returns the claims.
func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	return s.validateClaims(tokenString, "access", "login")
}

func (s *Service) validateClaims(tokenString, tokenType, sessionKind string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.NewUnauthorizedError("invalid token signing method")
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithExpirationRequired())

	if err != nil {
		return nil, errors.NewUnauthorizedError("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.TokenType != tokenType {
		return nil, errors.NewUnauthorizedError("invalid token claims")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.IsTokenBlacklisted(ctx, tokenString) {
		return nil, errors.NewUnauthorizedError("token revoked")
	}
	if err := s.validateSession(ctx, claims.ID, claims.UserID, sessionKind); err != nil {
		return nil, errors.NewUnauthorizedError("invalid or revoked session")
	}
	return claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the claims.
func (s *Service) ValidateRefreshToken(tokenString string) (*RefreshClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.NewUnauthorizedError("invalid token signing method")
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithExpirationRequired())

	if err != nil {
		return nil, errors.NewUnauthorizedError("invalid refresh token")
	}

	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid || claims.TokenType != "refresh" {
		return nil, errors.NewUnauthorizedError("invalid refresh token claims")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.IsTokenBlacklisted(ctx, tokenString) {
		return nil, errors.NewUnauthorizedError("token revoked")
	}
	if err := s.validateSession(ctx, claims.ID, claims.UserID, "login"); err != nil {
		return nil, errors.NewUnauthorizedError("invalid or revoked session")
	}
	return claims, nil
}

// GenerateTokenPair gives access and refresh tokens one revocable session.
func (s *Service) GenerateTokenPair(userID int64, username, role string, expiry time.Duration, expectedPassword ...string) (string, string, error) {
	if expiry <= 0 {
		expiry = s.config.TokenExpiry
	}
	lifetime := s.config.RefreshTokenExpiry
	if expiry > lifetime {
		lifetime = expiry
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := s.newSession(ctx, userID, "login", lifetime, expectedPassword...)
	if err != nil {
		return "", "", err
	}
	access, err := s.signAccessToken(session.ID, userID, username, role, expiry, "access")
	if err != nil {
		return "", "", err
	}
	refresh, err := s.signRefreshToken(session.ID, userID)
	return access, refresh, err
}

// ConsumeRefreshToken must succeed before issuing replacement tokens. Its delete
// is atomic across requests and processes, so a refresh cannot be replayed.
func (s *Service) ConsumeRefreshToken(ctx context.Context, token string) error {
	claims, err := s.ValidateRefreshToken(token)
	if err != nil {
		return err
	}
	return s.consumeSession(ctx, claims.ID)
}

func (s *Service) GenerateLoginChallenge(userID int64, expectedPassword string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	session, err := s.newSession(ctx, userID, "2fa", 5*time.Minute, expectedPassword)
	if err != nil {
		return "", err
	}
	return s.signAccessToken(session.ID, userID, "", "", 5*time.Minute, "2fa")
}

func (s *Service) ValidateLoginChallenge(token string, userID int64) error {
	claims, err := s.validateClaims(token, "2fa", "2fa")
	if err != nil {
		return err
	}
	if claims.UserID != userID {
		return errors.NewUnauthorizedError("challenge user mismatch")
	}
	return nil
}

func (s *Service) ConsumeLoginChallenge(ctx context.Context, token string, userID int64) error {
	claims, err := s.validateClaims(token, "2fa", "2fa")
	if err != nil {
		return err
	}
	if claims.UserID != userID {
		return errors.NewUnauthorizedError("challenge user mismatch")
	}
	return s.consumeSession(ctx, claims.ID)
}

// HashPassword hashes a password using bcrypt.
func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash.
func (s *Service) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateTemporaryPassword generates a random temporary password.
func (s *Service) GenerateTemporaryPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	const length = 12

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// SECURITY FIX: Do not fall back to weak randomness
			// If crypto/rand fails, the system is in a bad state
			panic(fmt.Sprintf("crypto/rand failed: %v - system entropy exhausted", err))
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateTOTPSecret generates a new TOTP secret for 2FA.
func (s *Service) GenerateTOTPSecret() (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567" // Base32 charset
	const length = 32

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[n.Int64()]
	}
	return string(b), nil
}

// VerifyTOTP verifies a TOTP code against a secret.
func (s *Service) VerifyTOTP(secret, code string) bool {
	return verifyTOTPAtTime(secret, code, time.Now().UTC())
}

func verifyTOTPAtTime(secret, code string, at time.Time) bool {
	if !isSixDigitCode(code) {
		return false
	}

	for offset := -1; offset <= 1; offset++ {
		candidate, err := generateTOTPCode(secret, at.Add(time.Duration(offset)*30*time.Second))
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}

	return false
}

func generateTOTPCode(secret string, at time.Time) (string, error) {
	normalizedSecret := strings.ToUpper(strings.TrimSpace(secret))
	if normalizedSecret == "" {
		return "", fmt.Errorf("missing TOTP secret")
	}

	decodedSecret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(normalizedSecret)
	if err != nil {
		return "", err
	}

	counter := uint64(at.UTC().Unix() / 30)
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)

	mac := hmac.New(sha1.New, decodedSecret)
	if _, err := mac.Write(counterBytes[:]); err != nil {
		return "", err
	}
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	return fmt.Sprintf("%06d", binaryCode%1000000), nil
}

func isSixDigitCode(code string) bool {
	if len(code) != 6 {
		return false
	}

	for _, r := range code {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}
