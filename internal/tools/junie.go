package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type junieAdapter struct{}

func (a *junieAdapter) Name() string { return "JetBrains Junie" }

func (a *junieAdapter) Detect(_ string) Installation {
	return detectGlobalDir("junie")
}

func (a *junieAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules, ConceptSkills, ConceptAgents, ConceptCommands:
		return Compatibility{Supported: true}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *junieAdapter) SupportsScope(_ Scope) Compatibility {
	// Junie itself supports a user-scope tree under ~/.junie/ for skills/agents/commands.
	// Root memory (AGENTS.md / guidelines) is project-only — Render skips it at user scope.
	return Compatibility{Supported: true}
}

func (a *junieAdapter) Alias(_ Concept) string { return "" }

func (a *junieAdapter) Notice() string {
	return "reads AGENTS.md from workspace root (project-only — rules and AGENTS.md are not synced at user scope); skills/agents/commands are honoured at user scope under ~/.junie/"
}

func (a *junieAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	var files []FileWrite

	// Root memory: project scope only. Junie's lookup chain is .junie/AGENTS.md →
	// AGENTS.md → .junie/guidelines.md; we write to root AGENTS.md so it is shared
	// with OpenCode/Codex (no duplicate content needed).
	if scope == ScopeProject {
		rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    "AGENTS.md",
			Content: []byte(rootContent),
		})
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".junie", "skills", skill.Dir, "SKILL.md"),
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
			Path:    filepath.Join(".junie", "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, cmd := range c.Commands {
		content := buildMDFrontmatter([]fmField{
			{key: "description", value: cmd.Description},
		}, cmd.Body)
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(".junie", "commands", cmd.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
