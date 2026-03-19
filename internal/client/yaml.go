package client

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModelManifest is the top-level structure of a YAML manifest file.
type ModelManifest struct {
	APIVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Namespace  string      `yaml:"namespace" json:"namespace"`
	Models     []ModelSpec `yaml:"models" json:"models"`
}

// ModelSpec describes a single model inside a manifest.
type ModelSpec struct {
	Name        string `yaml:"name" json:"name"`
	Namespace   string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	FilePath    string `yaml:"file_path,omitempty" json:"file_path,omitempty"`
	ModelType   string `yaml:"model_type,omitempty" json:"model_type,omitempty"`
	ModelSize   int64  `yaml:"model_size,omitempty" json:"model_size,omitempty"`
	Replicas    int32  `yaml:"replicas,omitempty" json:"replicas,omitempty"`
	InputFormat string `yaml:"input_format,omitempty" json:"input_format,omitempty"`
}

// ParseManifest reads a YAML manifest file and returns the parsed structure.
func ParseManifest(path string) (*ModelManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest %s: %w", path, err)
	}

	var manifest ModelManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("invalid YAML in %s: %w", path, err)
	}

	if err := validateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("manifest validation failed: %w", err)
	}

	return &manifest, nil
}

// validateManifest checks required fields and value constraints.
func validateManifest(m *ModelManifest) error {
	if m.APIVersion != "edgernetes.ai/v1" {
		return fmt.Errorf("unsupported apiVersion %q (expected edgernetes.ai/v1)", m.APIVersion)
	}
	if m.Kind != "ModelManifest" {
		return fmt.Errorf("unsupported kind %q (expected ModelManifest)", m.Kind)
	}
	if len(m.Models) == 0 {
		return fmt.Errorf("manifest must contain at least one model")
	}

	validTypes := map[string]bool{
		"cnn": true, "linear": true, "decision_tree": true, "llm": true, "": true,
	}

	for i, model := range m.Models {
		if model.Name == "" {
			return fmt.Errorf("model[%d]: name is required", i)
		}
		if !validTypes[model.ModelType] {
			return fmt.Errorf("model[%d] %q: invalid model_type %q (expected cnn|linear|decision_tree|llm)",
				i, model.Name, model.ModelType)
		}
	}
	return nil
}

// ResolveNamespace determines the effective namespace for a model using the
// precedence: cliOverride > per-model > file-level > configDefault > "default".
func ResolveNamespace(cliOverride, perModel, fileLevel, configDefault string) string {
	if cliOverride != "" {
		return cliOverride
	}
	if perModel != "" {
		return perModel
	}
	if fileLevel != "" {
		return fileLevel
	}
	if configDefault != "" {
		return configDefault
	}
	return "default"
}
