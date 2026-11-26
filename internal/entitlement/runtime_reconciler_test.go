package entitlement

import (
	"context"
	"testing"
	"time"

	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestRuntimeReconcilerRunOnce_CleansMissingNodeAndRevokedUserRuntime(t *testing.T) {
	service, db := setupTestService(t)
	reconciler := NewRuntimeReconciler(nil, service, repository.NewProxyRepository(db), repository.NewNodeRepository(db), logger.NewNopLogger())

	activeUser := createTestUser(t, db, "reconciler-active-user")
	expiredUser := createTestUser(t, db, "reconciler-expired-user")
	validNode := createTestNode(t, db, "reconciler-valid-node")
	validNodeRef := validNode.ID
	missingNodeID := int64(999999)

	expiredAt := time.Now().Add(-time.Hour)
	expiredUser.ExpiresAt = &expiredAt
	if err := db.Save(expiredUser).Error; err != nil {
		t.Fatalf("failed to expire user: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), expiredUser.ID, validNode.ID); err != nil {
		t.Fatalf("failed to assign expired user: %v", err)
	}

	fixtures := []*repository.Proxy{
		{
			UserID:   activeUser.ID,
			NodeID:   &missingNodeID,
			Name:     "active-user-missing-node-proxy",
			Protocol: "vmess",
			Port:     25001,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
		{
			UserID:   0,
			NodeID:   &missingNodeID,
			Name:     "shared-missing-node-proxy",
			Protocol: "vmess",
			Port:     25002,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
		{
			UserID:   expiredUser.ID,
			NodeID:   &validNodeRef,
			Name:     "expired-user-proxy",
			Protocol: "vmess",
			Port:     25003,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
	}
	for _, proxyModel := range fixtures {
		if err := db.Create(proxyModel).Error; err != nil {
			t.Fatalf("failed to create proxy fixture %q: %v", proxyModel.Name, err)
		}
	}

	stats, err := reconciler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected reconciler run to succeed, got %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil reconciler stats")
	}
	if stats.DeletedMissingNode != 2 {
		t.Fatalf("expected 2 missing-node proxies deleted, got %d", stats.DeletedMissingNode)
	}
	if stats.ForbiddenUsersDetected == 0 {
		t.Fatal("expected revoked users to be detected")
	}

	var remaining []*repository.Proxy
	if err := db.Order("id asc").Find(&remaining).Error; err != nil {
		t.Fatalf("failed to load remaining proxies: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected all stale proxies to be removed, got %d remaining", len(remaining))
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), expiredUser.ID)
	if err != nil {
		t.Fatalf("failed to load expired-user assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("expected expired-user assignment cleanup, got %+v", assignment)
	}
}

func TestRuntimeReconcilerRunOnce_PreservesPausedExpiredSubscriptionRuntime(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "reconciler-paused-user")
	node := createTestNode(t, db, "reconciler-paused-node")
	nodeRef := node.ID
	expiredAt := time.Now().Add(-time.Hour)
	user.ExpiresAt = &expiredAt
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to expire paused user: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign paused user: %v", err)
	}

	pause := &repository.SubscriptionPause{
		UserID:           user.ID,
		PausedAt:         time.Now().Add(-30 * time.Minute),
		RemainingDays:    3,
		RemainingTraffic: 2048,
		AutoResumeAt:     time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(pause).Error; err != nil {
		t.Fatalf("failed to create active pause: %v", err)
	}

	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "paused-user-proxy",
		Protocol: "vmess",
		Port:     25004,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create paused-user proxy: %v", err)
	}

	reconciler := NewRuntimeReconciler(nil, service, repository.NewProxyRepository(db), repository.NewNodeRepository(db), logger.NewNopLogger())
	stats, err := reconciler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("expected reconciler run to succeed, got %v", err)
	}
	if stats == nil {
		t.Fatal("expected non-nil reconciler stats")
	}

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count paused-user proxies: %v", err)
	}
	if remainingProxies != 1 {
		t.Fatalf("expected paused-user proxy to remain, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to load paused-user assignment: %v", err)
	}
	if assignment == nil || assignment.NodeID != node.ID {
		t.Fatalf("expected paused-user assignment to remain on node %d, got %+v", node.ID, assignment)
	}
}

func TestNewRuntimeReconciler_DefaultsInterval(t *testing.T) {
	dbService, db := setupTestService(t)
	_ = trialsvc.NewService(repository.NewTrialRepository(db), repository.NewUserRepository(db), logger.NewNopLogger(), nil)
	reconciler := NewRuntimeReconciler(nil, dbService, repository.NewProxyRepository(db), repository.NewNodeRepository(db), logger.NewNopLogger())
	if reconciler.config == nil || reconciler.config.Interval <= 0 {
		t.Fatal("expected default reconciler interval to be set")
	}
}
