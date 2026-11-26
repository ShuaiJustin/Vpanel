package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func TestPersistNodeSSHPrivateKeyWritesKeyFile(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("VPANEL_DATA_DIR", dataDir)

	keyPath, err := persistNodeSSHPrivateKey(42, "  PRIVATE KEY DATA\n\n")
	if err != nil {
		t.Fatalf("expected private key to be persisted, got %v", err)
	}

	require.Equal(t, filepath.Join(dataDir, "node-ssh-keys"), filepath.Dir(keyPath))
	require.True(t, strings.HasPrefix(filepath.Base(keyPath), "node-42-"))
	require.Equal(t, ".key", filepath.Ext(keyPath))

	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read persisted private key: %v", err)
	}
	if string(data) != "PRIVATE KEY DATA\n" {
		t.Fatalf("unexpected key content: %q", string(data))
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("failed to stat persisted private key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected key permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestPersistNodeSSHPrivateKeyIgnoresEmptyKey(t *testing.T) {
	if keyPath, err := persistNodeSSHPrivateKey(42, " \n\t "); err != nil || keyPath != "" {
		t.Fatalf("expected empty key to be ignored, got path=%q err=%v", keyPath, err)
	}
}

func TestSaveNodeSSHMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler, nodeRepo, _ := newNodeCertificateAutoMatchHandler(t)
	require.NoError(t, nodeRepo.Create(t.Context(), &repository.Node{
		ID: 28, Name: "test-node", Address: "2001:db8::1", SSHHost: "2001:db8::1",
	}))

	router := gin.New()
	router.POST("/nodes/:id/ssh-metadata", handler.SaveSSHMetadata)
	router.POST("/nodes", handler.Create)
	router.PUT("/nodes/:id", handler.Update)
	deployHandler := NewNodeDeployHandler(nil, handler.nodeService, nil, logger.NewNopLogger())
	router.POST("/nodes/:id/deploy", deployHandler.DeployAgent)
	send := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	post := func(body string) *httptest.ResponseRecorder {
		return send(http.MethodPost, "/nodes/28/ssh-metadata", body)
	}
	postFields := func(fields map[string]any) *httptest.ResponseRecorder {
		body, err := json.Marshal(fields)
		require.NoError(t, err)
		return post(string(body))
	}
	makeKey := func() string {
		_, privateKey, err := ed25519.GenerateKey(nil)
		require.NoError(t, err)
		key, err := ssh.MarshalPrivateKey(privateKey, "")
		require.NoError(t, err)
		return string(pem.EncodeToMemory(key))
	}
	originalKey, replacementKey := makeKey(), makeKey()
	payload := map[string]any{"host": "2001:db8::2", "port": 22, "username": "root", "private_key": originalKey}

	dataDir := t.TempDir()
	dataFile := filepath.Join(dataDir, "not-a-directory")
	require.NoError(t, os.WriteFile(dataFile, nil, 0600))
	t.Setenv("VPANEL_DATA_DIR", dataFile)
	failed := postFields(payload)
	require.Equal(t, http.StatusInternalServerError, failed.Code, failed.Body.String())
	require.NotContains(t, failed.Body.String(), "PRIVATE KEY")
	saved, err := nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, "2001:db8::1", saved.SSHHost)

	t.Setenv("VPANEL_DATA_DIR", dataDir)
	invalid := post(`{"port":65536,"private_key":"test-key"}`)
	require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
	require.NoDirExists(t, filepath.Join(dataDir, "node-ssh-keys"))

	success := postFields(payload)
	require.Equal(t, http.StatusOK, success.Code, success.Body.String())
	require.NotContains(t, success.Body.String(), "PRIVATE KEY")
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, "2001:db8::2", saved.SSHHost)
	require.Equal(t, 22, saved.SSHPort)
	keyData, err := os.ReadFile(saved.SSHKeyPath)
	require.NoError(t, err)
	require.Equal(t, originalKey, string(keyData))

	keyPath := saved.SSHKeyPath
	invalidKey := post(`{"private_key":"invalid-replacement"}`)
	require.Equal(t, http.StatusBadRequest, invalidKey.Code, invalidKey.Body.String())
	require.NotContains(t, invalidKey.Body.String(), "invalid-replacement")
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, keyPath, saved.SSHKeyPath)
	invalidRequests := []struct{ method, path, body string }{
		{http.MethodPost, "/nodes", `{"name":"new","address":"2001:db8::5","ssh":{"host":"2001:db8::5","username":"root","private_key":"invalid-replacement"}}`},
		{http.MethodPut, "/nodes/28", `{"name":"changed","ssh":{"host":"2001:db8::2","username":"root","private_key":"invalid-replacement"}}`},
		{http.MethodPost, "/nodes/28/deploy", `{"host":"2001:db8::2","username":"root","private_key":"invalid-replacement"}`},
	}
	for _, request := range invalidRequests {
		response := send(request.method, request.path, request.body)
		require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
		require.Contains(t, response.Body.String(), invalidNodeSSHPrivateKeyMessage)
	}
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, "test-node", saved.Name)
	require.Empty(t, saved.Token)
	require.Equal(t, keyPath, saved.SSHKeyPath)

	workingService := handler.nodeService
	handler.nodeService = node.NewService(failingSSHConfigRepo{nodeRepo}, nil, nil, logger.NewNopLogger())
	failedUpdate := postFields(map[string]any{"host": "2001:db8::3", "private_key": replacementKey})
	require.Equal(t, http.StatusInternalServerError, failedUpdate.Code, failedUpdate.Body.String())
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, "2001:db8::2", saved.SSHHost)
	require.Equal(t, keyPath, saved.SSHKeyPath)
	keyData, err = os.ReadFile(keyPath)
	require.NoError(t, err)
	require.Equal(t, originalKey, string(keyData))
	files, err := os.ReadDir(filepath.Dir(keyPath))
	require.NoError(t, err)
	require.Len(t, files, 1)

	handler.nodeService = workingService
	replacement := postFields(map[string]any{"PrivateKey": replacementKey})
	require.Equal(t, http.StatusOK, replacement.Code, replacement.Body.String())
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.NotEqual(t, keyPath, saved.SSHKeyPath)
	require.NoFileExists(t, keyPath)

	partial := post(`{"username":"admin"}`)
	require.Equal(t, http.StatusOK, partial.Code, partial.Body.String())
	saved, err = nodeRepo.GetByID(t.Context(), 28)
	require.NoError(t, err)
	require.Equal(t, 22, saved.SSHPort)
	require.Equal(t, "admin", saved.SSHUser)
	require.NotEmpty(t, saved.SSHKeyPath)
}

type failingSSHConfigRepo struct {
	repository.NodeRepository
}

func (r failingSSHConfigRepo) UpdateSSHConfig(context.Context, int64, map[string]any, *string) error {
	return errors.New("SSH config database write failed")
}
