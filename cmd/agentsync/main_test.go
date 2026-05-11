package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/gitignore"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestApplyGitignoreFlowSkipsAtUserScope(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeUser, strings.NewReader("y\n"), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Errorf("user scope should never mutate config")
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("user scope should not create .gitignore (err=%v)", err)
	}
}

func TestApplyGitignoreFlowSkipsWhenAlreadyPromptedAndManageFalse(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	cfg.Gitignore = config.GitignoreConfig{Manage: false, Prompted: true}
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader(""), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Errorf("Manage=false Prompted=true must not mutate config")
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf("Manage=false should not create .gitignore (err=%v)", err)
	}
}

func TestApplyGitignoreFlowRefreshesBlockWhenAlreadyPromptedAndManageTrue(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	cfg.Gitignore = config.GitignoreConfig{Manage: true, Prompted: true}
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader(""), &out, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Errorf("refresh should not mutate config (already set)")
	}
	got := mustReadFile(t, filepath.Join(ws, ".gitignore"))
	if !strings.Contains(got, gitignore.BeginMarker) {
		t.Errorf("expected managed block, got %q", got)
	}
	if !strings.Contains(got, ".claude/") {
		t.Errorf("expected .claude/ entry, got %q", got)
	}
}

func TestApplyGitignoreFlowOnFirstRunTtyApplyWritesBlock(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader("y\n"), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Errorf("expected config to be marked changed")
	}
	if !cfg.Gitignore.Manage || !cfg.Gitignore.Prompted {
		t.Errorf("expected Manage=true Prompted=true, got %+v", cfg.Gitignore)
	}
	got := mustReadFile(t, filepath.Join(ws, ".gitignore"))
	if !strings.Contains(got, gitignore.BeginMarker) {
		t.Errorf("expected managed block, got %q", got)
	}
}

func TestApplyGitignoreFlowOnFirstRunTtySkipRemovesBlock(t *testing.T) {
	ws := t.TempDir()
	// Pre-seed an existing managed block.
	if err := gitignore.Update(ws, []string{".old/"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader("n\n"), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Errorf("expected config to be marked changed")
	}
	if cfg.Gitignore.Manage {
		t.Errorf("expected Manage=false")
	}
	if !cfg.Gitignore.Prompted {
		t.Errorf("expected Prompted=true")
	}
	got := mustReadFile(t, filepath.Join(ws, ".gitignore"))
	if strings.Contains(got, gitignore.BeginMarker) {
		t.Errorf("block should have been removed, got %q", got)
	}
}

func TestApplyGitignoreFlowOnFirstRunTtyRejectsInvalidThenAcceptsY(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader("q\ny\n"), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !changed {
		t.Errorf("expected config to be marked changed after eventual y")
	}
	if !cfg.Gitignore.Manage {
		t.Errorf("expected Manage=true")
	}
	// Output should contain the prompt at least twice (after rejection it re-prompts).
	if strings.Count(out.String(), "[y/n]") < 2 {
		t.Errorf("expected re-prompt, got: %q", out.String())
	}
}

func TestApplyGitignoreFlowOnFirstRunTtyGivesUpAfterThreeBadAnswers(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader("q\nq\nq\n"), &out, true)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Errorf("3 invalid answers should not mutate config")
	}
	if cfg.Gitignore.Prompted {
		t.Errorf("Prompted should remain false after giving up")
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore should not have been written, err=%v", err)
	}
}

func TestApplyGitignoreFlowOnFirstRunNonTtyLogsHintAndLeavesPromptedFalse(t *testing.T) {
	ws := t.TempDir()
	cfg := config.Default(tools.Names())
	var out bytes.Buffer
	changed, err := applyGitignoreFlowCLI(ws, cfg, tools.All(), tools.ScopeProject, strings.NewReader(""), &out, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if changed {
		t.Errorf("non-tty must not mutate config")
	}
	if cfg.Gitignore.Prompted {
		t.Errorf("Prompted should remain false in non-tty mode")
	}
	if out.Len() == 0 {
		t.Errorf("expected hint to be logged to out")
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); !os.IsNotExist(err) {
		t.Errorf(".gitignore should not have been written in non-tty mode (err=%v)", err)
	}
}
