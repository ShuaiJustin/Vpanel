package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"v/internal/database/repository"
)

func TestShouldPromoteNodeOnlineFromHeartbeat(t *testing.T) {
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOffline, nil))
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOnline, nil))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, nil))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, &NodeMetrics{XrayRunning: false}))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, &NodeMetrics{XrayRunning: true}))
}
