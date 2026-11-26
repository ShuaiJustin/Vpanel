package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
)

func TestPortalStatsHandler_GetTrafficStatsRejectsInvalidUserContext(t *testing.T) {
	handler := NewPortalStatsHandler(nil, logger.NewNopLogger())

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "not-an-int64")
		c.Next()
	})
	router.GET("/portal/stats/traffic", handler.GetTrafficStats)

	req := httptest.NewRequest(http.MethodGet, "/portal/stats/traffic", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid user context, got %d: %s", w.Code, w.Body.String())
	}
}
