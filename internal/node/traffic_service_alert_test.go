package node

import (
	"context"
	"testing"

	"v/internal/database/repository"
	"v/internal/logger"
)

func TestTrafficService_RecordTrafficBatchTriggersThresholdAlertOnCrossing(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	nodeID := int64(41)

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

	alerts := make([]*NodeTrafficAlert, 0)
	service.WithNodeTrafficAlertHook(func(ctx context.Context, alert *NodeTrafficAlert) {
		alerts = append(alerts, alert)
	})

	if err := db.Create(&repository.Node{
		ID:                    nodeID,
		Name:                  "threshold-node",
		Address:               "127.0.0.1",
		Status:                repository.NodeStatusOnline,
		TrafficTotal:          79,
		TrafficLimit:          100,
		AlertTrafficThreshold: 80,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	if err := db.Create(&repository.User{ID: 1, Username: "alert-user-1", PasswordHash: "x"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	if err := service.RecordTrafficBatch(ctx, []*TrafficRecord{{NodeID: nodeID, UserID: 1, Upload: 1}}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0] == nil || alerts[0].Level != "threshold" {
		t.Fatalf("expected threshold alert, got %+v", alerts[0])
	}
	if alerts[0].NodeID != nodeID {
		t.Fatalf("expected node id %d, got %d", nodeID, alerts[0].NodeID)
	}
	if alerts[0].ThresholdPercent != 80 {
		t.Fatalf("expected threshold 80, got %.2f", alerts[0].ThresholdPercent)
	}
}

func TestTrafficService_RecordTrafficBatchTriggersLimitAlertOnCrossing(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	nodeID := int64(42)

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

	alerts := make([]*NodeTrafficAlert, 0)
	service.WithNodeTrafficAlertHook(func(ctx context.Context, alert *NodeTrafficAlert) {
		alerts = append(alerts, alert)
	})

	if err := db.Create(&repository.Node{
		ID:                    nodeID,
		Name:                  "limit-node",
		Address:               "127.0.0.1",
		Status:                repository.NodeStatusOnline,
		TrafficTotal:          95,
		TrafficLimit:          100,
		AlertTrafficThreshold: 80,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	if err := db.Create(&repository.User{ID: 2, Username: "alert-user-2", PasswordHash: "x"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	if err := service.RecordTrafficBatch(ctx, []*TrafficRecord{{NodeID: nodeID, UserID: 2, Upload: 5}}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0] == nil || alerts[0].Level != "limit" {
		t.Fatalf("expected limit alert, got %+v", alerts[0])
	}
	if alerts[0].ThresholdPercent != 100 {
		t.Fatalf("expected threshold 100, got %.2f", alerts[0].ThresholdPercent)
	}
}

func TestTrafficService_RecordTrafficBatchDoesNotRealertWhenAlreadyOverThreshold(t *testing.T) {
	db := setupTrafficServiceTestDB(t)
	ctx := context.Background()
	nodeID := int64(43)

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

	alerts := make([]*NodeTrafficAlert, 0)
	service.WithNodeTrafficAlertHook(func(ctx context.Context, alert *NodeTrafficAlert) {
		alerts = append(alerts, alert)
	})

	if err := db.Create(&repository.Node{
		ID:                    nodeID,
		Name:                  "steady-high-node",
		Address:               "127.0.0.1",
		Status:                repository.NodeStatusOnline,
		TrafficTotal:          85,
		TrafficLimit:          100,
		AlertTrafficThreshold: 80,
	}).Error; err != nil {
		t.Fatalf("failed to seed node: %v", err)
	}
	if err := db.Create(&repository.User{ID: 3, Username: "alert-user-3", PasswordHash: "x"}).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	if err := service.RecordTrafficBatch(ctx, []*TrafficRecord{{NodeID: nodeID, UserID: 3, Upload: 1}}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(alerts) != 0 {
		t.Fatalf("expected no alerts, got %d: %+v", len(alerts), alerts)
	}
}
