package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
)

type proxySyncStatusNodeRepo struct {
	repository.NodeRepository
	nodeID   int64
	status   string
	syncedAt *time.Time
	calls    int
}

func (r *proxySyncStatusNodeRepo) UpdateSyncStatus(_ context.Context, id int64, status string, syncedAt *time.Time) error {
	r.nodeID = id
	r.status = status
	r.syncedAt = syncedAt
	r.calls++
	return nil
}

func TestProxyHandler_QueueNodeConfigSyncMarksNodePending(t *testing.T) {
	nodeID := int64(3)
	nodeRepo := &proxySyncStatusNodeRepo{}
	recoveryTracker := NewNodeRecoveryTracker(logger.NewNopLogger())
	handler := NewProxyHandler(nil, nil, logger.NewNopLogger()).
		WithNodeRepository(nodeRepo).
		WithRecoveryTracker(recoveryTracker)

	handler.queueNodeConfigSync(context.Background(), &nodeID, "proxy_update", "proxy updated")

	require.Equal(t, 1, nodeRepo.calls)
	assert.Equal(t, nodeID, nodeRepo.nodeID)
	assert.Equal(t, repository.NodeSyncStatusPending, nodeRepo.status)
	assert.Nil(t, nodeRepo.syncedAt)

	commands := recoveryTracker.GetPendingCommands(nodeID)
	require.Len(t, commands, 2)
	assert.Equal(t, commandTypeConfigSync, commands[0].Type)
	assert.Equal(t, commandTypeXrayRestart, commands[1].Type)
}
