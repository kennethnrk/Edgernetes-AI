package agentmonitor

import (
	"log"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

func CheckHealth(a *agent.Agent) ([]agent.ModelReplicaDetails, bool, error) {

	success := true
	for _, model := range a.AssignedModels {

		if model.Status != constants.ModelReplicaStatusRunning {
			success = false
			break
		}
	}
	return a.AssignedModels, success, nil
}

func MonitorHeartbeatStaleness(agentInfo *agent.Agent, controlPlaneAddress string, registerFn func(string, *agent.Agent) error, deregisterFn func(string, string) error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if agentInfo.IsHeartbeatStale(60 * time.Second) {
			log.Printf("Heartbeat is stale (no request for >60s). Re-registering node %s...", agentInfo.ID)

			// Attempt to deregister first
			if agentInfo.ID != "" {
				if err := deregisterFn(controlPlaneAddress, agentInfo.ID); err != nil {
					log.Printf("Failed to deregister with control-plane (ignoring): %v", err)
				}
			}

			if err := registerFn(controlPlaneAddress, agentInfo); err != nil {
				log.Printf("Failed to re-register with control-plane: %v", err)
			} else {
				log.Printf("Successfully re-registered agent. Node ID: %s", agentInfo.ID)
				agentInfo.UpdateLastHeartbeat() // reset heartbeat timer after successful re-registration
			}
		}
	}
}
