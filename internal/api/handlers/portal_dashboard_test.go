package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/entitlement"
	"v/internal/logger"
)

type stubPortalDashboardEntitlement struct {
	state *entitlement.AccessState
	err   error
}

func (s *stubPortalDashboardEntitlement) EvaluateAccess(ctx context.Context, userID int64) (*entitlement.AccessState, error) {
	return s.state, s.err
}

func setupPortalDashboardHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open dashboard test database: %v", err)
	}
	if err := db.AutoMigrate(&repository.User{}); err != nil {
		t.Fatalf("failed to migrate dashboard test tables: %v", err)
	}
	return db
}

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

func TestPortalDashboardHandler_GetDashboardUsesEffectiveTrafficState(t *testing.T) {
	db := setupPortalDashboardHandlerTestDB(t)
	userRepo := repository.NewUserRepository(db)

	expiresAt := time.Now().Add(24 * time.Hour).UTC()
	user := &repository.User{
		Username:     "dashboard-user",
		PasswordHash: "x",
		Email:        "dashboard@example.com",
		Enabled:      true,
		TrafficLimit: 10,
		TrafficUsed:  9,
		ExpiresAt:    &expiresAt,
	}
	if err := userRepo.Create(context.Background(), user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	handler := NewPortalDashboardHandler(userRepo, nil, nil, logger.NewNopLogger()).WithEntitlementService(&stubPortalDashboardEntitlement{
		state: &entitlement.AccessState{
			EffectiveTrafficLimit: 2048,
			EffectiveTrafficUsed:  1024,
		},
	})

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Next()
	})
	router.GET("/portal/dashboard", handler.GetDashboard)

	req := httptest.NewRequest(http.MethodGet, "/portal/dashboard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var payload struct {
		Traffic struct {
			Used       int64   `json:"used"`
			Limit      int64   `json:"limit"`
			Percentage float64 `json:"percentage"`
			LimitStr   string  `json:"limit_str"`
		} `json:"traffic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if payload.Traffic.Used != 1024 {
		t.Fatalf("expected effective used 1024, got %d", payload.Traffic.Used)
	}
	if payload.Traffic.Limit != 2048 {
		t.Fatalf("expected effective limit 2048, got %d", payload.Traffic.Limit)
	}
	if payload.Traffic.Percentage != 50 {
		t.Fatalf("expected traffic percentage 50, got %v", payload.Traffic.Percentage)
	}
	if payload.Traffic.LimitStr != "2.00 KB" {
		t.Fatalf("expected formatted limit 2.00 KB, got %q", payload.Traffic.LimitStr)
	}
}
