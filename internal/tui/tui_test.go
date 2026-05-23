package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// mouseMsg builds a tea.MouseMsg with the given coords/button/action. Used by
// the mouse-handling tests below.
func mouseMsg(x, y int, btn tea.MouseButton, action tea.MouseAction) tea.MouseMsg {
	return tea.MouseMsg(tea.MouseEvent{X: x, Y: y, Button: btn, Action: action})
}

// newTestModel constructs an initialModel rooted at ws with project scope and
// applies a 120x40 WindowSizeMsg, returning the (mutated) model. Used by mouse
// tests that need a real layout.
func newTestModel(t *testing.T, ws string) model {
	t.Helper()
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())
	var m tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(model)
}

// writeAgentsMD seeds ws/.agentsync/AGENTS.md so canonical.Load succeeds.
func writeAgentsMD(t *testing.T, ws string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTestModelScoped constructs an initialModel at the given scope and sizes
// the window to 120x40. Used by sync-screen tests that need to flip scope.
func newTestModelScoped(t *testing.T, ws string, scope tools.Scope) model {
	t.Helper()
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())
	var m tea.Model = initialModel(ws, scope, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m.(model)
}

func TestRuleAppendNotice(t *testing.T) {
	cases := []struct {
		adapter string
		want    string
	}{
		{"JetBrains Junie", "appended to AGENTS.md"},
		{"Cline", ""}, // Cline supports per-file rules — no append notice
		{"Gemini CLI", "appended to GEMINI.md"},
		{"Claude Code", ""},
		{"Mistral Vibe", "appended to AGENTS.md"},
	}
	for _, c := range cases {
		t.Run(c.adapter, func(t *testing.T) {
			if got := ruleAppendNotice(c.adapter); got != c.want {
				t.Errorf("ruleAppendNotice(%q): got %q, want %q", c.adapter, got, c.want)
			}
		})
	}
}

func TestBuildFileItemsOrder(t *testing.T) {
	c := &canonical.Canonical{
		AgentsMD: "# rules",
		Skills:   []*canonical.Skill{{Dir: "code-review", Name: "code-review", Description: "test"}},
		Agents:   []*canonical.Agent{{Filename: "explorer", Name: "explorer", Description: "test"}},
		Commands: []*canonical.Command{{Filename: "commit", Description: "test"}},
		Rules:    []*canonical.Rule{{Filename: "style-guide", Description: "test"}},
	}
	items := buildFileItems(c)

	// One skill (no docs) expands to a dir node + manifest row (both kindSkill).
	wantKinds := []fileKind{kindAgentsMD, kindSkill, kindSkill, kindAgent, kindCommand, kindRule}
	if len(items) != len(wantKinds) {
		t.Fatalf("buildFileItems: want %d items, got %d", len(wantKinds), len(items))
	}
	for i, want := range wantKinds {
		if items[i].kind != want {
			t.Errorf("items[%d]: want kind %d, got %d (label=%q)", i, want, items[i].kind, items[i].label)
		}
	}
}

func TestBuildFileItemsPlaceholdersForEmptyGroups(t *testing.T) {
	// Empty canonical: AGENTS.md row + one placeholder per concept group.
	c := &canonical.Canonical{
		AgentsMD: "",
		Skills:   []*canonical.Skill{},
		Agents:   []*canonical.Agent{},
		Commands: []*canonical.Command{},
		Rules:    []*canonical.Rule{},
	}
	items := buildFileItems(c)
	wantKinds := []fileKind{kindAgentsMD, kindSkill, kindAgent, kindCommand, kindRule}
	if len(items) != len(wantKinds) {
		t.Fatalf("want %d items, got %d", len(wantKinds), len(items))
	}
	for i, want := range wantKinds {
		if items[i].kind != want {
			t.Errorf("items[%d].kind: want %d got %d (label=%q)", i, want, items[i].kind, items[i].label)
		}
		if i == 0 {
			if items[i].placeholder {
				t.Error("AGENTS.md row should not be a placeholder")
			}
		} else {
			if !items[i].placeholder {
				t.Errorf("items[%d] (%s) should be a placeholder", i, items[i].label)
			}
		}
	}
}

func TestBuildFileItemsMixedGroups(t *testing.T) {
	// Skills populated, others empty: real Skill rows + placeholders for the rest.
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{Dir: "alpha", Name: "alpha", Description: "d"}},
	}
	items := buildFileItems(c)
	// AGENTS.md, skill dir node, skill manifest, placeholder agent/command/rule
	if got := len(items); got != 6 {
		t.Fatalf("want 6 items, got %d", got)
	}
	if !items[1].isSkillDir || items[1].skill == nil || items[1].skill.Dir != "alpha" {
		t.Errorf("items[1] should be the 'alpha' dir node, got %+v", items[1])
	}
	if items[2].isSkillDir || items[2].skillDoc != "" || items[2].skill == nil {
		t.Errorf("items[2] should be the manifest row, got %+v", items[2])
	}
	for _, idx := range []int{3, 4, 5} {
		if !items[idx].placeholder {
			t.Errorf("items[%d] (%s) should be placeholder", idx, items[idx].label)
		}
	}
}

func TestBuildFileItemsSkillTree(t *testing.T) {
	// Arrange: a skill with a sibling doc and a nested doc.
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir: "pdf-tools", Name: "pdf-tools",
			Docs: []canonical.SkillDoc{
				{RelPath: "reference.md", Content: "r"},
				{RelPath: "examples/invoice.md", Content: "i"},
			},
		}},
	}

	// Act
	items := buildFileItems(c)

	// Assert: dir node, manifest (pinned first), then docs in order.
	// items[0] is AGENTS.md.
	dirNode := items[1]
	if !dirNode.isSkillDir || dirNode.skill.Dir != "pdf-tools" {
		t.Fatalf("items[1] should be the pdf-tools dir node, got %+v", dirNode)
	}
	manifest := items[2]
	if manifest.isSkillDir || manifest.skillDoc != "" {
		t.Fatalf("items[2] should be the manifest, got %+v", manifest)
	}
	// Files come before subdir nodes; nested docs sit under a subdir node.
	if items[3].skillDoc != "reference.md" {
		t.Errorf("items[3] should be reference.md, got %q", items[3].skillDoc)
	}
	if items[4].skillSubdir != "examples" || items[4].label != "examples/" {
		t.Errorf("items[4] should be the examples/ subdir node, got %+v", items[4])
	}
	if items[5].skillDoc != "examples/invoice.md" || items[5].label != "invoice.md" {
		t.Errorf("items[5] should be examples/invoice.md (basename label), got %+v", items[5])
	}
}

func TestBuildFileItemsLabelsOmitFolderPrefix(t *testing.T) {
	// Group headers already name the concept, so rows show just the filename.
	c := &canonical.Canonical{
		Agents:   []*canonical.Agent{{Filename: "explorer", Name: "explorer"}},
		Commands: []*canonical.Command{{Filename: "commit"}},
		Rules:    []*canonical.Rule{{Filename: "style-guide"}},
	}
	items := buildFileItems(c)

	want := map[fileKind]string{
		kindAgent:   "explorer.md",
		kindCommand: "commit.md",
		kindRule:    "style-guide.md",
	}
	for _, f := range items {
		if f.placeholder {
			continue
		}
		if w, ok := want[f.kind]; ok {
			if f.label != w {
				t.Errorf("kind %d label = %q, want %q", f.kind, f.label, w)
			}
		}
	}
}

func TestBuildFileItemsSubdirIndentation(t *testing.T) {
	c := &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir: "pdf-tools", Name: "pdf-tools",
			Docs: []canonical.SkillDoc{{RelPath: "tests/nummer1.md"}},
		}},
	}
	items := buildFileItems(c)

	var subNode, leaf *fileItem
	for i := range items {
		if items[i].skillSubdir == "tests" {
			subNode = &items[i]
		}
		if items[i].skillDoc == "tests/nummer1.md" {
			leaf = &items[i]
		}
	}
	if subNode == nil || leaf == nil {
		t.Fatalf("expected a tests/ subdir node and a nested doc; items=%+v", items)
	}
	if len(rowIndent(*leaf)) <= len(rowIndent(*subNode)) {
		t.Errorf("nested doc (%q) should indent deeper than its subdir node (%q)",
			rowIndent(*leaf), rowIndent(*subNode))
	}
}

func TestAddDocToFocusedSubdir(t *testing.T) {
	ws := seedSkillWorkspace(t)
	if err := canonical.SaveSkillDoc(ws, "pdf-tools", "tests/nummer1.md", "x\n"); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, ws)
	idx := -1
	for i, f := range m.files {
		if f.skillDoc == "tests/nummer1.md" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("tests/nummer1.md row not found")
	}
	m.fileIdx = idx

	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("nummer2.md")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "tests", "nummer2.md")); err != nil {
		t.Errorf("add-doc should create under the focused subdir tests/: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "nummer2.md")); !os.IsNotExist(err) {
		t.Error("add-doc must not create at skill root when focused on a subdir")
	}
}

func TestDeleteSubdirNodeRemovesFolder(t *testing.T) {
	ws := seedSkillWorkspace(t)
	if err := canonical.SaveSkillDoc(ws, "pdf-tools", "tests/nummer1.md", "x\n"); err != nil {
		t.Fatal(err)
	}
	if err := canonical.SaveSkillDoc(ws, "pdf-tools", "tests/nummer2.md", "y\n"); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, ws)
	idx := -1
	for i, f := range m.files {
		if f.skillSubdir == "tests" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("tests/ subdir node not found")
	}
	m.deleteTarget = idx

	_, _ = m.confirmDelete()

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "tests")); !os.IsNotExist(err) {
		t.Error("subdir node delete should remove the folder")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md")); err != nil {
		t.Error("manifest must survive subdir delete")
	}
}

func TestMatchesFileItemSkillManifestAndDoc(t *testing.T) {
	skill := &canonical.Skill{Dir: "pdf-tools"}
	dirNode := fileItem{kind: kindSkill, skill: skill, isSkillDir: true}
	manifest := fileItem{kind: kindSkill, skill: skill}
	doc := fileItem{kind: kindSkill, skill: skill, skillDoc: "reference.md"}

	if !matchesFileItem(manifest, ".claude/skills/pdf-tools/SKILL.md") {
		t.Error("manifest row should match its SKILL.md path")
	}
	if matchesFileItem(manifest, ".claude/skills/pdf-tools/reference.md") {
		t.Error("manifest row must not match a doc path")
	}
	if !matchesFileItem(doc, ".claude/skills/pdf-tools/reference.md") {
		t.Error("doc row should match its doc path")
	}
	if matchesFileItem(doc, ".claude/skills/pdf-tools/SKILL.md") {
		t.Error("doc row must not match the manifest path")
	}
	// Dir node rolls up the whole skill (manifest + docs).
	if !matchesFileItem(dirNode, ".claude/skills/pdf-tools/SKILL.md") ||
		!matchesFileItem(dirNode, ".claude/skills/pdf-tools/reference.md") {
		t.Error("dir node should match every file under the skill")
	}
}

func TestMatchesFileItemSkillNoPartialDirCollision(t *testing.T) {
	manifest := fileItem{kind: kindSkill, skill: &canonical.Skill{Dir: "pdf-tools"}}
	if matchesFileItem(manifest, ".claude/skills/x-pdf-tools/SKILL.md") {
		t.Error("pdf-tools manifest must not match x-pdf-tools paths")
	}
}

func TestValidateNewName(t *testing.T) {
	existing := &canonical.Canonical{
		Skills:   []*canonical.Skill{{Dir: "release-prep"}},
		Rules:    []*canonical.Rule{{Filename: "style-guide"}},
		Agents:   []*canonical.Agent{{Filename: "adapter-reviewer"}},
		Commands: []*canonical.Command{{Filename: "ship"}},
	}

	cases := []struct {
		name    string
		kind    fileKind
		input   string
		wantOK  bool
		wantOut string
	}{
		{"empty", kindRule, "", false, ""},
		{"whitespace only", kindRule, "   ", false, ""},
		{"trailing .md stripped", kindRule, "foo.md", true, "foo"},
		{"uppercase rejected", kindRule, "Foo", false, ""},
		{"slash rejected", kindRule, "foo/bar", false, ""},
		{"space rejected", kindRule, "foo bar", false, ""},
		{"dot rejected literal", kindRule, ".", false, ""},
		{"dotdot rejected literal", kindRule, "..", false, ""},
		{"reserved general for rule", kindRule, "general", false, ""},
		{"general allowed for non-rule", kindSkill, "general", true, "general"},
		{"duplicate skill", kindSkill, "release-prep", false, ""},
		{"duplicate rule", kindRule, "style-guide", false, ""},
		{"duplicate agent", kindAgent, "adapter-reviewer", false, ""},
		{"duplicate command", kindCommand, "ship", false, ""},
		{"valid rule slug", kindRule, "new-rule", true, "new-rule"},
		{"valid skill dir", kindSkill, "new_skill.v2", true, "new_skill.v2"},
		{"unicode rejected", kindRule, "café", false, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validateNewName(c.kind, c.input, existing)
			if c.wantOK {
				if err != nil {
					t.Fatalf("want ok, got err: %v", err)
				}
				if got != c.wantOut {
					t.Errorf("got slug %q, want %q", got, c.wantOut)
				}
			} else {
				if err == nil {
					t.Fatalf("want err, got slug %q", got)
				}
			}
		})
	}
}

// seedSkillWorkspace writes a skill (manifest + one doc) so model tests can
// exercise the skill tree. Returns the workspace root.
func seedSkillWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "SKILL.md"), []byte("---\nname: pdf-tools\n---\n# manifest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "reference.md"), []byte("# reference\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte("# rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws
}

// Row layout for seedSkillWorkspace: 0 AGENTS.md, 1 dir node, 2 SKILL.md, 3 reference.md.

func TestAddDocKeyCreatesDocUnderSkill(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws)
	m.fileIdx = 2 // SKILL.md manifest row

	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if !tm.(model).inputting {
		t.Fatal("pressing 'a' on a skill row should open the add-doc input")
	}
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("docs/test.md")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "docs", "test.md")); err != nil {
		t.Errorf("add-doc did not create the doc: %v", err)
	}
}

func TestAddDocRejectsNonMarkdownName(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws)
	m.fileIdx = 2

	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("notes.txt")})
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if tm.(model).inputErr == "" {
		t.Error("non-.md doc name should be rejected with an inputErr")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "notes.txt")); !os.IsNotExist(err) {
		t.Error("invalid doc must not be created")
	}
}

func TestSaveDocRowWritesDoc(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws)
	m.fileIdx = 3 // reference.md doc row

	if err := m.saveCurrentFile("# edited reference\n"); err != nil {
		t.Fatalf("saveCurrentFile: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "reference.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "# edited reference\n" {
		t.Errorf("doc content = %q, want edited", got)
	}
	man, _ := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md"))
	if !strings.Contains(string(man), "manifest") {
		t.Errorf("manifest must be unchanged by a doc save, got %q", man)
	}
}

func TestDeleteDocRowDeletesOnlyFile(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws)
	m.deleteTarget = 3 // reference.md doc row

	if _, _ = m.confirmDelete(); false {
		t.Fatal("unreachable")
	}

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "reference.md")); !os.IsNotExist(err) {
		t.Error("doc row delete should remove the doc")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md")); err != nil {
		t.Error("manifest must survive a doc delete")
	}
}

func TestDeleteSkillDirNodeDeletesWholeSkill(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws)
	m.deleteTarget = 1 // dir node

	_, _ = m.confirmDelete()

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools")); !os.IsNotExist(err) {
		t.Error("dir node delete should remove the whole skill folder")
	}
}

func TestPreviewFollowsCursor(t *testing.T) {
	ws := seedSkillWorkspace(t)
	m := newTestModel(t, ws) // idx 0 = AGENTS.md, idx 1 = pdf-tools dir node

	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}) // move to dir node

	got := tm.(model).preview.View()
	if !strings.Contains(got, "skill folder") {
		t.Errorf("preview should follow the cursor to the skill dir node; got %q", got)
	}
}

func TestToolsScreenEnterOpensInfoModal(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())

	var m tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // → screenTools

	if m.(model).screen != screenTools {
		t.Fatalf("want screenTools, got %d", m.(model).screen)
	}

	// space toggles the highlighted tool's enabled state, does NOT open info.
	wantToggled := !m.(model).toolItems[m.(model).toolIdx].enabled
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	if got := m.(model).toolItems[m.(model).toolIdx].enabled; got != wantToggled {
		t.Errorf("space: want toggle to %v, got %v", wantToggled, got)
	}
	if m.(model).showToolInfo {
		t.Error("space should not open the info modal")
	}

	// enter opens the info modal.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !m.(model).showToolInfo {
		t.Error("enter on Tools tab should open the info modal")
	}

	// esc closes the modal.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.(model).showToolInfo {
		t.Error("esc should close the info modal")
	}

	// View renders without panic when modal is open.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = m.(model).View()
}

func TestCopyKeyCopiesCurrentFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	const agentsBody = "# Rules\n\nbe concise\n"
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte(agentsBody), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())

	var captured string
	prev := clipboardWrite
	clipboardWrite = func(s string) error { captured = s; return nil }
	t.Cleanup(func() { clipboardWrite = prev })

	var m tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Cursor starts on AGENTS.md, the only non-placeholder row in this canonical.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if captured != agentsBody {
		t.Errorf("clipboard payload: got %q, want %q", captured, agentsBody)
	}
	mm := m.(model)
	if mm.flash == "" {
		t.Error("flash should be set after copy")
	}

	// Any other key clears the flash.
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if got := m.(model).flash; got != "" {
		t.Errorf("flash should clear on next keystroke, got %q", got)
	}
}

func TestCopyKeyOnPlaceholderIsNoop(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())

	called := false
	prev := clipboardWrite
	clipboardWrite = func(string) error { called = true; return nil }
	t.Cleanup(func() { clipboardWrite = prev })

	var m tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	// Move down to first placeholder row (skills group is empty).
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if !m.(model).files[m.(model).fileIdx].placeholder {
		t.Fatalf("expected placeholder at idx %d", m.(model).fileIdx)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if called {
		t.Error("clipboardWrite should not be called on a placeholder row")
	}
	if got := m.(model).flash; got != "" {
		t.Errorf("flash should be empty for placeholder copy, got %q", got)
	}
}

func TestSmoke(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte("# Rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())

	var m tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	m, _ = m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	// Tab cycles forward: Files(0) → Tools(1) → Sync(2) → Files(0)
	for i, want := range []screen{screenTools, screenSync, screenFiles} {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		if got := m.(model).screen; got != want {
			t.Errorf("tab[%d]: want screen %d, got %d", i, want, got)
		}
	}

	// Shift+Tab cycles backward: Files(0) → Sync(2) → Tools(1) → Files(0)
	for i, want := range []screen{screenSync, screenTools, screenFiles} {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
		if got := m.(model).screen; got != want {
			t.Errorf("shift+tab[%d]: want screen %d, got %d", i, want, got)
		}
	}

	// j/k navigation doesn't panic
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	_ = m.View()
}

func TestMouseClickTabs(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)

	// Each tab is rendered with the same width regardless of active state
	// (Padding(0,2) on both styleTab and styleTabActive). Click in the middle
	// of each tab and assert the screen switches.
	tabWidth := lipgloss.Width(styleTabActive.Render("[1] Files"))

	cases := []struct {
		name string
		x    int
		want screen
	}{
		{"Files", tabWidth / 2, screenFiles},
		{"Tools", tabWidth + tabWidth/2, screenTools},
		{"Sync", 2*tabWidth + tabWidth/2, screenSync},
	}
	var tm tea.Model = m
	for _, c := range cases {
		tm, _ = tm.Update(mouseMsg(c.x, 0, tea.MouseButtonLeft, tea.MouseActionPress))
		if got := tm.(model).screen; got != c.want {
			t.Errorf("click tab %q at X=%d: want screen %d, got %d", c.name, c.x, c.want, got)
		}
	}
}

func TestMouseClickScope(t *testing.T) {
	// Redirect HOME so toggleScope's user-scope load reads from a temp dir.
	t.Setenv("HOME", t.TempDir())

	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	startScope := m.scope

	l := m.computeLayout()
	clickX := (l.scopeXRange.start + l.scopeXRange.end) / 2

	var tm tea.Model = m
	tm, _ = tm.Update(mouseMsg(clickX, 0, tea.MouseButtonLeft, tea.MouseActionPress))

	if got := tm.(model).scope; got == startScope {
		t.Errorf("click scope label: scope did not flip from %v", startScope)
	}
}

func TestMouseClickFileRow(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	// Seed three skills so we have predictable, non-placeholder rows.
	skillsDir := filepath.Join(ws, ".agentsync", "skills")
	for _, name := range []string{"alpha", "bravo", "charlie"} {
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: t\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(skillsDir, name, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := newTestModel(t, ws)
	// Pick the second skill row (index 2 = AGENTS.md(0), skill[0](1), skill[1](2)).
	targetIdx := 2
	l := m.computeLayout()
	rowY := l.filesListInnerY0 + m.fileRowYOffset(targetIdx) - m.fileList.YOffset

	var tm tea.Model = m
	tm, _ = tm.Update(mouseMsg(l.filesLeftPanelX+3, rowY, tea.MouseButtonLeft, tea.MouseActionPress))
	if got := tm.(model).fileIdx; got != targetIdx {
		t.Errorf("click file row: want fileIdx=%d, got %d", targetIdx, got)
	}
}

func TestMouseWheelFilesLeft(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	// Seed enough skills to overflow a 40-row terminal's left panel.
	skillsDir := filepath.Join(ws, ".agentsync", "skills")
	for i := 0; i < 80; i++ {
		name := fmt.Sprintf("skill-%03d", i)
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: t\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(skillsDir, name, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := newTestModel(t, ws)
	l := m.computeLayout()
	leftX := l.filesLeftPanelX + 3
	rowY := l.filesListInnerY0 + 5

	var tm tea.Model = m
	for i := 0; i < 5; i++ {
		tm, _ = tm.Update(mouseMsg(leftX, rowY, tea.MouseButtonWheelDown, tea.MouseActionPress))
	}
	mm := tm.(model)
	if mm.fileList.YOffset == 0 {
		t.Errorf("wheel down on left list: fileList.YOffset still 0")
	}
	if mm.preview.YOffset != 0 {
		t.Errorf("wheel down on left list: preview.YOffset = %d, want 0", mm.preview.YOffset)
	}
}

func TestMouseWheelFilesRight(t *testing.T) {
	ws := t.TempDir()
	// Seed AGENTS.md with a long body so the preview overflows the right panel.
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Rules\n\n"
	for i := 0; i < 200; i++ {
		body += fmt.Sprintf("line %d of long content\n", i)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t, ws)
	l := m.computeLayout()
	rightX := l.filesRightPanelX + 5
	rowY := l.filesPanelTopY + 5

	var tm tea.Model = m
	for i := 0; i < 5; i++ {
		tm, _ = tm.Update(mouseMsg(rightX, rowY, tea.MouseButtonWheelDown, tea.MouseActionPress))
	}
	mm := tm.(model)
	if mm.preview.YOffset == 0 {
		t.Errorf("wheel down on right pane: preview.YOffset still 0")
	}
	if mm.fileList.YOffset != 0 {
		t.Errorf("wheel down on right pane: fileList.YOffset = %d, want 0", mm.fileList.YOffset)
	}
}

func TestCtrlDOnTools(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)

	// Use a tiny window so the 9-adapter tool list overflows the visible area
	// and HalfPageDown actually advances YOffset.
	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load canonical: %v", err)
	}
	cfg := config.Default(tools.Names())
	var tm tea.Model = initialModel(ws, tools.ScopeProject, c, cfg, tools.All())
	tm, _ = tm.Update(tea.WindowSizeMsg{Width: 120, Height: 12})

	startIdx := tm.(model).toolIdx
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyTab}) // → screenTools
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyCtrlD})

	mm := tm.(model)
	if mm.toolList.YOffset == 0 {
		t.Errorf("ctrl+d on Tools: toolList.YOffset still 0 (height=%d, total rows=%d)",
			mm.toolList.Height, len(mm.toolItems))
	}
	if mm.toolIdx != startIdx {
		t.Errorf("ctrl+d on Tools: toolIdx changed from %d to %d (should stay put)", startIdx, mm.toolIdx)
	}
}

func TestFileCursorFollow(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	skillsDir := filepath.Join(ws, ".agentsync", "skills")
	for i := 0; i < 80; i++ {
		name := fmt.Sprintf("skill-%03d", i)
		if err := os.MkdirAll(filepath.Join(skillsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: t\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(skillsDir, name, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := newTestModel(t, ws)
	var tm tea.Model = m
	for i := 0; i < 60; i++ {
		tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	}
	mm := tm.(model)
	rowY := mm.fileRowYOffset(mm.fileIdx)
	if rowY < mm.fileList.YOffset || rowY >= mm.fileList.YOffset+mm.fileList.Height {
		t.Errorf("cursor row Y=%d outside visible window [%d, %d) after follow",
			rowY, mm.fileList.YOffset, mm.fileList.YOffset+mm.fileList.Height)
	}
}

func TestMouseIgnoredWhenEditing(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Rules\n\n"
	for i := 0; i < 200; i++ {
		body += fmt.Sprintf("line %d\n", i)
	}
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", "AGENTS.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t, ws)
	var tm tea.Model = m
	tm, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if !tm.(model).editing {
		t.Fatalf("expected editor to be open after pressing e")
	}

	l := tm.(model).computeLayout()
	rightX := l.filesRightPanelX + 5
	rowY := l.filesPanelTopY + 5
	for i := 0; i < 5; i++ {
		tm, _ = tm.Update(mouseMsg(rightX, rowY, tea.MouseButtonWheelDown, tea.MouseActionPress))
	}

	mm := tm.(model)
	if mm.preview.YOffset != 0 {
		t.Errorf("wheel during editing scrolled preview: YOffset=%d, want 0", mm.preview.YOffset)
	}
	if mm.fileList.YOffset != 0 {
		t.Errorf("wheel during editing scrolled fileList: YOffset=%d, want 0", mm.fileList.YOffset)
	}
}

// ---- gitignore banner ----

func TestSyncBannerShowsManagingSummaryWhenManageTrue(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.config.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
	out := m.viewSync()
	if !strings.Contains(out, "manages") {
		t.Errorf("expected banner to mention managing, got %q", out)
	}
	if !strings.Contains(out, ".claude/") {
		t.Errorf("expected banner to list at least .claude/, got %q", out)
	}
}

func TestSyncBannerShowsOffWhenPromptedAndManageFalse(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.config.Gitignore = config.GitignoreConfig{Manage: false, Prompted: true}
	out := m.viewSync()
	if !strings.Contains(out, "off") {
		t.Errorf("expected banner to say off, got %q", out)
	}
}

func TestSyncBannerShowsNotConfiguredWhenNotPrompted(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	out := m.viewSync()
	if !strings.Contains(out, "not configured") {
		t.Errorf("expected banner to say not configured, got %q", out)
	}
}

func TestSyncBannerHiddenAtUserScope(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModelScoped(t, ws, tools.ScopeUser)
	out := m.viewSync()
	for _, marker := range []string{"manages", "not configured", "off — edit"} {
		if strings.Contains(out, marker) {
			t.Errorf("user scope should not show gitignore banner (%q found in %q)", marker, out)
		}
	}
}

// ---- gitignore first-sync modal ----

// pressKey sends a single rune keypress to the model and returns the updated
// model + any Cmd produced. Strings like "esc" and "enter" map to the matching
// key types.
func pressKey(t *testing.T, m tea.Model, key string) (tea.Model, tea.Cmd) {
	t.Helper()
	switch key {
	case "esc":
		return m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	case "enter":
		return m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	}
	return m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
}

func TestGitignoreModalShowsOnFirstSyncProjectScope(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, cmd := pressKey(t, tm, "s")
	if !tm.(model).showGitignorePrompt {
		t.Errorf("expected showGitignorePrompt to be true after first sync")
	}
	if cmd != nil {
		t.Errorf("sync should not dispatch while modal is open (got non-nil Cmd)")
	}
}

func TestGitignoreModalNotShownWhenPrompted(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.config.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
	m.screen = screenSync
	var tm tea.Model = m
	tm, cmd := pressKey(t, tm, "s")
	if tm.(model).showGitignorePrompt {
		t.Errorf("modal must not appear when Prompted=true")
	}
	if cmd == nil {
		t.Errorf("expected sync to dispatch (non-nil Cmd) when Prompted=true")
	}
}

func TestGitignoreModalApplyWritesBlockPersistsConfigAndContinuesSync(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	if !tm.(model).showGitignorePrompt {
		t.Fatalf("modal should be open")
	}
	tm, cmd := pressKey(t, tm, "a")
	mm := tm.(model)
	if mm.showGitignorePrompt {
		t.Errorf("modal should close after apply")
	}
	if !mm.config.Gitignore.Manage || !mm.config.Gitignore.Prompted {
		t.Errorf("expected Manage=true Prompted=true, got %+v", mm.config.Gitignore)
	}
	got, err := os.ReadFile(filepath.Join(ws, ".gitignore"))
	if err != nil {
		t.Fatalf("expected .gitignore: %v", err)
	}
	if !strings.Contains(string(got), "# BEGIN agentsync managed") {
		t.Errorf("expected managed block, got %q", string(got))
	}
	// Re-load config from disk to verify persistence.
	loaded, err := config.Load(ws, tools.Names())
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if !loaded.Gitignore.Manage || !loaded.Gitignore.Prompted {
		t.Errorf("config not persisted: %+v", loaded.Gitignore)
	}
	if cmd == nil {
		t.Errorf("expected sync to dispatch after apply (got nil Cmd)")
	}
}

func TestGitignoreModalSkipRemovesBlockPersistsConfigAndContinuesSync(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	// Pre-seed a managed block to verify Skip removes it.
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte("# BEGIN agentsync managed\n.old/\n# END agentsync managed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	tm, cmd := pressKey(t, tm, "s") // 's' inside modal = skip
	mm := tm.(model)
	if mm.showGitignorePrompt {
		t.Errorf("modal should close after skip")
	}
	if mm.config.Gitignore.Manage {
		t.Errorf("expected Manage=false after skip")
	}
	if !mm.config.Gitignore.Prompted {
		t.Errorf("expected Prompted=true after skip")
	}
	got, err := os.ReadFile(filepath.Join(ws, ".gitignore"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(got), "# BEGIN agentsync managed") {
		t.Errorf("block should have been removed, got %q", string(got))
	}
	loaded, err := config.Load(ws, tools.Names())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if loaded.Gitignore.Manage || !loaded.Gitignore.Prompted {
		t.Errorf("config not persisted: %+v", loaded.Gitignore)
	}
	if cmd == nil {
		t.Errorf("expected sync to dispatch after skip (got nil Cmd)")
	}
}

func TestGitignoreModalEscDismissesWithoutPersistOrSync(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	tm, cmd := pressKey(t, tm, "esc")
	mm := tm.(model)
	if mm.showGitignorePrompt {
		t.Errorf("modal should close after esc")
	}
	if mm.config.Gitignore.Prompted {
		t.Errorf("esc must not flip Prompted")
	}
	if cmd != nil {
		t.Errorf("esc must not dispatch sync (got non-nil Cmd)")
	}
	// And modal must re-open on next 's'.
	tm, _ = pressKey(t, tm, "s")
	if !tm.(model).showGitignorePrompt {
		t.Errorf("modal should re-open on next 's' after esc")
	}
}

func TestGitignoreModalNeverShowsAtUserScope(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModelScoped(t, ws, tools.ScopeUser)
	m.screen = screenSync
	var tm tea.Model = m
	tm, cmd := pressKey(t, tm, "s")
	if tm.(model).showGitignorePrompt {
		t.Errorf("modal must not appear at user scope")
	}
	if cmd == nil {
		t.Errorf("sync should dispatch at user scope (no prompt needed)")
	}
}

func TestGitignoreModalBlocksMouseWhileOpen(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	if !tm.(model).showGitignorePrompt {
		t.Fatalf("modal should be open")
	}
	// Mouse events must be dropped while the modal is up so background viewports
	// don't scroll behind it.
	before := tm.(model)
	tm, _ = tm.Update(mouseMsg(10, 10, tea.MouseButtonWheelDown, tea.MouseActionPress))
	after := tm.(model)
	if before.logView.YOffset != after.logView.YOffset {
		t.Errorf("logView scrolled while modal open: before=%d after=%d", before.logView.YOffset, after.logView.YOffset)
	}
}

func TestGitignoreModalViewRendersWithoutPanic(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	_ = tm.(model).View()
}

func TestGitignoreModalDeferredBehindDivergence(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	// Seed a snapshot referencing CLAUDE.md with a known hash, then write a
	// CLAUDE.md whose disk content hashes to something else → Status reports
	// it as Divergent.
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync", ".state"), 0o755); err != nil {
		t.Fatal(err)
	}
	snap := `{"files":{"CLAUDE.md":"0000000000000000000000000000000000000000000000000000000000000000"}}`
	if err := os.WriteFile(filepath.Join(ws, ".agentsync", ".state", "snapshot.json"), []byte(snap), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "CLAUDE.md"), []byte("externally edited content"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, ws)
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	mm := tm.(model)
	if !mm.showDiv {
		t.Errorf("expected divergence modal to be shown first")
	}
	if mm.showGitignorePrompt {
		t.Errorf("gitignore prompt must defer behind divergence modal")
	}
}

func TestGitignoreRefreshesOnSubsequentSyncWhenManageTrue(t *testing.T) {
	ws := t.TempDir()
	writeAgentsMD(t, ws)
	// Pre-seed an empty .gitignore so the silent refresh has somewhere to land.
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	m := newTestModel(t, ws)
	m.config.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
	m.screen = screenSync
	var tm tea.Model = m
	tm, _ = pressKey(t, tm, "s")
	got, err := os.ReadFile(filepath.Join(ws, ".gitignore"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "# BEGIN agentsync managed") {
		t.Errorf("expected silent refresh to write managed block, got %q", string(got))
	}
}
