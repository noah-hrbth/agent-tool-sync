package wizard

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// seedFile writes content at ws-relative rel, creating parent dirs.
func seedFile(t *testing.T, ws, rel, content string) {
	t.Helper()
	abs := filepath.Join(ws, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildOptionsPinsClaudeFirstRecommended(t *testing.T) {
	ws := t.TempDir()
	seedFile(t, ws, ".claude/skills/x/SKILL.md", "---\nname: x\ndescription: d\n---\nbody\n")
	seedFile(t, ws, ".cursor/rules/a.mdc", "rule body\n")
	seedFile(t, ws, ".rules", "zed rules\n")

	detected, options := BuildOptions(ws, tools.ScopeProject)

	for _, want := range []string{"Claude Code", "Cursor", "Zed"} {
		if !slices.Contains(detected, want) {
			t.Errorf("detectedNames missing %q, got %v", want, detected)
		}
	}
	if len(options) != 2 {
		t.Fatalf("want 2 options (claude, cursor), got %d: %+v", len(options), options)
	}
	if options[0].Tool.Meta.Key != "claude" || !options[0].Recommended {
		t.Errorf("options[0]: want claude recommended, got key=%q recommended=%v",
			options[0].Tool.Meta.Key, options[0].Recommended)
	}
	if options[1].Tool.Meta.Key != "cursor" || options[1].Recommended {
		t.Errorf("options[1]: want cursor not recommended, got key=%q recommended=%v",
			options[1].Tool.Meta.Key, options[1].Recommended)
	}
}

func TestBuildOptionsEmptyWorkspace(t *testing.T) {
	ws := t.TempDir()

	detected, options := BuildOptions(ws, tools.ScopeProject)

	if len(detected) != 0 {
		t.Errorf("want no detections in empty workspace, got %v", detected)
	}
	if len(options) != 0 {
		t.Errorf("want no options in empty workspace, got %+v", options)
	}
}

// seededWizard seeds ws with claude+cursor+zed installations and returns a
// wizard model whose options are [claude (recommended), cursor].
func seededWizard(t *testing.T, ws string) Model {
	t.Helper()
	seedFile(t, ws, ".claude/skills/x/SKILL.md", "---\nname: x\ndescription: d\n---\nbody\n")
	seedFile(t, ws, ".cursor/rules/a.mdc", "rule body\n")
	seedFile(t, ws, ".rules", "zed rules\n")
	_, options := BuildOptions(ws, tools.ScopeProject)
	if len(options) == 0 {
		t.Fatal("seeded workspace produced no import options")
	}
	return New(ws, tools.ScopeProject, config.Default(tools.Names()), options)
}

// runUntilDone executes cmd synchronously, unpacking tea.Batch and returning
// the first message that is not a spinner tick.
func runUntilDone(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return msg
	}
	for _, sub := range batch {
		if m := sub(); m != nil {
			if _, tick := m.(spinner.TickMsg); !tick {
				return m
			}
		}
	}
	t.Fatal("batch produced no non-tick message")
	return nil
}

// pressKey feeds one key to the model and returns the updated Model + cmd.
func pressKey(t *testing.T, m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(msg)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want wizard.Model", updated)
	}
	return next, cmd
}

func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

// assertQuit executes cmd and fails unless it yields tea.QuitMsg.
func assertQuit(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("want quit cmd, got nil")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("want tea.QuitMsg from cmd")
	}
}

func TestWizardMethodStepShowsChoices(t *testing.T) {
	m := seededWizard(t, t.TempDir())

	view := m.View()

	for _, want := range []string{"Import from a detected tool", "Start fresh", "project"} {
		if !strings.Contains(view, want) {
			t.Errorf("method view missing %q:\n%s", want, view)
		}
	}
}

func TestWizardNavigationAndSelectTool(t *testing.T) {
	m := seededWizard(t, t.TempDir())

	m, _ = pressKey(t, m, keyRune('j'))
	m, _ = pressKey(t, m, keyRune('k'))
	m, _ = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m, _ = pressKey(t, m, tea.KeyMsg{Type: tea.KeyUp})
	m, _ = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // select import

	if m.step != stepTool {
		t.Fatalf("want stepTool after selecting import, got %v", m.step)
	}
	view := m.View()
	claudeIdx := strings.Index(view, "Claude Code (recommended)")
	cursorIdx := strings.Index(view, "Cursor")
	if claudeIdx < 0 {
		t.Fatalf("tool view missing pinned %q:\n%s", "Claude Code (recommended)", view)
	}
	if cursorIdx >= 0 && cursorIdx < claudeIdx {
		t.Errorf("Claude Code should be listed before Cursor:\n%s", view)
	}
}

func TestWizardEscAborts(t *testing.T) {
	m := seededWizard(t, t.TempDir())

	m, cmd := pressKey(t, m, tea.KeyMsg{Type: tea.KeyEsc})

	assertQuit(t, cmd)
	if !m.Outcome().Aborted {
		t.Error("want Outcome().Aborted after esc at stepMethod")
	}
}

func TestWizardSelectFreshScaffolds(t *testing.T) {
	ws := t.TempDir()
	m := seededWizard(t, ws)

	m, _ = pressKey(t, m, tea.KeyMsg{Type: tea.KeyDown}) // move to "Start fresh"
	m, cmd := pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("want batched spinner-tick + scaffold cmd after selecting fresh, got nil")
	}
	msg := runUntilDone(t, cmd) // run Scaffold synchronously, skipping spinner ticks
	updated, doneCmd := m.Update(msg)
	m = updated.(Model)

	if m.step != stepDone {
		t.Fatalf("want stepDone, got %v", m.step)
	}
	view := m.View()
	for _, want := range []string{"✓ Initialized .agentsync/", "Run 'agentsync' to start"} {
		if !strings.Contains(view, want) {
			t.Errorf("done view missing %q:\n%s", want, view)
		}
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync")); err != nil {
		t.Errorf(".agentsync should exist on disk: %v", err)
	}
	assertQuit(t, doneCmd)
	out := m.Outcome()
	if out.Imported || out.Aborted || out.Err != nil {
		t.Errorf("fresh outcome should be clean, got %+v", out)
	}
}

// wizardAtRunning drives a seeded wizard to stepRunning by selecting the
// pinned claude option, returning the model and the returned cmd.
func wizardAtRunning(t *testing.T, ws string) (Model, tea.Cmd) {
	t.Helper()
	m := seededWizard(t, ws)
	m, _ = pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter})    // select import
	m, cmd := pressKey(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // select claude
	return m, cmd
}

func TestWizardSelectToolStartsImportWithSpinner(t *testing.T) {
	m, cmd := wizardAtRunning(t, t.TempDir())

	if m.step != stepRunning {
		t.Fatalf("want stepRunning after selecting a tool, got %v", m.step)
	}
	if cmd == nil {
		t.Fatal("want batched spinner-tick + import cmd, got nil")
	}
	view := m.View()
	frame := strings.TrimSpace(m.spin.View())
	if frame == "" {
		t.Fatal("spinner frame should be non-empty")
	}
	if !strings.Contains(view, frame) {
		t.Errorf("running view missing spinner frame %q:\n%s", frame, view)
	}
	if !strings.Contains(view, "Importing from Claude Code") {
		t.Errorf("running view missing import label:\n%s", view)
	}
}

func TestWizardImportDoneShowsSummary(t *testing.T) {
	m, _ := wizardAtRunning(t, t.TempDir())
	summary := syncer.ImportSummary{RootMemoryFrom: "CLAUDE.md", Skills: 3, Agents: 2}

	updated, doneCmd := m.Update(importDoneMsg{summary: summary})
	m = updated.(Model)

	if m.step != stepDone {
		t.Fatalf("want stepDone, got %v", m.step)
	}
	view := m.View()
	for _, want := range []string{"✓ Initialized .agentsync/ from Claude Code", syncer.FormatImportSummary(summary), "Run 'agentsync' to start"} {
		if !strings.Contains(view, want) {
			t.Errorf("done view missing %q:\n%s", want, view)
		}
	}
	assertQuit(t, doneCmd)
	out := m.Outcome()
	if !out.Imported || out.ToolName != "Claude Code" {
		t.Errorf("outcome: want Imported from Claude Code, got %+v", out)
	}
	if !reflect.DeepEqual(out.Summary, summary) {
		t.Errorf("outcome summary: want %+v, got %+v", summary, out.Summary)
	}
}

func TestWizardImportErrorSetsErr(t *testing.T) {
	m, _ := wizardAtRunning(t, t.TempDir())

	updated, _ := m.Update(importDoneMsg{err: errors.New("boom")})
	m = updated.(Model)

	if m.step != stepError {
		t.Fatalf("want stepError, got %v", m.step)
	}
	if !strings.Contains(m.View(), "boom") {
		t.Errorf("error view missing error text:\n%s", m.View())
	}
	if m.Outcome().Err == nil {
		t.Error("want Outcome().Err set")
	}
	_, quitCmd := pressKey(t, m, keyRune('x')) // any key quits
	assertQuit(t, quitCmd)
}

func TestWizardDoneHintIsScopeAware(t *testing.T) {
	ws := t.TempDir()
	seedFile(t, ws, ".claude/skills/x/SKILL.md", "---\nname: x\ndescription: d\n---\nbody\n")
	_, options := BuildOptions(ws, tools.ScopeUser)
	m := New(ws, tools.ScopeUser, config.Default(tools.Names()), append(options, SourceOption{}))

	updated, _ := m.Update(freshDoneMsg{})
	view := updated.(Model).View()

	if !strings.Contains(view, "agentsync --global") {
		t.Errorf("user-scope done view must hint 'agentsync --global', got: %s", view)
	}
}

func TestWizardCtrlCIgnoredWhileRunning(t *testing.T) {
	m, _ := wizardAtRunning(t, t.TempDir())

	next, cmd := pressKey(t, m, tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd != nil {
		t.Error("ctrl+c at stepRunning must not emit a cmd; the operation finishes")
	}
	if next.Outcome().Aborted {
		t.Error("ctrl+c at stepRunning must not mark the outcome aborted")
	}
	if next.step != stepRunning {
		t.Errorf("step changed to %v, want stepRunning", next.step)
	}
}
