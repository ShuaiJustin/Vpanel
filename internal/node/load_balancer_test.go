package node

import (
	"context"
	"testing"
)

func TestLeastConnectionsStrategy_PrefersLowerTrafficPressureOnTie(t *testing.T) {
	strategy := NewLeastConnectionsStrategy()
	nodes := []*Node{
		{ID: 1, Name: "higher-pressure", CurrentUsers: 10, Weight: 10, TrafficTotal: 790, TrafficLimit: 1000, AlertTrafficThreshold: 80},
		{ID: 2, Name: "lower-pressure", CurrentUsers: 10, Weight: 10, TrafficTotal: 200, TrafficLimit: 1000, AlertTrafficThreshold: 80},
	}

	selected, err := strategy.Select(context.Background(), nodes, &SelectOptions{Strategy: StrategyLeastConnections})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if selected == nil || selected.ID != 2 {
		t.Fatalf("expected node 2 to be selected, got %+v", selected)
	}
}

func TestWeightedStrategy_EffectiveWeightAvoidsHighTrafficPressure(t *testing.T) {
	strategy := NewWeightedStrategy()
	nodes := []*Node{
		{ID: 1, Name: "low-pressure", Weight: 10, TrafficTotal: 200, TrafficLimit: 1000, AlertTrafficThreshold: 80},
		{ID: 2, Name: "high-pressure", Weight: 10, TrafficTotal: 790, TrafficLimit: 1000, AlertTrafficThreshold: 80},
	}

	counts := map[int64]int{}
	for i := 0; i < 120; i++ {
		selected, err := strategy.Select(context.Background(), nodes, &SelectOptions{Strategy: StrategyWeighted})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		counts[selected.ID]++
	}

	if counts[1] <= counts[2] {
		t.Fatalf("expected low-pressure node to be selected more often, got counts=%v", counts)
	}
}

func TestSortNodesBySelectionPriority_PrefersTrafficHeadroom(t *testing.T) {
	nodes := []*Node{
		{ID: 1, Name: "high-pressure", CurrentUsers: 1, Weight: 10, TrafficTotal: 790, TrafficLimit: 1000, AlertTrafficThreshold: 80},
		{ID: 2, Name: "lower-pressure", CurrentUsers: 1, Weight: 10, TrafficTotal: 200, TrafficLimit: 1000, AlertTrafficThreshold: 80},
		{ID: 3, Name: "unlimited", CurrentUsers: 3, Weight: 5, TrafficLimit: 0},
	}

	sortNodesBySelectionPriority(nodes)

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if nodes[0].ID != 3 || nodes[1].ID != 2 || nodes[2].ID != 1 {
		t.Fatalf("expected order [3 2 1], got [%d %d %d]", nodes[0].ID, nodes[1].ID, nodes[2].ID)
	}
}
