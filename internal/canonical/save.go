package canonical

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/noah-hrbth/agentsync/internal/safepath"
)

// SaveAgentsMD writes content to <workspace>/.agentsync/AGENTS.md.
func SaveAgentsMD(workspace, content string) error {
	return safepath.WriteFile(workspace, filepath.Join(".agentsync", "AGENTS.md"), []byte(content), 0o644)
}

// SaveSkill writes only the skill manifest (SKILL.md frontmatter + body) to
// <workspace>/.agentsync/skills/<dir>/SKILL.md. Skill docs (other .md files) are
// written individually via SaveSkillDoc, not here.
func SaveSkill(workspace string, s *Skill) error {
	out, err := RenderSkill(s)
	if err != nil {
		return fmt.Errorf("marshal skill %s: %w", s.Dir, err)
	}
	return safepath.WriteFile(workspace, filepath.Join(".agentsync", "skills", s.Dir, "SKILL.md"), []byte(out), 0o644)
}

// SaveSkillDoc writes a skill doc (an additional .md file) at
// .agentsync/skills/<dir>/<relPath>. relPath must be a relative .md path within
// the skill dir (never SKILL.md); parent subdirs are created as needed.
func SaveSkillDoc(workspace, dir, relPath, content string) error {
	if err := ValidateSkillDocRelPath(relPath); err != nil {
		return err
	}
	rel := filepath.Join(".agentsync", "skills", dir, filepath.FromSlash(relPath))
	return safepath.WriteFile(workspace, rel, []byte(content), 0o644)
}

// DeleteSkillDoc removes .agentsync/skills/<dir>/<relPath> and prunes any parent
// subdirectories left empty, stopping at (never removing) the skill dir.
func DeleteSkillDoc(workspace, dir, relPath string) error {
	if err := ValidateSkillDocRelPath(relPath); err != nil {
		return err
	}
	skillDir := filepath.Join(".agentsync", "skills", dir)
	rel := filepath.Join(skillDir, filepath.FromSlash(relPath))
	if err := safepath.Remove(workspace, rel); err != nil {
		return err
	}
	return pruneEmptyDirs(workspace, skillDir, filepath.Dir(rel))
}

// DeleteSkillSubdir removes a subfolder (and all its contents) under a skill dir:
// .agentsync/skills/<dir>/<relDir>. relDir must be a relative path within the
// skill dir (no "..", not absolute).
func DeleteSkillSubdir(workspace, dir, relDir string) error {
	if err := validateSkillSubdirRelPath(relDir); err != nil {
		return err
	}
	return safepath.RemoveAll(workspace, filepath.Join(".agentsync", "skills", dir, filepath.FromSlash(relDir)))
}

// validateSkillSubdirRelPath rejects subdir paths that are empty, absolute, or
// escape the skill dir.
func validateSkillSubdirRelPath(relDir string) error {
	if relDir == "" {
		return fmt.Errorf("skill subdir is empty")
	}
	if strings.HasPrefix(relDir, "/") {
		return fmt.Errorf("skill subdir %q must be relative", relDir)
	}
	clean := filepath.ToSlash(filepath.Clean(relDir))
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." {
			return fmt.Errorf("skill subdir %q must not contain ..", relDir)
		}
	}
	return nil
}

// pruneEmptyDirs removes empty directories from dir upward, stopping before stop.
func pruneEmptyDirs(workspace, stop, dir string) error {
	for dir != stop {
		abs, err := safepath.Resolve(workspace, dir)
		if err != nil {
			return err
		}
		entries, err := os.ReadDir(abs)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(entries) != 0 {
			return nil
		}
		if err := safepath.Remove(workspace, dir); err != nil {
			return err
		}
		dir = filepath.Dir(dir)
	}
	return nil
}

// ValidateSkillDocRelPath rejects skill-doc paths that are absolute, name a
// folder, aren't .md, are the SKILL.md manifest, or escape the skill dir.
// Exported so the TUI's add-doc input shares one validation rule.
func ValidateSkillDocRelPath(relPath string) error {
	switch {
	case relPath == "":
		return fmt.Errorf("skill doc path is empty")
	case strings.HasPrefix(relPath, "/"):
		return fmt.Errorf("skill doc path %q must be relative", relPath)
	case strings.HasSuffix(relPath, "/"):
		return fmt.Errorf("skill doc path %q must name a .md file, not a folder", relPath)
	case filepath.Ext(relPath) != ".md":
		return fmt.Errorf("skill doc path %q must end in .md", relPath)
	}
	clean := filepath.ToSlash(filepath.Clean(relPath))
	if clean == "SKILL.md" {
		return fmt.Errorf("SKILL.md is the manifest, not a skill doc")
	}
	for _, seg := range strings.Split(clean, "/") {
		if seg == ".." {
			return fmt.Errorf("skill doc path %q must not contain ..", relPath)
		}
	}
	return nil
}

// SaveAgent writes an agent file to <workspace>/.agentsync/agents/<filename>.md.
func SaveAgent(workspace string, a *Agent) error {
	out, err := RenderAgent(a)
	if err != nil {
		return fmt.Errorf("marshal agent %s: %w", a.Filename, err)
	}
	return safepath.WriteFile(workspace, filepath.Join(".agentsync", "agents", a.Filename+".md"), []byte(out), 0o644)
}

// SaveCommand writes a command file to <workspace>/.agentsync/commands/<filename>.md.
func SaveCommand(workspace string, cmd *Command) error {
	out, err := RenderCommand(cmd)
	if err != nil {
		return fmt.Errorf("marshal command %s: %w", cmd.Filename, err)
	}
	return safepath.WriteFile(workspace, filepath.Join(".agentsync", "commands", cmd.Filename+".md"), []byte(out), 0o644)
}

// SaveRule writes a rule file to <workspace>/.agentsync/rules/<filename>.md.
func SaveRule(workspace string, r *Rule) error {
	out, err := RenderRule(r)
	if err != nil {
		return fmt.Errorf("marshal rule %s: %w", r.Filename, err)
	}
	return safepath.WriteFile(workspace, filepath.Join(".agentsync", "rules", r.Filename+".md"), []byte(out), 0o644)
}

// DeleteRule removes .agentsync/rules/<slug>.md.
func DeleteRule(workspace, slug string) error {
	return safepath.Remove(workspace, filepath.Join(".agentsync", "rules", slug+".md"))
}

// DeleteSkill removes the entire .agentsync/skills/<dir>/ folder.
func DeleteSkill(workspace, dir string) error {
	return safepath.RemoveAll(workspace, filepath.Join(".agentsync", "skills", dir))
}

// DeleteAgent removes .agentsync/agents/<slug>.md.
func DeleteAgent(workspace, slug string) error {
	return safepath.Remove(workspace, filepath.Join(".agentsync", "agents", slug+".md"))
}

// DeleteCommand removes .agentsync/commands/<slug>.md.
func DeleteCommand(workspace, slug string) error {
	return safepath.Remove(workspace, filepath.Join(".agentsync", "commands", slug+".md"))
}

// CreateEmptyRule writes a minimal rule file at .agentsync/rules/<slug>.md
// and returns the populated struct. Body is set to "# <slug>\n" so the file
// isn't a degenerate stub when all frontmatter fields are omitempty.
func CreateEmptyRule(workspace, slug string) (*Rule, error) {
	r := &Rule{Filename: slug, Body: "# " + slug + "\n"}
	if err := SaveRule(workspace, r); err != nil {
		return nil, err
	}
	return r, nil
}

// CreateEmptySkill writes a minimal skill at .agentsync/skills/<dir>/SKILL.md
// (Name defaults to dir, Description empty) and returns the populated struct.
func CreateEmptySkill(workspace, dir string) (*Skill, error) {
	s := &Skill{Dir: dir, Name: dir, Description: "", Body: "# " + dir + "\n"}
	if err := SaveSkill(workspace, s); err != nil {
		return nil, err
	}
	return s, nil
}

// CreateEmptySkillDoc writes a minimal skill doc at
// .agentsync/skills/<dir>/<relPath> (body "# <basename>\n") and returns it.
// Parent subdirs in relPath are created implicitly.
func CreateEmptySkillDoc(workspace, dir, relPath string) (*SkillDoc, error) {
	base := strings.TrimSuffix(filepath.Base(relPath), ".md")
	doc := &SkillDoc{RelPath: filepath.ToSlash(relPath), Content: "# " + base + "\n"}
	if err := SaveSkillDoc(workspace, dir, relPath, doc.Content); err != nil {
		return nil, err
	}
	return doc, nil
}

// CreateEmptyAgent writes a minimal subagent at .agentsync/agents/<slug>.md
// and returns the populated struct.
func CreateEmptyAgent(workspace, slug string) (*Agent, error) {
	a := &Agent{Filename: slug, Name: slug, Description: "", Body: "# " + slug + "\n"}
	if err := SaveAgent(workspace, a); err != nil {
		return nil, err
	}
	return a, nil
}

// CreateEmptyCommand writes a minimal command at .agentsync/commands/<slug>.md
// and returns the populated struct.
func CreateEmptyCommand(workspace, slug string) (*Command, error) {
	cmd := &Command{Filename: slug, Description: "", Body: "# " + slug + "\n"}
	if err := SaveCommand(workspace, cmd); err != nil {
		return nil, err
	}
	return cmd, nil
}

// RenderSkill serializes a skill to its on-disk format (frontmatter + body).
func RenderSkill(s *Skill) (string, error) {
	return renderFile(s, s.Body)
}

// RenderAgent serializes an agent to its on-disk format (frontmatter + body).
func RenderAgent(a *Agent) (string, error) {
	return renderFile(a, a.Body)
}

// RenderCommand serializes a command to its on-disk format (frontmatter + body).
func RenderCommand(cmd *Command) (string, error) {
	return renderFile(cmd, cmd.Body)
}

// RenderRule serializes a rule to its on-disk format (frontmatter + body).
func RenderRule(r *Rule) (string, error) {
	return renderFile(r, r.Body)
}

// renderFile produces a frontmatter + body file string.
// fm is marshaled as YAML; fields tagged yaml:"-" are excluded automatically.
func renderFile(fm any, body string) (string, error) {
	data, err := yaml.Marshal(fm)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	sb.Write(data)
	sb.WriteString("---\n")
	sb.WriteString(body)
	return sb.String(), nil
}
