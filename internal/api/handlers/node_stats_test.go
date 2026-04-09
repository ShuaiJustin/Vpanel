package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func newNodeStatsTestHandler(t *testing.T) (*NodeStatsHandler, *repository.Repositories) {
	t.Helper()

	db := setupStatsTestDB(t)
	if err := db.AutoMigrate(&repository.NodeTraffic{}); err != nil {
		t.Fatalf("failed to migrate node traffic table: %v", err)
	}

	nodeTrafficRepo := repository.NewNodeTrafficRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	userRepo := repository.NewUserRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	groupRepo := repository.NewNodeGroupRepository(db)
	trafficService := node.NewTrafficService(db, nodeTrafficRepo, trafficRepo, proxyRepo, userRepo, nodeRepo, groupRepo, logger.NewNopLogger())

	return NewNodeStatsHandler(trafficService, nil, nil, logger.NewNopLogger()), repository.NewRepositories(db)
}

func TestRecordTraffic_AllowsZeroUploadOrDownload(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.POST("/api/admin/nodes/traffic", handler.RecordTraffic)

	payload, _ := json.Marshal(map[string]any{
		"node_id":  1,
		"user_id":  2,
		"upload":   0,
		"download": 1024,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/traffic", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusBadRequest {
		t.Fatalf("expected zero upload/download to be accepted, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecordTraffic_AllowsSharedTrafficUserIDZero(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.POST("/api/admin/nodes/traffic", handler.RecordTraffic)

	payload, _ := json.Marshal(map[string]any{
		"node_id":  1,
		"user_id":  0,
		"upload":   512,
		"download": 0,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/traffic", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected shared traffic to be accepted, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecordTraffic_RejectsZeroTotalTraffic(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.POST("/api/admin/nodes/traffic", handler.RecordTraffic)

	payload, _ := json.Marshal(map[string]any{
		"node_id":  1,
		"user_id":  2,
		"upload":   0,
		"download": 0,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/traffic", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for zero-total traffic, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRecordTrafficBatch_RejectsNegativeTraffic(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.POST("/api/admin/nodes/traffic/batch", handler.RecordTrafficBatch)

	payload, _ := json.Marshal(map[string]any{
		"records": []map[string]any{
			{
				"node_id":  1,
				"user_id":  2,
				"upload":   -1,
				"download": 1024,
			},
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/traffic/batch", bytes.NewBuffer(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative traffic, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTotalTraffic_RejectsInvalidStartTime(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.GET("/api/admin/nodes/traffic/total", handler.GetTotalTraffic)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/nodes/traffic/total?start=not-a-time", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid start time, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTotalTraffic_RejectsEndBeforeStart(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.GET("/api/admin/nodes/traffic/total", handler.GetTotalTraffic)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/nodes/traffic/total?start=2026-03-20T01:00:00Z&end=2026-03-20T00:00:00Z",
		nil,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for end before start, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetTotalTraffic_FormatsRangeAsRFC3339UTC(t *testing.T) {
	handler, repos := newNodeStatsTestHandler(t)

	recordedAt := time.Date(2026, time.March, 20, 0, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := repos.DB().Create(&repository.NodeTraffic{
		NodeID:     1,
		UserID:     1,
		Upload:     10,
		Download:   20,
		RecordedAt: recordedAt,
	}).Error; err != nil {
		t.Fatalf("failed to seed node traffic: %v", err)
	}

	router := gin.New()
	router.GET("/api/admin/nodes/traffic/total", handler.GetTotalTraffic)

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/admin/nodes/traffic/total?start=2026-03-20T00:00:00%2B08:00&end=2026-03-20T01:00:00%2B08:00",
		nil,
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload struct {
		Start string `json:"start"`
		End   string `json:"end"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	start, err := time.Parse(time.RFC3339, payload.Start)
	if err != nil {
		t.Fatalf("expected RFC3339 start time, got %q: %v", payload.Start, err)
	}
	end, err := time.Parse(time.RFC3339, payload.End)
	if err != nil {
		t.Fatalf("expected RFC3339 end time, got %q: %v", payload.End, err)
	}

	expectedStart := time.Date(2026, time.March, 20, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60)).UTC()
	expectedEnd := time.Date(2026, time.March, 20, 1, 0, 0, 0, time.FixedZone("CST", 8*60*60)).UTC()
	if !start.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, start)
	}
	if !end.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, end)
	}
}

func TestCleanupOldRecords_RejectsNonPositiveRetentionDays(t *testing.T) {
	handler, _ := newNodeStatsTestHandler(t)

	router := gin.New()
	router.POST("/api/admin/nodes/traffic/cleanup", handler.CleanupOldRecords)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/traffic/cleanup?retention_days=0", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-positive retention days, got %d: %s", w.Code, w.Body.String())
	}
}
