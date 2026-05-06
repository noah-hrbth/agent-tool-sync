package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type clineAdapter struct{}

func (a *clineAdapter) Name() string { return "Cline" }

// Detect reports installation via ~/.cline/, the root Cline uses for skills and
// runtime state. Cline's other user-scope tree (~/Documents/Cline/ for rules and
// workflows) is not used for detection because rule/workflow directories may be
// absent on a fresh install while ~/.cline/ is created on first run.
func (a *clineAdapter) Detect(_ string) Installation {
	return detectGlobalDir("cline")
}

func (a *clineAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules, ConceptSkills, ConceptCommands:
		return Compatibility{Supported: true}
	case ConceptAgents:
		return Compatibility{Supported: false, Reason: "Cline has no file-defined sub-agents"}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *clineAdapter) SupportsScope(_ Scope) Compatibility {
	return Compatibility{Supported: true}
}

func (a *clineAdapter) Alias(_ Concept) string { return "" }

func (a *clineAdapter) Notice() string {
	return "rules: .clinerules/<name>.md (or AGENTS.md at root); skills: .cline/skills/ (project AND user — Cline reads skills from ~/.cline/skills/); workflows: .clinerules/workflows/; user-scope rules and workflows live under ~/Documents/Cline/"
}

func (a *clineAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	var files []FileWrite

	// Root memory: project scope only. Cline auto-detects AGENTS.md at the workspace
	// root. We use buildRootMemoryContent so the shared root path stays consistent
	// with OpenCode/Codex/Junie (last-writer-wins is harmless when content matches).
	if scope == ScopeProject {
		rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    "AGENTS.md",
			Content: []byte(rootContent),
		})
	}

	// Per-rule files. Project: .clinerules/<name>.md. User: Documents/Cline/Rules/<name>.md.
	rulesDir := ".clinerules"
	if scope == ScopeUser {
		rulesDir = filepath.Join("Documents", "Cline", "Rules")
	}
	for _, r := range c.Rules {
		content := buildMDFrontmatter([]fmField{
			{key: "paths", value: r.Paths},
		}, r.Body)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    filepath.Join(rulesDir, r.Filename+".md"),
			Content: []byte(content),
		})
	}

	// Skills live at .cline/skills/ at both scopes (project: <ws>/.cline/, user: ~/.cline/).
	// Cline uses two user-scope trees: ~/.cline/ holds skills + runtime state,
	// while ~/Documents/Cline/ holds rules and workflows. Skills do NOT move to
	// Documents/Cline/Skills/ — the docs at https://docs.cline.bot/customization/skills
	// are explicit that ~/.cline/skills/ is where Cline reads user-level skills.
	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
		}, skill.Body)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(".cline", "skills", skill.Dir, "SKILL.md"),
			Content: []byte(content),
		})
	}

	// Workflows = slash commands. Project: .clinerules/workflows/. User: Documents/Cline/Workflows/.
	workflowsDir := filepath.Join(".clinerules", "workflows")
	if scope == ScopeUser {
		workflowsDir = filepath.Join("Documents", "Cline", "Workflows")
	}
	for _, cmd := range c.Commands {
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(workflowsDir, cmd.Filename+".md"),
			Content: []byte(cmd.Body),
		})
	}

	return files, nil
}
