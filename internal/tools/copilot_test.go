package tools_test

import (
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func TestCopilotRegistered(t *testing.T) {
	adapterByName(t, "GitHub Copilot")
}

func TestCopilotSupportsMatrix(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	for _, concept := range []tools.Concept{
		tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands,
	} {
		got := copilot.Meta.Supports(concept)
		if !got.Supported {
			t.Errorf("Copilot.Supports(%v): want supported=true, got false (%s)", concept, got.Reason)
		}
		if got.Deprecated {
			t.Errorf("Copilot.Supports(%v): unexpected deprecated=true", concept)
		}
	}
}

func TestCopilotRendersRootProject(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{AgentsMD: "# Project memory\n\nBody.\n"}
	writes, err := copilot.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if containsPath(writes, "AGENTS.md") {
		t.Error("Copilot should not emit root AGENTS.md (Copilot loads it independently if present)")
	}
	var root *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".github/copilot-instructions.md" {
			root = &writes[i]
			break
		}
	}
	if root == nil {
		t.Fatalf("expected .github/copilot-instructions.md, got %v", pathsOf(writes))
	}
	got := string(root.Content)
	if got != "# Project memory\n\nBody.\n" {
		t.Errorf(".github/copilot-instructions.md should be plain body (no frontmatter); got %q", got)
	}
	if strings.HasPrefix(got, "---") {
		t.Errorf(".github/copilot-instructions.md should not have frontmatter; got %q", got)
	}
	if root.Concept != tools.ConceptRules {
		t.Errorf("root write Concept: want Rules, got %v", root.Concept)
	}
}

func TestCopilotRendersRootUser(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{AgentsMD: "# User memory\n"}
	writes, err := copilot.Render(c, tools.ScopeUser)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if containsPath(writes, ".github/copilot-instructions.md") {
		t.Error("Copilot user scope must not write .github/ paths (those are project-scope)")
	}
	if !containsPath(writes, ".copilot/copilot-instructions.md") {
		t.Errorf("expected .copilot/copilot-instructions.md at user scope, got %v", pathsOf(writes))
	}
}

func TestCopilotRendersRule_SinglePath(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{
			Filename:    "style",
			Description: "Style rules",
			Paths:       []string{"src/**/*.ts"},
			Body:        "Use const.\n",
		}},
	}
	writes, err := copilot.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	const wantPath = ".github/instructions/style.instructions.md"
	if !containsPath(writes, wantPath) {
		t.Fatalf("expected %s, got %v", wantPath, pathsOf(writes))
	}
	var rule *tools.FileWrite
	for i := range writes {
		if writes[i].Path == wantPath {
			rule = &writes[i]
			break
		}
	}
	content := string(rule.Content)
	if !strings.Contains(content, "applyTo: src/**/*.ts") {
		t.Errorf("expected applyTo: src/**/*.ts in:\n%s", content)
	}
	if !strings.Contains(content, "description: Style rules") {
		t.Errorf("expected description in:\n%s", content)
	}
	if !strings.Contains(content, "Use const.") {
		t.Errorf("expected body in:\n%s", content)
	}
	if rule.Concept != tools.ConceptRules {
		t.Errorf("Concept: want Rules, got %v", rule.Concept)
	}
}

func TestCopilotRendersRule_MultiPath(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{
			Filename: "style",
			Paths:    []string{"src/**/*.ts", "test/**/*.ts"},
			Body:     "x",
		}},
	}
	writes, _ := copilot.Render(c, tools.ScopeProject)
	var rule *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".github/instructions/style.instructions.md" {
			rule = &writes[i]
			break
		}
	}
	if rule == nil {
		t.Fatal("expected .github/instructions/style.instructions.md")
	}
	content := string(rule.Content)
	// Brace-expanded glob starts with "{" (a YAML flow indicator), so it must be
	// quoted — otherwise it parses as a mapping and breaks the adopt round-trip.
	if !strings.Contains(content, `applyTo: "{src/**/*.ts,test/**/*.ts}"`) {
		t.Errorf("expected quoted brace-expanded applyTo in:\n%s", content)
	}
}

func TestCopilotRendersRule_NoPaths(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{Filename: "style", Body: "x"}},
	}
	writes, _ := copilot.Render(c, tools.ScopeProject)
	var rule *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".github/instructions/style.instructions.md" {
			rule = &writes[i]
			break
		}
	}
	if rule == nil {
		t.Fatal("expected .github/instructions/style.instructions.md")
	}
	content := string(rule.Content)
	if strings.Contains(content, "applyTo:") {
		t.Errorf("expected NO applyTo when Paths is empty:\n%s", content)
	}
}

func TestCopilotRendersRule_UserScope(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{Filename: "style", Paths: []string{"**/*.go"}, Body: "x"}},
	}
	writes, _ := copilot.Render(c, tools.ScopeUser)
	if containsPath(writes, ".github/instructions/style.instructions.md") {
		t.Error("user scope must not write .github/ paths")
	}
	if !containsPath(writes, ".copilot/instructions/style.instructions.md") {
		t.Errorf("expected .copilot/instructions/style.instructions.md at user scope, got %v", pathsOf(writes))
	}
}

func TestCopilotRendersSkills(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:         "code-review",
			Name:        "code-review",
			Description: "Review code",
			Paths:       []string{"src/**"},
			Body:        "Skill instructions.\n",
		}},
	}
	for _, tc := range []struct {
		scope    tools.Scope
		wantPath string
	}{
		{tools.ScopeProject, ".github/skills/code-review/SKILL.md"},
		{tools.ScopeUser, ".copilot/skills/code-review/SKILL.md"},
	} {
		writes, err := copilot.Render(c, tc.scope)
		if err != nil {
			t.Fatalf("Render(%v): %v", tc.scope, err)
		}
		if !containsPath(writes, tc.wantPath) {
			t.Errorf("scope=%v: expected %s, got %v", tc.scope, tc.wantPath, pathsOf(writes))
			continue
		}
		var skill *tools.FileWrite
		for i := range writes {
			if writes[i].Path == tc.wantPath {
				skill = &writes[i]
				break
			}
		}
		content := string(skill.Content)
		for _, want := range []string{"name: code-review", "description: Review code", "Skill instructions."} {
			if !strings.Contains(content, want) {
				t.Errorf("scope=%v: missing %q in:\n%s", tc.scope, want, content)
			}
		}
		for _, omit := range []string{"paths:", "globs:", "applyTo:"} {
			if strings.Contains(content, omit) {
				t.Errorf("scope=%v: should omit %q (Copilot skills have no path-scoping field):\n%s", tc.scope, omit, content)
			}
		}
		if skill.Concept != tools.ConceptSkills {
			t.Errorf("Concept: want Skills, got %v", skill.Concept)
		}
	}
}

func TestCopilotRendersAgents(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Agents: []*canonical.Agent{{
			Filename:    "debugger",
			Name:        "debugger",
			Description: "find bugs",
			Tools:       []string{"Read", "Grep"},
			Model:       "sonnet",
			Body:        "System prompt.\n",
		}},
	}
	for _, tc := range []struct {
		scope    tools.Scope
		wantPath string
	}{
		{tools.ScopeProject, ".github/agents/debugger.agent.md"},
		{tools.ScopeUser, ".copilot/agents/debugger.agent.md"},
	} {
		writes, _ := copilot.Render(c, tc.scope)
		if !containsPath(writes, tc.wantPath) {
			t.Errorf("scope=%v: expected %s, got %v", tc.scope, tc.wantPath, pathsOf(writes))
			continue
		}
		var agent *tools.FileWrite
		for i := range writes {
			if writes[i].Path == tc.wantPath {
				agent = &writes[i]
				break
			}
		}
		content := string(agent.Content)
		for _, want := range []string{"name: debugger", "description: find bugs", "tools: [Read, Grep]", "model: sonnet", "System prompt."} {
			if !strings.Contains(content, want) {
				t.Errorf("scope=%v: missing %q in:\n%s", tc.scope, want, content)
			}
		}
		if agent.Concept != tools.ConceptAgents {
			t.Errorf("Concept: want Agents, got %v", agent.Concept)
		}
	}
}

func TestCopilotRendersCommands_ProjectOnly(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	c := &canonical.Canonical{
		Commands: []*canonical.Command{{
			Filename:     "deploy",
			Description:  "Deploy app",
			ArgumentHint: "[env]",
			AllowedTools: []string{"Bash", "Read"},
			Model:        "sonnet",
			Body:         "Deploy steps.\n",
		}},
	}

	// Project: emits .github/prompts/<n>.prompt.md with tools/model/description.
	projectWrites, _ := copilot.Render(c, tools.ScopeProject)
	const wantPath = ".github/prompts/deploy.prompt.md"
	if !containsPath(projectWrites, wantPath) {
		t.Fatalf("expected %s, got %v", wantPath, pathsOf(projectWrites))
	}
	var cmd *tools.FileWrite
	for i := range projectWrites {
		if projectWrites[i].Path == wantPath {
			cmd = &projectWrites[i]
			break
		}
	}
	content := string(cmd.Content)
	for _, want := range []string{
		"description: Deploy app",
		`argument-hint: "[env]"`,
		"tools: [Bash, Read]",
		"model: sonnet",
		"Deploy steps.",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("missing %q in:\n%s", want, content)
		}
	}
	if strings.Contains(content, "allowed-tools:") {
		t.Errorf("Copilot prompts use `tools:` not `allowed-tools:` in:\n%s", content)
	}
	if cmd.Concept != tools.ConceptCommands {
		t.Errorf("Concept: want Commands, got %v", cmd.Concept)
	}

	// User: no prompts emitted (no documented user fs path).
	userWrites, _ := copilot.Render(c, tools.ScopeUser)
	for _, w := range userWrites {
		if strings.Contains(w.Path, "/prompts/") || strings.HasSuffix(w.Path, ".prompt.md") {
			t.Errorf("Copilot user scope must not write prompts; got %q", w.Path)
		}
	}
}

func TestCopilotConceptInfo(t *testing.T) {
	copilot := adapterByName(t, "GitHub Copilot")
	cases := []struct {
		concept  tools.Concept
		wantSubs []string
	}{
		{tools.ConceptRules, []string{".github/copilot-instructions.md", ".github/instructions/", "applyTo"}},
		{tools.ConceptSkills, []string{".github/skills/", "directory name"}},
		{tools.ConceptAgents, []string{".github/agents/", ".agent.md"}},
		{tools.ConceptCommands, []string{".github/prompts/", "user scope"}},
	}
	for _, tc := range cases {
		got := copilot.Meta.Info(tc.concept)
		if got == "" {
			t.Errorf("ConceptInfo(%v): want non-empty info, got empty", tc.concept)
			continue
		}
		for _, sub := range tc.wantSubs {
			if !strings.Contains(got, sub) {
				t.Errorf("ConceptInfo(%v): missing substring %q in %q", tc.concept, sub, got)
			}
		}
	}
}
