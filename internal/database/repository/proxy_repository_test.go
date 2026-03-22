package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupProxyRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := db.AutoMigrate(&User{}, &Proxy{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

func TestProxyRepository_RuntimeQueriesIgnoreOrphans(t *testing.T) {
	db := setupProxyRepositoryTestDB(t)
	ctx := context.Background()
	repo := NewProxyRepository(db)

	validUser := &User{
		Username:     "valid-proxy-user",
		PasswordHash: "hashedpassword",
		Email:        "valid-proxy@example.com",
	}
	if err := db.WithContext(ctx).Create(validUser).Error; err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	nodeID := int64(9)
	validProxy := &Proxy{
		UserID:    validUser.ID,
		NodeID:    &nodeID,
		Name:      "valid-proxy",
		Protocol:  "vmess",
		Port:      21001,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.WithContext(ctx).Create(validProxy).Error; err != nil {
		t.Fatalf("Failed to create valid proxy: %v", err)
	}

	sharedProxy := &Proxy{
		UserID:    0,
		NodeID:    &nodeID,
		Name:      "shared-proxy",
		Protocol:  "vmess",
		Port:      21003,
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := db.WithContext(ctx).Create(sharedProxy).Error; err != nil {
		t.Fatalf("Failed to create shared proxy: %v", err)
	}

	if err := db.WithContext(ctx).Exec(
		"INSERT INTO proxies (user_id, node_id, name, protocol, port, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		int64(999999),
		nodeID,
		"orphan-proxy",
		"vmess",
		21002,
		true,
		time.Now(),
		time.Now(),
	).Error; err != nil {
		t.Fatalf("Failed to create orphan proxy: %v", err)
	}

	enabled, err := repo.GetEnabled(ctx)
	if err != nil {
		t.Fatalf("GetEnabled failed: %v", err)
	}
	if len(enabled) != 2 {
		t.Fatalf("Expected valid and shared proxy from GetEnabled, got %+v", enabled)
	}

	enabledByID := map[int64]bool{}
	for _, proxy := range enabled {
		enabledByID[proxy.ID] = true
	}
	if !enabledByID[validProxy.ID] || !enabledByID[sharedProxy.ID] {
		t.Fatalf("Expected only valid proxy from GetEnabled, got %+v", enabled)
	}

	byNode, err := repo.GetByNodeID(ctx, nodeID)
	if err != nil {
		t.Fatalf("GetByNodeID failed: %v", err)
	}
	if len(byNode) != 2 {
		t.Fatalf("Expected valid and shared proxy from GetByNodeID, got %+v", byNode)
	}

	byNodeID := map[int64]bool{}
	for _, proxy := range byNode {
		byNodeID[proxy.ID] = true
	}
	if !byNodeID[validProxy.ID] || !byNodeID[sharedProxy.ID] {
		t.Fatalf("Expected valid and shared proxy from GetByNodeID, got %+v", byNode)
	}

	portProxy, err := repo.GetByPort(ctx, 21002)
	if err != nil {
		t.Fatalf("GetByPort failed: %v", err)
	}
	if portProxy != nil {
		t.Fatalf("Expected orphan proxy port to be ignored, got proxy ID %d", portProxy.ID)
	}

	sharedPortProxy, err := repo.GetByPort(ctx, 21003)
	if err != nil {
		t.Fatalf("GetByPort for shared proxy failed: %v", err)
	}
	if sharedPortProxy == nil || sharedPortProxy.ID != sharedProxy.ID {
		t.Fatalf("Expected shared proxy port lookup to return proxy %d, got %+v", sharedProxy.ID, sharedPortProxy)
	}
}
