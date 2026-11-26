package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Installs seeded by older versions gave every customer proxy:write, which is
// the admin inbound API. Startup must take it back, and leave custom roles alone.
func TestEnsureSystemRolesReconcilesSeededPermissions(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&Role{}))
	repo := NewRoleRepository(db)
	ctx := context.Background()

	legacy := &Role{Name: "user", IsSystem: true}
	require.NoError(t, legacy.SetPermissionsList([]string{"proxy:read", "proxy:write", "profile:read", "profile:write"}))
	require.NoError(t, repo.Create(ctx, legacy))
	custom := &Role{Name: "ops"}
	require.NoError(t, custom.SetPermissionsList([]string{"proxy:write"}))
	require.NoError(t, repo.Create(ctx, custom))

	require.NoError(t, repo.EnsureSystemRoles(ctx))

	user, err := repo.GetByName(ctx, "user")
	require.NoError(t, err)
	perms, err := user.GetPermissionsList()
	require.NoError(t, err)
	require.Equal(t, []string{"profile:read", "profile:write"}, perms)

	ops, err := repo.GetByName(ctx, "ops")
	require.NoError(t, err)
	perms, err = ops.GetPermissionsList()
	require.NoError(t, err)
	require.Equal(t, []string{"proxy:write"}, perms)

	admin, err := repo.GetByName(ctx, "admin")
	require.NoError(t, err)
	perms, _ = admin.GetPermissionsList()
	require.Equal(t, []string{"*"}, perms)
}

func TestDeleteTwoFactorSecretClearsLoginFlag(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	user := &User{Username: "totp", PasswordHash: "x", Enabled: true}
	require.NoError(t, NewUserRepository(db).Create(ctx, user))
	tokens := NewAuthTokenRepository(db)
	require.NoError(t, tokens.CreateTwoFactorSecret(ctx, &TwoFactorSecret{UserID: user.ID, Secret: "JBSWY3DPEHPK3PXP"}))
	require.NoError(t, tokens.EnableTwoFactor(ctx, user.ID))

	require.NoError(t, tokens.DeleteTwoFactorSecret(ctx, user.ID))

	var stored User
	require.NoError(t, db.First(&stored, user.ID).Error)
	require.False(t, stored.TwoFactorEnabled, "login would still demand a code that can no longer be verified")
}

func TestGiftCardSetStatusCannotReviveRedeemedCard(t *testing.T) {
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&GiftCard{}))
	repo := NewGiftCardRepository(db)
	ctx := context.Background()
	card := &GiftCard{Code: "GC-REDEEMED", Value: 1000, Status: GiftCardStatusRedeemed}
	require.NoError(t, db.Create(card).Error)

	require.ErrorIs(t, repo.SetStatus(ctx, card.ID, GiftCardStatusDisabled), ErrCommercialState)
	require.ErrorIs(t, repo.SetStatus(ctx, card.ID, GiftCardStatusActive), ErrCommercialState)
	var stored GiftCard
	require.NoError(t, db.First(&stored, card.ID).Error)
	require.Equal(t, GiftCardStatusRedeemed, stored.Status)

	fresh := &GiftCard{Code: "GC-ACTIVE", Value: 1000, Status: GiftCardStatusActive}
	require.NoError(t, db.Create(fresh).Error)
	require.NoError(t, repo.SetStatus(ctx, fresh.ID, GiftCardStatusDisabled))
	require.NoError(t, repo.SetStatus(ctx, fresh.ID, GiftCardStatusActive))
}
