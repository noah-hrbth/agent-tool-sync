package tools_test

import (
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func TestAlias(t *testing.T) {
	tests := []struct {
		name    string
		adapter tools.Adapter
		cases   []struct {
			concept tools.Concept
			want    string
		}
	}{
		{
			name:    "Claude Code",
			adapter: tools.All()[0],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "CLAUDE.md"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "OpenCode",
			adapter: tools.All()[1],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Cursor",
			adapter: tools.All()[2],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "general.mdc"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Gemini CLI",
			adapter: tools.All()[3],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "GEMINI.md"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Codex CLI",
			adapter: tools.All()[4],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Zed",
			adapter: tools.All()[5],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ".rules"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Cline",
			adapter: tools.All()[6],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "JetBrains Junie",
			adapter: tools.All()[7],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.cases {
				got := tt.adapter.Alias(tc.concept)
				if got != tc.want {
					t.Errorf("Alias(%q): got %q, want %q", tc.concept, got, tc.want)
				}
			}
		})
	}
}

func TestCursorSupportsAllConcepts(t *testing.T) {
	cursor := tools.All()[2]
	// ConceptCommands is excluded here because it is deprecated (not unsupported);
	// its deprecated-specific assertions live in TestCursorCommandsDeprecated.
	for _, concept := range []tools.Concept{
		tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents,
	} {
		if got := cursor.Supports(concept); !got.Supported {
			t.Errorf("Cursor.Supports(%v): want supported=true, got false (%s)", concept, got.Reason)
		}
	}
}

func TestCursorCommandsDeprecated(t *testing.T) {
	cursor := tools.All()[2]
	compat := cursor.Supports(tools.ConceptCommands)
	if !compat.Supported {
		t.Error("Cursor commands should be Supported=true (backward-compat)")
	}
	if !compat.Deprecated {
		t.Error("Cursor commands should be Deprecated=true")
	}
	if compat.Reason == "" {
		t.Error("Cursor commands should have a non-empty Reason")
	}
	if compat.Replacement != "skills" {
		t.Errorf("Cursor commands Replacement: got %q, want %q", compat.Replacement, "skills")
	}
}

func TestClaudeCommandsDeprecated(t *testing.T) {
	claude := tools.All()[0]
	compat := claude.Supports(tools.ConceptCommands)
	if !compat.Supported {
		t.Error("Claude commands should still be Supported=true (backward-compat)")
	}
	if !compat.Deprecated {
		t.Error("Claude commands should be Deprecated=true")
	}
	if compat.Reason == "" {
		t.Error("Claude commands should have a non-empty Reason")
	}
}

func TestCursorSkillRendersGlobsNotPaths(t *testing.T) {
	cursor := tools.All()[2]

	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:         "path-scoped",
			Name:        "path-scoped",
			Description: "test",
			Paths:       []string{"src/**/*.ts", "lib/**/*.ts"},
		}},
	}
	writes, err := cursor.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var skillWrite *tools.FileWrite
	for i := range writes {
		if writes[i].Concept == tools.ConceptSkills {
			skillWrite = &writes[i]
			break
		}
	}
	if skillWrite == nil {
		t.Fatal("no ConceptSkills write found")
	}

	content := string(skillWrite.Content)
	if !strings.Contains(content, "globs:") {
		t.Errorf("expected frontmatter to contain 'globs:', got:\n%s", content)
	}
	if strings.Contains(content, "paths:") {
		t.Errorf("expected frontmatter NOT to contain 'paths:', got:\n%s", content)
	}
	for _, glob := range []string{"src/**/*.ts", "lib/**/*.ts"} {
		if !strings.Contains(content, glob) {
			t.Errorf("expected glob value %q in output, got:\n%s", glob, content)
		}
	}
}

func TestCursorSkillWithNoPathsOmitsGlobs(t *testing.T) {
	cursor := tools.All()[2]

	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:         "no-paths",
			Name:        "no-paths",
			Description: "test",
		}},
	}
	writes, err := cursor.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, w := range writes {
		if w.Concept == tools.ConceptSkills {
			content := string(w.Content)
			if strings.Contains(content, "globs:") {
				t.Errorf("expected no 'globs:' when Paths is empty, got:\n%s", content)
			}
			if strings.Contains(content, "paths:") {
				t.Errorf("expected no 'paths:' when Paths is empty, got:\n%s", content)
			}
			return
		}
	}
	t.Fatal("no ConceptSkills write found")
}

func TestGeminiSupportsAllConcepts(t *testing.T) {
	gemini := tools.All()[3]
	for _, concept := range []tools.Concept{
		tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands,
	} {
		compat := gemini.Supports(concept)
		if !compat.Supported {
			t.Errorf("Gemini.Supports(%v): want supported=true, got false (%s)", concept, compat.Reason)
		}
		if compat.Deprecated {
			t.Errorf("Gemini.Supports(%v): want Deprecated=false, got true", concept)
		}
	}
}

func TestClaudeRuleRendersPerFile(t *testing.T) {
	claude := tools.All()[0]
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{
			Filename:    "style-guide",
			Description: "Style conventions",
			Paths:       []string{"src/**/*.ts"},
			Body:        "Use const.\n",
		}},
	}
	writes, err := claude.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var found *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".claude/rules/style-guide.md" {
			found = &writes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected .claude/rules/style-guide.md in output")
	}
	content := string(found.Content)
	if !strings.Contains(content, "paths: [src/**/*.ts]") {
		t.Errorf("expected paths field in output:\n%s", content)
	}
	if !strings.Contains(content, "Use const.") {
		t.Errorf("expected body in output:\n%s", content)
	}
}

func TestCursorRuleRendersPerFileWithGlobs(t *testing.T) {
	cursor := tools.All()[2]
	c := &canonical.Canonical{
		Rules: []*canonical.Rule{{
			Filename:    "style-guide",
			Description: "Style conventions",
			Paths:       []string{"src/**/*.ts"},
			Body:        "Use const.\n",
		}},
	}
	writes, err := cursor.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var found *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".cursor/rules/style-guide.mdc" {
			found = &writes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected .cursor/rules/style-guide.mdc in output")
	}
	content := string(found.Content)
	if !strings.Contains(content, "globs:") {
		t.Errorf("expected globs field in output:\n%s", content)
	}
	if strings.Contains(content, "paths:") {
		t.Errorf("expected no paths field in output:\n%s", content)
	}
	if strings.Contains(content, "alwaysApply:") && strings.Contains(content, "true") {
		t.Errorf("per-rule .mdc should not have alwaysApply: true:\n%s", content)
	}
}

func TestGeminiRuleAppendsToRootMemory(t *testing.T) {
	gemini := tools.All()[3]
	c := &canonical.Canonical{
		AgentsMD: "# Project rules\n",
		Rules: []*canonical.Rule{{
			Filename: "style-guide",
			Body:     "Use const.\n",
		}},
	}
	writes, err := gemini.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var found *tools.FileWrite
	for i := range writes {
		if writes[i].Path == "GEMINI.md" {
			found = &writes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected GEMINI.md at workspace root in output")
	}
	content := string(found.Content)
	if !strings.Contains(content, "# Project rules") {
		t.Errorf("expected AgentsMD content in root file:\n%s", content)
	}
	if !strings.Contains(content, "## style-guide") {
		t.Errorf("expected rule section heading in root file:\n%s", content)
	}
	if !strings.Contains(content, "Use const.") {
		t.Errorf("expected rule body in root file:\n%s", content)
	}
	// No separate rule file should exist for Gemini
	for _, w := range writes {
		if strings.Contains(w.Path, "rules/") {
			t.Errorf("unexpected rules-dir file for Gemini: %s", w.Path)
		}
	}
}

func TestCodexCommandsDeprecated(t *testing.T) {
	codex := tools.All()[4]

	// Skills and agents should be fully supported
	for _, concept := range []tools.Concept{tools.ConceptSkills, tools.ConceptAgents} {
		compat := codex.Supports(concept)
		if !compat.Supported {
			t.Errorf("Codex.Supports(%v): want supported=true", concept)
		}
		if compat.Deprecated {
			t.Errorf("Codex.Supports(%v): want Deprecated=false", concept)
		}
	}

	// Commands should be deprecated
	cmdCompat := codex.Supports(tools.ConceptCommands)
	if !cmdCompat.Supported {
		t.Error("Codex commands should be Supported=true (backward-compat)")
	}
	if !cmdCompat.Deprecated {
		t.Error("Codex commands should be Deprecated=true")
	}
	if cmdCompat.Reason == "" {
		t.Error("Codex commands should have a non-empty Reason")
	}
}

func TestZedSupportsMatrix(t *testing.T) {
	zed := tools.All()[5]

	if compat := zed.Supports(tools.ConceptRules); !compat.Supported {
		t.Errorf("Zed.Supports(Rules): want supported=true, got false (%s)", compat.Reason)
	}

	for _, concept := range []tools.Concept{tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands} {
		compat := zed.Supports(concept)
		if compat.Supported {
			t.Errorf("Zed.Supports(%v): want supported=false, got true", concept)
		}
		if compat.Reason == "" {
			t.Errorf("Zed.Supports(%v): expected non-empty Reason for unsupported concept", concept)
		}
	}
}

func TestZedRuleAppendsToRootMemory(t *testing.T) {
	zed := tools.All()[5]
	c := &canonical.Canonical{
		AgentsMD: "# Project rules\n",
		Rules: []*canonical.Rule{{
			Filename: "style-guide",
			Body:     "Use const.\n",
		}},
	}
	writes, err := zed.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var found *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".rules" {
			found = &writes[i]
			break
		}
	}
	if found == nil {
		t.Fatal("expected .rules at workspace root in output")
	}
	content := string(found.Content)
	if !strings.Contains(content, "# Project rules") {
		t.Errorf("expected AgentsMD content in .rules:\n%s", content)
	}
	if !strings.Contains(content, "## style-guide") {
		t.Errorf("expected rule section heading in .rules:\n%s", content)
	}
	if !strings.Contains(content, "Use const.") {
		t.Errorf("expected rule body in .rules:\n%s", content)
	}
}

func TestZedDoesNotEmitSkillsAgentsCommands(t *testing.T) {
	zed := tools.All()[5]
	c := &canonical.Canonical{
		AgentsMD: "# Memory\n",
		Skills: []*canonical.Skill{{
			Dir:         "demo",
			Name:        "demo",
			Description: "test",
		}},
		Agents: []*canonical.Agent{{
			Filename:    "reviewer",
			Name:        "reviewer",
			Description: "test",
		}},
		Commands: []*canonical.Command{{
			Filename:    "do-thing",
			Description: "test",
		}},
	}
	writes, err := zed.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(writes) != 1 {
		t.Fatalf("expected exactly 1 FileWrite (rules only), got %d", len(writes))
	}
	if writes[0].Path != ".rules" {
		t.Errorf("expected sole write to .rules, got %q", writes[0].Path)
	}
	if writes[0].Concept != tools.ConceptRules {
		t.Errorf("expected Concept=Rules, got %q", writes[0].Concept)
	}
}

func TestSupportsScope(t *testing.T) {
	cases := []struct {
		toolName        string
		userSupported   bool
	}{
		{"Claude Code", true},
		{"OpenCode", true},
		{"Cursor", false},
		{"Gemini CLI", true},
		{"Codex CLI", true},
		{"Zed", false},
		{"Cline", true},
		{"JetBrains Junie", true},
	}
	byName := map[string]tools.Adapter{}
	for _, a := range tools.All() {
		byName[a.Name()] = a
	}
	for _, c := range cases {
		t.Run(c.toolName, func(t *testing.T) {
			a := byName[c.toolName]
			if got := a.SupportsScope(tools.ScopeProject); !got.Supported {
				t.Errorf("SupportsScope(Project) for %s: want true, got false", c.toolName)
			}
			got := a.SupportsScope(tools.ScopeUser)
			if got.Supported != c.userSupported {
				t.Errorf("SupportsScope(User) for %s: want %v, got %v", c.toolName, c.userSupported, got.Supported)
			}
			if !c.userSupported && got.Reason == "" {
				t.Errorf("SupportsScope(User) for %s: expected non-empty Reason for unsupported scope", c.toolName)
			}
		})
	}
}

func TestUserScopeRendersDifferentPaths(t *testing.T) {
	c := &canonical.Canonical{
		AgentsMD: "# rules",
		Skills:   []*canonical.Skill{{Dir: "demo", Name: "demo", Description: "test"}},
	}

	// Claude: project memory = CLAUDE.md (workspace root), user memory = .claude/CLAUDE.md
	claude := tools.All()[0]
	claudeProject, _ := claude.Render(c, tools.ScopeProject)
	claudeUser, _ := claude.Render(c, tools.ScopeUser)
	if !containsPath(claudeProject, "CLAUDE.md") {
		t.Error("Claude project: expected CLAUDE.md at workspace root")
	}
	if containsPath(claudeProject, ".claude/CLAUDE.md") {
		t.Error("Claude project: did not expect .claude/CLAUDE.md (that's the user-scope path)")
	}
	if !containsPath(claudeUser, ".claude/CLAUDE.md") {
		t.Error("Claude user: expected .claude/CLAUDE.md")
	}

	// OpenCode: project root memory = AGENTS.md (workspace root), user = .config/opencode/AGENTS.md
	openCode := tools.All()[1]
	projectWrites, _ := openCode.Render(c, tools.ScopeProject)
	userWrites, _ := openCode.Render(c, tools.ScopeUser)
	if !containsPath(projectWrites, "AGENTS.md") {
		t.Error("OpenCode project: expected AGENTS.md at workspace root")
	}
	if containsPath(projectWrites, ".opencode/AGENTS.md") {
		t.Error("OpenCode project: did not expect .opencode/AGENTS.md (CLI does not read that path)")
	}
	if !containsPath(userWrites, ".config/opencode/AGENTS.md") {
		t.Error("OpenCode user: expected .config/opencode/AGENTS.md")
	}

	// Codex: project root memory = AGENTS.md (workspace root), user = .codex/AGENTS.md
	// Project skills = .agents/skills/, user skills = .codex/skills/
	codex := tools.All()[4]
	codexProject, _ := codex.Render(c, tools.ScopeProject)
	codexUser, _ := codex.Render(c, tools.ScopeUser)
	if !containsPath(codexProject, "AGENTS.md") {
		t.Error("Codex project: expected AGENTS.md at workspace root")
	}
	if containsPath(codexProject, ".codex/AGENTS.md") {
		t.Error("Codex project: did not expect .codex/AGENTS.md (CLI does not read that path)")
	}
	if !containsPath(codexUser, ".codex/AGENTS.md") {
		t.Error("Codex user: expected .codex/AGENTS.md")
	}
	if !containsPath(codexProject, ".agents/skills/demo/SKILL.md") {
		t.Error("Codex project skills: expected .agents/skills/demo/SKILL.md")
	}
	if !containsPath(codexUser, ".codex/skills/demo/SKILL.md") {
		t.Error("Codex user skills: expected .codex/skills/demo/SKILL.md")
	}

	// Gemini: project root memory = GEMINI.md (workspace root), user = .gemini/GEMINI.md
	gemini := tools.All()[3]
	geminiProject, _ := gemini.Render(c, tools.ScopeProject)
	geminiUser, _ := gemini.Render(c, tools.ScopeUser)
	if !containsPath(geminiProject, "GEMINI.md") {
		t.Error("Gemini project: expected GEMINI.md at workspace root")
	}
	if containsPath(geminiProject, ".gemini/GEMINI.md") {
		t.Error("Gemini project: did not expect .gemini/GEMINI.md (CLI does not read that path)")
	}
	if !containsPath(geminiUser, ".gemini/GEMINI.md") {
		t.Error("Gemini user: expected .gemini/GEMINI.md")
	}
}

func TestUserScopeUnsupportedRendersEmpty(t *testing.T) {
	c := &canonical.Canonical{AgentsMD: "# rules"}
	for _, name := range []string{"Cursor", "Zed"} {
		var a tools.Adapter
		for _, candidate := range tools.All() {
			if candidate.Name() == name {
				a = candidate
				break
			}
		}
		writes, err := a.Render(c, tools.ScopeUser)
		if err != nil {
			t.Errorf("%s.Render(User): unexpected error %v", name, err)
		}
		if len(writes) != 0 {
			t.Errorf("%s.Render(User): expected 0 writes, got %d", name, len(writes))
		}
	}
}

func containsPath(writes []tools.FileWrite, path string) bool {
	for _, w := range writes {
		if w.Path == path {
			return true
		}
	}
	return false
}

func TestZedNotice(t *testing.T) {
	zed := tools.All()[5]
	if zed.Notice() == "" {
		t.Error("Zed.Notice() should be non-empty — both rules-at-root and unsupported concepts warrant TUI surfacing")
	}
}

func adapterByName(t *testing.T, name string) tools.Adapter {
	t.Helper()
	for _, a := range tools.All() {
		if a.Name() == name {
			return a
		}
	}
	t.Fatalf("adapter %q not registered", name)
	return nil
}

// --- Cline ---

func TestClineSupportsMatrix(t *testing.T) {
	cline := adapterByName(t, "Cline")
	for _, concept := range []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptCommands} {
		if got := cline.Supports(concept); !got.Supported {
			t.Errorf("Cline.Supports(%v): want supported=true, got false (%s)", concept, got.Reason)
		}
	}
	got := cline.Supports(tools.ConceptAgents)
	if got.Supported {
		t.Error("Cline.Supports(Agents): want supported=false, got true")
	}
	if !strings.Contains(got.Reason, "sub-agents") {
		t.Errorf("Cline.Supports(Agents) reason should mention sub-agents, got %q", got.Reason)
	}
}

func TestClineRendersProjectRules(t *testing.T) {
	cline := adapterByName(t, "Cline")
	c := &canonical.Canonical{
		AgentsMD: "# root",
		Rules: []*canonical.Rule{{
			Filename: "style",
			Body:     "rule body",
			Paths:    []string{"src/**"},
		}},
	}
	writes, err := cline.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	var rootWrite, ruleWrite *tools.FileWrite
	for i := range writes {
		switch writes[i].Path {
		case "AGENTS.md":
			rootWrite = &writes[i]
		case ".clinerules/style.md":
			ruleWrite = &writes[i]
		}
	}
	if rootWrite == nil {
		t.Fatal("expected AGENTS.md write at workspace root")
	}
	rootContent := string(rootWrite.Content)
	if !strings.Contains(rootContent, "# root") {
		t.Errorf("AGENTS.md missing original AgentsMD content: %s", rootContent)
	}
	if !strings.Contains(rootContent, "## style") {
		t.Errorf("AGENTS.md should append rule as ## style section (matches OpenCode/Codex/Junie content): %s", rootContent)
	}
	if !strings.Contains(rootContent, "rule body") {
		t.Errorf("AGENTS.md should include rule body: %s", rootContent)
	}

	if ruleWrite == nil {
		t.Fatal("expected .clinerules/style.md write")
	}
	ruleContent := string(ruleWrite.Content)
	if !strings.Contains(ruleContent, "paths: [src/**]") {
		t.Errorf(".clinerules/style.md should have paths frontmatter: %s", ruleContent)
	}
	if !strings.Contains(ruleContent, "rule body") {
		t.Errorf(".clinerules/style.md should include rule body: %s", ruleContent)
	}
}

func TestClineUserScopeRulesPath(t *testing.T) {
	cline := adapterByName(t, "Cline")
	c := &canonical.Canonical{
		AgentsMD: "# root",
		Rules: []*canonical.Rule{{
			Filename: "style",
			Body:     "rule body",
		}},
	}
	writes, err := cline.Render(c, tools.ScopeUser)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if containsPath(writes, "AGENTS.md") {
		t.Error("Cline user scope must not write AGENTS.md (Cline reads no user-level AGENTS.md)")
	}
	if !containsPath(writes, "Documents/Cline/Rules/style.md") {
		t.Errorf("Cline user rules: expected Documents/Cline/Rules/style.md, got %v", pathsOf(writes))
	}
}

func TestClineRendersSkills(t *testing.T) {
	cline := adapterByName(t, "Cline")
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:         "foo",
			Name:        "foo",
			Description: "test skill",
			Paths:       []string{"src/**"},
		}},
	}
	for _, scope := range []tools.Scope{tools.ScopeProject, tools.ScopeUser} {
		writes, err := cline.Render(c, scope)
		if err != nil {
			t.Fatalf("Render(%v): %v", scope, err)
		}
		if !containsPath(writes, ".cline/skills/foo/SKILL.md") {
			t.Errorf("Cline %s skills: expected .cline/skills/foo/SKILL.md, got %v", scope, pathsOf(writes))
		}
		var skill *tools.FileWrite
		for i := range writes {
			if writes[i].Path == ".cline/skills/foo/SKILL.md" {
				skill = &writes[i]
				break
			}
		}
		content := string(skill.Content)
		if !strings.Contains(content, "name: foo") || !strings.Contains(content, "description: test skill") {
			t.Errorf("Cline skill frontmatter missing name/description: %s", content)
		}
		if strings.Contains(content, "paths:") || strings.Contains(content, "globs:") || strings.Contains(content, "allowed-tools:") {
			t.Errorf("Cline skill frontmatter should not include paths/globs/allowed-tools: %s", content)
		}
	}
}

func TestClineRendersWorkflows(t *testing.T) {
	cline := adapterByName(t, "Cline")
	c := &canonical.Canonical{
		Commands: []*canonical.Command{{
			Filename: "deploy",
			Body:     "deploy steps",
		}},
	}
	projectWrites, _ := cline.Render(c, tools.ScopeProject)
	userWrites, _ := cline.Render(c, tools.ScopeUser)
	if !containsPath(projectWrites, ".clinerules/workflows/deploy.md") {
		t.Errorf("Cline project workflows: expected .clinerules/workflows/deploy.md, got %v", pathsOf(projectWrites))
	}
	if !containsPath(userWrites, "Documents/Cline/Workflows/deploy.md") {
		t.Errorf("Cline user workflows: expected Documents/Cline/Workflows/deploy.md, got %v", pathsOf(userWrites))
	}
	for _, w := range projectWrites {
		if w.Path == ".clinerules/workflows/deploy.md" {
			if string(w.Content) != "deploy steps" {
				t.Errorf("Cline workflow body should be plain (no frontmatter); got %q", string(w.Content))
			}
		}
	}
}

func TestClineNotice(t *testing.T) {
	cline := adapterByName(t, "Cline")
	notice := cline.Notice()
	if notice == "" {
		t.Fatal("Cline.Notice() should be non-empty — three roots warrant explanation")
	}
	for _, fragment := range []string{".clinerules", ".cline/skills", "Documents/Cline"} {
		if !strings.Contains(notice, fragment) {
			t.Errorf("Cline.Notice() should mention %q, got %q", fragment, notice)
		}
	}
}

// --- JetBrains Junie ---

func TestJunieSupportsMatrix(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	for _, concept := range []tools.Concept{tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands} {
		got := junie.Supports(concept)
		if !got.Supported {
			t.Errorf("Junie.Supports(%v): want supported=true, got false (%s)", concept, got.Reason)
		}
		if got.Deprecated {
			t.Errorf("Junie.Supports(%v): unexpected deprecated=true", concept)
		}
	}
}

func TestJunieRuleAppendsToRootMemory(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	c := &canonical.Canonical{
		AgentsMD: "# root",
		Rules: []*canonical.Rule{{
			Filename: "style",
			Body:     "rule body",
		}},
	}
	writes, err := junie.Render(c, tools.ScopeProject)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	var root *tools.FileWrite
	for i := range writes {
		if writes[i].Path == "AGENTS.md" {
			root = &writes[i]
		}
		// Ensure no per-rule files exist for Junie.
		if strings.HasPrefix(writes[i].Path, ".junie/rules/") {
			t.Errorf("Junie should not emit per-rule files; got %s", writes[i].Path)
		}
	}
	if root == nil {
		t.Fatal("Junie project: expected AGENTS.md at workspace root")
	}
	content := string(root.Content)
	if !strings.Contains(content, "# root") {
		t.Errorf("AGENTS.md missing AgentsMD content: %s", content)
	}
	if !strings.Contains(content, "## style") || !strings.Contains(content, "rule body") {
		t.Errorf("AGENTS.md missing appended rule section: %s", content)
	}
}

func TestJunieUserScopeSkipsRootMemory(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	c := &canonical.Canonical{
		AgentsMD: "# root",
		Rules:    []*canonical.Rule{{Filename: "style", Body: "rule body"}},
	}
	writes, err := junie.Render(c, tools.ScopeUser)
	if err != nil {
		t.Fatalf("Render(User): %v", err)
	}
	for _, w := range writes {
		if strings.HasSuffix(w.Path, "AGENTS.md") {
			t.Errorf("Junie user scope must not emit AGENTS.md (no user-level guidelines path); got %s", w.Path)
		}
	}
}

func TestJunieRendersSkills(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:                    "foo",
			Name:                   "foo",
			Description:            "test skill",
			AllowedTools:           []string{"Read"},
			DisableModelInvocation: true,
			Paths:                  []string{"src/**"},
		}},
	}
	for _, scope := range []tools.Scope{tools.ScopeProject, tools.ScopeUser} {
		writes, _ := junie.Render(c, scope)
		if !containsPath(writes, ".junie/skills/foo/SKILL.md") {
			t.Errorf("Junie %s skills: expected .junie/skills/foo/SKILL.md, got %v", scope, pathsOf(writes))
		}
		for _, w := range writes {
			if w.Path == ".junie/skills/foo/SKILL.md" {
				content := string(w.Content)
				if !strings.Contains(content, "name: foo") || !strings.Contains(content, "description: test skill") {
					t.Errorf("Junie skill frontmatter missing name/description: %s", content)
				}
				for _, omit := range []string{"allowed-tools", "disable-model-invocation", "paths", "globs"} {
					if strings.Contains(content, omit+":") {
						t.Errorf("Junie skill frontmatter should omit %q (Junie supports name/description only): %s", omit, content)
					}
				}
			}
		}
	}
}

func TestJunieRendersAgents(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	c := &canonical.Canonical{
		Agents: []*canonical.Agent{{
			Filename:    "debugger",
			Name:        "debugger",
			Description: "find bugs",
			Tools:       []string{"Read", "Grep"},
			Model:       "sonnet",
		}},
	}
	writes, _ := junie.Render(c, tools.ScopeProject)
	if !containsPath(writes, ".junie/agents/debugger.md") {
		t.Errorf("Junie agents: expected .junie/agents/debugger.md, got %v", pathsOf(writes))
	}
	for _, w := range writes {
		if w.Path == ".junie/agents/debugger.md" {
			content := string(w.Content)
			for _, want := range []string{"name: debugger", "description: find bugs", "tools: [Read, Grep]", "model: sonnet"} {
				if !strings.Contains(content, want) {
					t.Errorf("Junie agent frontmatter missing %q: %s", want, content)
				}
			}
		}
	}
}

func TestJunieRendersCommands(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	c := &canonical.Canonical{
		Commands: []*canonical.Command{{
			Filename:     "summarize",
			Description:  "summarize PR",
			ArgumentHint: "[pr-num]",
			AllowedTools: []string{"Read"},
			Body:         "do stuff",
		}},
	}
	writes, _ := junie.Render(c, tools.ScopeProject)
	if !containsPath(writes, ".junie/commands/summarize.md") {
		t.Errorf("Junie commands: expected .junie/commands/summarize.md, got %v", pathsOf(writes))
	}
	for _, w := range writes {
		if w.Path == ".junie/commands/summarize.md" {
			content := string(w.Content)
			if !strings.Contains(content, "description: summarize PR") {
				t.Errorf("Junie command should have description frontmatter: %s", content)
			}
			for _, omit := range []string{"argument-hint", "allowed-tools", "model"} {
				if strings.Contains(content, omit+":") {
					t.Errorf("Junie command should omit %q (only description supported): %s", omit, content)
				}
			}
			if !strings.Contains(content, "do stuff") {
				t.Errorf("Junie command body missing: %s", content)
			}
		}
	}
}

func TestJunieNotice(t *testing.T) {
	junie := adapterByName(t, "JetBrains Junie")
	notice := junie.Notice()
	if notice == "" {
		t.Fatal("Junie.Notice() should be non-empty — project-only AGENTS.md warrants explanation")
	}
	if !strings.Contains(notice, "AGENTS.md") || !strings.Contains(notice, "project") {
		t.Errorf("Junie.Notice() should mention AGENTS.md and project-only behaviour: %q", notice)
	}
}

func pathsOf(writes []tools.FileWrite) []string {
	out := make([]string, len(writes))
	for i, w := range writes {
		out[i] = w.Path
	}
	return out
}
