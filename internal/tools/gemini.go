package tools

import (
	"path/filepath"

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

func (a *geminiAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "GEMINI.md"
	}
	return ""
}

func (a *geminiAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	files := []FileWrite{
		{Concept: ConceptRules, Path: "GEMINI.md", Content: []byte(c.Rules)},
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
