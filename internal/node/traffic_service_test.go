package node

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

func setupTrafficServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(
		&repository.User{},
		&repository.Node{},
		&repository.Proxy{},
		&repository.Traffic{},
		&repository.NodeTraffic{},
		&repository.Trial{},
	); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}

	return db
}

func TestTrafficService_RecordTrafficBatchUpdatesActiveTrialTraffic(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	now := time.Now()
	nodeID := int64(7)

	service := NewTrafficService(
		db,
		repository.NewNodeTrafficRepository(db),
		repository.NewTrafficRepository(db),
		repository.NewProxyRepository(db),
		repository.NewUserRepository(db),
		repository.NewNodeRepository(db),
		nil,
		logger.NewNopLogger(),
	)

	if err := db.Create(&repository.Node{
		ID:      nodeID,
		Name:    "node-1",
		Address: "127.0.0.1",
		Status:  repository.NodeStatusOnline,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	users := []*repository.User{
		{ID: 1, Username: "active-trial", PasswordHash: "x"},
		{ID: 2, Username: "expired-trial", PasswordHash: "x"},
		{ID: 3, Username: "converted-trial", PasswordHash: "x"},
	}
	for _, user := range users {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user %d: %v", user.ID, err)
		}
	}

	proxies := []*repository.Proxy{
		{ID: 11, UserID: 1, NodeID: &nodeID, Name: "p1", Protocol: "vmess", Port: 20001, Host: "127.0.0.1", Enabled: true},
		{ID: 12, UserID: 2, NodeID: &nodeID, Name: "p2", Protocol: "vmess", Port: 20002, Host: "127.0.0.1", Enabled: true},
		{ID: 13, UserID: 3, NodeID: &nodeID, Name: "p3", Protocol: "vmess", Port: 20003, Host: "127.0.0.1", Enabled: true},
	}
	for _, proxy := range proxies {
		if err := db.Create(proxy).Error; err != nil {
			t.Fatalf("failed to seed proxy %d: %v", proxy.ID, err)
		}
	}

	trials := []*repository.Trial{
		{ID: 21, UserID: 1, Status: "active", StartAt: now.Add(-time.Hour), ExpireAt: now.Add(time.Hour), TrafficUsed: 1000},
		{ID: 22, UserID: 2, Status: "active", StartAt: now.Add(-2 * time.Hour), ExpireAt: now.Add(-time.Minute), TrafficUsed: 2000},
		{ID: 23, UserID: 3, Status: "converted", StartAt: now.Add(-2 * time.Hour), ExpireAt: now.Add(time.Hour), TrafficUsed: 3000},
	}
	for _, trial := range trials {
		if err := db.Create(trial).Error; err != nil {
			t.Fatalf("failed to seed trial %d: %v", trial.ID, err)
		}
	}

	records := []*TrafficRecord{
		{NodeID: nodeID, UserID: 1, ProxyID: &proxies[0].ID, Upload: 100, Download: 50},
		{NodeID: nodeID, UserID: 2, ProxyID: &proxies[1].ID, Upload: 20, Download: 30},
		{NodeID: nodeID, UserID: 3, ProxyID: &proxies[2].ID, Upload: 5, Download: 6},
	}

	if err := service.RecordTrafficBatch(ctx, records); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var activeUser repository.User
	if err := db.First(&activeUser, 1).Error; err != nil {
		t.Fatalf("failed to load active trial user: %v", err)
	}
	if activeUser.TrafficUsed != 150 {
		t.Fatalf("expected active trial user traffic_used 150, got %d", activeUser.TrafficUsed)
	}

	var activeTrial repository.Trial
	if err := db.First(&activeTrial, 21).Error; err != nil {
		t.Fatalf("failed to load active trial: %v", err)
	}
	if activeTrial.TrafficUsed != 1150 {
		t.Fatalf("expected active trial traffic_used 1150, got %d", activeTrial.TrafficUsed)
	}

	var expiredTrial repository.Trial
	if err := db.First(&expiredTrial, 22).Error; err != nil {
		t.Fatalf("failed to load expired trial: %v", err)
	}
	if expiredTrial.TrafficUsed != 2000 {
		t.Fatalf("expected expired trial traffic_used to stay 2000, got %d", expiredTrial.TrafficUsed)
	}

	var convertedTrial repository.Trial
	if err := db.First(&convertedTrial, 23).Error; err != nil {
		t.Fatalf("failed to load converted trial: %v", err)
	}
	if convertedTrial.TrafficUsed != 3000 {
		t.Fatalf("expected converted trial traffic_used to stay 3000, got %d", convertedTrial.TrafficUsed)
	}

	var trafficCount int64
	if err := db.Model(&repository.Traffic{}).Count(&trafficCount).Error; err != nil {
		t.Fatalf("failed to count traffic records: %v", err)
	}
	if trafficCount != 3 {
		t.Fatalf("expected 3 traffic records, got %d", trafficCount)
	}

	var nodeTrafficCount int64
	if err := db.Model(&repository.NodeTraffic{}).Count(&nodeTrafficCount).Error; err != nil {
		t.Fatalf("failed to count node traffic records: %v", err)
	}
	if nodeTrafficCount != 3 {
		t.Fatalf("expected 3 node traffic records, got %d", nodeTrafficCount)
	}
}

func TestTrafficService_RecordTrafficBatchTriggersAccessRevokedHook(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	nodeID1 := int64(7)
	nodeID2 := int64(8)

	service := NewTrafficService(
		db,
		repository.NewNodeTrafficRepository(db),
		repository.NewTrafficRepository(db),
		repository.NewProxyRepository(db),
		repository.NewUserRepository(db),
		repository.NewNodeRepository(db),
		nil,
		logger.NewNopLogger(),
	)

	userRepo := repository.NewUserRepository(db)
	service.WithUserAccessCheck(func(ctx context.Context, userID int64) error {
		user, err := userRepo.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if user.IsTrafficExceeded() {
			return apperrors.NewForbiddenError("traffic limit exceeded")
		}
		return nil
	})

	var (
		hookUserID  int64
		hookNodeIDs []int64
		hookReason  string
		hookCalls   int
	)
	service.WithAccessRevokedHook(func(ctx context.Context, userID int64, nodeIDs []int64, reason string) {
		hookCalls++
		hookUserID = userID
		hookNodeIDs = append([]int64(nil), nodeIDs...)
		hookReason = reason
	})

	for _, nodeData := range []*repository.Node{
		{ID: nodeID1, Name: "node-1", Address: "127.0.0.1", Token: "node-token-1", Status: repository.NodeStatusOnline},
		{ID: nodeID2, Name: "node-2", Address: "127.0.0.2", Token: "node-token-2", Status: repository.NodeStatusOnline},
	} {
		if err := db.Create(nodeData).Error; err != nil {
			t.Fatalf("failed to seed node %d: %v", nodeData.ID, err)
		}
	}

	user := &repository.User{ID: 1, Username: "threshold-user", PasswordHash: "x", TrafficLimit: 100, TrafficUsed: 95}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	for _, proxy := range []*repository.Proxy{
		{ID: 11, UserID: 1, NodeID: &nodeID1, Name: "p1", Protocol: "vmess", Port: 20001, Host: "127.0.0.1", Enabled: true},
		{ID: 12, UserID: 1, NodeID: &nodeID2, Name: "p2", Protocol: "vmess", Port: 20002, Host: "127.0.0.1", Enabled: true},
	} {
		if err := db.Create(proxy).Error; err != nil {
			t.Fatalf("failed to seed proxy %d: %v", proxy.ID, err)
		}
	}

	records := []*TrafficRecord{{NodeID: nodeID1, UserID: 1, Upload: 3, Download: 4}}
	if err := service.RecordTrafficBatch(ctx, records); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if hookCalls != 1 {
		t.Fatalf("expected hook to be called once, got %d", hookCalls)
	}
	if hookUserID != 1 {
		t.Fatalf("expected hook user id 1, got %d", hookUserID)
	}
	sort.Slice(hookNodeIDs, func(i, j int) bool { return hookNodeIDs[i] < hookNodeIDs[j] })
	if !reflect.DeepEqual(hookNodeIDs, []int64{nodeID1, nodeID2}) {
		t.Fatalf("expected hook node ids [%d %d], got %v", nodeID1, nodeID2, hookNodeIDs)
	}
	if hookReason != "traffic limit exceeded" {
		t.Fatalf("expected hook reason 'traffic limit exceeded', got %q", hookReason)
	}

	var updated repository.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("failed to load updated user: %v", err)
	}
	if updated.TrafficUsed != 102 {
		t.Fatalf("expected updated traffic_used 102, got %d", updated.TrafficUsed)
	}
}

func TestTrafficService_RecordTrafficBatchMarksNodeUnhealthyWhenLimitExceeded(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	nodeID := int64(7)

	service := NewTrafficService(
		db,
		repository.NewNodeTrafficRepository(db),
		repository.NewTrafficRepository(db),
		repository.NewProxyRepository(db),
		repository.NewUserRepository(db),
		repository.NewNodeRepository(db),
		nil,
		logger.NewNopLogger(),
	)

	var (
		hookNodeID int64
		hookReason string
		hookCalls  int
	)
	service.WithNodeTrafficLimitExceededHook(func(ctx context.Context, nodeID int64, reason string) {
		hookCalls++
		hookNodeID = nodeID
		hookReason = reason
	})

	limit := int64(100)
	if err := db.Create(&repository.Node{
		ID:           nodeID,
		Name:         "quota-node",
		Address:      "127.0.0.1",
		Status:       repository.NodeStatusOnline,
		TrafficTotal: 95,
		TrafficLimit: limit,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	if err := db.Create(&repository.User{ID: 1, Username: "quota-user", PasswordHash: "x"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	records := []*TrafficRecord{{NodeID: nodeID, UserID: 1, Upload: 3, Download: 4}}
	if err := service.RecordTrafficBatch(ctx, records); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var updated repository.Node
	if err := db.First(&updated, nodeID).Error; err != nil {
		t.Fatalf("failed to load updated node: %v", err)
	}
	if updated.TrafficTotal != 102 {
		t.Fatalf("expected traffic total 102, got %d", updated.TrafficTotal)
	}
	if updated.Status != repository.NodeStatusUnhealthy {
		t.Fatalf("expected node unhealthy, got %s", updated.Status)
	}
	if hookCalls != 1 {
		t.Fatalf("expected hook to be called once, got %d", hookCalls)
	}
	if hookNodeID != nodeID {
		t.Fatalf("expected hook node id %d, got %d", nodeID, hookNodeID)
	}
	if hookReason != "node traffic limit exceeded" {
		t.Fatalf("expected hook reason node traffic limit exceeded, got %q", hookReason)
	}
}

func TestTrafficService_ProcessMonthlyTrafficResets_InitializesMissingResetAt(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	nodeID := int64(31)

	service := NewTrafficService(
		db,
		repository.NewNodeTrafficRepository(db),
		repository.NewTrafficRepository(db),
		repository.NewProxyRepository(db),
		repository.NewUserRepository(db),
		repository.NewNodeRepository(db),
		nil,
		logger.NewNopLogger(),
	)

	if err := db.Create(&repository.Node{
		ID:           nodeID,
		Name:         "cycle-anchor-node",
		Address:      "127.0.0.1",
		Status:       repository.NodeStatusOnline,
		TrafficUp:    100,
		TrafficDown:  200,
		TrafficTotal: 300,
		TrafficLimit: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	resetNodeIDs, err := service.ProcessMonthlyTrafficResets(ctx, now)
	if err != nil {
		t.Fatalf("ProcessMonthlyTrafficResets returned error: %v", err)
	}
	if len(resetNodeIDs) != 0 {
		t.Fatalf("expected no reset node ids, got %v", resetNodeIDs)
	}

	var updated repository.Node
	if err := db.First(&updated, nodeID).Error; err != nil {
		t.Fatalf("failed to load updated node: %v", err)
	}
	if updated.TrafficResetAt == nil {
		t.Fatalf("expected traffic_reset_at to be initialized")
	}
	if !updated.TrafficResetAt.Equal(now) {
		t.Fatalf("expected traffic_reset_at %v, got %v", now, updated.TrafficResetAt)
	}
	if updated.TrafficUp != 100 || updated.TrafficDown != 200 || updated.TrafficTotal != 300 {
		t.Fatalf("expected traffic counters to remain unchanged, got up=%d down=%d total=%d", updated.TrafficUp, updated.TrafficDown, updated.TrafficTotal)
	}
}

func TestTrafficService_ProcessMonthlyTrafficResets_ResetsDueNode(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	nodeID := int64(32)
	oldResetAt := now.AddDate(0, -1, -1)

	service := NewTrafficService(
		db,
		repository.NewNodeTrafficRepository(db),
		repository.NewTrafficRepository(db),
		repository.NewProxyRepository(db),
		repository.NewUserRepository(db),
		repository.NewNodeRepository(db),
		nil,
		logger.NewNopLogger(),
	)

	if err := db.Create(&repository.Node{
		ID:             nodeID,
		Name:           "due-reset-node",
		Address:        "127.0.0.1",
		Status:         repository.NodeStatusUnhealthy,
		TrafficUp:      400,
		TrafficDown:    500,
		TrafficTotal:   900,
		TrafficLimit:   1000,
		TrafficResetAt: &oldResetAt,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}

	resetNodeIDs, err := service.ProcessMonthlyTrafficResets(ctx, now)
	if err != nil {
		t.Fatalf("ProcessMonthlyTrafficResets returned error: %v", err)
	}
	if !reflect.DeepEqual(resetNodeIDs, []int64{nodeID}) {
		t.Fatalf("expected reset node ids [%d], got %v", nodeID, resetNodeIDs)
	}

	var updated repository.Node
	if err := db.First(&updated, nodeID).Error; err != nil {
		t.Fatalf("failed to load updated node: %v", err)
	}
	if updated.TrafficUp != 0 || updated.TrafficDown != 0 || updated.TrafficTotal != 0 {
		t.Fatalf("expected traffic counters to be reset, got up=%d down=%d total=%d", updated.TrafficUp, updated.TrafficDown, updated.TrafficTotal)
	}
	if updated.TrafficResetAt == nil {
		t.Fatalf("expected traffic_reset_at to remain set")
	}
	if !updated.TrafficResetAt.Equal(now) {
		t.Fatalf("expected traffic_reset_at %v, got %v", now, updated.TrafficResetAt)
	}
}
