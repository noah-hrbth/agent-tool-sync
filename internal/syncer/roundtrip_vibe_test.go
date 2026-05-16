package syncer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// TestRoundtripVibe verifies Render → AdoptExternal round-trips for the
// Mistral Vibe adapter at both scopes. Reversible map is skills-only because:
//   - AGENTS.md adoption is covered by explicit TestAdoptVibeAgentsMD* tests
//     (no per-rule canonical entity exists for Vibe).
//   - .vibe/skills/<command-name>/SKILL.md (command-as-skill) round-trips back
//     as a canonical skill, not a command — vendor-recommended canonical form,
//     documented in the adapter's ConceptInfo for ConceptCommands.
//   - .vibe/agents/*.toml + .vibe/prompts/*.md TOML reverse mapping is deferred
//     (parity with Codex CLI's TOML-agent gap).
func TestRoundtripVibe(t *testing.T) {
	probe := &canonical.Canonical{
		AgentsMD: "# Probe\n\nRoot memory body.\n",
		Rules: []*canonical.Rule{{
			Filename: "sample-rule",
			Body:     "Rule body content.\n",
		}},
		Skills: []*canonical.Skill{{
			Dir:         "sample-skill",
			Name:        "sample-skill",
			Description: "Probe skill description",
			Body:        "Skill instructions.\n",
		}},
		Agents: []*canonical.Agent{{
			Filename:    "sample-agent",
			Name:        "sample-agent",
			Description: "Probe agent description",
			Tools:       []string{"read_file", "grep"},
			Model:       "mistral-medium-3.5",
			Body:        "Agent system prompt.\n",
		}},
		Commands: []*canonical.Command{{
			Filename:    "sample-command",
			Description: "Probe command description",
			Body:        "Command prompt body.\n",
		}},
	}

	cases := []struct {
		scope      tools.Scope
		reversible map[string]string
	}{
		{
			scope: tools.ScopeProject,
			reversible: map[string]string{
				".vibe/skills/sample-skill/SKILL.md": "skill",
			},
		},
		{
			scope: tools.ScopeUser,
			reversible: map[string]string{
				".vibe/skills/sample-skill/SKILL.md": "skill",
			},
		},
	}

	for _, tc := range cases {
		t.Run("Mistral Vibe/"+tc.scope.String(), func(t *testing.T) {
			ws := t.TempDir()
			for _, sub := range []string{"rules", "skills", "agents", "commands"} {
				if err := os.MkdirAll(filepath.Join(ws, ".agentsync", sub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte(probe.AgentsMD), 0o644); err != nil {
				t.Fatal(err)
			}

			var adapter tools.Tool
			found := false
			for _, a := range tools.All() {
				if a.Meta.Name == "Mistral Vibe" {
					adapter = a
					found = true
					break
				}
			}
			if !found {
				t.Fatal("adapter Mistral Vibe not registered")
			}

			writes, err := adapter.Render(probe, tc.scope)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			for _, fw := range writes {
				abs := filepath.Join(ws, fw.Path)
				if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(abs, fw.Content, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			for path := range tc.reversible {
				if err := syncer.AdoptExternal(ws, path); err != nil {
					t.Errorf("AdoptExternal(%q): unexpected error: %v", path, err)
				}
			}

			// Verify the legitimate canonical skill came back with the right body.
			saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "sample-skill", "SKILL.md"))
			if err != nil {
				t.Fatalf("read canonical skill: %v", err)
			}
			if !strings.Contains(string(saved), "Skill instructions.") {
				t.Errorf("canonical skill body missing in roundtrip: %s", string(saved))
			}
		})
	}
}
