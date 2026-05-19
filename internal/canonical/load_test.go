package canonical

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadRejectsSymlinkedAgentsMD proves the read-side escape is blocked: a
// repo-supplied .agentsync/AGENTS.md symlinked to a file outside the workspace
// must not have its content slurped into Canonical (and thence into rendered,
// committable tool files).
func TestLoadRejectsSymlinkedAgentsMD(t *testing.T) {
	// Arrange
	ws, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	secret := filepath.Join(outside, "id_rsa")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(ws, ".agentsync", "AGENTS.md")); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = Load(ws)

	// Assert: Load must fail rather than disclose the outside file
	if err == nil {
		t.Fatal("Load followed a symlinked AGENTS.md instead of rejecting it")
	}
}

// TestLoadRejectsSymlinkedSkillsDir covers a planted symlinked subdirectory.
func TestLoadRejectsSymlinkedSkillsDir(t *testing.T) {
	// Arrange
	ws, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(ws, ".agentsync"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(ws, ".agentsync", "skills")); err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = Load(ws)

	// Assert
	if err == nil {
		t.Fatal("Load enumerated a symlinked skills/ dir instead of rejecting it")
	}
}
