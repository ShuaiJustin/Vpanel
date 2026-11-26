// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"v/internal/database/repository"
	"v/internal/logger"
	"v/internal/notification"
)

// Failover manager errors
var (
	ErrFailoverInProgress    = errors.New("failover already in progress for this node")
	ErrNoTargetNodes         = errors.New("no target nodes available for failover")
	ErrMaxConcurrentExceeded = errors.New("maximum concurrent migrations exceeded")
	ErrNodeNotUnhealthy      = errors.New("node is not unhealthy")
)

// FailoverConfig holds configuration for the failover manager.
type FailoverConfig struct {
	// MaxConcurrentMigrations is the maximum number of users that can be migrated concurrently
	MaxConcurrentMigrations int
	// MigrationTimeout is the timeout for a single user migration
	MigrationTimeout time.Duration
	// PreferSameGroup indicates whether to prefer nodes in the same group for failover
	PreferSameGroup bool
	// AllowCrossGroupFailover indicates whether to allow failover to nodes in other groups
	AllowCrossGroupFailover bool
	// RecoveryEnabled indicates whether to migrate users back when a node recovers
	RecoveryEnabled bool
}

// DefaultFailoverConfig returns the default failover configuration.
func DefaultFailoverConfig() *FailoverConfig {
	return &FailoverConfig{
		MaxConcurrentMigrations: 10,
		MigrationTimeout:        30 * time.Second,
		PreferSameGroup:         true,
		AllowCrossGroupFailover: true,
		RecoveryEnabled:         false,
	}
}

// FailoverEvent represents a failover event for logging and notification.
type FailoverEvent struct {
	NodeID          int64
	NodeName        string
	AffectedUsers   int
	MigratedUsers   int
	FailedUsers     int
	TargetNodes     []int64
	StartedAt       time.Time
	CompletedAt     time.Time
	Reason          string
	CrossGroupUsed  bool
}

// MigrationResult represents the result of a user migration.
type MigrationResult struct {
	UserID       int64
	SourceNodeID int64
	TargetNodeID int64
	Success      bool
	Error        error
	Duration     time.Duration
}

// FailoverManager handles automatic failover when nodes become unhealthy.
type FailoverManager struct {
	config          *FailoverConfig
	nodeRepo        repository.NodeRepository
	groupRepo       repository.NodeGroupRepository
	assignmentRepo  repository.UserNodeAssignmentRepository
	logger          logger.Logger
	notificationSvc *notification.Service

	// State tracking
	activeFailovers   map[int64]bool // nodeID -> failover in progress
	activeFailoversMu sync.RWMutex

	// Concurrent migration control
	currentMigrations int32 // atomic counter

	// Callbacks
	onFailoverComplete func(event *FailoverEvent)
}

// NewFailoverManager creates a new failover manager.
func NewFailoverManager(
	config *FailoverConfig,
	nodeRepo repository.NodeRepository,
	groupRepo repository.NodeGroupRepository,
	assignmentRepo repository.UserNodeAssignmentRepository,
	log logger.Logger,
) *FailoverManager {
	if config == nil {
		config = DefaultFailoverConfig()
	}

	return &FailoverManager{
		config:          config,
		nodeRepo:        nodeRepo,
		groupRepo:       groupRepo,
		assignmentRepo:  assignmentRepo,
		logger:          log,
		activeFailovers: make(map[int64]bool),
	}
}

// SetNotificationService sets the notification service for sending alerts.
func (fm *FailoverManager) SetNotificationService(svc *notification.Service) {
	fm.notificationSvc = svc
}

// SetOnFailoverComplete sets the callback for failover completion.
func (fm *FailoverManager) SetOnFailoverComplete(callback func(event *FailoverEvent)) {
	fm.onFailoverComplete = callback
}

// UpdateConfig updates the failover manager configuration.
func (fm *FailoverManager) UpdateConfig(config *FailoverConfig) {
	fm.config = config
}

// GetConfig returns the current failover configuration.
func (fm *FailoverManager) GetConfig() *FailoverConfig {
	return fm.config
}


// TriggerFailover initiates failover for an unhealthy node.
// It migrates all users from the unhealthy node to healthy nodes.
func (fm *FailoverManager) TriggerFailover(ctx context.Context, nodeID int64) (*FailoverEvent, error) {
	// Check if failover is already in progress for this node
	fm.activeFailoversMu.Lock()
	if fm.activeFailovers[nodeID] {
		fm.activeFailoversMu.Unlock()
		return nil, ErrFailoverInProgress
	}
	fm.activeFailovers[nodeID] = true
	fm.activeFailoversMu.Unlock()

	defer func() {
		fm.activeFailoversMu.Lock()
		delete(fm.activeFailovers, nodeID)
		fm.activeFailoversMu.Unlock()
	}()

	// Get the unhealthy node
	node, err := fm.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	// Verify node is unhealthy
	if node.Status != repository.NodeStatusUnhealthy {
		return nil, ErrNodeNotUnhealthy
	}

	event := &FailoverEvent{
		NodeID:    nodeID,
		NodeName:  node.Name,
		StartedAt: time.Now(),
		Reason:    "Node became unhealthy",
	}

	// Get users assigned to this node
	userIDs, err := fm.assignmentRepo.GetUserIDsByNodeID(ctx, nodeID)
	if err != nil {
		fm.logger.Error("Failed to get users for failover",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return nil, err
	}

	event.AffectedUsers = len(userIDs)

	if len(userIDs) == 0 {
		fm.logger.Info("No users to migrate during failover",
			logger.F("node_id", nodeID),
			logger.F("node_name", node.Name))
		event.CompletedAt = time.Now()
		fm.notifyFailoverComplete(event)
		return event, nil
	}

	// Get target nodes for migration
	targetNodes, crossGroupUsed, err := fm.getTargetNodes(ctx, nodeID)
	if err != nil {
		fm.logger.Error("Failed to get target nodes for failover",
			logger.Err(err),
			logger.F("node_id", nodeID))
		return nil, err
	}

	if len(targetNodes) == 0 {
		fm.logger.Error("No target nodes available for failover",
			logger.F("node_id", nodeID))
		return nil, ErrNoTargetNodes
	}

	event.CrossGroupUsed = crossGroupUsed
	for _, n := range targetNodes {
		event.TargetNodes = append(event.TargetNodes, n.ID)
	}

	// Migrate users
	results := fm.migrateUsers(ctx, userIDs, targetNodes)

	// Count results
	for _, result := range results {
		if result.Success {
			event.MigratedUsers++
		} else {
			event.FailedUsers++
		}
	}

	event.CompletedAt = time.Now()

	fm.logger.Info("Failover completed",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name),
		logger.F("affected_users", event.AffectedUsers),
		logger.F("migrated_users", event.MigratedUsers),
		logger.F("failed_users", event.FailedUsers),
		logger.F("cross_group_used", crossGroupUsed),
		logger.F("duration_ms", event.CompletedAt.Sub(event.StartedAt).Milliseconds()))

	fm.notifyFailoverComplete(event)

	return event, nil
}

// TriggerRecovery handles node recovery by optionally migrating users back.
func (fm *FailoverManager) TriggerRecovery(ctx context.Context, nodeID int64) error {
	if !fm.config.RecoveryEnabled {
		fm.logger.Debug("Recovery migration disabled, skipping",
			logger.F("node_id", nodeID))
		return nil
	}

	node, err := fm.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return err
	}

	fm.logger.Info("Node recovered",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name))

	// Recovery migration is optional and not implemented in this version
	// Users will naturally be assigned to the recovered node for new requests
	return nil
}

// IsFailoverInProgress checks if a failover is in progress for a node.
func (fm *FailoverManager) IsFailoverInProgress(nodeID int64) bool {
	fm.activeFailoversMu.RLock()
	defer fm.activeFailoversMu.RUnlock()
	return fm.activeFailovers[nodeID]
}

// GetCurrentMigrations returns the current number of active migrations.
func (fm *FailoverManager) GetCurrentMigrations() int {
	return int(atomic.LoadInt32(&fm.currentMigrations))
}


// getTargetNodes returns healthy nodes for failover, prioritizing same-group nodes.
// Returns the target nodes and whether cross-group failover was used.
func (fm *FailoverManager) getTargetNodes(ctx context.Context, sourceNodeID int64) ([]*repository.Node, bool, error) {
	var targetNodes []*repository.Node
	crossGroupUsed := false

	if fm.config.PreferSameGroup {
		// Get groups that the source node belongs to
		groupIDs, err := fm.groupRepo.GetGroupIDsForNode(ctx, sourceNodeID)
		if err != nil {
			fm.logger.Warn("Failed to get groups for node, falling back to all nodes",
				logger.Err(err),
				logger.F("node_id", sourceNodeID))
		} else if len(groupIDs) > 0 {
			// Get healthy nodes from the same groups
			targetNodes, err = fm.getHealthyNodesInGroups(ctx, groupIDs, sourceNodeID)
			if err != nil {
				fm.logger.Warn("Failed to get healthy nodes in groups",
					logger.Err(err))
			}
		}
	}

	// If no same-group nodes available and cross-group is allowed
	if len(targetNodes) == 0 && fm.config.AllowCrossGroupFailover {
		crossGroupUsed = true
		nodes, err := fm.nodeRepo.GetAvailable(ctx)
		if err != nil {
			return nil, false, err
		}

		// Filter out the source node
		for _, n := range nodes {
			if n.ID != sourceNodeID {
				targetNodes = append(targetNodes, n)
			}
		}
	}

	return targetNodes, crossGroupUsed, nil
}

// getHealthyNodesInGroups returns healthy nodes that belong to any of the specified groups.
func (fm *FailoverManager) getHealthyNodesInGroups(ctx context.Context, groupIDs []int64, excludeNodeID int64) ([]*repository.Node, error) {
	nodeMap := make(map[int64]*repository.Node)

	for _, groupID := range groupIDs {
		nodes, err := fm.groupRepo.GetNodes(ctx, groupID)
		if err != nil {
			continue
		}

		for _, n := range nodes {
			// Skip the source node and unhealthy/offline nodes
			if n.ID == excludeNodeID {
				continue
			}
			if n.Status != repository.NodeStatusOnline {
				continue
			}
			// Skip nodes at capacity
			if n.MaxUsers > 0 && n.CurrentUsers >= n.MaxUsers {
				continue
			}
			nodeMap[n.ID] = n
		}
	}

	result := make([]*repository.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		result = append(result, n)
	}

	return result, nil
}

// migrateUsers migrates users to target nodes with concurrency control.
func (fm *FailoverManager) migrateUsers(ctx context.Context, userIDs []int64, targetNodes []*repository.Node) []*MigrationResult {
	results := make([]*MigrationResult, len(userIDs))
	
	// Create a semaphore for concurrency control
	sem := make(chan struct{}, fm.config.MaxConcurrentMigrations)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for i, userID := range userIDs {
		wg.Add(1)
		go func(idx int, uid int64) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			atomic.AddInt32(&fm.currentMigrations, 1)
			defer func() {
				<-sem
				atomic.AddInt32(&fm.currentMigrations, -1)
			}()

			// Select target node using round-robin
			targetNode := targetNodes[idx%len(targetNodes)]

			// Perform migration with timeout
			result := fm.migrateUser(ctx, uid, targetNode.ID)
			result.TargetNodeID = targetNode.ID

			resultsMu.Lock()
			results[idx] = result
			resultsMu.Unlock()
		}(i, userID)
	}

	wg.Wait()
	return results
}

// migrateUser migrates a single user to a target node.
func (fm *FailoverManager) migrateUser(ctx context.Context, userID, targetNodeID int64) *MigrationResult {
	start := time.Now()
	result := &MigrationResult{
		UserID:       userID,
		TargetNodeID: targetNodeID,
	}

	// Get current assignment to record source node
	assignment, err := fm.assignmentRepo.GetByUserID(ctx, userID)
	if err == nil && assignment != nil {
		result.SourceNodeID = assignment.NodeID
	}

	// Create timeout context
	migrationCtx, cancel := context.WithTimeout(ctx, fm.config.MigrationTimeout)
	defer cancel()

	// Perform reassignment
	err = fm.assignmentRepo.Reassign(migrationCtx, userID, targetNodeID)
	result.Duration = time.Since(start)

	if err != nil {
		result.Success = false
		result.Error = err
		fm.logger.Error("Failed to migrate user",
			logger.Err(err),
			logger.F("user_id", userID),
			logger.F("target_node_id", targetNodeID))
	} else {
		result.Success = true
		fm.logger.Debug("User migrated successfully",
			logger.F("user_id", userID),
			logger.F("source_node_id", result.SourceNodeID),
			logger.F("target_node_id", targetNodeID))
	}

	return result
}

// notifyFailoverComplete sends notifications and triggers callbacks for failover completion.
func (fm *FailoverManager) notifyFailoverComplete(event *FailoverEvent) {
	// Send notification
	if fm.notificationSvc != nil {
		data := notification.NodeStatusChangeData{
			NodeID:    event.NodeID,
			NodeName:  event.NodeName,
			OldStatus: repository.NodeStatusOnline,
			NewStatus: repository.NodeStatusUnhealthy,
			Reason: fmt.Sprintf("Failover completed: %d/%d users migrated",
				event.MigratedUsers, event.AffectedUsers),
			Timestamp: event.CompletedAt,
		}
		if err := fm.notificationSvc.NotifyNodeStatusChange(data); err != nil {
			fm.logger.Error("Failed to send failover notification",
				logger.Err(err),
				logger.F("node_id", event.NodeID))
		}
	}

	// Trigger callback
	if fm.onFailoverComplete != nil {
		fm.onFailoverComplete(event)
	}
}


// MigrateUsersFromNode migrates all users from a source node to available target nodes.
// This is a public method that can be called directly for manual migration.
func (fm *FailoverManager) MigrateUsersFromNode(ctx context.Context, sourceNodeID int64, targetNodeIDs []int64) (*FailoverEvent, error) {
	// Get the source node
	node, err := fm.nodeRepo.GetByID(ctx, sourceNodeID)
	if err != nil {
		return nil, err
	}

	event := &FailoverEvent{
		NodeID:      sourceNodeID,
		NodeName:    node.Name,
		StartedAt:   time.Now(),
		Reason:      "Manual migration",
		TargetNodes: targetNodeIDs,
	}

	// Get users assigned to this node
	userIDs, err := fm.assignmentRepo.GetUserIDsByNodeID(ctx, sourceNodeID)
	if err != nil {
		return nil, err
	}

	event.AffectedUsers = len(userIDs)

	if len(userIDs) == 0 {
		event.CompletedAt = time.Now()
		return event, nil
	}

	// Get target nodes
	var targetNodes []*repository.Node
	if len(targetNodeIDs) > 0 {
		for _, id := range targetNodeIDs {
			n, err := fm.nodeRepo.GetByID(ctx, id)
			if err != nil {
				continue
			}
			if n.Status == repository.NodeStatusOnline {
				targetNodes = append(targetNodes, n)
			}
		}
	} else {
		// Use all available nodes except source
		nodes, err := fm.nodeRepo.GetAvailable(ctx)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			if n.ID != sourceNodeID {
				targetNodes = append(targetNodes, n)
			}
		}
	}

	if len(targetNodes) == 0 {
		return nil, ErrNoTargetNodes
	}

	// Migrate users
	results := fm.migrateUsers(ctx, userIDs, targetNodes)

	for _, result := range results {
		if result.Success {
			event.MigratedUsers++
		} else {
			event.FailedUsers++
		}
	}

	event.CompletedAt = time.Now()
	return event, nil
}

// MigrateSingleUser migrates a single user to a specific target node.
func (fm *FailoverManager) MigrateSingleUser(ctx context.Context, userID, targetNodeID int64) (*MigrationResult, error) {
	// Verify target node exists and is healthy
	targetNode, err := fm.nodeRepo.GetByID(ctx, targetNodeID)
	if err != nil {
		return nil, err
	}

	if targetNode.Status != repository.NodeStatusOnline {
		return nil, fmt.Errorf("target node is not online: %s", targetNode.Status)
	}

	// Check capacity
	if targetNode.MaxUsers > 0 && targetNode.CurrentUsers >= targetNode.MaxUsers {
		return nil, ErrNodeAtCapacity
	}

	// Check concurrent migration limit
	current := atomic.LoadInt32(&fm.currentMigrations)
	if int(current) >= fm.config.MaxConcurrentMigrations {
		return nil, ErrMaxConcurrentExceeded
	}

	// Perform migration
	atomic.AddInt32(&fm.currentMigrations, 1)
	defer atomic.AddInt32(&fm.currentMigrations, -1)

	result := fm.migrateUser(ctx, userID, targetNodeID)
	return result, nil
}

// GetMigrationStats returns statistics about migrations.
type MigrationStats struct {
	ActiveMigrations      int
	MaxConcurrentAllowed  int
	ActiveFailovers       int
	ActiveFailoverNodeIDs []int64
}

// GetMigrationStats returns current migration statistics.
func (fm *FailoverManager) GetMigrationStats() *MigrationStats {
	fm.activeFailoversMu.RLock()
	defer fm.activeFailoversMu.RUnlock()

	nodeIDs := make([]int64, 0, len(fm.activeFailovers))
	for nodeID := range fm.activeFailovers {
		nodeIDs = append(nodeIDs, nodeID)
	}

	return &MigrationStats{
		ActiveMigrations:      int(atomic.LoadInt32(&fm.currentMigrations)),
		MaxConcurrentAllowed:  fm.config.MaxConcurrentMigrations,
		ActiveFailovers:       len(fm.activeFailovers),
		ActiveFailoverNodeIDs: nodeIDs,
	}
}

// CanAcceptMigration checks if the failover manager can accept more migrations.
func (fm *FailoverManager) CanAcceptMigration() bool {
	current := atomic.LoadInt32(&fm.currentMigrations)
	return int(current) < fm.config.MaxConcurrentMigrations
}


// GetSameGroupNodes returns healthy nodes that are in the same groups as the source node.
// This is used for same-group priority failover.
func (fm *FailoverManager) GetSameGroupNodes(ctx context.Context, sourceNodeID int64) ([]*repository.Node, error) {
	// Get groups that the source node belongs to
	groupIDs, err := fm.groupRepo.GetGroupIDsForNode(ctx, sourceNodeID)
	if err != nil {
		return nil, err
	}

	if len(groupIDs) == 0 {
		return nil, nil
	}

	return fm.getHealthyNodesInGroups(ctx, groupIDs, sourceNodeID)
}

// GetCrossGroupNodes returns healthy nodes that are NOT in the same groups as the source node.
// This is used for cross-group failover when same-group nodes are unavailable.
func (fm *FailoverManager) GetCrossGroupNodes(ctx context.Context, sourceNodeID int64) ([]*repository.Node, error) {
	// Get all available nodes
	allNodes, err := fm.nodeRepo.GetAvailable(ctx)
	if err != nil {
		return nil, err
	}

	// Get groups that the source node belongs to
	groupIDs, err := fm.groupRepo.GetGroupIDsForNode(ctx, sourceNodeID)
	if err != nil {
		// If we can't get groups, return all nodes except source
		var result []*repository.Node
		for _, n := range allNodes {
			if n.ID != sourceNodeID {
				result = append(result, n)
			}
		}
		return result, nil
	}

	// Get nodes in same groups
	sameGroupNodes, err := fm.getHealthyNodesInGroups(ctx, groupIDs, sourceNodeID)
	if err != nil {
		sameGroupNodes = nil
	}

	// Create a set of same-group node IDs
	sameGroupSet := make(map[int64]bool)
	for _, n := range sameGroupNodes {
		sameGroupSet[n.ID] = true
	}

	// Filter to get cross-group nodes
	var crossGroupNodes []*repository.Node
	for _, n := range allNodes {
		if n.ID != sourceNodeID && !sameGroupSet[n.ID] {
			crossGroupNodes = append(crossGroupNodes, n)
		}
	}

	return crossGroupNodes, nil
}

// SelectTargetNodesWithPriority selects target nodes for failover with same-group priority.
// Returns same-group nodes first, then cross-group nodes if needed.
func (fm *FailoverManager) SelectTargetNodesWithPriority(ctx context.Context, sourceNodeID int64, requiredCount int) ([]*repository.Node, bool, error) {
	var result []*repository.Node
	crossGroupUsed := false

	// First, try to get same-group nodes
	if fm.config.PreferSameGroup {
		sameGroupNodes, err := fm.GetSameGroupNodes(ctx, sourceNodeID)
		if err == nil && len(sameGroupNodes) > 0 {
			result = append(result, sameGroupNodes...)
		}
	}

	// If we don't have enough nodes and cross-group is allowed
	if len(result) < requiredCount && fm.config.AllowCrossGroupFailover {
		crossGroupNodes, err := fm.GetCrossGroupNodes(ctx, sourceNodeID)
		if err == nil && len(crossGroupNodes) > 0 {
			// Add cross-group nodes
			for _, n := range crossGroupNodes {
				if len(result) >= requiredCount {
					break
				}
				// Check if not already in result
				found := false
				for _, existing := range result {
					if existing.ID == n.ID {
						found = true
						break
					}
				}
				if !found {
					result = append(result, n)
					crossGroupUsed = true
				}
			}
		}
	}

	if len(result) == 0 {
		return nil, false, ErrNoTargetNodes
	}

	return result, crossGroupUsed, nil
}

// IsSameGroupNode checks if two nodes share at least one common group.
func (fm *FailoverManager) IsSameGroupNode(ctx context.Context, nodeID1, nodeID2 int64) (bool, error) {
	groups1, err := fm.groupRepo.GetGroupIDsForNode(ctx, nodeID1)
	if err != nil {
		return false, err
	}

	groups2, err := fm.groupRepo.GetGroupIDsForNode(ctx, nodeID2)
	if err != nil {
		return false, err
	}

	// Check for common groups
	groupSet := make(map[int64]bool)
	for _, g := range groups1 {
		groupSet[g] = true
	}

	for _, g := range groups2 {
		if groupSet[g] {
			return true, nil
		}
	}

	return false, nil
}


// SetMaxConcurrentMigrations sets the maximum number of concurrent migrations.
func (fm *FailoverManager) SetMaxConcurrentMigrations(max int) {
	if max < 1 {
		max = 1
	}
	fm.config.MaxConcurrentMigrations = max
}

// GetMaxConcurrentMigrations returns the maximum number of concurrent migrations.
func (fm *FailoverManager) GetMaxConcurrentMigrations() int {
	return fm.config.MaxConcurrentMigrations
}

// WaitForMigrationSlot waits until a migration slot is available.
// Returns an error if the context is cancelled.
func (fm *FailoverManager) WaitForMigrationSlot(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			current := atomic.LoadInt32(&fm.currentMigrations)
			if int(current) < fm.config.MaxConcurrentMigrations {
				return nil
			}
			// Small sleep to avoid busy waiting
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Continue checking
			}
		}
	}
}

// TryAcquireMigrationSlot attempts to acquire a migration slot without blocking.
// Returns true if a slot was acquired, false otherwise.
func (fm *FailoverManager) TryAcquireMigrationSlot() bool {
	for {
		current := atomic.LoadInt32(&fm.currentMigrations)
		if int(current) >= fm.config.MaxConcurrentMigrations {
			return false
		}
		if atomic.CompareAndSwapInt32(&fm.currentMigrations, current, current+1) {
			return true
		}
	}
}

// ReleaseMigrationSlot releases a migration slot.
func (fm *FailoverManager) ReleaseMigrationSlot() {
	atomic.AddInt32(&fm.currentMigrations, -1)
}

// MigrateUsersWithConcurrencyLimit migrates users with explicit concurrency control.
// This method provides more control over the migration process.
func (fm *FailoverManager) MigrateUsersWithConcurrencyLimit(
	ctx context.Context,
	userIDs []int64,
	targetNodes []*repository.Node,
	maxConcurrent int,
) []*MigrationResult {
	if maxConcurrent <= 0 {
		maxConcurrent = fm.config.MaxConcurrentMigrations
	}

	results := make([]*MigrationResult, len(userIDs))
	
	// Create a semaphore for concurrency control
	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var resultsMu sync.Mutex

	for i, userID := range userIDs {
		wg.Add(1)
		go func(idx int, uid int64) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				// Got a slot
			case <-ctx.Done():
				resultsMu.Lock()
				results[idx] = &MigrationResult{
					UserID:  uid,
					Success: false,
					Error:   ctx.Err(),
				}
				resultsMu.Unlock()
				return
			}

			atomic.AddInt32(&fm.currentMigrations, 1)
			defer func() {
				<-sem
				atomic.AddInt32(&fm.currentMigrations, -1)
			}()

			// Select target node using round-robin
			targetNode := targetNodes[idx%len(targetNodes)]

			// Perform migration with timeout
			result := fm.migrateUser(ctx, uid, targetNode.ID)
			result.TargetNodeID = targetNode.ID

			resultsMu.Lock()
			results[idx] = result
			resultsMu.Unlock()
		}(i, userID)
	}

	wg.Wait()
	return results
}


// TriggerCrossGroupFailover triggers failover specifically using cross-group nodes.
// This is useful when all nodes in the same group are unhealthy.
func (fm *FailoverManager) TriggerCrossGroupFailover(ctx context.Context, nodeID int64) (*FailoverEvent, error) {
	// Check if failover is already in progress for this node
	fm.activeFailoversMu.Lock()
	if fm.activeFailovers[nodeID] {
		fm.activeFailoversMu.Unlock()
		return nil, ErrFailoverInProgress
	}
	fm.activeFailovers[nodeID] = true
	fm.activeFailoversMu.Unlock()

	defer func() {
		fm.activeFailoversMu.Lock()
		delete(fm.activeFailovers, nodeID)
		fm.activeFailoversMu.Unlock()
	}()

	// Get the node
	node, err := fm.nodeRepo.GetByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	event := &FailoverEvent{
		NodeID:         nodeID,
		NodeName:       node.Name,
		StartedAt:      time.Now(),
		Reason:         "Cross-group failover - all same-group nodes unhealthy",
		CrossGroupUsed: true,
	}

	// Get users assigned to this node
	userIDs, err := fm.assignmentRepo.GetUserIDsByNodeID(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	event.AffectedUsers = len(userIDs)

	if len(userIDs) == 0 {
		event.CompletedAt = time.Now()
		return event, nil
	}

	// Get cross-group nodes only
	crossGroupNodes, err := fm.GetCrossGroupNodes(ctx, nodeID)
	if err != nil {
		return nil, err
	}

	if len(crossGroupNodes) == 0 {
		return nil, ErrNoTargetNodes
	}

	for _, n := range crossGroupNodes {
		event.TargetNodes = append(event.TargetNodes, n.ID)
	}

	// Migrate users
	results := fm.migrateUsers(ctx, userIDs, crossGroupNodes)

	for _, result := range results {
		if result.Success {
			event.MigratedUsers++
		} else {
			event.FailedUsers++
		}
	}

	event.CompletedAt = time.Now()

	fm.logger.Info("Cross-group failover completed",
		logger.F("node_id", nodeID),
		logger.F("node_name", node.Name),
		logger.F("affected_users", event.AffectedUsers),
		logger.F("migrated_users", event.MigratedUsers),
		logger.F("failed_users", event.FailedUsers))

	fm.notifyFailoverComplete(event)

	return event, nil
}

// CheckGroupHealth checks if all nodes in a group are unhealthy.
// Returns true if all nodes are unhealthy, false otherwise.
func (fm *FailoverManager) CheckGroupHealth(ctx context.Context, groupID int64) (bool, error) {
	nodes, err := fm.groupRepo.GetNodes(ctx, groupID)
	if err != nil {
		return false, err
	}

	if len(nodes) == 0 {
		return true, nil // Empty group is considered "all unhealthy"
	}

	for _, n := range nodes {
		if n.Status == repository.NodeStatusOnline {
			return false, nil // At least one healthy node
		}
	}

	return true, nil // All nodes are unhealthy
}

// GetGroupsWithAllUnhealthyNodes returns groups where all nodes are unhealthy.
func (fm *FailoverManager) GetGroupsWithAllUnhealthyNodes(ctx context.Context) ([]int64, error) {
	groups, err := fm.groupRepo.List(ctx, 0, 0)
	if err != nil {
		return nil, err
	}

	var unhealthyGroups []int64
	for _, g := range groups {
		allUnhealthy, err := fm.CheckGroupHealth(ctx, g.ID)
		if err != nil {
			continue
		}
		if allUnhealthy {
			unhealthyGroups = append(unhealthyGroups, g.ID)
		}
	}

	return unhealthyGroups, nil
}

// ShouldUseCrossGroupFailover determines if cross-group failover should be used.
// Returns true if same-group nodes are unavailable and cross-group is allowed.
func (fm *FailoverManager) ShouldUseCrossGroupFailover(ctx context.Context, nodeID int64) (bool, error) {
	if !fm.config.AllowCrossGroupFailover {
		return false, nil
	}

	// Check if there are any healthy same-group nodes
	sameGroupNodes, err := fm.GetSameGroupNodes(ctx, nodeID)
	if err != nil {
		return true, nil // If we can't determine, allow cross-group
	}

	return len(sameGroupNodes) == 0, nil
}

// TriggerFailoverWithStrategy triggers failover with explicit strategy selection.
func (fm *FailoverManager) TriggerFailoverWithStrategy(ctx context.Context, nodeID int64, preferSameGroup bool, allowCrossGroup bool) (*FailoverEvent, error) {
	// Temporarily override config
	originalPreferSameGroup := fm.config.PreferSameGroup
	originalAllowCrossGroup := fm.config.AllowCrossGroupFailover
	
	fm.config.PreferSameGroup = preferSameGroup
	fm.config.AllowCrossGroupFailover = allowCrossGroup
	
	defer func() {
		fm.config.PreferSameGroup = originalPreferSameGroup
		fm.config.AllowCrossGroupFailover = originalAllowCrossGroup
	}()

	return fm.TriggerFailover(ctx, nodeID)
}
