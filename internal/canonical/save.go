package canonical

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// SaveAgentsMD writes content to <workspace>/.agentsync/AGENTS.md.
func SaveAgentsMD(workspace, content string) error {
	path := filepath.Join(workspace, ".agentsync", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// SaveSkill writes a skill's SKILL.md (frontmatter + body) to
// <workspace>/.agentsync/skills/<dir>/SKILL.md.
func SaveSkill(workspace string, s *Skill) error {
	out, err := RenderSkill(s)
	if err != nil {
		return fmt.Errorf("marshal skill %s: %w", s.Dir, err)
	}
	path := filepath.Join(workspace, ".agentsync", "skills", s.Dir, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// SaveAgent writes an agent file to <workspace>/.agentsync/agents/<filename>.md.
func SaveAgent(workspace string, a *Agent) error {
	out, err := RenderAgent(a)
	if err != nil {
		return fmt.Errorf("marshal agent %s: %w", a.Filename, err)
	}
	path := filepath.Join(workspace, ".agentsync", "agents", a.Filename+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// SaveCommand writes a command file to <workspace>/.agentsync/commands/<filename>.md.
func SaveCommand(workspace string, cmd *Command) error {
	out, err := RenderCommand(cmd)
	if err != nil {
		return fmt.Errorf("marshal command %s: %w", cmd.Filename, err)
	}
	path := filepath.Join(workspace, ".agentsync", "commands", cmd.Filename+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// SaveRule writes a rule file to <workspace>/.agentsync/rules/<filename>.md.
func SaveRule(workspace string, r *Rule) error {
	out, err := RenderRule(r)
	if err != nil {
		return fmt.Errorf("marshal rule %s: %w", r.Filename, err)
	}
	path := filepath.Join(workspace, ".agentsync", "rules", r.Filename+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
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
