package store

import (
	"encoding/json"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

type ModelInfo struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Version     string              `json:"version"`
	FilePath    string              `json:"file_path"`
	ModelType   constants.ModelType `json:"model_type"`
	ModelSize   int64               `json:"model_size"`
	Replicas    int                 `json:"replicas"`
	InputFormat json.RawMessage     `json:"input_format"`
	AssignedTo  string              `json:"assigned_to"`
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
