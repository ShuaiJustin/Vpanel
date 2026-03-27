package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"v/internal/database/repository"
)

func TestShouldTrustRecentHeartbeat(t *testing.T) {
	now := time.Now()
	recent := now.Add(-20 * time.Second)
	stale := now.Add(-3 * time.Minute)

	assert.True(t, shouldTrustRecentHeartbeat(&repository.Node{
		LastSeenAt:  &recent,
		XrayRunning: true,
	}, 30*time.Second, now))

	assert.False(t, shouldTrustRecentHeartbeat(&repository.Node{
		LastSeenAt:  &stale,
		XrayRunning: true,
	}, 30*time.Second, now))

	assert.False(t, shouldTrustRecentHeartbeat(&repository.Node{
		LastSeenAt:  &recent,
		XrayRunning: false,
	}, 30*time.Second, now))

	assert.False(t, shouldTrustRecentHeartbeat(nil, 30*time.Second, now))
}

func TestShouldAcceptHeartbeatFallback(t *testing.T) {
	assert.False(t, shouldAcceptHeartbeatFallback(false, false, false))
	assert.True(t, shouldAcceptHeartbeatFallback(true, false, false))
	assert.True(t, shouldAcceptHeartbeatFallback(true, true, true))
	assert.False(t, shouldAcceptHeartbeatFallback(true, true, false))
}

func TestHeartbeatFallbackMessage(t *testing.T) {
	assert.Equal(t, "Recent heartbeat confirms Xray is running", heartbeatFallbackMessage(false))
	assert.Equal(t, "Recent heartbeat confirms Xray is running and at least one sampled proxy endpoint is reachable", heartbeatFallbackMessage(true))
}

func TestResolveHealthCheckProxyHost_PrefersNodeAddressForAutoProvisionedProxy(t *testing.T) {
	nodeModel := &repository.Node{Address: "node.example.com"}
	proxyModel := &repository.Proxy{
		Host:   "stale.example.com",
		Port:   20002,
		Remark: "auto provisioned",
	}

	assert.Equal(t, "node.example.com", resolveHealthCheckProxyHost(nodeModel, proxyModel))
}

func TestNodeTrafficLimitExceeded(t *testing.T) {
	assert.True(t, nodeTrafficLimitExceeded(&repository.Node{TrafficTotal: 100, TrafficLimit: 100}))
	assert.True(t, nodeTrafficLimitExceeded(&repository.Node{TrafficTotal: 120, TrafficLimit: 100}))
	assert.False(t, nodeTrafficLimitExceeded(&repository.Node{TrafficTotal: 99, TrafficLimit: 100}))
	assert.False(t, nodeTrafficLimitExceeded(&repository.Node{TrafficTotal: 100, TrafficLimit: 0}))
	assert.False(t, nodeTrafficLimitExceeded(nil))
}
