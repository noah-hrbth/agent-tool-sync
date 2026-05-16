package tools

import (
	"github.com/noah-hrbth/agentsync/internal/canonical"
)

var zedMeta = ToolMeta{
	Key:    "zed",
	Name:   "Zed",
	Detect: detectConfigDir("zed"),
	Aliases: map[Concept]string{
		ConceptRules: ".rules",
	},
	Concepts: map[Concept]Compatibility{
		ConceptRules:    {Supported: true},
		ConceptSkills:   {Supported: false, Reason: "Zed has no skills concept"},
		ConceptAgents:   {Supported: false, Reason: "Zed's agent_servers expects executable specs, not markdown system prompts"},
		ConceptCommands: {Supported: false, Reason: "Zed slash commands are WASM extensions, not file-defined"},
	},
	Scopes: map[Scope]Compatibility{
		ScopeProject: {Supported: true},
		ScopeUser:    {Supported: false, Reason: "Zed has no global rules file — only project-root .rules"},
	},
	ConceptInfo: map[Concept]string{
		ConceptRules:    "Written to .rules at workspace root. Project scope only — Zed has no global rules file.",
		ConceptSkills:   "Zed has no skills concept.",
		ConceptAgents:   "Zed's agent_servers expects executable specs, not markdown system prompts.",
		ConceptCommands: "Zed slash commands are WASM extensions, not file-defined.",
	},
}

func renderZed(c *canonical.Canonical, scope Scope) ([]FileWrite, error) {
	if scope == ScopeUser {
		return nil, nil
	}
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	return []FileWrite{
		{Concept: ConceptRules, Path: ".rules", Content: []byte(rootContent)},
	}, nil
}
