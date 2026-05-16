package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var codexMeta = ToolMeta{
	Key:    "codex",
	Name:   "Codex CLI",
	Detect: detectGlobalDir("codex"),
	Concepts: map[Concept]Compatibility{
		ConceptRules:  {Supported: true},
		ConceptSkills: {Supported: true},
		ConceptAgents: {Supported: true},
		ConceptCommands: {
			Supported:   true,
			Deprecated:  true,
			Reason:      "legacy prompts deprecated — prefer skills",
			Replacement: "skills",
		},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at AGENTS.md (project) or ~/.codex/AGENTS.md (user). Per-file rules append to AGENTS.md — Codex CLI has no per-rule files.",
		ConceptSkills:   "Project skills at .agents/skills/<dir>/SKILL.md (cross-tool, auto-scanned by Codex). User skills at ~/.codex/skills/.",
		ConceptAgents:   "Subagents at .codex/agents/<name>.toml (TOML config with developer_instructions field for the body).",
		ConceptCommands: "Codex legacy prompts are deprecated — prefer skills. Commands are not rendered.",
	},
}

func renderCodex(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
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
