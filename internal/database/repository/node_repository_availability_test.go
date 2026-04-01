package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupNodeRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&Node{}); err != nil {
		t.Fatalf("failed to migrate node table: %v", err)
	}

	return db
}

func TestNodeRepository_GetAvailable_ExcludesNodesPastTrafficThreshold(t *testing.T) {
	db := setupNodeRepositoryTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	nodes := []*Node{
		{ID: 1, Name: "healthy", Address: "1.1.1.1", Token: "token-1", Status: NodeStatusOnline, TrafficLimit: 1000, TrafficTotal: 790, AlertTrafficThreshold: 80},
		{ID: 2, Name: "soft-capped", Address: "2.2.2.2", Token: "token-2", Status: NodeStatusOnline, TrafficLimit: 1000, TrafficTotal: 800, AlertTrafficThreshold: 80},
		{ID: 3, Name: "unlimited", Address: "3.3.3.3", Token: "token-3", Status: NodeStatusOnline, TrafficLimit: 0, TrafficTotal: 999999, AlertTrafficThreshold: 80},
	}
	for _, node := range nodes {
		if err := repo.Create(ctx, node); err != nil {
			t.Fatalf("failed to create node %d: %v", node.ID, err)
		}
	}

	available, err := repo.GetAvailable(ctx)
	if err != nil {
		t.Fatalf("GetAvailable returned error: %v", err)
	}
	if len(available) != 2 {
		t.Fatalf("expected 2 available nodes, got %d", len(available))
	}

	availableIDs := map[int64]struct{}{}
	for _, node := range available {
		availableIDs[node.ID] = struct{}{}
	}
	if _, ok := availableIDs[1]; !ok {
		t.Fatalf("expected healthy node to remain available")
	}
	if _, ok := availableIDs[3]; !ok {
		t.Fatalf("expected unlimited node to remain available")
	}
	if _, ok := availableIDs[2]; ok {
		t.Fatalf("expected soft-capped node to be excluded")
	}
}

func TestNodeRepository_GetAvailable_PrioritizesLowerTrafficPressure(t *testing.T) {
	db := setupNodeRepositoryTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	nodes := []*Node{
		{ID: 11, Name: "higher-usage", Address: "11.11.11.11", Token: "token-11", Status: NodeStatusOnline, CurrentUsers: 1, TrafficLimit: 1000, TrafficTotal: 700, AlertTrafficThreshold: 80},
		{ID: 12, Name: "lower-usage", Address: "12.12.12.12", Token: "token-12", Status: NodeStatusOnline, CurrentUsers: 1, TrafficLimit: 1000, TrafficTotal: 200, AlertTrafficThreshold: 80},
		{ID: 13, Name: "unlimited", Address: "13.13.13.13", Token: "token-13", Status: NodeStatusOnline, CurrentUsers: 3, TrafficLimit: 0, TrafficTotal: 999999, AlertTrafficThreshold: 80},
	}
	for _, node := range nodes {
		if err := repo.Create(ctx, node); err != nil {
			t.Fatalf("failed to create node %d: %v", node.ID, err)
		}
	}

	available, err := repo.GetAvailable(ctx)
	if err != nil {
		t.Fatalf("GetAvailable returned error: %v", err)
	}
	if len(available) != 3 {
		t.Fatalf("expected 3 available nodes, got %d", len(available))
	}
	if available[0].ID != 13 {
		t.Fatalf("expected unlimited node to be preferred first, got %d", available[0].ID)
	}
	if available[1].ID != 12 {
		t.Fatalf("expected lower-usage node second, got %d", available[1].ID)
	}
	if available[2].ID != 11 {
		t.Fatalf("expected higher-usage node last, got %d", available[2].ID)
	}
}

func TestNodeRepository_ListAndCount_WithSearchFilter(t *testing.T) {
	db := setupNodeRepositoryTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	fixtures := []*Node{
		{ID: 21, Name: "Tokyo Edge", Address: "jp.example.com", Region: "Japan", Token: "token-21", Status: NodeStatusOnline},
		{ID: 22, Name: "Singapore Core", Address: "sg.example.com", Region: "Singapore", Token: "token-22", Status: NodeStatusOnline},
		{ID: 23, Name: "US West", Address: "us.example.com", Region: "United States", Token: "token-23", Status: NodeStatusOffline},
	}
	for _, node := range fixtures {
		if err := repo.Create(ctx, node); err != nil {
			t.Fatalf("failed to create node %d: %v", node.ID, err)
		}
	}

	filter := &NodeFilter{Search: "singapore", Limit: 50}
	nodes, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != 22 {
		t.Fatalf("expected node 22 to match search, got %d", nodes[0].ID)
	}

	count, err := repo.Count(ctx, filter)
	if err != nil {
		t.Fatalf("Count returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	addressFilter := &NodeFilter{Search: "us.example.com", Limit: 50}
	addressCount, err := repo.Count(ctx, addressFilter)
	if err != nil {
		t.Fatalf("Count with address filter returned error: %v", err)
	}
	if addressCount != 1 {
		t.Fatalf("expected address count 1, got %d", addressCount)
	}
}
