package agent

import (
	"errors"
	"slices"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type ModelReplicaDetails struct {
	ID        string
	Name      string
	Version   string
	FilePath  string
	ModelType constants.ModelType
	ModelSize int64
	Status    constants.ModelReplicaStatus
	LogFile   string
}

type Agent struct {
	ID                   string
	Name                 string
	IP                   string
	Port                 int
	Metadata             store.NodeMetadata
	ResourceCapabilities store.ResourceCapabilities
	AssignedModels       []ModelReplicaDetails
}

func (a *Agent) AssignModel(model ModelReplicaDetails) error {
	if slices.ContainsFunc(a.AssignedModels, func(m ModelReplicaDetails) bool { return m.ID == model.ID }) {
		return errors.New("model already assigned")
	}
	a.AssignedModels = append(a.AssignedModels, model)
	return nil
}
