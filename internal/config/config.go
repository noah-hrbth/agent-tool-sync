package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"

	"github.com/noah-hrbth/agentsync/internal/safepath"
)

// Load reads .agentsync/config.yaml from workspace. If the file is missing,
// it returns Default(toolNames) without error.
func Load(workspace string, toolNames []string) (*Config, error) {
	data, err := safepath.ReadFile(workspace, filepath.Join(".agentsync", "config.yaml"))
	if errors.Is(err, os.ErrNotExist) {
		return Default(toolNames), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

// Save writes cfg to <workspace>/.agentsync/config.yaml, creating the directory if needed.
func Save(workspace string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := safepath.WriteFile(workspace, filepath.Join(".agentsync", "config.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
