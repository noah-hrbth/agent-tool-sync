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

// renderToolFile renders the registered tool with the given Key at scope and
// returns the rendered bytes of the FileWrite at wantPath. It drives adopt tests
// from the adapter's ACTUAL output instead of hand-written fixtures, so the
// render↔adopt round-trip (incl. YAML quoting) is exercised, not assumed.
func renderToolFile(t *testing.T, key string, c *canonical.Canonical, scope tools.Scope, wantPath string) []byte {
	t.Helper()
	for _, tool := range tools.All() {
		if tool.Meta.Key != key {
			continue
		}
		writes, err := tool.Render(c, scope)
		if err != nil {
			t.Fatalf("%s render: %v", key, err)
		}
		var got []string
		for _, fw := range writes {
			if fw.Path == wantPath {
				return fw.Content
			}
			got = append(got, fw.Path)
		}
		t.Fatalf("%s render: no file at %q (rendered %v)", key, wantPath, got)
	}
	t.Fatalf("tool %q not registered", key)
	return nil
}

func buildAdoptWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	for _, dir := range []string{
		".agentsync/skills",
		".agentsync/agents",
		".agentsync/commands",
		".claude/skills/code-review",
		".claude/agents",
		".claude/commands",
		".cursor/rules",
		".gemini",
	} {
		if err := os.MkdirAll(filepath.Join(ws, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func TestAdoptRulesFromClaude(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	content := "# My edited rules\n"
	if err := os.WriteFile(filepath.Join(ws, "CLAUDE.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, "CLAUDE.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read canonical: %v", err)
	}
	if string(saved) != content {
		t.Errorf("canonical rules: got %q, want %q", string(saved), content)
	}
}

func TestAdoptVibeAgentsMDProject(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	content := "# Vibe edited rules\n"
	if err := os.WriteFile(filepath.Join(ws, "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, "AGENTS.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, _ := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if string(saved) != content {
		t.Errorf("canonical rules: got %q, want %q", string(saved), content)
	}
}

func TestAdoptVibeAgentsMDUser(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".vibe"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Vibe user-scope rules\n"
	if err := os.WriteFile(filepath.Join(ws, ".vibe", "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".vibe/AGENTS.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, _ := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if string(saved) != content {
		t.Errorf("canonical rules: got %q, want %q", string(saved), content)
	}
}

func TestAdoptVibeSkill(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".vibe", "skills", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: foo\ndescription: Probe skill\nallowed-tools: [read_file, grep]\n---\nSkill body.\n"
	if err := os.WriteFile(filepath.Join(ws, ".vibe", "skills", "foo", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".vibe/skills/foo/SKILL.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "foo", "SKILL.md"))
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical skill is empty")
	}
	if !strings.Contains(string(saved), "Skill body.") {
		t.Errorf("canonical skill body missing: %s", string(saved))
	}
}

func TestAdoptRulesFromGemini(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	content := "# Gemini edited rules\n"
	if err := os.WriteFile(filepath.Join(ws, "GEMINI.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, "GEMINI.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, _ := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if string(saved) != content {
		t.Errorf("canonical rules: got %q, want %q", string(saved), content)
	}
}

func TestAdoptRulesFromCursorMDC(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	body := "# Rules body\nSome content.\n"
	wrapped := "---\nalwaysApply: true\ndescription: Synced by agentsync\n---\n" + body
	if err := os.WriteFile(filepath.Join(ws, ".cursor", "rules", "general.mdc"), []byte(wrapped), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".cursor/rules/general.mdc"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, _ := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if string(saved) != body {
		t.Errorf("canonical rules (cursor): got %q, want %q", string(saved), body)
	}
}

func TestAdoptSkill(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	skillContent := "---\nname: code-review\ndescription: Reviews code\nallowed-tools: [Read, Grep]\n---\nReview the code.\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "skills", "code-review", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/skills/code-review/SKILL.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "code-review", "SKILL.md"))
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical skill is empty")
	}
}

func TestAdoptSkillDocFromClaude(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	docContent := "# reference\n\nSome reference text.\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "skills", "code-review", "reference.md"), []byte(docContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/skills/code-review/reference.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "code-review", "reference.md"))
	if err != nil {
		t.Fatalf("read canonical skill doc: %v", err)
	}
	if string(saved) != docContent {
		t.Errorf("skill doc: got %q, want %q", saved, docContent)
	}
}

func TestAdoptNestedSkillDocFromClaude(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".claude", "skills", "code-review", "examples"), 0o755); err != nil {
		t.Fatal(err)
	}
	docContent := "# invoice example\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "skills", "code-review", "examples", "invoice.md"), []byte(docContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/skills/code-review/examples/invoice.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "code-review", "examples", "invoice.md"))
	if err != nil {
		t.Fatalf("read nested canonical skill doc: %v", err)
	}
	if string(saved) != docContent {
		t.Errorf("nested skill doc: got %q, want %q", saved, docContent)
	}
}

func TestAdoptSkillDocWithRulesSubfolderNotMisroutedToRule(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".claude", "skills", "code-review", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	doc := "# nested doc under a rules/ subfolder\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "skills", "code-review", "rules", "x.md"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/skills/code-review/rules/x.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	// Must round-trip as a skill doc, not be claimed by the generic rule matcher.
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "code-review", "rules", "x.md")); err != nil {
		t.Errorf("skill doc with a rules/ subfolder should adopt as a skill doc: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "x.md")); !os.IsNotExist(err) {
		t.Error("skill doc must not be mis-adopted as a canonical rule")
	}
}

func TestAdoptSkillDocDoesNotClaimAgentPath(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	agentContent := "---\nname: foo\ndescription: an agent\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "agents", "foo.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/agents/foo.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	// Must land as an agent, not as a skill doc.
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "agents", "foo.md")); err != nil {
		t.Errorf("agent path must adopt as an agent: %v", err)
	}
}

func TestAdoptAgent(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	agentContent := "---\nname: researcher\ndescription: Researches topics\ntools: [Read]\nmodel: sonnet\n---\nResearch the topic.\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "agents", "researcher.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/agents/researcher.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "agents", "researcher.md"))
	if err != nil {
		t.Fatalf("read canonical agent: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical agent is empty")
	}
}

func TestAdoptCommand(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	cmdContent := "---\ndescription: Stage and commit\nallowed-tools: [Bash]\n---\nRun git commit.\n"
	if err := os.WriteFile(filepath.Join(ws, ".claude", "commands", "commit.md"), []byte(cmdContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".claude/commands/commit.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "commands", "commit.md"))
	if err != nil {
		t.Fatalf("read canonical command: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical command is empty")
	}
}

func TestAdoptUnknownPath(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	err := syncer.AdoptExternal(ws, "some/unknown/path.md")
	if err == nil {
		t.Error("expected error for unknown path, got nil")
	}
}

func TestAdoptSkillFromCline(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".cline", "skills", "foo"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: foo\ndescription: Cline skill\n---\nDo the thing.\n"
	if err := os.WriteFile(filepath.Join(ws, ".cline", "skills", "foo", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".cline/skills/foo/SKILL.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "foo", "SKILL.md"))
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical Cline skill empty")
	}
}

func TestAdoptSkillFromJunie(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".junie", "skills", "bar"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := "---\nname: bar\ndescription: Junie skill\n---\nBody.\n"
	if err := os.WriteFile(filepath.Join(ws, ".junie", "skills", "bar", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".junie/skills/bar/SKILL.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "bar", "SKILL.md"))
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical Junie skill empty")
	}
}

func TestAdoptAgentFromJunie(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".junie", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	agentContent := "---\nname: debugger\ndescription: bug hunter\nmodel: sonnet\n---\nFind bugs.\n"
	if err := os.WriteFile(filepath.Join(ws, ".junie", "agents", "debugger.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".junie/agents/debugger.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "agents", "debugger.md"))
	if err != nil {
		t.Fatalf("read canonical agent: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical Junie agent empty")
	}
}

func TestAdoptCommandFromJunie(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".junie", "commands"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmdContent := "---\ndescription: Summarize PR\n---\nRun summary.\n"
	if err := os.WriteFile(filepath.Join(ws, ".junie", "commands", "summarize.md"), []byte(cmdContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := syncer.AdoptExternal(ws, ".junie/commands/summarize.md"); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "commands", "summarize.md"))
	if err != nil {
		t.Fatalf("read canonical command: %v", err)
	}
	if len(saved) == 0 {
		t.Error("canonical Junie command empty")
	}
}

func TestAdoptClineWorkflow(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"project", ".clinerules/workflows/deploy.md"},
		{"user", "Documents/Cline/Workflows/deploy.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := buildAdoptWorkspace(t)
			if err := os.MkdirAll(filepath.Join(ws, filepath.Dir(tc.path)), 0o755); err != nil {
				t.Fatal(err)
			}
			body := "do the deploy steps\n"
			if err := os.WriteFile(filepath.Join(ws, tc.path), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := syncer.AdoptExternal(ws, tc.path); err != nil {
				t.Fatalf("adopt: %v", err)
			}
			saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "commands", "deploy.md"))
			if err != nil {
				t.Fatalf("read canonical command: %v", err)
			}
			if len(saved) == 0 {
				t.Error("canonical Cline workflow empty")
			}
		})
	}
}

func TestAdoptClineRule(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"project", ".clinerules/style.md"},
		{"user", "Documents/Cline/Rules/style.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := buildAdoptWorkspace(t)
			if err := os.MkdirAll(filepath.Join(ws, filepath.Dir(tc.path)), 0o755); err != nil {
				t.Fatal(err)
			}
			ruleContent := "---\npaths: [src/**]\n---\nRule body.\n"
			if err := os.WriteFile(filepath.Join(ws, tc.path), []byte(ruleContent), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := syncer.AdoptExternal(ws, tc.path); err != nil {
				t.Fatalf("adopt: %v", err)
			}
			saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "rules", "style.md"))
			if err != nil {
				t.Fatalf("read canonical rule: %v", err)
			}
			if len(saved) == 0 {
				t.Error("canonical Cline rule empty")
			}
		})
	}
}

func TestAdoptPiCommand(t *testing.T) {
	ws := buildAdoptWorkspace(t)

	// Drive adopt from the Pi adapter's ACTUAL render output (not a hand-quoted
	// literal). This exercises the full render↔adopt round-trip, including the
	// YAML-quoting of the bracketed argument-hint that a raw "[style-name]" would
	// otherwise break (cannot unmarshal !!seq into string). allowed-tools/model are
	// set to confirm Pi drops them — they are not Pi prompt fields.
	c := &canonical.Canonical{Commands: []*canonical.Command{{
		Filename:     "style",
		Description:  "Apply code style",
		ArgumentHint: "[style-name]",
		AllowedTools: []string{"Read", "Write"},
		Model:        "sonnet",
		Body:         "Format this file.\n",
	}}}
	rendered := renderToolFile(t, "pi", c, tools.ScopeProject, ".pi/prompts/style.md")
	if err := os.MkdirAll(filepath.Join(ws, ".pi", "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".pi", "prompts", "style.md"), rendered, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".pi/prompts/style.md"); err != nil {
		t.Fatalf("adopt Pi command: %v", err)
	}

	// Verify canonical Command was saved with the bracketed hint preserved by value.
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "commands", "style.md"))
	if err != nil {
		t.Fatalf("read canonical command: %v", err)
	}
	for _, want := range []string{"description: Apply code style", "[style-name]", "Format this file."} {
		if !strings.Contains(string(saved), want) {
			t.Errorf("canonical command missing %q:\n%s", want, string(saved))
		}
	}
	// Pi drops allowed-tools/model on render, so they must not survive the round-trip.
	for _, omit := range []string{"allowed-tools", "model:"} {
		if strings.Contains(string(saved), omit) {
			t.Errorf("canonical command should not contain %q (Pi drops it):\n%s", omit, string(saved))
		}
	}
}

func TestAdoptPiCommandUser(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".pi", "agent", "prompts"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Pi prompts only support description and argument-hint
	promptContent := `---
description: User scope style
argument-hint: "[preset]"
---
Apply user style.
`
	if err := os.WriteFile(filepath.Join(ws, ".pi", "agent", "prompts", "style.md"), []byte(promptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".pi/agent/prompts/style.md"); err != nil {
		t.Fatalf("adopt Pi user command: %v", err)
	}

	// Verify canonical Command was saved
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "commands", "style.md"))
	if err != nil {
		t.Fatalf("read canonical command: %v", err)
	}
	if !strings.Contains(string(saved), "description: User scope style") {
		t.Errorf("canonical user command missing description: %s", string(saved))
	}
}

func TestAdoptPiSkill(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".pi", "skills", "code-review"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillContent := `---
name: code-review
description: Review pull requests
allowed-tools: [Read, Grep]
disable-model-invocation: true
---
Review this code.
`
	if err := os.WriteFile(filepath.Join(ws, ".pi", "skills", "code-review", "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".pi/skills/code-review/SKILL.md"); err != nil {
		t.Fatalf("adopt Pi skill: %v", err)
	}

	// Verify canonical Skill was saved
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "code-review", "SKILL.md"))
	if err != nil {
		t.Fatalf("read canonical skill: %v", err)
	}
	if !strings.Contains(string(saved), "name: code-review") {
		t.Errorf("canonical skill missing name: %s", string(saved))
	}
	if !strings.Contains(string(saved), "description: Review pull requests") {
		t.Errorf("canonical skill missing description: %s", string(saved))
	}
	// allowed-tools may be saved as inline array or YAML list - just check both fields exist
	if !strings.Contains(string(saved), "allowed-tools:") {
		t.Errorf("canonical skill missing allowed-tools field: %s", string(saved))
	}
	if !strings.Contains(string(saved), "Read") || !strings.Contains(string(saved), "Grep") {
		t.Errorf("canonical skill missing allowed-tools values (Read, Grep): %s", string(saved))
	}
	if !strings.Contains(string(saved), "disable-model-invocation: true") {
		t.Errorf("canonical skill missing disable-model-invocation: %s", string(saved))
	}
	if !strings.Contains(string(saved), "Review this code.") {
		t.Errorf("canonical skill missing body: %s", string(saved))
	}
}

func TestAdoptPiRootMemoryUser(t *testing.T) {
	ws := buildAdoptWorkspace(t)
	if err := os.MkdirAll(filepath.Join(ws, ".pi", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "# Pi user-scope root memory\n"
	if err := os.WriteFile(filepath.Join(ws, ".pi", "agent", "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := syncer.AdoptExternal(ws, ".pi/agent/AGENTS.md"); err != nil {
		t.Fatalf("adopt Pi user root memory: %v", err)
	}

	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read canonical AGENTS.md: %v", err)
	}
	if string(saved) != content {
		t.Errorf("canonical AGENTS.md: got %q, want %q", string(saved), content)
	}
}
