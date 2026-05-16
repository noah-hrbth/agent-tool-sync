package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var junieMeta = ToolMeta{
	Key:    "junie",
	Name:   "JetBrains Junie",
	Detect: detectGlobalDir("junie"),
	Concepts: map[Concept]Compatibility{
		ConceptRules:    {Supported: true},
		ConceptSkills:   {Supported: true},
		ConceptAgents:   {Supported: true},
		ConceptCommands: {Supported: true},
	},
	// Junie itself supports a user-scope tree under ~/.junie/ for skills/agents/commands.
	// Root memory (AGENTS.md / guidelines) is project-only — Render skips it at user scope.
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at AGENTS.md (project root). Project scope only — rules and AGENTS.md are not synced at user scope. Per-file rules append to AGENTS.md.",
		ConceptSkills:   "Skills at .junie/skills/<dir>/SKILL.md (project) or ~/.junie/skills/ (user).",
		ConceptAgents:   "Subagents at .junie/agents/<name>.md (project) or ~/.junie/agents/ (user).",
		ConceptCommands: "Commands at .junie/commands/<name>.md (project) or ~/.junie/commands/ (user).",
	},
}

func renderJunie(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
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
