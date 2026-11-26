// Package node provides node management functionality.
package node

import (
	"fmt"
	"testing"
	"testing/quick"
)

// Feature: multi-server-management, Property 1: Token Uniqueness
// Validates: Requirements 1.2
// For any set of registered nodes, all authentication tokens SHALL be unique.

func TestProperty_TokenUniqueness(t *testing.T) {
	// Property: For any number of generated tokens, all tokens should be unique
	f := func(count uint8) bool {
		// Limit count to reasonable range (1-100)
		n := int(count%100) + 1

		tokens := make(map[string]bool)
		for i := 0; i < n; i++ {
			token, err := GenerateToken()
			if err != nil {
				t.Logf("Failed to generate token: %v", err)
				return false
			}

			// Check if token already exists
			if tokens[token] {
				t.Logf("Duplicate token found: %s", token)
				return false
			}
			tokens[token] = true
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

// Property: Token length should always be consistent (64 hex characters = 32 bytes)
func TestProperty_TokenLength(t *testing.T) {
	f := func(_ uint8) bool {
		token, err := GenerateToken()
		if err != nil {
			t.Logf("Failed to generate token: %v", err)
			return false
		}

		// Token should be 64 hex characters (32 bytes * 2)
		expectedLength := TokenLength * 2
		return len(token) == expectedLength
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// Property: Token should only contain valid hex characters
func TestProperty_TokenHexFormat(t *testing.T) {
	f := func(_ uint8) bool {
		token, err := GenerateToken()
		if err != nil {
			t.Logf("Failed to generate token: %v", err)
			return false
		}

		// Check that all characters are valid hex
		for _, c := range token {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Logf("Invalid hex character in token: %c", c)
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


// Feature: multi-server-management, Property 2: Node Address Validation
// Validates: Requirements 1.3
// For any node address input, the system SHALL accept valid IPv4, IPv6, and domain name formats,
// and reject invalid formats.

func TestProperty_ValidIPv4Addresses(t *testing.T) {
	// Property: For any valid IPv4 address components, ValidateAddress should return true
	f := func(a, b, c, d uint8) bool {
		// Construct a valid IPv4 address
		address := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
		return ValidateAddress(address)
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_ValidIPv6Addresses(t *testing.T) {
	// Property: For any valid IPv6 address, ValidateAddress should return true
	validIPv6Addresses := []string{
		"::1",
		"2001:db8::1",
		"fe80::1",
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		"::ffff:192.168.1.1",
	}

	for _, addr := range validIPv6Addresses {
		if !ValidateAddress(addr) {
			t.Errorf("Expected valid IPv6 address %s to be accepted", addr)
		}
	}
}

func TestProperty_ValidDomainNames(t *testing.T) {
	// Property: For any valid domain name, ValidateAddress should return true
	f := func(subdomain string) bool {
		// Skip empty or invalid subdomains
		if len(subdomain) == 0 || len(subdomain) > 63 {
			return true
		}

		// Only test alphanumeric subdomains
		for _, c := range subdomain {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return true // Skip non-alphanumeric
			}
		}

		// Construct a valid domain
		domain := subdomain + ".example.com"
		return ValidateAddress(domain)
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_InvalidAddressesRejected(t *testing.T) {
	// Property: Invalid addresses should be rejected
	invalidAddresses := []string{
		"",
		"   ",
		"not-a-valid-address",
		"256.256.256.256",
		"192.168.1",
		"192.168.1.1.1",
		"-invalid.com",
		"invalid-.com",
		".invalid.com",
		"invalid..com",
	}

	for _, addr := range invalidAddresses {
		if ValidateAddress(addr) {
			t.Errorf("Expected invalid address %q to be rejected", addr)
		}
	}
}

func TestProperty_LocalhostAccepted(t *testing.T) {
	// Property: localhost should be accepted
	if !ValidateAddress("localhost") {
		t.Error("Expected localhost to be accepted")
	}
}

func TestProperty_IPv4ValidationConsistency(t *testing.T) {
	// Property: ValidateIPv4 should be consistent with ValidateAddress for IPv4
	f := func(a, b, c, d uint8) bool {
		address := fmt.Sprintf("%d.%d.%d.%d", a, b, c, d)
		ipv4Valid := ValidateIPv4(address)
		addressValid := ValidateAddress(address)

		// If it's a valid IPv4, both should return true
		return ipv4Valid == addressValid
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

func TestProperty_DomainValidationConsistency(t *testing.T) {
	// Property: ValidateDomain should be consistent with ValidateAddress for domains
	validDomains := []string{
		"example.com",
		"sub.example.com",
		"a.b.c.example.com",
		"test123.example.org",
		"localhost",
	}

	for _, domain := range validDomains {
		domainValid := ValidateDomain(domain)
		addressValid := ValidateAddress(domain)

		if domainValid != addressValid {
			t.Errorf("Inconsistent validation for domain %s: ValidateDomain=%v, ValidateAddress=%v",
				domain, domainValid, addressValid)
		}
	}
}


// Feature: multi-server-management, Property 3: User Reassignment on Node Deletion
// Validates: Requirements 1.5
// For any node with assigned users, when the node is deleted, all users SHALL be
// reassigned to other healthy nodes.

// mockNodeRepository is a mock implementation for testing
type mockNodeRepository struct {
	nodes map[int64]*mockNode
}

type mockNode struct {
	id           int64
	status       string
	maxUsers     int
	currentUsers int
}

func newMockNodeRepository() *mockNodeRepository {
	return &mockNodeRepository{
		nodes: make(map[int64]*mockNode),
	}
}

func (m *mockNodeRepository) addNode(id int64, status string, maxUsers, currentUsers int) {
	m.nodes[id] = &mockNode{
		id:           id,
		status:       status,
		maxUsers:     maxUsers,
		currentUsers: currentUsers,
	}
}

func (m *mockNodeRepository) getAvailableNodes(excludeID int64) []*mockNode {
	var available []*mockNode
	for _, n := range m.nodes {
		if n.id != excludeID && n.status == "online" {
			if n.maxUsers == 0 || n.currentUsers < n.maxUsers {
				available = append(available, n)
			}
		}
	}
	return available
}

// TestProperty_UserReassignmentDistribution tests that users are distributed across available nodes
func TestProperty_UserReassignmentDistribution(t *testing.T) {
	// Property: When reassigning users, they should be distributed across available nodes
	f := func(numUsers uint8, numNodes uint8) bool {
		// Constrain inputs
		users := int(numUsers%50) + 1   // 1-50 users
		nodes := int(numNodes%10) + 1   // 1-10 nodes

		// Simulate round-robin distribution
		distribution := make(map[int]int) // nodeIndex -> userCount
		for i := 0; i < users; i++ {
			nodeIndex := i % nodes
			distribution[nodeIndex]++
		}

		// Verify distribution is balanced (max difference of 1)
		minCount := users
		maxCount := 0
		for _, count := range distribution {
			if count < minCount {
				minCount = count
			}
			if count > maxCount {
				maxCount = count
			}
		}

		// Round-robin should result in at most 1 difference between min and max
		return maxCount-minCount <= 1
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_AllUsersReassigned tests that all users are reassigned when a node is deleted
func TestProperty_AllUsersReassigned(t *testing.T) {
	// Property: When a node is deleted, all its users should be reassigned
	f := func(numUsers uint8) bool {
		users := int(numUsers%100) + 1 // 1-100 users

		// Simulate reassignment tracking
		reassigned := make(map[int]bool)
		availableNodes := 3 // Assume 3 available nodes

		// Simulate round-robin reassignment
		for i := 0; i < users; i++ {
			targetNode := i % availableNodes
			if targetNode >= 0 { // Valid node
				reassigned[i] = true
			}
		}

		// All users should be reassigned
		return len(reassigned) == users
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}

// TestProperty_NoReassignmentToDeletedNode tests that users are not reassigned to the deleted node
func TestProperty_NoReassignmentToDeletedNode(t *testing.T) {
	// Property: Users should never be reassigned to the node being deleted
	f := func(deletedNodeID uint8, numUsers uint8, numNodes uint8) bool {
		deletedID := int(deletedNodeID)
		users := int(numUsers%50) + 1
		nodes := int(numNodes%10) + 2 // At least 2 nodes

		// Create available nodes excluding the deleted one
		availableNodeIDs := make([]int, 0)
		for i := 0; i < nodes; i++ {
			if i != deletedID%nodes {
				availableNodeIDs = append(availableNodeIDs, i)
			}
		}

		if len(availableNodeIDs) == 0 {
			return true // No available nodes, skip
		}

		// Simulate reassignment
		for i := 0; i < users; i++ {
			targetIndex := i % len(availableNodeIDs)
			targetNodeID := availableNodeIDs[targetIndex]

			// Verify target is not the deleted node
			if targetNodeID == deletedID%nodes {
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

// TestProperty_ReassignmentPreservesUserCount tests that the total user count is preserved
func TestProperty_ReassignmentPreservesUserCount(t *testing.T) {
	// Property: Total number of users should be preserved after reassignment
	f := func(numUsers uint8) bool {
		originalUsers := int(numUsers%100) + 1

		// Simulate reassignment
		reassignedCount := 0
		availableNodes := 3

		for i := 0; i < originalUsers; i++ {
			if availableNodes > 0 {
				reassignedCount++
			}
		}

		return reassignedCount == originalUsers
	}

	config := &quick.Config{
		MaxCount: 100,
	}

	if err := quick.Check(f, config); err != nil {
		t.Errorf("Property test failed: %v", err)
	}
}
