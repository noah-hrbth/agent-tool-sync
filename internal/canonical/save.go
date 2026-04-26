package canonical

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// SaveRules writes content to <workspace>/.agentsync/AGENTS.md.
func SaveRules(workspace, content string) error {
	path := filepath.Join(workspace, ".agentsync", "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// SaveSkill writes a skill's SKILL.md (frontmatter + body) to
// <workspace>/.agentsync/skills/<dir>/SKILL.md.
func SaveSkill(workspace string, s *Skill) error {
	type skillFrontmatter struct {
		Name                   string   `yaml:"name"`
		Description            string   `yaml:"description"`
		AllowedTools           []string `yaml:"allowed-tools,omitempty"`
		DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty"`
	}

	fm := skillFrontmatter{
		Name:                   s.Name,
		Description:            s.Description,
		AllowedTools:           s.AllowedTools,
		DisableModelInvocation: s.DisableModelInvocation,
	}

	out, err := marshalFile(fm, s.Body)
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
	type agentFrontmatter struct {
		Name        string   `yaml:"name"`
		Description string   `yaml:"description"`
		Tools       []string `yaml:"tools,omitempty"`
		Model       string   `yaml:"model,omitempty"`
	}

	fm := agentFrontmatter{
		Name:        a.Name,
		Description: a.Description,
		Tools:       a.Tools,
		Model:       a.Model,
	}

	out, err := marshalFile(fm, a.Body)
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
	type commandFrontmatter struct {
		Description  string   `yaml:"description"`
		ArgumentHint string   `yaml:"argument-hint,omitempty"`
		AllowedTools []string `yaml:"allowed-tools,omitempty"`
		Model        string   `yaml:"model,omitempty"`
	}

	fm := commandFrontmatter{
		Description:  cmd.Description,
		ArgumentHint: cmd.ArgumentHint,
		AllowedTools: cmd.AllowedTools,
		Model:        cmd.Model,
	}

	out, err := marshalFile(fm, cmd.Body)
	if err != nil {
		return fmt.Errorf("marshal command %s: %w", cmd.Filename, err)
	}

	path := filepath.Join(workspace, ".agentsync", "commands", cmd.Filename+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// marshalFile produces a frontmatter + body file string.
func marshalFile(fm any, body string) (string, error) {
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
