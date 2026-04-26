package tools

import "github.com/noah-hrbth/agentsync/internal/canonical"

// Concept is a category of AI tool configuration.
type Concept string

const (
	ConceptRules    Concept = "rules"
	ConceptSkills   Concept = "skills"
	ConceptAgents   Concept = "agents"
	ConceptCommands Concept = "commands"
)

// Compatibility reports whether a tool supports a given concept.
type Compatibility struct {
	Supported  bool
	Reason     string // non-empty when not supported; shown in TUI badge
	Deprecated bool   // vendor recommends a successor concept; not rendered
}

// Installation reports whether a tool appears to be installed in the workspace.
type Installation struct {
	Found bool
	Path  string // detected folder or binary path
}

// FileWrite is a file to be written, with a path relative to the workspace root.
type FileWrite struct {
	Concept Concept
	Path    string
	Content []byte
}

// Adapter is implemented by each supported AI tool.
type Adapter interface {
	// Name returns the human-readable tool name (e.g. "Claude Code").
	Name() string

	// Detect reports whether the tool appears installed in workspace.
	Detect(workspace string) Installation

	// Supports reports whether the tool has a native concept for category.
	Supports(concept Concept) Compatibility

	// Render produces the set of workspace-relative files to write for this tool
	// given the canonical source. Paths are relative to workspace root.
	Render(c *canonical.Canonical) ([]FileWrite, error)

	// Alias returns the display filename for the tool's per-concept output when it
	// differs from the canonical name. Returns an empty string when no alias applies.
	Alias(concept Concept) string
}
