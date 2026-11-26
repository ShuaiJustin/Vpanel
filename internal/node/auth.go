// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"v/internal/logger"
)

// Authentication errors
var (
	ErrIPBlocked           = errors.New("IP is temporarily blocked")
	ErrIPNotWhitelisted    = errors.New("IP is not in whitelist")
	ErrAuthenticationFailed = errors.New("authentication failed")
)

// AuthConfig holds configuration for node authentication.
type AuthConfig struct {
	// MaxFailures is the maximum number of authentication failures before blocking
	MaxFailures int
	// BlockDuration is how long an IP is blocked after exceeding max failures
	BlockDuration time.Duration
	// FailureWindow is the time window for counting failures
	FailureWindow time.Duration
}

// DefaultAuthConfig returns the default authentication configuration.
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		MaxFailures:   5,
		BlockDuration: 15 * time.Minute,
		FailureWindow: 5 * time.Minute,
	}
}

// AuthFailureRecord represents an authentication failure record.
type AuthFailureRecord struct {
	IP           string
	Attempts     int
	FirstAttempt time.Time
	BlockedUntil *time.Time
}

// AuthFailureStore defines the interface for storing authentication failures.
type AuthFailureStore interface {
	// GetFailures retrieves the failure record for an IP
	GetFailures(ctx context.Context, ip string) (*AuthFailureRecord, error)
	// RecordFailure records an authentication failure for an IP
	RecordFailure(ctx context.Context, ip string) error
	// ClearFailures clears the failure record for an IP
	ClearFailures(ctx context.Context, ip string) error
	// IsBlocked checks if an IP is currently blocked
	IsBlocked(ctx context.Context, ip string) (bool, *time.Time, error)
	// BlockIP blocks an IP until the specified time
	BlockIP(ctx context.Context, ip string, until time.Time) error
}

// Authenticator provides node authentication functionality.
type Authenticator struct {
	nodeService  *Service
	failureStore AuthFailureStore
	config       *AuthConfig
	logger       logger.Logger
}

// NewAuthenticator creates a new node authenticator.
func NewAuthenticator(
	nodeService *Service,
	failureStore AuthFailureStore,
	config *AuthConfig,
	log logger.Logger,
) *Authenticator {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &Authenticator{
		nodeService:  nodeService,
		failureStore: failureStore,
		config:       config,
		logger:       log,
	}
}

// AuthenticateResult represents the result of an authentication attempt.
type AuthenticateResult struct {
	Node      *Node
	Allowed   bool
	Error     error
	ErrorCode string
}

// Authenticate authenticates a node connection using token and IP.
func (a *Authenticator) Authenticate(ctx context.Context, token string, clientIP string) *AuthenticateResult {
	result := &AuthenticateResult{}

	// Check if IP is blocked
	blocked, blockedUntil, err := a.failureStore.IsBlocked(ctx, clientIP)
	if err != nil {
		a.logger.Error("Failed to check IP block status",
			logger.Err(err),
			logger.F("ip", clientIP))
	}
	if blocked {
		result.Error = ErrIPBlocked
		result.ErrorCode = "IP_BLOCKED"
		a.logger.Warn("Authentication attempt from blocked IP",
			logger.F("ip", clientIP),
			logger.F("blocked_until", blockedUntil))
		return result
	}

	// Validate token
	if token == "" {
		a.handleAuthFailure(ctx, clientIP, "empty token")
		result.Error = ErrInvalidToken
		result.ErrorCode = "INVALID_TOKEN"
		return result
	}

	node, err := a.nodeService.ValidateToken(ctx, token)
	if err != nil {
		a.handleAuthFailure(ctx, clientIP, err.Error())
		if errors.Is(err, ErrTokenRevoked) {
			result.Error = ErrTokenRevoked
			result.ErrorCode = "TOKEN_REVOKED"
		} else {
			result.Error = ErrInvalidToken
			result.ErrorCode = "INVALID_TOKEN"
		}
		return result
	}

	// Check IP whitelist if configured
	if len(node.IPWhitelist) > 0 {
		if !a.isIPWhitelisted(clientIP, node.IPWhitelist) {
			a.handleAuthFailure(ctx, clientIP, "IP not in whitelist")
			result.Error = ErrIPNotWhitelisted
			result.ErrorCode = "IP_NOT_WHITELISTED"
			a.logger.Warn("Authentication attempt from non-whitelisted IP",
				logger.F("ip", clientIP),
				logger.F("node_id", node.ID),
				logger.F("node_name", node.Name))
			return result
		}
	}

	// Authentication successful - clear any failure records
	if err := a.failureStore.ClearFailures(ctx, clientIP); err != nil {
		a.logger.Error("Failed to clear auth failures",
			logger.Err(err),
			logger.F("ip", clientIP))
	}

	result.Node = node
	result.Allowed = true
	a.logger.Info("Node authenticated successfully",
		logger.F("node_id", node.ID),
		logger.F("node_name", node.Name),
		logger.F("ip", clientIP))

	return result
}

// handleAuthFailure handles an authentication failure.
func (a *Authenticator) handleAuthFailure(ctx context.Context, ip string, reason string) {
	a.logger.Warn("Authentication failed",
		logger.F("ip", ip),
		logger.F("reason", reason))

	// Record the failure
	if err := a.failureStore.RecordFailure(ctx, ip); err != nil {
		a.logger.Error("Failed to record auth failure",
			logger.Err(err),
			logger.F("ip", ip))
		return
	}

	// Check if we should block the IP
	record, err := a.failureStore.GetFailures(ctx, ip)
	if err != nil {
		a.logger.Error("Failed to get failure record",
			logger.Err(err),
			logger.F("ip", ip))
		return
	}

	if record != nil && record.Attempts >= a.config.MaxFailures {
		blockUntil := time.Now().Add(a.config.BlockDuration)
		if err := a.failureStore.BlockIP(ctx, ip, blockUntil); err != nil {
			a.logger.Error("Failed to block IP",
				logger.Err(err),
				logger.F("ip", ip))
			return
		}
		a.logger.Warn("IP blocked due to too many auth failures",
			logger.F("ip", ip),
			logger.F("attempts", record.Attempts),
			logger.F("blocked_until", blockUntil))
	}
}

// isIPWhitelisted checks if an IP is in the whitelist.
func (a *Authenticator) isIPWhitelisted(clientIP string, whitelist []string) bool {
	if len(whitelist) == 0 {
		return true // No whitelist means all IPs are allowed
	}

	parsedClientIP := net.ParseIP(clientIP)
	if parsedClientIP == nil {
		return false
	}

	for _, entry := range whitelist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// Check if it's a CIDR notation
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err != nil {
				continue
			}
			if network.Contains(parsedClientIP) {
				return true
			}
		} else {
			// Direct IP comparison
			whitelistedIP := net.ParseIP(entry)
			if whitelistedIP != nil && whitelistedIP.Equal(parsedClientIP) {
				return true
			}
		}
	}

	return false
}

// ValidateToken validates a token and returns the associated node.
// This is a convenience method that wraps the node service's ValidateToken.
func (a *Authenticator) ValidateToken(ctx context.Context, token string) (*Node, error) {
	return a.nodeService.ValidateToken(ctx, token)
}

// RotateToken rotates a node's token, invalidating the old one immediately.
func (a *Authenticator) RotateToken(ctx context.Context, nodeID int64) (string, error) {
	return a.nodeService.RotateToken(ctx, nodeID)
}

// RevokeToken revokes a node's token.
func (a *Authenticator) RevokeToken(ctx context.Context, nodeID int64) error {
	return a.nodeService.RevokeToken(ctx, nodeID)
}

// ============================================
// In-Memory Auth Failure Store Implementation
// ============================================

// InMemoryAuthFailureStore is an in-memory implementation of AuthFailureStore.
type InMemoryAuthFailureStore struct {
	mu       sync.RWMutex
	failures map[string]*AuthFailureRecord
	config   *AuthConfig
}

// NewInMemoryAuthFailureStore creates a new in-memory auth failure store.
func NewInMemoryAuthFailureStore(config *AuthConfig) *InMemoryAuthFailureStore {
	if config == nil {
		config = DefaultAuthConfig()
	}
	return &InMemoryAuthFailureStore{
		failures: make(map[string]*AuthFailureRecord),
		config:   config,
	}
}

// GetFailures retrieves the failure record for an IP.
func (s *InMemoryAuthFailureStore) GetFailures(ctx context.Context, ip string) (*AuthFailureRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.failures[ip]
	if !exists {
		return nil, nil
	}

	// Check if the failure window has expired
	if time.Since(record.FirstAttempt) > s.config.FailureWindow {
		return nil, nil
	}

	return record, nil
}

// RecordFailure records an authentication failure for an IP.
func (s *InMemoryAuthFailureStore) RecordFailure(ctx context.Context, ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.failures[ip]
	if !exists || time.Since(record.FirstAttempt) > s.config.FailureWindow {
		// Start a new failure window
		s.failures[ip] = &AuthFailureRecord{
			IP:           ip,
			Attempts:     1,
			FirstAttempt: time.Now(),
		}
		return nil
	}

	// Increment attempts within the window
	record.Attempts++
	return nil
}

// ClearFailures clears the failure record for an IP.
func (s *InMemoryAuthFailureStore) ClearFailures(ctx context.Context, ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.failures, ip)
	return nil
}

// IsBlocked checks if an IP is currently blocked.
func (s *InMemoryAuthFailureStore) IsBlocked(ctx context.Context, ip string) (bool, *time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.failures[ip]
	if !exists {
		return false, nil, nil
	}

	if record.BlockedUntil == nil {
		return false, nil, nil
	}

	if time.Now().After(*record.BlockedUntil) {
		return false, nil, nil
	}

	return true, record.BlockedUntil, nil
}

// BlockIP blocks an IP until the specified time.
func (s *InMemoryAuthFailureStore) BlockIP(ctx context.Context, ip string, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	record, exists := s.failures[ip]
	if !exists {
		record = &AuthFailureRecord{
			IP:           ip,
			Attempts:     0,
			FirstAttempt: time.Now(),
		}
		s.failures[ip] = record
	}

	record.BlockedUntil = &until
	return nil
}

// Cleanup removes expired records from the store.
func (s *InMemoryAuthFailureStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for ip, record := range s.failures {
		// Remove if both the failure window has expired and the block has expired
		windowExpired := now.Sub(record.FirstAttempt) > s.config.FailureWindow
		blockExpired := record.BlockedUntil == nil || now.After(*record.BlockedUntil)

		if windowExpired && blockExpired {
			delete(s.failures, ip)
		}
	}
}
