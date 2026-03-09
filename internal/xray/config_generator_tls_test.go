package xray

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
	apperrors "v/pkg/errors"
)

type mockCertificateRepoForGenerator struct {
	certs map[string]*repository.Certificate
}

func (m *mockCertificateRepoForGenerator) Create(ctx context.Context, cert *repository.Certificate) error {
	if m.certs == nil {
		m.certs = make(map[string]*repository.Certificate)
	}
	m.certs[cert.Domain] = cert
	return nil
}

func (m *mockCertificateRepoForGenerator) GetByID(ctx context.Context, id int64) (*repository.Certificate, error) {
	for _, cert := range m.certs {
		if cert.ID == id {
			return cert, nil
		}
	}
	return nil, apperrors.NewNotFoundError("certificate", id)
}

func (m *mockCertificateRepoForGenerator) GetByDomain(ctx context.Context, domain string) (*repository.Certificate, error) {
	if cert, ok := m.certs[domain]; ok {
		return cert, nil
	}
	return nil, apperrors.NewNotFoundError("certificate", domain)
}

func (m *mockCertificateRepoForGenerator) Update(ctx context.Context, cert *repository.Certificate) error {
	if m.certs == nil {
		m.certs = make(map[string]*repository.Certificate)
	}
	m.certs[cert.Domain] = cert
	return nil
}

func (m *mockCertificateRepoForGenerator) Delete(ctx context.Context, id int64) error {
	for domain, cert := range m.certs {
		if cert.ID == id {
			delete(m.certs, domain)
			return nil
		}
	}
	return apperrors.NewNotFoundError("certificate", id)
}

func (m *mockCertificateRepoForGenerator) List(ctx context.Context, limit, offset int) ([]*repository.Certificate, error) {
	result := make([]*repository.Certificate, 0, len(m.certs))
	for _, cert := range m.certs {
		result = append(result, cert)
	}
	return result, nil
}

func (m *mockCertificateRepoForGenerator) Count(ctx context.Context) (int64, error) {
	return int64(len(m.certs)), nil
}

func (m *mockCertificateRepoForGenerator) GetExpiring(ctx context.Context, days int) ([]*repository.Certificate, error) {
	return nil, nil
}

func (m *mockCertificateRepoForGenerator) GetAutoRenew(ctx context.Context) ([]*repository.Certificate, error) {
	return nil, nil
}

func TestGenerateStreamSettings_AutoMatchesWildcardCertificate(t *testing.T) {
	generator := &ConfigGenerator{
		certRepo: &mockCertificateRepoForGenerator{certs: map[string]*repository.Certificate{
			"*.example.com": {
				ID:       1,
				Domain:   "*.example.com",
				Status:   "active",
				CertPath: "/etc/ssl/example/fullchain.pem",
				KeyPath:  "/etc/ssl/example/privkey.pem",
			},
		}},
		logger: logger.NewNopLogger(),
	}

	stream := generator.generateStreamSettings(context.Background(), map[string]any{
		"network":     "ws",
		"security":    "tls",
		"server_name": "api.example.com",
	})

	require.NotNil(t, stream)
	require.NotNil(t, stream.TLSSettings)
	require.Len(t, stream.TLSSettings.Certificates, 1)
	assert.Equal(t, "/etc/ssl/example/fullchain.pem", stream.TLSSettings.Certificates[0].CertificateFile)
	assert.Equal(t, "/etc/ssl/example/privkey.pem", stream.TLSSettings.Certificates[0].KeyFile)
}

func TestGenerateStreamSettings_ManualFilesTakePrecedence(t *testing.T) {
	generator := &ConfigGenerator{
		certRepo: &mockCertificateRepoForGenerator{certs: map[string]*repository.Certificate{
			"example.com": {
				ID:       2,
				Domain:   "example.com",
				Status:   "active",
				CertPath: "/auto/fullchain.pem",
				KeyPath:  "/auto/privkey.pem",
			},
		}},
		logger: logger.NewNopLogger(),
	}

	stream := generator.generateStreamSettings(context.Background(), map[string]any{
		"security":    "tls",
		"server_name": "example.com",
		"cert_file":   "/manual/fullchain.pem",
		"key_file":    "/manual/privkey.pem",
	})

	require.NotNil(t, stream)
	require.NotNil(t, stream.TLSSettings)
	require.Len(t, stream.TLSSettings.Certificates, 1)
	assert.Equal(t, "/manual/fullchain.pem", stream.TLSSettings.Certificates[0].CertificateFile)
	assert.Equal(t, "/manual/privkey.pem", stream.TLSSettings.Certificates[0].KeyFile)
}

func TestGenerateStreamSettings_SupportsInlineCertificateContent(t *testing.T) {
	generator := &ConfigGenerator{logger: logger.NewNopLogger()}

	stream := generator.generateStreamSettings(context.Background(), map[string]any{
		"security":    "tls",
		"server_name": "example.com",
		"certificate": "-----BEGIN CERTIFICATE-----test-----END CERTIFICATE-----",
		"key":         "-----BEGIN PRIVATE KEY-----test-----END PRIVATE KEY-----",
	})

	require.NotNil(t, stream)
	require.NotNil(t, stream.TLSSettings)
	require.Len(t, stream.TLSSettings.Certificates, 1)
	assert.Equal(t, []string{"-----BEGIN CERTIFICATE-----test-----END CERTIFICATE-----"}, stream.TLSSettings.Certificates[0].Certificate)
	assert.Equal(t, []string{"-----BEGIN PRIVATE KEY-----test-----END PRIVATE KEY-----"}, stream.TLSSettings.Certificates[0].Key)
}

func TestGenerateStreamSettings_AutoMatchesStoredCertificateContent(t *testing.T) {
	generator := &ConfigGenerator{
		certRepo: &mockCertificateRepoForGenerator{certs: map[string]*repository.Certificate{
			"example.com": {
				ID:          3,
				Domain:      "example.com",
				Status:      "active",
				Certificate: "-----BEGIN CERTIFICATE-----stored-----END CERTIFICATE-----",
				PrivateKey:  "-----BEGIN PRIVATE KEY-----stored-----END PRIVATE KEY-----",
			},
		}},
		logger: logger.NewNopLogger(),
	}

	stream := generator.generateStreamSettings(context.Background(), map[string]any{
		"security":    "tls",
		"server_name": "example.com",
	})

	require.NotNil(t, stream)
	require.NotNil(t, stream.TLSSettings)
	require.Len(t, stream.TLSSettings.Certificates, 1)
	assert.Equal(t, []string{"-----BEGIN CERTIFICATE-----stored-----END CERTIFICATE-----"}, stream.TLSSettings.Certificates[0].Certificate)
	assert.Equal(t, []string{"-----BEGIN PRIVATE KEY-----stored-----END PRIVATE KEY-----"}, stream.TLSSettings.Certificates[0].Key)
}

func TestBuildCertificateCandidates(t *testing.T) {
	assert.Equal(t, []string{"api.example.com", "*.example.com"}, buildCertificateCandidates("api.example.com"))
	assert.Equal(t, []string{"example.com"}, buildCertificateCandidates("example.com"))
}

func TestNormalizeTLSDomain(t *testing.T) {
	assert.Equal(t, "example.com", normalizeTLSDomain(" *.Example.com "))
	assert.Equal(t, "api.example.com", normalizeTLSDomain("API.EXAMPLE.COM"))
	assert.False(t, time.Now().IsZero())
}
