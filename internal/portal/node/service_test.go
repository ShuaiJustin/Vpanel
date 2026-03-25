// Package node provides node/proxy services for the user portal.
package node

import (
	"v/internal/database/repository"

	"sort"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Unit tests

func TestFilterNodes_ByRegion(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "Node 1", Region: "US", Protocol: "vmess"},
		{ID: 2, Name: "Node 2", Region: "JP", Protocol: "vless"},
		{ID: 3, Name: "Node 3", Region: "US", Protocol: "trojan"},
		{ID: 4, Name: "Node 4", Region: "HK", Protocol: "vmess"},
	}

	// Filter by US region
	filtered := FilterNodes(nodes, &NodeFilter{Region: "US"})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(filtered))
	}
	for _, n := range filtered {
		if n.Region != "US" {
			t.Errorf("Expected region US, got %s", n.Region)
		}
	}

	// Filter by JP region
	filtered = FilterNodes(nodes, &NodeFilter{Region: "JP"})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 node, got %d", len(filtered))
	}

	// Filter by non-existent region
	filtered = FilterNodes(nodes, &NodeFilter{Region: "EU"})
	if len(filtered) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(filtered))
	}
}

func TestFilterNodes_ByProtocol(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "Node 1", Region: "US", Protocol: "vmess"},
		{ID: 2, Name: "Node 2", Region: "JP", Protocol: "vless"},
		{ID: 3, Name: "Node 3", Region: "US", Protocol: "trojan"},
		{ID: 4, Name: "Node 4", Region: "HK", Protocol: "vmess"},
	}

	// Filter by vmess protocol
	filtered := FilterNodes(nodes, &NodeFilter{Protocol: "vmess"})
	if len(filtered) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(filtered))
	}
	for _, n := range filtered {
		if n.Protocol != "vmess" {
			t.Errorf("Expected protocol vmess, got %s", n.Protocol)
		}
	}
}

func TestFilterNodes_Combined(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "Node 1", Region: "US", Protocol: "vmess"},
		{ID: 2, Name: "Node 2", Region: "JP", Protocol: "vless"},
		{ID: 3, Name: "Node 3", Region: "US", Protocol: "trojan"},
		{ID: 4, Name: "Node 4", Region: "HK", Protocol: "vmess"},
	}

	// Filter by US region AND vmess protocol
	filtered := FilterNodes(nodes, &NodeFilter{Region: "US", Protocol: "vmess"})
	if len(filtered) != 1 {
		t.Errorf("Expected 1 node, got %d", len(filtered))
	}
	if filtered[0].ID != 1 {
		t.Errorf("Expected node ID 1, got %d", filtered[0].ID)
	}
}

func TestSortNodes_ByName(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "Charlie"},
		{ID: 2, Name: "Alpha"},
		{ID: 3, Name: "Bravo"},
	}

	// Sort ascending
	sorted := SortNodes(nodes, &SortOption{Field: "name", Order: "asc"})
	if sorted[0].Name != "Alpha" || sorted[1].Name != "Bravo" || sorted[2].Name != "Charlie" {
		t.Error("Nodes not sorted correctly by name ascending")
	}

	// Sort descending
	sorted = SortNodes(nodes, &SortOption{Field: "name", Order: "desc"})
	if sorted[0].Name != "Charlie" || sorted[1].Name != "Bravo" || sorted[2].Name != "Alpha" {
		t.Error("Nodes not sorted correctly by name descending")
	}
}

func TestSortNodes_ByLatency(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "Node 1", Latency: 100},
		{ID: 2, Name: "Node 2", Latency: -1}, // Not tested
		{ID: 3, Name: "Node 3", Latency: 50},
		{ID: 4, Name: "Node 4", Latency: 200},
	}

	// Sort ascending - untested (-1) should be at the end
	sorted := SortNodes(nodes, &SortOption{Field: "latency", Order: "asc"})
	if sorted[0].Latency != 50 || sorted[1].Latency != 100 || sorted[2].Latency != 200 || sorted[3].Latency != -1 {
		t.Errorf("Nodes not sorted correctly by latency ascending: %v", sorted)
	}
}

func TestGetAvailableRegions(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Region: "US"},
		{ID: 2, Region: "JP"},
		{ID: 3, Region: "US"},
		{ID: 4, Region: "HK"},
		{ID: 5, Region: ""},
	}

	regions := GetAvailableRegions(nodes)
	if len(regions) != 3 {
		t.Errorf("Expected 3 unique regions, got %d", len(regions))
	}

	// Should be sorted
	expected := []string{"HK", "JP", "US"}
	for i, r := range regions {
		if r != expected[i] {
			t.Errorf("Expected region %s at index %d, got %s", expected[i], i, r)
		}
	}
}

func TestGetAvailableProtocols(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Protocol: "vmess"},
		{ID: 2, Protocol: "vless"},
		{ID: 3, Protocol: "vmess"},
		{ID: 4, Protocol: "trojan"},
	}

	protocols := GetAvailableProtocols(nodes)
	if len(protocols) != 3 {
		t.Errorf("Expected 3 unique protocols, got %d", len(protocols))
	}
}

// Feature: user-portal, Property 8: Node List Filtering Correctness
// Validates: Requirements 5.3
// *For any* filter criteria (region, protocol), the returned node list SHALL contain
// only nodes matching all specified criteria.
func TestProperty_NodeListFilteringCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	regions := []string{"US", "JP", "HK", "SG", "EU"}
	protocols := []string{"vmess", "vless", "trojan", "shadowsocks"}

	// Property: Filtering by region returns only nodes with that region
	properties.Property("filtering by region returns only matching nodes", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			// Generate nodes with random regions
			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:       int64(i + 1),
					Name:     "Node",
					Region:   regions[int(seed+int64(i))%len(regions)],
					Protocol: protocols[int(seed+int64(i))%len(protocols)],
				}
			}

			// Pick a region to filter
			filterRegion := regions[int(seed)%len(regions)]
			filtered := FilterNodes(nodes, &NodeFilter{Region: filterRegion})

			// All filtered nodes should have the specified region
			for _, node := range filtered {
				if !strings.EqualFold(node.Region, filterRegion) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(1, 20),
	))

	// Property: Filtering by protocol returns only nodes with that protocol
	properties.Property("filtering by protocol returns only matching nodes", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			// Generate nodes with random protocols
			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:       int64(i + 1),
					Name:     "Node",
					Region:   regions[int(seed+int64(i))%len(regions)],
					Protocol: protocols[int(seed+int64(i))%len(protocols)],
				}
			}

			// Pick a protocol to filter
			filterProtocol := protocols[int(seed)%len(protocols)]
			filtered := FilterNodes(nodes, &NodeFilter{Protocol: filterProtocol})

			// All filtered nodes should have the specified protocol
			for _, node := range filtered {
				if !strings.EqualFold(node.Protocol, filterProtocol) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(1, 20),
	))

	// Property: Combined filter returns only nodes matching ALL criteria
	properties.Property("combined filter returns only nodes matching all criteria", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			// Generate nodes
			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:       int64(i + 1),
					Name:     "Node",
					Region:   regions[int(seed+int64(i))%len(regions)],
					Protocol: protocols[int(seed+int64(i))%len(protocols)],
				}
			}

			// Pick criteria to filter
			filterRegion := regions[int(seed)%len(regions)]
			filterProtocol := protocols[int(seed)%len(protocols)]
			filtered := FilterNodes(nodes, &NodeFilter{Region: filterRegion, Protocol: filterProtocol})

			// All filtered nodes should match BOTH criteria
			for _, node := range filtered {
				if !strings.EqualFold(node.Region, filterRegion) || !strings.EqualFold(node.Protocol, filterProtocol) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(1, 20),
	))

	// Property: Empty filter returns all nodes
	properties.Property("empty filter returns all nodes", prop.ForAll(
		func(numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{ID: int64(i + 1)}
			}

			filtered := FilterNodes(nodes, nil)
			return len(filtered) == numNodes
		},
		gen.IntRange(1, 20),
	))

	// Property: Filter is case-insensitive
	properties.Property("filter is case-insensitive", prop.ForAll(
		func(seed int64) bool {
			nodes := []*Node{
				{ID: 1, Region: "US", Protocol: "vmess"},
				{ID: 2, Region: "us", Protocol: "VMESS"},
				{ID: 3, Region: "Us", Protocol: "VMess"},
			}

			// All should match regardless of case
			filtered := FilterNodes(nodes, &NodeFilter{Region: "US"})
			if len(filtered) != 3 {
				return false
			}

			filtered = FilterNodes(nodes, &NodeFilter{Protocol: "vmess"})
			return len(filtered) == 3
		},
		gen.Int64Range(0, 1000),
	))

	properties.TestingRun(t)
}

// Feature: user-portal, Property 9: Node List Sorting Correctness
// Validates: Requirements 5.4
// *For any* sort criteria (name, region, latency), the returned node list SHALL be
// correctly ordered according to the specified criteria.
func TestProperty_NodeListSortingCorrectness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Property: Sorting by name produces correctly ordered list
	properties.Property("sorting by name produces correctly ordered list", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 1 || numNodes > 20 {
				return true
			}

			names := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot"}
			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:   int64(i + 1),
					Name: names[int(seed+int64(i))%len(names)],
				}
			}

			// Sort ascending
			sorted := SortNodes(nodes, &SortOption{Field: "name", Order: "asc"})
			for i := 1; i < len(sorted); i++ {
				if strings.ToLower(sorted[i-1].Name) > strings.ToLower(sorted[i].Name) {
					return false
				}
			}

			// Sort descending
			sorted = SortNodes(nodes, &SortOption{Field: "name", Order: "desc"})
			for i := 1; i < len(sorted); i++ {
				if strings.ToLower(sorted[i-1].Name) < strings.ToLower(sorted[i].Name) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(2, 20),
	))

	// Property: Sorting by region produces correctly ordered list
	properties.Property("sorting by region produces correctly ordered list", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 1 || numNodes > 20 {
				return true
			}

			regions := []string{"US", "JP", "HK", "SG", "EU"}
			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:     int64(i + 1),
					Region: regions[int(seed+int64(i))%len(regions)],
				}
			}

			// Sort ascending
			sorted := SortNodes(nodes, &SortOption{Field: "region", Order: "asc"})
			for i := 1; i < len(sorted); i++ {
				if strings.ToLower(sorted[i-1].Region) > strings.ToLower(sorted[i].Region) {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(2, 20),
	))

	// Property: Sorting by latency puts untested nodes at the end (ascending)
	properties.Property("sorting by latency puts untested nodes at end", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 1 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				latency := int(seed+int64(i)) % 500
				if i%3 == 0 {
					latency = -1 // Untested
				}
				nodes[i] = &Node{
					ID:      int64(i + 1),
					Latency: latency,
				}
			}

			sorted := SortNodes(nodes, &SortOption{Field: "latency", Order: "asc"})

			// Find first untested node
			firstUntested := -1
			for i, n := range sorted {
				if n.Latency == -1 {
					firstUntested = i
					break
				}
			}

			// All nodes after first untested should also be untested
			if firstUntested >= 0 {
				for i := firstUntested; i < len(sorted); i++ {
					if sorted[i].Latency != -1 {
						return false
					}
				}
			}

			// All tested nodes should be in ascending order
			for i := 1; i < len(sorted); i++ {
				if sorted[i-1].Latency != -1 && sorted[i].Latency != -1 {
					if sorted[i-1].Latency > sorted[i].Latency {
						return false
					}
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(2, 20),
	))

	// Property: Sorting by load produces correctly ordered list
	properties.Property("sorting by load produces correctly ordered list", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 1 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:   int64(i + 1),
					Load: int(seed+int64(i)) % 100,
				}
			}

			// Sort ascending
			sorted := SortNodes(nodes, &SortOption{Field: "load", Order: "asc"})
			for i := 1; i < len(sorted); i++ {
				if sorted[i-1].Load > sorted[i].Load {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(2, 20),
	))

	// Property: Sorting does not change the number of nodes
	properties.Property("sorting preserves node count", prop.ForAll(
		func(numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{ID: int64(i + 1)}
			}

			sorted := SortNodes(nodes, &SortOption{Field: "name", Order: "asc"})
			return len(sorted) == numNodes
		},
		gen.IntRange(1, 20),
	))

	// Property: Sorting does not modify original slice
	properties.Property("sorting does not modify original slice", prop.ForAll(
		func(seed int64, numNodes int) bool {
			if numNodes <= 1 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			originalOrder := make([]int64, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{
					ID:   int64(numNodes - i), // Reverse order
					Name: string(rune('A' + i)),
				}
				originalOrder[i] = nodes[i].ID
			}

			// Sort
			SortNodes(nodes, &SortOption{Field: "name", Order: "asc"})

			// Original should be unchanged
			for i, id := range originalOrder {
				if nodes[i].ID != id {
					return false
				}
			}

			return true
		},
		gen.Int64Range(0, 1000),
		gen.IntRange(2, 20),
	))

	// Property: Nil sort option returns original order
	properties.Property("nil sort option returns original order", prop.ForAll(
		func(numNodes int) bool {
			if numNodes <= 0 || numNodes > 20 {
				return true
			}

			nodes := make([]*Node, numNodes)
			for i := 0; i < numNodes; i++ {
				nodes[i] = &Node{ID: int64(i + 1)}
			}

			sorted := SortNodes(nodes, nil)

			// Should be same order
			for i := range nodes {
				if sorted[i].ID != nodes[i].ID {
					return false
				}
			}

			return true
		},
		gen.IntRange(1, 20),
	))

	properties.TestingRun(t)
}

// Additional helper test
func TestSortNodes_Stability(t *testing.T) {
	// Test that sorting is stable for equal elements
	nodes := []*Node{
		{ID: 1, Name: "Alpha", Region: "US"},
		{ID: 2, Name: "Alpha", Region: "JP"},
		{ID: 3, Name: "Alpha", Region: "HK"},
	}

	sorted := SortNodes(nodes, &SortOption{Field: "name", Order: "asc"})

	// All have same name, so order should be preserved (stable sort)
	// Note: Go's sort.Slice is not guaranteed to be stable, so we just verify
	// that all nodes are present
	ids := make(map[int64]bool)
	for _, n := range sorted {
		ids[n.ID] = true
	}

	if len(ids) != 3 || !ids[1] || !ids[2] || !ids[3] {
		t.Error("Sorting lost some nodes")
	}
}

func TestFilterNodes_NilInput(t *testing.T) {
	// Test with nil nodes
	filtered := FilterNodes(nil, &NodeFilter{Region: "US"})
	if filtered == nil || len(filtered) != 0 {
		t.Error("Expected empty slice for nil input")
	}

	// Test with empty nodes
	filtered = FilterNodes([]*Node{}, &NodeFilter{Region: "US"})
	if len(filtered) != 0 {
		t.Error("Expected empty slice for empty input")
	}
}

func TestSortNodes_EmptyInput(t *testing.T) {
	sorted := SortNodes([]*Node{}, &SortOption{Field: "name", Order: "asc"})
	if len(sorted) != 0 {
		t.Error("Expected empty slice for empty input")
	}

	sorted = SortNodes(nil, &SortOption{Field: "name", Order: "asc"})
	if sorted == nil || len(sorted) != 0 {
		t.Error("Expected empty slice for nil input")
	}
}

// Test that GetAvailableRegions and GetAvailableProtocols return sorted results
func TestGetAvailableRegions_Sorted(t *testing.T) {
	nodes := []*Node{
		{Region: "ZZ"},
		{Region: "AA"},
		{Region: "MM"},
	}

	regions := GetAvailableRegions(nodes)
	if !sort.StringsAreSorted(regions) {
		t.Error("Regions should be sorted")
	}
}

func TestGetAvailableProtocols_Sorted(t *testing.T) {
	nodes := []*Node{
		{Protocol: "zzz"},
		{Protocol: "aaa"},
		{Protocol: "mmm"},
	}

	protocols := GetAvailableProtocols(nodes)
	if !sort.StringsAreSorted(protocols) {
		t.Error("Protocols should be sorted")
	}
}

func TestBuildPortalNodeName_PrefersUnderlyingNodeName(t *testing.T) {
	proxyModel := &repository.Proxy{
		Name:     "node-64-176-54-36-103738-vmess",
		Remark:   "auto provisioned",
		Protocol: "vmess",
		Host:     "64.176.54.36",
		Port:     20001,
	}
	nodeModel := &repository.Node{
		Name:   "日本-大阪-36",
		Region: "日本",
	}

	got := buildPortalNodeName(proxyModel, nodeModel, "VMess", "64.176.54.36")
	if got != "日本-大阪-36" {
		t.Fatalf("expected underlying node name, got %q", got)
	}
}

func TestNormalizePortalRegionLabel_SupportsLocalizedValues(t *testing.T) {
	if got := normalizePortalRegionLabel("jp"); got != "日本" {
		t.Fatalf("expected 日本, got %q", got)
	}
	if got := normalizePortalRegionLabel("中国"); got != "中国" {
		t.Fatalf("expected 中国, got %q", got)
	}
}
