package node

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestPendingGraceExpiresDespiteHeartbeatUpdates(t *testing.T) {
	hc := NewHealthChecker(nil, nil, nil, nil, nil, nil)
	node := &repository.Node{ID: 1, SyncStatus: repository.NodeSyncStatusPending}
	now := time.Now()
	require.True(t, hc.pendingSyncWithinGrace(node, now))
	node.UpdatedAt = now.Add(10 * time.Minute)
	require.False(t, hc.pendingSyncWithinGrace(node, now.Add(2*time.Minute)))
	node.SyncStatus = repository.NodeSyncStatusSynced
	require.False(t, hc.pendingSyncWithinGrace(node, now.Add(3*time.Minute)))
	node.SyncStatus = repository.NodeSyncStatusPending
	require.True(t, hc.pendingSyncWithinGrace(node, now.Add(4*time.Minute)))
}

func TestNetworkQualityRecordsTCPFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	repo := samplingProxyRepo{proxies: []*repository.Proxy{{ID: 1, Enabled: true, Port: port}}}
	hc := NewHealthChecker(nil, nil, repo, nil, nil, logger.NewNopLogger())
	result := &HealthCheckResult{Status: repository.HealthCheckStatusSuccess}
	hc.applyNetworkQuality(&repository.Node{ID: 1, Address: "127.0.0.1"}, result)
	require.Equal(t, -1, result.Latency)
	require.Equal(t, repository.HealthCheckStatusFailed, result.Status)
	require.Contains(t, result.Message, "TCP失败率 100%")
}

type samplingProxyRepo struct {
	repository.ProxyRepository
	proxies []*repository.Proxy
}

type recoveryNodeRepository struct {
	repository.NodeRepository
	node *repository.Node
}

func (r recoveryNodeRepository) UpdateMetrics(context.Context, int64, int, int) error { return nil }
func (r recoveryNodeRepository) UpdateStatus(_ context.Context, _ int64, status string) error {
	r.node.Status = status
	return nil
}

func TestRecoveryRequiresStableWindowAfterTransientSuccess(t *testing.T) {
	n := &repository.Node{ID: 1, Status: repository.NodeStatusUnhealthy}
	hc := NewHealthChecker(nil, recoveryNodeRepository{node: n}, nil, nil, nil, logger.NewNopLogger())
	success := &HealthCheckResult{Status: repository.HealthCheckStatusSuccess, Latency: 150}
	for i := 0; i < 2; i++ {
		hc.handleCheckResult(n, success)
	}
	require.Equal(t, repository.NodeStatusUnhealthy, n.Status)
	hc.handleCheckResult(n, &HealthCheckResult{Status: repository.HealthCheckStatusFailed})
	for i := 0; i < hc.config.HealthyThreshold-1; i++ {
		hc.handleCheckResult(n, success)
	}
	require.Equal(t, repository.NodeStatusUnhealthy, n.Status)
	hc.handleCheckResult(n, success)
	require.Equal(t, repository.NodeStatusOnline, n.Status)
}

func (r samplingProxyRepo) GetByNodeID(context.Context, int64) ([]*repository.Proxy, error) {
	return r.proxies, nil
}

func TestPartialTCPFailureDoesNotReportHealthy(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer agent.Close()
	host, p, err := net.SplitHostPort(agent.Listener.Addr().String())
	require.NoError(t, err)
	agentPort, _ := strconv.Atoi(p)
	healthy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer healthy.Close()
	bad, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	badPort := bad.Addr().(*net.TCPAddr).Port
	require.NoError(t, bad.Close())
	repo := samplingProxyRepo{proxies: []*repository.Proxy{{ID: 1, Port: healthy.Addr().(*net.TCPAddr).Port}, {ID: 2, Port: badPort}}}
	hc := NewHealthChecker(&HealthCheckConfig{Interval: time.Second, Timeout: time.Second}, nil, repo, nil, nil, logger.NewNopLogger())
	now := time.Now()
	result := hc.performCheck(&repository.Node{ID: 1, Address: host, Port: agentPort, Status: "online", SyncStatus: "synced", LastSeenAt: &now, XrayRunning: true})
	require.Equal(t, repository.HealthCheckStatusFailed, result.Status)
	require.Contains(t, result.Message, "at least one")
}

func TestProxySamplingRotatesAndRetainsFailures(t *testing.T) {
	proxies := []*repository.Proxy{}
	for i := int64(1); i <= 7; i++ {
		proxies = append(proxies, &repository.Proxy{ID: i, Port: int(20000 + i)})
	}
	seen := map[int64]bool{}
	cursor := 0
	for i := 0; i < 3; i++ {
		var sample []*repository.Proxy
		sample, cursor = selectProxySample(proxies, nil, cursor)
		require.Len(t, sample, 3)
		for _, p := range sample {
			seen[p.ID] = true
		}
	}
	require.Len(t, seen, 7)
	for i := 0; i < 4; i++ {
		var sample []*repository.Proxy
		sample, cursor = selectProxySample(proxies, map[int64]bool{6: true}, cursor)
		require.Equal(t, int64(6), sample[0].ID)
	}
}
