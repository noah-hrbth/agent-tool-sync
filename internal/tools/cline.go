package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var clineMeta = ToolMeta{
	Key:  "cline",
	Name: "Cline",
	// Detect reports installation via ~/.cline/, the root Cline uses for skills and
	// runtime state. Cline's other user-scope tree (~/Documents/Cline/ for rules and
	// workflows) is not used for detection because rule/workflow directories may be
	// absent on a fresh install while ~/.cline/ is created on first run.
	Detect: detectGlobalDir("cline"),
	Concepts: map[Concept]Compatibility{
		ConceptRules:    {Supported: true},
		ConceptSkills:   {Supported: true},
		ConceptAgents:   {Supported: false, Reason: "Cline has no file-defined sub-agents"},
		ConceptCommands: {Supported: true},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at AGENTS.md (project root). Per-file rules at .clinerules/<name>.md (project) or ~/Documents/Cline/Rules/<name>.md (user).",
		ConceptSkills:   "Skills at .cline/skills/<dir>/SKILL.md (project) or ~/.cline/skills/ (user) — Cline does NOT read user skills from Documents/Cline/.",
		ConceptAgents:   "Cline has no file-defined sub-agents.",
		ConceptCommands: "Workflows (= slash commands) at .clinerules/workflows/<name>.md (project) or ~/Documents/Cline/Workflows/<name>.md (user).",
	},
}

func renderCline(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
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
