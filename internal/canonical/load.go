package canonical

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
)

// Load reads the .agentsync/ directory under workspace and returns a populated Canonical.
// Missing AGENTS.md or missing skills/agents/commands directories are not errors.
func Load(workspace string) (*Canonical, error) {
	base := filepath.Join(workspace, ".agentsync")

	c := &Canonical{
		Workspace: workspace,
		Skills:    []*Skill{},
		Agents:    []*Agent{},
		Commands:  []*Command{},
	}

	rules, err := loadRules(base)
	if err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}
	c.Rules = rules

	skills, err := loadSkills(base)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	c.Skills = skills

	agents, err := loadAgents(base)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	c.Agents = agents

	commands, err := loadCommands(base)
	if err != nil {
		return nil, fmt.Errorf("load commands: %w", err)
	}
	c.Commands = commands

	return c, nil
}

func loadRules(base string) (string, error) {
	path := filepath.Join(base, "AGENTS.md")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadSkills(base string) ([]*Skill, error) {
	dir := filepath.Join(base, "skills")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Skill{}, nil
	}
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read skill %s: %w", entry.Name(), err)
		}

		var s Skill
		body, err := frontmatter.Parse(strings.NewReader(string(data)), &s)
		if err != nil {
			return nil, fmt.Errorf("parse skill %s: %w", entry.Name(), err)
		}
		s.Dir = entry.Name()
		s.Body = string(body)
		skills = append(skills, &s)
	}

	if skills == nil {
		return []*Skill{}, nil
	}
	return skills, nil
}

func loadAgents(base string) ([]*Agent, error) {
	dir := filepath.Join(base, "agents")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Agent{}, nil
	}
	if err != nil {
		return nil, err
	}

	var agents []*Agent
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read agent %s: %w", entry.Name(), err)
		}

		var a Agent
		body, err := frontmatter.Parse(strings.NewReader(string(data)), &a)
		if err != nil {
			return nil, fmt.Errorf("parse agent %s: %w", entry.Name(), err)
		}
		a.Filename = strings.TrimSuffix(entry.Name(), ".md")
		a.Body = string(body)
		agents = append(agents, &a)
	}

	if agents == nil {
		return []*Agent{}, nil
	}
	return agents, nil
}

func loadCommands(base string) ([]*Command, error) {
	dir := filepath.Join(base, "commands")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []*Command{}, nil
	}
	if err != nil {
		return nil, err
	}

	var commands []*Command
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read command %s: %w", entry.Name(), err)
		}

		var cmd Command
		body, err := frontmatter.Parse(strings.NewReader(string(data)), &cmd)
		if err != nil {
			return nil, fmt.Errorf("parse command %s: %w", entry.Name(), err)
		}
		cmd.Filename = strings.TrimSuffix(entry.Name(), ".md")
		cmd.Body = string(body)
		commands = append(commands, &cmd)
	}

	if commands == nil {
		return []*Command{}, nil
	}
	return commands, nil
}
