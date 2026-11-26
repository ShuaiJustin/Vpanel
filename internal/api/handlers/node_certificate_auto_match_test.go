package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func newNodeCertificateAutoMatchHandler(t *testing.T) (*NodeHandler, repository.NodeRepository, repository.CertificateRepository) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Node{}, &repository.Certificate{}))

	nodeRepo := repository.NewNodeRepository(db)
	certRepo := repository.NewCertificateRepository(db)
	require.NoError(t, certRepo.Create(t.Context(), &repository.Certificate{
		ID:        7,
		Domain:    "*.shcrystal.top",
		Status:    "active",
		CertPath:  "/etc/xray/certs/shcrystal/fullchain.pem",
		KeyPath:   "/etc/xray/certs/shcrystal/privkey.pem",
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}))

	handler := NewNodeHandler(
		node.NewService(nodeRepo, nil, nil, logger.NewNopLogger()),
		nil,
		nil,
		nil,
		logger.NewNopLogger(),
	).WithCertificateAutomation(certRepo, nil)

	return handler, nodeRepo, certRepo
}

func TestNodeHandlerCreateAutoMatchesWildcardCertificate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, nodeRepo, _ := newNodeCertificateAutoMatchHandler(t)

	router := gin.New()
	router.POST("/nodes", handler.Create)

	body := []byte(`{
		"name": "jp36",
		"address": "64.176.54.36",
		"port": 18443,
		"protocols": ["trojan"],
		"tls_enabled": true,
		"tls_domain": "www.shcrystal.top"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/nodes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var response struct {
		ID            int64  `json:"id"`
		CertificateID *int64 `json:"certificate_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.CertificateID)
	require.Equal(t, int64(7), *response.CertificateID)

	saved, err := nodeRepo.GetByID(t.Context(), response.ID)
	require.NoError(t, err)
	require.NotNil(t, saved.CertificateID)
	require.Equal(t, int64(7), *saved.CertificateID)
}

func TestNodeHandlerUpdateAutoMatchesWildcardCertificate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, nodeRepo, _ := newNodeCertificateAutoMatchHandler(t)

	require.NoError(t, nodeRepo.Create(t.Context(), &repository.Node{
		ID:      3,
		Name:    "jp36",
		Address: "64.176.54.36",
		Port:    18443,
		Token:   "node-token",
		Status:  repository.NodeStatusOffline,
	}))

	router := gin.New()
	router.PUT("/nodes/:id", handler.Update)

	req := httptest.NewRequest(http.MethodPut, "/nodes/3", bytes.NewReader([]byte(`{"tls_enabled":true}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response struct {
		CertificateID *int64 `json:"certificate_id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.NotNil(t, response.CertificateID)
	require.Equal(t, int64(7), *response.CertificateID)

	saved, err := nodeRepo.GetByID(t.Context(), 3)
	require.NoError(t, err)
	require.Equal(t, node.DefaultTLSDomain, saved.TLSDomain)
	require.NotNil(t, saved.CertificateID)
	require.Equal(t, int64(7), *saved.CertificateID)
}

func TestNodeHandlerAccessDomainAndIPv6Address(t *testing.T) {
	handler, nodeRepo, _ := newNodeCertificateAutoMatchHandler(t)
	router := gin.New()
	router.POST("/nodes", handler.Create)
	router.PUT("/nodes/:id", handler.Update)
	request := func(method, path, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	require.NoError(t, nodeRepo.Create(t.Context(), &repository.Node{
		ID: 1, Name: "legacy", Address: "2001:0db8:0:0:0:0:0:1", Port: 18443, Token: "legacy-token",
	}))
	duplicate := request(http.MethodPost, "/nodes", `{"name":"duplicate","address":"[2001:db8::1]","port":18443}`)
	require.Equal(t, http.StatusConflict, duplicate.Code, duplicate.Body.String())
	created := request(http.MethodPost, "/nodes", `{"name":"edge","address":"2001:0db8::1","port":19443,"access_domain":" EDGE.Example.com. ","tls_enabled":true}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var added NodeResponse
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &added))
	require.Equal(t, "2001:db8::1", added.Address)
	require.Equal(t, "edge.example.com", added.AccessDomain)
	require.Equal(t, "www.shcrystal.top", added.TLSDomain)
	require.NotNil(t, added.CertificateID)
	require.Equal(t, int64(7), *added.CertificateID)

	path := fmt.Sprintf("/nodes/%d", added.ID)
	duplicate = request(http.MethodPut, path, `{"port":18443}`)
	require.Equal(t, http.StatusConflict, duplicate.Code, duplicate.Body.String())
	invalid := request(http.MethodPut, path, `{"access_domain":"https://edge.example.com:443"}`)
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
	preserved := request(http.MethodPut, path, `{"region":"jp"}`)
	require.Equal(t, http.StatusOK, preserved.Code, preserved.Body.String())
	require.Contains(t, preserved.Body.String(), `"access_domain":"edge.example.com"`)
	cleared := request(http.MethodPut, path, `{"access_domain":""}`)
	require.Equal(t, http.StatusOK, cleared.Code, cleared.Body.String())
	saved, err := nodeRepo.GetByID(t.Context(), added.ID)
	require.NoError(t, err)
	require.Empty(t, saved.AccessDomain)
	require.Equal(t, 19443, saved.Port)
}
