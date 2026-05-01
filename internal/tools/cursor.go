package tools

import (
	"fmt"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type cursorAdapter struct{}

func (a *cursorAdapter) Name() string { return "Cursor" }

func (a *cursorAdapter) Detect(_ string) Installation {
	return detectGlobalDir("cursor")
}

func (a *cursorAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptCommands:
		return Compatibility{
			Supported:   true,
			Deprecated:  true,
			Reason:      "Cursor promotes skills as the slash-command surface — prefer skills",
			Replacement: "skills",
		}
	default:
		return Compatibility{Supported: true}
	}
}

func (a *cursorAdapter) SupportsScope(scope Scope) Compatibility {
	if scope == ScopeUser {
		return Compatibility{Supported: false, Reason: "Cursor user rules are managed in the Settings UI, not files"}
	}
	return Compatibility{Supported: true}
}

func (a *cursorAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return "general.mdc"
	}
	return ""
}

func (a *cursorAdapter) Notice() string { return "" }

func (a *cursorAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	if scope == ScopeUser {
		return nil, nil
	}
	rulesContent := fmt.Sprintf("---\nalwaysApply: true\n---\n%s", c.AgentsMD)
	files := []FileWrite{
		{Concept: ConceptRules, Path: ".cursor/rules/general.mdc", Content: []byte(rulesContent)},
	}

	for _, r := range c.Rules {
		content := buildMDFrontmatter([]fmField{
			{key: "description", value: r.Description},
			{key: "globs", value: r.Paths},
			{key: "alwaysApply", value: false},
		}, r.Body)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    filepath.Join(".cursor", "rules", r.Filename+".mdc"),
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
