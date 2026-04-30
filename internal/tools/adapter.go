package tools

import (
	"os"
	"path/filepath"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

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
	Supported   bool
	Reason      string // non-empty when not supported; shown in TUI badge
	Deprecated  bool   // vendor recommends a successor concept; not rendered
	Replacement string // concept name that supersedes this one when Deprecated
}

// Installation reports whether a tool is installed via the user's global config dir (`~/.<tool>`).
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

	// Detect reports whether the tool is installed via the user's global config dir (`~/.<tool>`).
	Detect(workspace string) Installation

	// Supports reports whether the tool has a native concept for category.
	Supports(concept Concept) Compatibility

	// Render produces the set of workspace-relative files to write for this tool
	// given the canonical source. Paths are relative to workspace root.
	Render(c *canonical.Canonical) ([]FileWrite, error)

	// Alias returns the display filename for the tool's per-concept output when it
	// differs from the canonical name. Returns an empty string when no alias applies.
	Alias(concept Concept) string

	// Notice returns an optional informational note about this tool to display in the
	// TUI tools screen (e.g. a non-obvious path split). Returns an empty string when
	// there is nothing noteworthy.
	Notice() string
}

// detectGlobalDir reports installation when ~/.<name> exists.
func detectGlobalDir(name string) Installation {
	home, err := os.UserHomeDir()
	if err != nil {
		return Installation{}
	}
	dir := filepath.Join(home, "."+name)
	if _, err := os.Stat(dir); err == nil {
		return Installation{Found: true, Path: dir}
	}
	return Installation{}
}

// detectConfigDir reports installation when ~/.config/<name>/ exists (XDG-style).
func detectConfigDir(name string) Installation {
	home, err := os.UserHomeDir()
	if err != nil {
		return Installation{}
	}
	dir := filepath.Join(home, ".config", name)
	if _, err := os.Stat(dir); err == nil {
		return Installation{Found: true, Path: dir}
	}
	return Installation{}
}
