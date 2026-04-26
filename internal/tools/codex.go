package tools

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type codexAdapter struct{}

func (a *codexAdapter) Name() string { return "Codex CLI" }

func (a *codexAdapter) Detect(workspace string) Installation {
	dir := filepath.Join(workspace, ".codex")
	if _, err := os.Stat(dir); err == nil {
		return Installation{Found: true, Path: dir}
	}
	if path, err := exec.LookPath("codex"); err == nil {
		return Installation{Found: true, Path: path}
	}
	return Installation{}
}

func (a *codexAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules, ConceptSkills, ConceptAgents:
		return Compatibility{Supported: true}
	case ConceptCommands:
		return Compatibility{
			Supported:  true,
			Deprecated: true,
			Reason:     "legacy prompts deprecated — prefer skills",
		}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *codexAdapter) Alias(_ Concept) string { return "" }

func (a *codexAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	files := []FileWrite{
		{Concept: ConceptRules, Path: "AGENTS.md", Content: []byte(c.Rules)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".agents", "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	for _, agent := range c.Agents {
		content := buildTOML([]fmField{
			{key: "name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "developer_instructions", value: agent.Body},
			{key: "model", value: agent.Model},
		})
		files = append(files, FileWrite{
			Concept: ConceptAgents,
			Path:    filepath.Join(".codex", "agents", agent.Filename+".toml"),
			Content: []byte(content),
		})
	}

	// Commands are deprecated for Codex CLI; not rendered.

	return files, nil
}
