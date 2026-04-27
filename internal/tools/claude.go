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

func (a *claudeAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "CLAUDE.md"
	}
	return ""
}

func (a *claudeAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	files := []FileWrite{
		{Concept: ConceptRules, Path: ".claude/CLAUDE.md", Content: []byte(c.Rules)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "allowed-tools", value: skill.AllowedTools},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
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
