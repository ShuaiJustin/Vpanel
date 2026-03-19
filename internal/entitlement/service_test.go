package entitlement

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
	pkgerrors "v/pkg/errors"
)

func setupTestService(t *testing.T) (*Service, *gorm.DB) {
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
		&repository.Trial{},
		&repository.UserNodeAssignment{},
	); err != nil {
		t.Fatalf("failed to migrate test schema: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	trialRepo := repository.NewTrialRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	trialService := trialsvc.NewService(trialRepo, userRepo, logger.NewNopLogger(), nil)

	service := NewService(
		userRepo,
		trialRepo,
		proxyRepo,
		nodeRepo,
		assignmentRepo,
		trialService,
		logger.NewNopLogger(),
	)

	return service, db
}

func createTestUser(t *testing.T, db *gorm.DB, username string) *repository.User {
	t.Helper()

	user := &repository.User{
		Username:     username,
		PasswordHash: "hashed-password",
		Email:        username + "@example.com",
		Enabled:      true,
		Role:         "user",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	return user
}

func createTestNode(t *testing.T, db *gorm.DB, name string) *repository.Node {
	t.Helper()

	node := &repository.Node{
		Name:    name,
		Address: name + ".example.com",
		Token:   name + "-token",
		Status:  repository.NodeStatusOnline,
	}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}
	return node
}

func createNodeProxy(t *testing.T, db *gorm.DB, nodeID int64, name string, port int) *repository.Proxy {
	t.Helper()

	nodeRef := nodeID
	proxy := &repository.Proxy{
		UserID:   0,
		NodeID:   &nodeRef,
		Name:     name,
		Protocol: "vmess",
		Port:     port,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxy).Error; err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}
	return proxy
}

func TestEvaluateAccess_AutoActivatesTrial(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "trial-user")

	state, err := service.EvaluateAccess(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected trial access, got error: %v", err)
	}

	if !state.HasActiveTrial {
		t.Fatalf("expected active trial to be auto-activated")
	}
	if state.EffectiveExpiresAt == nil {
		t.Fatalf("expected effective trial expiry")
	}

	var repoTrial repository.Trial
	if err := db.First(&repoTrial, "user_id = ?", user.ID).Error; err != nil {
		t.Fatalf("expected persisted trial, got error: %v", err)
	}
	if repoTrial.Status != "active" {
		t.Fatalf("expected active trial status, got %s", repoTrial.Status)
	}
	if repoTrial.ExpireAt.Before(time.Now().AddDate(0, 0, 6)) {
		t.Fatalf("expected trial expiry about 7 days ahead, got %v", repoTrial.ExpireAt)
	}
}

func TestEvaluateAccess_DeniesExpiredTrial(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "expired-trial-user")

	expiredTrial := &repository.Trial{
		UserID:      user.ID,
		Status:      "active",
		StartAt:     time.Now().AddDate(0, 0, -8),
		ExpireAt:    time.Now().AddDate(0, 0, -1),
		TrafficUsed: 0,
	}
	if err := db.Create(expiredTrial).Error; err != nil {
		t.Fatalf("failed to create expired trial: %v", err)
	}

	_, err := service.EvaluateAccess(context.Background(), user.ID)
	if err == nil || !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for expired trial, got %v", err)
	}

	var repoTrial repository.Trial
	if err := db.First(&repoTrial, expiredTrial.ID).Error; err != nil {
		t.Fatalf("failed to reload trial: %v", err)
	}
	if repoTrial.Status != "expired" {
		t.Fatalf("expected expired trial status to be persisted, got %s", repoTrial.Status)
	}
}

func TestGetAccessibleProxies_AssignsSingleNodeInsteadOfGlobalFallback(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "assigned-user")
	nodeOne := createTestNode(t, db, "node-one")
	nodeTwo := createTestNode(t, db, "node-two")

	primaryProxy := createNodeProxy(t, db, nodeOne.ID, "node-one-proxy-a", 10001)
	createNodeProxy(t, db, nodeOne.ID, "node-one-proxy-b", 10011)
	createNodeProxy(t, db, nodeTwo.ID, "node-two-proxy-a", 10002)
	createNodeProxy(t, db, nodeTwo.ID, "node-two-proxy-b", 10003)

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected assigned node proxies, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected only proxies from one assigned node, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != nodeOne.ID {
		t.Fatalf("expected assignment to lowest-load node %d, got %+v", nodeOne.ID, proxies[0].NodeID)
	}
	if proxies[0].ID != primaryProxy.ID {
		t.Fatalf("expected primary proxy %d from assigned node, got %d", primaryProxy.ID, proxies[0].ID)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to load assignment: %v", err)
	}
	if assignment == nil || assignment.NodeID != nodeOne.ID {
		t.Fatalf("expected persisted assignment to node %d, got %+v", nodeOne.ID, assignment)
	}
}

func TestGetAccessibleProxies_ReassignsOfflineNode(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "reassign-user")

	offlineNode := createTestNode(t, db, "offline-node")
	offlineNode.Status = repository.NodeStatusOffline
	if err := db.Save(offlineNode).Error; err != nil {
		t.Fatalf("failed to mark node offline: %v", err)
	}
	onlineNode := createTestNode(t, db, "online-node")

	createNodeProxy(t, db, offlineNode.ID, "offline-proxy", 11001)
	expectedProxy := createNodeProxy(t, db, onlineNode.ID, "online-proxy", 11002)

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, offlineNode.ID); err != nil {
		t.Fatalf("failed to create initial assignment: %v", err)
	}

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected reassigned accessible proxy, got error: %v", err)
	}
	if len(proxies) != 1 || proxies[0].ID != expectedProxy.ID {
		t.Fatalf("expected reassigned online proxy %d, got %+v", expectedProxy.ID, proxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload assignment: %v", err)
	}
	if assignment == nil || assignment.NodeID != onlineNode.ID {
		t.Fatalf("expected reassignment to node %d, got %+v", onlineNode.ID, assignment)
	}
}
