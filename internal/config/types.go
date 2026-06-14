package config

// Config is loaded from .agentsync/config.yaml and persisted on change.
type Config struct {
	Tools     map[string]ToolConfig `yaml:"tools"`
	Gitignore GitignoreConfig       `yaml:"gitignore"`
}

// ToolConfig holds per-tool sync preferences.
type ToolConfig struct {
	Enabled bool `yaml:"enabled"`
}

// GitignoreConfig records the user's choice for managing a .gitignore block
// covering derived per-tool dirs/files. Manage drives whether sync writes or
// refreshes the block; Prompted gates the first-sync prompt (CLI/TUI).
type GitignoreConfig struct {
	Manage   bool `yaml:"manage"`
	Prompted bool `yaml:"prompted"`
}

// Default returns a Config with all supported tools enabled.
func Default(toolNames []string) *Config {
	cfg := &Config{Tools: make(map[string]ToolConfig, len(toolNames))}
	for _, name := range toolNames {
		cfg.Tools[name] = ToolConfig{Enabled: true}
	}
	return cfg
}

// WithEnabled returns a Config listing every tool in toolNames, enabling only
// those whose name appears in enabledNames. Names in enabledNames that are not
// in toolNames are silently ignored.
func WithEnabled(toolNames, enabledNames []string) *Config {
	enabled := make(map[string]bool, len(enabledNames))
	for _, name := range enabledNames {
		enabled[name] = true
	}
	cfg := &Config{Tools: make(map[string]ToolConfig, len(toolNames))}
	for _, name := range toolNames {
		cfg.Tools[name] = ToolConfig{Enabled: enabled[name]}
	}
	return cfg
}

// IsEnabled reports whether the named tool is enabled for syncing.
func (c *Config) IsEnabled(toolName string) bool {
	tc, ok := c.Tools[toolName]
	return ok && tc.Enabled
}
