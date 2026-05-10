package tui

import (
	"fmt"
	"os"
	"path/filepath"
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

	wantKinds := []fileKind{kindAgentsMD, kindSkill, kindAgent, kindCommand, kindRule}
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
	// AGENTS.md, skill, placeholder agent, placeholder command, placeholder rule
	if got := len(items); got != 5 {
		t.Fatalf("want 5 items, got %d", got)
	}
	if items[1].placeholder {
		t.Error("real skill row should not be placeholder")
	}
	if items[1].skill == nil || items[1].skill.Dir != "alpha" {
		t.Errorf("items[1] should reference skill 'alpha', got %+v", items[1])
	}
	for _, idx := range []int{2, 3, 4} {
		if !items[idx].placeholder {
			t.Errorf("items[%d] (%s) should be placeholder", idx, items[idx].label)
		}
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
