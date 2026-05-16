// TODO(opencode-adapter): Agent schema mismatch — we emit `name` and `tools` as flat frontmatter
// fields; OpenCode uses the filename as the agent name and wraps tool access in a `permission`
// object. Missing fields: `mode`, `temperature`, `top_p`, `steps`, `disable`, `hidden`, `color`.
//
// TODO(opencode-adapter): Commands schema mismatch — body should be nested under a `template` key;
// `argument-hint` and `allowed-tools` are Claude-isms that OpenCode does not recognise and ignores.

package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var openCodeMeta = ToolMeta{
	Key:  "opencode",
	Name: "OpenCode",
	Detect: func(ws string) Installation {
		if inst := detectConfigDir("opencode")(ws); inst.Found {
			return inst
		}
		return detectGlobalDir("opencode")(ws)
	},
	Concepts: map[Concept]Compatibility{
		ConceptRules:    {Supported: true},
		ConceptSkills:   {Supported: true},
		ConceptAgents:   {Supported: true},
		ConceptCommands: {Supported: true},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at AGENTS.md (project) or ~/.config/opencode/AGENTS.md (user). Per-file rules append to AGENTS.md — OpenCode has no per-rule files.",
		ConceptSkills:   "Skills at .opencode/skills/<dir>/SKILL.md (project) or ~/.config/opencode/skills/ (user).",
		ConceptAgents:   "Subagents at .opencode/agents/<name>.md.",
		ConceptCommands: "Commands at .opencode/commands/<name>.md.",
	},
}

// openCodeBase returns the path prefix relative to the scope's base directory.
// Project scope: .opencode/. User scope: .config/opencode/ (OpenCode docs:
// "global rules in a ~/.config/opencode/AGENTS.md").
func openCodeBase(scope Scope) string {
	if scope == ScopeUser {
		return filepath.Join(".config", "opencode")
	}
	return ".opencode"
}

func renderOpenCode(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	base := openCodeBase(scope)
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	// OpenCode reads AGENTS.md from the workspace root (and parent dirs) at project
	// scope; user scope reads from ~/.config/opencode/AGENTS.md.
	rootPath := "AGENTS.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(base, "AGENTS.md")
	}
	files := []FileWrite{
		{Concept: ConceptRules, Path: rootPath, Content: []byte(rootContent)},
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
			Path:    filepath.Join(base, "skills", skill.Dir, "SKILL.md"),
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
			Path:    filepath.Join(base, "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, cmd := range c.Commands {
		content := buildMDFrontmatter([]fmField{
			{key: "description", value: cmd.Description},
			{key: "argument-hint", value: cmd.ArgumentHint},
			{key: "allowed-tools", value: cmd.AllowedTools},
			{key: "model", value: cmd.Model},
		}, cmd.Body)
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(base, "commands", cmd.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
