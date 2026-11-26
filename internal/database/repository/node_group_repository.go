// Package repository provides data access implementations.
package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"v/pkg/errors"
)

// NodeGroup represents a group of nodes for organization and load balancing.
type NodeGroup struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	Name        string    `gorm:"size:64;not null"`
	Description string    `gorm:"size:256"`
	Region      string    `gorm:"size:64"`
	Strategy    string    `gorm:"size:32;default:round-robin"` // Load balancing strategy
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName returns the table name for NodeGroup.
func (NodeGroup) TableName() string {
	return "node_groups"
}

// NodeGroupMember represents the many-to-many relationship between nodes and groups.
type NodeGroupMember struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	NodeID    int64     `gorm:"uniqueIndex:idx_node_group_member,priority:1;not null"`
	GroupID   int64     `gorm:"uniqueIndex:idx_node_group_member,priority:2;index;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`

	Node  *Node      `gorm:"foreignKey:NodeID"`
	Group *NodeGroup `gorm:"foreignKey:GroupID"`
}

// TableName returns the table name for NodeGroupMember.
func (NodeGroupMember) TableName() string {
	return "node_group_members"
}

// LoadBalanceStrategy constants
const (
	StrategyRoundRobin       = "round-robin"
	StrategyLeastConnections = "least-connections"
	StrategyWeighted         = "weighted"
	StrategyGeographic       = "geographic"
)

// NodeGroupStats represents aggregate statistics for a node group.
type NodeGroupStats struct {
	GroupID      int64
	TotalNodes   int64
	HealthyNodes int64
	TotalUsers   int64
}

// NodeGroupRepository defines the interface for node group data access.
type NodeGroupRepository interface {
	// CRUD operations
	Create(ctx context.Context, group *NodeGroup) error
	GetByID(ctx context.Context, id int64) (*NodeGroup, error)
	Update(ctx context.Context, group *NodeGroup) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, limit, offset int) ([]*NodeGroup, error)
	Count(ctx context.Context) (int64, error)

	// Member operations
	AddNode(ctx context.Context, groupID, nodeID int64) error
	RemoveNode(ctx context.Context, groupID, nodeID int64) error
	GetNodes(ctx context.Context, groupID int64) ([]*Node, error)
	GetNodeIDs(ctx context.Context, groupID int64) ([]int64, error)
	GetGroupsForNode(ctx context.Context, nodeID int64) ([]*NodeGroup, error)
	GetGroupIDsForNode(ctx context.Context, nodeID int64) ([]int64, error)
	IsNodeInGroup(ctx context.Context, groupID, nodeID int64) (bool, error)

	// Statistics
	GetStats(ctx context.Context, groupID int64) (*NodeGroupStats, error)
	GetAllStats(ctx context.Context) ([]*NodeGroupStats, error)

	// Query operations
	GetByRegion(ctx context.Context, region string) ([]*NodeGroup, error)
	GetByStrategy(ctx context.Context, strategy string) ([]*NodeGroup, error)

	// Bulk operations
	RemoveAllNodes(ctx context.Context, groupID int64) error
	SetNodes(ctx context.Context, groupID int64, nodeIDs []int64) error
}

// nodeGroupRepository implements NodeGroupRepository.
type nodeGroupRepository struct {
	db *gorm.DB
}

// NewNodeGroupRepository creates a new node group repository.
func NewNodeGroupRepository(db *gorm.DB) NodeGroupRepository {
	return &nodeGroupRepository{db: db}
}

// Create creates a new node group.
func (r *nodeGroupRepository) Create(ctx context.Context, group *NodeGroup) error {
	result := r.db.WithContext(ctx).Create(group)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to create node group", result.Error)
	}
	return nil
}

// GetByID retrieves a node group by ID.
func (r *nodeGroupRepository) GetByID(ctx context.Context, id int64) (*NodeGroup, error) {
	var group NodeGroup
	result := r.db.WithContext(ctx).First(&group, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, errors.NewNotFoundError("node group", id)
		}
		return nil, errors.NewDatabaseError("failed to get node group", result.Error)
	}
	return &group, nil
}

// Update updates a node group.
func (r *nodeGroupRepository) Update(ctx context.Context, group *NodeGroup) error {
	result := r.db.WithContext(ctx).Save(group)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to update node group", result.Error)
	}
	return nil
}

// Delete deletes a node group by ID (does not delete member nodes).
func (r *nodeGroupRepository) Delete(ctx context.Context, id int64) error {
	// First remove all member associations
	if err := r.RemoveAllNodes(ctx, id); err != nil {
		return err
	}

	// Then delete the group
	result := r.db.WithContext(ctx).Delete(&NodeGroup{}, id)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to delete node group", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.NewNotFoundError("node group", id)
	}
	return nil
}

// List retrieves node groups with pagination.
func (r *nodeGroupRepository) List(ctx context.Context, limit, offset int) ([]*NodeGroup, error) {
	var groups []*NodeGroup
	query := r.db.WithContext(ctx)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	result := query.Find(&groups)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to list node groups", result.Error)
	}
	return groups, nil
}

// Count returns the total number of node groups.
func (r *nodeGroupRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	result := r.db.WithContext(ctx).Model(&NodeGroup{}).Count(&count)
	if result.Error != nil {
		return 0, errors.NewDatabaseError("failed to count node groups", result.Error)
	}
	return count, nil
}


// AddNode adds a node to a group.
func (r *nodeGroupRepository) AddNode(ctx context.Context, groupID, nodeID int64) error {
	member := &NodeGroupMember{
		GroupID: groupID,
		NodeID:  nodeID,
	}
	result := r.db.WithContext(ctx).Create(member)
	if result.Error != nil {
		return errors.NewDatabaseError("failed to add node to group", result.Error)
	}
	return nil
}

// RemoveNode removes a node from a group.
func (r *nodeGroupRepository) RemoveNode(ctx context.Context, groupID, nodeID int64) error {
	result := r.db.WithContext(ctx).
		Where("group_id = ? AND node_id = ?", groupID, nodeID).
		Delete(&NodeGroupMember{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to remove node from group", result.Error)
	}
	return nil
}

// GetNodes retrieves all nodes in a group.
func (r *nodeGroupRepository) GetNodes(ctx context.Context, groupID int64) ([]*Node, error) {
	var nodes []*Node
	result := r.db.WithContext(ctx).
		Joins("JOIN node_group_members ON node_group_members.node_id = nodes.id").
		Where("node_group_members.group_id = ?", groupID).
		Find(&nodes)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get nodes in group", result.Error)
	}
	return nodes, nil
}

// GetNodeIDs retrieves all node IDs in a group.
func (r *nodeGroupRepository) GetNodeIDs(ctx context.Context, groupID int64) ([]int64, error) {
	var nodeIDs []int64
	result := r.db.WithContext(ctx).
		Model(&NodeGroupMember{}).
		Where("group_id = ?", groupID).
		Pluck("node_id", &nodeIDs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get node IDs in group", result.Error)
	}
	return nodeIDs, nil
}

// GetGroupsForNode retrieves all groups that a node belongs to.
func (r *nodeGroupRepository) GetGroupsForNode(ctx context.Context, nodeID int64) ([]*NodeGroup, error) {
	var groups []*NodeGroup
	result := r.db.WithContext(ctx).
		Joins("JOIN node_group_members ON node_group_members.group_id = node_groups.id").
		Where("node_group_members.node_id = ?", nodeID).
		Find(&groups)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get groups for node", result.Error)
	}
	return groups, nil
}

// GetGroupIDsForNode retrieves all group IDs that a node belongs to.
func (r *nodeGroupRepository) GetGroupIDsForNode(ctx context.Context, nodeID int64) ([]int64, error) {
	var groupIDs []int64
	result := r.db.WithContext(ctx).
		Model(&NodeGroupMember{}).
		Where("node_id = ?", nodeID).
		Pluck("group_id", &groupIDs)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get group IDs for node", result.Error)
	}
	return groupIDs, nil
}

// IsNodeInGroup checks if a node is a member of a group.
func (r *nodeGroupRepository) IsNodeInGroup(ctx context.Context, groupID, nodeID int64) (bool, error) {
	var count int64
	result := r.db.WithContext(ctx).
		Model(&NodeGroupMember{}).
		Where("group_id = ? AND node_id = ?", groupID, nodeID).
		Count(&count)
	if result.Error != nil {
		return false, errors.NewDatabaseError("failed to check node membership", result.Error)
	}
	return count > 0, nil
}

// GetStats retrieves aggregate statistics for a node group.
func (r *nodeGroupRepository) GetStats(ctx context.Context, groupID int64) (*NodeGroupStats, error) {
	stats := &NodeGroupStats{GroupID: groupID}

	// Get total nodes count
	var totalNodes int64
	err := r.db.WithContext(ctx).
		Model(&NodeGroupMember{}).
		Where("group_id = ?", groupID).
		Count(&totalNodes).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to count nodes in group", err)
	}
	stats.TotalNodes = totalNodes

	// Get healthy nodes count and total users
	type nodeStats struct {
		HealthyCount int64
		TotalUsers   int64
	}
	var ns nodeStats
	err = r.db.WithContext(ctx).
		Model(&Node{}).
		Select("COUNT(CASE WHEN status = ? THEN 1 END) as healthy_count, COALESCE(SUM(current_users), 0) as total_users", NodeStatusOnline).
		Joins("JOIN node_group_members ON node_group_members.node_id = nodes.id").
		Where("node_group_members.group_id = ?", groupID).
		Scan(&ns).Error
	if err != nil {
		return nil, errors.NewDatabaseError("failed to get group stats", err)
	}
	stats.HealthyNodes = ns.HealthyCount
	stats.TotalUsers = ns.TotalUsers

	return stats, nil
}

// GetAllStats retrieves aggregate statistics for all node groups.
func (r *nodeGroupRepository) GetAllStats(ctx context.Context) ([]*NodeGroupStats, error) {
	// Get all groups first
	var groups []*NodeGroup
	if err := r.db.WithContext(ctx).Find(&groups).Error; err != nil {
		return nil, errors.NewDatabaseError("failed to list groups", err)
	}

	stats := make([]*NodeGroupStats, 0, len(groups))
	for _, g := range groups {
		s, err := r.GetStats(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetByRegion retrieves node groups by region.
func (r *nodeGroupRepository) GetByRegion(ctx context.Context, region string) ([]*NodeGroup, error) {
	var groups []*NodeGroup
	result := r.db.WithContext(ctx).Where("region = ?", region).Find(&groups)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get groups by region", result.Error)
	}
	return groups, nil
}

// GetByStrategy retrieves node groups by load balancing strategy.
func (r *nodeGroupRepository) GetByStrategy(ctx context.Context, strategy string) ([]*NodeGroup, error) {
	var groups []*NodeGroup
	result := r.db.WithContext(ctx).Where("strategy = ?", strategy).Find(&groups)
	if result.Error != nil {
		return nil, errors.NewDatabaseError("failed to get groups by strategy", result.Error)
	}
	return groups, nil
}

// RemoveAllNodes removes all nodes from a group.
func (r *nodeGroupRepository) RemoveAllNodes(ctx context.Context, groupID int64) error {
	result := r.db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Delete(&NodeGroupMember{})
	if result.Error != nil {
		return errors.NewDatabaseError("failed to remove all nodes from group", result.Error)
	}
	return nil
}

// SetNodes sets the nodes for a group (replaces existing members).
func (r *nodeGroupRepository) SetNodes(ctx context.Context, groupID int64, nodeIDs []int64) error {
	// Use transaction to ensure atomicity
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Remove existing members
		if err := tx.Where("group_id = ?", groupID).Delete(&NodeGroupMember{}).Error; err != nil {
			return errors.NewDatabaseError("failed to remove existing members", err)
		}

		// Add new members
		if len(nodeIDs) > 0 {
			members := make([]*NodeGroupMember, len(nodeIDs))
			for i, nodeID := range nodeIDs {
				members[i] = &NodeGroupMember{
					GroupID: groupID,
					NodeID:  nodeID,
				}
			}
			if err := tx.Create(&members).Error; err != nil {
				return errors.NewDatabaseError("failed to add new members", err)
			}
		}

		return nil
	})
}
