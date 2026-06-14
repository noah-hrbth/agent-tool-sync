package syncer

import (
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/safepath"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// Scaffold creates the .agentsync/ skeleton under ws (concept dirs plus
// .state), writes a scope-worded starter AGENTS.md only when none exists, and
// persists cfg to .agentsync/config.yaml. Safe to call on an already
// initialized workspace.
func Scaffold(ws string, scope tools.Scope, cfg *config.Config) error {
	for _, sub := range []string{".", "skills", "agents", "commands", "rules", ".state"} {
		if err := safepath.MkdirAll(ws, filepath.Join(".agentsync", sub), 0o755); err != nil {
			return err
		}
	}

	agentsRel := filepath.Join(".agentsync", "AGENTS.md")
	if _, err := os.Stat(filepath.Join(ws, agentsRel)); os.IsNotExist(err) {
		if err := safepath.WriteFile(ws, agentsRel, []byte(starterAgentsMD(scope)), 0o644); err != nil {
			return err
		}
	}

	return config.Save(ws, cfg)
}

// starterAgentsMD returns the scope-appropriate starter root-memory content.
func starterAgentsMD(scope tools.Scope) string {
	if scope == tools.ScopeUser {
		return "# User Rules\n\nPersonal AI agent instructions applied across all your projects.\n" +
			"This file is synced to user-level tool config dirs (~/.claude, ~/.codex, etc.) by agentsync.\n"
	}
	return "# Project Rules\n\nAdd your AI agent instructions here.\n" +
		"This file is synced to all enabled AI tools by agentsync.\n"
}
