package tools

import (
	"path/filepath"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var copilotMeta = ToolMeta{
	Key:    "copilot",
	Name:   "GitHub Copilot",
	Detect: detectGlobalDir("copilot"),
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
		ConceptRules:    "Root memory at .github/copilot-instructions.md (project) or ~/.copilot/copilot-instructions.md (user). Per-file rules at .github/instructions/<name>.instructions.md with applyTo: glob — agentsync brace-expands multiple Paths into a single glob.",
		ConceptSkills:   "Skills at .github/skills/<dir>/SKILL.md (project) or ~/.copilot/skills/ (user). Skill directory name must match the skill's name.",
		ConceptAgents:   "Subagents at .github/agents/<name>.agent.md (project) or ~/.copilot/agents/ (user). Legacy .chatmode.md is not emitted — Copilot renamed chat modes to custom agents.",
		ConceptCommands: "Prompts at .github/prompts/<name>.prompt.md (project only — Copilot has no documented user-level prompts directory; commands are not synced at user scope).",
	},
}

func renderCopilot(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	var files []FileWrite

	// Root memory: plain Markdown body, no frontmatter. Copilot's dedicated path
	// (.github/copilot-instructions.md at project, ~/.copilot/copilot-instructions.md
	// at user) avoids double-loading when other adapters emit root AGENTS.md.
	if c.AgentsMD != "" {
		rootPath := copilotDirProject + "/copilot-instructions.md"
		if scope == ScopeUser {
			rootPath = copilotDirUser + "/copilot-instructions.md"
		}
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    rootPath,
			Content: []byte(c.AgentsMD),
		})
	}

	// Per-rule instructions files. Copilot's applyTo is a single glob string —
	// brace-expand multi-path canonical Rules into one glob; omit when no paths.
	rulesDir := filepath.Join(copilotDirProject, "instructions")
	if scope == ScopeUser {
		rulesDir = filepath.Join(copilotDirUser, "instructions")
	}
	for _, r := range c.Rules {
		content := buildMDFrontmatter([]fmField{
			{key: "applyTo", value: translateApplyTo(r.Paths)},
			{key: "description", value: r.Description},
		}, r.Body)
		files = append(files, FileWrite{
			Concept: ConceptRules,
			Path:    filepath.Join(rulesDir, r.Filename+".instructions.md"),
			Content: []byte(content),
		})
	}

	// Skills. Copilot enforces "skill name == parent dir name"; emit Skill.Dir
	// as name to make the invariant explicit at render time. Adoption via the
	// generic matchSkillPath branch parses name: back into Skill.Name — benign
	// because canonical already normalizes Skill.Name == Skill.Dir on save.
	skillsDir := filepath.Join(copilotDirProject, "skills")
	if scope == ScopeUser {
		skillsDir = filepath.Join(copilotDirUser, "skills")
	}
	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Dir},
			{key: "description", value: skill.Description},
		}, skill.Body)
		skillDir := filepath.Join(skillsDir, skill.Dir)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillDir, "SKILL.md"),
			Content: []byte(content),
		})
		files = appendSkillDocs(files, skillDir, skill.Docs)
	}

	// Agents. The .agent.md suffix is mandatory — Copilot renamed legacy
	// .chatmode.md to .agent.md (custom agents). We never emit the legacy form.
	agentsDir := filepath.Join(copilotDirProject, "agents")
	if scope == ScopeUser {
		agentsDir = filepath.Join(copilotDirUser, "agents")
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
			Path:    filepath.Join(agentsDir, agent.Filename+".agent.md"),
			Content: []byte(content),
		})
	}

	// Commands (prompt files). Project scope only — Copilot has no documented
	// user-level prompts directory. canonical AllowedTools maps to Copilot's
	// `tools:` field.
	if scope == ScopeProject {
		for _, cmd := range c.Commands {
			content := buildMDFrontmatter([]fmField{
				{key: "description", value: cmd.Description},
				{key: "argument-hint", value: cmd.ArgumentHint},
				{key: "tools", value: cmd.AllowedTools},
				{key: "model", value: cmd.Model},
			}, cmd.Body)
			files = append(files, FileWrite{
				Concept: ConceptCommands,
				Path:    filepath.Join(copilotDirProject, "prompts", cmd.Filename+".prompt.md"),
				Content: []byte(content),
			})
		}
	}

	return files, nil
}

// translateApplyTo collapses a canonical Rule.Paths slice into a single Copilot
// applyTo glob: "" (omit), the one path verbatim, or brace-expanded {a,b,c}.
func translateApplyTo(paths []string) string {
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return paths[0]
	default:
		return "{" + strings.Join(paths, ",") + "}"
	}
}
