package certificate

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
)

type recordingAlertNotifier struct {
	alerts []notification.CertificateAlertData
}

func (r *recordingAlertNotifier) NotifyCertificateAlert(data notification.CertificateAlertData) error {
	r.alerts = append(r.alerts, data)
	return nil
}

func newTestCertificateService(t *testing.T) (*Service, repository.CertificateRepository, repository.NodeRepository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Certificate{}, &repository.Node{}, &repository.CertificateDeployment{}))

	certRepo := repository.NewCertificateRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	deploymentRepo := repository.NewCertificateDeploymentRepository(db)
	svc := NewService(certRepo, nodeRepo, deploymentRepo, logger.NewNopLogger(), t.TempDir())
	return svc, certRepo, nodeRepo
}

func generateTestPEMPair(t *testing.T, domain string, key *rsa.PrivateKey) ([]byte, []byte) {
	t.Helper()

	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{domain},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}

func TestServiceUploadRejectsMismatchedPrivateKey(t *testing.T) {
	svc, _, _ := newTestCertificateService(t)

	certKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	certPEM, _ := generateTestPEMPair(t, "example.com", certKey)
	_, otherKeyPEM := generateTestPEMPair(t, "example.com", otherKey)

	_, err = svc.Upload(context.Background(), "example.com", certPEM, otherKeyPEM)
	require.Error(t, err)
	require.Contains(t, err.Error(), "证书与私钥不匹配")
}

func TestServiceUploadWildcardAndDeleteUnassignsNodes(t *testing.T) {
	svc, certRepo, nodeRepo := newTestCertificateService(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	certPEM, keyPEM := generateTestPEMPair(t, "*.example.com", key)

	cert, err := svc.Upload(context.Background(), "*.example.com", certPEM, keyPEM)
	require.NoError(t, err)
	require.Contains(t, cert.CertPath, "_wildcard_.example.com")
	require.NotContains(t, cert.CertPath, "*")
	require.FileExists(t, cert.CertPath)
	require.FileExists(t, cert.KeyPath)

	require.NoError(t, nodeRepo.Create(context.Background(), &repository.Node{
		Name:          "edge-1",
		Address:       "203.0.113.10",
		Port:          18443,
		CertificateID: &cert.ID,
	}))

	certDir := strings.TrimSuffix(cert.CertPath, "/fullchain.pem")
	require.NoError(t, svc.Delete(context.Background(), cert.ID))

	_, err = certRepo.GetByID(context.Background(), cert.ID)
	require.Error(t, err)

	node, err := nodeRepo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	require.Nil(t, node.CertificateID)

	_, err = os.Stat(certDir)
	require.True(t, os.IsNotExist(err))
}

func TestCertificateNeedsRenewalIncludesExpiredCertificates(t *testing.T) {
	now := time.Now()
	soon := now.Add(29 * 24 * time.Hour)
	later := now.Add(31 * 24 * time.Hour)
	expired := now.Add(-24 * time.Hour)

	require.True(t, certificateNeedsRenewal(&soon, now, 30*24*time.Hour))
	require.True(t, certificateNeedsRenewal(&expired, now, 30*24*time.Hour))
	require.False(t, certificateNeedsRenewal(&later, now, 30*24*time.Hour))
	require.False(t, certificateNeedsRenewal(nil, now, 30*24*time.Hour))
}

func TestWithAutoRenewConfig(t *testing.T) {
	svc, _, _ := newTestCertificateService(t)
	svc.WithAutoRenewConfig(2*time.Hour, 14)

	require.Equal(t, 2*time.Hour, svc.checkInterval)
	require.Equal(t, 14*24*time.Hour, svc.renewThreshold)
}

func TestRenewalLockPreventsConcurrentWorkForSameCertificate(t *testing.T) {
	svc, _, _ := newTestCertificateService(t)

	require.True(t, svc.beginRenewal(9))
	require.False(t, svc.beginRenewal(9))
	require.True(t, svc.beginRenewal(10), "different certificates may renew concurrently")

	svc.finishRenewal(9)
	require.True(t, svc.beginRenewal(9))
}

func TestNotifyCertificateAlertUsesFriendlyCertificateDetails(t *testing.T) {
	svc, _, _ := newTestCertificateService(t)
	notifier := &recordingAlertNotifier{}
	svc.SetNotificationService(notifier)
	expiresAt := time.Now().Add(-24 * time.Hour)

	svc.notifyCertificateAlert(&repository.Certificate{
		ID:           5,
		Domain:       "*.example.com",
		ExpireDate:   &expiresAt,
		ExpiresAt:    expiresAt,
		AutoRenew:    true,
		ErrorMessage: "ACME 签发状态缺失，无法直接续期",
	}, "renewal_failed", "ACME 签发状态缺失，无法直接续期")

	require.Len(t, notifier.alerts, 1)
	require.Equal(t, int64(5), notifier.alerts[0].CertificateID)
	require.Equal(t, "*.example.com", notifier.alerts[0].Domain)
	require.Equal(t, "renewal_failed", notifier.alerts[0].Level)
	require.Contains(t, notifier.alerts[0].Reason, "ACME 签发状态缺失")
}

func TestDeployToNodeMarksConfigPendingBeforeSSH(t *testing.T) {
	svc, _, nodeRepo := newTestCertificateService(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	certPEM, keyPEM := generateTestPEMPair(t, "example.com", key)
	cert, err := svc.Upload(context.Background(), "example.com", certPEM, keyPEM)
	require.NoError(t, err)

	require.NoError(t, nodeRepo.Create(context.Background(), &repository.Node{
		Name:          "edge-1",
		Address:       "203.0.113.10",
		Port:          18443,
		SyncStatus:    repository.NodeSyncStatusSynced,
		CertificateID: &cert.ID,
	}))

	err = svc.DeployToNode(context.Background(), cert.ID, 1)
	require.ErrorContains(t, err, "未配置 SSH 认证方式")

	nodeModel, getErr := nodeRepo.GetByID(context.Background(), 1)
	require.NoError(t, getErr)
	require.Equal(t, repository.NodeSyncStatusPending, nodeModel.SyncStatus)
}
