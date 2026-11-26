package repository

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupNodePreloadTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	if err := db.AutoMigrate(&Node{}, &NodeGroup{}, &NodeGroupMember{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

// TestNodeRepository_List_PreloadsGroups verifies that List preloads groups
func TestNodeRepository_List_PreloadsGroups(t *testing.T) {
	db := setupNodePreloadTestDB(t)
	repo := NewNodeRepository(db)
	groupRepo := NewNodeGroupRepository(db)
	ctx := context.Background()

	// Create groups
	group1 := &NodeGroup{Name: "Group 1", Description: "Test group 1"}
	if err := groupRepo.Create(ctx, group1); err != nil {
		t.Fatalf("Failed to create group 1: %v", err)
	}

	group2 := &NodeGroup{Name: "Group 2", Description: "Test group 2"}
	if err := groupRepo.Create(ctx, group2); err != nil {
		t.Fatalf("Failed to create group 2: %v", err)
	}

	// Create nodes
	node1 := &Node{Name: "Node 1", Address: "192.168.1.1", Port: 443, Status: "online", Token: "token1"}
	if err := repo.Create(ctx, node1); err != nil {
		t.Fatalf("Failed to create node 1: %v", err)
	}

	node2 := &Node{Name: "Node 2", Address: "192.168.1.2", Port: 443, Status: "online", Token: "token2"}
	if err := repo.Create(ctx, node2); err != nil {
		t.Fatalf("Failed to create node 2: %v", err)
	}

	// Add nodes to groups
	if err := groupRepo.AddNode(ctx, group1.ID, node1.ID); err != nil {
		t.Fatalf("Failed to add node 1 to group 1: %v", err)
	}

	if err := groupRepo.AddNode(ctx, group2.ID, node1.ID); err != nil {
		t.Fatalf("Failed to add node 1 to group 2: %v", err)
	}

	if err := groupRepo.AddNode(ctx, group1.ID, node2.ID); err != nil {
		t.Fatalf("Failed to add node 2 to group 1: %v", err)
	}

	// List nodes
	nodes, err := repo.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}

	if len(nodes) != 2 {
		t.Fatalf("Expected 2 nodes, got %d", len(nodes))
	}

	// Verify node 1 has 2 groups preloaded
	if len(nodes[0].Groups) != 2 {
		t.Errorf("Expected node 1 to have 2 groups, got %d", len(nodes[0].Groups))
	}

	// Verify node 2 has 1 group preloaded
	if len(nodes[1].Groups) != 1 {
		t.Errorf("Expected node 2 to have 1 group, got %d", len(nodes[1].Groups))
	}

	// Verify group IDs are correct
	groupIDs := make(map[int64]bool)
	for _, group := range nodes[0].Groups {
		groupIDs[group.ID] = true
	}

	if !groupIDs[group1.ID] || !groupIDs[group2.ID] {
		t.Errorf("Node 1 should belong to both groups")
	}
}

// TestNodeRepository_GetByID_PreloadsGroups verifies that GetByID preloads groups
func TestNodeRepository_GetByID_PreloadsGroups(t *testing.T) {
	db := setupNodePreloadTestDB(t)
	repo := NewNodeRepository(db)
	groupRepo := NewNodeGroupRepository(db)
	ctx := context.Background()

	// Create group
	group := &NodeGroup{Name: "Test Group", Description: "Test group"}
	if err := groupRepo.Create(ctx, group); err != nil {
		t.Fatalf("Failed to create group: %v", err)
	}

	// Create node
	node := &Node{Name: "Test Node", Address: "192.168.1.1", Port: 443, Status: "online", Token: "test-token"}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	// Add node to group
	if err := groupRepo.AddNode(ctx, group.ID, node.ID); err != nil {
		t.Fatalf("Failed to add node to group: %v", err)
	}

	// Get node by ID
	fetchedNode, err := repo.GetByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("Failed to get node by ID: %v", err)
	}

	// Verify groups are preloaded
	if len(fetchedNode.Groups) != 1 {
		t.Errorf("Expected node to have 1 group, got %d", len(fetchedNode.Groups))
	}

	if fetchedNode.Groups[0].ID != group.ID {
		t.Errorf("Expected group ID %d, got %d", group.ID, fetchedNode.Groups[0].ID)
	}
}
