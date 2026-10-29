// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// UserNodeAssignment represents the assignment of a user to a specific node.
type UserNodeAssignment struct {
	ID         int64     `gorm:"primaryKey;autoIncrement"`
	UserID     int64     `gorm:"uniqueIndex;not null"`
	NodeID     int64     `gorm:"index;not null"`
	AssignedAt time.Time `gorm:""`
	UpdatedAt  time.Time `gorm:"autoUpdateTime"`

	User *User `gorm:"foreignKey:UserID"`
	Node *Node `gorm:"foreignKey:NodeID"`
}

// TableName returns the table name for UserNodeAssignment.
func (UserNodeAssignment) TableName() string {
	return "user_node_assignments"
}

// UserNodeAssignmentRepository defines the interface for user-node assignment data access.
type UserNodeAssignmentRepository interface {
	// CRUD operations
	Create(ctx context.Context, assignment *UserNodeAssignment) error
	GetByID(ctx context.Context, id int64) (*UserNodeAssignment, error)
	Update(ctx context.Context, assignment *UserNodeAssignment) error
	Delete(ctx context.Context, id int64) error

	// Query operations
	GetByUserID(ctx context.Context, userID int64) (*UserNodeAssignment, error)
	GetByNodeID(ctx context.Context, nodeID int64) ([]*UserNodeAssignment, error)
	GetUserIDsByNodeID(ctx context.Context, nodeID int64) ([]int64, error)
	CountByNodeID(ctx context.Context, nodeID int64) (int64, error)

	// Assignment operations
	Assign(ctx context.Context, userID, nodeID int64) error
	Reassign(ctx context.Context, userID, newNodeID int64) error
	Unassign(ctx context.Context, userID int64) error
	BulkReassign(ctx context.Context, userIDs []int64, newNodeID int64) error

	// Batch operations
	DeleteByNodeID(ctx context.Context, nodeID int64) error
	GetUnassignedUsers(ctx context.Context, limit int) ([]int64, error)

	// Transaction operations
	ReassignInTx(ctx context.Context, userID, newNodeID int64) error
}

// userNodeAssignmentRepository implements UserNodeAssignmentRepository.
type userNodeAssignmentRepository struct {
	db *gorm.DB
}

// NewUserNodeAssignmentRepository creates a new user-node assignment repository.
func NewUserNodeAssignmentRepository(db *gorm.DB) UserNodeAssignmentRepository {
	return &userNodeAssignmentRepository{db: db}
}

// Create creates a new user-node assignment.
func (r *userNodeAssignmentRepository) Create(ctx context.Context, assignment *UserNodeAssignment) error {
	result := r.db.WithContext(ctx).Create(assignment)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create user-node assignment", result.Error)
	}
	return nil
}

// GetByID retrieves a user-node assignment by ID.
func (r *userNodeAssignmentRepository) GetByID(ctx context.Context, id int64) (*UserNodeAssignment, error) {
	var assignment UserNodeAssignment
	result := r.db.WithContext(ctx).First(&assignment, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("user-node assignment", id)
		}
		return nil, errors.NewDatabaseError("failed to get user-node assignment", result.Error)
	}
	return &assignment, nil
}

// Update updates a user-node assignment.
func (r *userNodeAssignmentRepository) Update(ctx context.Context, assignment *UserNodeAssignment) error {
	result := r.db.WithContext(ctx).Save(assignment)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update user-node assignment", result.Error)
	}
	return nil
}

// Delete deletes a user-node assignment by ID.
func (r *userNodeAssignmentRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&UserNodeAssignment{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete user-node assignment", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("user-node assignment", id)
	}
	return nil
}

// GetByUserID retrieves the assignment for a user.
func (r *userNodeAssignmentRepository) GetByUserID(ctx context.Context, userID int64) (*UserNodeAssignment, error) {
	var assignment UserNodeAssignment
	result := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&assignment)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, nil // No assignment
		}
		return nil, errors.NewDatabaseError("failed to get assignment by user ID", result.Error)
	}
	return &assignment, nil
}

// GetByNodeID retrieves all assignments for a node.
func (r *userNodeAssignmentRepository) GetByNodeID(ctx context.Context, nodeID int64) ([]*UserNodeAssignment, error) {
	var assignments []*UserNodeAssignment
	result := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Find(&assignments)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get assignments by node ID", result.Error)
	}
	return assignments, nil
}

// GetUserIDsByNodeID retrieves all user IDs assigned to a node.
func (r *userNodeAssignmentRepository) GetUserIDsByNodeID(ctx context.Context, nodeID int64) ([]int64, error) {
	var userIDs []int64
	result := r.db.WithContext(ctx).
		Model(&UserNodeAssignment{}).
		Where("node_id = ?", nodeID).
		Pluck("user_id", &userIDs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get user IDs by node ID", result.Error)
	}
	return userIDs, nil
}

// CountByNodeID counts the number of users assigned to a node.
func (r *userNodeAssignmentRepository) CountByNodeID(ctx context.Context, nodeID int64) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&UserNodeAssignment{}).
		Where("node_id = ?", nodeID).
		Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count assignments by node ID", result.Error)
	}
	return count, nil
}

// Assign assigns a user to a node (creates or updates assignment).
func (r *userNodeAssignmentRepository) Assign(ctx context.Context, userID, nodeID int64) error {
	now := time.Now()
	assignment := &UserNodeAssignment{
		UserID:     userID,
		NodeID:     nodeID,
		AssignedAt: now,
		UpdatedAt:  now,
	}

	// Use upsert to handle existing assignments
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Assign(map[string]interface{}{
			"node_id":    nodeID,
			"updated_at": now,
		}).
		FirstOrCreate(assignment)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to assign user to node", result.Error)
	}
	return nil
}

// Reassign reassigns a user to a different node.
func (r *userNodeAssignmentRepository) Reassign(ctx context.Context, userID, newNodeID int64) error {
	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&UserNodeAssignment{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"node_id":    newNodeID,
			"updated_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to reassign user", result.Error)
	}
	if result.RowsAffected == 0 {
		// No existing assignment, create one
		return r.Assign(ctx, userID, newNodeID)
	}
	return nil
}

// Unassign removes a user's node assignment.
func (r *userNodeAssignmentRepository) Unassign(ctx context.Context, userID int64) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Delete(&UserNodeAssignment{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to unassign user", result.Error)
	}
	return nil
}

// BulkReassign reassigns multiple users to a new node.
func (r *userNodeAssignmentRepository) BulkReassign(ctx context.Context, userIDs []int64, newNodeID int64) error {
	if len(userIDs) == 0 {
		return nil
	}

	now := time.Now()
	result := r.db.WithContext(ctx).
		Model(&UserNodeAssignment{}).
		Where("user_id IN ?", userIDs).
		Updates(map[string]interface{}{
			"node_id":    newNodeID,
			"updated_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to bulk reassign users", result.Error)
	}
	return nil
}

// DeleteByNodeID deletes all assignments for a node.
func (r *userNodeAssignmentRepository) DeleteByNodeID(ctx context.Context, nodeID int64) error {
	result := r.getDB(ctx).
		Where("node_id = ?", nodeID).
		Delete(&UserNodeAssignment{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete assignments by node ID", result.Error)
	}
	return nil
}

// GetUnassignedUsers retrieves user IDs that don't have a node assignment.
func (r *userNodeAssignmentRepository) GetUnassignedUsers(ctx context.Context, limit int) ([]int64, error) {
	var userIDs []int64
	query := r.db.WithContext(ctx).
		Table("users").
		Select("users.id").
		Joins("LEFT JOIN user_node_assignments ON users.id = user_node_assignments.user_id").
		Where("user_node_assignments.id IS NULL").
		Where("users.enabled = ?", true)
	if limit > 0 {
		query = query.Limit(limit)
	}
	result := query.Pluck("users.id", &userIDs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get unassigned users", result.Error)
	}
	return userIDs, nil
}

// ReassignInTx reassigns a user to a different node within a transaction.
func (r *userNodeAssignmentRepository) ReassignInTx(ctx context.Context, userID, newNodeID int64) error {
	db := r.getDB(ctx)
	now := time.Now()
	result := db.
		Model(&UserNodeAssignment{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"node_id":    newNodeID,
			"updated_at": now,
		})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to reassign user in transaction", result.Error)
	}
	if result.RowsAffected == 0 {
		// No existing assignment, create one
		assignment := &UserNodeAssignment{
			UserID:     userID,
			NodeID:     newNodeID,
			AssignedAt: now,
			UpdatedAt:  now,
		}
		if err := db.Create(assignment).Error; err != nil {
			return errors.NewDatabaseError("failed to create assignment in transaction", err)
		}
	}
	return nil
}

// getDB returns the appropriate database connection (transaction or regular).
func (r *userNodeAssignmentRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok {
		return tx
	}
	return r.db.WithContext(ctx)
}
