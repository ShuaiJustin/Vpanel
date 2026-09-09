package node

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestShouldDeferProxyEndpointFailureForConfigSync(t *testing.T) {
	proxyHealth := sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: false,
		AllReachable: false,
	}

	assert.True(t, shouldDeferProxyEndpointFailureForConfigSync(&repository.Node{
		SyncStatus: repository.NodeSyncStatusPending,
	}, proxyHealth))
	assert.False(t, shouldDeferProxyEndpointFailureForConfigSync(&repository.Node{
		SyncStatus: repository.NodeSyncStatusSynced,
	}, proxyHealth))
	assert.False(t, shouldDeferProxyEndpointFailureForConfigSync(&repository.Node{
		SyncStatus: repository.NodeSyncStatusPending,
	}, sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: true,
	}))
	assert.False(t, shouldDeferProxyEndpointFailureForConfigSync(&repository.Node{
		SyncStatus: repository.NodeSyncStatusPending,
	}, sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: false,
		TLSFailure:   true,
	}))
}

func TestHeartbeatFallbackMessage(t *testing.T) {
	assert.Equal(t, "Recent heartbeat confirms Xray is running", heartbeatFallbackMessage(false))
	assert.Equal(t, "Recent heartbeat confirms Xray is running and at least one sampled proxy endpoint is reachable", heartbeatFallbackMessage(true))
}

func TestSampledProxyEndpointsHealthyForPrimary(t *testing.T) {
	assert.True(t, sampledProxyEndpointsHealthyForPrimary(sampledProxyEndpointHealth{}))
	assert.True(t, sampledProxyEndpointsHealthyForPrimary(sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: true,
		AllReachable: true,
	}))
	assert.False(t, sampledProxyEndpointsHealthyForPrimary(sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: true,
		AllReachable: false,
	}))
	assert.False(t, sampledProxyEndpointsHealthyForPrimary(sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: false,
		AllReachable: false,
	}))
}

func TestSampledProxyEndpointFailureMessageIncludesTarget(t *testing.T) {
	message := sampledProxyEndpointFailureMessage(sampledProxyEndpointHealth{
		FirstUnreachableTarget: "node.example.com:20001",
		FirstFailureReason:     "served TLS certificate expired",
	})

	assert.Contains(t, message, "node.example.com:20001")
	assert.Contains(t, message, "sampled proxy endpoint")
	assert.Contains(t, message, "certificate expired")
}

func TestProxyUsesTLS(t *testing.T) {
	assert.True(t, proxyUsesTLS(&repository.Proxy{
		Protocol: "vmess",
		Settings: map[string]any{"security": "tls"},
	}))
	assert.True(t, proxyUsesTLS(&repository.Proxy{
		Protocol: "trojan",
	}))
	assert.False(t, proxyUsesTLS(&repository.Proxy{
		Protocol: "trojan",
		Settings: map[string]any{"security": "none"},
	}))
	assert.True(t, proxyUsesTLS(&repository.Proxy{
		Protocol: "vmess",
		Settings: map[string]any{"tls": true},
	}))
	assert.False(t, proxyUsesTLS(&repository.Proxy{
		Protocol: "vmess",
		Settings: map[string]any{"security": "none"},
	}))
}

func TestCheckProxyEndpointDoesNotClassifyTCPRefusalAsTLSFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	checker := NewHealthChecker(&HealthCheckConfig{Timeout: time.Second}, nil, nil, nil, nil, nil)
	health := checker.checkProxyEndpoint(
		&repository.Node{},
		&repository.Proxy{Port: port, Settings: map[string]any{"security": "tls"}},
		"127.0.0.1",
	)

	assert.False(t, health.Reachable)
	assert.False(t, health.TLSFailure)
	assert.Contains(t, health.Reason, "TCP connection failed")
}

func TestSampledProxyEndpointTLSFailureIsCriticalWhenAnotherEndpointIsReachable(t *testing.T) {
	health := sampledProxyEndpointHealth{
		HasSampled:             true,
		AnyReachable:           true,
		FirstUnreachableTarget: "node.example.com:20001",
		FirstFailureReason:     "served TLS certificate expired",
		TLSFailure:             true,
	}
	assert.True(t, sampledProxyEndpointHasTLSFailure(health))
	assert.Contains(t, sampledProxyEndpointTLSFailureMessage(health), "node.example.com:20001")
	assert.Contains(t, sampledProxyEndpointTLSFailureMessage(health), "certificate expired")
	assert.False(t, sampledProxyEndpointHasTLSFailure(sampledProxyEndpointHealth{
		HasSampled:   true,
		AnyReachable: true,
	}))
}

func TestVerifyServedTLSCertificateRejectsExpiredLeaf(t *testing.T) {
	now := time.Now()
	err := verifyServedTLSCertificate(tls.ConnectionState{PeerCertificates: []*x509.Certificate{{
		NotBefore: now.Add(-48 * time.Hour),
		NotAfter:  now.Add(-24 * time.Hour),
	}}}, "www.example.com", now, nil)

	assert.ErrorContains(t, err, "expired")
}

func TestVerifyServedTLSCertificateRejectsWrongName(t *testing.T) {
	now := time.Now()
	err := verifyServedTLSCertificate(tls.ConnectionState{PeerCertificates: []*x509.Certificate{{
		NotBefore: now.Add(-time.Hour),
		NotAfter:  now.Add(time.Hour),
		DNSNames:  []string{"other.example.com"},
	}}}, "www.example.com", now, nil)

	assert.ErrorContains(t, err, "does not match")
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

func TestEvaluateCertificateHealthMarksExpiredCertificateFailed(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(-72 * time.Hour)

	health := evaluateCertificateHealth(&repository.Certificate{
		Domain:     "*.example.com",
		Status:     "active",
		ExpireDate: &expiresAt,
		ExpiresAt:  expiresAt,
	}, now)

	assert.True(t, health.Failed)
	assert.Contains(t, health.Message, "关联证书已过期 3 天")
}

func TestEvaluateCertificateHealthKeepsExpiringCertificateAsWarning(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(5 * 24 * time.Hour)

	health := evaluateCertificateHealth(&repository.Certificate{
		Domain:     "*.example.com",
		Status:     "active",
		ExpireDate: &expiresAt,
		ExpiresAt:  expiresAt,
	}, now)

	assert.False(t, health.Failed)
	assert.Contains(t, health.Message, "证书将在 5 天后过期")
}

func TestNodeUsesCertificateRequiresTLS(t *testing.T) {
	certificateID := int64(5)

	assert.False(t, nodeUsesCertificate(&repository.Node{
		TLSEnabled:    false,
		CertificateID: &certificateID,
	}))
	assert.False(t, nodeUsesCertificate(&repository.Node{TLSEnabled: true}))
	assert.True(t, nodeUsesCertificate(&repository.Node{
		TLSEnabled:    true,
		CertificateID: &certificateID,
	}))
}

func TestApplyCertificateHealthReplacesSuccessfulMessageOnFailure(t *testing.T) {
	result := &HealthCheckResult{
		Status:  repository.HealthCheckStatusSuccess,
		Message: "All checks passed",
	}

	applyCertificateHealth(result, certificateHealth{
		Message: "关联证书已过期 3 天",
		Failed:  true,
	})

	assert.Equal(t, repository.HealthCheckStatusFailed, result.Status)
	assert.Equal(t, "关联证书已过期 3 天", result.Message)
	assert.NotContains(t, result.Message, "All checks passed")
}
