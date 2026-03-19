package client

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds persisted edgectl settings.
type Config struct {
	Endpoint         string `yaml:"endpoint"`
	DefaultNamespace string `yaml:"default_namespace"`
	OutputFormat     string `yaml:"output_format"`
	TimeoutSeconds   int    `yaml:"timeout_seconds"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Endpoint:         "localhost:50051",
		DefaultNamespace: "default",
		OutputFormat:     "table",
		TimeoutSeconds:   10,
	}
}

// configDir returns the path to ~/.edgectl/ (creating it if necessary).
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".edgectl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}
	return dir, nil
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// LoadConfig reads the config from disk, or returns defaults if the file
// does not exist.
func LoadConfig() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes the config to disk.
func SaveConfig(cfg *Config) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not serialize config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	return nil
}
