package syncer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// SkippedFile records one file the import engine examined but did not adopt,
// with a human-readable reason.
type SkippedFile struct {
	Path   string
	Reason string
}

// ImportSummary aggregates what one bulk import wrote to canonical: per-concept
// adoption counts (keyed by what adoptExternal actually wrote, not by source
// dir) plus every file that was examined and skipped.
type ImportSummary struct {
	RootMemoryFrom string // source path of the imported root memory, "" when none
	Rules          int
	Skills         int
	SkillDocs      int
	Agents         int
	Commands       int
	Skipped        []SkippedFile
}

// ImportFromSources walks the given on-disk locations under base and adopts
// every mappable file into canonical .agentsync/. Per-file problems (no
// canonical mapping, parse errors) are recorded as Skipped and never abort the
// import; only infrastructure failures (a dir walk erroring on I/O) return an
// error.
func ImportFromSources(base string, src tools.ImportSources) (ImportSummary, error) {
	var summary ImportSummary
	importRootFiles(base, src.RootFiles, &summary)
	entries, err := collectSourceFiles(base, src.Dirs)
	if err != nil {
		return summary, importError(src.ToolKey, err)
	}
	claimed := stringSet(src.RootFiles)
	for _, e := range entries {
		if claimed[e.relPath] {
			continue // already handled in the root-files pass (e.g. Cursor's catch-all)
		}
		importSourceFile(base, e, &summary)
	}
	return summary, nil
}

// importError wraps an infrastructure failure with the source tool's key.
func importError(toolKey string, err error) error {
	if toolKey == "" {
		return err
	}
	return fmt.Errorf("import %s: %w", toolKey, err)
}

// importRootFiles adopts the first existing root-memory candidate (slice order
// is priority order) and records every later existing candidate as skipped.
func importRootFiles(base string, rootFiles []string, summary *ImportSummary) {
	for _, rf := range rootFiles {
		info, err := os.Lstat(filepath.Join(base, filepath.FromSlash(rf)))
		if err != nil {
			continue // absent candidate → try the next one
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			summary.Skipped = append(summary.Skipped, SkippedFile{Path: rf, Reason: "symlink, not followed"})
			continue
		}
		if summary.RootMemoryFrom != "" {
			summary.Skipped = append(summary.Skipped, SkippedFile{
				Path:   rf,
				Reason: "root memory already imported from " + summary.RootMemoryFrom,
			})
			continue
		}
		if _, err := adoptExternal(base, rf); err != nil {
			summary.Skipped = append(summary.Skipped, SkippedFile{Path: rf, Reason: adoptSkipReason(err)})
			continue
		}
		summary.RootMemoryFrom = rf
	}
}

// stringSet builds a membership set from a slice.
func stringSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

// ImportFromTool derives t's on-disk import locations at scope and imports
// everything found under base. See ImportFromSources for the engine semantics.
func ImportFromTool(base string, t tools.Tool, scope tools.Scope) (ImportSummary, error) {
	src, err := tools.DeriveImportSources(t, scope)
	if err != nil {
		return ImportSummary{}, err
	}
	return ImportFromSources(base, src)
}

// FormatImportSummary renders a one-line human summary of an import, e.g.
// "imported 3 skills, 2 agents, AGENTS.md; skipped 4 files". Zero counts are
// omitted; an import that wrote nothing reads "imported nothing".
func FormatImportSummary(s ImportSummary) string {
	var segments []string
	segments = appendCountSegment(segments, s.Rules, "rule")
	segments = appendCountSegment(segments, s.Skills, "skill")
	segments = appendCountSegment(segments, s.SkillDocs, "skill doc")
	segments = appendCountSegment(segments, s.Agents, "agent")
	segments = appendCountSegment(segments, s.Commands, "command")
	if s.RootMemoryFrom != "" {
		segments = append(segments, "AGENTS.md")
	}

	out := "imported nothing"
	if len(segments) > 0 {
		out = "imported " + strings.Join(segments, ", ")
	}
	if n := len(s.Skipped); n > 0 {
		out += fmt.Sprintf("; skipped %d %s", n, pluralize(n, "file"))
	}
	return out
}

// appendCountSegment appends "<n> <noun>[s]" when n > 0.
func appendCountSegment(segments []string, n int, noun string) []string {
	if n == 0 {
		return segments
	}
	return append(segments, fmt.Sprintf("%d %s", n, pluralize(n, noun)))
}

// pluralize naively appends "s" to noun when n != 1.
func pluralize(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

// sourceEntry is one file discovered while walking the import source dirs.
type sourceEntry struct {
	relPath string
	symlink bool // entry is a symlink; never read, skipped with a reason
}

// collectSourceFiles walks every existing dir (missing dirs are silently
// skipped) and returns the discovered files as base-relative slash paths,
// deduped and sorted for deterministic processing across dirs.
func collectSourceFiles(base string, dirs []string) ([]sourceEntry, error) {
	byPath := map[string]sourceEntry{}
	for _, dir := range dirs {
		root := filepath.Join(base, filepath.FromSlash(dir))
		walkFn := func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				if p == root && errors.Is(err, fs.ErrNotExist) {
					return nil // missing source dir → nothing to import
				}
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(base, p)
			if err != nil {
				return err
			}
			relPath := filepath.ToSlash(rel)
			byPath[relPath] = sourceEntry{relPath: relPath, symlink: d.Type()&fs.ModeSymlink != 0}
			return nil
		}
		if err := filepath.WalkDir(root, walkFn); err != nil {
			return nil, fmt.Errorf("walk %s: %w", dir, err)
		}
	}

	entries := make([]sourceEntry, 0, len(byPath))
	for _, e := range byPath {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].relPath < entries[j].relPath })
	return entries, nil
}

// importSourceFile adopts one discovered file into canonical, recording the
// outcome on summary. Errors become Skipped entries, never aborts.
func importSourceFile(base string, e sourceEntry, summary *ImportSummary) {
	if e.symlink {
		summary.Skipped = append(summary.Skipped, SkippedFile{Path: e.relPath, Reason: "symlink, not followed"})
		return
	}
	if !isMarkdownPath(e.relPath) {
		summary.Skipped = append(summary.Skipped, SkippedFile{Path: e.relPath, Reason: "not markdown"})
		return
	}
	if reason, reserved := reservedRuleSkip(e.relPath); reserved {
		summary.Skipped = append(summary.Skipped, SkippedFile{Path: e.relPath, Reason: reason})
		return
	}
	kind, err := adoptExternal(base, e.relPath)
	if err != nil {
		summary.Skipped = append(summary.Skipped, SkippedFile{Path: e.relPath, Reason: adoptSkipReason(err)})
		return
	}
	recordAdopted(kind, e.relPath, summary)
}

// isMarkdownPath reports whether path has an adoptable markdown extension.
func isMarkdownPath(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".md" || ext == ".mdc"
}

// reservedRuleSkip reports whether path would adopt as a canonical rule whose
// filename is reserved (canonical.Load would reject it), with the skip reason.
func reservedRuleSkip(path string) (string, bool) {
	slug, isRule := adoptedRuleSlug(path)
	if !isRule || !canonical.IsReservedRuleName(slug) {
		return "", false
	}
	return fmt.Sprintf("reserved rule name %q — %s", slug, canonical.ReservedRuleReason(slug)), true
}

// adoptedRuleSlug returns the canonical rule filename path would adopt as and
// whether path is rule-class at all. MUST mirror the load-bearing matcher order
// in adoptExternal: cline workflows precede cline rules, and skill paths
// precede the generic /rules/ matcher, so e.g. a skill doc under a
// "<skill>/rules/" subfolder is never treated as a rule. Cursor's catch-all
// general.mdc is excluded by matchRulePath itself (root memory, not a rule).
func adoptedRuleSlug(path string) (string, bool) {
	switch {
	case matchClineWorkflowPath(path):
		return "", false
	case matchClineRulePath(path):
		return strings.TrimSuffix(filepath.Base(path), ".md"), true
	case matchCopilotInstructionPath(path):
		return strings.TrimSuffix(filepath.Base(path), ".instructions.md"), true
	case matchSkillPath(path):
		return "", false
	case matchRulePath(path):
		return ruleFilename(path), true
	default:
		return "", false
	}
}

// adoptSkipReason normalizes adoptExternal errors into skip reasons: the
// unmapped-path case becomes the stable "no canonical mapping"; anything else
// (parse/frontmatter/safepath failures) keeps the error text.
func adoptSkipReason(err error) string {
	if strings.Contains(err.Error(), "no canonical mapping") {
		return "no canonical mapping"
	}
	return err.Error()
}

// recordAdopted increments the summary counter matching what adoptExternal
// actually wrote (cross-mapped files count as their canonical concept). A
// root-memory adoption from the walk (a candidate not listed in RootFiles)
// follows the same first-wins rule as the root-files pass.
func recordAdopted(kind adoptedKind, path string, summary *ImportSummary) {
	switch kind {
	case adoptedRootMemory:
		if summary.RootMemoryFrom != "" {
			summary.Skipped = append(summary.Skipped, SkippedFile{
				Path:   path,
				Reason: "root memory already imported from " + summary.RootMemoryFrom,
			})
			return
		}
		summary.RootMemoryFrom = path
	case adoptedRule:
		summary.Rules++
	case adoptedSkill:
		summary.Skills++
	case adoptedSkillDoc:
		summary.SkillDocs++
	case adoptedAgent:
		summary.Agents++
	case adoptedCommand:
		summary.Commands++
	}
}
