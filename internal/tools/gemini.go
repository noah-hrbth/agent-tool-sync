package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type geminiAdapter struct{}

func (a *geminiAdapter) Name() string { return "Gemini CLI" }

func (a *geminiAdapter) Detect(_ string) Installation {
	return detectGlobalDir("gemini")
}

func (a *geminiAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules, ConceptSkills, ConceptAgents, ConceptCommands:
		return Compatibility{Supported: true}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *geminiAdapter) SupportsScope(_ Scope) Compatibility {
	return Compatibility{Supported: true}
}

func (a *geminiAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "GEMINI.md"
	}
	return ""
}

func (a *geminiAdapter) Notice() string { return "" }

func (a *geminiAdapter) Render(c *canonical.Canonical, _ Scope) ([]FileWrite, error) {
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	files := []FileWrite{
		{Concept: ConceptRules, Path: filepath.Join(".gemini", "GEMINI.md"), Content: []byte(rootContent)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".gemini", "skills", skill.Dir, "SKILL.md"),
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
			Path:    filepath.Join(".gemini", "agents", agent.Filename+".md"),
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
			Path:    filepath.Join(".gemini", "commands", cmd.Filename+".toml"),
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
