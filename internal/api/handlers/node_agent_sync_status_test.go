package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
)

type mockNodeSyncStatusUpdater struct {
	nodeID   int64
	status   string
	syncedAt *time.Time
	calls    int
}

func (m *mockNodeSyncStatusUpdater) UpdateSyncStatus(ctx context.Context, id int64, status string, syncedAt *time.Time) error {
	m.nodeID = id
	m.status = status
	m.syncedAt = syncedAt
	m.calls++
	return nil
}

func TestUpdateNodeSyncStatusFromCommandResult_MarksConfigSyncSuccessAsSynced(t *testing.T) {
	repo := &mockNodeSyncStatusUpdater{}

	err := updateNodeSyncStatusFromCommandResult(context.Background(), repo, 3, commandTypeConfigSync, true)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls)
	assert.Equal(t, int64(3), repo.nodeID)
	assert.Equal(t, repository.NodeSyncStatusSynced, repo.status)
	require.NotNil(t, repo.syncedAt)
}

func TestUpdateNodeSyncStatusFromCommandResult_MarksConfigSyncFailureAsFailed(t *testing.T) {
	repo := &mockNodeSyncStatusUpdater{}

	err := updateNodeSyncStatusFromCommandResult(context.Background(), repo, 3, commandTypeConfigSync, false)

	require.NoError(t, err)
	assert.Equal(t, 1, repo.calls)
	assert.Equal(t, repository.NodeSyncStatusFailed, repo.status)
	assert.Nil(t, repo.syncedAt)
}

func TestUpdateNodeSyncStatusFromCommandResult_IgnoresOtherCommands(t *testing.T) {
	repo := &mockNodeSyncStatusUpdater{}

	err := updateNodeSyncStatusFromCommandResult(context.Background(), repo, 3, commandTypeXrayStart, true)

	require.NoError(t, err)
	assert.Equal(t, 0, repo.calls)
}
