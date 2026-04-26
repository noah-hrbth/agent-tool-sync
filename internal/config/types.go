package config

// Config is loaded from .agentsync/config.yaml and persisted on change.
type Config struct {
	Tools map[string]ToolConfig `yaml:"tools"`
}

// ToolConfig holds per-tool sync preferences.
type ToolConfig struct {
	Enabled bool `yaml:"enabled"`
}

// Default returns a Config with all supported tools enabled.
func Default(toolNames []string) *Config {
	cfg := &Config{Tools: make(map[string]ToolConfig, len(toolNames))}
	for _, name := range toolNames {
		cfg.Tools[name] = ToolConfig{Enabled: true}
	}
	return cfg
}

// IsEnabled reports whether the named tool is enabled for syncing.
func (c *Config) IsEnabled(toolName string) bool {
	tc, ok := c.Tools[toolName]
	return ok && tc.Enabled
}
