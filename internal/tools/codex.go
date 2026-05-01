package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type codexAdapter struct{}

func (a *codexAdapter) Name() string { return "Codex CLI" }

func (a *codexAdapter) Detect(_ string) Installation {
	return detectGlobalDir("codex")
}

func (a *codexAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules, ConceptSkills, ConceptAgents:
		return Compatibility{Supported: true}
	case ConceptCommands:
		return Compatibility{
			Supported:   true,
			Deprecated:  true,
			Reason:      "legacy prompts deprecated — prefer skills",
			Replacement: "skills",
		}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *codexAdapter) SupportsScope(_ Scope) Compatibility {
	return Compatibility{Supported: true}
}

func (a *codexAdapter) Alias(_ Concept) string { return "" }

func (a *codexAdapter) Notice() string {
	return "project skills are written to .agents/skills/ (cross-tool), user skills to ~/.codex/skills/"
}

func (a *codexAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	// Codex CLI reads AGENTS.md from the workspace root (and parent dirs) at project
	// scope; user scope reads from ~/.codex/AGENTS.md.
	rootPath := "AGENTS.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(".codex", "AGENTS.md")
	}
	files := []FileWrite{
		{Concept: ConceptRules, Path: rootPath, Content: []byte(rootContent)},
	}

	// Project skills live at .agents/skills/ (auto-scanned by Codex from cwd to repo root).
	// User skills live at ~/.codex/skills/ (per Codex docs).
	skillBase := ".agents"
	if scope == ScopeUser {
		skillBase = ".codex"
	}
	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillBase, "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	for _, agent := range c.Agents {
		content := buildTOML([]fmField{
			{key: "name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "developer_instructions", value: agent.Body},
			{key: "model", value: agent.Model},
		})
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(".codex", "agents", agent.Filename+".toml"),
			Content: []byte(content),
		})
	}

	// Commands are deprecated for Codex CLI; not rendered.

	return files, nil
}
