package safepath_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/safepath"
)

// realTempDir mirrors the resolveBase contract: the root handed to safepath is
// already realpath-resolved. On macOS t.TempDir() lives under /var (a symlink
// to /private/var), so an unresolved root would make every "ok" case fail.
func realTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("eval temp dir: %v", err)
	}
	return dir
}

func TestResolve(t *testing.T) {
	root := realTempDir(t)

	// existing real ancestor dir + a symlinked ancestor + a symlinked file
	if err := os.MkdirAll(filepath.Join(root, "real", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := realTempDir(t)
	if err := os.Symlink(outside, filepath.Join(root, "linkdir")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "secret"), filepath.Join(root, "linkfile")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		rel      string
		wantErr  bool
		wantPath string // checked only when wantErr is false
		isUnsafe bool   // expect errors.Is(err, ErrUnsafe)
	}{
		{name: "local path ok", rel: "a/b/c.md", wantPath: filepath.Join(root, "a/b/c.md")},
		{name: "existing real ancestor ok", rel: "real/nested/x.md", wantPath: filepath.Join(root, "real/nested/x.md")},
		{name: "deep non-existent ok", rel: "new/deep/file.md", wantPath: filepath.Join(root, "new/deep/file.md")},
		{name: "dot resolves to root", rel: ".", wantPath: root},
		{name: "parent traversal rejected", rel: "../escape", wantErr: true, isUnsafe: true},
		{name: "embedded traversal rejected", rel: "a/../../escape", wantErr: true, isUnsafe: true},
		{name: "absolute rejected", rel: "/etc/passwd", wantErr: true, isUnsafe: true},
		{name: "empty rejected", rel: "", wantErr: true, isUnsafe: true},
		{name: "symlinked ancestor rejected", rel: "linkdir/x.md", wantErr: true, isUnsafe: true},
		{name: "symlinked final target rejected", rel: "linkfile", wantErr: true, isUnsafe: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := safepath.Resolve(root, tc.rel)

			// Assert
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Resolve(%q) = %q, want error", tc.rel, got)
				}
				if tc.isUnsafe && !errors.Is(err, safepath.ErrUnsafe) {
					t.Fatalf("Resolve(%q) err = %v, want errors.Is ErrUnsafe", tc.rel, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q) unexpected error: %v", tc.rel, err)
			}
			if got != tc.wantPath {
				t.Fatalf("Resolve(%q) = %q, want %q", tc.rel, got, tc.wantPath)
			}
		})
	}
}

func TestResolveRootReachedViaSymlinkAllowed(t *testing.T) {
	// Arrange: linkRoot -> realRoot, caller resolves it (as resolveBase does)
	parent := realTempDir(t)
	realRoot := filepath.Join(parent, "realroot")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realRoot, filepath.Join(parent, "linkroot")); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(filepath.Join(parent, "linkroot"))
	if err != nil {
		t.Fatal(err)
	}

	// Act
	got, err := safepath.Resolve(root, "a.md")

	// Assert
	if err != nil {
		t.Fatalf("Resolve under symlink-reached root: %v", err)
	}
	if got != filepath.Join(realRoot, "a.md") {
		t.Fatalf("got %q, want %q", got, filepath.Join(realRoot, "a.md"))
	}
}

func TestWriteFileRejectsSymlinkedAncestor(t *testing.T) {
	// Arrange: plant root/.claude -> evil so a follow would write outside root
	root := realTempDir(t)
	evil := realTempDir(t)
	if err := os.Symlink(evil, filepath.Join(root, ".claude")); err != nil {
		t.Fatal(err)
	}

	// Act
	err := safepath.WriteFile(root, filepath.Join(".claude", "x.md"), []byte("pwned"), 0o644)

	// Assert
	if err == nil || !errors.Is(err, safepath.ErrUnsafe) {
		t.Fatalf("WriteFile through symlinked ancestor err = %v, want ErrUnsafe", err)
	}
	if _, statErr := os.Stat(filepath.Join(evil, "x.md")); statErr == nil {
		t.Fatal("file was written through the symlink into the outside directory")
	}
}
