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
