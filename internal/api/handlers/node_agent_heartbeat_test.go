package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"v/internal/database/repository"
)

func TestShouldPromoteNodeOnlineFromHeartbeat(t *testing.T) {
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOffline))
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOnline))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy))
}
