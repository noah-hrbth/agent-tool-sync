package canonical

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCreateEmptyRoundTrip verifies that each CreateEmpty* helper produces a
// file that round-trips through Load — i.e. the minimal frontmatter we write
// is parseable.
func TestCreateEmptyRoundTrip(t *testing.T) {
	ws := t.TempDir()

	if _, err := CreateEmptyRule(ws, "style-guide"); err != nil {
		t.Fatalf("CreateEmptyRule: %v", err)
	}
	if _, err := CreateEmptySkill(ws, "release-prep"); err != nil {
		t.Fatalf("CreateEmptySkill: %v", err)
	}
	if _, err := CreateEmptyAgent(ws, "adapter-reviewer"); err != nil {
		t.Fatalf("CreateEmptyAgent: %v", err)
	}
	if _, err := CreateEmptyCommand(ws, "ship"); err != nil {
		t.Fatalf("CreateEmptyCommand: %v", err)
	}

	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Rules) != 1 || c.Rules[0].Filename != "style-guide" {
		t.Errorf("rules: want 1 entry 'style-guide', got %+v", c.Rules)
	}
	if len(c.Skills) != 1 || c.Skills[0].Dir != "release-prep" || c.Skills[0].Name != "release-prep" {
		t.Errorf("skills: want 1 entry 'release-prep', got %+v", c.Skills)
	}
	if len(c.Agents) != 1 || c.Agents[0].Filename != "adapter-reviewer" {
		t.Errorf("agents: want 1 entry 'adapter-reviewer', got %+v", c.Agents)
	}
	if len(c.Commands) != 1 || c.Commands[0].Filename != "ship" {
		t.Errorf("commands: want 1 entry 'ship', got %+v", c.Commands)
	}
}

// TestDeleteHelpers verifies that each Delete* removes its on-disk artifact
// and that a subsequent Load returns the entity gone from canonical.
func TestDeleteHelpers(t *testing.T) {
	ws := t.TempDir()
	if _, err := CreateEmptyRule(ws, "r1"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEmptySkill(ws, "s1"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEmptyAgent(ws, "a1"); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateEmptyCommand(ws, "c1"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteRule(ws, "r1"); err != nil {
		t.Fatalf("DeleteRule: %v", err)
	}
	if err := DeleteSkill(ws, "s1"); err != nil {
		t.Fatalf("DeleteSkill: %v", err)
	}
	if err := DeleteAgent(ws, "a1"); err != nil {
		t.Fatalf("DeleteAgent: %v", err)
	}
	if err := DeleteCommand(ws, "c1"); err != nil {
		t.Fatalf("DeleteCommand: %v", err)
	}

	for _, p := range []string{
		filepath.Join(ws, ".agentsync", "rules", "r1.md"),
		filepath.Join(ws, ".agentsync", "skills", "s1"),
		filepath.Join(ws, ".agentsync", "agents", "a1.md"),
		filepath.Join(ws, ".agentsync", "commands", "c1.md"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed, got err=%v", p, err)
		}
	}

	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Rules) != 0 || len(c.Skills) != 0 || len(c.Agents) != 0 || len(c.Commands) != 0 {
		t.Errorf("expected empty canonical after deletes, got %+v", c)
	}
}

func TestSaveSkillDocWritesAtRelPath(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := SaveSkillDoc(ws, "pdf-tools", "reference.md", "# reference\n"); err != nil {
		t.Fatalf("SaveSkillDoc: %v", err)
	}

	// Assert
	got, err := os.ReadFile(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "reference.md"))
	if err != nil {
		t.Fatalf("read doc: %v", err)
	}
	if string(got) != "# reference\n" {
		t.Errorf("content = %q, want %q", got, "# reference\n")
	}
}

func TestSaveSkillDocCreatesNestedParents(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := SaveSkillDoc(ws, "pdf-tools", "examples/invoice.md", "# invoice\n"); err != nil {
		t.Fatalf("SaveSkillDoc: %v", err)
	}

	// Assert
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "examples", "invoice.md")); err != nil {
		t.Errorf("nested doc not written: %v", err)
	}
}

func TestCreateEmptySkillDocPersistsStub(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}

	// Act
	doc, err := CreateEmptySkillDoc(ws, "pdf-tools", "docs/test.md")
	if err != nil {
		t.Fatalf("CreateEmptySkillDoc: %v", err)
	}

	// Assert
	if doc.RelPath != "docs/test.md" {
		t.Errorf("RelPath = %q, want docs/test.md", doc.RelPath)
	}
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if d := skillDocByRelPath(c.Skills[0], "docs/test.md"); d == nil {
		t.Errorf("created doc not loaded back; got %+v", c.Skills[0].Docs)
	}
}

func TestDeleteSkillDocRemovesFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSkillDoc(ws, "pdf-tools", "reference.md", "# ref\n"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := DeleteSkillDoc(ws, "pdf-tools", "reference.md"); err != nil {
		t.Fatalf("DeleteSkillDoc: %v", err)
	}

	// Assert: doc gone, manifest preserved
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "reference.md")); !os.IsNotExist(err) {
		t.Errorf("doc not removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md")); err != nil {
		t.Errorf("manifest must survive doc delete: %v", err)
	}
}

func TestDeleteSkillDocPrunesEmptyParentsStoppingAtSkillDir(t *testing.T) {
	// Arrange: deeply nested doc, manifest at top
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSkillDoc(ws, "pdf-tools", "a/b/c.md", "x\n"); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := DeleteSkillDoc(ws, "pdf-tools", "a/b/c.md"); err != nil {
		t.Fatalf("DeleteSkillDoc: %v", err)
	}

	// Assert: empty intermediate dirs pruned, skill dir + manifest remain
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "a")); !os.IsNotExist(err) {
		t.Errorf("empty parent a/ should be pruned, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md")); err != nil {
		t.Errorf("skill dir must not be pruned: %v", err)
	}
}

func TestDeleteSkillSubdirRemovesFolder(t *testing.T) {
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSkillDoc(ws, "pdf-tools", "tests/a.md", "a\n"); err != nil {
		t.Fatal(err)
	}
	if err := SaveSkillDoc(ws, "pdf-tools", "tests/b.md", "b\n"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSkillSubdir(ws, "pdf-tools", "tests"); err != nil {
		t.Fatalf("DeleteSkillSubdir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "tests")); !os.IsNotExist(err) {
		t.Error("subdir folder should be removed")
	}
	if _, err := os.Stat(filepath.Join(ws, ".agentsync", "skills", "pdf-tools", "SKILL.md")); err != nil {
		t.Error("manifest must survive subdir delete")
	}
}

func TestDeleteSkillSubdirRejectsBad(t *testing.T) {
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "s"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "..", "../x", "/abs"} {
		if err := DeleteSkillSubdir(ws, "s", bad); err == nil {
			t.Errorf("DeleteSkillSubdir(%q) = nil, want error", bad)
		}
	}
}

func TestSaveSkillDocRejectsBadRelPaths(t *testing.T) {
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "pdf-tools"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"../escape.md", "SKILL.md", "/abs.md", "a/../../b.md", "notes.txt", "docs/"} {
		if err := SaveSkillDoc(ws, "pdf-tools", rel, "x\n"); err == nil {
			t.Errorf("SaveSkillDoc(%q) = nil, want error", rel)
		}
	}
}

// TestCreateEmptySkillFolderLayout verifies a skill creates the SKILL.md
// inside its named folder, not as a flat file.
func TestCreateEmptySkillFolderLayout(t *testing.T) {
	ws := t.TempDir()
	if _, err := CreateEmptySkill(ws, "my-skill"); err != nil {
		t.Fatalf("CreateEmptySkill: %v", err)
	}
	skillFile := filepath.Join(ws, ".agentsync", "skills", "my-skill", "SKILL.md")
	c, err := Load(ws)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Skills) != 1 {
		t.Fatalf("want 1 skill, got %d", len(c.Skills))
	}
	if c.Skills[0].Dir != "my-skill" {
		t.Errorf("Dir = %q, want my-skill", c.Skills[0].Dir)
	}
	if _, err := os.Stat(skillFile); err != nil {
		t.Errorf("expected SKILL.md at %s: %v", skillFile, err)
	}
}
