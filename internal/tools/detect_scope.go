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
	neutral := neutralRootMemoryFiles(scope)
	candidates := make([]string, 0, len(sources.Dirs)+len(sources.RootFiles)+len(sources.DetectFiles))
	candidates = append(candidates, sources.Dirs...)
	for _, rf := range sources.RootFiles {
		// a root file emitted by >1 tool (e.g. the bare shared AGENTS.md) cannot
		// identify THIS tool, so it must not count as a detection signal
		if neutral[rf] {
			continue
		}
		candidates = append(candidates, rf)
	}
	candidates = append(candidates, sources.DetectFiles...)
	for _, rel := range candidates {
		abs := filepath.Join(ws, rel)
		if _, err := os.Stat(abs); err == nil {
			return Installation{Found: true, Path: abs}
		}
	}
	return Installation{}
}

// neutralRootMemoryFiles returns the project-scope root-memory paths emitted by
// more than one tool at scope. Such a file (notably the bare shared AGENTS.md)
// cannot distinguish which tool wrote it, so DetectAtScope must not treat its
// presence as a positive detection signal for any single tool.
func neutralRootMemoryFiles(scope Scope) map[string]bool {
	counts := map[string]int{}
	for _, t := range All() {
		src, err := DeriveImportSources(t, scope)
		if err != nil {
			continue
		}
		for _, rf := range src.RootFiles {
			counts[rf]++
		}
	}
	neutral := map[string]bool{}
	for f, n := range counts {
		if n > 1 {
			neutral[f] = true
		}
	}
	return neutral
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
