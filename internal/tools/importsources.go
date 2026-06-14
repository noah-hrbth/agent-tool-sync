package tools

import (
	"path"
	"sort"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
)

// probeMarker is the name prefix used by the synthetic probe canonical. Any
// rendered path segment containing it identifies an entity-derived location.
// Never "general" — that filename is reserved (Cursor's catch-all).
const probeMarker = "agentsync-probe"

// ImportSources describes where an existing installation of one tool keeps
// importable configuration at one scope. All paths are relative to the scope's
// base directory (workspace root for ScopeProject, user home for ScopeUser).
type ImportSources struct {
	ToolKey     string
	Dirs        []string // deduped, sorted walkable concept dirs (e.g. ".claude/skills")
	RootFiles   []string // importable root-memory candidates, in rootMemoryFiles() order (catch-all last)
	DetectFiles []string // exact rendered files used for detection only, never imported (e.g. Zed ".rules")
}

// DeriveImportSources derives a tool's on-disk import locations at the given
// scope by rendering a synthetic probe canonical and reverse-reading the
// emitted paths. Unsupported scopes yield empty sources and a nil error; a
// render failure is returned as-is.
func DeriveImportSources(t Tool, scope Scope) (ImportSources, error) {
	sources := ImportSources{ToolKey: t.Meta.Key}
	if !t.Meta.SupportsScope(scope).Supported {
		return sources, nil
	}

	writes, err := t.Render(probeCanonical(), scope)
	if err != nil {
		return ImportSources{}, err
	}

	dirSet := map[string]bool{}
	rootSet := map[string]bool{}
	for _, fw := range writes {
		if dir, ok := dirBeforeProbeSegment(fw.Path); ok {
			dirSet[dir] = true
			continue
		}
		if isRootMemoryPath(fw.Path) {
			rootSet[fw.Path] = true
			continue
		}
		sources.DetectFiles = append(sources.DetectFiles, fw.Path)
	}

	unionAdoptAlternates(dirSet, rootSet)
	sources.Dirs = sortedDirSet(dirSet)
	sources.RootFiles = orderedRootFiles(rootSet)
	return sources, nil
}

// unionAdoptAlternates widens render-derived sources with locations adopt.go
// reverses but the current render no longer emits (e.g. deprecated
// .claude/commands) and root-memory alternates under the same tool root (e.g.
// .claude/CLAUDE.md alongside CLAUDE.md). Matching is full-prefix equality on
// the parent dir — NEVER first-segment — so e.g. Pi's user-scope
// ".pi/agent/skills" (parent ".pi/agent") stays out of project sources whose
// derived root is ".pi".
func unionAdoptAlternates(dirSet, rootSet map[string]bool) {
	roots := derivedRoots(dirSet)

	var adoptPrefixes []string
	adoptPrefixes = append(adoptPrefixes, SkillDirPrefixes()...)
	adoptPrefixes = append(adoptPrefixes, AgentDirPrefixes()...)
	adoptPrefixes = append(adoptPrefixes, OpenCodeAgentDirPrefixes()...)
	adoptPrefixes = append(adoptPrefixes, CommandDirPrefixes()...)
	for _, prefix := range adoptPrefixes {
		dir := strings.TrimSuffix(prefix, "/")
		if roots[path.Dir(dir)] {
			dirSet[dir] = true
		}
	}

	for _, rm := range rootMemoryFiles() {
		parent := path.Dir(rm)
		// bare root files (parent ".") qualify only when actually rendered
		if parent == "." {
			continue
		}
		if roots[parent] {
			rootSet[rm] = true
		}
	}
}

// derivedRoots maps render-derived concept dirs to the tool root dirs they
// live under: a dir ending in a concept sub-dir contributes its parent
// (".claude/skills" → ".claude"); any other dir counts as its own root
// (".clinerules" → ".clinerules").
func derivedRoots(dirSet map[string]bool) map[string]bool {
	roots := map[string]bool{}
	for dir := range dirSet {
		switch path.Base(dir) {
		case skillsSub, agentsSub, commandsSub:
			roots[path.Dir(dir)] = true
		default:
			roots[dir] = true
		}
	}
	return roots
}

// probeCanonical builds the synthetic canonical used to derive import sources:
// one rule, one skill (+one doc), one agent, one command, plus AGENTS.md
// content, all named with the probeMarker prefix so rendered paths reveal
// where each concept lands. Entity shapes/values mirror the render-safe probe
// in internal/syncer/contract_test.go.
func probeCanonical() *canonical.Canonical {
	return &canonical.Canonical{
		AgentsMD: "# agentsync-probe\n\nRoot memory body.\n",
		Rules: []*canonical.Rule{{
			Filename: "agentsync-probe-rule", Description: "probe rule",
			Paths: []string{"src/**/*.ts", "test/**/*.ts"}, Body: "Rule body.\n",
		}},
		Skills: []*canonical.Skill{{
			Dir: "agentsync-probe-skill", Name: "agentsync-probe-skill",
			Description: "probe skill", Body: "Skill instructions.\n",
			Docs: []canonical.SkillDoc{{RelPath: "agentsync-probe-reference.md", Content: "# reference\n"}},
		}},
		Agents: []*canonical.Agent{{
			Filename: "agentsync-probe-agent", Name: "agentsync-probe-agent",
			Description: "probe agent", Tools: []string{"Read"},
			Model: "sonnet", Body: "Agent prompt.\n",
		}},
		Commands: []*canonical.Command{{
			Filename: "agentsync-probe-command", Description: "probe command",
			ArgumentHint: "[arg]", Body: "Command body.\n",
		}},
	}
}

// dirBeforeProbeSegment truncates path before the first segment containing the
// probe marker, yielding the walkable concept dir the entity rendered into.
// Reports false when no segment carries the marker (root memory / fixed files).
func dirBeforeProbeSegment(path string) (string, bool) {
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		if !strings.Contains(seg, probeMarker) {
			continue
		}
		// i == 0 would mean a probe file at the base root; no tool renders that
		if i == 0 {
			return "", false
		}
		return strings.Join(segments[:i], "/"), true
	}
	return "", false
}

// isRootMemoryPath reports whether path is an adopt-recognized root-memory
// location (rootMemoryFiles list or Cursor's catch-all).
func isRootMemoryPath(path string) bool {
	for _, rm := range rootMemoryFiles() {
		if path == rm {
			return true
		}
	}
	return path == cursorCatchAll
}

// sortedDirSet flattens a dir set into a deduped, sorted slice (nil when empty).
func sortedDirSet(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	dirs := make([]string, 0, len(set))
	for dir := range set {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

// orderedRootFiles flattens a root-file set into rootMemoryFiles() list order,
// with Cursor's catch-all last (nil when empty).
func orderedRootFiles(set map[string]bool) []string {
	var files []string
	for _, rm := range rootMemoryFiles() {
		if set[rm] {
			files = append(files, rm)
		}
	}
	if set[cursorCatchAll] {
		files = append(files, cursorCatchAll)
	}
	return files
}
