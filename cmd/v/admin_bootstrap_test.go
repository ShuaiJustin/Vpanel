package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"v/internal/auth"
	"v/internal/config"
	"v/internal/database/repository"
	"v/internal/logger"
)

func TestEnsureAdminPreservesChangedPasswordAndProfile(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "bootstrap.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&repository.User{}))
	users := repository.NewUserRepository(db)
	svc := auth.NewService(auth.Config{JWTSecret: "bootstrap-test"})
	cfg := &config.Config{}
	cfg.Auth.AdminUsername = "admin"
	cfg.Auth.AdminPassword = "initial-password123"
	log := logger.NewNopLogger()
	require.NoError(t, ensureAdminUser(users, svc, cfg, log))
	user, err := users.GetByUsername(context.Background(), "admin")
	require.NoError(t, err)
	require.True(t, svc.VerifyPassword(cfg.Auth.AdminPassword, user.PasswordHash))
	changed, err := svc.HashPassword("user-changed-password456")
	require.NoError(t, err)
	user.PasswordHash = changed
	user.DisplayName = ""
	require.NoError(t, users.Update(context.Background(), user))
	before, err := users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.NoError(t, ensureAdminUser(users, svc, cfg, log))
	after, err := users.GetByID(context.Background(), user.ID)
	require.NoError(t, err)
	require.Equal(t, before.PasswordHash, after.PasswordHash)
	require.Equal(t, before.DisplayName, after.DisplayName)
	require.Equal(t, before.UpdatedAt, after.UpdatedAt)
	require.False(t, svc.VerifyPassword(cfg.Auth.AdminPassword, after.PasswordHash))
}

func TestStartupRejectsPositionalCommandsBeforeInitialization(t *testing.T) {
	require.NoError(t, validateStartupArguments(nil))
	require.ErrorContains(t, validateStartupArguments([]string{"version"}), "use -version")
	require.Error(t, validateStartupArguments([]string{"unknown"}))
}
