// TODO(opencode-adapter): Agent schema mismatch — we emit `name` and `tools` as flat frontmatter
// fields; OpenCode uses the filename as the agent name and wraps tool access in a `permission`
// object. Missing fields: `mode`, `temperature`, `top_p`, `steps`, `disable`, `hidden`, `color`.
//
// TODO(opencode-adapter): Commands schema mismatch — body should be nested under a `template` key;
// `argument-hint` and `allowed-tools` are Claude-isms that OpenCode does not recognise and ignores.
//
// TODO(opencode-adapter): Global detect path mismatch — `detectGlobalDir("opencode")` resolves to
// `~/.opencode/`; OpenCode follows the XDG standard and actually uses `~/.config/opencode/`.

package tools

import (
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type openCodeAdapter struct{}

func (a *openCodeAdapter) Name() string { return "OpenCode" }

func (a *openCodeAdapter) Detect(_ string) Installation {
	return detectGlobalDir("opencode")
}

func (a *openCodeAdapter) Supports(concept Concept) Compatibility {
	return Compatibility{Supported: true}
}

func (a *openCodeAdapter) Alias(_ Concept) string { return "" }

func (a *openCodeAdapter) Notice() string { return "" }

func (a *openCodeAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	files := []FileWrite{
		{Concept: ConceptRules, Path: filepath.Join(".opencode", "AGENTS.md"), Content: []byte(rootContent)},
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
