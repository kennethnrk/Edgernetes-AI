package constants

type ModelType string

const (
	ModelTypeVisionTransformer  ModelType = "vision_transformer"
	ModelTypeCNN                ModelType = "cnn"
	ModelTypeMLP                ModelType = "mlp"
	ModelTypeLargeLanguageModel ModelType = "large_language_model"
	ModelTypeDecisionTree       ModelType = "decision_tree"
	ModelTypeLinear             ModelType = "linear"
)

type ModelReplicaStatus string

const (
	ModelReplicaStatusUnknown ModelReplicaStatus = "unknown"
	ModelReplicaStatusPending ModelReplicaStatus = "pending"
	ModelReplicaStatusRunning ModelReplicaStatus = "running"
	ModelReplicaStatusFailed  ModelReplicaStatus = "failed"
)

type ModelStatus string

const (
	ModelStatusPending        ModelStatus = "pending"
	ModelStatusRunning        ModelStatus = "running"
	ModelStatusPartialRunning ModelStatus = "partial_running"
	ModelStatusFailed         ModelStatus = "failed"
)
