package auth

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"

	"v/internal/database/repository"
)

func sessionTestService(t *testing.T) (*Service, *gorm.DB, *repository.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "sessions.db")), &gorm.Config{Logger: gormlog.Default.LogMode(gormlog.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}))
	user := &repository.User{Username: "session-user", PasswordHash: "original-password-hash", Role: "admin", Enabled: true}
	require.NoError(t, db.Create(user).Error)
	store, err := NewDatabaseSessionStore(db)
	require.NoError(t, err)
	service := NewService(Config{JWTSecret: "session-test-secret"}).WithSessionStore(store).WithUserRepository(repository.NewUserRepository(db))
	return service, db, user
}

func TestSessionPurposeAndPersistentLogout(t *testing.T) {
	s, db, user := sessionTestService(t)
	access, refresh, err := s.GenerateTokenPair(user.ID, user.Username, user.Role, time.Hour, user.PasswordHash)
	require.NoError(t, err)
	_, err = s.ValidateToken(access)
	require.NoError(t, err)
	_, err = s.ValidateRefreshToken(refresh)
	require.NoError(t, err)
	_, err = s.ValidateToken(refresh)
	require.Error(t, err)
	_, err = s.ValidateRefreshToken(access)
	require.Error(t, err)
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": user.ID, "role": "admin", "exp": time.Now().Add(time.Hour).Unix()}).SignedString([]byte("session-test-secret"))
	require.NoError(t, err)
	_, err = s.ValidateToken(legacy)
	require.Error(t, err)
	require.NoError(t, s.RevokeToken(context.Background(), access, time.Time{}))
	store, err := NewDatabaseSessionStore(db)
	require.NoError(t, err)
	restarted := NewService(s.config).WithSessionStore(store).WithUserRepository(repository.NewUserRepository(db))
	_, err = restarted.ValidateToken(access)
	require.Error(t, err)
	_, err = restarted.ValidateRefreshToken(refresh)
	require.Error(t, err)
}

func TestPasswordChangeInvalidatesEveryTokenAndPendingChallenge(t *testing.T) {
	s, db, user := sessionTestService(t)
	access, refresh, err := s.GenerateTokenPair(user.ID, user.Username, user.Role, time.Hour, user.PasswordHash)
	require.NoError(t, err)
	challenge, err := s.GenerateLoginChallenge(user.ID, user.PasswordHash)
	require.NoError(t, err)
	require.NoError(t, db.Model(&repository.User{}).Where("id = ?", user.ID).Update("password", "new-password-hash").Error)
	_, err = s.ValidateToken(access)
	require.Error(t, err)
	_, err = s.ValidateRefreshToken(refresh)
	require.Error(t, err)
	require.Error(t, s.ValidateLoginChallenge(challenge, user.ID))
	_, _, err = s.GenerateTokenPair(user.ID, user.Username, user.Role, time.Hour, user.PasswordHash)
	require.Error(t, err, "a password check that raced a password change cannot open a new session")
}

func TestRefreshAndChallengeHaveOneAtomicConsumer(t *testing.T) {
	s, db, user := sessionTestService(t)
	access, refresh, err := s.GenerateTokenPair(user.ID, user.Username, user.Role, time.Hour)
	require.NoError(t, err)
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.ConsumeRefreshToken(context.Background(), refresh) == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), successes.Load())
	_, err = s.ValidateToken(access)
	require.Error(t, err)
	challenge, err := s.GenerateLoginChallenge(user.ID, user.PasswordHash)
	require.NoError(t, err)
	_, err = s.ValidateToken(challenge)
	require.Error(t, err)
	_, err = s.ValidateRefreshToken(challenge)
	require.Error(t, err)
	require.Error(t, s.ValidateLoginChallenge(challenge, user.ID+1))
	require.NoError(t, s.ConsumeLoginChallenge(context.Background(), challenge, user.ID))
	require.Error(t, s.ConsumeLoginChallenge(context.Background(), challenge, user.ID))
	challenge, err = s.GenerateLoginChallenge(user.ID, user.PasswordHash)
	require.NoError(t, err)
	claims, err := s.validateClaims(challenge, "2fa", "2fa")
	require.NoError(t, err)
	require.NoError(t, db.Model(&Session{}).Where("id = ?", claims.ID).Update("expires_at", time.Now().Add(-time.Minute)).Error)
	require.Error(t, s.ValidateLoginChallenge(challenge, user.ID))
}
