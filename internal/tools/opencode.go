// TODO(opencode-adapter): Agent schema mismatch — we emit `name` and `tools` as flat frontmatter
// fields; OpenCode uses the filename as the agent name and wraps tool access in a `permission`
// object. Missing fields: `mode`, `temperature`, `top_p`, `steps`, `disable`, `hidden`, `color`.
//
// TODO(opencode-adapter): Commands schema mismatch — body should be nested under a `template` key;
// `argument-hint` and `allowed-tools` are Claude-isms that OpenCode does not recognise and ignores.

package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type openCodeAdapter struct{}

func (a *openCodeAdapter) Name() string { return "OpenCode" }

func (a *openCodeAdapter) Detect(_ string) Installation {
	if inst := detectConfigDir("opencode"); inst.Found {
		return inst
	}
	return detectGlobalDir("opencode")
}

func (a *openCodeAdapter) Supports(concept Concept) Compatibility {
	return Compatibility{Supported: true}
}

func (a *openCodeAdapter) SupportsScope(_ Scope) Compatibility {
	return Compatibility{Supported: true}
}

func (a *openCodeAdapter) Alias(_ Concept) string { return "" }

func (a *openCodeAdapter) Notice() string { return "" }

// openCodeBase returns the path prefix relative to the scope's base directory.
// Project scope: .opencode/. User scope: .config/opencode/ (OpenCode docs:
// "global rules in a ~/.config/opencode/AGENTS.md").
func openCodeBase(scope Scope) string {
	if scope == ScopeUser {
		return filepath.Join(".config", "opencode")
	}
	return ".opencode"
}

func (a *openCodeAdapter) Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	base := openCodeBase(scope)
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	files := []FileWrite{
		{Concept: ConceptRules, Path: filepath.Join(base, "AGENTS.md"), Content: []byte(rootContent)},
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
			Path:    filepath.Join(base, "skills", skill.Dir, "SKILL.md"),
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
			Path:    filepath.Join(base, "agents", agent.Filename+".md"),
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
			Path:    filepath.Join(base, "commands", cmd.Filename+".md"),
			Content: []byte(content),
		})
	}

	return files, nil
}
