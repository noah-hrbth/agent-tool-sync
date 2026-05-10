package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

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
