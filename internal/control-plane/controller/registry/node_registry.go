package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// RegisterNode stores a new NodeInfo under the given nodeID.
func RegisterNode(s *store.Store, nodeID string, nodeInfo store.NodeInfo) error {
	if nodeID == "" {
		return errors.New("nodeID cannot be empty")
	}

	// Ensure the stored NodeInfo has a consistent ID and timestamps.
	if nodeInfo.ID == "" {
		nodeInfo.ID = nodeID
	}
	now := time.Now()
	if nodeInfo.RegisteredAt.IsZero() {
		nodeInfo.RegisteredAt = now
	}
	nodeInfo.UpdatedAt = now
	if nodeInfo.LastActivity.IsZero() {
		nodeInfo.LastActivity = now
	}

	deviceInfoBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("marshal node info: %w", err)
	}
	return s.Put("node:"+nodeID, deviceInfoBytes)
}

// DeRegisterNode removes a node from the store.
func DeRegisterNode(s *store.Store, nodeID string) error {
	if nodeID == "" {
		return errors.New("nodeID cannot be empty")
	}
	return s.Delete("node:" + nodeID)
}

// UpdateNodeInfo replaces the stored NodeInfo for a nodeID.
func UpdateNodeInfo(s *store.Store, nodeID string, info store.NodeInfo) error {
	if nodeID == "" {
		return errors.New("nodeID cannot be empty")
	}

	// Ensure ID consistency.
	if info.ID != "" && info.ID != nodeID {
		return fmt.Errorf("node info ID %q does not match nodeID %q", info.ID, nodeID)
	}
	info.ID = nodeID

	// Preserve original RegisteredAt if the node already exists.
	if existing, found, err := GetNodeByID(s, nodeID); err != nil {
		return err
	} else if found {
		if info.RegisteredAt.IsZero() {
			info.RegisteredAt = existing.RegisteredAt
		}
		if info.LastHeartbeat.IsZero() {
			info.LastHeartbeat = existing.LastHeartbeat
		}
		if info.LastActivity.IsZero() {
			info.LastActivity = existing.LastActivity
		}
	}

	now := time.Now()
	info.UpdatedAt = now
	if info.LastActivity.IsZero() {
		info.LastActivity = now
	}

	b, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal node info: %w", err)
	}
	return s.Put("node:"+nodeID, b)
}

// UpdateNodeStatus updates only the Status (and related timestamps) of a node.
func UpdateNodeStatus(s *store.Store, nodeID string, status store.Status) error {
	if nodeID == "" {
		return errors.New("nodeID cannot be empty")
	}

	var info store.NodeInfo
	raw, found, err := getNodeRaw(s, nodeID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("node %q not found", nodeID)
	}
	if err := json.Unmarshal(raw, &info); err != nil {
		return fmt.Errorf("unmarshal node info: %w", err)
	}

	now := time.Now()
	info.Status = status
	info.UpdatedAt = now

	// Treat any status change as a heartbeat for now.
	info.LastHeartbeat = now
	if info.LastActivity.IsZero() {
		info.LastActivity = now
	}

	b, err := json.Marshal(&info)
	if err != nil {
		return fmt.Errorf("marshal updated node info: %w", err)
	}
	return s.Put("node:"+nodeID, b)
}

// GetNodeByID loads a NodeInfo by ID.
// Returns (zero NodeInfo, false, nil) if the node is not found.
func GetNodeByID(s *store.Store, nodeID string) (store.NodeInfo, bool, error) {
	if nodeID == "" {
		return store.NodeInfo{}, false, errors.New("nodeID cannot be empty")
	}

	raw, ok := s.Get("node:" + nodeID)
	if !ok {
		return store.NodeInfo{}, false, nil
	}

	var info store.NodeInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return store.NodeInfo{}, false, fmt.Errorf("unmarshal node info: %w", err)
	}
	return info, true, nil
}

// ListNodes returns all NodeInfo records currently in the store.
func ListNodes(s *store.Store) ([]store.NodeInfo, error) {
	keys := s.Keys()
	nodes := make([]store.NodeInfo, 0, len(keys))

	for _, k := range keys {
		// Only consider keys for nodes.
		const prefix = "node:"
		if len(k) < len(prefix) || k[:len(prefix)] != prefix {
			continue
		}

		raw, ok := s.Get(k)
		if !ok {
			continue
		}
		var info store.NodeInfo
		if err := json.Unmarshal(raw, &info); err != nil {
			return nil, fmt.Errorf("unmarshal node %q: %w", k, err)
		}
		nodes = append(nodes, info)
	}

	return nodes, nil
}

// getNodeRaw is a small helper to fetch the raw JSON for a node.
func getNodeRaw(s *store.Store, nodeID string) ([]byte, bool, error) {
	if nodeID == "" {
		return nil, false, errors.New("nodeID cannot be empty")
	}
	raw, ok := s.Get("node:" + nodeID)
	return raw, ok, nil
}
