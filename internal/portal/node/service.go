// Package node provides node/proxy services for the user portal.
package node

import (
	"context"
	"sort"
	"strings"

	"v/internal/database/repository"
	proxylib "v/internal/proxy"
)

// Service provides node operations for the user portal.
type Service struct {
	proxyRepo repository.ProxyRepository
	userRepo  repository.UserRepository
	nodeRepo  repository.NodeRepository
}

// NewService creates a new node service.
func NewService(proxyRepo repository.ProxyRepository, userRepo repository.UserRepository, nodeRepo repository.NodeRepository) *Service {
	return &Service{
		proxyRepo: proxyRepo,
		userRepo:  userRepo,
		nodeRepo:  nodeRepo,
	}
}

// Node represents a proxy node for the user portal.
type Node struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Region   string `json:"region"`
	Status   string `json:"status"` // online, offline, maintenance
	Load     int    `json:"load"`   // 0-100 percentage
	Latency  int    `json:"latency,omitempty"` // milliseconds, -1 if not tested
}

// NodeFilter represents filter options for listing nodes.
type NodeFilter struct {
	Region   string
	Protocol string
}

// SortOption represents sorting options for nodes.
type SortOption struct {
	Field string // name, region, latency, load
	Order string // asc, desc
}

// ListNodes retrieves available nodes for a user with optional filtering.
func (s *Service) ListNodes(ctx context.Context, userID int64, filter *NodeFilter) ([]*Node, error) {
	// Get user's proxies
	proxies, err := s.proxyRepo.GetByUserID(ctx, userID, 1000, 0)
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(proxies))
	for _, p := range proxies {
		if !p.Enabled {
			continue
		}

		node := s.proxyToNode(ctx, p)

		// Apply filters
		if filter != nil {
			if filter.Region != "" && !strings.EqualFold(node.Region, filter.Region) {
				continue
			}
			if filter.Protocol != "" && !strings.EqualFold(node.Protocol, filter.Protocol) {
				continue
			}
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// ListAllNodes retrieves all enabled nodes (for users with access to all nodes).
func (s *Service) ListAllNodes(ctx context.Context, filter *NodeFilter) ([]*Node, error) {
	proxies, err := s.proxyRepo.GetEnabled(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*Node, 0, len(proxies))
	for _, p := range proxies {
		node := s.proxyToNode(ctx, p)

		// Apply filters
		if filter != nil {
			if filter.Region != "" && !strings.EqualFold(node.Region, filter.Region) {
				continue
			}
			if filter.Protocol != "" && !strings.EqualFold(node.Protocol, filter.Protocol) {
				continue
			}
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

// GetNode retrieves a single node by ID.
func (s *Service) GetNode(ctx context.Context, id int64) (*Node, error) {
	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.proxyToNode(ctx, proxy), nil
}

// SortNodes sorts nodes by the specified criteria.
func SortNodes(nodes []*Node, sortOpt *SortOption) []*Node {
	if sortOpt == nil || sortOpt.Field == "" {
		return nodes
	}

	// Create a copy to avoid modifying the original slice
	sorted := make([]*Node, len(nodes))
	copy(sorted, nodes)

	ascending := sortOpt.Order != "desc"

	sort.Slice(sorted, func(i, j int) bool {
		var less bool
		switch sortOpt.Field {
		case "name":
			less = strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
		case "region":
			less = strings.ToLower(sorted[i].Region) < strings.ToLower(sorted[j].Region)
		case "latency":
			// -1 means not tested, should be at the end
			if sorted[i].Latency == -1 && sorted[j].Latency == -1 {
				less = false
			} else if sorted[i].Latency == -1 {
				less = false
			} else if sorted[j].Latency == -1 {
				less = true
			} else {
				less = sorted[i].Latency < sorted[j].Latency
			}
		case "load":
			less = sorted[i].Load < sorted[j].Load
		case "protocol":
			less = strings.ToLower(sorted[i].Protocol) < strings.ToLower(sorted[j].Protocol)
		default:
			less = sorted[i].ID < sorted[j].ID
		}

		if ascending {
			return less
		}
		return !less
	})

	return sorted
}

// FilterNodes filters nodes by the specified criteria.
func FilterNodes(nodes []*Node, filter *NodeFilter) []*Node {
	if filter == nil || (filter.Region == "" && filter.Protocol == "") {
		return nodes
	}

	filtered := make([]*Node, 0)
	for _, node := range nodes {
		if filter.Region != "" && !strings.EqualFold(node.Region, filter.Region) {
			continue
		}
		if filter.Protocol != "" && !strings.EqualFold(node.Protocol, filter.Protocol) {
			continue
		}
		filtered = append(filtered, node)
	}

	return filtered
}

// GetAvailableRegions returns unique regions from the node list.
func GetAvailableRegions(nodes []*Node) []string {
	regionSet := make(map[string]bool)
	for _, node := range nodes {
		if node.Region != "" {
			regionSet[node.Region] = true
		}
	}

	regions := make([]string, 0, len(regionSet))
	for region := range regionSet {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// GetAvailableProtocols returns unique protocols from the node list.
func GetAvailableProtocols(nodes []*Node) []string {
	protocolSet := make(map[string]bool)
	for _, node := range nodes {
		if node.Protocol != "" {
			protocolSet[node.Protocol] = true
		}
	}

	protocols := make([]string, 0, len(protocolSet))
	for protocol := range protocolSet {
		protocols = append(protocols, protocol)
	}
	sort.Strings(protocols)
	return protocols
}

// proxyToNode converts a Proxy to a Node and resolves a client-connectable host.
func (s *Service) proxyToNode(ctx context.Context, p *repository.Proxy) *Node {
	resolvedHost := ""
	if p.Settings != nil {
		if explicitServer, ok := p.Settings["server"].(string); ok {
			resolvedHost = proxylib.NormalizeShareHost(explicitServer)
		}
	}
	if resolvedHost == "" && p.NodeID != nil && s.nodeRepo != nil {
		if n, err := s.nodeRepo.GetByID(ctx, *p.NodeID); err == nil {
			resolvedHost = proxylib.NormalizeShareHost(n.Address)
		}
	}
	if resolvedHost == "" {
		resolvedHost = proxylib.ResolveServerAddress(p.Host, p.Settings)
	}
	if resolvedHost == "" {
		resolvedHost = p.Host
	}

	node := &Node{
		ID:       p.ID,
		Name:     p.Name,
		Protocol: p.Protocol,
		Host:     resolvedHost,
		Port:     p.Port,
		Status:   "online", // Default to online if enabled
		Load:     0,
		Latency:  -1, // Not tested
	}

	// Extract region from settings if available
	if p.Settings != nil {
		if region, ok := p.Settings["region"].(string); ok {
			node.Region = region
		}
		if load, ok := p.Settings["load"].(float64); ok {
			node.Load = int(load)
		}
		if status, ok := p.Settings["status"].(string); ok {
			node.Status = status
		}
	}

	// If no region in settings, try to extract from name or remark
	if node.Region == "" && p.Remark != "" {
		node.Region = p.Remark
	}

	return node
}
