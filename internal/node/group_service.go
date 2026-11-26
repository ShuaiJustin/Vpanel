// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"errors"
	"fmt"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
)

// Group service errors
var (
	ErrGroupNotFound     = errors.New("node group not found")
	ErrInvalidGroup      = errors.New("invalid group data")
	ErrNodeNotInGroup    = errors.New("node is not in the group")
	ErrNodeAlreadyInGroup = errors.New("node is already in the group")
)

// NodeGroup represents a node group in the service layer.
type NodeGroup struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Region      string    `json:"region"`
	Strategy    string    `json:"strategy"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NodeGroupStats represents aggregate statistics for a node group.
type NodeGroupStats struct {
	GroupID      int64 `json:"group_id"`
	TotalNodes   int64 `json:"total_nodes"`
	HealthyNodes int64 `json:"healthy_nodes"`
	TotalUsers   int64 `json:"total_users"`
}

// CreateGroupRequest represents a request to create a node group.
type CreateGroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Region      string `json:"region"`
	Strategy    string `json:"strategy"`
}

// UpdateGroupRequest represents a request to update a node group.
type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Region      *string `json:"region"`
	Strategy    *string `json:"strategy"`
}

// GroupService provides node group management operations.
type GroupService struct {
	groupRepo repository.NodeGroupRepository
	nodeRepo  repository.NodeRepository
	logger    logger.Logger
}

// NewGroupService creates a new group service.
func NewGroupService(
	groupRepo repository.NodeGroupRepository,
	nodeRepo repository.NodeRepository,
	log logger.Logger,
) *GroupService {
	return &GroupService{
		groupRepo: groupRepo,
		nodeRepo:  nodeRepo,
		logger:    log,
	}
}


// ============================================
// Group CRUD Operations
// ============================================

// Create creates a new node group.
func (s *GroupService) Create(ctx context.Context, req *CreateGroupRequest) (*NodeGroup, error) {
	if req.Name == "" {
		return nil, ErrInvalidGroup
	}

	// Set default strategy if not provided
	strategy := req.Strategy
	if strategy == "" {
		strategy = repository.StrategyRoundRobin
	}

	repoGroup := &repository.NodeGroup{
		Name:        req.Name,
		Description: req.Description,
		Region:      req.Region,
		Strategy:    strategy,
	}

	if err := s.groupRepo.Create(ctx, repoGroup); err != nil {
		s.logger.Error("Failed to create node group", logger.Err(err))
		return nil, err
	}

	s.logger.Info("Created node group",
		logger.F("id", repoGroup.ID),
		logger.F("name", repoGroup.Name))

	return s.toNodeGroup(repoGroup), nil
}

// GetByID retrieves a node group by ID.
func (s *GroupService) GetByID(ctx context.Context, id int64) (*NodeGroup, error) {
	repoGroup, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrGroupNotFound
	}
	return s.toNodeGroup(repoGroup), nil
}

// Update updates a node group.
func (s *GroupService) Update(ctx context.Context, id int64, req *UpdateGroupRequest) (*NodeGroup, error) {
	repoGroup, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrGroupNotFound
	}

	if req.Name != nil {
		if *req.Name == "" {
			return nil, ErrInvalidGroup
		}
		repoGroup.Name = *req.Name
	}
	if req.Description != nil {
		repoGroup.Description = *req.Description
	}
	if req.Region != nil {
		repoGroup.Region = *req.Region
	}
	if req.Strategy != nil {
		repoGroup.Strategy = *req.Strategy
	}

	if err := s.groupRepo.Update(ctx, repoGroup); err != nil {
		s.logger.Error("Failed to update node group", logger.Err(err), logger.F("id", id))
		return nil, err
	}

	s.logger.Info("Updated node group",
		logger.F("id", id),
		logger.F("name", repoGroup.Name))

	return s.toNodeGroup(repoGroup), nil
}

// Delete deletes a node group.
// This removes all member associations but does NOT delete the nodes themselves.
func (s *GroupService) Delete(ctx context.Context, id int64) error {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, id)
	if err != nil {
		return ErrGroupNotFound
	}

	// Delete the group (repository handles removing member associations)
	if err := s.groupRepo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete node group", logger.Err(err), logger.F("id", id))
		return err
	}

	s.logger.Info("Deleted node group", logger.F("id", id))
	return nil
}

// List lists node groups with pagination.
func (s *GroupService) List(ctx context.Context, limit, offset int) ([]*NodeGroup, int64, error) {
	repoGroups, err := s.groupRepo.List(ctx, limit, offset)
	if err != nil {
		s.logger.Error("Failed to list node groups", logger.Err(err))
		return nil, 0, err
	}

	total, err := s.groupRepo.Count(ctx)
	if err != nil {
		s.logger.Error("Failed to count node groups", logger.Err(err))
		return nil, 0, err
	}

	groups := make([]*NodeGroup, len(repoGroups))
	for i, rg := range repoGroups {
		groups[i] = s.toNodeGroup(rg)
	}

	return groups, total, nil
}


// ============================================
// Member Management Operations
// ============================================

// AddNode adds a node to a group.
func (s *GroupService) AddNode(ctx context.Context, groupID, nodeID int64) error {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}

	// Verify node exists
	_, err = s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return ErrNodeNotFound
	}

	// Check if node is already in group
	inGroup, err := s.groupRepo.IsNodeInGroup(ctx, groupID, nodeID)
	if err != nil {
		s.logger.Error("Failed to check node membership",
			logger.Err(err),
			logger.F("group_id", groupID),
			logger.F("node_id", nodeID))
		return err
	}
	if inGroup {
		return ErrNodeAlreadyInGroup
	}

	// Add node to group
	if err := s.groupRepo.AddNode(ctx, groupID, nodeID); err != nil {
		s.logger.Error("Failed to add node to group",
			logger.Err(err),
			logger.F("group_id", groupID),
			logger.F("node_id", nodeID))
		return err
	}

	s.logger.Info("Added node to group",
		logger.F("group_id", groupID),
		logger.F("node_id", nodeID))

	return nil
}

// RemoveNode removes a node from a group.
func (s *GroupService) RemoveNode(ctx context.Context, groupID, nodeID int64) error {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}

	// Check if node is in group
	inGroup, err := s.groupRepo.IsNodeInGroup(ctx, groupID, nodeID)
	if err != nil {
		s.logger.Error("Failed to check node membership",
			logger.Err(err),
			logger.F("group_id", groupID),
			logger.F("node_id", nodeID))
		return err
	}
	if !inGroup {
		return ErrNodeNotInGroup
	}

	// Remove node from group
	if err := s.groupRepo.RemoveNode(ctx, groupID, nodeID); err != nil {
		s.logger.Error("Failed to remove node from group",
			logger.Err(err),
			logger.F("group_id", groupID),
			logger.F("node_id", nodeID))
		return err
	}

	s.logger.Info("Removed node from group",
		logger.F("group_id", groupID),
		logger.F("node_id", nodeID))

	return nil
}

// GetNodes returns all nodes in a group.
func (s *GroupService) GetNodes(ctx context.Context, groupID int64) ([]*Node, error) {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, ErrGroupNotFound
	}

	repoNodes, err := s.groupRepo.GetNodes(ctx, groupID)
	if err != nil {
		s.logger.Error("Failed to get nodes in group",
			logger.Err(err),
			logger.F("group_id", groupID))
		return nil, err
	}

	nodes := make([]*Node, len(repoNodes))
	for i, rn := range repoNodes {
		nodes[i] = repoNodeToNode(rn)
	}

	return nodes, nil
}

// GetGroupsForNode returns all groups that a node belongs to.
func (s *GroupService) GetGroupsForNode(ctx context.Context, nodeID int64) ([]*NodeGroup, error) {
	// Verify node exists
	_, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, ErrNodeNotFound
	}

	repoGroups, err := s.groupRepo.GetGroupsForNode(ctx, nodeID)
	if err != nil {
		s.logger.Error("Failed to get groups for node",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return nil, err
	}

	groups := make([]*NodeGroup, len(repoGroups))
	for i, rg := range repoGroups {
		groups[i] = s.toNodeGroup(rg)
	}

	return groups, nil
}

// IsNodeInGroup checks if a node is a member of a group.
func (s *GroupService) IsNodeInGroup(ctx context.Context, groupID, nodeID int64) (bool, error) {
	return s.groupRepo.IsNodeInGroup(ctx, groupID, nodeID)
}

// SetNodes sets the nodes for a group (replaces existing members).
func (s *GroupService) SetNodes(ctx context.Context, groupID int64, nodeIDs []int64) error {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return ErrGroupNotFound
	}

	// Verify all nodes exist
	for _, nodeID := range nodeIDs {
		_, err := s.nodeRepo.GetByID(ctx, nodeID)
		if err != nil {
			return ErrNodeNotFound
		}
	}

	// Set nodes
	if err := s.groupRepo.SetNodes(ctx, groupID, nodeIDs); err != nil {
		s.logger.Error("Failed to set nodes for group",
			logger.Err(err),
			logger.F("group_id", groupID),
			logger.F("node_count", len(nodeIDs)))
		return err
	}

	s.logger.Info("Set nodes for group",
		logger.F("group_id", groupID),
		logger.F("node_count", len(nodeIDs)))

	return nil
}


// ============================================
// Statistics Operations
// ============================================

// GetStats returns aggregate statistics for a node group.
func (s *GroupService) GetStats(ctx context.Context, groupID int64) (*NodeGroupStats, error) {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, ErrGroupNotFound
	}

	repoStats, err := s.groupRepo.GetStats(ctx, groupID)
	if err != nil {
		s.logger.Error("Failed to get group stats",
			logger.Err(err),
			logger.F("group_id", groupID))
		return nil, err
	}

	return &NodeGroupStats{
		GroupID:      repoStats.GroupID,
		TotalNodes:   repoStats.TotalNodes,
		HealthyNodes: repoStats.HealthyNodes,
		TotalUsers:   repoStats.TotalUsers,
	}, nil
}

// GetAllStats returns aggregate statistics for all node groups.
func (s *GroupService) GetAllStats(ctx context.Context) ([]*NodeGroupStats, error) {
	repoStats, err := s.groupRepo.GetAllStats(ctx)
	if err != nil {
		s.logger.Error("Failed to get all group stats", logger.Err(err))
		return nil, err
	}

	stats := make([]*NodeGroupStats, len(repoStats))
	for i, rs := range repoStats {
		stats[i] = &NodeGroupStats{
			GroupID:      rs.GroupID,
			TotalNodes:   rs.TotalNodes,
			HealthyNodes: rs.HealthyNodes,
			TotalUsers:   rs.TotalUsers,
		}
	}

	return stats, nil
}

// CalculateGroupStats calculates statistics for a group from its nodes.
// This is useful for verifying stats accuracy.
func (s *GroupService) CalculateGroupStats(ctx context.Context, groupID int64) (*NodeGroupStats, error) {
	// Verify group exists
	_, err := s.groupRepo.GetByID(ctx, groupID)
	if err != nil {
		return nil, ErrGroupNotFound
	}

	// Get all nodes in the group
	nodes, err := s.groupRepo.GetNodes(ctx, groupID)
	if err != nil {
		return nil, err
	}

	stats := &NodeGroupStats{
		GroupID:      groupID,
		TotalNodes:   int64(len(nodes)),
		HealthyNodes: 0,
		TotalUsers:   0,
	}

	for _, node := range nodes {
		if node.Status == repository.NodeStatusOnline {
			stats.HealthyNodes++
		}
		stats.TotalUsers += int64(node.CurrentUsers)
	}

	return stats, nil
}

// ============================================
// Query Operations
// ============================================

// GetByRegion returns all groups in a specific region.
func (s *GroupService) GetByRegion(ctx context.Context, region string) ([]*NodeGroup, error) {
	repoGroups, err := s.groupRepo.GetByRegion(ctx, region)
	if err != nil {
		s.logger.Error("Failed to get groups by region",
			logger.Err(err),
			logger.F("region", region))
		return nil, err
	}

	groups := make([]*NodeGroup, len(repoGroups))
	for i, rg := range repoGroups {
		groups[i] = s.toNodeGroup(rg)
	}

	return groups, nil
}

// GetByStrategy returns all groups using a specific load balancing strategy.
func (s *GroupService) GetByStrategy(ctx context.Context, strategy string) ([]*NodeGroup, error) {
	repoGroups, err := s.groupRepo.GetByStrategy(ctx, strategy)
	if err != nil {
		s.logger.Error("Failed to get groups by strategy",
			logger.Err(err),
			logger.F("strategy", strategy))
		return nil, err
	}

	groups := make([]*NodeGroup, len(repoGroups))
	for i, rg := range repoGroups {
		groups[i] = s.toNodeGroup(rg)
	}

	return groups, nil
}

// ============================================
// Helper Methods
// ============================================

// toNodeGroup converts a repository node group to a service node group.
func (s *GroupService) toNodeGroup(rg *repository.NodeGroup) *NodeGroup {
	return &NodeGroup{
		ID:          rg.ID,
		Name:        rg.Name,
		Description: rg.Description,
		Region:      rg.Region,
		Strategy:    rg.Strategy,
		CreatedAt:   rg.CreatedAt,
		UpdatedAt:   rg.UpdatedAt,
	}
}


// ============================================
// Multi-Group Membership Operations
// ============================================

// AddNodeToGroups adds a node to multiple groups at once.
func (s *GroupService) AddNodeToGroups(ctx context.Context, nodeID int64, groupIDs []int64) error {
	// Verify node exists
	_, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return ErrNodeNotFound
	}

	for _, groupID := range groupIDs {
		// Verify group exists
		_, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			s.logger.Warn("Group not found, skipping",
				logger.F("group_id", groupID),
				logger.F("node_id", nodeID))
			continue
		}

		// Check if already in group
		inGroup, err := s.groupRepo.IsNodeInGroup(ctx, groupID, nodeID)
		if err != nil {
			s.logger.Error("Failed to check node membership",
				logger.Err(err),
				logger.F("group_id", groupID),
				logger.F("node_id", nodeID))
			continue
		}
		if inGroup {
			continue // Already in group, skip
		}

		// Add to group
		if err := s.groupRepo.AddNode(ctx, groupID, nodeID); err != nil {
			s.logger.Error("Failed to add node to group",
				logger.Err(err),
				logger.F("group_id", groupID),
				logger.F("node_id", nodeID))
			return err
		}
	}

	s.logger.Info("Added node to multiple groups",
		logger.F("node_id", nodeID),
		logger.F("group_count", len(groupIDs)))

	return nil
}

// RemoveNodeFromAllGroups removes a node from all groups it belongs to.
func (s *GroupService) RemoveNodeFromAllGroups(ctx context.Context, nodeID int64) error {
	// Get all groups the node belongs to
	groupIDs, err := s.groupRepo.GetGroupIDsForNode(ctx, nodeID)
	if err != nil {
		s.logger.Error("Failed to get groups for node",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return err
	}

	for _, groupID := range groupIDs {
		if err := s.groupRepo.RemoveNode(ctx, groupID, nodeID); err != nil {
			s.logger.Error("Failed to remove node from group",
				logger.Err(err),
				logger.F("group_id", groupID),
				logger.F("node_id", nodeID))
			// Continue removing from other groups
		}
	}

	s.logger.Info("Removed node from all groups",
		logger.F("node_id", nodeID),
		logger.F("group_count", len(groupIDs)))

	return nil
}

// GetGroupCountForNode returns the number of groups a node belongs to.
func (s *GroupService) GetGroupCountForNode(ctx context.Context, nodeID int64) (int, error) {
	groupIDs, err := s.groupRepo.GetGroupIDsForNode(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	return len(groupIDs), nil
}

// SyncNodeGroups synchronizes a node's group memberships to match the provided list.
// Groups not in the list will be removed, groups in the list will be added.
func (s *GroupService) SyncNodeGroups(ctx context.Context, nodeID int64, targetGroupIDs []int64) error {
	// Verify node exists
	_, err := s.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return ErrNodeNotFound
	}

	// Get current groups
	currentGroupIDs, err := s.groupRepo.GetGroupIDsForNode(ctx, nodeID)
	if err != nil {
		return err
	}

	// Create maps for efficient lookup
	currentMap := make(map[int64]bool)
	for _, id := range currentGroupIDs {
		currentMap[id] = true
	}

	targetMap := make(map[int64]bool)
	for _, id := range targetGroupIDs {
		targetMap[id] = true
	}

	// Remove from groups not in target
	var syncErr error
	for _, groupID := range currentGroupIDs {
		if !targetMap[groupID] {
			if err := s.groupRepo.RemoveNode(ctx, groupID, nodeID); err != nil {
				s.logger.Error("Failed to remove node from group during sync",
					logger.Err(err),
					logger.F("group_id", groupID),
					logger.F("node_id", nodeID))
				if syncErr == nil {
					syncErr = fmt.Errorf("remove node from group %d: %w", groupID, err)
				}
			}
		}
	}

	// Add to groups in target but not current
	for _, groupID := range targetGroupIDs {
		if !currentMap[groupID] {
			// Verify group exists
			_, err := s.groupRepo.GetByID(ctx, groupID)
			if err != nil {
				s.logger.Warn("Target group not found, skipping",
					logger.F("group_id", groupID))
				if syncErr == nil {
					syncErr = fmt.Errorf("target group %d not found: %w", groupID, err)
				}
				continue
			}

			if err := s.groupRepo.AddNode(ctx, groupID, nodeID); err != nil {
				s.logger.Error("Failed to add node to group during sync",
					logger.Err(err),
					logger.F("group_id", groupID),
					logger.F("node_id", nodeID))
				if syncErr == nil {
					syncErr = fmt.Errorf("add node to group %d: %w", groupID, err)
				}
			}
		}
	}

	s.logger.Info("Synced node group memberships",
		logger.F("node_id", nodeID),
		logger.F("target_groups", len(targetGroupIDs)))

	return syncErr
}
