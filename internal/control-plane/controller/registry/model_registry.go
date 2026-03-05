package registrycontroller

import (
	"encoding/json"
	"errors"
	"fmt"

	replicascheduler "github.com/kennethnrk/edgernetes-ai/internal/control-plane/scheduler/replica"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type NodeAddress struct {
	NodeID string
	IP     string
	Port   int32
}

// RegisterModel stores a new ModelInfo under the given modelID.
// It enforces that the model name is non-empty and unique within its namespace
// Returns an error if a model with the same name and namespace
// already exists.
func RegisterModel(s *store.Store, modelID string, info store.ModelInfo) error {
	if modelID == "" {
		return errors.New("modelID cannot be empty")
	}
	if info.Name == "" {
		return errors.New("model name cannot be empty")
	}

	// Reject duplicate model names within the same namespace.
	if existing, found, err := GetModelByNamespaceAndName(s, info.Namespace, info.Name); err != nil {
		return fmt.Errorf("check model name uniqueness: %w", err)
	} else if found {
		return fmt.Errorf("model name %q is already registered in namespace %q (id=%s)", info.Name, info.Namespace, existing.ID)
	}

	// Ensure the stored ModelInfo has a consistent ID.
	if info.ID == "" {
		info.ID = modelID
	} else if info.ID != modelID {
		return fmt.Errorf("model info ID %q does not match modelID %q", info.ID, modelID)
	}

	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal model info: %w", err)
	}
	return s.Put("model:"+modelID, b)
}

// DeRegisterModel removes a model from the store.
func DeRegisterModel(s *store.Store, modelID string) error {
	if modelID == "" {
		return errors.New("modelID cannot be empty")
	}
	return s.Delete("model:" + modelID)
}

// UpdateModelInfo replaces the stored ModelInfo for a modelID.
func UpdateModelInfo(s *store.Store, modelID string, info store.ModelInfo) error {
	if modelID == "" {
		return errors.New("modelID cannot be empty")
	}

	// Ensure ID consistency.
	if info.ID != "" && info.ID != modelID {
		return fmt.Errorf("model info ID %q does not match modelID %q", info.ID, modelID)
	}
	info.ID = modelID

	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal model info: %w", err)
	}
	return s.Put("model:"+modelID, b)
}

// GetModelByID loads a ModelInfo by ID.
// Returns (zero ModelInfo, false, nil) if the model is not found.
func GetModelByID(s *store.Store, modelID string) (store.ModelInfo, bool, error) {
	if modelID == "" {
		return store.ModelInfo{}, false, errors.New("modelID cannot be empty")
	}

	raw, ok := s.Get("model:" + modelID)
	if !ok {
		return store.ModelInfo{}, false, nil
	}

	var info store.ModelInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return store.ModelInfo{}, false, fmt.Errorf("unmarshal model info: %w", err)
	}
	return info, true, nil
}

// ListModels returns all ModelInfo records currently in the store.
func ListModels(s *store.Store) ([]store.ModelInfo, error) {
	keys := s.Keys()
	models := make([]store.ModelInfo, 0, len(keys))

	for _, k := range keys {
		// Only consider keys for models.
		const prefix = "model:"
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}

		raw, ok := s.Get(k)
		if !ok {
			continue
		}
		var info store.ModelInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return nil, fmt.Errorf("unmarshal model %q: %w", k, err)
		}
		models = append(models, info)
	}

	return models, nil
}

// GetModelByNamespaceAndName looks up a model by its namespace and human-readable name.
// Returns (zero ModelInfo, false, nil) if no model with that namespace and name exists.
func GetModelByNamespaceAndName(s *store.Store, namespace, name string) (store.ModelInfo, bool, error) {
	if name == "" {
		return store.ModelInfo{}, false, errors.New("model name cannot be empty")
	}

	keys := s.Keys()
	const prefix = "model:"
	for _, k := range keys {
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}
		raw, ok := s.Get(k)
		if !ok {
			continue
		}
		var info store.ModelInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return store.ModelInfo{}, false, fmt.Errorf("unmarshal model %q: %w", k, err)
		}
		if info.Namespace == namespace && info.Name == name {
			return info, true, nil
		}
	}

	return store.ModelInfo{}, false, nil
}

func GetNodesByModelName(s *store.Store, namespace, modelName string) (string, []NodeAddress, error) {
	if modelName == "" {
		return "", nil, fmt.Errorf("model name cannot be empty")
	}

	modelInfo, found, err := GetModelByNamespaceAndName(s, namespace, modelName)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get model by name: %w", err)
	}
	if !found {
		return "", nil, fmt.Errorf("model not found")
	}

	replicas, err := replicascheduler.ListReplicasByModelID(s, modelInfo.ID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list replicas: %w", err)
	}

	// create a map of replica IDs
	replicaIDMap := make(map[string]bool)
	for _, req := range replicas {
		replicaIDMap[req.ID] = true
	}

	var nodeAddresses []NodeAddress
	nodeSeen := make(map[string]bool)

	nodes, err := ListNodes(s)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		for _, assignedModel := range node.AssignedModels {
			if replicaIDMap[assignedModel] {
				if !nodeSeen[node.ID] {
					nodeAddresses = append(nodeAddresses, NodeAddress{
						NodeID: node.ID,
						IP:     node.IP,
						Port:   int32(node.Port),
					})
					nodeSeen[node.ID] = true
				}
				break
			}
		}
	}

	return modelInfo.ID, nodeAddresses, nil
}
