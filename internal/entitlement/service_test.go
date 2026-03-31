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
	"v/internal/proxy"
	"v/internal/proxy/protocols/shadowsocks"
	"v/internal/proxy/protocols/trojan"
	"v/internal/proxy/protocols/vless"
	"v/internal/proxy/protocols/vmess"
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
		&repository.SubscriptionPause{},
		&repository.UserNodeAssignment{},
	); err != nil {
		t.Fatalf("failed to migrate test schema: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	trialRepo := repository.NewTrialRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	pauseRepo := repository.NewPauseRepository(db)
	trialService := trialsvc.NewService(trialRepo, userRepo, logger.NewNopLogger(), nil)
	proxyManager := proxy.NewManager(proxyRepo)
	proxyManager.RegisterProtocol(vmess.New())
	proxyManager.RegisterProtocol(vless.New())
	proxyManager.RegisterProtocol(trojan.New())
	proxyManager.RegisterProtocol(shadowsocks.New())

	service := NewService(
		userRepo,
		trialRepo,
		proxyRepo,
		nodeRepo,
		assignmentRepo,
		trialService,
		logger.NewNopLogger(),
	).WithProxyManager(proxyManager).WithPauseRepository(pauseRepo)

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
	node := createTestNode(t, db, "expired-trial-node")
	nodeRef := node.ID
	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "expired-trial-proxy",
		Protocol: "vmess",
		Port:     12001,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create expired-trial proxy: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign expired-trial user: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

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

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count remaining proxies: %v", err)
	}
	if remainingProxies != 0 {
		t.Fatalf("expected expired trial proxies to be removed, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload expired-trial assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("expected expired trial assignment cleanup, got %+v", assignment)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}
}

func TestEvaluateExistingAccess_CleansRuntimeForPersistedExpiredTrial(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "persisted-expired-trial-user")
	node := createTestNode(t, db, "persisted-expired-trial-node")
	nodeRef := node.ID

	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "persisted-expired-trial-proxy",
		Protocol: "vmess",
		Port:     12002,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create persisted expired-trial proxy: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign persisted expired-trial user: %v", err)
	}

	persistedExpiredTrial := &repository.Trial{
		UserID:      user.ID,
		Status:      "expired",
		StartAt:     time.Now().AddDate(0, 0, -14),
		ExpireAt:    time.Now().AddDate(0, 0, -7),
		TrafficUsed: 0,
	}
	if err := db.Create(persistedExpiredTrial).Error; err != nil {
		t.Fatalf("failed to create persisted expired trial: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	_, err := service.EvaluateExistingAccess(context.Background(), user.ID)
	if err == nil || !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for persisted expired trial, got %v", err)
	}

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count remaining proxies: %v", err)
	}
	if remainingProxies != 0 {
		t.Fatalf("expected persisted expired trial proxies to be removed, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload persisted expired-trial assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("expected persisted expired trial assignment cleanup, got %+v", assignment)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}
}

func TestEvaluateExistingAccess_CleansRuntimeForExpiredSubscription(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "expired-subscription-user")
	node := createTestNode(t, db, "expired-subscription-node")
	nodeRef := node.ID
	expiredAt := time.Now().Add(-2 * time.Hour)
	user.ExpiresAt = &expiredAt
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to expire test user: %v", err)
	}

	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "expired-subscription-proxy",
		Protocol: "vmess",
		Port:     12003,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create expired-subscription proxy: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign expired-subscription user: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	_, err := service.EvaluateExistingAccess(context.Background(), user.ID)
	if err == nil || !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for expired subscription, got %v", err)
	}

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count remaining proxies: %v", err)
	}
	if remainingProxies != 0 {
		t.Fatalf("expected expired subscription proxies to be removed, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload expired-subscription assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("expected expired subscription assignment cleanup, got %+v", assignment)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}
}

func TestEvaluateExistingAccess_SkipsCleanupForPausedExpiredSubscription(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "paused-subscription-user")
	node := createTestNode(t, db, "paused-subscription-node")
	nodeRef := node.ID
	expiredAt := time.Now().Add(-2 * time.Hour)
	user.ExpiresAt = &expiredAt
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to expire paused test user: %v", err)
	}

	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "paused-subscription-proxy",
		Protocol: "vmess",
		Port:     12004,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create paused-subscription proxy: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign paused-subscription user: %v", err)
	}

	activePause := &repository.SubscriptionPause{
		UserID:           user.ID,
		PausedAt:         time.Now().Add(-time.Hour),
		RemainingDays:    7,
		RemainingTraffic: 1024,
		AutoResumeAt:     time.Now().Add(24 * time.Hour),
	}
	if err := db.Create(activePause).Error; err != nil {
		t.Fatalf("failed to create active pause: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	_, err := service.EvaluateExistingAccess(context.Background(), user.ID)
	if err == nil || !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for paused expired subscription, got %v", err)
	}

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count remaining proxies: %v", err)
	}
	if remainingProxies != 1 {
		t.Fatalf("expected paused subscription proxy to remain, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload paused-subscription assignment: %v", err)
	}
	if assignment == nil || assignment.NodeID != node.ID {
		t.Fatalf("expected paused subscription assignment to remain on node %d, got %+v", node.ID, assignment)
	}
	if syncedNodeID != 0 {
		t.Fatalf("expected paused subscription cleanup to skip config sync, got %d", syncedNodeID)
	}
}

func TestEvaluateExistingAccess_CleansRuntimeForDisabledUser(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "disabled-user-cleanup")
	node := createTestNode(t, db, "disabled-user-node")
	nodeRef := node.ID
	user.Enabled = false
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to disable test user: %v", err)
	}

	proxyModel := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "disabled-user-proxy",
		Protocol: "vmess",
		Port:     12005,
		Host:     "127.0.0.1",
		Enabled:  true,
	}
	if err := db.Create(proxyModel).Error; err != nil {
		t.Fatalf("failed to create disabled-user proxy: %v", err)
	}

	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	if err := assignmentRepo.Assign(context.Background(), user.ID, node.ID); err != nil {
		t.Fatalf("failed to assign disabled user: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	_, err := service.EvaluateExistingAccess(context.Background(), user.ID)
	if err == nil || !pkgerrors.IsForbidden(err) {
		t.Fatalf("expected forbidden error for disabled user, got %v", err)
	}

	var remainingProxies int64
	if err := db.Model(&repository.Proxy{}).Where("user_id = ?", user.ID).Count(&remainingProxies).Error; err != nil {
		t.Fatalf("failed to count remaining proxies: %v", err)
	}
	if remainingProxies != 0 {
		t.Fatalf("expected disabled user proxies to be removed, got %d", remainingProxies)
	}

	assignment, err := assignmentRepo.GetByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("failed to reload disabled-user assignment: %v", err)
	}
	if assignment != nil {
		t.Fatalf("expected disabled user assignment cleanup, got %+v", assignment)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
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

func TestGetAccessibleProxies_AutoProvisionsDefaultProxyOnEmptyNode(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "auto-provision-user")
	node := createTestNode(t, db, "empty-node")
	node.Protocols = `["vmess"]`
	if err := db.Save(node).Error; err != nil {
		t.Fatalf("failed to update node protocols: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected auto provisioned proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one auto provisioned proxy, got %d", len(proxies))
	}
	if proxies[0].UserID != user.ID {
		t.Fatalf("expected proxy to belong to user %d, got %d", user.ID, proxies[0].UserID)
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != node.ID {
		t.Fatalf("expected proxy node %d, got %+v", node.ID, proxies[0].NodeID)
	}
	if proxies[0].Protocol != "vmess" {
		t.Fatalf("expected vmess proxy, got %s", proxies[0].Protocol)
	}
	if proxies[0].Port < autoProvisionPortMin || proxies[0].Port > autoProvisionPortMax {
		t.Fatalf("expected auto provisioned port in range, got %d", proxies[0].Port)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}

	var persisted []*repository.Proxy
	if err := db.Where("user_id = ?", user.ID).Find(&persisted).Error; err != nil {
		t.Fatalf("failed to load persisted proxies: %v", err)
	}
	if len(persisted) != 1 {
		t.Fatalf("expected one persisted proxy, got %d", len(persisted))
	}
}

func TestGetAccessibleProxies_DoesNotShareAutoProvisionedUserProxy(t *testing.T) {
	service, db := setupTestService(t)
	firstUser := createTestUser(t, db, "auto-provision-first")
	secondUser := createTestUser(t, db, "auto-provision-second")
	node := createTestNode(t, db, "exclusive-node")
	node.Protocols = `["vmess"]`
	if err := db.Save(node).Error; err != nil {
		t.Fatalf("failed to update node protocols: %v", err)
	}

	firstProxies, _, err := service.GetAccessibleProxies(context.Background(), firstUser.ID)
	if err != nil {
		t.Fatalf("expected first user's proxy, got error: %v", err)
	}
	if len(firstProxies) != 1 {
		t.Fatalf("expected one first-user proxy, got %d", len(firstProxies))
	}

	secondProxies, _, err := service.GetAccessibleProxies(context.Background(), secondUser.ID)
	if err != nil {
		t.Fatalf("expected second user's proxy, got error: %v", err)
	}
	if len(secondProxies) != 1 {
		t.Fatalf("expected one second-user proxy, got %d", len(secondProxies))
	}
	if secondProxies[0].ID == firstProxies[0].ID {
		t.Fatalf("expected distinct proxies for different users, got shared proxy %d", secondProxies[0].ID)
	}
	if secondProxies[0].UserID != secondUser.ID {
		t.Fatalf("expected second user's proxy ownership %d, got %d", secondUser.ID, secondProxies[0].UserID)
	}

	var count int64
	if err := db.Model(&repository.Proxy{}).Where("node_id = ?", node.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count node proxies: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected two user-specific proxies on node, got %d", count)
	}
}

func TestGetAccessibleProxies_AutoProvisionedTLSDefaultsToVLESSWhenNodeProtocolsEmpty(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "tls-default-vless-user")
	node := createTestNode(t, db, "tls-default-vless-node")
	node.Protocols = "[]"
	node.TLSEnabled = true
	node.TLSDomain = "panel.example.com"
	if err := db.Save(node).Error; err != nil {
		t.Fatalf("failed to update node tls settings: %v", err)
	}

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected tls auto provisioned proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one tls auto provisioned proxy, got %d", len(proxies))
	}
	if proxies[0].Protocol != "vless" {
		t.Fatalf("expected vless proxy for tls-enabled node without explicit protocol preference, got %s", proxies[0].Protocol)
	}
	if got := proxies[0].Settings["security"]; got != "tls" {
		t.Fatalf("expected tls security, got %#v", got)
	}
	if got := proxies[0].Settings["server_name"]; got != "panel.example.com" {
		t.Fatalf("expected server_name to inherit tls domain, got %#v", got)
	}
}

func TestGetAccessibleProxies_AutoProvisionedProxyInheritsNodeTLS(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "tls-auto-provision-user")
	node := createTestNode(t, db, "tls-node")
	node.Protocols = `["shadowsocks","vmess"]`
	node.TLSEnabled = true
	node.TLSDomain = "panel.example.com"
	if err := db.Save(node).Error; err != nil {
		t.Fatalf("failed to update node tls settings: %v", err)
	}

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected tls auto provisioned proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one tls auto provisioned proxy, got %d", len(proxies))
	}
	if proxies[0].Protocol != "vmess" {
		t.Fatalf("expected vmess proxy for tls-enabled node, got %s", proxies[0].Protocol)
	}
	if got := proxies[0].Settings["security"]; got != "tls" {
		t.Fatalf("expected tls security, got %#v", got)
	}
	if got := proxies[0].Settings["server"]; got != node.Address {
		t.Fatalf("expected server address to use node address, got %#v", got)
	}
	if got := proxies[0].Settings["server_name"]; got != "panel.example.com" {
		t.Fatalf("expected server_name to inherit tls domain, got %#v", got)
	}
}

func TestGetAccessibleProxies_ReconcilesExistingAutoProvisionedProxyToTLS(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "tls-reconcile-user")
	node := createTestNode(t, db, "tls-reconcile-node")
	node.TLSEnabled = true
	node.TLSDomain = "edge.example.com"
	if err := db.Save(node).Error; err != nil {
		t.Fatalf("failed to update node tls settings: %v", err)
	}

	nodeRef := node.ID
	existingProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeRef,
		Name:     "legacy-auto-proxy",
		Protocol: "vmess",
		Port:     24001,
		Host:     node.Address,
		Settings: map[string]any{
			"uuid":     "3f9b4ca6-7df4-4dd9-a61e-bba0e4d2c2d3",
			"alterId":  0,
			"network":  "tcp",
			"security": "none",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(existingProxy).Error; err != nil {
		t.Fatalf("failed to create legacy auto provisioned proxy: %v", err)
	}

	var syncedNodeID int64
	service.WithConfigSyncHook(func(nodeID int64, source, reason string) {
		syncedNodeID = nodeID
	})

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected reconciled proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one reconciled proxy, got %d", len(proxies))
	}
	if got := proxies[0].Settings["security"]; got != "tls" {
		t.Fatalf("expected existing proxy security upgraded to tls, got %#v", got)
	}
	if got := proxies[0].Settings["server"]; got != node.Address {
		t.Fatalf("expected existing proxy server upgraded to node address, got %#v", got)
	}
	if syncedNodeID != node.ID {
		t.Fatalf("expected config sync for node %d, got %d", node.ID, syncedNodeID)
	}

	var persisted repository.Proxy
	if err := db.First(&persisted, existingProxy.ID).Error; err != nil {
		t.Fatalf("failed to reload reconciled proxy: %v", err)
	}
	if got := persisted.Settings["security"]; got != "tls" {
		t.Fatalf("expected persisted proxy security to be tls, got %#v", got)
	}
}

func TestGetSubscriptionProxies_IncludesMultipleNodes(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "subscription-multi-node-user")

	nodeOne := createTestNode(t, db, "subscription-node-one")
	nodeOne.Protocols = `["vmess"]`
	if err := db.Save(nodeOne).Error; err != nil {
		t.Fatalf("failed to update node one protocols: %v", err)
	}

	nodeTwo := createTestNode(t, db, "subscription-node-two")
	nodeTwo.Protocols = `["vmess"]`
	if err := db.Save(nodeTwo).Error; err != nil {
		t.Fatalf("failed to update node two protocols: %v", err)
	}

	nodeOneRef := nodeOne.ID
	existingProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &nodeOneRef,
		Name:     "existing-user-proxy",
		Protocol: "vmess",
		Port:     21001,
		Host:     nodeOne.Address,
		Settings: map[string]any{
			"uuid": "12345678-1234-1234-1234-123456789012",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(existingProxy).Error; err != nil {
		t.Fatalf("failed to create existing proxy: %v", err)
	}

	proxies, _, err := service.GetSubscriptionProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected subscription proxies, got error: %v", err)
	}
	if len(proxies) != 2 {
		t.Fatalf("expected two subscription proxies, got %d", len(proxies))
	}

	nodeIDs := map[int64]bool{}
	for _, proxyModel := range proxies {
		if proxyModel.NodeID != nil {
			nodeIDs[*proxyModel.NodeID] = true
		}
	}
	if !nodeIDs[nodeOne.ID] || !nodeIDs[nodeTwo.ID] {
		t.Fatalf("expected proxies from nodes %d and %d, got %+v", nodeOne.ID, nodeTwo.ID, nodeIDs)
	}

	var persisted []*repository.Proxy
	if err := db.Where("user_id = ?", user.ID).Order("id asc").Find(&persisted).Error; err != nil {
		t.Fatalf("failed to load persisted proxies: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected two persisted proxies after subscription provisioning, got %d", len(persisted))
	}
}

func TestGetAccessibleProxies_ReassignsWhenExistingProxyNodeUnhealthy(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "reassign-on-unhealthy-user")
	badNode := createTestNode(t, db, "bad-node")
	badNode.Status = repository.NodeStatusUnhealthy
	if err := db.Save(badNode).Error; err != nil {
		t.Fatalf("failed to mark bad node unhealthy: %v", err)
	}
	goodNode := createTestNode(t, db, "good-node")

	badNodeRef := badNode.ID
	existingProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &badNodeRef,
		Name:     "stale-auto-proxy",
		Protocol: "vmess",
		Port:     24009,
		Host:     badNode.Address,
		Settings: map[string]any{
			"uuid": "79dc0ef2-b56d-4430-9e2b-baf0ce8ebd7b",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(existingProxy).Error; err != nil {
		t.Fatalf("failed to create unhealthy proxy: %v", err)
	}

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected healthy fallback proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one accessible proxy, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != goodNode.ID {
		t.Fatalf("expected reassigned proxy on node %d, got %+v", goodNode.ID, proxies[0].NodeID)
	}

	var assignment repository.UserNodeAssignment
	if err := db.First(&assignment, "user_id = ?", user.ID).Error; err != nil {
		t.Fatalf("expected reassigned node assignment, got error: %v", err)
	}
	if assignment.NodeID != goodNode.ID {
		t.Fatalf("expected assignment moved to node %d, got %d", goodNode.ID, assignment.NodeID)
	}
}

func TestGetSubscriptionProxies_ExcludesUnhealthyExistingNode(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "subscription-healthy-only-user")
	badNode := createTestNode(t, db, "subscription-bad-node")
	badNode.Status = repository.NodeStatusUnhealthy
	if err := db.Save(badNode).Error; err != nil {
		t.Fatalf("failed to mark subscription bad node unhealthy: %v", err)
	}
	goodNode := createTestNode(t, db, "subscription-good-node")
	goodNode.Protocols = `["vmess"]`
	if err := db.Save(goodNode).Error; err != nil {
		t.Fatalf("failed to update good node protocols: %v", err)
	}

	badNodeRef := badNode.ID
	badProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &badNodeRef,
		Name:     "bad-existing-proxy",
		Protocol: "vmess",
		Port:     25001,
		Host:     badNode.Address,
		Settings: map[string]any{
			"uuid": "80f5a7b7-523f-4b5d-9225-f56d37c18b9b",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(badProxy).Error; err != nil {
		t.Fatalf("failed to create bad subscription proxy: %v", err)
	}

	proxies, _, err := service.GetSubscriptionProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected subscription proxies, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one healthy subscription proxy, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != goodNode.ID {
		t.Fatalf("expected subscription proxy on node %d, got %+v", goodNode.ID, proxies[0].NodeID)
	}
}

func TestGetAccessibleProxies_SkipsMissingExistingNode(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "missing-node-access-user")
	orphanNode := createTestNode(t, db, "missing-node-access-orphan")
	goodNode := createTestNode(t, db, "missing-node-access-good")

	orphanNodeRef := orphanNode.ID
	orphanProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &orphanNodeRef,
		Name:     "orphan-access-proxy",
		Protocol: "vmess",
		Port:     25011,
		Host:     orphanNode.Address,
		Settings: map[string]any{
			"uuid": "f8fd4ccc-3a93-4a68-bf63-2018a3c6099e",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(orphanProxy).Error; err != nil {
		t.Fatalf("failed to create orphan access proxy: %v", err)
	}
	if err := db.Delete(&repository.Node{}, orphanNode.ID).Error; err != nil {
		t.Fatalf("failed to delete orphan node: %v", err)
	}

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected healthy fallback proxy, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one accessible proxy, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != goodNode.ID {
		t.Fatalf("expected fallback proxy on node %d, got %+v", goodNode.ID, proxies[0].NodeID)
	}
}

func TestGetSubscriptionProxies_SkipsMissingExistingNode(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "missing-node-subscription-user")
	orphanNode := createTestNode(t, db, "missing-node-subscription-orphan")
	goodNode := createTestNode(t, db, "missing-node-subscription-good")
	goodNode.Protocols = `["vmess"]`
	if err := db.Save(goodNode).Error; err != nil {
		t.Fatalf("failed to update good node protocols: %v", err)
	}

	orphanNodeRef := orphanNode.ID
	orphanProxy := &repository.Proxy{
		UserID:   user.ID,
		NodeID:   &orphanNodeRef,
		Name:     "orphan-subscription-proxy",
		Protocol: "vmess",
		Port:     25012,
		Host:     orphanNode.Address,
		Settings: map[string]any{
			"uuid": "49d03fe8-62da-4d80-b224-12b2826ddaf6",
		},
		Enabled: true,
		Remark:  "auto provisioned",
	}
	if err := db.Create(orphanProxy).Error; err != nil {
		t.Fatalf("failed to create orphan subscription proxy: %v", err)
	}
	if err := db.Delete(&repository.Node{}, orphanNode.ID).Error; err != nil {
		t.Fatalf("failed to delete orphan node: %v", err)
	}

	proxies, _, err := service.GetSubscriptionProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected subscription proxies, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected one healthy subscription proxy, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != goodNode.ID {
		t.Fatalf("expected subscription proxy on node %d, got %+v", goodNode.ID, proxies[0].NodeID)
	}
}

func TestGetAccessibleProxies_PrefersLowerTrafficPressureWhenAssignmentCountsTie(t *testing.T) {
	service, db := setupTestService(t)
	user := createTestUser(t, db, "traffic-aware-user")

	highPressureNode := createTestNode(t, db, "high-pressure-node")
	highPressureNode.TrafficLimit = 1000
	highPressureNode.TrafficTotal = 700
	highPressureNode.AlertTrafficThreshold = 80
	if err := db.Save(highPressureNode).Error; err != nil {
		t.Fatalf("failed to update high-pressure node: %v", err)
	}

	lowPressureNode := createTestNode(t, db, "low-pressure-node")
	lowPressureNode.TrafficLimit = 1000
	lowPressureNode.TrafficTotal = 200
	lowPressureNode.AlertTrafficThreshold = 80
	if err := db.Save(lowPressureNode).Error; err != nil {
		t.Fatalf("failed to update low-pressure node: %v", err)
	}

	createNodeProxy(t, db, highPressureNode.ID, "high-pressure-proxy", 11001)
	preferredProxy := createNodeProxy(t, db, lowPressureNode.ID, "low-pressure-proxy", 11002)

	proxies, _, err := service.GetAccessibleProxies(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("expected assigned node proxies, got error: %v", err)
	}
	if len(proxies) != 1 {
		t.Fatalf("expected only one assigned proxy, got %d", len(proxies))
	}
	if proxies[0].NodeID == nil || *proxies[0].NodeID != lowPressureNode.ID {
		t.Fatalf("expected assignment to lower traffic pressure node %d, got %+v", lowPressureNode.ID, proxies[0].NodeID)
	}
	if proxies[0].ID != preferredProxy.ID {
		t.Fatalf("expected proxy %d from lower traffic node, got %d", preferredProxy.ID, proxies[0].ID)
	}
}
