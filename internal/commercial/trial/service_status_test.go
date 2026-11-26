package trial

import (
	"context"
	"testing"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

func TestGetTrialNormalizesExpiredStatus(t *testing.T) {
	db := setupTestDB(t)
	trialRepo := repository.NewTrialRepository(db)
	userRepo := repository.NewUserRepository(db)
	if err := createTestUser(db, 1); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repoTrial := &repository.Trial{
		UserID:      1,
		Status:      "active",
		StartAt:     time.Now().AddDate(0, 0, -8),
		ExpireAt:    time.Now().Add(-2 * time.Hour),
		TrafficUsed: 128,
	}
	if err := trialRepo.Create(context.Background(), repoTrial); err != nil {
		t.Fatalf("create trial: %v", err)
	}

	svc := NewService(trialRepo, userRepo, logger.NewNopLogger(), DefaultConfig())
	trial, err := svc.GetTrial(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTrial failed: %v", err)
	}
	if trial.Status != "expired" {
		t.Fatalf("expected expired status, got %s", trial.Status)
	}

	stored, err := trialRepo.GetByUserID(context.Background(), 1)
	if err != nil {
		t.Fatalf("reload trial: %v", err)
	}
	if stored.Status != "expired" {
		t.Fatalf("expected stored status expired, got %s", stored.Status)
	}
}

func TestGetTrialMarksConvertedWhenUserHasActiveSubscription(t *testing.T) {
	db := setupTestDB(t)
	trialRepo := repository.NewTrialRepository(db)
	userRepo := repository.NewUserRepository(db)
	if err := createTestUser(db, 2); err != nil {
		t.Fatalf("create user: %v", err)
	}

	ctx := context.Background()
	user, err := userRepo.GetByID(ctx, 2)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	future := time.Now().Add(48 * time.Hour)
	if err := db.Model(user).Updates(map[string]any{"expires_at": future, "traffic_limit": 1024, "enabled": true}).Error; err != nil {
		t.Fatalf("update user: %v", err)
	}

	repoTrial := &repository.Trial{
		UserID:      2,
		Status:      "active",
		StartAt:     time.Now().Add(-24 * time.Hour),
		ExpireAt:    time.Now().Add(24 * time.Hour),
		TrafficUsed: 256,
	}
	if err := trialRepo.Create(ctx, repoTrial); err != nil {
		t.Fatalf("create trial: %v", err)
	}

	svc := NewService(trialRepo, userRepo, logger.NewNopLogger(), DefaultConfig())
	trial, err := svc.GetTrial(ctx, 2)
	if err != nil {
		t.Fatalf("GetTrial failed: %v", err)
	}
	if trial.Status != "converted" {
		t.Fatalf("expected converted status, got %s", trial.Status)
	}
	if trial.ConvertedAt == nil {
		t.Fatalf("expected converted_at to be set")
	}
}

func TestGetTrialRemainingDaysUsesCeiling(t *testing.T) {
	db := setupTestDB(t)
	trialRepo := repository.NewTrialRepository(db)
	userRepo := repository.NewUserRepository(db)
	if err := createTestUser(db, 3); err != nil {
		t.Fatalf("create user: %v", err)
	}

	repoTrial := &repository.Trial{
		UserID:      3,
		Status:      "active",
		StartAt:     time.Now().Add(-12 * time.Hour),
		ExpireAt:    time.Now().Add(12 * time.Hour),
		TrafficUsed: 0,
	}
	if err := trialRepo.Create(context.Background(), repoTrial); err != nil {
		t.Fatalf("create trial: %v", err)
	}

	svc := NewService(trialRepo, userRepo, logger.NewNopLogger(), DefaultConfig())
	trial, err := svc.GetTrial(context.Background(), 3)
	if err != nil {
		t.Fatalf("GetTrial failed: %v", err)
	}
	if trial.Status != "active" {
		t.Fatalf("expected active status, got %s", trial.Status)
	}
	if trial.RemainingDays != 1 {
		t.Fatalf("expected remaining_days 1, got %d", trial.RemainingDays)
	}
}

func TestActivateTrialRejectsActiveSubscription(t *testing.T) {
	db := setupTestDB(t)
	trialRepo := repository.NewTrialRepository(db)
	userRepo := repository.NewUserRepository(db)
	if err := createTestUser(db, 4); err != nil {
		t.Fatalf("create user: %v", err)
	}

	ctx := context.Background()
	user, err := userRepo.GetByID(ctx, 4)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	future := time.Now().Add(24 * time.Hour)
	if err := db.Model(user).Updates(map[string]any{"expires_at": future, "traffic_limit": 2048, "enabled": true}).Error; err != nil {
		t.Fatalf("update user: %v", err)
	}

	svc := NewService(trialRepo, userRepo, logger.NewNopLogger(), DefaultConfig())
	trial, err := svc.ActivateTrial(ctx, 4)
	if err != ErrActiveSubscription {
		t.Fatalf("expected ErrActiveSubscription, got %v", err)
	}
	if trial != nil {
		t.Fatalf("expected nil trial when activation is rejected")
	}
}

// The usual support request is "my trial ran out, give me a few more days".
// trials.user_id is unique, so that used to fail with a 500.
func TestGrantTrialReopensUsedTrialAndRejectsUnknownUser(t *testing.T) {
	db := setupTestDB(t)
	trialRepo := repository.NewTrialRepository(db)
	userRepo := repository.NewUserRepository(db)
	if err := createTestUser(db, 5); err != nil {
		t.Fatalf("create user: %v", err)
	}
	ctx := context.Background()
	converted := time.Now().Add(-48 * time.Hour)
	if err := trialRepo.Create(ctx, &repository.Trial{UserID: 5, Status: "expired", TrafficUsed: 4096,
		StartAt: time.Now().Add(-10 * 24 * time.Hour), ExpireAt: time.Now().Add(-3 * 24 * time.Hour), ConvertedAt: &converted}); err != nil {
		t.Fatalf("create trial: %v", err)
	}
	svc := NewService(trialRepo, userRepo, logger.NewNopLogger(), DefaultConfig())

	granted, err := svc.GrantTrial(ctx, 5, 7)
	if err != nil {
		t.Fatalf("re-grant failed: %v", err)
	}
	if granted.Status != "active" {
		t.Fatalf("status = %s, want active", granted.Status)
	}
	stored, err := trialRepo.GetByUserID(ctx, 5)
	if err != nil {
		t.Fatalf("load trial: %v", err)
	}
	if stored.TrafficUsed != 0 || stored.ConvertedAt != nil {
		t.Fatalf("re-granted trial must start clean: used=%d converted=%v", stored.TrafficUsed, stored.ConvertedAt)
	}
	if d := time.Until(stored.ExpireAt); d < 6*24*time.Hour || d > 7*24*time.Hour+time.Minute {
		t.Fatalf("expires in %s, want about 7 days", d)
	}

	if _, err := svc.GrantTrial(ctx, 999, 7); err != ErrUserNotFound {
		t.Fatalf("grant to unknown user: err = %v, want ErrUserNotFound", err)
	}
	if exists, _ := trialRepo.ExistsByUserID(ctx, 999); exists {
		t.Fatal("a trial was created for a user that does not exist")
	}
}
