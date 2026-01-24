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
