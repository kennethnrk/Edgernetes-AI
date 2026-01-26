package replicascheduler

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// CreateReplica stores a new ReplicaInfo under the given replicaID.
func CreateReplica(s *store.Store, replicaID string, info store.ReplicaInfo) error {
	if replicaID == "" {
		return errors.New("replicaID cannot be empty")
	}

	// Ensure the stored ReplicaInfo has a consistent ID.
	if info.ID == "" {
		info.ID = replicaID
	} else if info.ID != replicaID {
		return fmt.Errorf("replica info ID %q does not match replicaID %q", info.ID, replicaID)
	}

	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal replica info: %w", err)
	}
	return s.Put("replica:"+replicaID, b)
}

// GetReplicaByID loads a ReplicaInfo by ID.
// Returns (zero ReplicaInfo, false, nil) if the replica is not found.
func GetReplicaByID(s *store.Store, replicaID string) (store.ReplicaInfo, bool, error) {
	if replicaID == "" {
		return store.ReplicaInfo{}, false, errors.New("replicaID cannot be empty")
	}

	raw, ok := s.Get("replica:" + replicaID)
	if !ok {
		return store.ReplicaInfo{}, false, nil
	}

	var info store.ReplicaInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return store.ReplicaInfo{}, false, fmt.Errorf("unmarshal replica info: %w", err)
	}
	return info, true, nil
}

// UpdateReplicaInfo replaces the stored ReplicaInfo for a replicaID.
func UpdateReplicaInfo(s *store.Store, replicaID string, info store.ReplicaInfo) error {
	if replicaID == "" {
		return errors.New("replicaID cannot be empty")
	}

	// Ensure ID consistency.
	if info.ID != "" && info.ID != replicaID {
		return fmt.Errorf("replica info ID %q does not match replicaID %q", info.ID, replicaID)
	}
	info.ID = replicaID

	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal replica info: %w", err)
	}
	return s.Put("replica:"+replicaID, b)
}

// DeleteReplica removes a replica from the store.
func DeleteReplica(s *store.Store, replicaID string) error {
	if replicaID == "" {
		return errors.New("replicaID cannot be empty")
	}
	return s.Delete("replica:" + replicaID)
}

// ListReplicas returns all ReplicaInfo records currently in the store.
func ListReplicas(s *store.Store) ([]store.ReplicaInfo, error) {
	keys := s.Keys()
	replicas := make([]store.ReplicaInfo, 0, len(keys))

	for _, k := range keys {
		// Only consider keys for replicas.
		const prefix = "replica:"
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}

		raw, ok := s.Get(k)
		if !ok {
			continue
		}
		var info store.ReplicaInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return nil, fmt.Errorf("unmarshal replica %q: %w", k, err)
		}
		replicas = append(replicas, info)
	}

	return replicas, nil
}

// ListReplicasByModelID returns all ReplicaInfo records for a specific modelID.
func ListReplicasByModelID(s *store.Store, modelID string) ([]store.ReplicaInfo, error) {
	if modelID == "" {
		return nil, errors.New("modelID cannot be empty")
	}

	allReplicas, err := ListReplicas(s)
	if err != nil {
		return nil, err
	}

	replicas := make([]store.ReplicaInfo, 0)
	for _, replica := range allReplicas {
		if replica.ModelID == modelID {
			replicas = append(replicas, replica)
		}
	}

	return replicas, nil
}
