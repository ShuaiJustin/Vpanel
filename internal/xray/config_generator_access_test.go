package xray

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

func TestGenerateForNode_FiltersInaccessibleUserProxies(t *testing.T) {
	repo := newMockProxyRepoForSync()
	nodeID := int64(7)

	require.NoError(t, repo.Create(context.Background(), &repository.Proxy{
		UserID:   0,
		NodeID:   &nodeID,
		Enabled:  true,
		Protocol: "vless",
		Port:     12001,
		Settings: map[string]any{"uuid": "shared-user"},
	}))
	require.NoError(t, repo.Create(context.Background(), &repository.Proxy{
		UserID:   101,
		NodeID:   &nodeID,
		Enabled:  true,
		Protocol: "vmess",
		Port:     12002,
		Settings: map[string]any{"uuid": "active-user"},
	}))
	require.NoError(t, repo.Create(context.Background(), &repository.Proxy{
		UserID:   202,
		NodeID:   &nodeID,
		Enabled:  true,
		Protocol: "trojan",
		Port:     12003,
		Settings: map[string]any{"password": "expired-user"},
	}))

	generator := NewConfigGenerator(repo, nil, nil, logger.NewNopLogger()).
		WithUserAccessCheck(func(ctx context.Context, userID int64) error {
			switch userID {
			case 101:
				return nil
			case 202:
				return apperrors.NewForbiddenError("user account has expired")
			default:
				return nil
			}
		})

	config, err := generator.GenerateForNode(context.Background(), nodeID)
	require.NoError(t, err)
	require.Len(t, config.Inbounds, 3)

	ports := make([]int, 0, len(config.Inbounds))
	for _, inbound := range config.Inbounds {
		ports = append(ports, inbound.Port)
	}

	assert.Contains(t, ports, 12001)
	assert.Contains(t, ports, 12002)
	assert.NotContains(t, ports, 12003)
}

func setupConfigGeneratorNodeTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&repository.Node{}, &repository.Proxy{}); err != nil {
		t.Fatalf("failed to migrate test tables: %v", err)
	}
	return db
}

func TestGenerateForNode_DisablesProxyInboundsWhenNodeTrafficLimitExceeded(t *testing.T) {
	db := setupConfigGeneratorNodeTestDB(t)
	ctx := context.Background()
	nodeID := int64(7)
	nodeRepo := repository.NewNodeRepository(db)
	proxyRepo := repository.NewProxyRepository(db)

	require.NoError(t, nodeRepo.Create(ctx, &repository.Node{
		ID:           nodeID,
		Name:         "quota-node",
		Address:      "127.0.0.1",
		Status:       repository.NodeStatusUnhealthy,
		TrafficTotal: 102,
		TrafficLimit: 100,
	}))
	require.NoError(t, proxyRepo.Create(ctx, &repository.Proxy{
		UserID:   0,
		NodeID:   &nodeID,
		Enabled:  true,
		Protocol: "vmess",
		Port:     12002,
		Host:     "127.0.0.1",
		Settings: map[string]any{"uuid": "quota-user"},
	}))

	generator := NewConfigGenerator(proxyRepo, nil, nodeRepo, logger.NewNopLogger())
	config, err := generator.GenerateForNode(ctx, nodeID)
	require.NoError(t, err)
	require.Len(t, config.Inbounds, 1)
	assert.Equal(t, "api", config.Inbounds[0].Tag)
}
