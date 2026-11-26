package repository

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// queryCounter counts the number of SQL queries executed
type queryCounter struct {
	count int64
}

func (qc *queryCounter) LogMode(level logger.LogLevel) logger.Interface {
	return qc
}

func (qc *queryCounter) Info(ctx context.Context, msg string, data ...interface{}) {}

func (qc *queryCounter) Warn(ctx context.Context, msg string, data ...interface{}) {}

func (qc *queryCounter) Error(ctx context.Context, msg string, data ...interface{}) {}

func (qc *queryCounter) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	atomic.AddInt64(&qc.count, 1)
}

func (qc *queryCounter) getCount() int64 {
	return atomic.LoadInt64(&qc.count)
}

func (qc *queryCounter) reset() {
	atomic.StoreInt64(&qc.count, 0)
}

// TestNodeRepository_List_QueryCount verifies that List executes minimal queries
func TestNodeRepository_List_QueryCount(t *testing.T) {
	counter := &queryCounter{}
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: counter,
	})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create tables
	if err := db.AutoMigrate(&Node{}, &NodeGroup{}, &NodeGroupMember{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

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

	// Create 10 nodes
	for i := 0; i < 10; i++ {
		node := &Node{
			Name:    fmt.Sprintf("Node %d", i+1),
			Address: fmt.Sprintf("192.168.1.%d", i+1),
			Port:    443,
			Status:  "online",
			Token:   fmt.Sprintf("token-%d", i+1),
		}
		if err := repo.Create(ctx, node); err != nil {
			t.Fatalf("Failed to create node %d: %v", i+1, err)
		}

		// Add each node to both groups
		if err := groupRepo.AddNode(ctx, group1.ID, node.ID); err != nil {
			t.Fatalf("Failed to add node %d to group 1: %v", i+1, err)
		}
		if err := groupRepo.AddNode(ctx, group2.ID, node.ID); err != nil {
			t.Fatalf("Failed to add node %d to group 2: %v", i+1, err)
		}
	}

	// Reset counter before the test
	counter.reset()

	// List nodes with Preload
	nodes, err := repo.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list nodes: %v", err)
	}

	if len(nodes) != 10 {
		t.Fatalf("Expected 10 nodes, got %d", len(nodes))
	}

	// Get query count
	queryCount := counter.getCount()

	// With Preload, we should execute:
	// 1. SELECT nodes
	// 2. SELECT groups with JOIN node_group_members
	// Total: 2 queries (or 3 at most depending on GORM implementation)
	//
	// Without Preload (N+1 pattern), we would execute:
	// 1. SELECT nodes
	// 2-11. SELECT groups for each node (10 queries)
	// Total: 11 queries
	if queryCount > 3 {
		t.Errorf("Expected at most 3 queries with Preload, got %d queries", queryCount)
		t.Logf("This indicates N+1 query problem is not fixed")
	} else {
		t.Logf("Query count: %d (optimized with Preload)", queryCount)
	}

	// Verify all nodes have groups loaded
	for i, node := range nodes {
		if len(node.Groups) != 2 {
			t.Errorf("Node %d should have 2 groups, got %d", i+1, len(node.Groups))
		}
	}
}
