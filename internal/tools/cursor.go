package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type cursorAdapter struct{}

func (a *cursorAdapter) Name() string { return "Cursor" }

func (a *cursorAdapter) Detect(workspace string) Installation {
	dir := filepath.Join(workspace, ".cursor")
	if _, err := os.Stat(dir); err == nil {
		return Installation{Found: true, Path: dir}
	}
	return Installation{}
}

func (a *cursorAdapter) Supports(_ Concept) Compatibility {
	return Compatibility{Supported: true}
}

func (a *cursorAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "general.mdc"
	}
	return ""
}

func (a *cursorAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	rulesContent := fmt.Sprintf("---\nalwaysApply: true\ndescription: Synced by agentsync\n---\n%s", c.Rules)
	files := []FileWrite{
		{Concept: ConceptRules, Path: ".cursor/rules/general.mdc", Content: []byte(rulesContent)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".cursor", "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	for _, agent := range c.Agents {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "model", value: agent.Model},
		}, agent.Body)
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(".cursor", "agents", agent.Filename+".md"),
			Content: []byte(content),
		})
	}

	for _, cmd := range c.Commands {
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(".cursor", "commands", cmd.Filename+".md"),
			Content: []byte(cmd.Body),
		})
	}

	return files, nil
}
