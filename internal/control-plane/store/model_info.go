package store

import (
	"encoding/json"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

type ModelInfo struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Version        string              `json:"version"`
	FilePath       string              `json:"file_path"`
	ModelType      constants.ModelType `json:"model_type"`
	ModelSize      int64               `json:"model_size"`
	Replicas       int                 `json:"replicas"`
	ActiveReplicas int                 `json:"active_replicas"`
	ReplicaIDs     []string            `json:"replica_ids"`
	InputFormat    json.RawMessage     `json:"input_format"`
}

// Examples of input formats:
// Car price DT
// {
//   "name": "string",
//   "price": "number",
//   "year": "number"
// }
// Linear Regression
// {
//   "data": "array"
// }
// CNN
// {
//   "image": "array",
// }
// LLM
// {
//   "prompt": "string",
//   "max_tokens": "number"
// }
