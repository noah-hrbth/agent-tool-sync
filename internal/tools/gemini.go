package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var geminiMeta = ToolMeta{
	Key:    "gemini",
	Name:   "Gemini CLI",
	Detect: detectGlobalDir("gemini"),
	Aliases: map[Concept]string{
		ConceptRules: "GEMINI.md",
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
		ConceptRules:    "Root memory at GEMINI.md (project) or ~/.gemini/GEMINI.md (user). Per-file rules append to GEMINI.md — Gemini CLI has no per-rule files.",
		ConceptSkills:   "Skills at .gemini/skills/<dir>/SKILL.md.",
		ConceptAgents:   "Subagents at .gemini/agents/<name>.md.",
		ConceptCommands: "Commands at .gemini/commands/<name>.toml (TOML format with description + prompt fields).",
	},
}

func renderGemini(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	// Gemini CLI reads GEMINI.md from the workspace root (and parent dirs) at project
	// scope; user scope reads from ~/.gemini/GEMINI.md.
	rootPath := "GEMINI.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(geminiDir, "GEMINI.md")
	}
	files := []FileWrite{
		{Concept: ConceptRules, Path: rootPath, Content: []byte(rootContent)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		skillDir := filepath.Join(geminiDir, "skills", skill.Dir)
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
			Path:    filepath.Join(geminiDir, "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, cmd := range c.Commands {
		content := buildTOML([]fmField{
			{key: "description", value: cmd.Description},
			{key: "prompt", value: cmd.Body},
		})
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(geminiDir, "commands", cmd.Filename+".toml"),
			Content: []byte(content),
		})
	}

	return files, nil
}

// buildRootMemoryContent appends canonical rules to the AGENTS.md content, one
// ## section per rule (filename as heading). Tools without a per-rule directory
// (Gemini, OpenCode, Codex) use this to produce a single composite memory file.
func buildRootMemoryContent(agentsMD string, rules []*canonical.Rule) string {
	if len(rules) == 0 {
		return agentsMD
	}
	var sb strings.Builder
	sb.WriteString(agentsMD)
	for _, r := range rules {
		fmt.Fprintf(&sb, "\n\n## %s\n\n%s", r.Filename, r.Body)
	}
	return sb.String()
}
