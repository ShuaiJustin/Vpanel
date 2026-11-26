package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func TestInstallCoreVersionRejectsInvalidPayload(t *testing.T) {
	h, _, _ := newNodeCertificateAutoMatchHandler(t)
	r := gin.New()
	r.POST("/nodes/:id/core/install-version", h.InstallCoreVersion)
	for _, body := range []string{`{"version":42}`, `{"version":`, `null`, `{"version":null}`, `{"version":"v"}`} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/nodes/1/core/install-version", strings.NewReader(body)))
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

func TestInstallStatusAfterPanelRestart(t *testing.T) {
	h, repo, _ := newNodeCertificateAutoMatchHandler(t)
	now := time.Now().UTC()
	for _, status := range []string{"running", "success"} {
		require.NoError(t, repo.Create(t.Context(), &repository.Node{
			Name: status, Address: "localhost", Token: status, InstallStatus: status,
			InstallMessage: "last message", InstallLogs: "last log", InstallUpdatedAt: &now,
			InstallSteps: `[{"name":"Connect","status":"running"}]`,
		}))
	}
	h.deployService = node.NewRemoteDeployService(logger.NewNopLogger(), repo)
	router := gin.New()
	router.GET("/nodes", h.List)
	router.GET("/nodes/:id/install-status", h.GetInstallStatus)
	get := func(path string, target any) {
		t.Helper()
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), target))
	}
	var result node.DeployTaskStatus
	get("/nodes/1/install-status", &result)
	require.Equal(t, "failed", result.Status)
	require.Contains(t, result.Message, "中断")
	require.Equal(t, "last log", result.Logs)
	require.Equal(t, []node.DeployStep{{Name: "Connect", Status: "failed"}}, result.Steps)
	require.True(t, result.UpdatedAt.Equal(now))
	get("/nodes/2/install-status", &result)
	require.Equal(t, "success", result.Status)
	require.Equal(t, "last message", result.Message)
	var list struct{ Nodes []NodeResponse }
	get("/nodes", &list)
	require.Equal(t, "failed", list.Nodes[0].InstallStatus)
	h.deployService.MarkInstallQueued(1, "retry")
	get("/nodes/1/install-status", &result)
	require.Equal(t, "queued", result.Status)
}

func TestDeployAgentRejectsConcurrentInstall(t *testing.T) {
	h, repo, _ := newNodeCertificateAutoMatchHandler(t)
	require.NoError(t, repo.Create(t.Context(), &repository.Node{
		ID: 1, Name: "node", Address: "localhost", Token: "install-test",
	}))
	service := node.NewRemoteDeployService(logger.NewNopLogger(), repo)
	release, ok := service.BeginInstall(1)
	require.True(t, ok)
	service.MarkInstallQueued(1, "existing task")

	router := gin.New()
	router.POST("/nodes/:id/deploy", NewNodeDeployHandler(service, h.nodeService, nil, logger.NewNopLogger()).DeployAgent)
	req := httptest.NewRequest(http.MethodPost, "/nodes/1/deploy", strings.NewReader(`{"host":"localhost","username":"root","password":"test-password"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	status, ok := service.GetInstallStatus(1)
	require.True(t, ok)
	require.Equal(t, "existing task", status.Message)

	release()
	releaseRetry, ok := service.BeginInstall(1)
	require.True(t, ok)
	releaseRetry()
}

func TestResolveDeployPanelURLPrefersCurrentPublicURLOverSavedNodeURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://internal:13212/api/admin/nodes/1/deploy", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("X-Forwarded-Host", "panel.shcrystal.top:13212")
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req

	got := resolveDeployPanelURL(c, "https://panel.shcrystal.top/", "", "https://panel.shcrystal.top:13212")

	assert.Equal(t, "https://panel.shcrystal.top", got)
}

func TestResolveDeployPanelURLAllowsExplicitOverride(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://internal:13212/api/admin/nodes/1/deploy", nil)

	got := resolveDeployPanelURL(c, "https://panel.shcrystal.top", "https://custom.example.com/", "https://old.example.com:13212")

	assert.Equal(t, "https://custom.example.com", got)
}

func TestNormalizeNodeGroupIDs(t *testing.T) {
	fallback := int64(5)

	tests := []struct {
		name     string
		groupIDs []int64
		fallback *int64
		want     []int64
	}{
		{
			name:     "deduplicates and ignores invalid values",
			groupIDs: []int64{3, 0, -1, 3, 4},
			fallback: &fallback,
			want:     []int64{3, 4},
		},
		{
			name:     "uses fallback when explicit groups are empty",
			groupIDs: nil,
			fallback: &fallback,
			want:     []int64{5},
		},
		{
			name:     "returns empty when no valid groups exist",
			groupIDs: []int64{0, -1},
			fallback: nil,
			want:     []int64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeNodeGroupIDs(tt.groupIDs, tt.fallback))
		})
	}
}

func TestNodeMutationErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantError  string
	}{
		{
			name:       "duplicate node returns conflict",
			err:        fmt.Errorf("%w: 节点地址 1.1.1.1:18443 已存在", node.ErrDuplicateNode),
			wantStatus: http.StatusConflict,
			wantError:  "节点地址 1.1.1.1:18443 已存在",
		},
		{
			name:       "invalid node returns bad request",
			err:        fmt.Errorf("%w: 节点名称不能为空", node.ErrInvalidNode),
			wantStatus: http.StatusBadRequest,
			wantError:  "节点名称不能为空",
		},
		{
			name:       "not found returns 404",
			err:        node.ErrNodeNotFound,
			wantStatus: http.StatusNotFound,
			wantError:  "Node not found",
		},
		{
			name:       "unknown error returns fallback",
			err:        fmt.Errorf("boom"),
			wantStatus: http.StatusInternalServerError,
			wantError:  "Failed to create node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, payload := nodeMutationErrorResponse(tt.err, "Failed to create node")
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, gin.H{"error": tt.wantError}, payload)
		})
	}
}
