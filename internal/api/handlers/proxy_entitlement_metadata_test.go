package handlers

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	trialsvc "v/internal/commercial/trial"
	"v/internal/database/repository"
	"v/internal/logger"
)

type mockUserMetadataRepo struct {
	user *repository.User
}

func (m *mockUserMetadataRepo) Create(ctx context.Context, user *repository.User) error {
	m.user = user
	return nil
}

func (m *mockUserMetadataRepo) GetByID(ctx context.Context, id int64) (*repository.User, error) {
	if m.user != nil && m.user.ID == id {
		return m.user, nil
	}
	return nil, context.Canceled
}

func (m *mockUserMetadataRepo) GetByUsername(ctx context.Context, username string) (*repository.User, error) {
	return nil, context.Canceled
}

func (m *mockUserMetadataRepo) GetByEmail(ctx context.Context, email string) (*repository.User, error) {
	return nil, context.Canceled
}

func (m *mockUserMetadataRepo) Update(ctx context.Context, user *repository.User) error {
	m.user = user
	return nil
}

func (m *mockUserMetadataRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockUserMetadataRepo) List(ctx context.Context, limit, offset int) ([]*repository.User, error) {
	return nil, nil
}

func (m *mockUserMetadataRepo) Count(ctx context.Context) (int64, error) {
	if m.user == nil {
		return 0, nil
	}
	return 1, nil
}

func (m *mockUserMetadataRepo) CountActive(ctx context.Context) (int64, error) {
	if m.user == nil || !m.user.Enabled {
		return 0, nil
	}
	return 1, nil
}

type mockTrialMetadataRepo struct {
	trial *repository.Trial
}

func (m *mockTrialMetadataRepo) Create(ctx context.Context, trial *repository.Trial) error {
	m.trial = trial
	return nil
}

func (m *mockTrialMetadataRepo) GetByID(ctx context.Context, id int64) (*repository.Trial, error) {
	if m.trial != nil && m.trial.ID == id {
		return m.trial, nil
	}
	return nil, context.Canceled
}

func (m *mockTrialMetadataRepo) GetByUserID(ctx context.Context, userID int64) (*repository.Trial, error) {
	if m.trial != nil && m.trial.UserID == userID {
		return m.trial, nil
	}
	return nil, context.Canceled
}

func (m *mockTrialMetadataRepo) Update(ctx context.Context, trial *repository.Trial) error {
	m.trial = trial
	return nil
}

func (m *mockTrialMetadataRepo) UpdateStatus(ctx context.Context, id int64, status string) error {
	if m.trial != nil && m.trial.ID == id {
		m.trial.Status = status
	}
	return nil
}

func (m *mockTrialMetadataRepo) UpdateTrafficUsed(ctx context.Context, id int64, trafficUsed int64) error {
	if m.trial != nil && m.trial.ID == id {
		m.trial.TrafficUsed = trafficUsed
	}
	return nil
}

func (m *mockTrialMetadataRepo) MarkConverted(ctx context.Context, userID int64) error {
	if m.trial != nil && m.trial.UserID == userID {
		now := time.Now()
		m.trial.Status = "converted"
		m.trial.ConvertedAt = &now
	}
	return nil
}

func (m *mockTrialMetadataRepo) ListExpired(ctx context.Context) ([]*repository.Trial, error) {
	return nil, nil
}

func (m *mockTrialMetadataRepo) ListActive(ctx context.Context) ([]*repository.Trial, error) {
	if m.trial != nil && m.trial.Status == "active" {
		return []*repository.Trial{m.trial}, nil
	}
	return nil, nil
}

func (m *mockTrialMetadataRepo) CountByStatus(ctx context.Context, status string) (int64, error) {
	if m.trial != nil && m.trial.Status == status {
		return 1, nil
	}
	return 0, nil
}

func (m *mockTrialMetadataRepo) CountConverted(ctx context.Context) (int64, error) {
	if m.trial != nil && m.trial.Status == "converted" {
		return 1, nil
	}
	return 0, nil
}

func (m *mockTrialMetadataRepo) CountTotal(ctx context.Context) (int64, error) {
	if m.trial == nil {
		return 0, nil
	}
	return 1, nil
}

func (m *mockTrialMetadataRepo) ExistsByUserID(ctx context.Context, userID int64) (bool, error) {
	return m.trial != nil && m.trial.UserID == userID, nil
}

func TestBuildProxyResponse_UsesActiveTrialTrafficLimit(t *testing.T) {
	userRepo := &mockUserMetadataRepo{
		user: &repository.User{
			ID:            42,
			Username:      "trial-user",
			Email:         "trial@example.com",
			PasswordHash:  "hashed",
			Enabled:       true,
			EmailVerified: true,
		},
	}
	trialRepo := &mockTrialMetadataRepo{
		trial: &repository.Trial{
			ID:          1,
			UserID:      42,
			Status:      "active",
			StartAt:     time.Now().Add(-time.Hour),
			ExpireAt:    time.Now().Add(7 * 24 * time.Hour),
			TrafficUsed: 0,
		},
	}

	trialService := trialsvc.NewService(trialRepo, userRepo, logger.NewNopLogger(), trialsvc.DefaultConfig())
	handler := NewProxyHandler(nil, nil, logger.NewNopLogger()).
		WithUserRepositories(userRepo, trialRepo).
		WithTrialService(trialService)

	response := handler.buildProxyResponse(context.Background(), &repository.Proxy{
		UserID:    42,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	if response.ExpirySource != "trial" {
		t.Fatalf("expected expiry source trial, got %q", response.ExpirySource)
	}
	if response.TrafficSource != "trial" {
		t.Fatalf("expected traffic source trial, got %q", response.TrafficSource)
	}
	if response.TrafficLimit != trialsvc.DefaultConfig().TrafficLimit {
		t.Fatalf("expected traffic limit %d, got %d", trialsvc.DefaultConfig().TrafficLimit, response.TrafficLimit)
	}
}

func TestDefaultTrialConfigUses100GBTrafficLimit(t *testing.T) {
	if got, want := trialsvc.DefaultConfig().TrafficLimit, int64(107374182400); got != want {
		t.Fatalf("expected default traffic limit %d, got %d", want, got)
	}
}

func TestProxyListBatchesNodeAndUserMetadata(t *testing.T) {
	db := setupStatsTestDB(t)
	if err := db.AutoMigrate(&repository.Trial{}); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	if err := db.Create(&repository.User{ID: 1, Username: "batch-user", ExpiresAt: &expires, TrafficLimit: 1000}).Error; err != nil {
		t.Fatal(err)
	}
	nodeID := int64(1)
	if err := db.Create(&repository.Node{ID: nodeID, Name: "batch-node", Address: "example.com", Token: "batch"}).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if err := db.Create(&repository.Proxy{UserID: 1, NodeID: &nodeID, Name: "proxy", Protocol: "vless", Port: 20000 + i}).Error; err != nil {
			t.Fatal(err)
		}
	}
	counts := map[string]int{}
	if err := db.Callback().Query().Before("gorm:query").Register("count_proxy_queries", func(tx *gorm.DB) { counts[tx.Statement.Table]++ }); err != nil {
		t.Fatal(err)
	}
	h := NewProxyHandler(nil, repository.NewProxyRepository(db), logger.NewNopLogger()).WithNodeRepository(repository.NewNodeRepository(db)).WithUserRepositories(repository.NewUserRepository(db), repository.NewTrialRepository(db))
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/proxies", nil)
	c.Set("user_id", int64(1))
	c.Set("role", "admin")
	h.List(c)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	var rows []ProxyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 20 || rows[0].Host != "example.com" || rows[0].TrafficLimit != 1000 {
		t.Fatalf("incorrect rows: %+v", rows)
	}
	if counts["nodes"] != 1 || counts["users"] != 1 || counts["trials"] != 1 {
		t.Fatalf("unexpected metadata queries: %+v", counts)
	}
}

type failingBatchNodeRepo struct{ repository.NodeRepository }

func (r *failingBatchNodeRepo) GetByIDs(context.Context, []int64) ([]*repository.Node, error) {
	return nil, context.DeadlineExceeded
}

func TestProxyListRetainsHostWhenBatchNodeMetadataFails(t *testing.T) {
	r := newMockProxyRepository()
	id := int64(1)
	if err := r.Create(context.Background(), &repository.Proxy{Name: "fallback", Host: "fallback.example", NodeID: &id}); err != nil {
		t.Fatal(err)
	}
	h := NewProxyHandler(nil, r, logger.NewNopLogger()).WithNodeRepository(&failingBatchNodeRepo{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/proxies", nil)
	c.Set("user_id", int64(1))
	c.Set("role", "admin")
	h.List(c)
	var rows []ProxyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || len(rows) != 1 || rows[0].Host != "fallback.example" {
		t.Fatalf("fallback failed: status=%d body=%s", w.Code, w.Body.String())
	}
}
