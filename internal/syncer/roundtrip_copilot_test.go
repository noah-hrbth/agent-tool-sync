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

// TestRoundtripCopilot renders a probe canonical through the GitHub Copilot
// adapter at both scopes, writes outputs to a temp workspace, then calls
// AdoptExternal on each reversible path. Catches drift between Render path
// conventions and adopt.go matchers.
func TestRoundtripCopilot(t *testing.T) {
	probe := &canonical.Canonical{
		AgentsMD: "# Probe\n\nRoot memory body.\n",
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

	cases := []struct {
		name       string
		scope      tools.Scope
		reversible map[string]string // path → kind ("rule"|"skill"|"agent"|"command")
	}{
		{
			name:  "project",
			scope: tools.ScopeProject,
			reversible: map[string]string{
				".github/copilot-instructions.md":                  "agentsmd",
				".github/instructions/sample-rule.instructions.md": "rule",
				".github/skills/sample-skill/SKILL.md":             "skill",
				".github/agents/sample-agent.agent.md":             "agent",
				".github/prompts/sample-command.prompt.md":         "command",
			},
		},
		{
			name:  "user",
			scope: tools.ScopeUser,
			reversible: map[string]string{
				".copilot/copilot-instructions.md":                  "agentsmd",
				".copilot/instructions/sample-rule.instructions.md": "rule",
				".copilot/skills/sample-skill/SKILL.md":             "skill",
				".copilot/agents/sample-agent.agent.md":             "agent",
				// no user-scope command path
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				if a.Meta.Name == "GitHub Copilot" {
					adapter = a
					found = true
					break
				}
			}
			if !found {
				t.Fatal("adapter \"GitHub Copilot\" not registered")
			}

			writes, err := adapter.Render(probe, tc.scope)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			// Cross-check: every "reversible" path must be present in writes.
			emitted := map[string]bool{}
			for _, fw := range writes {
				emitted[fw.Path] = true
			}
			for path := range tc.reversible {
				if !emitted[path] {
					t.Errorf("adapter did not emit expected path %q; got %v", path, pathsList(writes))
				}
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
					t.Errorf("AdoptExternal(%q): %v", path, err)
				}
			}

			checks := map[string]string{
				"agentsmd": filepath.Join(".agentsync", "AGENTS.md"),
				"rule":     filepath.Join(".agentsync", "rules", "sample-rule.md"),
				"skill":    filepath.Join(".agentsync", "skills", "sample-skill", "SKILL.md"),
				"agent":    filepath.Join(".agentsync", "agents", "sample-agent.md"),
				"command":  filepath.Join(".agentsync", "commands", "sample-command.md"),
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
				switch kind {
				case "agentsmd":
					if !strings.Contains(string(data), "Root memory body.") {
						t.Errorf("canonical AGENTS.md body missing: %s", string(data))
					}
				case "rule":
					if !strings.Contains(string(data), "Rule body content.") {
						t.Errorf("canonical rule body missing: %s", string(data))
					}
					if !strings.Contains(string(data), "src/**/*.ts") {
						t.Errorf("canonical rule should preserve applyTo as Paths; got: %s", string(data))
					}
				case "skill":
					if !strings.Contains(string(data), "Skill instructions.") {
						t.Errorf("canonical skill body missing: %s", string(data))
					}
				case "agent":
					if !strings.Contains(string(data), "Agent system prompt.") {
						t.Errorf("canonical agent body missing: %s", string(data))
					}
				case "command":
					if !strings.Contains(string(data), "Command prompt body.") {
						t.Errorf("canonical command body missing: %s", string(data))
					}
				}
			}
		})
	}
}

func pathsList(writes []tools.FileWrite) []string {
	out := make([]string, len(writes))
	for i, w := range writes {
		out[i] = w.Path
	}
	return out
}
