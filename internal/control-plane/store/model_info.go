package store

import "encoding/json"

type ModelInfo struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Version     string          `json:"version"`
	FilePath    string          `json:"file_path"`
	ModelType   ModelType       `json:"model_type"`
	ModelSize   int64           `json:"model_size"`
	Replicas    int             `json:"replicas"`
	InputFormat json.RawMessage `json:"input_format"`
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

type ModelType string

const (
	ModelTypeVisionTransformer  ModelType = "vision_transformer"
	ModelTypeCNN                ModelType = "cnn"
	ModelTypeMLP                ModelType = "mlp"
	ModelTypeLargeLanguageModel ModelType = "large_language_model"
	ModelTypeDecisionTree       ModelType = "decision_tree"
	ModelTypeLinear             ModelType = "linear"
)
