package node

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
	pkgerrors "v/pkg/errors"
)

func setupNodeServiceDeleteTestDB(t *testing.T) *gorm.DB {
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
		&repository.UserNodeAssignment{},
		&repository.Trial{},
	); err != nil {
		t.Fatalf("failed to migrate test schema: %v", err)
	}

	return db
}

func TestServiceDelete_RemovesNodeBoundProxies(t *testing.T) {
	db := setupNodeServiceDeleteTestDB(t)
	ctx := context.Background()

	nodeRepo := repository.NewNodeRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	service := NewService(nodeRepo, assignmentRepo, proxyRepo, logger.NewNopLogger())

	deletedNode := &repository.Node{
		Name:    "delete-me-node",
		Address: "delete-me.example.com",
		Token:   "delete-me-token",
		Status:  repository.NodeStatusOnline,
	}
	if err := db.Create(deletedNode).Error; err != nil {
		t.Fatalf("failed to create deleted node: %v", err)
	}

	targetNode := &repository.Node{
		Name:    "target-node",
		Address: "target.example.com",
		Token:   "target-token",
		Status:  repository.NodeStatusOnline,
	}
	if err := db.Create(targetNode).Error; err != nil {
		t.Fatalf("failed to create target node: %v", err)
	}

	user := &repository.User{
		Username:     "delete-node-user",
		PasswordHash: "hashed-password",
		Enabled:      true,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	if err := assignmentRepo.Assign(ctx, user.ID, deletedNode.ID); err != nil {
		t.Fatalf("failed to assign user to deleted node: %v", err)
	}

	deletedNodeRef := deletedNode.ID
	targetNodeRef := targetNode.ID
	fixtures := []*repository.Proxy{
		{
			UserID:   0,
			NodeID:   &deletedNodeRef,
			Name:     "shared-on-deleted-node",
			Protocol: "vmess",
			Port:     22001,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
		{
			UserID:   user.ID,
			NodeID:   &deletedNodeRef,
			Name:     "user-on-deleted-node",
			Protocol: "vmess",
			Port:     22002,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
		{
			UserID:   user.ID,
			NodeID:   &deletedNodeRef,
			Name:     "disabled-on-deleted-node",
			Protocol: "vmess",
			Port:     22003,
			Host:     "127.0.0.1",
			Enabled:  false,
		},
		{
			UserID:   0,
			NodeID:   &targetNodeRef,
			Name:     "shared-on-target-node",
			Protocol: "vmess",
			Port:     22004,
			Host:     "127.0.0.1",
			Enabled:  true,
		},
	}
	for _, proxyModel := range fixtures {
		if err := db.Create(proxyModel).Error; err != nil {
			t.Fatalf("failed to create proxy fixture %q: %v", proxyModel.Name, err)
		}
	}

	if err := service.Delete(ctx, deletedNode.ID); err != nil {
		t.Fatalf("expected node delete to succeed, got %v", err)
	}

	_, err := nodeRepo.GetByID(ctx, deletedNode.ID)
	if !pkgerrors.IsNotFound(err) {
		t.Fatalf("expected deleted node to be gone, got %v", err)
	}

	assignment, err := assignmentRepo.GetByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to reload assignment: %v", err)
	}
	if assignment == nil || assignment.NodeID != targetNode.ID {
		t.Fatalf("expected reassignment to node %d, got %+v", targetNode.ID, assignment)
	}

	var remaining []*repository.Proxy
	if err := db.Order("id asc").Find(&remaining).Error; err != nil {
		t.Fatalf("failed to load remaining proxies: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected exactly one proxy to remain, got %d", len(remaining))
	}
	if remaining[0].NodeID == nil || *remaining[0].NodeID != targetNode.ID {
		t.Fatalf("expected remaining proxy to stay on node %d, got %+v", targetNode.ID, remaining[0].NodeID)
	}
	if remaining[0].Name != "shared-on-target-node" {
		t.Fatalf("expected target-node proxy to remain, got %q", remaining[0].Name)
	}
}

func TestServiceUpdateSSHConfigPersistsFields(t *testing.T) {
	db := setupNodeServiceDeleteTestDB(t)
	ctx := context.Background()

	nodeRepo := repository.NewNodeRepository(db)
	assignmentRepo := repository.NewUserNodeAssignmentRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	service := NewService(nodeRepo, assignmentRepo, proxyRepo, logger.NewNopLogger())

	nodeModel := &repository.Node{
		Name:    "ssh-node",
		Address: "198.51.100.10",
		Token:   "ssh-node-token",
		Status:  repository.NodeStatusOnline,
	}
	if err := db.Create(nodeModel).Error; err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	if err := service.UpdateSSHConfig(ctx, nodeModel.ID, "198.51.100.20", 2222, "admin", "secret-password", ""); err != nil {
		t.Fatalf("expected UpdateSSHConfig to succeed, got %v", err)
	}

	reloaded, err := service.GetByID(ctx, nodeModel.ID)
	if err != nil {
		t.Fatalf("failed to reload node: %v", err)
	}

	rawNode, err := nodeRepo.GetByID(ctx, nodeModel.ID)
	if err != nil {
		t.Fatalf("failed to reload raw node: %v", err)
	}

	if reloaded.SSHHost != "198.51.100.20" {
		t.Fatalf("expected SSHHost to be persisted, got %q", reloaded.SSHHost)
	}
	if reloaded.SSHPort != 2222 {
		t.Fatalf("expected SSHPort 2222, got %d", reloaded.SSHPort)
	}
	if reloaded.SSHUser != "admin" {
		t.Fatalf("expected SSHUser admin, got %q", reloaded.SSHUser)
	}
	if reloaded.SSHPassword != "secret-password" {
		t.Fatalf("expected SSHPassword to be persisted, got %q", reloaded.SSHPassword)
	}
	if rawNode.SSHPassword == "secret-password" {
		t.Fatal("expected raw SSHPassword to be encrypted at rest")
	}
}
