package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	portalnode "v/internal/portal/node"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestPortalNodeHandler_TestLatencyFailureDoesNotQueueRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repo := newMockProxyRepository()
	nodeID := int64(3)
	if err := repo.Create(nil, &repository.Proxy{
		ID:       1,
		UserID:   99,
		NodeID:   &nodeID,
		Name:     "node-64-176-54-36-103738-vmess",
		Protocol: "vmess",
		Host:     "127.0.0.1",
		Port:     1,
		Enabled:  true,
		Remark:   "auto provisioned",
	}); err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	nodeService := portalnode.NewService(repo, nil, nil)
	recoveryTracker := NewNodeRecoveryTracker(logger.NewNopLogger())
	handler := NewPortalNodeHandler(nodeService, recoveryTracker, logger.NewNopLogger())

	router := gin.New()
	router.POST("/nodes/:id/ping", func(c *gin.Context) {
		setUserContext(c, 99, "user")
		handler.TestLatency(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/nodes/1/ping", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected status %d, got %d", http.StatusBadGateway, w.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["message"] != "连接失败" {
		t.Fatalf("expected failure message without recovery hint, got %#v", response["message"])
	}

	if pending := recoveryTracker.GetPendingCommands(nodeID); len(pending) != 0 {
		t.Fatalf("expected no recovery commands queued, got %d", len(pending))
	}
}
