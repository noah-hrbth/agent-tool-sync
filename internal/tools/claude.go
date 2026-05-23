package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var claudeMeta = ToolMeta{
	Key:    "claude",
	Name:   "Claude Code",
	Detect: detectGlobalDir("claude"),
	Aliases: map[Concept]string{
		ConceptRules: "CLAUDE.md",
	},
	Concepts: map[Concept]Compatibility{
		ConceptRules:  {Supported: true},
		ConceptSkills: {Supported: true},
		ConceptAgents: {Supported: true},
		ConceptCommands: {
			Supported:   true,
			Deprecated:  true,
			Reason:      "merged into skills 2026-01-24 — prefer skills",
			Replacement: "skills",
		},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at CLAUDE.md (project) or ~/.claude/CLAUDE.md (user). Per-file rules at .claude/rules/<name>.md.",
		ConceptSkills:   "Skills at .claude/skills/<dir>/SKILL.md.",
		ConceptAgents:   "Subagents at .claude/agents/<name>.md.",
		ConceptCommands: "Claude Code merged commands into skills on 2026-01-24 — prefer skills. Commands are not rendered.",
	},
}

func renderClaude(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	// Project memory lives at <workspace>/CLAUDE.md (auto-discovered by Claude Code
	// from cwd up the tree). User memory lives at ~/.claude/CLAUDE.md.
	rootPath := "CLAUDE.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(claudeDir, "CLAUDE.md")
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
			Path:    filepath.Join(claudeDir, "rules", r.Filename+".md"),
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
		skillDir := filepath.Join(claudeDir, "skills", skill.Dir)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillDir, "SKILL.md"),
			Content: []byte(content),
		})
		files = appendSkillDocs(files, skillDir, skill.Docs)
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
			Path:    filepath.Join(claudeDir, "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
