package syncer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// realWorkspace mirrors the resolveBase contract: the workspace handed to the
// syncer is already realpath-resolved (macOS t.TempDir() lives under a /var
// symlink), so safepath only forbids symlink crossings strictly below it.
func realWorkspace(t *testing.T, scenario string) string {
	t.Helper()
	ws, err := filepath.EvalSymlinks(copyScenario(t, scenario))
	if err != nil {
		t.Fatalf("eval workspace: %v", err)
	}
	return ws
}

func TestRunSyncDoesNotFollowSymlink(t *testing.T) {
	// Arrange: a pre-planted root CLAUDE.md -> outside sentinel
	ws := realWorkspace(t, "empty-workspace")
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "authorized_keys")
	if err := os.WriteFile(sentinel, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(ws, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Act
	result, err := syncer.RunSync(ws, c, adapters, cfg, tools.ScopeProject, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Assert: symlink not followed, sentinel untouched
	data, err := os.ReadFile(sentinel)
	if err != nil || string(data) != "original" {
		t.Fatalf("sentinel was modified through symlink: data=%q err=%v", data, err)
	}
	if !hasErrMentioning(result.Errors, "CLAUDE.md") {
		t.Fatalf("expected an aggregated error for CLAUDE.md, got %v", result.Errors)
	}
	// Other non-symlinked outputs still written (loop continues past the error)
	if _, err := os.Stat(filepath.Join(ws, "AGENTS.md")); err != nil {
		t.Fatalf("expected AGENTS.md to still be written: %v", err)
	}
}

func TestRunSyncRejectsSymlinkedAncestorDir(t *testing.T) {
	// Arrange: ws/.cursor -> outside dir so a followed MkdirAll would escape
	ws := realWorkspace(t, "empty-workspace")
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(ws, ".cursor")); err != nil {
		t.Fatal(err)
	}

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Act
	result, err := syncer.RunSync(ws, c, adapters, cfg, tools.ScopeProject, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Assert: write rejected, nothing created inside the outside dir
	if !hasErrMentioning(result.Errors, ".cursor") {
		t.Fatalf("expected an aggregated error for .cursor path, got %v", result.Errors)
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) > 0 {
		t.Fatalf("files were written through the symlinked ancestor: %v", entries)
	}
}

func TestOrphanCleanupDoesNotFollowSymlink(t *testing.T) {
	// Arrange: a snapshot entry whose path is a symlink to an outside file
	ws := realWorkspace(t, "empty-workspace")
	outside := t.TempDir()
	sentinel := filepath.Join(outside, "victim")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(ws, "orphan-link")); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(ws, ".agentsync", ".state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// "orphan-link" is in no adapter's render set, so cleanup will consider it
	snap := `{"files":{"orphan-link":"deadbeef"}}`
	if err := os.WriteFile(filepath.Join(stateDir, "snapshot.json"), []byte(snap), 0o644); err != nil {
		t.Fatal(err)
	}

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Act
	result, err := syncer.RunSync(ws, c, adapters, cfg, tools.ScopeProject, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Assert: outside file not removed; the unsafe orphan surfaced as an error
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("outside file was deleted through the symlinked orphan: %v", err)
	}
	if string(mustRead(t, sentinel)) != "keep me" {
		t.Fatal("outside file content changed")
	}
	if !hasErrMentioning(result.Errors, "orphan-link") {
		t.Fatalf("expected an aggregated error for orphan-link, got %v", result.Errors)
	}
}

func hasErrMentioning(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
