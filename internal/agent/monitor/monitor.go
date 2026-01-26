package agentmonitor

import (
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
