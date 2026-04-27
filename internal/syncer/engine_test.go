package syncer_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/config"
	"github.com/noah-hrbth/agentsync/internal/syncer"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func copyScenario(t *testing.T, scenario string) string {
	t.Helper()
	src := filepath.Join("..", "..", "testdata", "scenarios", scenario, "input")
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copy scenario %s: %v", scenario, err)
	}
	return dst
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func TestEmptyWorkspaceSync(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, err := canonical.Load(ws)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.AgentsMD == "" {
		t.Fatal("expected rules to be loaded")
	}

	adapters := tools.All()
	cfg := config.Default(tools.Names())

	result, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.Written) == 0 {
		t.Fatal("expected files to be written")
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Verify key output files exist and contain the canonical rules
	checks := []string{
		// Rules
		".claude/CLAUDE.md",
		".codex/AGENTS.md",
		".opencode/AGENTS.md",
		".gemini/GEMINI.md",
		".cursor/rules/general.mdc",
		// Gemini new concepts
		".gemini/skills/code-reviewer/SKILL.md",
		".gemini/agents/debugger.md",
		".gemini/commands/summarize.toml",
		// Codex new concepts
		".agents/skills/code-reviewer/SKILL.md",
		".codex/agents/debugger.toml",
	}
	for _, rel := range checks {
		data, err := os.ReadFile(filepath.Join(ws, rel))
		if err != nil {
			t.Errorf("expected file %s: %v", rel, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("file %s is empty", rel)
		}
	}

	// Claude commands are deprecated and must not be rendered.
	claudeCmdPath := filepath.Join(ws, ".claude", "commands")
	if _, err := os.Stat(claudeCmdPath); err == nil {
		entries, _ := os.ReadDir(claudeCmdPath)
		if len(entries) > 0 {
			t.Errorf("expected .claude/commands/ to be empty or absent, got %d file(s)", len(entries))
		}
	}
}

func TestStatusAfterSync(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Before sync: all files should be StatusNew
	results, err := syncer.Status(ws, c, adapters, cfg)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	for _, r := range results {
		if r.Status != syncer.StatusNew {
			t.Errorf("expected StatusNew for %s, got %v", r.Path, r.Status)
		}
	}

	// After sync: all files should be StatusSynced
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	results, err = syncer.Status(ws, c, adapters, cfg)
	if err != nil {
		t.Fatalf("status after sync: %v", err)
	}
	for _, r := range results {
		if r.Status != syncer.StatusSynced {
			t.Errorf("expected StatusSynced for %s, got %v", r.Path, r.Status)
		}
	}
}

func TestDivergenceDetection(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Sync once to establish snapshot
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// Externally edit .claude/CLAUDE.md
	claudePath := filepath.Join(ws, ".claude", "CLAUDE.md")
	if err := os.WriteFile(claudePath, []byte("# Externally edited\n"), 0o644); err != nil {
		t.Fatalf("external edit: %v", err)
	}

	// Status should now report divergence for that file
	results, err := syncer.Status(ws, c, adapters, cfg)
	if err != nil {
		t.Fatalf("status: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Path == ".claude/CLAUDE.md" && r.Status == syncer.StatusDivergent {
			found = true
		}
	}
	if !found {
		t.Error("expected .claude/CLAUDE.md to be StatusDivergent after external edit")
	}
}

func TestRunSyncRespectsSkip(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Initial sync to establish snapshot
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	originalContent, err := os.ReadFile(filepath.Join(ws, ".claude", "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	// Change canonical rules
	if err := canonical.SaveAgentsMD(ws, "# Modified rules\n"); err != nil {
		t.Fatalf("save rules: %v", err)
	}
	c, _ = canonical.Load(ws)

	// Sync with .claude/CLAUDE.md in skip
	result, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{Skip: map[string]bool{".claude/CLAUDE.md": true}})
	if err != nil {
		t.Fatalf("skip sync: %v", err)
	}

	found := false
	for _, f := range result.Skipped {
		if f.Path == ".claude/CLAUDE.md" {
			found = true
		}
	}
	if !found {
		t.Error("expected .claude/CLAUDE.md in result.Skipped")
	}

	afterContent, _ := os.ReadFile(filepath.Join(ws, ".claude", "CLAUDE.md"))
	if string(afterContent) != string(originalContent) {
		t.Error("expected .claude/CLAUDE.md content unchanged after skip")
	}
}

func TestPartialInstall(t *testing.T) {
	ws := copyScenario(t, "partial-install")

	// Only create .claude and .cursor dirs to simulate partial installation
	for _, dir := range []string{".claude", ".cursor"} {
		if err := os.MkdirAll(filepath.Join(ws, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	c, _ := canonical.Load(ws)
	cfg := config.Default(tools.Names())

	// Enable only Claude Code and Cursor
	for name := range cfg.Tools {
		cfg.Tools[name] = config.ToolConfig{Enabled: name == "Claude Code" || name == "Cursor"}
	}

	result, err := syncer.RunSync(ws, c, tools.All(), cfg, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Should not have written OpenCode or Gemini files
	for _, f := range result.Written {
		if strings.HasPrefix(f.Path, ".opencode") {
			t.Errorf("wrote opencode file despite being disabled: %s", f.Path)
		}
	}
}

func readSnapshot(t *testing.T, ws string) map[string]interface{} {
	t.Helper()
	path := filepath.Join(ws, ".agentsync", ".state", "snapshot.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	return m
}

func writeSnapshot(t *testing.T, ws string, snap map[string]interface{}) {
	t.Helper()
	path := filepath.Join(ws, ".agentsync", ".state", "snapshot.json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}
}

func hashBytesHex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestOrphanCleanupSafeDelete(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Initial sync to establish snapshot
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// Plant a stale command file (simulates pre-deprecation state)
	orphanRel := ".claude/commands/leftover.md"
	orphanAbs := filepath.Join(ws, orphanRel)
	if err := os.MkdirAll(filepath.Dir(orphanAbs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	orphanContent := []byte("# stale command\n")
	if err := os.WriteFile(orphanAbs, orphanContent, 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}

	// Add matching entry to snapshot
	snap := readSnapshot(t, ws)
	if snap["files"] == nil {
		snap["files"] = map[string]interface{}{}
	}
	snap["files"].(map[string]interface{})[orphanRel] = hashBytesHex(orphanContent)
	writeSnapshot(t, ws, snap)

	// Re-sync: orphan should be deleted (hash matches snapshot)
	result, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if _, statErr := os.Stat(orphanAbs); !os.IsNotExist(statErr) {
		t.Error("expected orphan file to be deleted")
	}
}

func TestOrphanCleanupDivergentPreserved(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Initial sync
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// Plant orphan with edited on-disk content but stale snapshot hash
	orphanRel := ".claude/commands/leftover.md"
	orphanAbs := filepath.Join(ws, orphanRel)
	if err := os.MkdirAll(filepath.Dir(orphanAbs), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	staleContent := []byte("# original content\n")
	editedContent := []byte("# user-edited content\n")
	if err := os.WriteFile(orphanAbs, editedContent, 0o644); err != nil {
		t.Fatalf("write orphan: %v", err)
	}

	// Snapshot records the stale hash (before user edit)
	snap := readSnapshot(t, ws)
	if snap["files"] == nil {
		snap["files"] = map[string]interface{}{}
	}
	snap["files"].(map[string]interface{})[orphanRel] = hashBytesHex(staleContent)
	writeSnapshot(t, ws, snap)

	// Re-sync: orphan should be preserved (hash mismatch = user edits)
	result, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if _, statErr := os.Stat(orphanAbs); statErr != nil {
		t.Errorf("expected divergent orphan to be preserved: %v", statErr)
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, orphanRel) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for divergent orphan %s, warnings: %v", orphanRel, result.Warnings)
	}
}

func TestDisabledToolFilesNotDeleted(t *testing.T) {
	ws := copyScenario(t, "empty-workspace")

	c, _ := canonical.Load(ws)
	adapters := tools.All()
	cfg := config.Default(tools.Names())

	// Initial sync with all tools enabled
	if _, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{}); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// Verify OpenCode's AGENTS.md was written
	opencodePath := filepath.Join(ws, ".opencode", "AGENTS.md")
	if _, err := os.Stat(opencodePath); err != nil {
		t.Fatalf("expected .opencode/AGENTS.md after initial sync: %v", err)
	}

	// Disable OpenCode and sync again
	for name := range cfg.Tools {
		if name == "OpenCode" {
			cfg.Tools[name] = config.ToolConfig{Enabled: false}
		}
	}

	result, err := syncer.RunSync(ws, c, adapters, cfg, syncer.SyncOptions{})
	if err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// .opencode/AGENTS.md must still exist — disabling a tool must not delete its files
	if _, err := os.Stat(opencodePath); err != nil {
		t.Errorf(".opencode/AGENTS.md was deleted after disabling OpenCode — should be preserved: %v", err)
	}
}
