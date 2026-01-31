package heartbeatcontroller

import (
	"log"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	heartbeatcaller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/heartbeat"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	replicascheduler "github.com/kennethnrk/edgernetes-ai/internal/control-plane/scheduler/replica"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

func HandleHeartbeat(s *store.Store) error {
	log.Println("Heartbeat controller started")
	nodes, err := registrycontroller.ListNodesByStatuses(s, []constants.Status{constants.StatusOnline, constants.StatusUnknown})
	if err != nil {
		return err
	}
	for _, node := range nodes {
		resp, err := heartbeatcaller.CallHeartbeat(node)
		if err != nil {
			log.Printf("Failed to call heartbeat for node %s", node.ID)
			if node.LastHeartbeat.Add(40 * time.Second).Before(time.Now()) {
				log.Printf("Node %s has not sent heartbeat in 40 seconds, setting status to offline", node.ID)
				registrycontroller.UpdateNodeStatus(s, node.ID, constants.StatusOffline)
			} else {
				registrycontroller.UpdateNodeStatus(s, node.ID, constants.StatusUnknown)
			}
			continue
		}
		log.Printf("Heartbeat response for node %s", node.ID)

		// Update status of all replicas in the node based on the response
		for _, replicaID := range node.AssignedModels {
			// Try to find the replica in the response
			var foundReplica *heartbeatpb.ModelReplicaDetails
			for _, replica := range resp.GetModelReplicas() {
				if replica.GetReplicaId() == replicaID {
					foundReplica = replica
					break
				}
			}

			// Get the existing replica info from the store
			replicaInfo, exists, err := replicascheduler.GetReplicaByID(s, replicaID)
			if err != nil {
				log.Printf("Failed to get replica %s from store: %v", replicaID, err)
				continue
			}

			if !exists {
				log.Printf("Replica %s not found in store, skipping update", replicaID)
				continue
			}

			// Update replica status based on whether it was found in the response
			if foundReplica != nil {
				// Replica found in response - update with status from response
				status := convertStringToReplicaStatus(foundReplica.GetStatus())
				replicaInfo.Status = status
				replicaInfo.ErrorCode = int(foundReplica.GetErrorCode())
				replicaInfo.ErrorMessage = foundReplica.GetErrorMessage()
				replicaInfo.LastHeartbeat = time.Now()
				log.Printf("Updating replica %s with status: %s", replicaID, status)
			} else {
				// Replica not found in response - set to unknown
				replicaInfo.Status = constants.ModelReplicaStatusUnknown
				replicaInfo.LastHeartbeat = time.Now()
				log.Printf("Replica %s not found in response, setting status to unknown", replicaID)
			}

			// Save the updated replica info
			if err := replicascheduler.UpdateReplicaInfo(s, replicaID, replicaInfo); err != nil {
				log.Printf("Failed to update replica %s: %v", replicaID, err)
				continue
			}
		}
	}
	log.Println("Heartbeat controller finished")
	return nil
}

// convertStringToReplicaStatus converts a string status to ModelReplicaStatus constant.
func convertStringToReplicaStatus(statusStr string) constants.ModelReplicaStatus {
	switch statusStr {
	case "pending":
		return constants.ModelReplicaStatusPending
	case "running":
		return constants.ModelReplicaStatusRunning
	case "completed":
		return constants.ModelReplicaStatusCompleted
	case "failed":
		return constants.ModelReplicaStatusFailed
	case "unknown":
		return constants.ModelReplicaStatusUnknown
	default:
		return constants.ModelReplicaStatusUnknown
	}
}

// startHeartbeatHandler runs the heartbeat handler periodically in a separate goroutine.
func StartHeartbeatHandler(store *store.Store, interval time.Duration) {
	// Get heartbeat interval from environment variable, default to 10 seconds

	log.Printf("Starting heartbeat handler with interval: %v", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on startup
	if err := HandleHeartbeat(store); err != nil {
		log.Printf("Error in initial heartbeat: %v", err)
	}

	// Then run periodically
	for range ticker.C {
		if err := HandleHeartbeat(store); err != nil {
			log.Printf("Error in heartbeat handler: %v", err)
		}
	}
}
