package pause

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	assert.Equal(t, permanentPauseRemainingDays, result.Pause.RemainingDays)

	err = svc.Resume(context.Background(), 1)
	require.NoError(t, err)

	user, err := userRepo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	assert.Nil(t, user.ExpiresAt)
}
