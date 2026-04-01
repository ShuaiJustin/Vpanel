package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func TestShouldPromoteNodeOnlineFromHeartbeat(t *testing.T) {
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOffline, nil))
	assert.True(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusOnline, nil))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, nil))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, &NodeMetrics{XrayRunning: false}))
	assert.False(t, shouldPromoteNodeOnlineFromHeartbeat(repository.NodeStatusUnhealthy, &NodeMetrics{XrayRunning: true}))
}

type stubNodeAgentTrafficRecorder struct {
	err   error
	calls int
}

func (s *stubNodeAgentTrafficRecorder) RecordTrafficBatch(ctx context.Context, records []*node.TrafficRecord) error {
	s.calls++
	return s.err
}

func setupNodeAgentHeartbeatTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.Node{}))

	return db
}

func newNodeAgentHeartbeatHandler(t *testing.T, recorder nodeAgentTrafficRecorder) *NodeAgentHandler {
	t.Helper()

	db := setupNodeAgentHeartbeatTestDB(t)
	nodeRepo := repository.NewNodeRepository(db)
	require.NoError(t, db.Create(&repository.Node{
		ID:      1,
		Name:    "node-1",
		Address: "127.0.0.1",
		Token:   "node-token",
		Status:  repository.NodeStatusOnline,
	}).Error)

	return NewNodeAgentHandler(
		node.NewService(nodeRepo, nil, nil, logger.NewNopLogger()),
		recorder,
		nodeRepo,
		nil,
		nil,
		logger.NewNopLogger(),
	)
}

func TestNodeAgentHeartbeat_DeduplicatesTrafficBatchID(t *testing.T) {
	recorder := &stubNodeAgentTrafficRecorder{}
	handler := newNodeAgentHeartbeatHandler(t, recorder)

	router := gin.New()
	router.POST("/api/node/heartbeat", handler.Heartbeat)

	body, err := json.Marshal(HeartbeatRequest{
		NodeID:         1,
		Token:          "node-token",
		TrafficBatchID: "batch-1",
		Traffic: []TrafficRecord{
			{UserID: 7, Upload: 100, Download: 200},
		},
	})
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/node/heartbeat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 on request %d, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	if recorder.calls != 1 {
		t.Fatalf("expected duplicate batch to be recorded once, got %d calls", recorder.calls)
	}
}

func TestNodeAgentHeartbeat_RetriesFailedTrafficBatchID(t *testing.T) {
	recorder := &stubNodeAgentTrafficRecorder{err: errors.New("write failed")}
	handler := newNodeAgentHeartbeatHandler(t, recorder)

	router := gin.New()
	router.POST("/api/node/heartbeat", handler.Heartbeat)

	body, err := json.Marshal(HeartbeatRequest{
		NodeID:         1,
		Token:          "node-token",
		TrafficBatchID: "batch-retry",
		Traffic: []TrafficRecord{
			{UserID: 7, Upload: 100, Download: 200},
		},
	})
	require.NoError(t, err)

	req1 := httptest.NewRequest(http.MethodPost, "/api/node/heartbeat", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusInternalServerError {
		t.Fatalf("expected first request to fail with 500, got %d: %s", w1.Code, w1.Body.String())
	}

	recorder.err = nil

	req2 := httptest.NewRequest(http.MethodPost, "/api/node/heartbeat", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected retry request to succeed, got %d: %s", w2.Code, w2.Body.String())
	}

	if recorder.calls != 2 {
		t.Fatalf("expected failed batch to be retried, got %d calls", recorder.calls)
	}
}
