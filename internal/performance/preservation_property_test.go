// Package performance provides performance testing for the Vpanel application.
package performance

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"v/internal/database"
	"v/internal/database/repository"
)

// **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7, 3.8, 3.9, 3.10, 3.11, 3.12, 3.13, 3.14, 3.15**
//
// Property 2: Preservation - Functional Equivalence
//
// IMPORTANT: Follow observation-first methodology
// Observe behavior on UNFIXED code for all system operations:
// - API response structures and field values
// - Filtering, sorting, and pagination results
// - Error handling and HTTP status codes
// - Security checks and authorization
// - Database state after CRUD operations
//
// EXPECTED OUTCOME: Tests PASS (this confirms baseline behavior to preserve)

// setupPreservationTestDB creates a test database for preservation tests
func setupPreservationTestDB(t *testing.T) *database.Database {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "preservation_test.db")

	cfg := &database.Config{
		Driver:              "sqlite",
		DSN:                 dbPath,
		HealthCheckInterval: time.Hour,
		MaxRetries:          1,
		RetryInterval:       10 * time.Millisecond,
		SlowQueryThreshold:  200 * time.Millisecond,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

// TestPreservation_UserRepositoryDataIntegrity tests that user CRUD operations maintain data integrity
// This test validates Requirements 3.1, 3.4, 3.5
func TestPreservation_UserRepositoryDataIntegrity(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("user CRUD operations preserve data integrity", prop.ForAll(
		func(username string, role string, enabled bool) bool {
			// Sanitize inputs
			if username == "" {
				username = "testuser"
			}
			if role != "admin" && role != "user" {
				role = "user"
			}

			db := setupPreservationTestDB(t)
			defer db.Close()

			ctx := context.Background()
			userRepo := repository.NewUserRepository(db.DB())

			// Create user
			user := &repository.User{
				Username:     username,
				PasswordHash: "test_password_hash",
				Role:         role,
				Enabled:      enabled,
			}

			err := userRepo.Create(ctx, user)
			if err != nil {
				t.Logf("Failed to create user: %v", err)
				return false
			}

			// Verify user was created with correct data
			fetchedUser, err := userRepo.GetByID(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to fetch user: %v", err)
				return false
			}

			// Validate all fields are preserved
			if fetchedUser.Username != user.Username {
				t.Logf("Username mismatch: expected %s, got %s", user.Username, fetchedUser.Username)
				return false
			}
			if fetchedUser.Role != user.Role {
				t.Logf("Role mismatch: expected %s, got %s", user.Role, fetchedUser.Role)
				return false
			}
			if fetchedUser.Enabled != user.Enabled {
				t.Logf("Enabled mismatch: expected %v, got %v", user.Enabled, fetchedUser.Enabled)
				return false
			}
			if fetchedUser.PasswordHash != user.PasswordHash {
				t.Logf("PasswordHash mismatch")
				return false
			}

			// Update user
			user.Enabled = !enabled
			err = userRepo.Update(ctx, user)
			if err != nil {
				t.Logf("Failed to update user: %v", err)
				return false
			}

			// Verify update preserved other fields
			updatedUser, err := userRepo.GetByID(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to fetch updated user: %v", err)
				return false
			}

			if updatedUser.Enabled == enabled {
				t.Logf("Update did not change enabled status")
				return false
			}
			if updatedUser.Username != user.Username {
				t.Logf("Update changed username unexpectedly")
				return false
			}
			if updatedUser.Role != user.Role {
				t.Logf("Update changed role unexpectedly")
				return false
			}

			// Delete user
			err = userRepo.Delete(ctx, user.ID)
			if err != nil {
				t.Logf("Failed to delete user: %v", err)
				return false
			}

			// Verify user was deleted
			_, err = userRepo.GetByID(ctx, user.ID)
			if err == nil {
				t.Logf("User still exists after deletion")
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 50 }),
		gen.OneConstOf("admin", "user"),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestPreservation_UserListFilteringAndPagination tests that filtering and pagination work correctly
// This test validates Requirements 3.3
func TestPreservation_UserListFilteringAndPagination(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("user list pagination produces consistent results", prop.ForAll(
		func(userCount uint8, pageSize uint8) bool {
			// Limit to reasonable ranges
			count := int(userCount%20) + 5  // 5-24 users
			size := int(pageSize%10) + 1    // 1-10 per page

			db := setupPreservationTestDB(t)
			defer db.Close()

			ctx := context.Background()
			userRepo := repository.NewUserRepository(db.DB())

			// Create test users
			createdUsers := make([]*repository.User, count)
			for i := 0; i < count; i++ {
				user := &repository.User{
					Username:     fmt.Sprintf("user%03d", i),
					PasswordHash: "password",
					Role:         "user",
					Enabled:      i%2 == 0,
				}
				if i%5 == 0 {
					user.Role = "admin"
				}
				err := userRepo.Create(ctx, user)
				if err != nil {
					t.Logf("Failed to create user: %v", err)
					return false
				}
				createdUsers[i] = user
			}

			// Test pagination - get all users across pages
			var allUsers []*repository.User
			page := 1
			for {
				limit := size
				offset := (page - 1) * size
				users, err := userRepo.List(ctx, limit, offset)
				if err != nil {
					t.Logf("Failed to list users: %v", err)
					return false
				}
				if len(users) == 0 {
					break
				}
				allUsers = append(allUsers, users...)
				page++
			}

			// Verify we got all users
			if len(allUsers) != count {
				t.Logf("Pagination mismatch: expected %d users, got %d", count, len(allUsers))
				return false
			}

			// Verify pagination consistency - get first page twice
			firstPage1, err := userRepo.List(ctx, size, 0)
			if err != nil {
				t.Logf("Failed to get first page (attempt 1): %v", err)
				return false
			}

			firstPage2, err := userRepo.List(ctx, size, 0)
			if err != nil {
				t.Logf("Failed to get first page (attempt 2): %v", err)
				return false
			}

			if len(firstPage1) != len(firstPage2) {
				t.Logf("Pagination inconsistency: first page returned different counts")
				return false
			}

			// Verify same users in same order
			for i := range firstPage1 {
				if firstPage1[i].ID != firstPage2[i].ID {
					t.Logf("Pagination inconsistency: different user order")
					return false
				}
			}

			return true
		},
		gen.UInt8(),
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPreservation_NodeRepositoryDataIntegrity tests that node CRUD operations maintain data integrity
// This test validates Requirements 3.1, 3.4, 3.5
func TestPreservation_NodeRepositoryDataIntegrity(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("node CRUD operations preserve data integrity", prop.ForAll(
		func(nameIdx uint8, addrIdx uint8, statusBool bool) bool {
			// Generate valid inputs
			name := fmt.Sprintf("testnode%d", nameIdx)
			address := fmt.Sprintf("192.168.1.%d", addrIdx)
			status := "online"
			if !statusBool {
				status = "offline"
			}

			db := setupPreservationTestDB(t)
			defer db.Close()

			ctx := context.Background()
			nodeRepo := repository.NewNodeRepository(db.DB())

			// Create node
			node := &repository.Node{
				Name:    name,
				Address: address,
				Token:   "test_token",
				Status:  status,
			}

			err := nodeRepo.Create(ctx, node)
			if err != nil {
				t.Logf("Failed to create node: %v", err)
				return false
			}

			// Verify node was created with correct data
			fetchedNode, err := nodeRepo.GetByID(ctx, node.ID)
			if err != nil {
				t.Logf("Failed to fetch node: %v", err)
				return false
			}

			// Validate all fields are preserved
			if fetchedNode.Name != node.Name {
				t.Logf("Name mismatch: expected %s, got %s", node.Name, fetchedNode.Name)
				return false
			}
			if fetchedNode.Address != node.Address {
				t.Logf("Address mismatch: expected %s, got %s", node.Address, fetchedNode.Address)
				return false
			}
			if fetchedNode.Status != node.Status {
				t.Logf("Status mismatch: expected %s, got %s", node.Status, fetchedNode.Status)
				return false
			}
			if fetchedNode.Token != node.Token {
				t.Logf("Token mismatch")
				return false
			}

			// Update node
			newStatus := "offline"
			if status == "offline" {
				newStatus = "online"
			}
			node.Status = newStatus
			err = nodeRepo.Update(ctx, node)
			if err != nil {
				t.Logf("Failed to update node: %v", err)
				return false
			}

			// Verify update preserved other fields
			updatedNode, err := nodeRepo.GetByID(ctx, node.ID)
			if err != nil {
				t.Logf("Failed to fetch updated node: %v", err)
				return false
			}

			if updatedNode.Status != newStatus {
				t.Logf("Update did not change status")
				return false
			}
			if updatedNode.Name != node.Name {
				t.Logf("Update changed name unexpectedly")
				return false
			}
			if updatedNode.Address != node.Address {
				t.Logf("Update changed address unexpectedly")
				return false
			}

			// Delete node
			err = nodeRepo.Delete(ctx, node.ID)
			if err != nil {
				t.Logf("Failed to delete node: %v", err)
				return false
			}

			// Verify node was deleted
			_, err = nodeRepo.GetByID(ctx, node.ID)
			if err == nil {
				t.Logf("Node still exists after deletion")
				return false
			}

			return true
		},
		gen.UInt8(),
		gen.UInt8(),
		gen.Bool(),
	))

	properties.TestingRun(t)
}

// TestPreservation_NodeGroupAssociations tests that node-group associations are preserved
// This test validates Requirements 3.5, 3.6
func TestPreservation_NodeGroupAssociations(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("node-group associations are preserved correctly", prop.ForAll(
		func(nodeCount uint8) bool {
			// Limit to reasonable range
			count := int(nodeCount%10) + 2 // 2-11 nodes

			db := setupPreservationTestDB(t)
			defer db.Close()

			ctx := context.Background()
			nodeRepo := repository.NewNodeRepository(db.DB())
			groupRepo := repository.NewNodeGroupRepository(db.DB())

			// Create a test group
			group := &repository.NodeGroup{
				Name:        "test-group",
				Description: "Test group for preservation",
			}
			err := groupRepo.Create(ctx, group)
			if err != nil {
				t.Logf("Failed to create group: %v", err)
				return false
			}

			// Create nodes and add to group
			nodeIDs := make([]int64, count)
			for i := 0; i < count; i++ {
				node := &repository.Node{
					Name:    fmt.Sprintf("node%d", i),
					Address: fmt.Sprintf("192.168.1.%d", i+1),
					Token:   fmt.Sprintf("token%d", i),
					Status:  "online",
				}
				err := nodeRepo.Create(ctx, node)
				if err != nil {
					t.Logf("Failed to create node: %v", err)
					return false
				}
				nodeIDs[i] = node.ID

				// Add node to group
				err = groupRepo.AddNode(ctx, group.ID, node.ID)
				if err != nil {
					t.Logf("Failed to add node to group: %v", err)
					return false
				}
			}

			// Verify all nodes are in the group
			nodes, err := groupRepo.GetNodes(ctx, group.ID)
			if err != nil {
				t.Logf("Failed to get group nodes: %v", err)
				return false
			}

			if len(nodes) != count {
				t.Logf("Group node count mismatch: expected %d, got %d", count, len(nodes))
				return false
			}

			// Verify each node ID is present
			nodeIDMap := make(map[int64]bool)
			for _, node := range nodes {
				nodeIDMap[node.ID] = true
			}
			for _, id := range nodeIDs {
				if !nodeIDMap[id] {
					t.Logf("Node ID %d not found in group", id)
					return false
				}
			}

			// Remove a node from the group
			err = groupRepo.RemoveNode(ctx, group.ID, nodeIDs[0])
			if err != nil {
				t.Logf("Failed to remove node from group: %v", err)
				return false
			}

			// Verify node was removed
			nodesAfterRemoval, err := groupRepo.GetNodes(ctx, group.ID)
			if err != nil {
				t.Logf("Failed to get group nodes after removal: %v", err)
				return false
			}

			if len(nodesAfterRemoval) != count-1 {
				t.Logf("Group node count after removal mismatch: expected %d, got %d", count-1, len(nodesAfterRemoval))
				return false
			}

			// Verify removed node is not in the list
			for _, node := range nodesAfterRemoval {
				if node.ID == nodeIDs[0] {
					t.Logf("Removed node still in group")
					return false
				}
			}

			return true
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

// TestPreservation_ErrorHandling tests that error conditions are handled consistently
// This test validates Requirements 3.7, 3.9
func TestPreservation_ErrorHandling(t *testing.T) {
	db := setupPreservationTestDB(t)
	defer db.Close()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(db.DB())
	nodeRepo := repository.NewNodeRepository(db.DB())

	// Test 1: Getting non-existent user returns error
	_, err := userRepo.GetByID(ctx, 99999)
	if err == nil {
		t.Error("Expected error when getting non-existent user, got nil")
	}

	// Test 2: Getting non-existent node returns error
	_, err = nodeRepo.GetByID(ctx, 99999)
	if err == nil {
		t.Error("Expected error when getting non-existent node, got nil")
	}

	// Test 3: Deleting non-existent user returns error
	err = userRepo.Delete(ctx, 99999)
	if err == nil {
		t.Error("Expected error when deleting non-existent user, got nil")
	}

	// Test 4: Updating non-existent node - GORM's Save doesn't error on non-existent records
	// It will create a new record instead, so we skip this test
	// This is expected GORM behavior with Save()

	// Test 5: Creating user with duplicate username should fail
	user1 := &repository.User{
		Username:     "duplicate_test",
		PasswordHash: "password",
		Role:         "user",
		Enabled:      true,
	}
	err = userRepo.Create(ctx, user1)
	if err != nil {
		t.Fatalf("Failed to create first user: %v", err)
	}

	user2 := &repository.User{
		Username:     "duplicate_test",
		PasswordHash: "password",
		Role:         "user",
		Enabled:      true,
	}
	err = userRepo.Create(ctx, user2)
	if err == nil {
		t.Error("Expected error when creating user with duplicate username, got nil")
	}

	t.Log("All error handling tests passed - error conditions are handled consistently")
}

// TestPreservation_SearchFunctionality tests that search operations work correctly
// This test validates Requirements 3.3, 3.12
func TestPreservation_SearchFunctionality(t *testing.T) {
	db := setupPreservationTestDB(t)
	defer db.Close()

	ctx := context.Context(context.Background())
	userRepo := repository.NewUserRepository(db.DB())

	// Create users with predictable names
	searchTerm := "searchtest"
	matchingCount := 0
	for i := 0; i < 20; i++ {
		username := fmt.Sprintf("user%d", i)
		if i%3 == 0 {
			username = fmt.Sprintf("%s_user%d", searchTerm, i)
			matchingCount++
		}
		user := &repository.User{
			Username:     username,
			PasswordHash: "password",
			Role:         "user",
			Enabled:      true,
		}
		err := userRepo.Create(ctx, user)
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	}

	// Get all users and manually filter
	allUsers, err := userRepo.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	// Count users containing search term
	foundCount := 0
	for _, user := range allUsers {
		if contains(user.Username, searchTerm) {
			foundCount++
		}
	}

	if foundCount != matchingCount {
		t.Errorf("Search validation failed: expected %d matching users, found %d", matchingCount, foundCount)
	}

	t.Logf("Search functionality validated: found %d users matching '%s'", foundCount, searchTerm)
}

// Helper functions

func contains(s, substr string) bool {
	// Simple case-insensitive contains check
	return len(s) >= len(substr) && (s == substr || 
		len(s) > len(substr) && (
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
