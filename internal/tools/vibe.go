package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var vibeMeta = ToolMeta{
	Key:    "vibe",
	Name:   "Mistral Vibe",
	Detect: detectGlobalDir("vibe"),
	Concepts: map[Concept]Compatibility{
		ConceptRules:  {Supported: true},
		ConceptSkills: {Supported: true},
		ConceptAgents: {Supported: true},
		ConceptCommands: {
			Supported:   true,
			Deprecated:  true,
			Reason:      "Mistral Vibe slash commands are skills with user-invocable: true — prefer skills",
			Replacement: "skills",
		},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Rules flatten into AGENTS.md at workspace root (or ~/.vibe/AGENTS.md at user scope) — Vibe has no per-file or glob-scoped rules.",
		ConceptSkills:   "Skills at .vibe/skills/<dir>/SKILL.md.",
		ConceptAgents:   "Subagents emit a TOML config at .vibe/agents/<name>.toml plus a prompt at .vibe/prompts/<name>.md (referenced by system_prompt_id).",
		ConceptCommands: "Commands render as user-invocable skills under .vibe/skills/<name>/SKILL.md — command names must not collide with skill names. Vibe has no separate commands concept; prefer skills.",
	},
}

func renderVibe(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	var files []FileWrite

	// Rules flatten into AGENTS.md (no per-file rules; Vibe has no globs concept).
	rootPath := "AGENTS.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(".vibe", "AGENTS.md")
	}
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	files = append(files, FileWrite{
		Concept: ConceptRules,
		Path:    rootPath,
		Content: []byte(rootContent),
	})

	// Skills: kebab-case YAML frontmatter; omit user-invocable (Vibe defaults to true)
	// so command-emitted skills (below) remain distinguishable.
	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "allowed-tools", value: skill.AllowedTools},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".vibe", "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	// Agents: TOML config + separate prompt file referenced by system_prompt_id.
	// The Markdown body has no native home in the TOML config block.
	// .vibe/agents/<name>.toml and .vibe/prompts/<name>.md are intentionally NOT
	// reverse-mapped in adopt.go — TOML agent adoption is deferred (parity with
	// Codex CLI's TOML-agent gap).
	for _, agent := range c.Agents {
		promptID := ""
		if agent.Body != "" {
			promptID = agent.Filename
		}
		toml := buildTOML([]fmField{
			{key: "display_name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "active_model", value: agent.Model},
			{key: "enabled_tools", value: agent.Tools},
			{key: "system_prompt_id", value: promptID},
		})
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(".vibe", "agents", agent.Filename+".toml"),
			Content: []byte(toml),
		})
		if agent.Body != "" {
			files = append(files, FileWrite{
				Concept: ConceptAgents,
				Path:    filepath.Join(".vibe", "prompts", agent.Filename+".md"),
				Content: []byte(agent.Body),
			})
		}
	}

	// Commands: Vibe has no separate commands concept — slash commands ARE skills
	// with user-invocable: true. Land them under .vibe/skills/ alongside skills.
	// The Deprecated flag in Supports(ConceptCommands) reflects canonical-concept
	// status (vendor-recommended successor is "skills"), not output suppression —
	// existing canonical commands continue to render here so users keep their
	// slash commands working.
	for _, cmd := range c.Commands {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: cmd.Filename},
			{key: "description", value: cmd.Description},
			{key: "user-invocable", value: true},
		}, cmd.Body)
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(".vibe", "skills", cmd.Filename, "SKILL.md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
