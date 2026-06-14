package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/gitignore"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func TestRequireInitialized(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, ws string)
		scope     tools.Scope
		wantErr   bool
		errSubstr string
	}{
		{
			name:      "missing dir project scope errors with init hint",
			setup:     func(t *testing.T, ws string) {},
			scope:     tools.ScopeProject,
			wantErr:   true,
			errSubstr: "agentsync init",
		},
		{
			name:      "missing dir user scope errors with global init hint",
			setup:     func(t *testing.T, ws string) {},
			scope:     tools.ScopeUser,
			wantErr:   true,
			errSubstr: "agentsync init --global",
		},
		{
			name: "existing dir passes",
			setup: func(t *testing.T, ws string) {
				if err := os.Mkdir(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			scope:   tools.ScopeProject,
			wantErr: false,
		},
		{
			name: "plain file at .agentsync errors",
			setup: func(t *testing.T, ws string) {
				if err := os.WriteFile(filepath.Join(ws, ".agentsync"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			scope:   tools.ScopeProject,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := t.TempDir()
			tt.setup(t, ws)

			err := requireInitialized(ws, tt.scope)

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("expected nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q missing %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

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

func TestIsTerminalRejectsNonTtyFiles(t *testing.T) {
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer devNull.Close()
	if isTerminal(devNull) {
		t.Errorf("isTerminal(%s) = true, want false", os.DevNull)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminal(r) {
		t.Error("isTerminal(pipe) = true, want false")
	}
}

func TestRequireInitializedDistinguishesMissingFromPlainFile(t *testing.T) {
	missing := t.TempDir()
	if !errors.Is(requireInitialized(missing, tools.ScopeProject), errNotInitialized) {
		t.Error("missing .agentsync/ must report errNotInitialized")
	}

	plainFile := t.TempDir()
	if err := os.WriteFile(filepath.Join(plainFile, ".agentsync"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := requireInitialized(plainFile, tools.ScopeProject)
	if err == nil || errors.Is(err, errNotInitialized) {
		t.Errorf("plain-file .agentsync must error without errNotInitialized, got %v", err)
	}
}
