package grpcdiscovery_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
	grpcdiscovery "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/discovery"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

func setupTestStore(t *testing.T) *store.Store {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "store")
	s, err := store.New(dataDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	t.Cleanup(func() {
		s.Close()
		os.RemoveAll(dataDir)
	})

	return s
}

func TestGetNodes(t *testing.T) {
	s := setupTestStore(t)
	server := grpcdiscovery.NewDiscoveryServer(s)

	now := time.Now()

	saveNode := func(info store.NodeInfo) {
		b, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("failed to marshal node: %v", err)
		}
		err = s.Put("node:"+info.ID, b)
		if err != nil {
			t.Fatalf("failed to put node: %v", err)
		}
	}

	// Register some mock nodes directly to store
	saveNode(store.NodeInfo{
		ID:            "node-1",
		IP:            "10.0.0.1",
		Port:          8080,
		Status:        constants.StatusOnline,
		LastHeartbeat: now,
	})
	saveNode(store.NodeInfo{
		ID:            "node-2",
		IP:            "10.0.0.2",
		Port:          8080,
		Status:        constants.StatusOffline,
		LastHeartbeat: now.Add(-time.Hour),
	})
	saveNode(store.NodeInfo{
		ID:            "node-3",
		IP:            "10.0.0.3",
		Port:          9090,
		Status:        constants.StatusOnline,
		LastHeartbeat: now,
	})

	ctx := context.Background()
	req := &discoverypb.GetNodesRequest{}

	res, err := server.GetNodes(ctx, req)
	if err != nil {
		t.Fatalf("GetNodes failed: %v", err)
	}

	if len(res.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(res.Nodes))
	}

	// Check that we only got the online nodes
	expectedNodes := map[string]bool{"node-1": true, "node-3": true}
	for _, n := range res.Nodes {
		if !expectedNodes[n.NodeId] {
			t.Errorf("unexpected node in response: %s", n.NodeId)
		}
		if n.NodeId == "node-1" {
			if n.Ip != "10.0.0.1" || n.Port != int32(8080) {
				t.Errorf("node-1 has incorrect values: %v", n)
			}
		} else if n.NodeId == "node-3" {
			if n.Ip != "10.0.0.3" || n.Port != int32(9090) {
				t.Errorf("node-3 has incorrect values: %v", n)
			}
		}
	}
}

