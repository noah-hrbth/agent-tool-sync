package syncer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/syncer"
)

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
