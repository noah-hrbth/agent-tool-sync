package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type claudeAdapter struct{}

func (a *claudeAdapter) Name() string { return "Claude Code" }

func (a *claudeAdapter) Detect(_ string) Installation {
	return detectGlobalDir("claude")
}

func (a *claudeAdapter) Supports(concept Concept) Compatibility {
	if concept == ConceptCommands {
		return Compatibility{
			Supported:   true,
			Deprecated:  true,
			Reason:      "merged into skills 2026-01-24 — prefer skills",
			Replacement: "skills",
		}
	}
	return Compatibility{Supported: true}
}

func (a *claudeAdapter) SupportsScope(_ Scope) Compatibility {
	return Compatibility{Supported: true}
}

func (a *claudeAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "CLAUDE.md"
	}
	return ""
}

func (a *claudeAdapter) Notice() string { return "" }

func (a *claudeAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	// Project memory lives at <workspace>/CLAUDE.md (auto-discovered by Claude Code
	// from cwd up the tree). User memory lives at ~/.claude/CLAUDE.md.
	rootPath := "CLAUDE.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(".claude", "CLAUDE.md")
	}
	files := []FileWrite{
		{Concept: ConceptRules, Path: rootPath, Content: []byte(c.AgentsMD)},
	}

	for _, r := range c.Rules {
		content := buildMDFrontmatter([]fmField{
			{key: "paths", value: r.Paths},
		}, r.Body)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    filepath.Join(".claude", "rules", r.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "allowed-tools", value: skill.AllowedTools},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
			{key: "paths", value: skill.Paths},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".claude", "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	for _, agent := range c.Agents {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "tools", value: agent.Tools},
			{key: "model", value: agent.Model},
		}, agent.Body)
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(".claude", "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
