package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/tools"
	"github.com/noah-hrbth/agentsync/internal/wizard"
)

// stubWizard records the arguments of a single wizardRunner invocation and
// returns the configured outcome.
type stubWizard struct {
	called  bool
	cfg     *config.Config
	options []wizard.SourceOption
	outcome wizard.Outcome
}

func (s *stubWizard) run(ws string, scope tools.Scope, cfg *config.Config, options []wizard.SourceOption) (wizard.Outcome, error) {
	s.called = true
	s.cfg = cfg
	s.options = options
	return s.outcome, nil
}

// failWizard is a wizardRunner that fails the test when invoked.
func failWizard(t *testing.T) wizardRunner {
	return func(string, tools.Scope, *config.Config, []wizard.SourceOption) (wizard.Outcome, error) {
		t.Fatal("wizard runner must not be called")
		return wizard.Outcome{}, nil
	}
}

// seedWorkspaceFile writes content at <ws>/<relPath>, creating parent dirs.
func seedWorkspaceFile(t *testing.T, ws, relPath, content string) {
	t.Helper()
	full := filepath.Join(ws, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertEnabledExactly fails unless the persisted config enables exactly the
// given tool names.
func assertEnabledExactly(t *testing.T, ws string, enabled ...string) {
	t.Helper()
	cfg, err := config.Load(ws, tools.Names())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	want := map[string]bool{}
	for _, name := range enabled {
		want[name] = true
	}
	for _, name := range tools.Names() {
		if cfg.IsEnabled(name) != want[name] {
			t.Errorf("tool %q enabled=%v, want %v", name, cfg.IsEnabled(name), want[name])
		}
	}
}

// seedInitializedWorkspace creates a populated .agentsync/ under ws so the
// re-init guard sees an initialized scope with deletable contents.
func seedInitializedWorkspace(t *testing.T, ws string) {
	t.Helper()
	for _, rel := range []string{
		".agentsync/rules/style.md",
		".agentsync/.state/snapshot.json",
	} {
		full := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("seed"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReinitGuardNotInitializedProceedsSilently(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader(""), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !proceed {
		t.Errorf("uninitialized workspace must proceed")
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
}

func TestReinitGuardTtyDeclineAborts(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader("n\n"), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if proceed {
		t.Errorf("decline must not proceed")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); err != nil {
		t.Errorf("contents must remain intact, stat: %v", err)
	}
}

func TestReinitGuardTtyDefaultNoOnEmpty(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader("\n"), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if proceed {
		t.Errorf("empty answer must default to No")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); err != nil {
		t.Errorf("contents must remain intact, stat: %v", err)
	}
}

func TestReinitGuardTtyConfirmWipes(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader("y\n"), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !proceed {
		t.Errorf("confirm must proceed")
	}
	for _, rel := range []string{
		".agentsync/.state/snapshot.json",
		".agentsync/rules/style.md",
	} {
		if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Errorf("%s should be wiped, stat err=%v", rel, err)
		}
	}
}

func TestReinitGuardNonTtyWithoutForceErrors(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader(""), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error %q should mention --force", err.Error())
	}
	if proceed {
		t.Errorf("error case must not proceed")
	}
}

func TestInitFreshUsesDetectionEnablement(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".cursor/rules/x.mdc", "---\ndescription: x\n---\nBody.\n")
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "", false, failWizard(t), strings.NewReader(""), &out, false)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertEnabledExactly(t, ws, "Cursor")
}

func TestInitFreshZeroDetectedAllDisabled(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "", false, failWizard(t), strings.NewReader(""), &out, false)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	assertEnabledExactly(t, ws)
}

func TestInitFromClaudeHeadless(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".claude/CLAUDE.md", "# Project memory\n\nRoot memory body.\n")
	seedWorkspaceFile(t, ws, ".claude/skills/foo/SKILL.md", "---\nname: foo\ndescription: Foo skill\n---\n\nSkill body.\n")
	seedWorkspaceFile(t, ws, ".claude/agents/a.md", "---\nname: a\ndescription: Agent a\n---\n\nAgent body.\n")
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "claude", false, failWizard(t), strings.NewReader(""), &out, false)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	loaded, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("canonical.Load: %v", err)
	}
	if !strings.Contains(loaded.AgentsMD, "Root memory body.") {
		t.Errorf("AGENTS.md not imported, got %q", loaded.AgentsMD)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Dir != "foo" {
		t.Errorf("skills = %+v, want one skill dir %q", loaded.Skills, "foo")
	}
	if len(loaded.Agents) != 1 || loaded.Agents[0].Filename != "a" {
		t.Errorf("agents = %+v, want one agent %q", loaded.Agents, "a")
	}
	if !strings.Contains(out.String(), "imported") {
		t.Errorf("out should contain import summary, got %q", out.String())
	}
	if !strings.Contains(out.String(), "Run 'agentsync'") {
		t.Errorf("out should contain project-scope run hint, got %q", out.String())
	}
	assertEnabledExactly(t, ws, "Claude Code")
}

func TestInitFromUnknownToolKey(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "windsurf", false, failWizard(t), strings.NewReader(""), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	for _, key := range []string{"claude", "cursor", "opencode"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q should list eligible key %q", err.Error(), key)
		}
	}
	if strings.Contains(err.Error(), "zed") {
		t.Errorf("error %q must not list ineligible key zed", err.Error())
	}
}

func TestInitFromIneligibleTool(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".rules", "# Zed rules\n")
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "zed", false, failWizard(t), strings.NewReader(""), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no importable concepts") {
		t.Errorf("error %q should explain no importable concepts", err.Error())
	}
}

func TestInitFromUndetectedTool(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "claude", false, failWizard(t), strings.NewReader(""), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "nothing to import: claude not found at project scope") {
		t.Errorf("error %q should be the nothing-to-import message", err.Error())
	}
}

func TestInitFromOnInitializedNonTtyWithoutForceErrors(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	seedWorkspaceFile(t, ws, ".claude/CLAUDE.md", "# Memory\n")
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "claude", false, failWizard(t), strings.NewReader(""), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error %q should mention --force", err.Error())
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); err != nil {
		t.Errorf("contents must remain intact, stat: %v", err)
	}
}

func TestInitTtyAbortedWizardLeavesNothing(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".claude/CLAUDE.md", "# Memory\n")
	stub := &stubWizard{outcome: wizard.Outcome{Aborted: true}}
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "", false, stub.run, strings.NewReader(""), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !stub.called {
		t.Fatal("wizard runner should have been called")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync")); !os.IsNotExist(err) {
		t.Errorf("aborted wizard must leave no .agentsync/, stat err=%v", err)
	}
	if stub.cfg == nil || !stub.cfg.IsEnabled("Claude Code") {
		t.Errorf("stub cfg should enable Claude Code, got %+v", stub.cfg)
	}
	if len(stub.options) == 0 {
		t.Errorf("stub should receive non-empty options")
	}
}

func TestInitTtyZeroOptionsAutoFresh(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".rules", "# Zed rules\n")
	var out bytes.Buffer

	err := runInitFlow(ws, tools.ScopeProject, "", false, failWizard(t), strings.NewReader(""), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "AGENTS.md")); err != nil {
		t.Errorf("auto-fresh should scaffold .agentsync/, stat: %v", err)
	}
	assertEnabledExactly(t, ws, "Zed")
}

func TestRootUninitializedNonTtyErrors(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	err := runRootFlow(ws, tools.ScopeProject, failWizard(t), &out, false)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "agentsync init") {
		t.Errorf("error %q should mention agentsync init", err.Error())
	}
}

func TestRootUninitializedTtyZeroDetectedAutoFresh(t *testing.T) {
	ws := t.TempDir()
	var out bytes.Buffer

	err := runRootFlow(ws, tools.ScopeProject, failWizard(t), &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "AGENTS.md")); err != nil {
		t.Errorf("auto-fresh should scaffold .agentsync/, stat: %v", err)
	}
	assertEnabledExactly(t, ws)
	if !strings.Contains(out.String(), "Initialized .agentsync/") {
		t.Errorf("out should contain success message, got %q", out.String())
	}
	if !strings.Contains(out.String(), "run 'agentsync") {
		t.Errorf("out should contain run hint, got %q", out.String())
	}
}

func TestRootUninitializedTtyDetectedRunsWizard(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".claude/CLAUDE.md", "# Memory\n")
	stub := &stubWizard{outcome: wizard.Outcome{Imported: true, ToolName: "Claude Code"}}
	var out bytes.Buffer

	err := runRootFlow(ws, tools.ScopeProject, stub.run, &out, true)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !stub.called {
		t.Fatal("wizard runner should have been called")
	}
	if len(stub.options) == 0 {
		t.Errorf("stub should receive non-empty options")
	}
}

func TestReinitGuardForceWipesWithoutPrompt(t *testing.T) {
	ws := t.TempDir()
	seedInitializedWorkspace(t, ws)
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, true, strings.NewReader(""), &out, false)

	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !proceed {
		t.Errorf("--force must proceed")
	}
	if strings.Contains(out.String(), "Continue?") {
		t.Errorf("--force must not prompt, got %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); !os.IsNotExist(err) {
		t.Errorf("contents should be wiped, stat err=%v", err)
	}
}

func TestInitFromInvalidNeverWipesExisting(t *testing.T) {
	cases := []struct {
		name    string
		fromKey string
		seed    func(t *testing.T, ws string)
	}{
		{name: "unknown key", fromKey: "windsurf", seed: func(*testing.T, string) {}},
		{name: "ineligible tool", fromKey: "zed", seed: func(t *testing.T, ws string) {
			seedWorkspaceFile(t, ws, ".rules", "rules\n")
		}},
		{name: "undetected tool", fromKey: "claude", seed: func(*testing.T, string) {}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			seedInitializedWorkspace(t, ws)
			tc.seed(t, ws)
			var out bytes.Buffer

			err := runInitFlow(ws, tools.ScopeProject, tc.fromKey, true, failWizard(t), strings.NewReader(""), &out, false)

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if _, statErr := os.Stat(filepath.Join(ws, ".agentsync", "rules", "style.md")); statErr != nil {
				t.Errorf("--from validation must run before the wipe; canonical lost: %v", statErr)
			}
		})
	}
}

func TestReinitGuardPlainFileErrors(t *testing.T) {
	ws := t.TempDir()
	seedWorkspaceFile(t, ws, ".agentsync", "not a dir\n")
	var out bytes.Buffer

	proceed, err := reinitGuard(ws, false, strings.NewReader(""), &out, true)

	if proceed || err == nil {
		t.Fatalf("want (false, error) for plain-file .agentsync, got (%v, %v)", proceed, err)
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error %q should explain .agentsync is not a directory", err.Error())
	}
}

func TestInitFromClaudeUserScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seedWorkspaceFile(t, home, ".claude/CLAUDE.md", "# user memory\n")
	var out bytes.Buffer

	err := runInitFlow(home, tools.ScopeUser, "claude", false, failWizard(t), strings.NewReader(""), &out, false)

	if err != nil {
		t.Fatalf("runInitFlow: %v", err)
	}
	if !strings.Contains(out.String(), "agentsync --global") {
		t.Errorf("user-scope hint must mention 'agentsync --global', got: %s", out.String())
	}
	c, err := canonical.Load(home)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.AgentsMD != "# user memory\n" {
		t.Errorf("AgentsMD = %q, want imported user memory", c.AgentsMD)
	}
	cfg, err := config.Load(home, tools.Names())
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if !cfg.IsEnabled("Claude Code") {
		t.Error("Claude Code should be enabled after user-scope import")
	}
}
