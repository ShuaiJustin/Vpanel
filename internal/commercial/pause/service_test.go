package pause

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

func createTestUserWithPermanentSubscription(t *testing.T, userRepo repository.UserRepository, userID int64) {
	t.Helper()

	user := &repository.User{
		ID:           userID,
		Username:     "permanent-user",
		Email:        "permanent@example.com",
		PasswordHash: "hashedpassword",
		Enabled:      true,
		TrafficLimit: 10737418240,
		TrafficUsed:  1073741824,
	}

	require.NoError(t, userRepo.Create(context.Background(), user))
}

func TestCanPause_AllowsPermanentSubscription(t *testing.T) {
	db := setupTestDB(t)
	pauseRepo := repository.NewPauseRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(pauseRepo, userRepo, logger.NewNopLogger(), DefaultConfig())

	createTestUserWithPermanentSubscription(t, userRepo, 1)

	canPause, reason := svc.CanPause(context.Background(), 1)

	assert.True(t, canPause)
	assert.Empty(t, reason)
}

func TestPauseResume_RestoresPermanentSubscription(t *testing.T) {
	db := setupTestDB(t)
	pauseRepo := repository.NewPauseRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(pauseRepo, userRepo, logger.NewNopLogger(), DefaultConfig())

	createTestUserWithPermanentSubscription(t, userRepo, 1)

	result, err := svc.Pause(context.Background(), 1)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, -1, result.Pause.RemainingDays)

	err = svc.Resume(context.Background(), 1)
	require.NoError(t, err)

	user, err := userRepo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, user.ExpiresAt)
}

func createTimedUser(t *testing.T, db *gorm.DB, userRepo repository.UserRepository, expires *time.Time, limit, used int64) {
	t.Helper()
	require.NoError(t, userRepo.Create(context.Background(), &repository.User{
		ID: 1, Username: "timed-user", Email: "timed@example.com", PasswordHash: "hashedpassword",
		Enabled: true, TrafficLimit: limit, TrafficUsed: used, ExpiresAt: expires, Balance: 500,
	}))
}

// The pause limit must also hold for plans with more than 30 days left.
func TestPause_LimitAppliesToLongPlans(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	ctx := context.Background()
	expires := time.Now().AddDate(0, 0, 300)
	createTimedUser(t, db, userRepo, &expires, 10<<30, 1<<30)

	_, err := svc.Pause(ctx, 1)
	require.NoError(t, err)
	require.NoError(t, svc.Resume(ctx, 1))
	_, err = svc.Pause(ctx, 1)
	require.Error(t, err, "a second pause within 30 days exceeds the default limit of one")
}

func TestPause_StopsSubscriptionUntilResume(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	ctx := context.Background()
	expires := time.Now().Add(10*24*time.Hour + time.Hour)
	createTimedUser(t, db, userRepo, &expires, 10<<30, 1<<30)

	_, err := svc.Pause(ctx, 1)
	require.NoError(t, err)
	paused, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, paused.ExpiresAt)
	assert.False(t, paused.ExpiresAt.After(time.Now()), "a paused subscription must not stay active")

	// A wallet change while paused must survive the resume.
	require.NoError(t, db.Model(&repository.User{}).Where("id = ?", 1).Update("balance", 900).Error)
	require.NoError(t, svc.Resume(ctx, 1))
	resumed, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, resumed.ExpiresAt)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, 10), *resumed.ExpiresAt, time.Minute)
	assert.Equal(t, int64(1<<30), resumed.TrafficUsed)
	assert.Equal(t, int64(900), resumed.Balance)
}

// Buying a plan while paused starts a new period; resuming must add the frozen
// days on top of it instead of replacing the purchase with the old remainder.
func TestResume_KeepsPlanBoughtWhilePaused(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	ctx := context.Background()
	expires := time.Now().Add(5*24*time.Hour + time.Hour)
	createTimedUser(t, db, userRepo, &expires, 100<<30, 80<<30)
	_, err := svc.Pause(ctx, 1)
	require.NoError(t, err)

	bought := time.Now().Add(30 * 24 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, db.Model(&repository.User{}).Where("id = ?", 1).Updates(map[string]any{
		"expires_at": bought, "traffic_limit": int64(200 << 30), "traffic_used": 0,
	}).Error)

	require.NoError(t, svc.Resume(ctx, 1))
	user, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, user.ExpiresAt)
	assert.WithinDuration(t, bought.AddDate(0, 0, 5), *user.ExpiresAt, time.Minute)
	assert.Equal(t, int64(0), user.TrafficUsed, "the new plan's quota must not be charged with the old plan's usage")
	assert.Equal(t, int64(200<<30), user.TrafficLimit)
}

func TestCanPause_RejectsAccountWithoutPlan(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	createTimedUser(t, db, userRepo, nil, 0, 0)

	canPause, reason := svc.CanPause(context.Background(), 1)
	assert.False(t, canPause)
	assert.Equal(t, "当前无有效订阅", reason)
}

func TestSubscriptionPauseJSONUsesSnakeCase(t *testing.T) {
	data, err := json.Marshal(repository.SubscriptionPause{RemainingDays: 3})
	require.NoError(t, err)
	for _, key := range []string{`"paused_at"`, `"auto_resume_at"`, `"remaining_days":3`, `"remaining_traffic"`, `"resumed_at"`} {
		assert.Contains(t, string(data), key)
	}
}

// A double click, or the auto-resume scheduler racing a click, must not hand
// the frozen days out twice.
func TestResume_HappensOnce(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	ctx := context.Background()
	expires := time.Now().Add(10*24*time.Hour + time.Hour)
	createTimedUser(t, db, userRepo, &expires, 10<<30, 1<<30)
	_, err := svc.Pause(ctx, 1)
	require.NoError(t, err)

	require.NoError(t, svc.Resume(ctx, 1))
	first, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)

	require.Error(t, svc.Resume(ctx, 1), "there is no pause left to resume")
	resumed, err := svc.AutoResumePaused(ctx)
	require.NoError(t, err)
	assert.Zero(t, resumed)

	second, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, second.ExpiresAt)
	assert.True(t, first.ExpiresAt.Equal(*second.ExpiresAt), "expiry moved from %v to %v", first.ExpiresAt, second.ExpiresAt)
	assert.WithinDuration(t, time.Now().AddDate(0, 0, 10), *second.ExpiresAt, time.Minute)
}

func TestPause_RejectsSecondPause(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	pauseRepo := repository.NewPauseRepository(db)
	expires := time.Now().Add(10 * 24 * time.Hour)
	createTimedUser(t, db, userRepo, &expires, 10<<30, 1<<30)
	atomicRepo := pauseRepo.(repository.AtomicPauseRepository)
	now := time.Now()

	require.NoError(t, atomicRepo.PauseSubscription(context.Background(), &repository.SubscriptionPause{UserID: 1, PausedAt: now, RemainingDays: 10, AutoResumeAt: now.AddDate(0, 0, 30)}, 3))
	err := atomicRepo.PauseSubscription(context.Background(), &repository.SubscriptionPause{UserID: 1, PausedAt: now, RemainingDays: 0, AutoResumeAt: now.AddDate(0, 0, 30)}, 3)
	require.ErrorIs(t, err, repository.ErrCommercialState, "a second pause would record the already-frozen account as having no days left")
}

// Access made permanent during the pause must stay permanent.
func TestResume_KeepsPermanentAccessGrantedWhilePaused(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewService(repository.NewPauseRepository(db), userRepo, logger.NewNopLogger(), DefaultConfig())
	ctx := context.Background()
	expires := time.Now().Add(5*24*time.Hour + time.Hour)
	createTimedUser(t, db, userRepo, &expires, 100<<30, 80<<30)
	_, err := svc.Pause(ctx, 1)
	require.NoError(t, err)
	require.NoError(t, db.Model(&repository.User{}).Where("id = ?", 1).Updates(map[string]any{"expires_at": nil, "traffic_used": 0}).Error)

	require.NoError(t, svc.Resume(ctx, 1))
	user, err := userRepo.GetByID(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, user.ExpiresAt)
	assert.Equal(t, int64(0), user.TrafficUsed)
}
