package tools

import (
	"github.com/noah-hrbth/agentsync/internal/canonical"
)

type zedAdapter struct{}

func (a *zedAdapter) Name() string { return "Zed" }

func (a *zedAdapter) Detect(_ string) Installation {
	return detectConfigDir("zed")
}

func (a *zedAdapter) Supports(concept Concept) Compatibility {
	switch concept {
	case ConceptRules:
		return Compatibility{Supported: true}
	case ConceptSkills:
		return Compatibility{Supported: false, Reason: "Zed has no skills concept"}
	case ConceptAgents:
		return Compatibility{Supported: false, Reason: "Zed's agent_servers expects executable specs, not markdown system prompts"}
	case ConceptCommands:
		return Compatibility{Supported: false, Reason: "Zed slash commands are WASM extensions, not file-defined"}
	default:
		return Compatibility{Supported: false}
	}
}

func (a *zedAdapter) Alias(concept Concept) string {
	if concept == ConceptRules {
		return ".rules"
	}
	return ""
}

func (a *zedAdapter) Notice() string {
	return "rules are written to .rules at workspace root; skills/agents/commands are not supported by Zed"
}

func (a *zedAdapter) Render(c *canonical.Canonical) ([]FileWrite, error) {
	rootContent := buildRootMemoryContent(c.AgentsMD, c.Rules)
	return []FileWrite{
		{Concept: ConceptRules, Path: ".rules", Content: []byte(rootContent)},
	}, nil
}
