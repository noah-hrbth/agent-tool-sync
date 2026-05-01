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

// Scope is the directory tree the canonical source maps onto. ScopeProject syncs
// to <workspace>/.<tool>/...; ScopeUser syncs to <home>/.<tool>/... (or whatever
// user-level path each tool actually reads).
type Scope int

const (
	ScopeProject Scope = iota
	ScopeUser
)

// String returns the human label for a scope.
func (s Scope) String() string {
	if s == ScopeUser {
		return "user"
	}
	return "project"
}

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

	// SupportsScope reports whether the tool has a file-based config layer for the
	// given scope. Adapters with no user-level file location (Cursor's user rules
	// are UI-managed; Zed has no global rules file) return Supported: false here.
	SupportsScope(scope Scope) Compatibility

	// Render produces the set of files to write for this tool given the canonical
	// source and target scope. Paths are relative to the scope's base directory
	// (workspace root for ScopeProject, user home for ScopeUser).
	Render(c *canonical.Canonical, scope Scope) ([]FileWrite, error)

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
