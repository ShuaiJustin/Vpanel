package node

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
	apperrors "v/pkg/errors"
)

func newHealthAlertOutbox(t *testing.T) (*notification.Service, func(int)) {
	t.Helper()
	svc := notification.NewService(&notification.NotificationConfig{
		AdminEmail:      "admin@example.com",
		EnabledChannels: map[notification.NotificationChannel]bool{notification.ChannelEmail: true},
		EnabledTypes:    map[notification.NotificationType]bool{notification.NotificationNodeStatusChange: true},
	})
	dir := filepath.Join(t.TempDir(), "outbox")
	require.NoError(t, svc.ConfigureOutbox(dir))
	return svc, func(want int) {
		t.Helper()
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, want)
	}
}

func TestICMPLossWithoutTCPDegradationDoesNotNotify(t *testing.T) {
	for _, loss := range []int{20, 50, 100} {
		t.Run(strconv.Itoa(loss), func(t *testing.T) {
			bin := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(bin, "ping"), []byte(fmt.Sprintf("#!/bin/sh\necho '%d%% packet loss'\n", loss)), 0700))
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			defer listener.Close()
			go func() {
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					conn.Close()
				}
			}()
			proxyRepo := samplingProxyRepo{proxies: []*repository.Proxy{{ID: 1, Enabled: true, Port: listener.Addr().(*net.TCPAddr).Port}}}
			hc := NewHealthChecker(nil, nil, proxyRepo, nil, nil, logger.NewNopLogger())
			svc, count := newHealthAlertOutbox(t)
			hc.SetNotificationService(svc)
			result := &HealthCheckResult{Status: repository.HealthCheckStatusSuccess}
			hc.applyNetworkQuality(&repository.Node{ID: 1, Address: "127.0.0.1", Status: repository.NodeStatusOnline}, result)
			require.Equal(t, repository.HealthCheckStatusSuccess, result.Status)
			require.Contains(t, result.Message, fmt.Sprintf("ICMP丢包率 %d%%", loss))
			count(0)
		})
	}
}

func TestHealthAlertsRequireConfirmedFailureAndRecovery(t *testing.T) {
	n := &repository.Node{ID: 1, Status: repository.NodeStatusOnline}
	hc := NewHealthChecker(nil, recoveryNodeRepository{node: n}, nil, nil, nil, logger.NewNopLogger())
	svc, count := newHealthAlertOutbox(t)
	hc.SetNotificationService(svc)
	failed := &HealthCheckResult{Status: repository.HealthCheckStatusFailed, Message: "TCP failure"}
	success := &HealthCheckResult{Status: repository.HealthCheckStatusSuccess, Latency: 150}
	for i := 0; i < hc.config.Load().UnhealthyThreshold-1; i++ {
		hc.handleCheckResult(n, failed)
		count(0)
	}
	hc.handleCheckResult(n, failed)
	require.Equal(t, repository.NodeStatusUnhealthy, n.Status)
	count(1)
	hc.handleCheckResult(n, failed)
	count(1)
	for i := 0; i < hc.config.Load().HealthyThreshold-1; i++ {
		hc.handleCheckResult(n, success)
		count(1)
	}
	hc.handleCheckResult(n, success)
	require.Equal(t, repository.NodeStatusOnline, n.Status)
	count(2)
	hc.handleCheckResult(n, success)
	count(2)
}

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
	require.Equal(t, repository.HealthCheckStatusSuccess, result.Status)
	require.Contains(t, result.Message, "TCP失败率 100%")
}

func TestNetworkQualityDistinguishesSlowAndFailedConnections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		failed int
		slow   int
		want   string
	}{
		{"slow but reachable", 0, 5, repository.HealthCheckStatusSuccess},
		{"one failed connection", 1, 4, repository.HealthCheckStatusSuccess},
		{"repeated connection failures", 2, 0, repository.HealthCheckStatusSuccess},
		{"confirmed failure", 0, 0, repository.HealthCheckStatusFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := &HealthCheckResult{Status: tc.want}
			recordTCPProbeQuality(result, tc.failed, tc.slow)
			require.Equal(t, tc.want, result.Status)
			if tc.slow >= 3 {
				require.Contains(t, result.Message, "TCP 慢连接偏多")
			}
		})
	}
}

func TestXrayStatusUsesBracketedIPv6URL(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 loopback unavailable")
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/xray/status", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	hc := NewHealthChecker(nil, nil, nil, nil, nil, logger.NewNopLogger())
	require.True(t, hc.checkXray("::1", listener.Addr().(*net.TCPAddr).Port))
}

func TestConfigSyncUsesBracketedIPv6URL(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("IPv6 loopback unavailable")
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/config/sync", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()
	svc := &configSync{config: ConfigSyncConfig{SyncTimeout: time.Second}, logger: logger.NewNopLogger()}
	require.NoError(t, svc.performSync(context.Background(), &repository.Node{Address: "::1", Port: listener.Addr().(*net.TCPAddr).Port, Token: "test-token"}, &NodeConfig{}))
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
	for i := 0; i < hc.config.Load().HealthyThreshold-1; i++ {
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

// A user over quota keeps their proxy row, but the node is told to close the
// port. That closed port is the intended state, not a node failure.
func TestHealthCheckIgnoresPortsOfUsersWithoutAccess(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer agent.Close()
	host, p, err := net.SplitHostPort(agent.Listener.Addr().String())
	require.NoError(t, err)
	agentPort, _ := strconv.Atoi(p)
	open, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer open.Close()
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	closedPort := closed.Addr().(*net.TCPAddr).Port
	require.NoError(t, closed.Close())

	repo := samplingProxyRepo{proxies: []*repository.Proxy{
		{ID: 1, UserID: 10, Enabled: true, Port: open.Addr().(*net.TCPAddr).Port},
		{ID: 2, UserID: 20, Enabled: true, Port: closedPort},
	}}
	hc := NewHealthChecker(&HealthCheckConfig{Interval: time.Second, Timeout: time.Second}, nil, repo, nil, nil, logger.NewNopLogger())
	hc.SetUserAccessCheck(func(_ context.Context, userID int64) error {
		if userID == 20 {
			return apperrors.NewForbiddenError("traffic limit exceeded")
		}
		return nil
	})
	now := time.Now()
	node := &repository.Node{ID: 1, Address: host, Port: agentPort, Status: "online", SyncStatus: "synced", LastSeenAt: &now, XrayRunning: true}

	health := hc.checkReachableProxyEndpoint(node)
	require.Equal(t, 1, health.CheckedCount, "only the entitled user's port is part of the node config")
	require.True(t, health.AllReachable)
	require.NotEqual(t, repository.HealthCheckStatusFailed, hc.performCheck(node).Status)
}

// The agent's own port answering says the process is alive, not that it still
// talks to the panel. In production one node passed 2,295 checks while its
// agent had not reported for 17 days.
func TestHealthCheckFailsWhenAgentStoppedReporting(t *testing.T) {
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer agent.Close()
	host, p, err := net.SplitHostPort(agent.Listener.Addr().String())
	require.NoError(t, err)
	agentPort, _ := strconv.Atoi(p)
	hc := NewHealthChecker(&HealthCheckConfig{Interval: time.Second, Timeout: time.Second}, nil, samplingProxyRepo{}, nil, nil, logger.NewNopLogger())
	longAgo := time.Now().Add(-17 * 24 * time.Hour)
	silent := &repository.Node{ID: 1, Address: host, Port: agentPort, Status: "online", SyncStatus: "synced", LastSeenAt: &longAgo, XrayRunning: true}

	// Right after a panel restart every agent looks silent; do not judge yet.
	hc.startedAt = time.Now()
	require.Equal(t, repository.HealthCheckStatusSuccess, hc.performCheck(silent).Status)

	hc.startedAt = time.Now().Add(-agentSilenceStartupGrace - time.Second)
	result := hc.performCheck(silent)
	require.Equal(t, repository.HealthCheckStatusFailed, result.Status)
	require.Contains(t, result.Message, "未向面板上报心跳")

	recent := time.Now()
	reporting := &repository.Node{ID: 2, Address: host, Port: agentPort, Status: "online", SyncStatus: "synced", LastSeenAt: &recent, XrayRunning: true}
	require.Equal(t, repository.HealthCheckStatusSuccess, hc.performCheck(reporting).Status)
}

// Closed-on-purpose ports must not use up the sample: with most owners over
// quota the check would otherwise probe nothing and miss a dead port.
func TestHealthCheckKeepsSamplingPastDeniedOwners(t *testing.T) {
	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	deadPort := closed.Addr().(*net.TCPAddr).Port
	require.NoError(t, closed.Close())

	proxies := []*repository.Proxy{}
	for i := int64(1); i <= 6; i++ { // six owners over quota, listed first
		proxies = append(proxies, &repository.Proxy{ID: i, UserID: i, Enabled: true, Port: int(21000 + i)})
	}
	proxies = append(proxies, &repository.Proxy{ID: 7, UserID: 7, Enabled: true, Port: deadPort})
	hc := NewHealthChecker(&HealthCheckConfig{Interval: time.Second, Timeout: time.Second}, nil, samplingProxyRepo{proxies: proxies}, nil, nil, logger.NewNopLogger())
	lookups := 0
	hc.SetUserAccessCheck(func(_ context.Context, userID int64) error {
		lookups++
		if userID <= 6 {
			return apperrors.NewForbiddenError("traffic limit exceeded")
		}
		return nil
	})

	health := hc.checkReachableProxyEndpoint(&repository.Node{ID: 1, Address: "127.0.0.1"})
	require.Equal(t, 1, health.CheckedCount, "the only port that should be open was probed in the same check")
	require.False(t, health.AllReachable)
	require.LessOrEqual(t, lookups, 12)
}
