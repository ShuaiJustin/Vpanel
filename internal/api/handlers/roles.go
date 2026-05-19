// Package handlers provides HTTP request handlers for the API.
package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"v/internal/api/middleware"
	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/monitor"
	"v/pkg/errors"
)

// RoleResponse represents a role in API responses.
type RoleResponse struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
	UserCount   int64    `json:"user_count"`
	IsSystem    bool     `json:"is_system"`
}

// RoleHandler handles role-related requests.
type RoleHandler struct {
	logger       logger.Logger
	roleRepo     repository.RoleRepository
	auditService monitor.AuditService
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(log logger.Logger, roleRepo repository.RoleRepository) *RoleHandler {
	return &RoleHandler{
		logger:   log,
		roleRepo: roleRepo,
	}
}

// WithAuditService wires the audit emitter for state-changing role ops.
func (h *RoleHandler) WithAuditService(audit monitor.AuditService) *RoleHandler {
	h.auditService = audit
	return h
}

// InitSystemRoles initializes system roles in the database.
func (h *RoleHandler) InitSystemRoles(ctx context.Context) error {
	return h.roleRepo.EnsureSystemRoles(ctx)
}

// toRoleResponse converts a repository Role to RoleResponse.
func (h *RoleHandler) toRoleResponse(ctx context.Context, role *repository.Role) (*RoleResponse, error) {
	perms, err := role.GetPermissionsList()
	if err != nil {
		return nil, err
	}

	userCount, err := h.roleRepo.GetUserCount(ctx, role.Name)
	if err != nil {
		return nil, err
	}

	return &RoleResponse{
		ID:          role.ID,
		Name:        role.Name,
		Description: role.Description,
		Permissions: perms,
		UserCount:   userCount,
		IsSystem:    role.IsSystem,
	}, nil
}

// ListRoles returns all roles.
func (h *RoleHandler) ListRoles(c *gin.Context) {
	ctx := c.Request.Context()

	roles, err := h.roleRepo.List(ctx)
	if err != nil {
		h.logger.Error("Failed to list roles", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("list roles", err))
		return
	}

	responses := make([]*RoleResponse, 0, len(roles))
	for _, role := range roles {
		resp, err := h.toRoleResponse(ctx, role)
		if err != nil {
			h.logger.Error("Failed to convert role", logger.F("error", err))
			continue
		}
		responses = append(responses, resp)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    responses,
	})
}

// GetRole returns a specific role.
func (h *RoleHandler) GetRole(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid role id", nil))
		return
	}

	role, err := h.roleRepo.GetByID(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get role", err))
		return
	}

	if role == nil {
		middleware.RespondWithError(c, errors.NewNotFoundError("role", id))
		return
	}

	resp, err := h.toRoleResponse(ctx, role)
	if err != nil {
		h.logger.Error("Failed to convert role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("failed to convert role", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    resp,
	})
}

// CreateRoleRequest represents a create role request.
type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// ValidPermissions contains all valid permission keys.
var ValidPermissions = map[string]bool{
	"*":             true,
	"proxy:read":    true,
	"proxy:write":   true,
	"user:read":     true,
	"user:write":    true,
	"role:read":     true,
	"role:write":    true,
	"stats:read":    true,
	"system:read":   true,
	"system:write":  true,
	"profile:read":  true,
	"profile:write": true,
}

// validatePermissions checks if all permissions are valid.
func validatePermissions(perms []string) []string {
	var invalid []string
	for _, p := range perms {
		if !ValidPermissions[p] {
			invalid = append(invalid, p)
		}
	}
	return invalid
}

// CreateRole creates a new role.
func (h *RoleHandler) CreateRole(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	// Validate permissions
	if invalidPerms := validatePermissions(req.Permissions); len(invalidPerms) > 0 {
		middleware.RespondWithError(c, errors.NewValidationError("invalid permissions", map[string]interface{}{
			"invalid_permissions": invalidPerms,
		}))
		return
	}

	// Check if role name already exists
	existing, err := h.roleRepo.GetByName(ctx, req.Name)
	if err != nil {
		h.logger.Error("Failed to check role name", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("check role name", err))
		return
	}
	if existing != nil {
		middleware.RespondWithError(c, errors.NewConflictError("role", "name", req.Name))
		return
	}

	role := &repository.Role{
		Name:        req.Name,
		Description: req.Description,
		IsSystem:    false,
	}
	if err := role.SetPermissionsList(req.Permissions); err != nil {
		h.logger.Error("Failed to set permissions", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("failed to set permissions", err))
		return
	}

	if err := h.roleRepo.Create(ctx, role); err != nil {
		h.logger.Error("Failed to create role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("create role", err))
		return
	}

	resp, err := h.toRoleResponse(ctx, role)
	if err != nil {
		h.logger.Error("Failed to convert role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("failed to convert role", err))
		return
	}

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionRoleCreate,
		ResourceType: monitor.ResourceRole,
		ResourceID:   strconv.FormatInt(role.ID, 10),
		Details:      map[string]any{"name": role.Name},
	})

	c.JSON(http.StatusCreated, gin.H{
		"code":    201,
		"message": "role created",
		"data":    resp,
	})
}

// UpdateRoleRequest represents an update role request.
type UpdateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

// UpdateRole updates an existing role.
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid role id", nil))
		return
	}

	role, err := h.roleRepo.GetByID(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get role", err))
		return
	}

	if role == nil {
		middleware.RespondWithError(c, errors.NewNotFoundError("role", id))
		return
	}

	// Prevent modification of system roles
	if role.IsSystem {
		middleware.RespondWithError(c, errors.NewForbiddenError("cannot modify system role"))
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid request", map[string]interface{}{
			"error": err.Error(),
		}))
		return
	}

	// Validate permissions if provided
	if req.Permissions != nil {
		if invalidPerms := validatePermissions(req.Permissions); len(invalidPerms) > 0 {
			middleware.RespondWithError(c, errors.NewValidationError("invalid permissions", map[string]interface{}{
				"invalid_permissions": invalidPerms,
			}))
			return
		}
	}

	// Check name uniqueness if changing
	if req.Name != "" && req.Name != role.Name {
		existing, err := h.roleRepo.GetByName(ctx, req.Name)
		if err != nil {
			h.logger.Error("Failed to check role name", logger.F("error", err))
			middleware.RespondWithError(c, errors.NewDatabaseError("check role name", err))
			return
		}
		if existing != nil {
			middleware.RespondWithError(c, errors.NewConflictError("role", "name", req.Name))
			return
		}
		role.Name = req.Name
	}

	if req.Description != "" {
		role.Description = req.Description
	}

	if req.Permissions != nil {
		if err := role.SetPermissionsList(req.Permissions); err != nil {
			h.logger.Error("Failed to set permissions", logger.F("error", err))
			middleware.RespondWithError(c, errors.NewInternalError("failed to set permissions", err))
			return
		}
	}

	if err := h.roleRepo.Update(ctx, role); err != nil {
		h.logger.Error("Failed to update role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("update role", err))
		return
	}

	resp, err := h.toRoleResponse(ctx, role)
	if err != nil {
		h.logger.Error("Failed to convert role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewInternalError("failed to convert role", err))
		return
	}

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionRoleUpdate,
		ResourceType: monitor.ResourceRole,
		ResourceID:   strconv.FormatInt(role.ID, 10),
		Details:      map[string]any{"name": role.Name},
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "role updated",
		"data":    resp,
	})
}

// DeleteRole deletes a role.
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	ctx := c.Request.Context()

	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.RespondWithError(c, errors.NewValidationError("invalid role id", nil))
		return
	}

	role, err := h.roleRepo.GetByID(ctx, id)
	if err != nil {
		h.logger.Error("Failed to get role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get role", err))
		return
	}

	if role == nil {
		middleware.RespondWithError(c, errors.NewNotFoundError("role", id))
		return
	}

	// Prevent deletion of system roles
	if role.IsSystem {
		middleware.RespondWithError(c, errors.NewForbiddenError("cannot delete system role"))
		return
	}

	// Reassign users to default role before deletion
	userCount, err := h.roleRepo.GetUserCount(ctx, role.Name)
	if err != nil {
		h.logger.Error("Failed to get user count", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("get user count", err))
		return
	}

	if userCount > 0 {
		if err := h.roleRepo.ReassignUsersToDefaultRole(ctx, role.Name); err != nil {
			h.logger.Error("Failed to reassign users", logger.F("error", err))
			middleware.RespondWithError(c, errors.NewDatabaseError("reassign users", err))
			return
		}
		h.logger.Info("Reassigned users to default role", logger.F("count", userCount), logger.F("from_role", role.Name))
	}

	if err := h.roleRepo.Delete(ctx, id); err != nil {
		h.logger.Error("Failed to delete role", logger.F("error", err))
		middleware.RespondWithError(c, errors.NewDatabaseError("delete role", err))
		return
	}

	emitAudit(c, h.auditService, monitor.AuditEntry{
		Action:       monitor.ActionRoleDelete,
		ResourceType: monitor.ResourceRole,
		ResourceID:   strconv.FormatInt(id, 10),
		Details:      map[string]any{"name": role.Name, "reassigned_users": userCount},
	})

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "role deleted",
		"data": gin.H{
			"reassigned_users": userCount,
		},
	})
}

// GetPermissions returns all available permissions.
func (h *RoleHandler) GetPermissions(c *gin.Context) {
	permissions := []map[string]string{
		{"key": "*", "name": "所有权限", "description": "拥有系统所有权限"},
		{"key": "proxy:read", "name": "查看代理", "description": "查看代理配置"},
		{"key": "proxy:write", "name": "管理代理", "description": "创建、修改、删除代理"},
		{"key": "user:read", "name": "查看用户", "description": "查看用户列表"},
		{"key": "user:write", "name": "管理用户", "description": "创建、修改、删除用户"},
		{"key": "role:read", "name": "查看角色", "description": "查看角色列表"},
		{"key": "role:write", "name": "管理角色", "description": "创建、修改、删除角色"},
		{"key": "stats:read", "name": "查看统计", "description": "查看系统统计数据"},
		{"key": "system:read", "name": "查看系统", "description": "查看系统信息"},
		{"key": "system:write", "name": "管理系统", "description": "修改系统设置"},
		{"key": "profile:read", "name": "查看个人信息", "description": "查看个人资料"},
		{"key": "profile:write", "name": "修改个人信息", "description": "修改个人资料"},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "success",
		"data":    permissions,
	})
}

// HasPermission checks if a role has a specific permission.
func (h *RoleHandler) HasPermission(ctx context.Context, roleName, permission string) (bool, error) {
	role, err := h.roleRepo.GetByName(ctx, roleName)
	if err != nil {
		return false, err
	}
	if role == nil {
		return false, nil
	}

	perms, err := role.GetPermissionsList()
	if err != nil {
		return false, err
	}

	// Admin role with "*" has all permissions
	for _, p := range perms {
		if p == "*" || p == permission {
			return true, nil
		}
	}

	return false, nil
}
