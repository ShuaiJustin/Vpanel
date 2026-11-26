// Package repository provides data access interfaces and implementations.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"gorm.io/gorm"
)

// Role represents a role in the database.
type Role struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"uniqueIndex;size:50;not null"`
	Description string    `gorm:"size:255"`
	Permissions string    `gorm:"type:text"` // JSON array stored as string
	IsSystem    bool      `gorm:"default:false;index"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for Role.
func (Role) TableName() string {
	return "roles"
}

// GetPermissionsList parses the permissions JSON string into a slice.
func (r *Role) GetPermissionsList() ([]string, error) {
	if r.Permissions == "" {
		return []string{}, nil
	}
	var perms []string
	err := json.Unmarshal([]byte(r.Permissions), &perms)
	return perms, err
}

// SetPermissionsList converts a slice to JSON string for storage.
func (r *Role) SetPermissionsList(perms []string) error {
	if perms == nil {
		perms = []string{}
	}
	data, err := json.Marshal(perms)
	if err != nil {
		return err
	}
	r.Permissions = string(data)
	return nil
}

// RoleRepository defines the interface for role data access.
type RoleRepository interface {
	Create(ctx context.Context, role *Role) error
	GetByID(ctx context.Context, id int64) (*Role, error)
	GetByName(ctx context.Context, name string) (*Role, error)
	Update(ctx context.Context, role *Role) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]*Role, error)
	GetUserCount(ctx context.Context, roleName string) (int64, error)
	EnsureSystemRoles(ctx context.Context) error
	ReassignUsersToDefaultRole(ctx context.Context, fromRoleName string) error
}

// roleRepository implements RoleRepository using GORM.
type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository creates a new role repository.
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

// Create creates a new role.
func (r *roleRepository) Create(ctx context.Context, role *Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// GetByID retrieves a role by ID.
func (r *roleRepository) GetByID(ctx context.Context, id int64) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).First(&role, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// GetByName retrieves a role by name.
func (r *roleRepository) GetByName(ctx context.Context, name string) (*Role, error) {
	var role Role
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &role, nil
}

// Update updates an existing role.
func (r *roleRepository) Update(ctx context.Context, role *Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete deletes a role by ID.
func (r *roleRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&Role{}, id).Error
}

// List retrieves all roles.
func (r *roleRepository) List(ctx context.Context) ([]*Role, error) {
	var roles []*Role
	err := r.db.WithContext(ctx).Order("id ASC").Find(&roles).Error
	return roles, err
}

// GetUserCount returns the number of users with a specific role.
func (r *roleRepository) GetUserCount(ctx context.Context, roleName string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&User{}).Where("role = ?", roleName).Count(&count).Error
	return count, err
}

// EnsureSystemRoles ensures that default system roles exist.
func (r *roleRepository) EnsureSystemRoles(ctx context.Context) error {
	systemRoles := []struct {
		Name        string
		Description string
		Permissions []string
	}{
		{
			Name:        "admin",
			Description: "系统管理员，拥有所有权限",
			Permissions: []string{"*"},
		},
		{
			Name:        "user",
			Description: "普通用户，通过用户门户使用服务",
			// No proxy:* here. /api/proxies is the admin inbound API: it accepts any
			// node, port and settings and makes the node re-sync, so a customer
			// holding proxy:write could open inbounds on every node.
			Permissions: []string{"profile:read", "profile:write"},
		},
		{
			Name:        "viewer",
			Description: "只读用户，只能查看信息",
			Permissions: []string{"proxy:read", "profile:read", "stats:read"},
		},
	}

	for _, sr := range systemRoles {
		existing, err := r.GetByName(ctx, sr.Name)
		if err != nil {
			return err
		}
		if existing == nil {
			role := &Role{
				Name:        sr.Name,
				Description: sr.Description,
				IsSystem:    true,
			}
			if err := role.SetPermissionsList(sr.Permissions); err != nil {
				return err
			}
			if err := r.Create(ctx, role); err != nil {
				return err
			}
			continue
		}
		// System roles cannot be edited through the API, so the definition above
		// is their only source: bring rows seeded by older versions in line.
		current, _ := existing.GetPermissionsList()
		if !existing.IsSystem || slices.Equal(current, sr.Permissions) {
			continue
		}
		if err := existing.SetPermissionsList(sr.Permissions); err != nil {
			return err
		}
		if err := r.Update(ctx, existing); err != nil {
			return err
		}
	}
	return nil
}

// ReassignUsersToDefaultRole reassigns all users from a role to the default "user" role.
func (r *roleRepository) ReassignUsersToDefaultRole(ctx context.Context, fromRoleName string) error {
	return r.db.WithContext(ctx).Model(&User{}).
		Where("role = ?", fromRoleName).
		Update("role", "user").Error
}
