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

// TestRenderAdoptContract is the drift tripwire: for every tool, every scope,
// every rendered path, the path's declared outcome in tools.ExpectedAdoptOutcome
// must match what syncer.AdoptExternal actually does. A render-path change that
// adopt.go does not handle (and that is not declared non-reversible) fails here.
func TestRenderAdoptContract(t *testing.T) {
	probe := &canonical.Canonical{
		AgentsMD: "# Probe\n\nRoot memory body.\n",
		Rules: []*canonical.Rule{{
			Filename: "sample-rule", Description: "probe rule",
			// Multi-path so Copilot brace-expands to applyTo "{a,b}". The leading "{"
			// is a YAML flow indicator and must be quoted, else adopt fails (!!map).
			Paths: []string{"src/**/*.ts", "test/**/*.ts"}, Body: "Rule body.\n",
		}},
		Skills: []*canonical.Skill{{
			Dir: "sample-skill", Name: "sample-skill",
			Description: "probe skill", Body: "Skill instructions.\n",
			Docs: []canonical.SkillDoc{{RelPath: "reference.md", Content: "# reference\n"}},
		}},
		Agents: []*canonical.Agent{{
			Filename: "sample-agent", Name: "sample-agent",
			Description: "probe agent", Tools: []string{"Read"},
			Model: "sonnet", Body: "Agent prompt.\n",
		}},
		Commands: []*canonical.Command{{
			Filename: "sample-command", Description: "probe command",
			// Bracketed argument-hint is the common case and a YAML-sequence trap:
			// it must be quoted on render or adopt fails (cannot unmarshal !!seq).
			ArgumentHint: "[arg]",
			Body:         "Command body.\n",
		}},
	}

	for _, tool := range tools.All() {
		for _, scope := range []tools.Scope{tools.ScopeProject, tools.ScopeUser} {
			if !tool.Meta.SupportsScope(scope).Supported {
				continue
			}
			writes, err := tool.Render(probe, scope)
			if err != nil {
				t.Fatalf("%s/%s render: %v", tool.Meta.Key, scope, err)
			}
			for _, fw := range writes {
				fw := fw
				name := tool.Meta.Key + "/" + scope.String() + "/" + fw.Path
				t.Run(name, func(t *testing.T) {
					out := tools.ExpectedAdoptOutcome(tool.Meta.Key, fw.Concept, fw.Path)
					ws := writeProbeWorkspace(t, fw)
					adoptErr := syncer.AdoptExternal(ws, fw.Path)

					if out.Kind == tools.OutcomeNonReversible {
						if adoptErr == nil {
							t.Fatalf("declared non-reversible (%s) but AdoptExternal succeeded", out.Reason)
						}
						if !strings.Contains(adoptErr.Error(), "no canonical mapping") {
							t.Fatalf("non-reversible: want \"no canonical mapping\" error, got: %v", adoptErr)
						}
						return
					}
					if adoptErr != nil {
						t.Fatalf("declared reversible/root/cross but AdoptExternal failed: %v\n"+
							"(undeclared drift: path renders but adopt.go cannot reverse it)", adoptErr)
					}

					// A skill doc reverses to a plain file under canonical skills/,
					// not a loadable manifest, so verify the file directly. (In this
					// isolated probe workspace there is no SKILL.md to load.)
					if fw.Concept == tools.ConceptSkills && filepath.Base(fw.Path) != "SKILL.md" {
						docPath := filepath.Join(ws, ".agentsync", "skills", "sample-skill", filepath.Base(fw.Path))
						if _, err := os.Stat(docPath); err != nil {
							t.Fatalf("skill doc adopted but not found in canonical: %v", err)
						}
						return
					}

					c, err := canonical.Load(ws)
					if err != nil {
						t.Fatalf("reload canonical: %v", err)
					}
					switch out.Kind {
					case tools.OutcomeRootMemory:
						if c.AgentsMD == "" {
							t.Fatalf("root-memory path adopted but canonical AGENTS.md is empty")
						}
					case tools.OutcomeCrossMapped:
						assertEntity(t, c, out.CrossTo)
					default: // OutcomeReversible
						assertEntity(t, c, fw.Concept)
					}
				})
			}
		}
	}
}

// writeProbeWorkspace creates a temp workspace with an empty canonical scaffold
// and writes the single rendered file at its path.
func writeProbeWorkspace(t *testing.T, fw tools.FileWrite) string {
	t.Helper()
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync")
	for _, sub := range []string{"rules", "skills", "agents", "commands"} {
		if err := os.MkdirAll(filepath.Join(base, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(base, "AGENTS.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(ws, fw.Path)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, fw.Content, 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// assertEntity fails unless canonical holds at least one non-empty entity of
// the given concept matching the probe identity.
func assertEntity(t *testing.T, c *canonical.Canonical, concept tools.Concept) {
	t.Helper()
	switch concept {
	case tools.ConceptRules:
		for _, r := range c.Rules {
			if r.Filename == "sample-rule" && r.Body != "" {
				return
			}
		}
		t.Fatalf("expected canonical Rule \"sample-rule\" with body; rules=%d", len(c.Rules))
	case tools.ConceptSkills:
		for _, s := range c.Skills {
			if s.Dir == "sample-skill" || s.Dir == "sample-command" { // vibe cmd→skill
				if s.Body != "" {
					return
				}
			}
		}
		t.Fatalf("expected canonical Skill with body; skills=%d", len(c.Skills))
	case tools.ConceptAgents:
		for _, a := range c.Agents {
			if a.Filename == "sample-agent" && a.Body != "" {
				return
			}
		}
		t.Fatalf("expected canonical Agent \"sample-agent\" with body; agents=%d", len(c.Agents))
	case tools.ConceptCommands:
		for _, cmd := range c.Commands {
			if cmd.Filename == "sample-command" && cmd.Body != "" {
				return
			}
		}
		t.Fatalf("expected canonical Command \"sample-command\" with body; commands=%d", len(c.Commands))
	}
}
