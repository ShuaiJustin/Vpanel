package handlers

import (
	"context"
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
