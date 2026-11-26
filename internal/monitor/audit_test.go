package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Feature: project-optimization, Property 25: Audit Logging
// *For any* sensitive operation (user creation, deletion, password change, role modification),
// an audit log entry SHALL be created with timestamp, user, action, and affected resource.
// **Validates: Requirements 1.5**

func setupAuditTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(&repository.AuditLog{})
	require.NoError(t, err)

	return db
}

func TestAuditLogging_SensitiveOperations(t *testing.T) {
	sensitiveActions := []AuditAction{
		ActionLogin,
		ActionLogout,
		ActionPasswordChange,
		ActionPasswordReset,
		ActionUserCreate,
		ActionUserUpdate,
		ActionUserDelete,
		ActionUserEnable,
		ActionUserDisable,
		ActionProxyCreate,
		ActionProxyUpdate,
		ActionProxyDelete,
		ActionRoleCreate,
		ActionRoleUpdate,
		ActionRoleDelete,
		ActionSettingsUpdate,
		ActionXrayRestart,
		ActionXrayConfig,
	}

	for _, action := range sensitiveActions {
		t.Run(string(action), func(t *testing.T) {
			db := setupAuditTestDB(t)
			repo := repository.NewAuditLogRepository(db)
			service := NewAuditService(repo, logger.NewNopLogger())
			ctx := context.Background()

			userID := int64(123)
			entry := &AuditEntry{
				UserID:       &userID,
				Username:     "testuser",
				Action:       action,
				ResourceType: ResourceUser,
				ResourceID:   "456",
				Details:      map[string]any{"key": "value"},
				IPAddress:    "192.168.1.1",
				UserAgent:    "test-agent",
				RequestID:    "req-123",
				Status:       StatusSuccess,
			}

			err := service.Log(ctx, entry)
			require.NoError(t, err)

			// Verify the log was created
			logs, err := repo.List(ctx, 10, 0)
			require.NoError(t, err)
			require.Len(t, logs, 1)

			log := logs[0]
			assert.Equal(t, string(action), log.Action)
			assert.Equal(t, "testuser", log.Username)
			assert.Equal(t, string(ResourceUser), log.ResourceType)
			assert.Equal(t, "456", log.ResourceID)
			assert.Equal(t, "192.168.1.1", log.IPAddress)
			assert.Equal(t, "req-123", log.RequestID)
			assert.Equal(t, "success", log.Status)
			assert.NotZero(t, log.CreatedAt)
		})
	}
}

func TestAuditLogging_RequiredFields(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	// Use predefined valid actions and resource types
	validActions := []string{"login", "logout", "user_create", "user_delete", "proxy_create"}
	validResourceTypes := []string{"user", "proxy", "role", "settings", "system"}

	properties.Property("audit logs contain required fields", prop.ForAll(
		func(actionIdx int, resourceTypeIdx int, resourceID string, username string) bool {
			action := validActions[actionIdx%len(validActions)]
			resourceType := validResourceTypes[resourceTypeIdx%len(validResourceTypes)]

			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				return false
			}
			db.AutoMigrate(&repository.AuditLog{})

			repo := repository.NewAuditLogRepository(db)
			service := NewAuditService(repo, logger.NewNopLogger())
			ctx := context.Background()

			entry := &AuditEntry{
				Username:     username,
				Action:       AuditAction(action),
				ResourceType: AuditResourceType(resourceType),
				ResourceID:   resourceID,
			}

			err = service.Log(ctx, entry)
			if err != nil {
				return false
			}

			logs, err := repo.List(ctx, 10, 0)
			if err != nil || len(logs) != 1 {
				return false
			}

			log := logs[0]
			// Verify required fields
			if log.Action != action {
				return false
			}
			if log.ResourceType != resourceType {
				return false
			}
			if log.CreatedAt.IsZero() {
				return false
			}

			return true
		},
		gen.IntRange(0, 100),
		gen.IntRange(0, 100),
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestAuditLogging_SuccessAndFailure(t *testing.T) {
	db := setupAuditTestDB(t)
	repo := repository.NewAuditLogRepository(db)
	service := NewAuditService(repo, logger.NewNopLogger())
	ctx := context.Background()

	// Test LogSuccess
	successEntry := &AuditEntry{
		Action:       ActionUserCreate,
		ResourceType: ResourceUser,
		ResourceID:   "1",
	}
	err := service.LogSuccess(ctx, successEntry)
	require.NoError(t, err)

	// Test LogFailure
	failureEntry := &AuditEntry{
		Action:       ActionUserDelete,
		ResourceType: ResourceUser,
		ResourceID:   "2",
	}
	err = service.LogFailure(ctx, failureEntry)
	require.NoError(t, err)

	// Verify both logs
	logs, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	require.Len(t, logs, 2)

	// Logs are ordered by created_at DESC
	assert.Equal(t, "failure", logs[0].Status)
	assert.Equal(t, "success", logs[1].Status)
}

func TestAuditLogging_QueryByAction(t *testing.T) {
	db := setupAuditTestDB(t)
	repo := repository.NewAuditLogRepository(db)
	service := NewAuditService(repo, logger.NewNopLogger())
	ctx := context.Background()

	// Create logs with different actions
	actions := []AuditAction{ActionUserCreate, ActionUserCreate, ActionUserDelete, ActionProxyCreate}
	for _, action := range actions {
		entry := &AuditEntry{
			Action:       action,
			ResourceType: ResourceUser,
		}
		service.Log(ctx, entry)
	}

	// Query by action
	logs, err := repo.GetByAction(ctx, string(ActionUserCreate), 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	for _, log := range logs {
		assert.Equal(t, string(ActionUserCreate), log.Action)
	}
}

func TestAuditLogging_QueryByDateRange(t *testing.T) {
	db := setupAuditTestDB(t)
	repo := repository.NewAuditLogRepository(db)
	ctx := context.Background()

	now := time.Now()

	// Create logs directly with different timestamps
	logs := []*repository.AuditLog{
		{Action: "action1", ResourceType: "user", CreatedAt: now.Add(-2 * time.Hour)},
		{Action: "action2", ResourceType: "user", CreatedAt: now.Add(-1 * time.Hour)},
		{Action: "action3", ResourceType: "user", CreatedAt: now},
	}

	for _, log := range logs {
		err := repo.Create(ctx, log)
		require.NoError(t, err)
	}

	// Query last hour
	start := now.Add(-90 * time.Minute)
	end := now.Add(time.Minute)
	result, err := repo.GetByDateRange(ctx, start, end, 10, 0)
	require.NoError(t, err)
	assert.Len(t, result, 2) // action2 and action3
}

func TestAuditLogging_DeleteOldLogs(t *testing.T) {
	db := setupAuditTestDB(t)
	repo := repository.NewAuditLogRepository(db)
	ctx := context.Background()

	now := time.Now()

	// Create old and new logs
	oldLog := &repository.AuditLog{
		Action:       "old_action",
		ResourceType: "user",
		CreatedAt:    now.Add(-30 * 24 * time.Hour), // 30 days ago
	}
	newLog := &repository.AuditLog{
		Action:       "new_action",
		ResourceType: "user",
		CreatedAt:    now,
	}

	repo.Create(ctx, oldLog)
	repo.Create(ctx, newLog)

	// Delete logs older than 7 days
	deleted, err := repo.DeleteOlderThan(ctx, now.Add(-7*24*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify only new log remains
	logs, err := repo.List(ctx, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 1)
	assert.Equal(t, "new_action", logs[0].Action)
}

func TestAuditLogging_NilEntry(t *testing.T) {
	db := setupAuditTestDB(t)
	repo := repository.NewAuditLogRepository(db)
	service := NewAuditService(repo, logger.NewNopLogger())
	ctx := context.Background()

	// Should not panic or error on nil entry
	err := service.Log(ctx, nil)
	assert.NoError(t, err)

	// Verify no logs created
	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
