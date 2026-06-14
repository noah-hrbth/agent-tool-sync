package syncer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func TestScaffoldCreatesSkeletonAndConfig(t *testing.T) {
	ws := t.TempDir()
	// distinctive config: only Claude Code enabled, so Load can't be Default
	cfg := config.WithEnabled(tools.Names(), []string{"Claude Code"})

	if err := syncer.Scaffold(ws, tools.ScopeProject, cfg); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	for _, sub := range []string{"skills", "agents", "commands", "rules", ".state"} {
		info, err := os.Stat(filepath.Join(ws, ".agentsync", sub))
		if err != nil {
			t.Errorf(".agentsync/%s: %v", sub, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf(".agentsync/%s is not a directory", sub)
		}
	}
	starter, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read starter AGENTS.md: %v", err)
	}
	if !strings.Contains(string(starter), "# Project Rules") {
		t.Errorf("starter AGENTS.md = %q, want project wording", starter)
	}
	loaded, err := config.Load(ws, tools.Names())
	if err != nil {
		t.Fatalf("config.Load after scaffold: %v", err)
	}
	if !loaded.IsEnabled("Claude Code") {
		t.Error("config: Claude Code should be enabled")
	}
	if loaded.IsEnabled("Cursor") {
		t.Error("config: Cursor should be disabled (passed cfg must be persisted, not Default)")
	}
}

func TestScaffoldUserScopeStarterText(t *testing.T) {
	ws := t.TempDir()

	if err := syncer.Scaffold(ws, tools.ScopeUser, config.Default(tools.Names())); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	starter, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read starter AGENTS.md: %v", err)
	}
	if !strings.Contains(string(starter), "# User Rules") {
		t.Errorf("starter AGENTS.md = %q, want user wording", starter)
	}
}

func TestScaffoldPreservesExistingAgentsMD(t *testing.T) {
	ws := t.TempDir()
	existing := "# Hand-authored root memory\n"
	seedImportFile(t, ws, ".agentsync/AGENTS.md", []byte(existing))

	if err := syncer.Scaffold(ws, tools.ScopeProject, config.Default(tools.Names())); err != nil {
		t.Fatalf("Scaffold: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	if string(saved) != existing {
		t.Errorf("AGENTS.md = %q, want existing content preserved", saved)
	}
}
