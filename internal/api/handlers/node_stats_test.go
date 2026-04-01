package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/node"
)

func TestRecordTraffic_AllowsZeroUploadOrDownload(t *testing.T) {
	db := setupStatsTestDB(t)
	nodeTrafficRepo := repository.NewNodeTrafficRepository(db)
	trafficRepo := repository.NewTrafficRepository(db)
	proxyRepo := repository.NewProxyRepository(db)
	userRepo := repository.NewUserRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	groupRepo := repository.NewNodeGroupRepository(db)
	trafficService := node.NewTrafficService(db, nodeTrafficRepo, trafficRepo, proxyRepo, userRepo, nodeRepo, groupRepo, logger.NewNopLogger())
	handler := NewNodeStatsHandler(trafficService, nil, nil, logger.NewNopLogger())

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
