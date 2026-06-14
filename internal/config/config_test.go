package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDefaultGitignoreZero(t *testing.T) {
	cfg := Default([]string{"a", "b"})
	if cfg.Gitignore.Manage {
		t.Errorf("default Manage should be false")
	}
	if cfg.Gitignore.Prompted {
		t.Errorf("default Prompted should be false")
	}
}

func TestConfigGitignoreRoundTripsThroughYAML(t *testing.T) {
	ws := t.TempDir()
	cfg := Default([]string{"a"})
	cfg.Gitignore = GitignoreConfig{Manage: true, Prompted: true}

	if err := Save(ws, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(ws, []string{"a"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !loaded.Gitignore.Manage || !loaded.Gitignore.Prompted {
		t.Fatalf("expected Manage=true Prompted=true, got %+v", loaded.Gitignore)
	}
}

func TestWithEnabledEnablesOnlyListed(t *testing.T) {
	toolNames := []string{"Claude Code", "Cursor", "Zed"}
	cfg := WithEnabled(toolNames, []string{"Claude Code"})

	for _, name := range toolNames {
		tc, ok := cfg.Tools[name]
		if !ok {
			t.Errorf("tool %q missing from config", name)
		}
		want := name == "Claude Code"
		if tc.Enabled != want {
			t.Errorf("tool %q: Enabled=%v, want %v", name, tc.Enabled, want)
		}
	}
}

func TestWithEnabledEmptyEnabledDisablesAll(t *testing.T) {
	toolNames := []string{"Claude Code", "Cursor", "Zed"}
	cfg := WithEnabled(toolNames, []string{})

	for _, name := range toolNames {
		tc, ok := cfg.Tools[name]
		if !ok {
			t.Errorf("tool %q missing from config", name)
		}
		if tc.Enabled {
			t.Errorf("tool %q: expected Enabled=false", name)
		}
	}
}

func TestWithEnabledIgnoresUnknownEnabledNames(t *testing.T) {
	toolNames := []string{"Claude Code", "Cursor"}
	cfg := WithEnabled(toolNames, []string{"Claude Code", "Unknown Tool"})

	if _, ok := cfg.Tools["Unknown Tool"]; ok {
		t.Errorf("unknown tool %q should not appear in config", "Unknown Tool")
	}
	if len(cfg.Tools) != len(toolNames) {
		t.Errorf("expected %d tools, got %d", len(toolNames), len(cfg.Tools))
	}
}

func TestConfigLoadLegacyYAMLWithoutGitignoreFieldDefaultsToZero(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Legacy config: only tools key, no gitignore section.
	legacy := "tools:\n  claude:\n    enabled: true\n"
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "config.yaml"), []byte(legacy), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg, err := Load(ws, []string{"claude"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Gitignore.Manage || cfg.Gitignore.Prompted {
		t.Fatalf("legacy config should leave Gitignore zero, got %+v", cfg.Gitignore)
	}
}
