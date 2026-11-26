package repository

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"v/pkg/errors"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto migrate
	if err := db.AutoMigrate(
		&User{},
		&Proxy{},
		&Traffic{},
		&Subscription{},
		&LoginHistory{},
		&UserNodeAssignment{},
		&PasswordResetToken{},
		&EmailVerificationToken{},
		&TwoFactorSecret{},
	); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	return db
}

func TestUserRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		Email:        "test@example.com",
		Role:         "user",
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be set after creation")
	}
}

func TestUserRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create a user first
	user := &User{
		Username:     "testuser",
		PasswordHash: "hashedpassword",
		Email:        "test@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, user)

	// Get by ID
	found, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if found.Username != user.Username {
		t.Errorf("Expected username %s, got %s", user.Username, found.Username)
	}

	// Test not found
	_, err = repo.GetByID(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestUserRepository_GetByUsername(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "uniqueuser",
		PasswordHash: "hashedpassword",
		Email:        "unique@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, user)

	found, err := repo.GetByUsername(ctx, "uniqueuser")
	if err != nil {
		t.Fatalf("Failed to get user by username: %v", err)
	}

	if found.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, found.ID)
	}

	// Test not found
	_, err = repo.GetByUsername(ctx, "nonexistent")
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestUserRepository_GetByUsernameIsTrimmedAndCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "CaseUser",
		PasswordHash: "hashedpassword",
		Email:        "case@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	found, err := repo.GetByUsername(ctx, "  caseuser ")
	if err != nil {
		t.Fatalf("Failed to get user by normalized username: %v", err)
	}

	if found.ID != user.ID {
		t.Fatalf("Expected user ID %d, got %d", user.ID, found.ID)
	}
	if found.Username != "CaseUser" {
		t.Fatalf("Expected original username CaseUser, got %q", found.Username)
	}
}

func TestUserRepository_GetByEmailIsCaseInsensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "emailcaseuser",
		PasswordHash: "hashedpassword",
		Email:        "CaseUser@Example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	found, err := repo.GetByEmail(ctx, "caseuser@example.com")
	if err != nil {
		t.Fatalf("Failed to get user by normalized email: %v", err)
	}

	if found.ID != user.ID {
		t.Errorf("Expected user ID %d, got %d", user.ID, found.ID)
	}

	if found.Email != "caseuser@example.com" {
		t.Errorf("Expected normalized email caseuser@example.com, got %s", found.Email)
	}
}

func TestUserRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "updateuser",
		PasswordHash: "hashedpassword",
		Email:        "update@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, user)

	// Update user
	user.Email = "updated@example.com"
	err := repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// Verify update
	found, _ := repo.GetByID(ctx, user.ID)
	if found.Email != "updated@example.com" {
		t.Errorf("Expected email updated@example.com, got %s", found.Email)
	}
}

func TestUserRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{
		Username:     "deleteuser",
		PasswordHash: "hashedpassword",
		Email:        "delete@example.com",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	repo.Create(ctx, user)

	// Delete user
	err := repo.Delete(ctx, user.ID)
	if err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	// Verify deletion
	_, err = repo.GetByID(ctx, user.ID)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error after deletion, got: %v", err)
	}

	// Test delete non-existent
	err = repo.Delete(ctx, 99999)
	if !errors.IsNotFound(err) {
		t.Errorf("Expected not found error for non-existent user, got: %v", err)
	}
}

func TestUserRepository_DeleteCleansRuntimeDependencies(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	now := time.Now()

	user := &User{
		Username:     "cleanup-user",
		PasswordHash: "hashedpassword",
		Email:        "cleanup@example.com",
	}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if err := db.WithContext(ctx).Create(&Proxy{
		UserID:   user.ID,
		Name:     "cleanup-proxy",
		Protocol: "vmess",
		Port:     20001,
		Enabled:  true,
	}).Error; err != nil {
		t.Fatalf("Failed to create proxy: %v", err)
	}
	if err := db.WithContext(ctx).Create(&Subscription{
		UserID: user.ID,
		Token:  "cleanup-subscription-token",
	}).Error; err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}
	if err := db.WithContext(ctx).Create(&LoginHistory{
		UserID: user.ID,
		IP:     "127.0.0.1",
	}).Error; err != nil {
		t.Fatalf("Failed to create login history: %v", err)
	}
	if err := db.WithContext(ctx).Create(&UserNodeAssignment{
		UserID:     user.ID,
		NodeID:     1,
		AssignedAt: now,
		UpdatedAt:  now,
	}).Error; err != nil {
		t.Fatalf("Failed to create assignment: %v", err)
	}
	if err := db.WithContext(ctx).Create(&PasswordResetToken{
		UserID:    user.ID,
		Token:     "cleanup-reset-token",
		ExpiresAt: now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("Failed to create password reset token: %v", err)
	}
	if err := db.WithContext(ctx).Create(&EmailVerificationToken{
		UserID:    user.ID,
		Email:     user.Email,
		Token:     "cleanup-verify-token",
		ExpiresAt: now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("Failed to create email verification token: %v", err)
	}
	if err := db.WithContext(ctx).Create(&TwoFactorSecret{
		UserID: user.ID,
		Secret: "cleanup-secret",
	}).Error; err != nil {
		t.Fatalf("Failed to create two-factor secret: %v", err)
	}

	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Failed to delete user: %v", err)
	}

	for _, check := range []struct {
		table string
	}{
		{table: "proxies"},
		{table: "subscriptions"},
		{table: "login_history"},
		{table: "user_node_assignments"},
		{table: "password_reset_tokens"},
		{table: "email_verification_tokens"},
		{table: "two_factor_secrets"},
	} {
		var count int64
		if err := db.WithContext(ctx).Table(check.table).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
			t.Fatalf("Failed to count %s: %v", check.table, err)
		}
		if count != 0 {
			t.Fatalf("Expected %s to be cleaned up, got %d rows", check.table, count)
		}
	}
}

func TestUserRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 15; i++ {
		user := &User{
			Username:     "listuser" + string(rune('a'+i)),
			PasswordHash: "hashedpassword",
			Email:        "list" + string(rune('a'+i)) + "@example.com",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		repo.Create(ctx, user)
	}

	// Test pagination
	users, err := repo.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("Failed to list users: %v", err)
	}

	if len(users) != 10 {
		t.Errorf("Expected 10 users on page 1, got %d", len(users))
	}

	// Test page 2
	users, err = repo.List(ctx, 10, 10)
	if err != nil {
		t.Fatalf("Failed to list users page 2: %v", err)
	}

	if len(users) != 5 {
		t.Errorf("Expected 5 users on page 2, got %d", len(users))
	}
}

// A handler loads a user, does slow work (bcrypt, SMTP), then saves it. Traffic
// ingestion, wallet debits and plan transactions land in between as atomic
// statements; the save must not write the stale copy over them.
func TestUserRepository_UpdateKeepsConcurrentLedgerWrites(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	oldExpiry := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	user := &User{Username: "buyer", PasswordHash: "hash", Role: "user", Enabled: true,
		TrafficLimit: 100, TrafficUsed: 90, Balance: 5000, ExpiresAt: &oldExpiry}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("create: %v", err)
	}

	stale, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// Meanwhile: a purchase debits the wallet and applies a plan, then a node reports traffic.
	newExpiry := oldExpiry.Add(30 * 24 * time.Hour)
	resetAt := MonthlyTrafficAnchor(time.Now().UTC())
	if err := db.Model(&User{}).Where("id = ?", user.ID).Updates(map[string]any{
		"balance": 2000, "traffic_limit": 1000, "traffic_used": 0, "expires_at": newExpiry, "traffic_reset_at": resetAt,
		"current_plan_id": 7, "current_plan_price": 3000, "current_plan_duration": 30, "current_plan_type": PlanTypeMonthly,
	}).Error; err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", user.ID).
		Update("traffic_used", gorm.Expr("traffic_used + ?", 42)).Error; err != nil {
		t.Fatalf("record traffic: %v", err)
	}

	ip := "203.0.113.9"
	stale.LastLoginIP = ip
	stale.DisplayName = "Buyer"
	stale.Enabled = false
	if err := repo.Update(ctx, stale); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.LastLoginIP != ip || got.DisplayName != "Buyer" || got.Enabled {
		t.Fatalf("profile columns not saved: ip=%q name=%q enabled=%v", got.LastLoginIP, got.DisplayName, got.Enabled)
	}
	if got.Balance != 2000 {
		t.Errorf("balance = %d, want 2000 (the wallet debit was undone)", got.Balance)
	}
	if got.TrafficUsed != 42 {
		t.Errorf("traffic_used = %d, want 42 (reported traffic was lost)", got.TrafficUsed)
	}
	if got.TrafficLimit != 1000 || got.CurrentPlanID != 7 || got.CurrentPlanPrice != 3000 ||
		got.CurrentPlanDuration != 30 || got.CurrentPlanType != PlanTypeMonthly {
		t.Errorf("purchased plan was reverted: limit=%d plan=%d price=%d duration=%d type=%q",
			got.TrafficLimit, got.CurrentPlanID, got.CurrentPlanPrice, got.CurrentPlanDuration, got.CurrentPlanType)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(newExpiry) {
		t.Errorf("expires_at = %v, want %v (purchased days were lost)", got.ExpiresAt, newExpiry)
	}
	if got.TrafficResetAt == nil || !got.TrafficResetAt.Equal(resetAt) {
		t.Errorf("traffic_reset_at = %v, want %v", got.TrafficResetAt, resetAt)
	}
}

// A stale save used to fall back to INSERT and bring a deleted account back.
func TestUserRepository_UpdateDoesNotResurrectDeletedUser(t *testing.T) {
	db := setupTestDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()

	user := &User{Username: "gone", PasswordHash: "hash", Role: "user", Enabled: true}
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("create: %v", err)
	}
	stale, err := repo.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := repo.Delete(ctx, user.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	stale.LastLoginIP = "203.0.113.9"
	if err := repo.Update(ctx, stale); !errors.IsNotFound(err) {
		t.Fatalf("update of deleted user: err = %v, want not found", err)
	}
	if _, err := repo.GetByID(ctx, user.ID); !errors.IsNotFound(err) {
		t.Fatalf("deleted user exists again: err = %v", err)
	}
}
