package tools

import (
	"fmt"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var cursorMeta = ToolMeta{
	Key:    "cursor",
	Name:   "Cursor",
	Detect: detectGlobalDir("cursor"),
	Aliases: map[Concept]string{
		ConceptRules: "general.mdc",
	},
	Concepts: map[Concept]Compatibility{
		ConceptRules:  {Supported: true},
		ConceptSkills: {Supported: true},
		ConceptAgents: {Supported: true},
		ConceptCommands: {
			Supported:   true,
			Deprecated:  true,
			Reason:      "Cursor promotes skills as the slash-command surface — prefer skills",
			Replacement: "skills",
		},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: false, Reason: "Cursor user rules are managed in the Settings UI, not files"},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "AGENTS.md flattens to .cursor/rules/general.mdc (catch-all). Per-file rules at .cursor/rules/<name>.mdc. User-level rules live in Cursor's Settings UI, not on disk — user scope is unsupported.",
		ConceptSkills:   "Skills at .cursor/skills/<dir>/SKILL.md.",
		ConceptAgents:   "Subagents at .cursor/agents/<name>.md.",
		ConceptCommands: "Commands at .cursor/commands/<name>.md, but Cursor promotes skills as the slash-command surface — prefer skills.",
	},
}

func renderCursor(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	if scope == ScopeUser {
		return nil, nil
	}
	rulesContent := fmt.Sprintf("---\nalwaysApply: true\n---\n%s", c.AgentsMD)
	files := []FileWrite{
		{Concept: ConceptRules, Path: cursorCatchAll, Content: []byte(rulesContent)},
	}

	for _, r := range c.Rules {
		content := buildMDFrontmatter([]fmField{
			{key: "description", value: r.Description},
			{key: "globs", value: r.Paths},
			{key: "alwaysApply", value: false},
		}, r.Body)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    filepath.Join(cursorDir, "rules", r.Filename+".mdc"),
			Content: []byte(content),
		})
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "globs", value: skill.Paths},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
		}, skill.Body)
		skillDir := filepath.Join(cursorDir, "skills", skill.Dir)
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
			{key: "model", value: agent.Model},
		}, agent.Body)
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(cursorDir, "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, cmd := range c.Commands {
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(cursorDir, "commands", cmd.Filename+".md"),
			Content: []byte(cmd.Body),
		})
	}

	return files, nil
}
