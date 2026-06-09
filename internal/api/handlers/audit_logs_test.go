package handlers

import (
	"encoding/json"
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
)

func TestAuditLogHandler_ListUsesSnakeCaseJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&repository.AuditLog{}))

	userID := int64(42)
	log := &repository.AuditLog{
		UserID:       &userID,
		Username:     "admin",
		Action:       "login",
		ResourceType: "auth",
		ResourceID:   "session",
		Details:      `{"result":"ok"}`,
		IPAddress:    "203.0.113.10",
		UserAgent:    "test-agent",
		RequestID:    "req-1",
		Status:       "success",
		CreatedAt:    time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC),
	}
	require.NoError(t, repository.NewAuditLogRepository(db).Create(t.Context(), log))

	handler := NewAuditLogHandler(repository.NewAuditLogRepository(db), logger.NewNopLogger())
	router := gin.New()
	router.GET("/audit-logs", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/audit-logs", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Logs []map[string]any `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Logs, 1)

	entry := body.Logs[0]
	require.Equal(t, "admin", entry["username"])
	require.Equal(t, "login", entry["action"])
	require.Equal(t, "auth", entry["resource_type"])
	require.Equal(t, "session", entry["resource_id"])
	require.Equal(t, "203.0.113.10", entry["ip_address"])
	require.Equal(t, "test-agent", entry["user_agent"])
	require.Equal(t, "req-1", entry["request_id"])
	require.Equal(t, "success", entry["status"])
	require.Contains(t, entry, "created_at")

	require.NotContains(t, entry, "ResourceType")
	require.NotContains(t, entry, "IPAddress")
	require.NotContains(t, entry, "CreatedAt")
}
