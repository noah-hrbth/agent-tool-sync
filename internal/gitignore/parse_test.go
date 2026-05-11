package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readGitignore(t *testing.T, ws string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	return string(b)
}

func expectedBlock(entries ...string) string {
	var b strings.Builder
	b.WriteString(BeginMarker + "\n")
	for _, e := range entries {
		b.WriteString(e + "\n")
	}
	b.WriteString(EndMarker + "\n")
	return b.String()
}

func TestUpdateCreatesGitignoreIfMissing(t *testing.T) {
	ws := t.TempDir()
	if err := Update(ws, []string{".claude/", "CLAUDE.md"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	want := expectedBlock(".claude/", "CLAUDE.md")
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

func TestUpdateReplacesExistingBlockInPlace(t *testing.T) {
	ws := t.TempDir()
	initial := "node_modules/\n" + expectedBlock(".old/") + "*.log\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Update(ws, []string{".new/"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	if !strings.Contains(got, "node_modules/\n") {
		t.Errorf("user content before block lost: %q", got)
	}
	if !strings.Contains(got, "*.log\n") {
		t.Errorf("user content after block lost: %q", got)
	}
	if strings.Contains(got, ".old/") {
		t.Errorf("old block entries still present: %q", got)
	}
	if !strings.Contains(got, ".new/\n") {
		t.Errorf("new block entries missing: %q", got)
	}
	if strings.Count(got, BeginMarker) != 1 {
		t.Errorf("expected exactly one BEGIN marker, got %d in %q", strings.Count(got, BeginMarker), got)
	}
}

func TestUpdatePreservesUserContentBeforeAndAfterBlock(t *testing.T) {
	ws := t.TempDir()
	initial := "# user header\nnode_modules/\nfoo/\n\n# tail comment\n*.bak\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Update(ws, []string{".claude/"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	for _, want := range []string{"# user header\n", "node_modules/\n", "foo/\n", "# tail comment\n", "*.bak\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing user content %q in %q", want, got)
		}
	}
	if !strings.Contains(got, expectedBlock(".claude/")) {
		t.Errorf("missing managed block in %q", got)
	}
}

func TestUpdateIsIdempotent(t *testing.T) {
	ws := t.TempDir()
	entries := []string{".claude/", ".cursor/", "CLAUDE.md"}
	if err := Update(ws, entries); err != nil {
		t.Fatalf("Update 1: %v", err)
	}
	first := readGitignore(t, ws)
	if err := Update(ws, entries); err != nil {
		t.Fatalf("Update 2: %v", err)
	}
	second := readGitignore(t, ws)
	if first != second {
		t.Fatalf("not idempotent:\nfirst:  %q\nsecond: %q", first, second)
	}
}

func TestUpdateHandlesFileWithoutTrailingNewline(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte("node_modules/"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Update(ws, []string{".claude/"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	if !strings.Contains(got, "node_modules/\n") {
		t.Errorf("trailing-newline normalization dropped content: %q", got)
	}
	if !strings.Contains(got, expectedBlock(".claude/")) {
		t.Errorf("missing managed block in %q", got)
	}
}

func TestUpdateCollapsesDuplicateBlocksToOne(t *testing.T) {
	ws := t.TempDir()
	initial := expectedBlock(".a/") + "\n" + expectedBlock(".b/") + "\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Update(ws, []string{".final/"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	if strings.Count(got, BeginMarker) != 1 {
		t.Fatalf("expected exactly one BEGIN marker, got %d in %q", strings.Count(got, BeginMarker), got)
	}
	if strings.Count(got, EndMarker) != 1 {
		t.Fatalf("expected exactly one END marker, got %d in %q", strings.Count(got, EndMarker), got)
	}
	if !strings.Contains(got, ".final/\n") {
		t.Errorf("new entries missing: %q", got)
	}
}

func TestUpdateInsertsBlankLineSeparatorsWhenAdjacentToUserContent(t *testing.T) {
	ws := t.TempDir()
	initial := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Update(ws, []string{".claude/"}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := readGitignore(t, ws)
	if !strings.Contains(got, "\n\n"+BeginMarker) {
		t.Errorf("expected blank line before BEGIN marker: %q", got)
	}
}

func TestRemoveDeletesOnlyManagedBlock(t *testing.T) {
	ws := t.TempDir()
	initial := "node_modules/\n\n" + expectedBlock(".claude/") + "\n*.log\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Remove(ws); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got := readGitignore(t, ws)
	if strings.Contains(got, BeginMarker) || strings.Contains(got, EndMarker) {
		t.Errorf("block still present: %q", got)
	}
	if !strings.Contains(got, "node_modules/\n") {
		t.Errorf("pre-block content lost: %q", got)
	}
	if !strings.Contains(got, "*.log\n") {
		t.Errorf("post-block content lost: %q", got)
	}
}

func TestRemoveIsNoopWhenNoBlock(t *testing.T) {
	ws := t.TempDir()
	initial := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(ws, ".gitignore"), []byte(initial), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := Remove(ws); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got := readGitignore(t, ws)
	if got != initial {
		t.Fatalf("file should be unchanged.\ngot:  %q\nwant: %q", got, initial)
	}
}

func TestRemoveIsNoopWhenFileMissing(t *testing.T) {
	ws := t.TempDir()
	if err := Remove(ws); err != nil {
		t.Fatalf("Remove on missing file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("expected .gitignore to remain absent, got err=%v", err)
	}
}
