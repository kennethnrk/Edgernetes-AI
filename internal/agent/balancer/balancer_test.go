package balancer

import (
	"math"
	"testing"

	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
)

func TestWeightedRoundRobin(t *testing.T) {
	lb := NewWeightedRoundRobin()

	endpoints := []*heartbeatpb.EndpointDetail{
		{NodeId: "node1", Healthy: true, Weight: 10},
		{NodeId: "node2", Healthy: true, Weight: 30},
		{NodeId: "node3", Healthy: false, Weight: 50}, // Should be ignored
	}

	counts := make(map[string]int)

	totalPicks := 400
	for i := 0; i < totalPicks; i++ {
		ep, err := lb.Pick(endpoints)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[ep.NodeId]++
	}

	// node2 (weight 30) should be picked roughly 3x as often as node1 (weight 10).
	// With 400 picks: node1 ~100, node2 ~300.
	if counts["node3"] != 0 {
		t.Errorf("expected node3 to have 0 picks because it is unhealthy, got %d", counts["node3"])
	}

	ratio := float64(counts["node2"]) / float64(counts["node1"])
	if math.Abs(ratio-3.0) > 0.5 {
		t.Errorf("expected ratio of node2/node1 picks to be roughly 3.0, got %f (node1: %d, node2: %d)", ratio, counts["node1"], counts["node2"])
	}
}

func TestWeightedRoundRobin_NoHealthy(t *testing.T) {
	lb := NewWeightedRoundRobin()

	endpoints := []*heartbeatpb.EndpointDetail{
		{NodeId: "node1", Healthy: false, Weight: 10},
	}

	_, err := lb.Pick(endpoints)
	if err == nil {
		t.Error("expected error when no healthy endpoints available, got nil")
	}
}
