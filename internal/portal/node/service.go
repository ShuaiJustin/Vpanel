// Package node provides node/proxy services for the user portal.
package node

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"v/internal/database/repository"
	"v/internal/entitlement"
	proxylib "v/internal/proxy"
)

// Service provides node operations for the user portal.
type Service struct {
	proxyRepo   repository.ProxyRepository
	userRepo    repository.UserRepository
	nodeRepo    repository.NodeRepository
	entitlement *entitlement.Service
}

// NewService creates a new node service.
func NewService(proxyRepo repository.ProxyRepository, userRepo repository.UserRepository, nodeRepo repository.NodeRepository) *Service {
	return &Service{
		proxyRepo: proxyRepo,
		userRepo:  userRepo,
		nodeRepo:  nodeRepo,
	}
}

// WithEntitlementService injects user entitlement logic for node access.
func (s *Service) WithEntitlementService(entitlementService *entitlement.Service) *Service {
	s.entitlement = entitlementService
	return s
}

// Node represents a proxy node for the user portal.
type Node struct {
	ID                    int64   `json:"id"`
	Name                  string  `json:"name"`
	Subtitle              string  `json:"subtitle,omitempty"`
	Protocol              string  `json:"protocol"`
	ProtocolLabel         string  `json:"protocol_label,omitempty"`
	Host                  string  `json:"host"`
	Port                  int     `json:"port"`
	Region                string  `json:"region"`
	RegionLabel           string  `json:"region_label,omitempty"`
	Status                string  `json:"status"`            // online, offline, unhealthy
	Load                  int     `json:"load"`              // 0-100 percentage
	Latency               int     `json:"latency,omitempty"` // milliseconds, -1 if not tested
	TrafficTotal          int64   `json:"traffic_total,omitempty"`
	TrafficLimit          int64   `json:"traffic_limit,omitempty"`
	TrafficResetAt        string  `json:"traffic_reset_at,omitempty"`
	AlertTrafficThreshold float64 `json:"alert_traffic_threshold,omitempty"`
	NodeID                *int64  `json:"-"` // underlying deployed node ID (not exposed to portal clients)
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
	var err error
	var proxies []*repository.Proxy
	if s.entitlement != nil {
		proxies, _, err = s.entitlement.GetAccessibleProxies(ctx, userID)
		if err != nil {
			return nil, err
		}
	} else {
		proxies, err = s.proxyRepo.GetByUserID(ctx, userID, 1000, 0)
		if err != nil {
			return nil, err
		}
		if len(proxies) == 0 {
			proxies, err = s.proxyRepo.GetEnabled(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	nodeMap := s.loadNodeMap(ctx, proxies)

	nodes := make([]*Node, 0, len(proxies))
	for _, p := range proxies {
		if !p.Enabled {
			continue
		}

		node := s.proxyToNode(p, nodeMap)

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

	nodeMap := s.loadNodeMap(ctx, proxies)

	nodes := make([]*Node, 0, len(proxies))
	for _, p := range proxies {
		node := s.proxyToNode(p, nodeMap)

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
func (s *Service) GetNode(ctx context.Context, userID, id int64) (*Node, error) {
	if s.entitlement != nil {
		proxy, _, err := s.entitlement.GetAccessibleProxy(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		return s.proxyToNode(proxy, s.loadNodeMap(ctx, []*repository.Proxy{proxy})), nil
	}

	proxy, err := s.proxyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.proxyToNode(proxy, s.loadNodeMap(ctx, []*repository.Proxy{proxy})), nil
}

// loadNodeMap batch-loads node records referenced by the given proxies in a
// single query, returning a map keyed by node ID. This replaces the previous
// per-proxy GetByID lookup (an N+1 query) inside proxyToNode.
func (s *Service) loadNodeMap(ctx context.Context, proxies []*repository.Proxy) map[int64]*repository.Node {
	if s.nodeRepo == nil || len(proxies) == 0 {
		return nil
	}

	seen := make(map[int64]struct{}, len(proxies))
	ids := make([]int64, 0, len(proxies))
	for _, p := range proxies {
		if p == nil || p.NodeID == nil {
			continue
		}
		id := *p.NodeID
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	nodes, err := s.nodeRepo.GetByIDs(ctx, ids)
	if err != nil || len(nodes) == 0 {
		return nil
	}

	out := make(map[int64]*repository.Node, len(nodes))
	for _, n := range nodes {
		if n != nil {
			out[n.ID] = n
		}
	}
	return out
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

func isGenericPortalProxyRemark(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "auto provisioned", "auto-provisioned":
		return true
	default:
		return false
	}
}

func looksGeneratedPortalProxyName(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(normalized, "node-") || isGenericPortalProxyRemark(normalized)
}

func getPortalProtocolLabel(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "vmess":
		return "VMess"
	case "vless":
		return "VLESS"
	case "trojan":
		return "Trojan"
	case "shadowsocks", "ss":
		return "Shadowsocks"
	default:
		return strings.ToUpper(strings.TrimSpace(protocol))
	}
}

func normalizePortalRegionLabel(region string) string {
	normalized := strings.ToLower(strings.TrimSpace(region))
	switch normalized {
	case "hk", "hong kong", "hongkong", "香港":
		return "香港"
	case "tw", "taiwan", "台湾":
		return "台湾"
	case "jp", "japan", "日本":
		return "日本"
	case "sg", "singapore", "新加坡":
		return "新加坡"
	case "us", "usa", "united states", "美国":
		return "美国"
	case "kr", "korea", "south korea", "韩国":
		return "韩国"
	case "de", "germany", "德国":
		return "德国"
	case "uk", "britain", "united kingdom", "英国":
		return "英国"
	case "cn", "china", "中国":
		return "中国"
	default:
		return strings.TrimSpace(region)
	}
}

func buildPortalNodeName(proxyModel *repository.Proxy, nodeModel *repository.Node, protocolLabel, resolvedHost string) string {
	if nodeModel != nil && strings.TrimSpace(nodeModel.Name) != "" {
		return strings.TrimSpace(nodeModel.Name)
	}
	if name := strings.TrimSpace(proxyModel.Name); name != "" && !looksGeneratedPortalProxyName(name) {
		return name
	}
	if remark := strings.TrimSpace(proxyModel.Remark); remark != "" && !isGenericPortalProxyRemark(remark) {
		return remark
	}
	if protocolLabel == "" {
		protocolLabel = "节点"
	}
	if resolvedHost != "" && proxyModel.Port > 0 {
		return fmt.Sprintf("%s · %s:%d", protocolLabel, resolvedHost, proxyModel.Port)
	}
	if resolvedHost != "" {
		return fmt.Sprintf("%s · %s", protocolLabel, resolvedHost)
	}
	return protocolLabel
}

func buildPortalNodeSubtitle(protocolLabel, resolvedHost string, port int) string {
	parts := make([]string, 0, 2)
	if protocolLabel != "" {
		parts = append(parts, protocolLabel)
	}
	if resolvedHost != "" {
		if port > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", resolvedHost, port))
		} else {
			parts = append(parts, resolvedHost)
		}
	}
	return strings.Join(parts, " · ")
}

func resolvePortalNodeLoad(nodeModel *repository.Node) int {
	if nodeModel == nil {
		return 0
	}
	if nodeModel.CPUUsage > 0 {
		load := int(math.Round(nodeModel.CPUUsage))
		if load < 0 {
			return 0
		}
		if load > 100 {
			return 100
		}
		return load
	}
	if nodeModel.MaxUsers > 0 {
		load := int(math.Round(float64(nodeModel.CurrentUsers) * 100 / float64(nodeModel.MaxUsers)))
		if load < 0 {
			return 0
		}
		if load > 100 {
			return 100
		}
		return load
	}
	return 0
}

// proxyToNode converts a Proxy to a Node and resolves a client-connectable host.
// nodeMap is a preloaded map of NodeID → Node (built by loadNodeMap) so this
// function does not hit the database per proxy.
func (s *Service) proxyToNode(p *repository.Proxy, nodeMap map[int64]*repository.Node) *Node {
	resolvedHost := ""
	if p.Settings != nil {
		if explicitServer, ok := p.Settings["server"].(string); ok {
			resolvedHost = proxylib.NormalizeShareHost(explicitServer)
		}
	}

	var nodeModel *repository.Node
	if p.NodeID != nil && nodeMap != nil {
		if n, ok := nodeMap[*p.NodeID]; ok && n != nil {
			nodeModel = n
			if resolvedHost == "" {
				resolvedHost = proxylib.NormalizeShareHost(n.Address)
			}
		}
	}
	if resolvedHost == "" {
		resolvedHost = proxylib.ResolveServerAddress(p.Host, p.Settings)
	}
	if resolvedHost == "" {
		resolvedHost = p.Host
	}

	protocolLabel := getPortalProtocolLabel(p.Protocol)
	node := &Node{
		ID:            p.ID,
		Name:          buildPortalNodeName(p, nodeModel, protocolLabel, resolvedHost),
		Subtitle:      buildPortalNodeSubtitle(protocolLabel, resolvedHost, p.Port),
		Protocol:      p.Protocol,
		ProtocolLabel: protocolLabel,
		Host:          resolvedHost,
		Port:          p.Port,
		Status:        "online",
		Load:          0,
		Latency:       -1,
		NodeID:        p.NodeID,
	}

	if nodeModel != nil {
		if strings.TrimSpace(nodeModel.Region) != "" {
			node.Region = strings.TrimSpace(nodeModel.Region)
		}
		if strings.TrimSpace(nodeModel.Status) != "" {
			node.Status = strings.TrimSpace(nodeModel.Status)
		}
		node.Load = resolvePortalNodeLoad(nodeModel)
		node.TrafficTotal = nodeModel.TrafficTotal
		node.TrafficLimit = nodeModel.TrafficLimit
		node.AlertTrafficThreshold = nodeModel.AlertTrafficThreshold
		if nodeModel.TrafficResetAt != nil {
			node.TrafficResetAt = nodeModel.TrafficResetAt.UTC().Format(time.RFC3339)
		}
	}

	if p.Settings != nil {
		if node.Region == "" {
			if region, ok := p.Settings["region"].(string); ok {
				node.Region = strings.TrimSpace(region)
			}
		}
		if node.Load == 0 {
			if load, ok := p.Settings["load"].(float64); ok {
				node.Load = int(load)
			}
		}
		if node.Status == "online" {
			if status, ok := p.Settings["status"].(string); ok && strings.TrimSpace(status) != "" {
				node.Status = strings.TrimSpace(status)
			}
		}
	}

	if node.Region == "" && p.Remark != "" && !isGenericPortalProxyRemark(p.Remark) {
		node.Region = strings.TrimSpace(p.Remark)
	}
	node.RegionLabel = normalizePortalRegionLabel(node.Region)

	return node
}
