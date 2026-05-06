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

func TestBuildFileItemsNoRulesSection(t *testing.T) {
	c := &canonical.Canonical{
		AgentsMD: "# rules",
		Skills:   []*canonical.Skill{{Dir: "s", Name: "s", Description: "d"}},
		Agents:   []*canonical.Agent{},
		Commands: []*canonical.Command{},
		Rules:    []*canonical.Rule{},
	}
	items := buildFileItems(c)
	for _, item := range items {
		if item.kind == kindRule {
			t.Error("expected no kindRule items when c.Rules is empty")
		}
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
