package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/certificate"
	"v/internal/database/repository"
	"v/internal/logger"
)

type blockingCertificateService struct {
	started chan struct{}
	release chan struct{}
	calls   chan struct{}
}

func (s *blockingCertificateService) Apply(context.Context, *certificate.ApplyRequest) (*repository.Certificate, error) {
	return nil, nil
}

func (s *blockingCertificateService) Upload(context.Context, string, []byte, []byte) (*repository.Certificate, error) {
	return nil, nil
}

func (s *blockingCertificateService) UpdateMaterial(context.Context, int64, []byte, []byte) (*repository.Certificate, error) {
	return nil, nil
}

func (s *blockingCertificateService) Renew(context.Context, int64) error {
	s.calls <- struct{}{}
	close(s.started)
	<-s.release
	return nil
}

func (s *blockingCertificateService) Delete(context.Context, int64) error { return nil }

func (s *blockingCertificateService) DeployToAssignedNodes(context.Context, int64) error { return nil }

func TestCertificateRenewRunsInBackgroundAndRejectsDuplicate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Certificate{}))

	repo := repository.NewCertificateRepository(db)
	expiresAt := time.Now().Add(20 * 24 * time.Hour)
	cert := &repository.Certificate{
		Domain:     "example.com",
		Provider:   "letsencrypt",
		CertPath:   "/tmp/example.com/fullchain.pem",
		KeyPath:    "/tmp/example.com/key.pem",
		Status:     "active",
		ExpiresAt:  expiresAt,
		ExpireDate: &expiresAt,
	}
	require.NoError(t, repo.Create(context.Background(), cert))

	service := &blockingCertificateService{
		started: make(chan struct{}),
		release: make(chan struct{}),
		calls:   make(chan struct{}, 2),
	}
	handler := NewCertificateHandler(repo, nil, service, logger.NewNopLogger())
	router := gin.New()
	router.POST("/certificates/:id/renew", handler.Renew)
	router.DELETE("/certificates/:id", handler.Delete)
	router.GET("/certificates", handler.List)

	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodPost, "/certificates/1/renew", nil))
	require.Equal(t, http.StatusAccepted, first.Code)

	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("renewal worker did not start")
	}

	list := httptest.NewRecorder()
	router.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/certificates", nil))
	require.Equal(t, http.StatusOK, list.Code)
	var payload struct {
		Certificates []CertificateResponse `json:"certificates"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &payload))
	require.Len(t, payload.Certificates, 1)
	require.Equal(t, "renewing", payload.Certificates[0].Status)

	duplicate := httptest.NewRecorder()
	router.ServeHTTP(duplicate, httptest.NewRequest(http.MethodPost, "/certificates/1/renew", nil))
	require.Equal(t, http.StatusConflict, duplicate.Code)
	deletion := httptest.NewRecorder()
	router.ServeHTTP(deletion, httptest.NewRequest(http.MethodDelete, "/certificates/1", nil))
	require.Equal(t, http.StatusConflict, deletion.Code)

	close(service.release)
	require.Eventually(t, func() bool { return !handler.isRenewing(cert.ID) }, time.Second, 10*time.Millisecond)
	require.Len(t, service.calls, 1, "duplicate request must not start a second renewal")
}

func TestCertificateRenewRejectsManualCertificate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Certificate{}))
	repo := repository.NewCertificateRepository(db)
	expiresAt := time.Now().Add(20 * 24 * time.Hour)
	require.NoError(t, repo.Create(context.Background(), &repository.Certificate{
		Domain:     "manual.example.com",
		Provider:   "manual",
		Status:     "active",
		ExpiresAt:  expiresAt,
		ExpireDate: &expiresAt,
	}))

	service := &blockingCertificateService{started: make(chan struct{}), release: make(chan struct{}), calls: make(chan struct{}, 1)}
	handler := NewCertificateHandler(repo, nil, service, logger.NewNopLogger())
	router := gin.New()
	router.POST("/certificates/:id/renew", handler.Renew)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/certificates/1/renew", nil))
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "不支持自动续期")
	require.Empty(t, service.calls)
}
