package repository

import (
	"context"
	"testing"
	"time"

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

func TestNodeRepositorySaveDoesNotRestoreRevokedToken(t *testing.T) {
	db := setupNodeRepositoryTestDB(t)
	repo := NewNodeRepository(db)
	ctx := t.Context()
	node := &Node{Name: "node", Address: "192.0.2.1", Token: "old-test-token", Status: NodeStatusOnline}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateToken(ctx, node.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateStatus(ctx, node.ID, NodeStatusOffline); err != nil {
		t.Fatal(err)
	}
	heartbeatAt := time.Now().UTC().Truncate(time.Second)
	if err := repo.UpdateLastSeen(ctx, node.ID, heartbeatAt); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateXrayStatus(ctx, node.ID, true, "test-version"); err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkConfigApplied(ctx, node.ID, "test-config", heartbeatAt); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(node).Updates(map[string]any{"traffic_reset_at": heartbeatAt, "traffic_total": 42}).Error; err != nil {
		t.Fatal(err)
	}
	// Keep the original snapshot used by a concurrent settings edit.
	node.TrafficResetAt = nil
	node.TrafficTotal = 0
	node.Name = "renamed-node"
	if err := repo.Update(ctx, node); err != nil {
		t.Fatal(err)
	}

	saved, err := repo.GetByID(ctx, node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != node.Name || saved.Token != "" || saved.Status != NodeStatusOffline {
		t.Fatal("node save must update its name without restoring the revoked token or online status")
	}
	if saved.LastSeenAt == nil || !saved.LastSeenAt.Equal(heartbeatAt) || !saved.XrayRunning || saved.XrayVersion != "test-version" ||
		saved.SyncStatus != NodeSyncStatusSynced || saved.ConfigVersion != "test-config" {
		t.Fatal("node save overwrote newer heartbeat, core, or sync state")
	}
	if saved.TrafficResetAt == nil || !saved.TrafficResetAt.Equal(heartbeatAt) || saved.TrafficTotal != 42 {
		t.Fatal("node save overwrote the current traffic cycle")
	}
	nextReset := heartbeatAt.AddDate(0, 1, 0)
	edit := *saved
	edit.TrafficResetAt = &nextReset
	if err := repo.UpdateSettings(ctx, &edit, node, false, false, true); err == nil {
		t.Fatal("a stale edit replaced the current traffic cycle")
	}
	if err := repo.UpdateSettings(ctx, &edit, saved, false, false, true); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.GetByID(ctx, node.ID)
	if err != nil || saved.TrafficResetAt == nil || !saved.TrafficResetAt.Equal(nextReset) || saved.TrafficTotal != 42 {
		t.Fatalf("explicit traffic reset date edit failed: %v", err)
	}

	if err := repo.Delete(ctx, node.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.Update(ctx, node); err == nil {
		t.Fatal("a stale update must not recreate a deleted node")
	}
}

func TestNodeRepositoryCanRevokeTokensForSeveralNodes(t *testing.T) {
	repo := NewNodeRepository(setupNodeRepositoryTestDB(t))
	ctx := t.Context()
	legacy := &Node{Name: "legacy", Address: "192.0.2.1"}
	first := &Node{Name: "first", Address: "192.0.2.2", Token: "first-test-token"}
	second := &Node{Name: "second", Address: "192.0.2.3", Token: "second-test-token"}
	for _, node := range []*Node{legacy, first, second} {
		if err := repo.Create(ctx, node); err != nil {
			t.Fatal(err)
		}
	}

	for _, node := range []*Node{first, second, legacy} {
		if err := repo.UpdateToken(ctx, node.ID, ""); err != nil {
			t.Fatalf("revoke %s: %v", node.Name, err)
		}
		saved, err := repo.GetByID(ctx, node.ID)
		if err != nil || saved.Token != "" {
			t.Fatalf("expected revoked token for %s, read error: %v", node.Name, err)
		}
	}
}

func TestHeartbeatPreservesMeasuredLatencyAndUnhealthyStatus(t *testing.T) {
	db := setupNodeRepositoryTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()
	node := &Node{Name: "lossy", Address: "127.0.0.1", Token: "lossy-token", Status: NodeStatusUnhealthy, Latency: 1234}
	if err := repo.Create(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateHeartbeatBatch(ctx, node.ID, NodeStatusOnline, time.Now(), 0, 2, 1, 2, 3, true, "test"); err != nil {
		t.Fatal(err)
	}
	var got Node
	if err := db.First(&got, node.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.Latency != 1234 || got.Status != NodeStatusUnhealthy || got.CurrentUsers != 2 || !got.XrayRunning {
		t.Fatalf("heartbeat corrupted measured health: latency=%d status=%s users=%d xray=%v", got.Latency, got.Status, got.CurrentUsers, got.XrayRunning)
	}
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
