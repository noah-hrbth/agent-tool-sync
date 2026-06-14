package tools

import (
	"os"
	"path/filepath"
)

// DetectAtScope reports whether an existing installation of the tool is present
// at the given scope. ScopeUser delegates to the tool's Meta.Detect probe;
// ScopeProject stats each derived import source (dirs, root files, detect-only
// files) under the workspace root and returns the first hit as an absolute path.
func DetectAtScope(ws string, t Tool, scope Scope) Installation {
	if scope == ScopeUser {
		return t.Meta.Detect(ws)
	}
	sources, err := DeriveImportSources(t, scope)
	if err != nil {
		return Installation{}
	}
	candidates := make([]string, 0, len(sources.Dirs)+len(sources.RootFiles)+len(sources.DetectFiles))
	candidates = append(candidates, sources.Dirs...)
	candidates = append(candidates, sources.RootFiles...)
	candidates = append(candidates, sources.DetectFiles...)
	for _, rel := range candidates {
		abs := filepath.Join(ws, rel)
		if _, err := os.Stat(abs); err == nil {
			return Installation{Found: true, Path: abs}
		}
	}
	return Installation{}
}

// ImportEligible reports whether importing from the tool at the given scope can
// produce any canonical entity: true iff at least one probe-rendered path has a
// reversible adopt outcome (own concept, root memory, or cross-mapped).
// Unsupported scopes and render failures are ineligible.
func ImportEligible(t Tool, scope Scope) bool {
	if !t.Meta.SupportsScope(scope).Supported {
		return false
	}
	writes, err := t.Render(probeCanonical(), scope)
	if err != nil {
		return false
	}
	for _, fw := range writes {
		if ExpectedAdoptOutcome(t.Meta.Key, fw.Concept, fw.Path).Kind != OutcomeNonReversible {
			return true
		}
	}
	return false
}
