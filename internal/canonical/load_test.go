package canonical

import (
	"os"
	"path/filepath"
	"testing"
)

// mustWriteFile writes content at path, creating parent dirs. Test helper.
func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// skillDocByRelPath returns the doc with the given RelPath, or nil.
func skillDocByRelPath(s *Skill, relPath string) *SkillDoc {
	for i := range s.Docs {
		if s.Docs[i].RelPath == relPath {
			return &s.Docs[i]
		}
	}
	return nil
}

func TestLoadSkillReadsSiblingDocs(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")
	mustWriteFile(t, filepath.Join(base, "reference.md"), "# reference body\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if len(c.Skills) != 1 {
		t.Fatalf("want 1 skill, got %d", len(c.Skills))
	}
	doc := skillDocByRelPath(c.Skills[0], "reference.md")
	if doc == nil {
		t.Fatalf("reference.md not loaded into Docs; got %+v", c.Skills[0].Docs)
	}
	if doc.Content != "# reference body\n" {
		t.Errorf("doc content = %q, want %q", doc.Content, "# reference body\n")
	}
}

func TestLoadSkillExcludesManifestFromDocs(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")
	mustWriteFile(t, filepath.Join(base, "reference.md"), "# reference\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if doc := skillDocByRelPath(c.Skills[0], "SKILL.md"); doc != nil {
		t.Errorf("SKILL.md must not appear in Docs, got %+v", c.Skills[0].Docs)
	}
}

func TestLoadSkillReadsNestedDocs(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")
	mustWriteFile(t, filepath.Join(base, "examples", "invoice.md"), "# invoice\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert: relPath preserved with forward slashes
	if doc := skillDocByRelPath(c.Skills[0], "examples/invoice.md"); doc == nil {
		t.Fatalf("nested doc examples/invoice.md not loaded; got %+v", c.Skills[0].Docs)
	}
}

func TestLoadSkillIgnoresNonMarkdown(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")
	mustWriteFile(t, filepath.Join(base, "scripts", "extract.py"), "print('hi')\n")
	mustWriteFile(t, filepath.Join(base, "reference.md"), "# reference\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert: only the .md doc is present
	if len(c.Skills[0].Docs) != 1 {
		t.Fatalf("want 1 doc (reference.md), got %+v", c.Skills[0].Docs)
	}
	if c.Skills[0].Docs[0].RelPath != "reference.md" {
		t.Errorf("doc = %q, want reference.md", c.Skills[0].Docs[0].RelPath)
	}
}

func TestLoadSkillDocsSortedByRelPath(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")
	mustWriteFile(t, filepath.Join(base, "zeta.md"), "z\n")
	mustWriteFile(t, filepath.Join(base, "alpha.md"), "a\n")
	mustWriteFile(t, filepath.Join(base, "examples", "beta.md"), "b\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	var got []string
	for _, d := range c.Skills[0].Docs {
		got = append(got, d.RelPath)
	}
	want := []string{"alpha.md", "examples/beta.md", "zeta.md"}
	if len(got) != len(want) {
		t.Fatalf("docs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("docs = %v, want %v", got, want)
		}
	}
}

func TestLoadSingleFileSkillHasNoDocs(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "pdf-tools")
	mustWriteFile(t, filepath.Join(base, "SKILL.md"), "---\nname: pdf-tools\n---\n# manifest\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if len(c.Skills[0].Docs) != 0 {
		t.Errorf("want no docs for single-file skill, got %+v", c.Skills[0].Docs)
	}
}

func TestLoadSkipsSkillDirWithoutManifest(t *testing.T) {
	// Arrange: a dir with docs but no SKILL.md is not a skill
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync", "skills", "orphan")
	mustWriteFile(t, filepath.Join(base, "reference.md"), "# reference\n")

	// Act
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if len(c.Skills) != 0 {
		t.Errorf("dir without SKILL.md must be skipped, got %+v", c.Skills)
	}
}

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
