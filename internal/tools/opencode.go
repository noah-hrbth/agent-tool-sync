// NOTE(opencode-adapter): `name` is emitted flat though OpenCode derives the agent name from the
// filename (redundant but harmless). Tool access renders as OpenCode's `tools` object — an allowlist
// via the deny-all sentinel `"*": false` plus the enabled tools (see openCodeAgentTools). OpenCode
// marks `tools` "deprecated, prefer permission", but it is the documented per-tool availability
// switch and the only field whose semantics match a Claude allowlist. The object form and the `"*"`
// glob sentinel are confirmed against OpenCode docs (config.mdx / agents.mdx), which key the object by
// tool name (built-ins, MCP globs like `my-mcp*`) without a fixed enum — so a Claude-only tool name
// that OpenCode does not know is a harmless no-op key, not a loader error.
//
// TODO(opencode-adapter): emit remaining optional agent fields when canonical gains them —
// `mode`, `temperature`, `top_p`, `steps`, `disable`, `hidden`, `color`.
//
// TODO(opencode-adapter): Commands schema mismatch — body should be nested under a `template` key;
// `argument-hint` and `allowed-tools` are Claude-isms that OpenCode does not recognise and ignores.

package tools

import (
	"path/filepath"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

// claudeToOpenCodeTool pins every first-class Claude tool name to its OpenCode
// key. Entries exist for two reasons: a real rename (LS→list, WebFetch→webfetch)
// or pure case-preservation (WebSearch→websearch) so CanonicalToolName can restore
// the original casing on adopt — without a pin the lowercase fallback is lossy
// (WebSearch→websearch→websearch, breaking the render↔adopt round-trip and the
// downstream Claude re-render). The map is bijective. A name absent here still
// falls back to lowercase, but that fallback is irreversible — keep new
// first-class tools listed.
var claudeToOpenCodeTool = map[string]string{
	"Read":         "read",
	"Write":        "write",
	"Edit":         "edit",
	"Bash":         "bash",
	"Grep":         "grep",
	"Glob":         "glob",
	"LS":           "list",
	"WebFetch":     "webfetch",
	"WebSearch":    "websearch",
	"Task":         "task",
	"TodoWrite":    "todowrite",
	"Skill":        "skill",
	"NotebookEdit": "notebookedit",
	"BashOutput":   "bashoutput",
	"SlashCommand": "slashcommand",
	"ExitPlanMode": "exitplanmode",
}

// OpenCodeToolName maps a canonical (Claude) tool name to OpenCode's tool name,
// lowercasing anything without an explicit mapping.
func OpenCodeToolName(claude string) string {
	if oc, ok := claudeToOpenCodeTool[claude]; ok {
		return oc
	}
	return strings.ToLower(claude)
}

// CanonicalToolName reverses OpenCodeToolName for explicitly-mapped tools. Names
// that were lowercase fallbacks (no explicit mapping) are returned unchanged.
func CanonicalToolName(openCode string) string {
	for claude, oc := range claudeToOpenCodeTool {
		if oc == openCode {
			return claude
		}
	}
	return openCode
}

// openCodeAgentTools converts a canonical allowlist into OpenCode's `tools`
// object: deny everything (`"*": false`), then re-enable each listed tool. An
// empty allowlist yields nil so no `tools` block is emitted (OpenCode then leaves
// every tool enabled, matching a Claude agent with no `tools` field).
func openCodeAgentTools(allowlist []string) fmYAMLMap {
	if len(allowlist) == 0 {
		return nil
	}
	m := fmYAMLMap{{key: "*", value: false}}
	seen := map[string]bool{}
	for _, t := range allowlist {
		oc := OpenCodeToolName(t)
		if seen[oc] {
			continue
		}
		seen[oc] = true
		m = append(m, fmYAMLMapEntry{key: oc, value: true})
	}
	return m
}

var openCodeMeta = ToolMeta{
	Key:  "opencode",
	Name: "OpenCode",
	Detect: func(ws string) Installation {
		if inst := detectConfigDir("opencode")(ws); inst.Found {
			return inst
		}
		return detectGlobalDir("opencode")(ws)
	},
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
		ConceptRules:    "Root memory at AGENTS.md (project) or ~/.config/opencode/AGENTS.md (user). Per-file rules append to AGENTS.md — OpenCode has no per-rule files.",
		ConceptSkills:   "Skills at .opencode/skills/<dir>/SKILL.md (project) or ~/.config/opencode/skills/ (user).",
		ConceptAgents:   "Subagents at .opencode/agents/<name>.md.",
		ConceptCommands: "Commands at .opencode/commands/<name>.md.",
	},
}

// openCodeBase returns the path prefix relative to the scope's base directory.
// Project scope: .opencode/. User scope: .config/opencode/ (OpenCode docs:
// "global rules in a ~/.config/opencode/AGENTS.md").
func openCodeBase(scope Scope) string {
	if scope == ScopeUser {
		return opencodeDirUser
	}
	return opencodeDirProject
}

func renderOpenCode(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	base := openCodeBase(scope)
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	// OpenCode reads AGENTS.md from the workspace root (and parent dirs) at project
	// scope; user scope reads from ~/.config/opencode/AGENTS.md.
	rootPath := "AGENTS.md"
	if scope == ScopeUser {
		rootPath = filepath.Join(base, "AGENTS.md")
	}
	files := []FileWrite{
		{Concept: ConceptRules, Path: rootPath, Content: []byte(rootContent)},
	}

	for _, skill := range c.Skills {
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: skill.Name},
			{key: "description", value: skill.Description},
			{key: "allowed-tools", value: skill.AllowedTools},
			{key: "disable-model-invocation", value: skill.DisableModelInvocation},
		}, skill.Body)
		skillDir := filepath.Join(base, "skills", skill.Dir)
		files = append(files, FileWrite{
			Concept: ConceptSkills,
			Path:    filepath.Join(skillDir, "SKILL.md"),
			Content: []byte(content),
		})
		files = appendSkillDocs(files, skillDir, skill.Docs)
	}

	for _, agent := range c.Agents {
		// translate canonical allowlist → OpenCode tools object: OpenCode requires an
		// object (toolName→bool), not the array — the array fails its config loader.
		// openCodeAgentTools emits the deny-all sentinel + enabled tools (see helper).
		content := buildMDFrontmatter([]fmField{
			{key: "name", value: agent.Name},
			{key: "description", value: agent.Description},
			{key: "model", value: agent.Model},
			{key: "tools", value: openCodeAgentTools(agent.Tools)},
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
