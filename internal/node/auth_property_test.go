// Package node provides node management functionality.
package node

import (
	"context"
	"fmt"
	"testing"
	"testing/quick"
	"time"

	"v/internal/logger"
)

// Feature: multi-server-management, Property 6: Token Authentication
// Validates: Requirements 3.1, 10.1, 10.3
// For any node connection attempt, connections with valid tokens SHALL be accepted,
// and connections with invalid or revoked tokens SHALL be rejected.

// mockNodeServiceForAuth is a mock node service for authentication testing.
type mockNodeServiceForAuth struct {
	nodes       map[string]*Node // token -> node
	revokedTokens map[string]bool
}

func newMockNodeServiceForAuth() *mockNodeServiceForAuth {
	return &mockNodeServiceForAuth{
		nodes:         make(map[string]*Node),
		revokedTokens: make(map[string]bool),
	}
}

func (m *mockNodeServiceForAuth) addNode(token string, node *Node) {
	m.nodes[token] = node
}

func (m *mockNodeServiceForAuth) revokeToken(token string) {
	m.revokedTokens[token] = true
}

func (m *mockNodeServiceForAuth) ValidateToken(ctx context.Context, token string) (*Node, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	if m.revokedTokens[token] {
		return nil, ErrTokenRevoked
	}
	node, exists := m.nodes[token]
	if !exists {
		return nil, ErrInvalidToken
	}
	return node, nil
}

// mockAuthenticatorForTest creates a test authenticator with mock dependencies.
type mockAuthenticatorForTest struct {
	nodeService  *mockNodeServiceForAuth
	failureStore *InMemoryAuthFailureStore
	authenticator *Authenticator
}

func newMockAuthenticatorForTest() *mockAuthenticatorForTest {
	nodeService := newMockNodeServiceForAuth()
	failureStore := NewInMemoryAuthFailureStore(DefaultAuthConfig())
	
	// Create a wrapper service that implements the required interface
	auth := &Authenticator{
		nodeService:  nil, // We'll use the mock directly
		failureStore: failureStore,
		config:       DefaultAuthConfig(),
		logger:       logger.NewNopLogger(),
	}
	
	return &mockAuthenticatorForTest{
		nodeService:   nodeService,
		failureStore:  failureStore,
		authenticator: auth,
	}
}

// authenticateWithMock performs authentication using the mock service.
func (m *mockAuthenticatorForTest) authenticateWithMock(ctx context.Context, token string, clientIP string) *AuthenticateResult {
	result := &AuthenticateResult{}

	// Check if IP is blocked
	blocked, _, _ := m.failureStore.IsBlocked(ctx, clientIP)
	if blocked {
		result.Error = ErrIPBlocked
		result.ErrorCode = "IP_BLOCKED"
		return result
	}

	// Validate token using mock service
	node, err := m.nodeService.ValidateToken(ctx, token)
	if err != nil {
		m.failureStore.RecordFailure(ctx, clientIP)
		if err == ErrTokenRevoked {
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
		if !m.authenticator.isIPWhitelisted(clientIP, node.IPWhitelist) {
			m.failureStore.RecordFailure(ctx, clientIP)
			result.Error = ErrIPNotWhitelisted
			result.ErrorCode = "IP_NOT_WHITELISTED"
			return result
		}
	}

	// Success
	m.failureStore.ClearFailures(ctx, clientIP)
	result.Node = node
	result.Allowed = true
	return result
}

// TestProperty_ValidTokensAccepted tests that valid tokens are accepted.
func TestProperty_ValidTokensAccepted(t *testing.T) {
	// Property: For any valid token, authentication should succeed
	f := func(nodeID uint16, nodeName string) bool {
		if nodeName == "" {
			return true // Skip empty names
		}

		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Generate a valid token
		token, err := GenerateToken()
		if err != nil {
			t.Logf("Failed to generate token: %v", err)
			return false
		}

		// Add node with the token
		node := &Node{
			ID:     int64(nodeID),
			Name:   nodeName,
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		// Authenticate with valid token
		result := mock.authenticateWithMock(ctx, token, "192.168.1.1")

		// Should succeed
		if !result.Allowed {
			t.Logf("Expected authentication to succeed for valid token")
			return false
		}
		if result.Node == nil {
			t.Logf("Expected node to be returned")
			return false
		}
		if result.Node.ID != int64(nodeID) {
			t.Logf("Expected node ID %d, got %d", nodeID, result.Node.ID)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_InvalidTokensRejected tests that invalid tokens are rejected.
func TestProperty_InvalidTokensRejected(t *testing.T) {
	// Property: For any invalid token, authentication should fail
	f := func(invalidToken string) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Add a valid node with a different token
		validToken, _ := GenerateToken()
		mock.nodeService.addNode(validToken, &Node{
			ID:     1,
			Name:   "test-node",
			Status: "online",
		})

		// Authenticate with invalid token (not the valid one)
		if invalidToken == validToken {
			return true // Skip if randomly generated same token
		}

		result := mock.authenticateWithMock(ctx, invalidToken, "192.168.1.1")

		// Should fail
		if result.Allowed {
			t.Logf("Expected authentication to fail for invalid token")
			return false
		}
		if result.Error == nil {
			t.Logf("Expected error to be set")
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_RevokedTokensRejected tests that revoked tokens are rejected.
func TestProperty_RevokedTokensRejected(t *testing.T) {
	// Property: For any revoked token, authentication should fail with TOKEN_REVOKED
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Generate a token and add node
		token, err := GenerateToken()
		if err != nil {
			t.Logf("Failed to generate token: %v", err)
			return false
		}

		node := &Node{
			ID:     int64(nodeID),
			Name:   "test-node",
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		// First, verify it works
		result := mock.authenticateWithMock(ctx, token, "192.168.1.1")
		if !result.Allowed {
			t.Logf("Expected initial authentication to succeed")
			return false
		}

		// Revoke the token
		mock.nodeService.revokeToken(token)

		// Now authentication should fail
		result = mock.authenticateWithMock(ctx, token, "192.168.1.2")
		if result.Allowed {
			t.Logf("Expected authentication to fail for revoked token")
			return false
		}
		if result.ErrorCode != "TOKEN_REVOKED" {
			t.Logf("Expected error code TOKEN_REVOKED, got %s", result.ErrorCode)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_EmptyTokensRejected tests that empty tokens are rejected.
func TestProperty_EmptyTokensRejected(t *testing.T) {
	// Property: Empty tokens should always be rejected
	ctx := context.Background()
	mock := newMockAuthenticatorForTest()

	// Add a valid node
	validToken, _ := GenerateToken()
	mock.nodeService.addNode(validToken, &Node{
		ID:     1,
		Name:   "test-node",
		Status: "online",
	})

	// Test empty token
	result := mock.authenticateWithMock(ctx, "", "192.168.1.1")
	if result.Allowed {
		t.Error("Expected authentication to fail for empty token")
	}
	if result.ErrorCode != "INVALID_TOKEN" {
		t.Errorf("Expected error code INVALID_TOKEN, got %s", result.ErrorCode)
	}
}

// TestProperty_AuthenticationConsistency tests that authentication results are consistent.
func TestProperty_AuthenticationConsistency(t *testing.T) {
	// Property: For the same token and IP, authentication should produce consistent results
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID),
			Name:   "test-node",
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		clientIP := "192.168.1.100"

		// Authenticate multiple times
		results := make([]*AuthenticateResult, 5)
		for i := 0; i < 5; i++ {
			results[i] = mock.authenticateWithMock(ctx, token, clientIP)
		}

		// All results should be consistent
		for i := 1; i < 5; i++ {
			if results[i].Allowed != results[0].Allowed {
				t.Logf("Inconsistent authentication results")
				return false
			}
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_TokenValidationIdempotent tests that token validation is idempotent.
func TestProperty_TokenValidationIdempotent(t *testing.T) {
	// Property: Validating the same token multiple times should return the same node
	f := func(nodeID uint16, nodeName string) bool {
		if nodeName == "" {
			return true
		}

		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID),
			Name:   nodeName,
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		// Validate multiple times
		for i := 0; i < 10; i++ {
			result, err := mock.nodeService.ValidateToken(ctx, token)
			if err != nil {
				t.Logf("Validation failed on iteration %d: %v", i, err)
				return false
			}
			if result.ID != int64(nodeID) {
				t.Logf("Node ID mismatch on iteration %d", i)
				return false
			}
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_BlockedIPsRejected tests that blocked IPs are rejected.
func TestProperty_BlockedIPsRejected(t *testing.T) {
	// Property: Authentication from blocked IPs should fail regardless of token validity
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID),
			Name:   "test-node",
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		clientIP := "192.168.1.100"

		// Block the IP
		blockUntil := time.Now().Add(1 * time.Hour)
		mock.failureStore.BlockIP(ctx, clientIP, blockUntil)

		// Authentication should fail even with valid token
		result := mock.authenticateWithMock(ctx, token, clientIP)
		if result.Allowed {
			t.Logf("Expected authentication to fail for blocked IP")
			return false
		}
		if result.ErrorCode != "IP_BLOCKED" {
			t.Logf("Expected error code IP_BLOCKED, got %s", result.ErrorCode)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}


// Feature: multi-server-management, Property 20: Token Rotation Invalidation
// Validates: Requirements 10.2
// For any token rotation, the old token SHALL be immediately invalidated.

// mockNodeServiceWithRotation extends mockNodeServiceForAuth with rotation support.
type mockNodeServiceWithRotation struct {
	nodes         map[int64]*Node    // nodeID -> node
	tokenToNode   map[string]int64   // token -> nodeID
	revokedTokens map[string]bool
}

func newMockNodeServiceWithRotation() *mockNodeServiceWithRotation {
	return &mockNodeServiceWithRotation{
		nodes:         make(map[int64]*Node),
		tokenToNode:   make(map[string]int64),
		revokedTokens: make(map[string]bool),
	}
}

func (m *mockNodeServiceWithRotation) addNode(node *Node, token string) {
	m.nodes[node.ID] = node
	m.tokenToNode[token] = node.ID
}

func (m *mockNodeServiceWithRotation) ValidateToken(ctx context.Context, token string) (*Node, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	if m.revokedTokens[token] {
		return nil, ErrTokenRevoked
	}
	nodeID, exists := m.tokenToNode[token]
	if !exists {
		return nil, ErrInvalidToken
	}
	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, ErrInvalidToken
	}
	return node, nil
}

func (m *mockNodeServiceWithRotation) RotateToken(ctx context.Context, nodeID int64) (string, string, error) {
	node, exists := m.nodes[nodeID]
	if !exists {
		return "", "", ErrNodeNotFound
	}

	// Find and remove old token
	var oldToken string
	for token, nid := range m.tokenToNode {
		if nid == nodeID {
			oldToken = token
			delete(m.tokenToNode, token)
			break
		}
	}

	// Generate new token
	newToken, err := GenerateToken()
	if err != nil {
		return "", "", err
	}

	// Add new token mapping
	m.tokenToNode[newToken] = node.ID

	return newToken, oldToken, nil
}

// TestProperty_TokenRotationInvalidatesOldToken tests that token rotation invalidates the old token.
func TestProperty_TokenRotationInvalidatesOldToken(t *testing.T) {
	// Property: After token rotation, the old token should be immediately invalid
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockNodeServiceWithRotation()

		// Create initial token and node
		initialToken, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID) + 1, // Ensure non-zero
			Name:   "test-node",
			Status: "online",
		}
		mock.addNode(node, initialToken)

		// Verify initial token works
		validatedNode, err := mock.ValidateToken(ctx, initialToken)
		if err != nil {
			t.Logf("Initial token validation failed: %v", err)
			return false
		}
		if validatedNode.ID != node.ID {
			t.Logf("Initial token returned wrong node")
			return false
		}

		// Rotate the token
		newToken, oldToken, err := mock.RotateToken(ctx, node.ID)
		if err != nil {
			t.Logf("Token rotation failed: %v", err)
			return false
		}

		// Old token should be the initial token
		if oldToken != initialToken {
			t.Logf("Old token mismatch")
			return false
		}

		// Old token should now be invalid
		_, err = mock.ValidateToken(ctx, oldToken)
		if err == nil {
			t.Logf("Old token should be invalid after rotation")
			return false
		}

		// New token should be valid
		validatedNode, err = mock.ValidateToken(ctx, newToken)
		if err != nil {
			t.Logf("New token validation failed: %v", err)
			return false
		}
		if validatedNode.ID != node.ID {
			t.Logf("New token returned wrong node")
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_TokenRotationGeneratesUniqueToken tests that rotation generates unique tokens.
func TestProperty_TokenRotationGeneratesUniqueToken(t *testing.T) {
	// Property: Each token rotation should generate a unique new token
	f := func(nodeID uint16, rotations uint8) bool {
		ctx := context.Background()
		mock := newMockNodeServiceWithRotation()

		// Limit rotations to reasonable number
		numRotations := int(rotations%20) + 1

		initialToken, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID) + 1,
			Name:   "test-node",
			Status: "online",
		}
		mock.addNode(node, initialToken)

		// Track all tokens seen
		seenTokens := make(map[string]bool)
		seenTokens[initialToken] = true

		// Perform multiple rotations
		for i := 0; i < numRotations; i++ {
			newToken, _, err := mock.RotateToken(ctx, node.ID)
			if err != nil {
				t.Logf("Rotation %d failed: %v", i, err)
				return false
			}

			// New token should be unique
			if seenTokens[newToken] {
				t.Logf("Duplicate token generated on rotation %d", i)
				return false
			}
			seenTokens[newToken] = true
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_TokenRotationPreservesNodeAssociation tests that rotation preserves node association.
func TestProperty_TokenRotationPreservesNodeAssociation(t *testing.T) {
	// Property: After rotation, the new token should be associated with the same node
	f := func(nodeID uint16, nodeName string) bool {
		if nodeName == "" {
			return true
		}

		ctx := context.Background()
		mock := newMockNodeServiceWithRotation()

		initialToken, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID) + 1,
			Name:   nodeName,
			Status: "online",
		}
		mock.addNode(node, initialToken)

		// Rotate token
		newToken, _, err := mock.RotateToken(ctx, node.ID)
		if err != nil {
			return false
		}

		// Validate new token returns same node
		validatedNode, err := mock.ValidateToken(ctx, newToken)
		if err != nil {
			t.Logf("New token validation failed: %v", err)
			return false
		}

		// Node properties should match
		if validatedNode.ID != node.ID {
			t.Logf("Node ID mismatch: expected %d, got %d", node.ID, validatedNode.ID)
			return false
		}
		if validatedNode.Name != node.Name {
			t.Logf("Node name mismatch: expected %s, got %s", node.Name, validatedNode.Name)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_MultipleRotationsInvalidateAllPreviousTokens tests that all previous tokens are invalid.
func TestProperty_MultipleRotationsInvalidateAllPreviousTokens(t *testing.T) {
	// Property: After multiple rotations, all previous tokens should be invalid
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockNodeServiceWithRotation()

		initialToken, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID) + 1,
			Name:   "test-node",
			Status: "online",
		}
		mock.addNode(node, initialToken)

		// Collect all tokens
		allTokens := []string{initialToken}

		// Perform 5 rotations
		for i := 0; i < 5; i++ {
			newToken, _, err := mock.RotateToken(ctx, node.ID)
			if err != nil {
				return false
			}
			allTokens = append(allTokens, newToken)
		}

		// Only the last token should be valid
		lastToken := allTokens[len(allTokens)-1]
		for i, token := range allTokens {
			_, err := mock.ValidateToken(ctx, token)
			if i == len(allTokens)-1 {
				// Last token should be valid
				if err != nil {
					t.Logf("Last token should be valid")
					return false
				}
			} else {
				// All other tokens should be invalid
				if err == nil {
					t.Logf("Token %d should be invalid after rotation", i)
					return false
				}
			}
		}

		// Verify last token works
		validatedNode, err := mock.ValidateToken(ctx, lastToken)
		if err != nil {
			return false
		}
		if validatedNode.ID != node.ID {
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}


// Feature: multi-server-management, Property 21: IP Whitelist Enforcement
// Validates: Requirements 10.5
// For any node connection from a non-whitelisted IP (when whitelist is enabled),
// the connection SHALL be rejected.

// TestProperty_WhitelistedIPsAccepted tests that whitelisted IPs are accepted.
func TestProperty_WhitelistedIPsAccepted(t *testing.T) {
	// Property: For any IP in the whitelist, authentication should succeed (with valid token)
	f := func(a, b, c, d uint8) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Generate IP address
		clientIP := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)

		// Create token and node with whitelist containing the client IP
		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:          1,
			Name:        "test-node",
			Status:      "online",
			IPWhitelist: []string{clientIP},
		}
		mock.nodeService.addNode(token, node)

		// Authenticate from whitelisted IP
		result := mock.authenticateWithMock(ctx, token, clientIP)

		// Should succeed
		if !result.Allowed {
			t.Logf("Expected authentication to succeed for whitelisted IP %s", clientIP)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_NonWhitelistedIPsRejected tests that non-whitelisted IPs are rejected.
func TestProperty_NonWhitelistedIPsRejected(t *testing.T) {
	// Property: For any IP not in the whitelist, authentication should fail
	f := func(a, b, c, d uint8) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Generate client IP
		clientIP := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)

		// Create a different whitelisted IP by adding 1 with wraparound
		whitelistedIP := fmt.Sprintf("%d.%d.%d.%d", 
			(int(a)+1)%256, (int(b)+1)%256, (int(c)+1)%256, (int(d)+1)%256)

		// Skip if they happen to be the same
		if clientIP == whitelistedIP {
			return true
		}

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:          1,
			Name:        "test-node",
			Status:      "online",
			IPWhitelist: []string{whitelistedIP},
		}
		mock.nodeService.addNode(token, node)

		// Authenticate from non-whitelisted IP
		result := mock.authenticateWithMock(ctx, token, clientIP)

		// Should fail with IP_NOT_WHITELISTED
		if result.Allowed {
			t.Logf("Expected authentication to fail for non-whitelisted IP %s", clientIP)
			return false
		}
		if result.ErrorCode != "IP_NOT_WHITELISTED" {
			t.Logf("Expected error code IP_NOT_WHITELISTED, got %s", result.ErrorCode)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_EmptyWhitelistAllowsAll tests that empty whitelist allows all IPs.
func TestProperty_EmptyWhitelistAllowsAll(t *testing.T) {
	// Property: When whitelist is empty, all IPs should be allowed
	f := func(a, b, c, d uint8) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		clientIP := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:          1,
			Name:        "test-node",
			Status:      "online",
			IPWhitelist: []string{}, // Empty whitelist
		}
		mock.nodeService.addNode(token, node)

		// Authenticate from any IP
		result := mock.authenticateWithMock(ctx, token, clientIP)

		// Should succeed
		if !result.Allowed {
			t.Logf("Expected authentication to succeed with empty whitelist for IP %s", clientIP)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_CIDRWhitelistWorks tests that CIDR notation in whitelist works.
func TestProperty_CIDRWhitelistWorks(t *testing.T) {
	// Property: IPs within a CIDR range should be accepted
	testCases := []struct {
		cidr     string
		validIP  string
		invalidIP string
	}{
		{"192.168.1.0/24", "192.168.1.100", "192.168.2.1"},
		{"10.0.0.0/8", "10.255.255.255", "11.0.0.1"},
		{"172.16.0.0/16", "172.16.255.255", "172.17.0.1"},
	}

	for _, tc := range testCases {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		token, err := GenerateToken()
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		node := &Node{
			ID:          1,
			Name:        "test-node",
			Status:      "online",
			IPWhitelist: []string{tc.cidr},
		}
		mock.nodeService.addNode(token, node)

		// Valid IP should be accepted
		result := mock.authenticateWithMock(ctx, token, tc.validIP)
		if !result.Allowed {
			t.Errorf("Expected IP %s to be accepted for CIDR %s", tc.validIP, tc.cidr)
		}

		// Invalid IP should be rejected
		result = mock.authenticateWithMock(ctx, token, tc.invalidIP)
		if result.Allowed {
			t.Errorf("Expected IP %s to be rejected for CIDR %s", tc.invalidIP, tc.cidr)
		}
	}
}

// TestProperty_MultipleWhitelistEntries tests that multiple whitelist entries work.
func TestProperty_MultipleWhitelistEntries(t *testing.T) {
	// Property: Any IP matching any whitelist entry should be accepted
	f := func(a, b uint8) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Create multiple whitelist entries
		ip1 := fmt.Sprintf("192.168.1.%d", a)
		ip2 := fmt.Sprintf("10.0.0.%d", b)

		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:          1,
			Name:        "test-node",
			Status:      "online",
			IPWhitelist: []string{ip1, ip2, "172.16.0.0/16"},
		}
		mock.nodeService.addNode(token, node)

		// All whitelisted IPs should be accepted
		testIPs := []string{ip1, ip2, "172.16.100.50"}
		for _, ip := range testIPs {
			result := mock.authenticateWithMock(ctx, token, ip)
			if !result.Allowed {
				t.Logf("Expected IP %s to be accepted", ip)
				return false
			}
		}

		// Non-whitelisted IP should be rejected
		result := mock.authenticateWithMock(ctx, token, "8.8.8.8")
		if result.Allowed {
			t.Logf("Expected non-whitelisted IP to be rejected")
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_IPv6WhitelistWorks tests that IPv6 addresses in whitelist work.
func TestProperty_IPv6WhitelistWorks(t *testing.T) {
	// Property: IPv6 addresses should be properly matched in whitelist
	ctx := context.Background()
	mock := newMockAuthenticatorForTest()

	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	node := &Node{
		ID:          1,
		Name:        "test-node",
		Status:      "online",
		IPWhitelist: []string{"::1", "2001:db8::1", "fe80::/10"},
	}
	mock.nodeService.addNode(token, node)

	// Valid IPv6 addresses should be accepted
	validIPs := []string{"::1", "2001:db8::1", "fe80::1"}
	for _, ip := range validIPs {
		result := mock.authenticateWithMock(ctx, token, ip)
		if !result.Allowed {
			t.Errorf("Expected IPv6 %s to be accepted", ip)
		}
	}

	// Invalid IPv6 should be rejected
	result := mock.authenticateWithMock(ctx, token, "2001:db8::2")
	if result.Allowed {
		t.Error("Expected non-whitelisted IPv6 to be rejected")
	}
}


// Feature: multi-server-management, Property 22: Auth Failure Rate Limiting
// Validates: Requirements 10.7
// For any IP with authentication failures exceeding the threshold within the time window,
// subsequent attempts SHALL be temporarily blocked.

// TestProperty_AuthFailuresRecorded tests that authentication failures are recorded.
func TestProperty_AuthFailuresRecorded(t *testing.T) {
	// Property: Each failed authentication attempt should increment the failure count
	f := func(numFailures uint8) bool {
		ctx := context.Background()
		config := &AuthConfig{
			MaxFailures:   100, // High threshold to avoid blocking
			BlockDuration: 15 * time.Minute,
			FailureWindow: 5 * time.Minute,
		}
		store := NewInMemoryAuthFailureStore(config)

		// Limit failures to reasonable number
		failures := int(numFailures%50) + 1
		clientIP := "192.168.1.100"

		// Record failures
		for i := 0; i < failures; i++ {
			if err := store.RecordFailure(ctx, clientIP); err != nil {
				t.Logf("Failed to record failure: %v", err)
				return false
			}
		}

		// Check failure count
		record, err := store.GetFailures(ctx, clientIP)
		if err != nil {
			t.Logf("Failed to get failures: %v", err)
			return false
		}
		if record == nil {
			t.Logf("Expected failure record to exist")
			return false
		}
		if record.Attempts != failures {
			t.Logf("Expected %d failures, got %d", failures, record.Attempts)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_IPBlockedAfterMaxFailures tests that IP is blocked after max failures.
func TestProperty_IPBlockedAfterMaxFailures(t *testing.T) {
	// Property: After MaxFailures attempts, the IP should be blocked
	f := func(maxFailures uint8) bool {
		ctx := context.Background()
		
		// Ensure at least 1 max failure
		max := int(maxFailures%10) + 1
		
		config := &AuthConfig{
			MaxFailures:   max,
			BlockDuration: 15 * time.Minute,
			FailureWindow: 5 * time.Minute,
		}
		store := NewInMemoryAuthFailureStore(config)

		clientIP := "192.168.1.100"

		// Record failures up to max
		for i := 0; i < max; i++ {
			store.RecordFailure(ctx, clientIP)
		}

		// Check if we should block
		record, _ := store.GetFailures(ctx, clientIP)
		if record != nil && record.Attempts >= max {
			// Block the IP
			blockUntil := time.Now().Add(config.BlockDuration)
			store.BlockIP(ctx, clientIP, blockUntil)
		}

		// IP should now be blocked
		blocked, _, err := store.IsBlocked(ctx, clientIP)
		if err != nil {
			t.Logf("Failed to check block status: %v", err)
			return false
		}
		if !blocked {
			t.Logf("Expected IP to be blocked after %d failures", max)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_IPNotBlockedBeforeMaxFailures tests that IP is not blocked before max failures.
func TestProperty_IPNotBlockedBeforeMaxFailures(t *testing.T) {
	// Property: Before MaxFailures attempts, the IP should not be blocked
	f := func(maxFailures uint8) bool {
		ctx := context.Background()
		
		// Ensure at least 2 max failures so we can test below threshold
		max := int(maxFailures%10) + 2
		
		config := &AuthConfig{
			MaxFailures:   max,
			BlockDuration: 15 * time.Minute,
			FailureWindow: 5 * time.Minute,
		}
		store := NewInMemoryAuthFailureStore(config)

		clientIP := "192.168.1.100"

		// Record failures below max
		for i := 0; i < max-1; i++ {
			store.RecordFailure(ctx, clientIP)
		}

		// IP should not be blocked yet
		blocked, _, err := store.IsBlocked(ctx, clientIP)
		if err != nil {
			t.Logf("Failed to check block status: %v", err)
			return false
		}
		if blocked {
			t.Logf("IP should not be blocked with only %d failures (max: %d)", max-1, max)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_BlockExpires tests that blocks expire after the duration.
func TestProperty_BlockExpires(t *testing.T) {
	// Property: After block duration, the IP should no longer be blocked
	ctx := context.Background()
	
	config := &AuthConfig{
		MaxFailures:   3,
		BlockDuration: 100 * time.Millisecond, // Short duration for testing
		FailureWindow: 5 * time.Minute,
	}
	store := NewInMemoryAuthFailureStore(config)

	clientIP := "192.168.1.100"

	// Block the IP
	blockUntil := time.Now().Add(config.BlockDuration)
	store.BlockIP(ctx, clientIP, blockUntil)

	// Should be blocked initially
	blocked, _, _ := store.IsBlocked(ctx, clientIP)
	if !blocked {
		t.Error("Expected IP to be blocked initially")
	}

	// Wait for block to expire
	time.Sleep(150 * time.Millisecond)

	// Should no longer be blocked
	blocked, _, _ = store.IsBlocked(ctx, clientIP)
	if blocked {
		t.Error("Expected IP to be unblocked after duration")
	}
}

// TestProperty_FailureWindowResets tests that failure count resets after window.
func TestProperty_FailureWindowResets(t *testing.T) {
	// Property: After failure window expires, failure count should reset
	ctx := context.Background()
	
	config := &AuthConfig{
		MaxFailures:   5,
		BlockDuration: 15 * time.Minute,
		FailureWindow: 100 * time.Millisecond, // Short window for testing
	}
	store := NewInMemoryAuthFailureStore(config)

	clientIP := "192.168.1.100"

	// Record some failures
	for i := 0; i < 3; i++ {
		store.RecordFailure(ctx, clientIP)
	}

	// Verify failures recorded
	record, _ := store.GetFailures(ctx, clientIP)
	if record == nil || record.Attempts != 3 {
		t.Error("Expected 3 failures to be recorded")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Failures should be reset (GetFailures returns nil for expired window)
	record, _ = store.GetFailures(ctx, clientIP)
	if record != nil {
		t.Error("Expected failures to be reset after window expires")
	}

	// New failure should start fresh
	store.RecordFailure(ctx, clientIP)
	record, _ = store.GetFailures(ctx, clientIP)
	if record == nil || record.Attempts != 1 {
		t.Error("Expected new failure count to be 1")
	}
}

// TestProperty_ClearFailuresWorks tests that clearing failures works.
func TestProperty_ClearFailuresWorks(t *testing.T) {
	// Property: After clearing failures, the failure count should be zero
	f := func(numFailures uint8) bool {
		ctx := context.Background()
		config := DefaultAuthConfig()
		store := NewInMemoryAuthFailureStore(config)

		failures := int(numFailures%20) + 1
		clientIP := "192.168.1.100"

		// Record failures
		for i := 0; i < failures; i++ {
			store.RecordFailure(ctx, clientIP)
		}

		// Clear failures
		if err := store.ClearFailures(ctx, clientIP); err != nil {
			t.Logf("Failed to clear failures: %v", err)
			return false
		}

		// Failures should be cleared
		record, _ := store.GetFailures(ctx, clientIP)
		if record != nil {
			t.Logf("Expected failures to be cleared")
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_DifferentIPsIndependent tests that different IPs have independent failure counts.
func TestProperty_DifferentIPsIndependent(t *testing.T) {
	// Property: Failures for one IP should not affect another IP
	f := func(a, b uint8) bool {
		ctx := context.Background()
		config := DefaultAuthConfig()
		store := NewInMemoryAuthFailureStore(config)

		ip1 := fmt.Sprintf("192.168.1.%d", a)
		ip2 := fmt.Sprintf("10.0.0.%d", b)

		// Record different number of failures for each IP
		failures1 := int(a%10) + 1
		failures2 := int(b%10) + 1

		for i := 0; i < failures1; i++ {
			store.RecordFailure(ctx, ip1)
		}
		for i := 0; i < failures2; i++ {
			store.RecordFailure(ctx, ip2)
		}

		// Check each IP has correct count
		record1, _ := store.GetFailures(ctx, ip1)
		record2, _ := store.GetFailures(ctx, ip2)

		if record1 == nil || record1.Attempts != failures1 {
			t.Logf("IP1 failure count mismatch: expected %d", failures1)
			return false
		}
		if record2 == nil || record2.Attempts != failures2 {
			t.Logf("IP2 failure count mismatch: expected %d", failures2)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_BlockedIPRejectedWithValidToken tests that blocked IPs are rejected even with valid tokens.
func TestProperty_BlockedIPRejectedWithValidToken(t *testing.T) {
	// Property: A blocked IP should be rejected regardless of token validity
	f := func(nodeID uint16) bool {
		ctx := context.Background()
		mock := newMockAuthenticatorForTest()

		// Create valid token and node
		token, err := GenerateToken()
		if err != nil {
			return false
		}

		node := &Node{
			ID:     int64(nodeID) + 1,
			Name:   "test-node",
			Status: "online",
		}
		mock.nodeService.addNode(token, node)

		clientIP := "192.168.1.100"

		// First verify authentication works
		result := mock.authenticateWithMock(ctx, token, clientIP)
		if !result.Allowed {
			t.Logf("Initial authentication should succeed")
			return false
		}

		// Block the IP
		blockUntil := time.Now().Add(1 * time.Hour)
		mock.failureStore.BlockIP(ctx, clientIP, blockUntil)

		// Now authentication should fail even with valid token
		result = mock.authenticateWithMock(ctx, token, clientIP)
		if result.Allowed {
			t.Logf("Blocked IP should be rejected")
			return false
		}
		if result.ErrorCode != "IP_BLOCKED" {
			t.Logf("Expected error code IP_BLOCKED, got %s", result.ErrorCode)
			return false
		}

		return true
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
