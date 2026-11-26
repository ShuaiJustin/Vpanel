// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"v/internal/database/repository"
	"v/internal/logger"
)

// mockRoleRepository is a mock implementation of RoleRepository for testing.
type mockRoleRepository struct {
	roles      map[int64]*repository.Role
	userCounts map[string]int64
	nextID     int64
}

func newMockRoleRepository() *mockRoleRepository {
	return &mockRoleRepository{
		roles:      make(map[int64]*repository.Role),
		userCounts: make(map[string]int64),
		nextID:     1,
	}
}

func (m *mockRoleRepository) Create(ctx context.Context, role *repository.Role) error {
	role.ID = m.nextID
	m.nextID++
	m.roles[role.ID] = role
	return nil
}

func (m *mockRoleRepository) GetByID(ctx context.Context, id int64) (*repository.Role, error) {
	role, ok := m.roles[id]
	if !ok {
		return nil, nil
	}
	return role, nil
}

func (m *mockRoleRepository) GetByName(ctx context.Context, name string) (*repository.Role, error) {
	for _, role := range m.roles {
		if role.Name == name {
			return role, nil
		}
	}
	return nil, nil
}

func (m *mockRoleRepository) Update(ctx context.Context, role *repository.Role) error {
	m.roles[role.ID] = role
	return nil
}

func (m *mockRoleRepository) Delete(ctx context.Context, id int64) error {
	delete(m.roles, id)
	return nil
}

func (m *mockRoleRepository) List(ctx context.Context) ([]*repository.Role, error) {
	roles := make([]*repository.Role, 0, len(m.roles))
	for _, role := range m.roles {
		roles = append(roles, role)
	}
	return roles, nil
}

func (m *mockRoleRepository) GetUserCount(ctx context.Context, roleName string) (int64, error) {
	return m.userCounts[roleName], nil
}

func (m *mockRoleRepository) EnsureSystemRoles(ctx context.Context) error {
	systemRoles := []struct {
		Name        string
		Description string
		Permissions []string
		IsSystem    bool
	}{
		{"admin", "系统管理员", []string{"*"}, true},
		{"user", "普通用户", []string{"proxy:read", "proxy:write"}, true},
		{"viewer", "只读用户", []string{"proxy:read"}, true},
	}

	for _, sr := range systemRoles {
		existing, _ := m.GetByName(ctx, sr.Name)
		if existing == nil {
			role := &repository.Role{
				Name:        sr.Name,
				Description: sr.Description,
				IsSystem:    sr.IsSystem,
			}
			role.SetPermissionsList(sr.Permissions)
			m.Create(ctx, role)
		}
	}
	return nil
}

func (m *mockRoleRepository) ReassignUsersToDefaultRole(ctx context.Context, fromRoleName string) error {
	m.userCounts["user"] += m.userCounts[fromRoleName]
	m.userCounts[fromRoleName] = 0
	return nil
}

func (m *mockRoleRepository) setUserCount(roleName string, count int64) {
	m.userCounts[roleName] = count
}

func setupRoleTestRouter(repo *mockRoleRepository) (*gin.Engine, *RoleHandler) {
	gin.SetMode(gin.TestMode)
	log := logger.NewNopLogger()
	handler := NewRoleHandler(log, repo)

	router := gin.New()
	router.GET("/roles", handler.ListRoles)
	router.POST("/roles", handler.CreateRole)
	router.GET("/roles/:id", handler.GetRole)
	router.PUT("/roles/:id", handler.UpdateRole)
	router.DELETE("/roles/:id", handler.DeleteRole)

	return router, handler
}

// Feature: project-optimization, Property 17: System Role Protection
// Validates: Requirements 19.4, 19.5
// *For any* attempt to delete or modify permissions of a system role (admin, user, viewer),
// the operation SHALL be rejected with a forbidden error.
func TestSystemRoleProtection_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	systemRoleNames := []string{"admin", "user", "viewer"}

	properties.Property("System roles cannot be deleted", prop.ForAll(
		func(roleIndex int) bool {
			repo := newMockRoleRepository()
			router, _ := setupRoleTestRouter(repo)

			// Initialize system roles
			repo.EnsureSystemRoles(context.Background())

			// Get the system role
			roleName := systemRoleNames[roleIndex%len(systemRoleNames)]
			role, _ := repo.GetByName(context.Background(), roleName)
			if role == nil {
				return false
			}

			// Try to delete the system role
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/roles/%d", role.ID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should be forbidden
			return w.Code == http.StatusForbidden
		},
		gen.IntRange(0, 2),
	))

	properties.Property("System roles cannot be modified", prop.ForAll(
		func(roleIndex int, newName string, newPerms []string) bool {
			repo := newMockRoleRepository()
			router, _ := setupRoleTestRouter(repo)

			// Initialize system roles
			repo.EnsureSystemRoles(context.Background())

			// Get the system role
			roleName := systemRoleNames[roleIndex%len(systemRoleNames)]
			role, _ := repo.GetByName(context.Background(), roleName)
			if role == nil {
				return false
			}

			// Try to update the system role
			updateReq := UpdateRoleRequest{
				Name:        newName,
				Permissions: newPerms,
			}
			body, _ := json.Marshal(updateReq)
			req, _ := http.NewRequest("PUT", fmt.Sprintf("/roles/%d", role.ID), bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should be forbidden
			return w.Code == http.StatusForbidden
		},
		gen.IntRange(0, 2),
		gen.AlphaString(),
		gen.SliceOf(gen.OneConstOf("proxy:read", "proxy:write", "user:read")),
	))

	properties.TestingRun(t)
}

// Feature: project-optimization, Property 18: Role Deletion User Reassignment
// Validates: Requirements 19.6
// *For any* role deletion where users are assigned to that role,
// all affected users SHALL be reassigned to the default "user" role.
func TestRoleDeletionUserReassignment_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	properties.Property("Users are reassigned when role is deleted", prop.ForAll(
		func(roleName string, userCount int64) bool {
			if roleName == "" || roleName == "admin" || roleName == "user" || roleName == "viewer" {
				return true // Skip system role names
			}

			repo := newMockRoleRepository()
			router, _ := setupRoleTestRouter(repo)

			// Initialize system roles
			repo.EnsureSystemRoles(context.Background())

			// Create a custom role
			customRole := &repository.Role{
				Name:        roleName,
				Description: "Custom role",
				IsSystem:    false,
			}
			customRole.SetPermissionsList([]string{"proxy:read"})
			repo.Create(context.Background(), customRole)

			// Set user count for this role
			if userCount < 0 {
				userCount = 0
			}
			if userCount > 100 {
				userCount = 100
			}
			repo.setUserCount(roleName, userCount)
			initialDefaultUserCount := repo.userCounts["user"]

			// Delete the custom role
			req, _ := http.NewRequest("DELETE", fmt.Sprintf("/roles/%d", customRole.ID), nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed
			if w.Code != http.StatusOK {
				return false
			}

			// Users should be reassigned to default role
			return repo.userCounts["user"] == initialDefaultUserCount+userCount
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && s != "admin" && s != "user" && s != "viewer"
		}),
		gen.Int64Range(0, 50),
	))

	properties.TestingRun(t)
}

// Unit tests for specific edge cases

func TestSystemRoleProtection_DeleteAdmin(t *testing.T) {
	repo := newMockRoleRepository()
	router, _ := setupRoleTestRouter(repo)
	repo.EnsureSystemRoles(context.Background())

	adminRole, _ := repo.GetByName(context.Background(), "admin")
	require.NotNil(t, adminRole)

	req, _ := http.NewRequest("DELETE", "/roles/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestSystemRoleProtection_UpdateUser(t *testing.T) {
	repo := newMockRoleRepository()
	router, _ := setupRoleTestRouter(repo)
	repo.EnsureSystemRoles(context.Background())

	userRole, _ := repo.GetByName(context.Background(), "user")
	require.NotNil(t, userRole)

	updateReq := UpdateRoleRequest{
		Permissions: []string{"*"},
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/roles/2", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestCustomRoleCanBeDeleted(t *testing.T) {
	repo := newMockRoleRepository()
	router, _ := setupRoleTestRouter(repo)
	repo.EnsureSystemRoles(context.Background())

	// Create a custom role
	customRole := &repository.Role{
		Name:        "custom",
		Description: "Custom role",
		IsSystem:    false,
	}
	customRole.SetPermissionsList([]string{"proxy:read"})
	repo.Create(context.Background(), customRole)

	req, _ := http.NewRequest("DELETE", "/roles/4", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCustomRoleCanBeUpdated(t *testing.T) {
	repo := newMockRoleRepository()
	router, _ := setupRoleTestRouter(repo)
	repo.EnsureSystemRoles(context.Background())

	// Create a custom role
	customRole := &repository.Role{
		Name:        "custom",
		Description: "Custom role",
		IsSystem:    false,
	}
	customRole.SetPermissionsList([]string{"proxy:read"})
	repo.Create(context.Background(), customRole)

	updateReq := UpdateRoleRequest{
		Description: "Updated description",
		Permissions: []string{"proxy:read", "proxy:write"},
	}
	body, _ := json.Marshal(updateReq)
	req, _ := http.NewRequest("PUT", "/roles/4", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// Feature: project-optimization, Property 19: Permission Validation
// Validates: Requirements 19.7
// *For any* role creation or update with invalid permission keys,
// the operation SHALL be rejected with a validation error.
func TestPermissionValidation_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	validPerms := []string{"*", "proxy:read", "proxy:write", "user:read", "user:write",
		"role:read", "role:write", "stats:read", "system:read", "system:write",
		"profile:read", "profile:write"}

	properties.Property("Valid permissions are accepted", prop.ForAll(
		func(permIndices []int) bool {
			repo := newMockRoleRepository()
			router, _ := setupRoleTestRouter(repo)
			repo.EnsureSystemRoles(context.Background())

			// Build valid permissions list
			perms := make([]string, 0, len(permIndices))
			for _, idx := range permIndices {
				if idx >= 0 && idx < len(validPerms) {
					perms = append(perms, validPerms[idx])
				}
			}

			createReq := CreateRoleRequest{
				Name:        "testrole",
				Description: "Test role",
				Permissions: perms,
			}
			body, _ := json.Marshal(createReq)
			req, _ := http.NewRequest("POST", "/roles", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should succeed (201 Created)
			return w.Code == http.StatusCreated
		},
		gen.SliceOfN(3, gen.IntRange(0, len(validPerms)-1)),
	))

	properties.Property("Invalid permissions are rejected", prop.ForAll(
		func(invalidPerm string) bool {
			// Skip if the generated string happens to be a valid permission
			if ValidPermissions[invalidPerm] {
				return true
			}

			repo := newMockRoleRepository()
			router, _ := setupRoleTestRouter(repo)
			repo.EnsureSystemRoles(context.Background())

			createReq := CreateRoleRequest{
				Name:        "testrole",
				Description: "Test role",
				Permissions: []string{invalidPerm},
			}
			body, _ := json.Marshal(createReq)
			req, _ := http.NewRequest("POST", "/roles", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Should be rejected (400 Bad Request)
			return w.Code == http.StatusBadRequest
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && !ValidPermissions[s]
		}),
	))

	properties.TestingRun(t)
}

// Test permission inheritance - admin with "*" has all permissions
func TestPermissionInheritance(t *testing.T) {
	repo := newMockRoleRepository()
	log := logger.NewNopLogger()
	handler := NewRoleHandler(log, repo)
	repo.EnsureSystemRoles(context.Background())

	ctx := context.Background()

	// Admin should have all permissions
	hasProxy, _ := handler.HasPermission(ctx, "admin", "proxy:read")
	assert.True(t, hasProxy, "admin should have proxy:read")

	hasUser, _ := handler.HasPermission(ctx, "admin", "user:write")
	assert.True(t, hasUser, "admin should have user:write")

	hasSystem, _ := handler.HasPermission(ctx, "admin", "system:write")
	assert.True(t, hasSystem, "admin should have system:write")

	// User should only have specific permissions
	hasProxyUser, _ := handler.HasPermission(ctx, "user", "proxy:read")
	assert.True(t, hasProxyUser, "user should have proxy:read")

	hasUserWrite, _ := handler.HasPermission(ctx, "user", "user:write")
	assert.False(t, hasUserWrite, "user should not have user:write")

	// Viewer should only have read permissions
	hasProxyViewer, _ := handler.HasPermission(ctx, "viewer", "proxy:read")
	assert.True(t, hasProxyViewer, "viewer should have proxy:read")

	hasProxyWriteViewer, _ := handler.HasPermission(ctx, "viewer", "proxy:write")
	assert.False(t, hasProxyWriteViewer, "viewer should not have proxy:write")
}
