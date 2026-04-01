package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"v/internal/logger"
)

func TestPortalDashboardHandler_RejectsInvalidUserContext(t *testing.T) {
	handler := NewPortalDashboardHandler(nil, nil, nil, logger.NewNopLogger())

	tests := []struct {
		name string
		path string
	}{
		{name: "dashboard", path: "/portal/dashboard"},
		{name: "traffic summary", path: "/portal/dashboard/traffic"},
		{name: "recent announcements", path: "/portal/dashboard/announcements"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("user_id", "not-an-int64")
				c.Next()
			})
			router.GET("/portal/dashboard", handler.GetDashboard)
			router.GET("/portal/dashboard/traffic", handler.GetTrafficSummary)
			router.GET("/portal/dashboard/announcements", handler.GetRecentAnnouncements)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401 for invalid user context on %s, got %d: %s", tt.path, w.Code, w.Body.String())
			}
		})
	}
}
