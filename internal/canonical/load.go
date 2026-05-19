package canonical

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"

	"github.com/noah-hrbth/agentsync/internal/safepath"
)

// canonicalRoot is the workspace-relative root of all canonical content.
const canonicalRoot = ".agentsync"

// Load reads the .agentsync/ directory under workspace and returns a populated Canonical.
// Missing AGENTS.md or missing skills/agents/commands/rules directories are not errors.
// Reads are routed through safepath so a symlinked .agentsync entry cannot
// exfiltrate file content from outside the workspace into rendered output.
func Load(workspace string) (*Canonical, error) {
	c := &Canonical{
		Workspace: workspace,
		Rules:     []*Rule{},
		Skills:    []*Skill{},
		Agents:    []*Agent{},
		Commands:  []*Command{},
	}

	agentsMD, err := loadAgentsMD(workspace)
	if err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}
	c.AgentsMD = agentsMD

	rules, err := loadRules(workspace)
	if err != nil {
		return nil, fmt.Errorf("load rules folder: %w", err)
	}
	c.Rules = rules

	skills, err := loadSkills(workspace)
	if err != nil {
		return nil, fmt.Errorf("load skills: %w", err)
	}
	c.Skills = skills

	agents, err := loadAgents(workspace)
	if err != nil {
		return nil, fmt.Errorf("load agents: %w", err)
	}
	c.Agents = agents

	commands, err := loadCommands(workspace)
	if err != nil {
		return nil, fmt.Errorf("load commands: %w", err)
	}
	c.Commands = commands

	return c, nil
}

// safeReadDir validates the workspace-relative dir (rejecting a symlinked
// component) and lists it. A missing dir is not an error.
func safeReadDir(workspace, rel string) ([]os.DirEntry, error) {
	abs, err := safepath.Resolve(workspace, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func loadAgentsMD(workspace string) (string, error) {
	data, err := safepath.ReadFile(workspace, filepath.Join(canonicalRoot, "AGENTS.md"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func loadRules(workspace string) ([]*Rule, error) {
	relDir := filepath.Join(canonicalRoot, "rules")
	entries, err := safeReadDir(workspace, relDir)
	if err != nil {
		return nil, err
	}

	var rules []*Rule
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		filename := strings.TrimSuffix(entry.Name(), ".md")
		if IsReservedRuleName(filename) {
			return nil, fmt.Errorf("reserved rule name %q — %s", filename, ReservedRuleReason(filename))
		}

		data, err := safepath.ReadFile(workspace, filepath.Join(relDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read rule %s: %w", entry.Name(), err)
		}

		var r Rule
		body, err := frontmatter.Parse(strings.NewReader(string(data)), &r)
		if err != nil {
			return nil, fmt.Errorf("parse rule %s: %w", entry.Name(), err)
		}
		r.Filename = filename
		r.Body = string(body)
		rules = append(rules, &r)
	}

	if rules == nil {
		return []*Rule{}, nil
	}
	return rules, nil
}

func loadSkills(workspace string) ([]*Skill, error) {
	relDir := filepath.Join(canonicalRoot, "skills")
	entries, err := safeReadDir(workspace, relDir)
	if err != nil {
		return nil, err
	}

	var skills []*Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		data, err := safepath.ReadFile(workspace, filepath.Join(relDir, entry.Name(), "SKILL.md"))
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

func loadAgents(workspace string) ([]*Agent, error) {
	relDir := filepath.Join(canonicalRoot, "agents")
	entries, err := safeReadDir(workspace, relDir)
	if err != nil {
		return nil, err
	}

	var agents []*Agent
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, err := safepath.ReadFile(workspace, filepath.Join(relDir, entry.Name()))
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

func loadCommands(workspace string) ([]*Command, error) {
	relDir := filepath.Join(canonicalRoot, "commands")
	entries, err := safeReadDir(workspace, relDir)
	if err != nil {
		return nil, err
	}

	var commands []*Command
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}

		data, err := safepath.ReadFile(workspace, filepath.Join(relDir, entry.Name()))
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
