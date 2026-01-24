package tests

import (
	"testing"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// helper to create a new temporary store for tests.
func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New() error = %v", err)
	}
	return s
}

func TestRegisterAndGetNode(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	nodeID := "node-1"
	info := store.NodeInfo{
		Name: "test-node",
	}

	if err := registrycontroller.RegisterNode(s, nodeID, info); err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}

	got, found, err := registrycontroller.GetNodeByID(s, nodeID)
	if err != nil {
		t.Fatalf("GetNodeByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetNodeByID() found = false, want true")
	}
	if got.ID != nodeID {
		t.Fatalf("GetNodeByID() ID = %q, want %q", got.ID, nodeID)
	}
	if got.Name != info.Name {
		t.Fatalf("GetNodeByID() Name = %q, want %q", got.Name, info.Name)
	}
	if got.RegisteredAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps to be set on registered node")
	}
}

func TestUpdateNodeInfo_PreservesRegisteredAt(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	nodeID := "node-1"
	initial := store.NodeInfo{
		Name: "initial",
	}
	if err := registrycontroller.RegisterNode(s, nodeID, initial); err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}

	original, found, err := registrycontroller.GetNodeByID(s, nodeID)
	if err != nil || !found {
		t.Fatalf("GetNodeByID() after register error = %v, found = %v", err, found)
	}

	// Sleep briefly to ensure UpdatedAt changes.
	time.Sleep(10 * time.Millisecond)

	updated := store.NodeInfo{
		Name: "updated",
	}
	if err := registrycontroller.UpdateNodeInfo(s, nodeID, updated); err != nil {
		t.Fatalf("UpdateNodeInfo() error = %v", err)
	}

	got, found, err := registrycontroller.GetNodeByID(s, nodeID)
	if err != nil || !found {
		t.Fatalf("GetNodeByID() after update error = %v, found = %v", err, found)
	}

	if got.Name != "updated" {
		t.Fatalf("Name = %q, want %q", got.Name, "updated")
	}
	if !got.RegisteredAt.Equal(original.RegisteredAt) {
		t.Fatalf("RegisteredAt changed, want preserved")
	}
	if !got.UpdatedAt.After(original.UpdatedAt) {
		t.Fatalf("UpdatedAt not advanced; got %v, original %v", got.UpdatedAt, original.UpdatedAt)
	}
}

func TestUpdateNodeStatus_NotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	err := registrycontroller.UpdateNodeStatus(s, "missing-node", constants.StatusOnline)
	if err == nil {
		t.Fatalf("UpdateNodeStatus() error = nil, want non-nil for missing node")
	}
}

func TestUpdateNodeStatus_Success(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	nodeID := "node-1"
	initial := store.NodeInfo{
		Name:   "status-node",
		Status: constants.StatusOffline,
	}
	if err := registrycontroller.RegisterNode(s, nodeID, initial); err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}

	before, found, err := registrycontroller.GetNodeByID(s, nodeID)
	if err != nil || !found {
		t.Fatalf("GetNodeByID() before status update error = %v, found = %v", err, found)
	}

	time.Sleep(10 * time.Millisecond)

	if err := registrycontroller.UpdateNodeStatus(s, nodeID, constants.StatusOnline); err != nil {
		t.Fatalf("UpdateNodeStatus() error = %v", err)
	}

	after, found, err := registrycontroller.GetNodeByID(s, nodeID)
	if err != nil || !found {
		t.Fatalf("GetNodeByID() after status update error = %v, found = %v", err, found)
	}

	if after.Status != constants.StatusOnline {
		t.Fatalf("Status = %q, want %q", after.Status, constants.StatusOnline)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("UpdatedAt not advanced; got %v, before %v", after.UpdatedAt, before.UpdatedAt)
	}
	if after.LastHeartbeat.IsZero() {
		t.Fatalf("LastHeartbeat not set on status update")
	}
}

func TestListDevices(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	nodes := []store.NodeInfo{
		{ID: "n1", Name: "node-1"},
		{ID: "n2", Name: "node-2"},
	}

	for _, n := range nodes {
		if err := registrycontroller.RegisterNode(s, n.ID, n); err != nil {
			t.Fatalf("RegisterNode(%q) error = %v", n.ID, err)
		}
	}

	list, err := registrycontroller.ListNodes(s)
	if err != nil {
		t.Fatalf("ListNodes() error = %v", err)
	}

	if len(list) != len(nodes) {
		t.Fatalf("ListNodes() len = %d, want %d", len(list), len(nodes))
	}

	seen := make(map[string]store.NodeInfo)
	for _, n := range list {
		seen[n.ID] = n
	}
	for _, want := range nodes {
		got, ok := seen[want.ID]
		if !ok {
			t.Fatalf("node %q not found in ListNodes()", want.ID)
		}
		if got.Name != want.Name {
			t.Fatalf("node %q Name = %q, want %q", want.ID, got.Name, want.Name)
		}
	}
}

func TestRegisterAndGetModel(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-1"
	info := store.ModelInfo{
		Name:      "test-model",
		Version:   "v1",
		FilePath:  "/models/test-model",
		ModelType: constants.ModelTypeCNN,
		ModelSize: 1024,
		Replicas:  2,
	}

	if err := registrycontroller.RegisterModel(s, modelID, info); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	got, found, err := registrycontroller.GetModelByID(s, modelID)
	if err != nil {
		t.Fatalf("GetModelByID() error = %v", err)
	}
	if !found {
		t.Fatalf("GetModelByID() found = false, want true")
	}
	if got.ID != modelID {
		t.Fatalf("GetModelByID() ID = %q, want %q", got.ID, modelID)
	}
	if got.Name != info.Name {
		t.Fatalf("GetModelByID() Name = %q, want %q", got.Name, info.Name)
	}
	if got.Version != info.Version {
		t.Fatalf("GetModelByID() Version = %q, want %q", got.Version, info.Version)
	}
	if got.FilePath != info.FilePath {
		t.Fatalf("GetModelByID() FilePath = %q, want %q", got.FilePath, info.FilePath)
	}
	if got.ModelType != info.ModelType {
		t.Fatalf("GetModelByID() ModelType = %q, want %q", got.ModelType, info.ModelType)
	}
	if got.ModelSize != info.ModelSize {
		t.Fatalf("GetModelByID() ModelSize = %d, want %d", got.ModelSize, info.ModelSize)
	}
	if got.Replicas != info.Replicas {
		t.Fatalf("GetModelByID() Replicas = %d, want %d", got.Replicas, info.Replicas)
	}
}

func TestUpdateModelInfo(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-1"
	initial := store.ModelInfo{
		Name:    "initial-model",
		Version: "v1",
	}
	if err := registrycontroller.RegisterModel(s, modelID, initial); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	updated := store.ModelInfo{
		Name:      "updated-model",
		Version:   "v2",
		FilePath:  "/models/updated-model",
		ModelType: constants.ModelTypeLinear,
		ModelSize: 2048,
		Replicas:  3,
	}
	if err := registrycontroller.UpdateModelInfo(s, modelID, updated); err != nil {
		t.Fatalf("UpdateModelInfo() error = %v", err)
	}

	got, found, err := registrycontroller.GetModelByID(s, modelID)
	if err != nil || !found {
		t.Fatalf("GetModelByID() after update error = %v, found = %v", err, found)
	}

	if got.ID != modelID {
		t.Fatalf("ID = %q, want %q", got.ID, modelID)
	}
	if got.Name != updated.Name {
		t.Fatalf("Name = %q, want %q", got.Name, updated.Name)
	}
	if got.Version != updated.Version {
		t.Fatalf("Version = %q, want %q", got.Version, updated.Version)
	}
	if got.FilePath != updated.FilePath {
		t.Fatalf("FilePath = %q, want %q", got.FilePath, updated.FilePath)
	}
	if got.ModelType != updated.ModelType {
		t.Fatalf("ModelType = %q, want %q", got.ModelType, updated.ModelType)
	}
	if got.ModelSize != updated.ModelSize {
		t.Fatalf("ModelSize = %d, want %d", got.ModelSize, updated.ModelSize)
	}
	if got.Replicas != updated.Replicas {
		t.Fatalf("Replicas = %d, want %d", got.Replicas, updated.Replicas)
	}
}

func TestDeRegisterModel(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelID := "model-1"
	info := store.ModelInfo{
		Name: "to-be-deleted",
	}
	if err := registrycontroller.RegisterModel(s, modelID, info); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	if err := registrycontroller.DeRegisterModel(s, modelID); err != nil {
		t.Fatalf("DeRegisterModel() error = %v", err)
	}

	_, found, err := registrycontroller.GetModelByID(s, modelID)
	if err != nil {
		t.Fatalf("GetModelByID() after delete error = %v", err)
	}
	if found {
		t.Fatalf("expected model to be deleted, but found = true")
	}
}

func TestListModels(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	models := []store.ModelInfo{
		{ID: "m1", Name: "model-1", Version: "v1"},
		{ID: "m2", Name: "model-2", Version: "v1"},
	}

	for _, m := range models {
		if err := registrycontroller.RegisterModel(s, m.ID, m); err != nil {
			t.Fatalf("RegisterModel(%q) error = %v", m.ID, err)
		}
	}

	list, err := registrycontroller.ListModels(s)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(list) != len(models) {
		t.Fatalf("ListModels() len = %d, want %d", len(list), len(models))
	}

	seen := make(map[string]store.ModelInfo)
	for _, m := range list {
		seen[m.ID] = m
	}
	for _, want := range models {
		got, ok := seen[want.ID]
		if !ok {
			t.Fatalf("model %q not found in ListModels()", want.ID)
		}
		if got.Name != want.Name {
			t.Fatalf("model %q Name = %q, want %q", want.ID, got.Name, want.Name)
		}
		if got.Version != want.Version {
			t.Fatalf("model %q Version = %q, want %q", want.ID, got.Version, want.Version)
		}
	}
}
