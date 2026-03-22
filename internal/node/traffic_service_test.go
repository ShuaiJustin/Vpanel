package node

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
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
