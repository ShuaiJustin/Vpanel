package certificate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
)

type channelCertificateNotifier chan notification.CertificateAlertData

func (c channelCertificateNotifier) NotifyCertificateAlert(a notification.CertificateAlertData) error {
	c <- a
	return nil
}

func TestCertificateMonitoringAlertsWithGlobalRenewalDisabled(t *testing.T) {
	svc, repo, _ := newTestCertificateService(t)
	expires := time.Now().Add(time.Hour)
	require.NoError(t, repo.Create(context.Background(), &repository.Certificate{Domain: "example.com", Provider: "manual", AutoRenew: false, ExpiresAt: expires, ExpireDate: &expires}))
	alerts := make(channelCertificateNotifier, 1)
	svc.SetNotificationService(alerts)
	require.NoError(t, svc.StartMonitoring(context.Background(), false))
	defer svc.StopAutoRenew()
	select {
	case alert := <-alerts:
		require.Equal(t, "expiring", alert.Level)
	case <-time.After(2 * time.Second):
		t.Fatal("lifecycle warnings must run when global automatic renewal is off")
	}
}

func TestCertificateAlertsIncludeManualAndDisabledRenewal(t *testing.T) {
	svc, repo, _ := newTestCertificateService(t)
	now := time.Now()
	for i, days := range []int{-1, 1, 7, 14, 30, 31} {
		expires := now.Add(time.Duration(days) * 24 * time.Hour)
		require.NoError(t, repo.Create(context.Background(), &repository.Certificate{Domain: strconv.Itoa(i) + ".example.com", Provider: "manual", AutoRenew: false, ExpiresAt: expires, ExpireDate: &expires}))
	}
	notifier := &recordingAlertNotifier{}
	svc.SetNotificationService(notifier)
	svc.scanCertificateAlerts(context.Background(), now)
	require.Len(t, notifier.alerts, 5)
	keys := map[string]bool{}
	for _, a := range notifier.alerts {
		require.NotEmpty(t, a.DedupKey)
		keys[a.DedupKey] = true
	}
	require.Len(t, keys, 5)
	svc.scanCertificateAlerts(context.Background(), now.Add(time.Hour))
	for _, a := range notifier.alerts[5:] {
		require.True(t, keys[a.DedupKey], "repeat sweep must use the same persistent deduplication key")
	}
}

func TestDeploymentRequiresServedCertificateAndCanConfirmAfterFailure(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Certificate{}, &repository.Node{}, &repository.CertificateDeployment{}, &repository.Proxy{}))
	cr := repository.NewCertificateRepository(db)
	nr := repository.NewNodeRepository(db)
	dr := repository.NewCertificateDeploymentRepository(db)
	pr := repository.NewProxyRepository(db)
	svc := NewService(cr, nr, dr, logger.NewNopLogger(), t.TempDir())
	svc.SetProxyRepository(pr)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	oldPEM, oldKey := generateTestPEMPair(t, "example.com", key)
	oldPair, err := tls.X509KeyPair(oldPEM, oldKey)
	require.NoError(t, err)
	newPEM, newKey := generateTestPEMPair(t, "example.com", key)
	newPair, err := tls.X509KeyPair(newPEM, newKey)
	require.NoError(t, err)
	cert, err := svc.Upload(context.Background(), "example.com", newPEM, newKey)
	require.NoError(t, err)
	var active atomic.Pointer[tls.Certificate]
	active.Store(&oldPair)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.TLS = &tls.Config{GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return active.Load(), nil }}
	server.StartTLS()
	defer server.Close()
	host, p, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(p)
	require.NoError(t, err)
	now := time.Now()
	n := &repository.Node{Name: "test", Address: host, TLSDomain: "example.com", TLSEnabled: true, XrayRunning: true, CertificateID: &cert.ID, SyncStatus: "synced", SyncedAt: &now}
	require.NoError(t, nr.Create(context.Background(), n))
	require.NoError(t, pr.Create(context.Background(), &repository.Proxy{NodeID: &n.ID, Protocol: "vmess", Port: port, Enabled: true, Settings: map[string]any{"security": "tls", "sni": "example.com"}}))
	d := &repository.CertificateDeployment{CertificateID: cert.ID, NodeID: n.ID, Status: "pending", CreatedAt: now.Add(-6 * time.Minute)}
	require.NoError(t, dr.Create(context.Background(), d))
	svc.reconcileCertificateDeployments(context.Background())
	d, err = dr.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", d.Status)
	require.Nil(t, d.DeployedAt)
	active.Store(&newPair)
	svc.reconcileCertificateDeployments(context.Background())
	d, err = dr.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	require.Equal(t, "success", d.Status)
	require.NotNil(t, d.DeployedAt)
	// A node with no TLS consumers must not claim a TLS handshake was verified.
	proxies, err := pr.GetByNodeID(context.Background(), n.ID)
	require.NoError(t, err)
	proxies[0].Enabled = false
	require.NoError(t, pr.Update(context.Background(), proxies[0]))
	d.Status = "pending"
	require.NoError(t, dr.Update(context.Background(), d))
	svc.reconcileCertificateDeployments(context.Background())
	d, err = dr.GetByID(context.Background(), d.ID)
	require.NoError(t, err)
	require.Equal(t, "success", d.Status)
	require.Contains(t, d.Message, "无需 TLS 验证")
	require.NotContains(t, d.Message, "通过 TLS 验证")
}

func TestSSHRestartFailureNeverReportsDeploymentSuccess(t *testing.T) {
	svc, _, nr := newTestCertificateService(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	certPEM, keyPEM := generateTestPEMPair(t, "example.com", key)
	cert, err := svc.Upload(context.Background(), "example.com", certPEM, keyPEM)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		server, chans, reqs, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(reqs)
		for ch := range chans {
			channel, requests, err := ch.Accept()
			if err != nil {
				return
			}
			go func() {
				defer channel.Close()
				for req := range requests {
					if req.Type != "exec" {
						req.Reply(false, nil)
						continue
					}
					var command struct{ Command string }
					_ = ssh.Unmarshal(req.Payload, &command)
					req.Reply(true, nil)
					status := uint32(0)
					if strings.Contains(command.Command, "systemctl restart") {
						status = 1
					}
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
					return
				}
			}()
		}
	}()
	host, p, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(p)
	n := &repository.Node{Name: "test", Address: host, SSHHost: host, SSHPort: port, SSHUser: "root", SSHPassword: "test-password", CertificateID: &cert.ID}
	require.NoError(t, nr.Create(context.Background(), n))
	err = svc.DeployToNode(context.Background(), cert.ID, n.ID)
	require.ErrorContains(t, err, "重启 Xray 服务失败")
	d, err := svc.deploymentRepo.GetLatestByNodeAndCert(context.Background(), n.ID, cert.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", d.Status)
	require.Nil(t, d.DeployedAt)
	<-done
}
