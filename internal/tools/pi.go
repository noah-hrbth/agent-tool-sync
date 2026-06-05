package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var piMeta = ToolMeta{
	Key:  "pi",
	Name: "Pi (CLI)",
	// Detect probes ~/.pi/ first (documented default), then ~/.config/pi/ as fallback
	Detect: func(ws string) Installation {
		if inst := detectGlobalDir("pi")(ws); inst.Found {
			return inst
		}
		return detectConfigDir("pi")(ws)
	},
	Concepts: map[Concept]Compatibility{
		ConceptRules:    {Supported: true},
		ConceptSkills:   {Supported: true},
		ConceptAgents:   {Supported: false, Reason: "Pi has no file-defined sub-agents"},
		ConceptCommands: {Supported: true},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: true},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Root memory at AGENTS.md (project) or ~/.pi/agent/AGENTS.md (user). Per-file rules append to AGENTS.md — Pi has no per-rule files.",
		ConceptSkills:   "Skills at .pi/skills/<dir>/SKILL.md (project) or ~/.pi/agent/skills/ (user). The paths field is dropped — Pi skills have no path scoping.",
		ConceptAgents:   "Pi has no file-defined sub-agents; agents spawn via the /spawn command and TypeScript extensions, not markdown.",
		ConceptCommands: "Prompts (= slash commands) at .pi/prompts/<name>.md (project) or ~/.pi/agent/prompts/ (user).",
	},
}

func renderPi(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	var files []FileWrite

	// Root memory: flattened rules into AGENTS.md
	// Project: AGENTS.md at workspace root
	// User: ~/.pi/agent/AGENTS.md
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	rootPath := "AGENTS.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(piDirUser, "AGENTS.md")
	}
	files = append(files, FileWrite{
		Concept: ConceptRules,
		Path:    rootPath,
		Content: []byte(rootContent),
	})

	// Determine base directory for scope-dependent paths
	baseDir := piDirProject
	if scope == ScopeUser {
		baseDir = piDirUser
	}

	// Skills: .pi/skills/<dir>/SKILL.md (project) or ~/.pi/agent/skills/<dir>/SKILL.md (user)
	// Pi skills support: name, description, allowed-tools, disable-model-invocation
	// Pi does NOT support the paths field — drop it
	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "allowed-tools", value: skill.AllowedTools},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
		}, skill.Body)
		skillDir := filepath.Join(baseDir, "skills", skill.Dir)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillDir, "SKILL.md"),
			Content: []byte(content),
		})
		files = appendSkillDocs(files, skillDir, skill.Docs)
	}

	// Commands (prompts): .pi/prompts/<name>.md (project) or ~/.pi/agent/prompts/<name>.md (user)
	// Pi prompts support: description, argument-hint
	// Pi does NOT support: allowed-tools, model — drop them
	for _, cmd := range c.Commands {
		content := buildMDFrontmatter([]fmField{
			{key: "description", value: cmd.Description},
			{key: "argument-hint", value: cmd.ArgumentHint},
		}, cmd.Body)
		promptsDir := filepath.Join(baseDir, "prompts")
		files = append(files, FileWrite{
			Concept: ConceptCommands,
			Path:    filepath.Join(promptsDir, cmd.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
