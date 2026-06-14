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

// Command returns the agentsync invocation that operates at this scope.
func (s Scope) Command() string {
	if s == ScopeUser {
		return "agentsync --global"
	}
	return "agentsync"
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

// DetectFunc reports whether the tool is installed for the given workspace.
type DetectFunc func(workspace string) Installation

// RenderFunc produces the set of files to write for a tool given the canonical
// source and target scope. Paths are relative to the scope's base directory
// (workspace root for ScopeProject, user home for ScopeUser).
type RenderFunc func(c *canonical.Canonical, scope Scope) ([]FileWrite, error)

// ToolMeta is the static data descriptor for a supported AI tool: the single
// source of truth for everything about the tool except how it renders. The
// concept/scope support matrix is visible at a glance in each literal.
type ToolMeta struct {
	Key         string                    // stable id, e.g. "claude"
	Name        string                    // human-readable, e.g. "Claude Code"
	Detect      DetectFunc                // installation probe
	Aliases     map[Concept]string        // display filename when it differs from canonical; nil/missing => none
	Concepts    map[Concept]Compatibility // all 4 concepts listed explicitly
	Scopes      map[Scope]Compatibility   // both scopes listed explicitly
	ConceptInfo map[Concept]string        // concept-specific help text shown in the TUI
}

// Supports reports concept compatibility. A concept missing from the map is
// treated as unsupported; every tool lists all concepts explicitly so the
// matrix stays fully visible in the literal.
func (m ToolMeta) Supports(concept Concept) Compatibility { return m.Concepts[concept] }

// SupportsScope reports whether the tool has a file-based config layer for the
// given scope. Tools with no user-level file location return Supported: false.
func (m ToolMeta) SupportsScope(scope Scope) Compatibility { return m.Scopes[scope] }

// Alias returns the display filename for the tool's per-concept output when it
// differs from the canonical name, or "" when no alias applies.
func (m ToolMeta) Alias(concept Concept) string { return m.Aliases[concept] }

// Info returns a short, concept-specific description of where this tool
// reads/writes files and any noteworthy mapping behavior, or "" when there is
// nothing extra worth surfacing beyond the badge state.
func (m ToolMeta) Info(concept Concept) string { return m.ConceptInfo[concept] }

// Tool is a supported AI tool: static metadata plus a render function. Tool is
// a pure value — read metadata via Meta, invoke rendering via Render.
type Tool struct {
	Meta   ToolMeta
	Render RenderFunc
}

// detectGlobalDir returns a DetectFunc reporting installation when ~/.<name> exists.
func detectGlobalDir(name string) DetectFunc {
	return func(string) Installation {
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
}

// detectConfigDir returns a DetectFunc reporting installation when
// ~/.config/<name>/ exists (XDG-style).
func detectConfigDir(name string) DetectFunc {
	return func(string) Installation {
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
}
