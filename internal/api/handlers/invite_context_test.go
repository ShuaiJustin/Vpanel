package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"v/internal/commercial/invite"
	"v/internal/database/repository"
	"v/internal/logger"
)

func setupInviteHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open invite test database: %v", err)
	}

	if err := db.AutoMigrate(&repository.CommercialInviteCode{}); err != nil {
		t.Fatalf("failed to migrate invite code table: %v", err)
	}

	return db
}

func TestGetInviteCode_UsesUserIDContextKey(t *testing.T) {
	db := setupInviteHandlerTestDB(t)
	repo := repository.NewInviteRepository(db)
	handler := NewInviteHandler(
		invite.NewService(repo, logger.NewNopLogger(), &invite.Config{BaseURL: "https://example.com"}),
		nil,
		logger.NewNopLogger(),
	)

	router := gin.New()
	router.GET("/invite/code", func(c *gin.Context) {
		c.Set("user_id", int64(42))
		handler.GetInviteCode(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/invite/code", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	code, err := repo.GetInviteCodeByUserID(context.Background(), 42)
	if err != nil {
		t.Fatalf("expected invite code to be created: %v", err)
	}
	if code == nil || code.Code == "" {
		t.Fatal("expected generated invite code")
	}
}
