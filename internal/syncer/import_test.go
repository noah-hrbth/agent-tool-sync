package syncer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// seedImportFile writes content at <ws>/<relPath>, creating parent dirs.
func seedImportFile(t *testing.T, ws, relPath string, content []byte) {
	t.Helper()
	full := filepath.Join(ws, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// seedClaudeConcepts seeds a claude-shaped rule, skill, agent (render-driven)
// and a hand-written command (renderClaude no longer emits deprecated commands).
func seedClaudeConcepts(t *testing.T, ws string) {
	t.Helper()
	c := &canonical.Canonical{
		Rules:  []*canonical.Rule{{Filename: "style", Paths: []string{"src/**"}, Body: "Rule body.\n"}},
		Skills: []*canonical.Skill{{Dir: "review", Name: "review", Description: "Reviews code", Body: "Skill body.\n"}},
		Agents: []*canonical.Agent{{Filename: "helper", Name: "helper", Description: "Helps out", Body: "Agent body.\n"}},
	}
	for _, path := range []string{
		".claude/rules/style.md",
		".claude/skills/review/SKILL.md",
		".claude/agents/helper.md",
	} {
		seedImportFile(t, ws, path, renderToolFile(t, "claude", c, tools.ScopeProject, path))
	}
	seedImportFile(t, ws, ".claude/commands/deploy.md", []byte("---\ndescription: Deploy the app\n---\nRun the deploy.\n"))
}

// claudeProjectSources hand-builds the claude project-scope import sources so
// engine tests do not depend on DeriveImportSources.
func claudeProjectSources() tools.ImportSources {
	return tools.ImportSources{
		ToolKey:   "claude",
		Dirs:      []string{".claude/agents", ".claude/commands", ".claude/rules", ".claude/skills"},
		RootFiles: []string{"CLAUDE.md", ".claude/CLAUDE.md"},
	}
}

func TestImportFromSourcesAdoptsAllConcepts(t *testing.T) {
	ws := t.TempDir()
	seedClaudeConcepts(t, ws)

	summary, err := syncer.ImportFromSources(ws, claudeProjectSources())

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	want := syncer.ImportSummary{Rules: 1, Skills: 1, Agents: 1, Commands: 1}
	if !reflect.DeepEqual(summary, want) {
		t.Errorf("summary = %+v, want %+v", summary, want)
	}
	loaded, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("canonical.Load after import: %v", err)
	}
	if len(loaded.Rules) != 1 || loaded.Rules[0].Filename != "style" {
		t.Errorf("canonical rules = %+v, want one rule %q", loaded.Rules, "style")
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Dir != "review" {
		t.Errorf("canonical skills = %+v, want one skill dir %q", loaded.Skills, "review")
	}
	if len(loaded.Agents) != 1 || loaded.Agents[0].Filename != "helper" {
		t.Errorf("canonical agents = %+v, want one agent %q", loaded.Agents, "helper")
	}
	if len(loaded.Commands) != 1 || loaded.Commands[0].Filename != "deploy" {
		t.Errorf("canonical commands = %+v, want one command %q", loaded.Commands, "deploy")
	}
}

func TestImportFromSourcesSkillDocsNested(t *testing.T) {
	ws := t.TempDir()
	c := &canonical.Canonical{Skills: []*canonical.Skill{{
		Dir: "foo", Name: "foo", Description: "Skill with docs", Body: "Skill body.\n",
		Docs: []canonical.SkillDoc{
			{RelPath: "reference.md", Content: "# reference\n"},
			{RelPath: "examples/deep.md", Content: "# deep example\n"},
		},
	}}}
	for _, path := range []string{
		".claude/skills/foo/SKILL.md",
		".claude/skills/foo/reference.md",
		".claude/skills/foo/examples/deep.md",
	} {
		seedImportFile(t, ws, path, renderToolFile(t, "claude", c, tools.ScopeProject, path))
	}

	summary, err := syncer.ImportFromSources(ws, tools.ImportSources{ToolKey: "claude", Dirs: []string{".claude/skills"}})

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.Skills != 1 || summary.SkillDocs != 2 {
		t.Errorf("summary = %+v, want Skills:1 SkillDocs:2", summary)
	}
	for _, rel := range []string{
		".agentsync/skills/foo/reference.md",
		".agentsync/skills/foo/examples/deep.md",
	} {
		if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel))); err != nil {
			t.Errorf("canonical skill doc %s: %v", rel, err)
		}
	}
}

func TestImportFromSourcesCursorMdcRules(t *testing.T) {
	ws := t.TempDir()
	c := &canonical.Canonical{Rules: []*canonical.Rule{{
		Filename: "style", Description: "Style rule", Paths: []string{"src/**"}, Body: "Rule body.\n",
	}}}
	seedImportFile(t, ws, ".cursor/rules/style.mdc", renderToolFile(t, "cursor", c, tools.ScopeProject, ".cursor/rules/style.mdc"))

	summary, err := syncer.ImportFromSources(ws, tools.ImportSources{ToolKey: "cursor", Dirs: []string{".cursor/rules"}})

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.Rules != 1 {
		t.Errorf("summary = %+v, want Rules:1", summary)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); err != nil {
		t.Errorf("canonical rule from .mdc: %v", err)
	}
}

func TestImportFromSourcesCatchAllCountedOnceAsRootMemory(t *testing.T) {
	ws := t.TempDir()
	c := &canonical.Canonical{AgentsMD: "# Root memory body\n"}
	seedImportFile(t, ws, ".cursor/rules/general.mdc", renderToolFile(t, "cursor", c, tools.ScopeProject, ".cursor/rules/general.mdc"))
	// general.mdc is both a RootFiles entry AND inside the walked .cursor/rules dir
	src := tools.ImportSources{
		ToolKey:   "cursor",
		Dirs:      []string{".cursor/rules"},
		RootFiles: []string{".cursor/rules/general.mdc"},
	}

	summary, err := syncer.ImportFromSources(ws, src)

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.RootMemoryFrom != ".cursor/rules/general.mdc" {
		t.Errorf("RootMemoryFrom = %q, want %q", summary.RootMemoryFrom, ".cursor/rules/general.mdc")
	}
	if summary.Rules != 0 {
		t.Errorf("Rules = %d, want 0 (catch-all counts once, as root memory)", summary.Rules)
	}
	if len(summary.Skipped) != 0 {
		t.Errorf("Skipped = %+v, want none (catch-all must not be re-processed in the walk)", summary.Skipped)
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read canonical AGENTS.md: %v", err)
	}
	if string(saved) != c.AgentsMD {
		t.Errorf("canonical AGENTS.md = %q, want %q (frontmatter stripped)", saved, c.AgentsMD)
	}
}

func TestImportFromSourcesSkipsNonMarkdown(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, ".claude/skills/foo/helper.py", []byte("print('hi')\n"))
	seedImportFile(t, ws, ".vibe/agents/conf.toml", []byte("name = \"x\"\n"))
	src := tools.ImportSources{Dirs: []string{".claude/skills", ".vibe/agents"}}

	summary, err := syncer.ImportFromSources(ws, src)

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	counts := summary
	counts.Skipped = nil
	if !reflect.DeepEqual(counts, syncer.ImportSummary{}) {
		t.Errorf("summary = %+v, want zero counts", summary)
	}
	if len(summary.Skipped) != 2 {
		t.Fatalf("Skipped = %+v, want 2 entries", summary.Skipped)
	}
	for _, s := range summary.Skipped {
		if s.Reason != "not markdown" {
			t.Errorf("Skipped %s reason = %q, want %q", s.Path, s.Reason, "not markdown")
		}
	}
}

func TestImportFromSourcesSkipsUnmappableMarkdown(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, ".cursor/skills/x/SKILL.md", []byte("---\nname: x\ndescription: Cursor skill\n---\nBody.\n"))

	summary, err := syncer.ImportFromSources(ws, tools.ImportSources{ToolKey: "cursor", Dirs: []string{".cursor/skills"}})

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	want := []syncer.SkippedFile{{Path: ".cursor/skills/x/SKILL.md", Reason: "no canonical mapping"}}
	if !reflect.DeepEqual(summary.Skipped, want) {
		t.Errorf("Skipped = %+v, want %+v", summary.Skipped, want)
	}
}

func TestImportFromSourcesUnmappableNeverAborts(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, ".cursor/agents/helper.md", []byte("---\nname: helper\n---\nbody\n"))
	// nested cline rule paths are rejected by matchClineRulePath → no mapping
	seedImportFile(t, ws, ".clinerules/sub/nested.md", []byte("nested rule body\n"))
	src := tools.ImportSources{Dirs: []string{".clinerules", ".cursor/agents"}}

	summary, err := syncer.ImportFromSources(ws, src)

	if err != nil {
		t.Fatalf("ImportFromSources must not abort on unmappable files: %v", err)
	}
	counts := summary
	counts.Skipped = nil
	if !reflect.DeepEqual(counts, syncer.ImportSummary{}) {
		t.Errorf("summary = %+v, want zero counts", summary)
	}
	if len(summary.Skipped) != 2 {
		t.Errorf("Skipped = %+v, want both unmappable files", summary.Skipped)
	}
}

func TestImportFromSourcesSymlinkSkipped(t *testing.T) {
	ws := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "outside.md")
	if err := os.WriteFile(target, []byte("# outside the workspace\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedImportFile(t, ws, ".claude/rules/style.md", []byte("---\npaths: [src/**]\n---\nRule body.\n"))
	if err := os.Symlink(target, filepath.Join(ws, ".claude", "rules", "evil.md")); err != nil {
		t.Fatal(err)
	}

	summary, err := syncer.ImportFromSources(ws, tools.ImportSources{Dirs: []string{".claude/rules"}})

	if err != nil {
		t.Fatalf("ImportFromSources must not abort on a symlink: %v", err)
	}
	if summary.Rules != 1 {
		t.Errorf("Rules = %d, want 1 (walk must continue past the symlink)", summary.Rules)
	}
	want := []syncer.SkippedFile{{Path: ".claude/rules/evil.md", Reason: "symlink, not followed"}}
	if !reflect.DeepEqual(summary.Skipped, want) {
		t.Errorf("Skipped = %+v, want %+v", summary.Skipped, want)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "outside.md")); !os.IsNotExist(err) {
		t.Error("symlink target content must never be adopted")
	}
}

func TestImportFromSourcesRootMemoryFirstMatchWins(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, "CLAUDE.md", []byte("# primary root memory\n"))
	seedImportFile(t, ws, ".claude/CLAUDE.md", []byte("# alternate root memory\n"))

	summary, err := syncer.ImportFromSources(ws, claudeProjectSources())

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.RootMemoryFrom != "CLAUDE.md" {
		t.Errorf("RootMemoryFrom = %q, want %q", summary.RootMemoryFrom, "CLAUDE.md")
	}
	saved, err := os.ReadFile(filepath.Join(ws, ".agentsync", "AGENTS.md"))
	if err != nil {
		t.Fatalf("read canonical AGENTS.md: %v", err)
	}
	if string(saved) != "# primary root memory\n" {
		t.Errorf("canonical AGENTS.md = %q, want first candidate's content", saved)
	}
	want := []syncer.SkippedFile{{Path: ".claude/CLAUDE.md", Reason: "root memory already imported from CLAUDE.md"}}
	if !reflect.DeepEqual(summary.Skipped, want) {
		t.Errorf("Skipped = %+v, want %+v", summary.Skipped, want)
	}
}

func TestImportFromSourcesRootMemoryAbsent(t *testing.T) {
	ws := t.TempDir()

	summary, err := syncer.ImportFromSources(ws, claudeProjectSources())

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.RootMemoryFrom != "" {
		t.Errorf("RootMemoryFrom = %q, want empty when no candidate exists", summary.RootMemoryFrom)
	}
	if len(summary.Skipped) != 0 {
		t.Errorf("Skipped = %+v, want none for absent root files", summary.Skipped)
	}
}

func TestImportFromSourcesSkipsReservedRuleNames(t *testing.T) {
	ws := t.TempDir()
	// every rule-class path family: generic rules dir, cline rules, copilot instructions
	seedImportFile(t, ws, ".claude/rules/general.md", []byte("---\npaths: [src/**]\n---\nRule body.\n"))
	seedImportFile(t, ws, ".clinerules/general.md", []byte("---\npaths: [src/**]\n---\nRule body.\n"))
	seedImportFile(t, ws, ".github/instructions/general.instructions.md", []byte("---\napplyTo: \"src/**\"\n---\nRule body.\n"))
	src := tools.ImportSources{Dirs: []string{".claude/rules", ".clinerules", ".github/instructions"}}

	summary, err := syncer.ImportFromSources(ws, src)

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.Rules != 0 {
		t.Errorf("Rules = %d, want 0 (reserved names must not be adopted)", summary.Rules)
	}
	if len(summary.Skipped) != 3 {
		t.Fatalf("Skipped = %+v, want 3 entries", summary.Skipped)
	}
	for _, s := range summary.Skipped {
		if !strings.Contains(s.Reason, "reserved") {
			t.Errorf("Skipped %s reason = %q, want it to mention %q", s.Path, s.Reason, "reserved")
		}
	}
	// a written general.md rule would make canonical.Load error — must stay loadable
	if _, err := canonical.Load(ws); err != nil {
		t.Errorf("canonical.Load after import: %v", err)
	}
}

func TestImportFromSourcesCatchAllNotReserved(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, ".cursor/rules/general.mdc", []byte("---\nalwaysApply: true\n---\n# Root memory body\n"))
	src := tools.ImportSources{
		ToolKey:   "cursor",
		Dirs:      []string{".cursor/rules"},
		RootFiles: []string{".cursor/rules/general.mdc"},
	}

	summary, err := syncer.ImportFromSources(ws, src)

	if err != nil {
		t.Fatalf("ImportFromSources: %v", err)
	}
	if summary.RootMemoryFrom != ".cursor/rules/general.mdc" {
		t.Errorf("RootMemoryFrom = %q, want the catch-all (it is root memory, not a reserved rule)", summary.RootMemoryFrom)
	}
	if len(summary.Skipped) != 0 {
		t.Errorf("Skipped = %+v, want none", summary.Skipped)
	}
}

// findTool returns the registered tool with the given key or fails the test.
func findTool(t *testing.T, key string) tools.Tool {
	t.Helper()
	for _, tool := range tools.All() {
		if tool.Meta.Key == key {
			return tool
		}
	}
	t.Fatalf("tool %q not registered", key)
	return tools.Tool{}
}

func TestImportFromToolClaudeEndToEnd(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, "CLAUDE.md", []byte("# project root memory\n"))
	seedClaudeConcepts(t, ws)
	seedImportFile(t, ws, ".claude/skills/review/reference.md", []byte("# reference\n"))

	summary, err := syncer.ImportFromTool(ws, findTool(t, "claude"), tools.ScopeProject)

	if err != nil {
		t.Fatalf("ImportFromTool: %v", err)
	}
	want := syncer.ImportSummary{
		RootMemoryFrom: "CLAUDE.md",
		Rules:          1,
		Skills:         1,
		SkillDocs:      1,
		Agents:         1,
		Commands:       1,
	}
	if !reflect.DeepEqual(summary, want) {
		t.Errorf("summary = %+v, want %+v", summary, want)
	}
	loaded, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("canonical.Load after import: %v", err)
	}
	if loaded.AgentsMD != "# project root memory\n" {
		t.Errorf("canonical AGENTS.md = %q, want CLAUDE.md content", loaded.AgentsMD)
	}
}

func TestImportFromToolEmptyToolDir(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	summary, err := syncer.ImportFromTool(ws, findTool(t, "claude"), tools.ScopeProject)

	if err != nil {
		t.Fatalf("ImportFromTool: %v", err)
	}
	if !reflect.DeepEqual(summary, syncer.ImportSummary{}) {
		t.Errorf("summary = %+v, want all-zero for an empty tool dir", summary)
	}
}

func TestFormatImportSummary(t *testing.T) {
	cases := []struct {
		name    string
		summary syncer.ImportSummary
		want    string
	}{
		{
			name:    "all zero",
			summary: syncer.ImportSummary{},
			want:    "imported nothing",
		},
		{
			name:    "singular everywhere",
			summary: syncer.ImportSummary{Rules: 1, Skills: 1, SkillDocs: 1, Agents: 1, Commands: 1},
			want:    "imported 1 rule, 1 skill, 1 skill doc, 1 agent, 1 command",
		},
		{
			name: "plural with root memory and skips",
			summary: syncer.ImportSummary{
				RootMemoryFrom: "CLAUDE.md",
				Skills:         3,
				Agents:         2,
				Skipped:        make([]syncer.SkippedFile, 4),
			},
			want: "imported 3 skills, 2 agents, AGENTS.md; skipped 4 files",
		},
		{
			name:    "root memory only",
			summary: syncer.ImportSummary{RootMemoryFrom: ".cursor/rules/general.mdc"},
			want:    "imported AGENTS.md",
		},
		{
			name:    "nothing imported but one skip",
			summary: syncer.ImportSummary{Skipped: make([]syncer.SkippedFile, 1)},
			want:    "imported nothing; skipped 1 file",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := syncer.FormatImportSummary(tc.summary); got != tc.want {
				t.Errorf("FormatImportSummary(%+v) = %q, want %q", tc.summary, got, tc.want)
			}
		})
	}
}

func TestImportFromSourcesDeterministicOrder(t *testing.T) {
	ws := t.TempDir()
	seedClaudeConcepts(t, ws)
	// unmappable markdown in two dirs → multiple Skipped entries to pin ordering
	seedImportFile(t, ws, ".cursor/agents/zeta.md", []byte("---\nname: zeta\n---\nbody\n"))
	seedImportFile(t, ws, ".cursor/agents/alpha.md", []byte("---\nname: alpha\n---\nbody\n"))
	seedImportFile(t, ws, ".cursor/commands/run.md", []byte("body\n"))
	src := claudeProjectSources()
	src.Dirs = append(src.Dirs, ".cursor/commands", ".cursor/agents")

	first, err := syncer.ImportFromSources(ws, src)
	if err != nil {
		t.Fatalf("first ImportFromSources: %v", err)
	}
	second, err := syncer.ImportFromSources(ws, src)
	if err != nil {
		t.Fatalf("second ImportFromSources: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Errorf("summaries differ across runs:\nfirst:  %+v\nsecond: %+v", first, second)
	}
	if len(first.Skipped) != 3 {
		t.Fatalf("Skipped = %+v, want 3 entries", first.Skipped)
	}
	wantOrder := []string{".cursor/agents/alpha.md", ".cursor/agents/zeta.md", ".cursor/commands/run.md"}
	for i, want := range wantOrder {
		if first.Skipped[i].Path != want {
			t.Errorf("Skipped[%d].Path = %q, want %q (sorted across dirs)", i, first.Skipped[i].Path, want)
		}
	}
}

func TestImportFromToolGeminiRootMemory(t *testing.T) {
	ws := t.TempDir()
	seedImportFile(t, ws, "GEMINI.md", []byte("# gemini memory\n"))

	summary, err := syncer.ImportFromTool(ws, findTool(t, "gemini"), tools.ScopeProject)

	if err != nil {
		t.Fatalf("ImportFromTool: %v", err)
	}
	if summary.RootMemoryFrom != "GEMINI.md" {
		t.Errorf("RootMemoryFrom = %q, want GEMINI.md", summary.RootMemoryFrom)
	}
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AgentsMD != "# gemini memory\n" {
		t.Errorf("AgentsMD = %q, want gemini memory content", c.AgentsMD)
	}
}
