package tests

import (
	"testing"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

func TestAssignModel_Success(t *testing.T) {
	a := &agent.Agent{
		ID:             "agent-1",
		AssignedModels: []agent.ModelReplicaDetails{},
	}

	model := agent.ModelReplicaDetails{
		ID:        "replica-1",
		ModelID:   "model-1",
		Name:      "test-model",
		Version:   "v1",
		ModelType: constants.ModelTypeCNN,
		Status:    constants.ModelReplicaStatusPending,
	}

	err := a.AssignModel(model)
	if err != nil {
		t.Fatalf("AssignModel() unexpected error: %v", err)
	}

	if len(a.AssignedModels) != 1 {
		t.Fatalf("Expected 1 assigned model, got %d", len(a.AssignedModels))
	}

	if a.AssignedModels[0].ID != "replica-1" {
		t.Fatalf("Assigned model ID mismatch. Expected 'replica-1', got '%s'", a.AssignedModels[0].ID)
	}
}

func TestAssignModel_AlreadyAssigned(t *testing.T) {
	model := agent.ModelReplicaDetails{
		ID:        "replica-1",
		ModelID:   "model-1",
		Name:      "test-model",
		Version:   "v1",
		ModelType: constants.ModelTypeCNN,
		Status:    constants.ModelReplicaStatusPending,
	}

	a := &agent.Agent{
		ID:             "agent-1",
		AssignedModels: []agent.ModelReplicaDetails{model},
	}

	err := a.AssignModel(model)
	if err == nil {
		t.Fatal("AssignModel() expected error for already assigned model, got nil")
	}

	if err.Error() != "model already assigned" {
		t.Fatalf("AssignModel() expected 'model already assigned' error, got: %v", err)
	}

	if len(a.AssignedModels) != 1 {
		t.Fatalf("Expected exactly 1 assigned model, got %d", len(a.AssignedModels))
	}
}
