package tools

import (
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type openCodeAdapter struct{}

func (a *openCodeAdapter) Name() string { return "OpenCode" }

func (a *openCodeAdapter) Detect(workspace string) Installation {
	dir := filepath.Join(workspace, ".opencode")
	if _, err := os.Stat(dir); err == nil {
		return Installation{Found: true, Path: dir}
	}
	return Installation{}
}

func (a *openCodeAdapter) Supports(concept Concept) Compatibility {
	return Compatibility{Supported: true}
}

func (a *openCodeAdapter) Alias(_ Concept) string { return "" }

func (a *openCodeAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	files := []FileWrite{
		{Concept: ConceptRules, Path: "AGENTS.md", Content: []byte(c.Rules)},
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
			Path:    filepath.Join(".opencode", "skills", skill.Dir, "SKILL.md"),
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
			Path:    filepath.Join(".opencode", "agents", agent.Filename+".md"),
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
			Path:    filepath.Join(".opencode", "commands", cmd.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
