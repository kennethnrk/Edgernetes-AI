package registry

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// RegisterModel stores a new ModelInfo under the given modelID.
func RegisterModel(s *store.Store, modelID string, info store.ModelInfo) error {
	if modelID == "" {
		return errors.New("modelID cannot be empty")
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
