package tests

import (
	"testing"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	statuscontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/status"
	replicascheduler "github.com/kennethnrk/edgernetes-ai/internal/control-plane/scheduler/replica"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

func TestGetModelStatus_AllRunning(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-running"
	requireRegisterModel(t, s, modelID, "ModelA", 3)

	requireCreateReplica(t, s, "rep-1", modelID, constants.ModelReplicaStatusRunning)
	requireCreateReplica(t, s, "rep-2", modelID, constants.ModelReplicaStatusRunning)
	requireCreateReplica(t, s, "rep-3", modelID, constants.ModelReplicaStatusRunning)

	result, err := statuscontroller.GetModelStatus(s, "default", "ModelA")
	if err != nil {
		t.Fatalf("GetModelStatus unexpected error: %v", err)
	}

	if result.Status != constants.ModelStatusRunning {
		t.Errorf("expected status %s, got %s", constants.ModelStatusRunning, result.Status)
	}
	if result.Breakdown.Running != 3 {
		t.Errorf("expected 3 running replicas, got %d", result.Breakdown.Running)
	}
}

func TestGetModelStatus_AllPending(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-pending"
	requireRegisterModel(t, s, modelID, "ModelB", 2)

	requireCreateReplica(t, s, "rep-1", modelID, constants.ModelReplicaStatusPending)
	requireCreateReplica(t, s, "rep-2", modelID, constants.ModelReplicaStatusPending)

	result, err := statuscontroller.GetModelStatus(s, "default", "ModelB")
	if err != nil {
		t.Fatalf("GetModelStatus unexpected error: %v", err)
	}

	if result.Status != constants.ModelStatusPending {
		t.Errorf("expected status %s, got %s", constants.ModelStatusPending, result.Status)
	}
	if result.Breakdown.Pending != 2 {
		t.Errorf("expected 2 pending replicas, got %d", result.Breakdown.Pending)
	}
}

func TestGetModelStatus_PartialRunning(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-partial"
	requireRegisterModel(t, s, modelID, "ModelC", 3)

	requireCreateReplica(t, s, "rep-1", modelID, constants.ModelReplicaStatusRunning)
	requireCreateReplica(t, s, "rep-2", modelID, constants.ModelReplicaStatusPending)
	requireCreateReplica(t, s, "rep-3", modelID, constants.ModelReplicaStatusFailed)

	result, err := statuscontroller.GetModelStatus(s, "default", "ModelC")
	if err != nil {
		t.Fatalf("GetModelStatus unexpected error: %v", err)
	}

	if result.Status != constants.ModelStatusPartialRunning {
		t.Errorf("expected status %s, got %s", constants.ModelStatusPartialRunning, result.Status)
	}
	if result.Breakdown.Running != 1 || result.Breakdown.Pending != 1 || result.Breakdown.Failed != 1 {
		t.Errorf("unexpected breakdown: %+v", result.Breakdown)
	}
}

func TestGetModelStatus_AllFailed(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-failed"
	requireRegisterModel(t, s, modelID, "ModelD", 2)

	requireCreateReplica(t, s, "rep-1", modelID, constants.ModelReplicaStatusFailed)
	requireCreateReplica(t, s, "rep-2", modelID, constants.ModelReplicaStatusFailed)

	result, err := statuscontroller.GetModelStatus(s, "default", "ModelD")
	if err != nil {
		t.Fatalf("GetModelStatus unexpected error: %v", err)
	}

	if result.Status != constants.ModelStatusFailed {
		t.Errorf("expected status %s, got %s", constants.ModelStatusFailed, result.Status)
	}
}

func TestGetModelStatus_ModelNotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_, err := statuscontroller.GetModelStatus(s, "default", "NonExistentModel")
	if err == nil {
		t.Errorf("expected error for non-existent model, got nil")
	}
}

func TestGetNodesByModelName_Success(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-nodes"
	requireRegisterModel(t, s, modelID, "ModelNodes", 2)

	requireCreateReplica(t, s, "rep-1", modelID, constants.ModelReplicaStatusRunning)
	requireCreateReplica(t, s, "rep-2", modelID, constants.ModelReplicaStatusPending)
	requireCreateReplica(t, s, "rep-other", "other-model", constants.ModelReplicaStatusRunning) // noise

	requireRegisterNodeWithReplicas(t, s, "node-1", "10.0.0.1", 5000, []string{"rep-1"})
	requireRegisterNodeWithReplicas(t, s, "node-2", "10.0.0.2", 5000, []string{"rep-2", "rep-other"})
	requireRegisterNodeWithReplicas(t, s, "node-3", "10.0.0.3", 5000, []string{"rep-other"})

	returnedModelID, nodes, err := registrycontroller.GetNodesByModelName(s, "default", "ModelNodes")
	if err != nil {
		t.Fatalf("GetNodesByModelName unexpected error: %v", err)
	}

	if returnedModelID != modelID {
		t.Fatalf("expected model ID %s, got %s", modelID, returnedModelID)
	}

	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}

	// Verify the correct nodes were returned
	var hasNode1, hasNode2 bool
	for _, n := range nodes {
		if n.NodeID == "node-1" && n.IP == "10.0.0.1" {
			hasNode1 = true
		}
		if n.NodeID == "node-2" && n.IP == "10.0.0.2" {
			hasNode2 = true
		}
	}

	if !hasNode1 || !hasNode2 {
		t.Errorf("missing expected nodes in response: %+v", nodes)
	}
}

func TestGetNodesByModelName_ModelNotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	_, _, err := registrycontroller.GetNodesByModelName(s, "default", "NonExistent")
	if err == nil {
		t.Errorf("expected error for non-existent model, got nil")
	}
}

// Helpers

func requireRegisterModel(t *testing.T, s *store.Store, id, name string, replicas int) {
	t.Helper()
	err := registrycontroller.RegisterModel(s, id, store.ModelInfo{
		ID:        id,
		Name:      name,
		Namespace: "default",
		Replicas:  replicas,
	})
	if err != nil {
		t.Fatalf("failed to register test model: %v", err)
	}
}

func requireCreateReplica(t *testing.T, s *store.Store, id, modelID string, status constants.ModelReplicaStatus) {
	t.Helper()
	err := replicascheduler.CreateReplica(s, id, store.ReplicaInfo{
		ID:      id,
		ModelID: modelID,
		Status:  status,
	})
	if err != nil {
		t.Fatalf("failed to create test replica: %v", err)
	}
}

func requireRegisterNodeWithReplicas(t *testing.T, s *store.Store, id, ip string, port int, assignedReplicas []string) {
	t.Helper()
	err := registrycontroller.RegisterNode(s, id, store.NodeInfo{
		ID:             id,
		IP:             ip,
		Port:           port,
		AssignedModels: assignedReplicas,
		RegisteredAt:   time.Now(),
		UpdatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to register test node: %v", err)
	}
}
