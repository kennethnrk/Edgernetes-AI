package tests

import (
	"context"
	"testing"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	grpcagent "github.com/kennethnrk/edgernetes-ai/internal/agent/api/grpc"
	agentmonitor "github.com/kennethnrk/edgernetes-ai/internal/agent/monitor"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
)

func TestCheckHealth_Success(t *testing.T) {
	a := &agent.Agent{
		ID: "agent-1",
		AssignedModels: []agent.ModelReplicaDetails{
			{
				ID:     "replica-1",
				Status: constants.ModelReplicaStatusRunning,
			},
			{
				ID:     "replica-2",
				Status: constants.ModelReplicaStatusRunning,
			},
		},
	}

	replicas, success, err := agentmonitor.CheckHealth(a)
	if err != nil {
		t.Fatalf("CheckHealth expected no error, got: %v", err)
	}

	if !success {
		t.Fatalf("CheckHealth expected success=true for all running models, got false")
	}

	if len(replicas) != 2 {
		t.Fatalf("CheckHealth expected 2 replicas, got %d", len(replicas))
	}
}

func TestCheckHealth_Failure(t *testing.T) {
	a := &agent.Agent{
		ID: "agent-1",
		AssignedModels: []agent.ModelReplicaDetails{
			{
				ID:     "replica-1",
				Status: constants.ModelReplicaStatusRunning,
			},
			{
				ID:     "replica-2",
				Status: constants.ModelReplicaStatusFailed,
			},
		},
	}

	_, success, err := agentmonitor.CheckHealth(a)
	if err != nil {
		t.Fatalf("CheckHealth expected no error, got: %v", err)
	}

	if success {
		t.Fatalf("CheckHealth expected success=false when a model is not running, got true")
	}
}

func TestModelReplicaToProto(t *testing.T) {
	replica := agent.ModelReplicaDetails{
		ID:           "replica-1",
		ModelID:      "model-1",
		Name:         "test-model",
		Version:      "v1",
		FilePath:     "/tmp/model",
		ModelType:    constants.ModelTypeCNN,
		ModelSize:    1024,
		Status:       constants.ModelReplicaStatusRunning,
		ErrorCode:    0,
		ErrorMessage: "",
	}

	pb := grpcagent.ModelReplicaToProto(&replica)

	if pb.ReplicaId != replica.ID {
		t.Errorf("Expected ReplicaId %s, got %s", replica.ID, pb.ReplicaId)
	}
	if pb.ModelId != replica.ModelID {
		t.Errorf("Expected ModelId %s, got %s", replica.ModelID, pb.ModelId)
	}
	if pb.Name != replica.Name {
		t.Errorf("Expected Name %s, got %s", replica.Name, pb.Name)
	}
	if pb.Version != replica.Version {
		t.Errorf("Expected Version %s, got %s", replica.Version, pb.Version)
	}
	if pb.FilePath != replica.FilePath {
		t.Errorf("Expected FilePath %s, got %s", replica.FilePath, pb.FilePath)
	}
	if pb.ModelType != string(replica.ModelType) {
		t.Errorf("Expected ModelType %s, got %s", string(replica.ModelType), pb.ModelType)
	}
	if pb.ModelSize != replica.ModelSize {
		t.Errorf("Expected ModelSize %d, got %d", replica.ModelSize, pb.ModelSize)
	}
	if pb.Status != string(replica.Status) {
		t.Errorf("Expected Status %s, got %s", string(replica.Status), pb.Status)
	}
}

func TestRequestHeartbeat(t *testing.T) {
	a := &agent.Agent{
		ID: "agent-test-1",
		AssignedModels: []agent.ModelReplicaDetails{
			{
				ID:     "rep-1",
				Status: constants.ModelReplicaStatusRunning,
			},
		},
	}

	server := grpcagent.NewHeartbeatServer(a)
	req := &heartbeatpb.RequestHeartbeatRequest{}

	res, err := server.RequestHeartbeat(context.Background(), req)
	if err != nil {
		t.Fatalf("RequestHeartbeat expected no error, got: %v", err)
	}

	if res.NodeID != "agent-test-1" {
		t.Errorf("Expected NodeID %s, got %s", "agent-test-1", res.NodeID)
	}

	if !res.Success {
		t.Errorf("Expected Success true, got false")
	}

	if len(res.ModelReplicas) != 1 {
		t.Fatalf("Expected 1 ModelReplica, got %d", len(res.ModelReplicas))
	}

	if res.ModelReplicas[0].ReplicaId != "rep-1" {
		t.Errorf("Expected ReplicaId rep-1, got %s", res.ModelReplicas[0].ReplicaId)
	}
}

func TestIsHeartbeatStale(t *testing.T) {
	a := &agent.Agent{}
	// Initialize heartbeat
	a.UpdateLastHeartbeat()

	// Should not be stale immediately
	if a.IsHeartbeatStale(60 * time.Second) {
		t.Errorf("Expected heartbeat to not be stale immediately")
	}

	// Manually set LastHeartbeat to 70 seconds ago to simulate staleness
	a.LastHeartbeat = time.Now().Add(-70 * time.Second)

	// Should be stale after 70 seconds with a 60 second timeout
	if !a.IsHeartbeatStale(60 * time.Second) {
		t.Errorf("Expected heartbeat to be stale after 70 seconds")
	}
}
