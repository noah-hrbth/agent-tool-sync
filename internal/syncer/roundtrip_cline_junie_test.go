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

// TestRoundtripClineJunie renders a probe canonical for both new adapters at
// both scopes, writes the output to a temp workspace, then calls AdoptExternal
// on every reversible path. Catches drift between Render path conventions and
// adopt.go matchers for Cline + JetBrains Junie.
func TestRoundtripClineJunie(t *testing.T) {
	probe := &canonical.Canonical{
		AgentsMD: "# Probe\n\nRoot memory body with multi-line content.\n",
		Rules: []*canonical.Rule{{
			Filename: "sample-rule",
			Body:     "Rule body content.\n",
			Paths:    []string{"src/**/*.ts"},
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
			Tools:       []string{"Read", "Grep"},
			Model:       "sonnet",
			Body:        "Agent system prompt.\n",
		}},
		Commands: []*canonical.Command{{
			Filename:    "sample-command",
			Description: "Probe command description",
			Body:        "Command prompt body.\n",
		}},
	}

	type expectation struct {
		path   string
		kind   string // "skill" | "agent" | "command" | "rule"
		expect bool   // true = adopt should succeed; false = path is intentionally non-reversible
	}

	cases := []struct {
		adapter   string
		scope     tools.Scope
		reversible map[string]string // path → kind expected to be reversible
	}{
		{
			adapter: "Cline",
			scope:   tools.ScopeProject,
			reversible: map[string]string{
				".clinerules/sample-rule.md":               "rule",
				".cline/skills/sample-skill/SKILL.md":      "skill",
				".clinerules/workflows/sample-command.md":  "command",
			},
		},
		{
			adapter: "Cline",
			scope:   tools.ScopeUser,
			reversible: map[string]string{
				"Documents/Cline/Rules/sample-rule.md":          "rule",
				".cline/skills/sample-skill/SKILL.md":           "skill",
				"Documents/Cline/Workflows/sample-command.md":   "command",
			},
		},
		{
			adapter: "JetBrains Junie",
			scope:   tools.ScopeProject,
			reversible: map[string]string{
				".junie/skills/sample-skill/SKILL.md":   "skill",
				".junie/agents/sample-agent.md":         "agent",
				".junie/commands/sample-command.md":     "command",
			},
		},
		{
			adapter: "JetBrains Junie",
			scope:   tools.ScopeUser,
			reversible: map[string]string{
				".junie/skills/sample-skill/SKILL.md": "skill",
				".junie/agents/sample-agent.md":       "agent",
				".junie/commands/sample-command.md":   "command",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.adapter+"/"+tc.scope.String(), func(t *testing.T) {
			ws := t.TempDir()
			// Seed the canonical dir so SaveX has somewhere to write.
			for _, sub := range []string{"rules", "skills", "agents", "commands"} {
				if err := os.MkdirAll(filepath.Join(ws, ".agentsync", sub), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			// Write initial AGENTS.md so reload works.
			if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte(probe.AgentsMD), 0o644); err != nil {
				t.Fatal(err)
			}

			var adapter tools.Adapter
			for _, a := range tools.All() {
				if a.Name() == tc.adapter {
					adapter = a
					break
				}
			}
			if adapter == nil {
				t.Fatalf("adapter %q not registered", tc.adapter)
			}

			writes, err := adapter.Render(probe, tc.scope)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			// Write every output to disk.
			for _, fw := range writes {
				abs := filepath.Join(ws, fw.Path)
				if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(abs, fw.Content, 0o644); err != nil {
					t.Fatal(err)
				}
			}

			// Adopt every path declared reversible; expect success.
			for path := range tc.reversible {
				if err := syncer.AdoptExternal(ws, path); err != nil {
					t.Errorf("AdoptExternal(%q): unexpected error: %v", path, err)
				}
			}

			// Verify the adopted entities exist in canonical with non-empty content.
			checks := map[string]string{
				"rule":    filepath.Join(".agentsync", "rules", "sample-rule.md"),
				"skill":   filepath.Join(".agentsync", "skills", "sample-skill", "SKILL.md"),
				"agent":   filepath.Join(".agentsync", "agents", "sample-agent.md"),
				"command": filepath.Join(".agentsync", "commands", "sample-command.md"),
			}
			seenKinds := map[string]bool{}
			for _, kind := range tc.reversible {
				seenKinds[kind] = true
			}
			for kind, rel := range checks {
				if !seenKinds[kind] {
					continue
				}
				abs := filepath.Join(ws, rel)
				data, err := os.ReadFile(abs)
				if err != nil {
					t.Errorf("expected canonical %s at %s: %v", kind, rel, err)
					continue
				}
				if len(data) == 0 {
					t.Errorf("canonical %s empty at %s", kind, rel)
				}
				// Sanity: the body should be in the adopted file.
				switch kind {
				case "rule":
					if !strings.Contains(string(data), "Rule body content.") {
						t.Errorf("canonical rule body missing in roundtrip: %s", string(data))
					}
				case "skill":
					if !strings.Contains(string(data), "Skill instructions.") {
						t.Errorf("canonical skill body missing in roundtrip: %s", string(data))
					}
				case "agent":
					if !strings.Contains(string(data), "Agent system prompt.") {
						t.Errorf("canonical agent body missing in roundtrip: %s", string(data))
					}
				case "command":
					if !strings.Contains(string(data), "Command prompt body.") {
						t.Errorf("canonical command body missing in roundtrip: %s", string(data))
					}
				}
			}
		})
	}
}
