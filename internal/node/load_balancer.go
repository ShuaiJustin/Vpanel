// Package node provides node management functionality for multi-server management.
package node

import (
	"context"
	"errors"
	"math"
	"sort"
	"sync"
	"sync/atomic"

	"v/internal/database/repository"
	"v/internal/ip"
	"v/internal/logger"
)

// Load balancer errors
var (
	ErrNoAvailableNodes = errors.New("no available nodes")
	ErrInvalidStrategy  = errors.New("invalid load balancing strategy")
	ErrUserNotAssigned  = errors.New("user not assigned to any node")
)

// Strategy constants for load balancing
const (
	StrategyRoundRobin       = "round-robin"
	StrategyLeastConnections = "least-connections"
	StrategyWeighted         = "weighted"
	StrategyGeographic       = "geographic"
)

// SelectOptions defines options for node selection
type SelectOptions struct {
	Strategy   string  // Load balancing strategy
	GroupID    *int64  // Limit to specific group
	ExcludeIDs []int64 // Nodes to exclude
	UserIP     string  // User IP for geographic strategy
	Sticky     bool    // Maintain user-node affinity
}

// BalanceStrategy defines the interface for load balancing strategies
type BalanceStrategy interface {
	// Select selects a node from the available nodes
	Select(ctx context.Context, nodes []*Node, opts *SelectOptions) (*Node, error)
	// Name returns the strategy name
	Name() string
}

// LoadBalancer provides load balancing functionality for node selection
type LoadBalancer struct {
	nodeRepo       repository.NodeRepository
	groupRepo      repository.NodeGroupRepository
	assignmentRepo repository.UserNodeAssignmentRepository
	geoService     *ip.GeolocationService
	logger         logger.Logger

	strategies map[string]BalanceStrategy
	mu         sync.RWMutex
}

// NewLoadBalancer creates a new load balancer
func NewLoadBalancer(
	nodeRepo repository.NodeRepository,
	groupRepo repository.NodeGroupRepository,
	assignmentRepo repository.UserNodeAssignmentRepository,
	geoService *ip.GeolocationService,
	log logger.Logger,
) *LoadBalancer {
	lb := &LoadBalancer{
		nodeRepo:       nodeRepo,
		groupRepo:      groupRepo,
		assignmentRepo: assignmentRepo,
		geoService:     geoService,
		logger:         log,
		strategies:     make(map[string]BalanceStrategy),
	}

	// Register default strategies
	lb.RegisterStrategy(NewRoundRobinStrategy())
	lb.RegisterStrategy(NewLeastConnectionsStrategy())
	lb.RegisterStrategy(NewWeightedStrategy())
	lb.RegisterStrategy(NewGeographicStrategy(geoService))

	return lb
}

// RegisterStrategy registers a load balancing strategy
func (lb *LoadBalancer) RegisterStrategy(strategy BalanceStrategy) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.strategies[strategy.Name()] = strategy
}

// GetStrategy returns a strategy by name
func (lb *LoadBalancer) GetStrategy(name string) (BalanceStrategy, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	strategy, ok := lb.strategies[name]
	if !ok {
		return nil, ErrInvalidStrategy
	}
	return strategy, nil
}

// SelectNode selects a node for a user based on the configured strategy
func (lb *LoadBalancer) SelectNode(ctx context.Context, userID int64, opts *SelectOptions) (*Node, error) {
	if opts == nil {
		opts = &SelectOptions{Strategy: StrategyRoundRobin}
	}

	// Check for sticky session - return existing assignment if enabled
	if opts.Sticky {
		existingNode, err := lb.GetUserNode(ctx, userID)
		if err == nil && existingNode != nil && existingNode.Status == repository.NodeStatusOnline {
			// Check if node is not at capacity
			if existingNode.MaxUsers == 0 || existingNode.CurrentUsers < existingNode.MaxUsers {
				return existingNode, nil
			}
		}
	}

	// Get available nodes
	nodes, err := lb.getAvailableNodes(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	// Get strategy
	strategyName := opts.Strategy
	if strategyName == "" {
		strategyName = StrategyRoundRobin
	}

	strategy, err := lb.GetStrategy(strategyName)
	if err != nil {
		lb.logger.Warn("Invalid strategy, falling back to round-robin",
			logger.F("strategy", strategyName))
		strategy, _ = lb.GetStrategy(StrategyRoundRobin)
	}

	// Select node using strategy
	selectedNode, err := strategy.Select(ctx, nodes, opts)
	if err != nil {
		return nil, err
	}

	return selectedNode, nil
}

// SelectNodes selects multiple nodes for a user
func (lb *LoadBalancer) SelectNodes(ctx context.Context, userID int64, count int, opts *SelectOptions) ([]*Node, error) {
	if opts == nil {
		opts = &SelectOptions{Strategy: StrategyRoundRobin}
	}

	// Get available nodes
	nodes, err := lb.getAvailableNodes(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	// Limit count to available nodes
	if count > len(nodes) {
		count = len(nodes)
	}

	// Get strategy
	strategyName := opts.Strategy
	if strategyName == "" {
		strategyName = StrategyRoundRobin
	}

	strategy, err := lb.GetStrategy(strategyName)
	if err != nil {
		strategy, _ = lb.GetStrategy(StrategyRoundRobin)
	}

	// Select nodes
	selectedNodes := make([]*Node, 0, count)
	excludeIDs := make([]int64, len(opts.ExcludeIDs))
	copy(excludeIDs, opts.ExcludeIDs)

	for i := 0; i < count; i++ {
		// Create options with updated exclude list
		selectOpts := &SelectOptions{
			Strategy:   opts.Strategy,
			GroupID:    opts.GroupID,
			ExcludeIDs: excludeIDs,
			UserIP:     opts.UserIP,
			Sticky:     false, // Don't use sticky for multiple selection
		}

		// Filter nodes
		filteredNodes := filterNodes(nodes, excludeIDs)
		if len(filteredNodes) == 0 {
			break
		}

		node, err := strategy.Select(ctx, filteredNodes, selectOpts)
		if err != nil {
			break
		}

		selectedNodes = append(selectedNodes, node)
		excludeIDs = append(excludeIDs, node.ID)
	}

	if len(selectedNodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	return selectedNodes, nil
}

// AssignUser assigns a user to a node
func (lb *LoadBalancer) AssignUser(ctx context.Context, userID, nodeID int64) error {
	return lb.assignmentRepo.Assign(ctx, userID, nodeID)
}

// UnassignUser removes a user's node assignment
func (lb *LoadBalancer) UnassignUser(ctx context.Context, userID int64) error {
	return lb.assignmentRepo.Unassign(ctx, userID)
}

// GetUserNode returns the node assigned to a user
func (lb *LoadBalancer) GetUserNode(ctx context.Context, userID int64) (*Node, error) {
	assignment, err := lb.assignmentRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if assignment == nil {
		return nil, ErrUserNotAssigned
	}

	repoNode, err := lb.nodeRepo.GetByID(ctx, assignment.NodeID)
	if err != nil {
		return nil, err
	}

	return repoNodeToNode(repoNode), nil
}

func sortNodesBySelectionPriority(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		return shouldPreferNodeForSelection(nodes[i], nodes[j])
	})
}

func shouldPreferNodeForSelection(left, right *Node) bool {
	if left == nil {
		return false
	}
	if right == nil {
		return true
	}

	leftTrafficPressure := nodeSelectionTrafficPressure(left)
	rightTrafficPressure := nodeSelectionTrafficPressure(right)
	if leftTrafficPressure != rightTrafficPressure {
		return leftTrafficPressure < rightTrafficPressure
	}

	leftCapacityPressure := nodeSelectionCapacityPressure(left)
	rightCapacityPressure := nodeSelectionCapacityPressure(right)
	if leftCapacityPressure != rightCapacityPressure {
		return leftCapacityPressure < rightCapacityPressure
	}

	if left.CurrentUsers != right.CurrentUsers {
		return left.CurrentUsers < right.CurrentUsers
	}
	if left.Weight != right.Weight {
		return left.Weight > right.Weight
	}
	return left.ID < right.ID
}

func nodeSelectionTrafficPressure(node *Node) float64 {
	if node == nil {
		return 1e9
	}
	if node.TrafficLimit <= 0 {
		return -1
	}
	threshold := node.AlertTrafficThreshold
	if threshold < 1 || threshold > 100 {
		threshold = 100
	}
	return (float64(node.TrafficTotal) * 100 / float64(node.TrafficLimit)) / threshold
}

func nodeSelectionCapacityPressure(node *Node) float64 {
	if node == nil {
		return 1e9
	}
	if node.MaxUsers <= 0 {
		return 0
	}
	return float64(node.CurrentUsers) / float64(node.MaxUsers)
}

func effectiveNodeSelectionWeight(node *Node) int {
	if node == nil {
		return 1
	}
	weight := node.Weight
	if weight <= 0 {
		weight = 1
	}
	if node.TrafficLimit <= 0 {
		return weight
	}

	remainingRatio := float64(node.TrafficLimit-node.TrafficTotal) / float64(node.TrafficLimit)
	if remainingRatio < 0 {
		remainingRatio = 0
	}
	if remainingRatio < 0.2 {
		remainingRatio = 0.2
	}

	effective := int(math.Round(float64(weight) * remainingRatio))
	if effective < 1 {
		effective = 1
	}
	return effective
}

// Rebalance redistributes users across nodes in a group
func (lb *LoadBalancer) Rebalance(ctx context.Context, groupID int64) error {
	// Get nodes in group
	nodes, err := lb.groupRepo.GetNodes(ctx, groupID)
	if err != nil {
		return err
	}

	// Filter to available nodes
	var availableNodes []*repository.Node
	for _, n := range nodes {
		if n.Status == repository.NodeStatusOnline {
			if n.MaxUsers == 0 || n.CurrentUsers < n.MaxUsers {
				availableNodes = append(availableNodes, n)
			}
		}
	}

	if len(availableNodes) == 0 {
		return ErrNoAvailableNodes
	}

	// Get all users assigned to nodes in this group
	var allUserIDs []int64
	for _, n := range nodes {
		userIDs, err := lb.assignmentRepo.GetUserIDsByNodeID(ctx, n.ID)
		if err != nil {
			continue
		}
		allUserIDs = append(allUserIDs, userIDs...)
	}

	// Redistribute users using round-robin
	for i, userID := range allUserIDs {
		targetNode := availableNodes[i%len(availableNodes)]
		if err := lb.assignmentRepo.Reassign(ctx, userID, targetNode.ID); err != nil {
			lb.logger.Error("Failed to reassign user during rebalance",
				logger.Err(err),
				logger.F("user_id", userID),
				logger.F("target_node_id", targetNode.ID))
		}
	}

	lb.logger.Info("Rebalanced users in group",
		logger.F("group_id", groupID),
		logger.F("user_count", len(allUserIDs)),
		logger.F("node_count", len(availableNodes)))

	return nil
}

// getAvailableNodes returns nodes that are available for selection
func (lb *LoadBalancer) getAvailableNodes(ctx context.Context, opts *SelectOptions) ([]*Node, error) {
	var repoNodes []*repository.Node
	var err error

	if opts.GroupID != nil {
		// Get nodes from specific group
		repoNodes, err = lb.groupRepo.GetNodes(ctx, *opts.GroupID)
		if err != nil {
			return nil, err
		}
	} else {
		// Get all available nodes
		repoNodes, err = lb.nodeRepo.GetAvailable(ctx)
		if err != nil {
			return nil, err
		}
	}

	// Filter nodes
	nodes := make([]*Node, 0, len(repoNodes))
	for _, rn := range repoNodes {
		// Skip offline/unhealthy nodes
		if rn.Status != repository.NodeStatusOnline {
			continue
		}

		// Skip nodes at capacity
		if rn.MaxUsers > 0 && rn.CurrentUsers >= rn.MaxUsers {
			continue
		}

		// Skip excluded nodes
		if containsID(opts.ExcludeIDs, rn.ID) {
			continue
		}

		nodes = append(nodes, repoNodeToNode(rn))
	}

	sortNodesBySelectionPriority(nodes)
	return nodes, nil
}

// filterNodes filters nodes by excluding specified IDs
func filterNodes(nodes []*Node, excludeIDs []int64) []*Node {
	if len(excludeIDs) == 0 {
		return nodes
	}

	filtered := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		if !containsID(excludeIDs, n.ID) {
			filtered = append(filtered, n)
		}
	}
	return filtered
}

// containsID checks if an ID is in the slice
func containsID(ids []int64, id int64) bool {
	for _, i := range ids {
		if i == id {
			return true
		}
	}
	return false
}

// repoNodeToNode converts a repository node to a service node
func repoNodeToNode(rn *repository.Node) *Node {
	return &Node{
		ID:                    rn.ID,
		Name:                  rn.Name,
		Address:               rn.Address,
		Port:                  rn.Port,
		Token:                 rn.Token,
		Status:                rn.Status,
		Region:                rn.Region,
		Weight:                rn.Weight,
		MaxUsers:              rn.MaxUsers,
		CurrentUsers:          rn.CurrentUsers,
		Latency:               rn.Latency,
		LastSeenAt:            rn.LastSeenAt,
		SyncStatus:            rn.SyncStatus,
		SyncedAt:              rn.SyncedAt,
		TrafficTotal:          rn.TrafficTotal,
		TrafficLimit:          rn.TrafficLimit,
		TrafficResetAt:        rn.TrafficResetAt,
		AlertTrafficThreshold: rn.AlertTrafficThreshold,
		CreatedAt:             rn.CreatedAt,
		UpdatedAt:             rn.UpdatedAt,
	}
}

// ============================================
// Round Robin Strategy
// ============================================

// RoundRobinStrategy implements round-robin load balancing
type RoundRobinStrategy struct {
	counter uint64
}

// NewRoundRobinStrategy creates a new round-robin strategy
func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

// Name returns the strategy name
func (s *RoundRobinStrategy) Name() string {
	return StrategyRoundRobin
}

// Select selects a node using round-robin
func (s *RoundRobinStrategy) Select(ctx context.Context, nodes []*Node, opts *SelectOptions) (*Node, error) {
	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	// Atomic increment and select
	idx := atomic.AddUint64(&s.counter, 1) - 1
	return nodes[idx%uint64(len(nodes))], nil
}

// ============================================
// Least Connections Strategy
// ============================================

// LeastConnectionsStrategy implements least-connections load balancing
type LeastConnectionsStrategy struct{}

// NewLeastConnectionsStrategy creates a new least-connections strategy
func NewLeastConnectionsStrategy() *LeastConnectionsStrategy {
	return &LeastConnectionsStrategy{}
}

// Name returns the strategy name
func (s *LeastConnectionsStrategy) Name() string {
	return StrategyLeastConnections
}

// Select selects the node with the fewest current users
func (s *LeastConnectionsStrategy) Select(ctx context.Context, nodes []*Node, opts *SelectOptions) (*Node, error) {
	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	// Find node with minimum connections
	minNode := nodes[0]
	minConnections := nodes[0].CurrentUsers

	for _, n := range nodes[1:] {
		if n.CurrentUsers < minConnections {
			minConnections = n.CurrentUsers
			minNode = n
			continue
		}
		if n.CurrentUsers == minConnections && shouldPreferNodeForSelection(n, minNode) {
			minNode = n
		}
	}

	return minNode, nil
}

// ============================================
// Weighted Strategy
// ============================================

// WeightedStrategy implements weighted load balancing
type WeightedStrategy struct {
	mu      sync.Mutex
	weights map[int64]int // nodeID -> current weight
}

// NewWeightedStrategy creates a new weighted strategy
func NewWeightedStrategy() *WeightedStrategy {
	return &WeightedStrategy{
		weights: make(map[int64]int),
	}
}

// Name returns the strategy name
func (s *WeightedStrategy) Name() string {
	return StrategyWeighted
}

// Select selects a node based on weights using weighted round-robin
func (s *WeightedStrategy) Select(ctx context.Context, nodes []*Node, opts *SelectOptions) (*Node, error) {
	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Calculate total weight and initialize current weights
	totalWeight := 0
	for _, n := range nodes {
		weight := effectiveNodeSelectionWeight(n)
		totalWeight += weight

		// Initialize weight if not exists
		if _, ok := s.weights[n.ID]; !ok {
			s.weights[n.ID] = 0
		}
	}

	// Weighted round-robin algorithm (smooth weighted round-robin)
	var selectedNode *Node
	maxWeight := -1

	for _, n := range nodes {
		weight := effectiveNodeSelectionWeight(n)

		// Add effective weight
		s.weights[n.ID] += weight

		// Select node with highest current weight
		if selectedNode == nil || s.weights[n.ID] > maxWeight || (s.weights[n.ID] == maxWeight && shouldPreferNodeForSelection(n, selectedNode)) {
			maxWeight = s.weights[n.ID]
			selectedNode = n
		}
	}

	// Reduce selected node's weight by total weight
	if selectedNode != nil {
		s.weights[selectedNode.ID] -= totalWeight
	}

	return selectedNode, nil
}

// ============================================
// Geographic Strategy
// ============================================

// GeographicStrategy implements geographic-based load balancing
type GeographicStrategy struct {
	geoService *ip.GeolocationService
}

// NewGeographicStrategy creates a new geographic strategy
func NewGeographicStrategy(geoService *ip.GeolocationService) *GeographicStrategy {
	return &GeographicStrategy{
		geoService: geoService,
	}
}

// Name returns the strategy name
func (s *GeographicStrategy) Name() string {
	return StrategyGeographic
}

// Select selects the node closest to the user's location
func (s *GeographicStrategy) Select(ctx context.Context, nodes []*Node, opts *SelectOptions) (*Node, error) {
	if len(nodes) == 0 {
		return nil, ErrNoAvailableNodes
	}

	// If no user IP or geo service unavailable, fall back to round-robin
	if opts == nil || opts.UserIP == "" || s.geoService == nil || !s.geoService.IsAvailable() {
		return nodes[0], nil
	}

	// Get user's location
	userGeo, err := s.geoService.Lookup(ctx, opts.UserIP)
	if err != nil || userGeo == nil || (userGeo.Latitude == 0 && userGeo.Longitude == 0) {
		// Fall back to first node if geo lookup fails
		return nodes[0], nil
	}

	// Find closest node
	var closestNode *Node
	minDistance := math.MaxFloat64

	for _, n := range nodes {
		// Get node's location from region (simplified - in production, store lat/long in node)
		nodeGeo := s.getNodeLocation(ctx, n)
		if nodeGeo == nil {
			continue
		}

		distance := haversineDistance(userGeo.Latitude, userGeo.Longitude, nodeGeo.Latitude, nodeGeo.Longitude)
		if distance < minDistance {
			minDistance = distance
			closestNode = n
		}
	}

	if closestNode == nil {
		return nodes[0], nil
	}

	return closestNode, nil
}

// getNodeLocation returns the geographic location of a node
func (s *GeographicStrategy) getNodeLocation(ctx context.Context, node *Node) *ip.GeoInfo {
	// Try to get location from node's address
	if s.geoService != nil && s.geoService.IsAvailable() {
		geo, err := s.geoService.Lookup(ctx, node.Address)
		if err == nil && geo != nil {
			return geo
		}
	}

	// Return nil if we can't determine location
	return nil
}

// haversineDistance calculates the distance between two points on Earth in kilometers
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // km

	lat1Rad := lat1 * math.Pi / 180
	lat2Rad := lat2 * math.Pi / 180
	deltaLat := (lat2 - lat1) * math.Pi / 180
	deltaLon := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}
