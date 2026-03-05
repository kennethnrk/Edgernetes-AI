package statuscontroller

import (
	"fmt"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	replicascheduler "github.com/kennethnrk/edgernetes-ai/internal/control-plane/scheduler/replica"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type ReplicaStatusBreakdown struct {
	Running int32
	Pending int32
	Failed  int32
	Unknown int32
}

type ModelStatusResult struct {
	ModelName     string
	ModelID       string
	Status        constants.ModelStatus
	TotalReplicas int32
	Breakdown     ReplicaStatusBreakdown
}

func GetModelStatus(s *store.Store, namespace, modelName string) (ModelStatusResult, error) {
	if modelName == "" {
		return ModelStatusResult{}, fmt.Errorf("model name cannot be empty")
	}

	modelInfo, found, err := registrycontroller.GetModelByNamespaceAndName(s, namespace, modelName)
	if err != nil {
		return ModelStatusResult{}, fmt.Errorf("failed to get model by name: %w", err)
	}
	if !found {
		return ModelStatusResult{}, fmt.Errorf("model not found")
	}

	replicas, err := replicascheduler.ListReplicasByModelID(s, modelInfo.ID)
	if err != nil {
		return ModelStatusResult{}, fmt.Errorf("failed to list replicas: %w", err)
	}

	var breakdown ReplicaStatusBreakdown
	for _, req := range replicas {
		switch req.Status {
		case constants.ModelReplicaStatusRunning:
			breakdown.Running++
		case constants.ModelReplicaStatusPending:
			breakdown.Pending++
		case constants.ModelReplicaStatusFailed:
			breakdown.Failed++
		default:
			breakdown.Unknown++
		}
	}

	total := int32(len(replicas))

	var status constants.ModelStatus
	if total == 0 {
		status = constants.ModelStatusPending
	} else if breakdown.Running == total {
		status = constants.ModelStatusRunning
	} else if breakdown.Pending == total {
		status = constants.ModelStatusPending
	} else if breakdown.Failed == total {
		status = constants.ModelStatusFailed
	} else if breakdown.Running > 0 {
		status = constants.ModelStatusPartialRunning
	} else {
		// mixed but no running
		status = constants.ModelStatusPending
	}

	return ModelStatusResult{
		ModelName:     modelInfo.Name,
		ModelID:       modelInfo.ID,
		Status:        status,
		TotalReplicas: total,
		Breakdown:     breakdown,
	}, nil
}
