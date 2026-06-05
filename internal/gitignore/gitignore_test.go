package gitignore

import (
	"reflect"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// newFake builds a minimal tools.Tool used to drive Compute without touching
// real adapters. Only Render is exercised; metadata is inert.
func newFake(name string, paths ...string) tools.Tool {
	var files []tools.FileWrite
	for _, p := range paths {
		files = append(files, tools.FileWrite{Path: p})
	}
	return tools.Tool{
		Meta: tools.ToolMeta{Name: name},
		Render: func(_ *canonical.Canonical, _ tools.Scope) ([]tools.FileWrite, error) {
			return files, nil
		},
	}
}

func TestComputeExtractsFirstSegmentForDirPaths(t *testing.T) {
	got := Compute([]tools.Tool{newFake("x", ".foo/bar/baz.md")})
	want := []string{".foo/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeIncludesTopLevelFiles(t *testing.T) {
	got := Compute([]tools.Tool{newFake("x", "WIDGET.md")})
	want := []string{"WIDGET.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestComputeExcludesDotGithub ensures `.github/` is never auto-added to the
// managed gitignore block. The GitHub Copilot adapter writes inside `.github/`,
// but the directory is shared with CI workflows that MUST stay tracked.
func TestComputeExcludesDotGithub(t *testing.T) {
	got := Compute([]tools.Tool{newFake("copilot",
		".github/copilot-instructions.md",
		".github/instructions/foo.instructions.md",
		".github/skills/foo/SKILL.md",
		".github/agents/foo.agent.md",
		".github/prompts/foo.prompt.md",
	)})
	for _, entry := range got {
		if entry == ".github/" || entry == ".github" {
			t.Errorf("Compute should never emit %q (would ignore CI workflows); got %v", entry, got)
		}
	}
}

// TestComputeExcludesDotGithubAgainstRealAdapters wires the actual adapter
// registry through Compute and confirms `.github/` stays out of the result.
func TestComputeExcludesDotGithubAgainstRealAdapters(t *testing.T) {
	got := Compute(tools.All())
	for _, entry := range got {
		if entry == ".github/" || entry == ".github" {
			t.Fatalf("real adapters caused Compute to emit %q; full result %v", entry, got)
		}
	}
}

func TestComputeReturnsSortedUnique(t *testing.T) {
	got := Compute([]tools.Tool{
		newFake("a", ".zeta/x", ".alpha/y", ".zeta/z"),
		newFake("b", ".alpha/q", ".beta/r"),
	})
	want := []string{".alpha/", ".beta/", ".zeta/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeExcludesAgentsyncDir(t *testing.T) {
	got := Compute([]tools.Tool{newFake("x", ".agentsync/foo.md", ".claude/skills/s.md")})
	want := []string{".claude/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeExcludesRootAGENTSmd(t *testing.T) {
	// Multiple adapters emit AGENTS.md at workspace root; it must not appear.
	got := Compute([]tools.Tool{
		newFake("a", "AGENTS.md", ".opencode/AGENTS.md"),
		newFake("b", "AGENTS.md", ".cline/x.md"),
	})
	for _, e := range got {
		if e == "AGENTS.md" {
			t.Fatalf("AGENTS.md should be excluded, got %v", got)
		}
	}
	// .opencode/ and .cline/ should still be there.
	want := []string{".cline/", ".opencode/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeUsesStubCanonicalToFlushAdapters(t *testing.T) {
	// With empty canonical, only Cursor would emit (it always writes
	// .cursor/rules/general.mdc). The stub canonical Compute uses must flush
	// every adapter so we get the full 13-entry set.
	got := Compute(tools.All())
	if len(got) < 10 {
		t.Fatalf("expected stub canonical to flush all adapters; got only %d entries: %v", len(got), got)
	}
}

func TestComputeFullRegisteredAdapterSet(t *testing.T) {
	got := Compute(tools.All())
	want := []string{
		".agents/",
		".claude/",
		".cline/",
		".clinerules/",
		".codex/",
		".cursor/",
		".gemini/",
		".junie/",
		".opencode/",
		".pi/",
		".rules",
		".vibe/",
		"CLAUDE.md",
		"GEMINI.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

// TestRootSkipMapsAreEnforced pins every documented carve-out: each key in
// rootSegmentSkips/rootFileSkips must be dropped by entryFor and must never
// appear in the real-adapter Compute output. Adding a map entry auto-extends
// coverage; the rationale must be non-empty.
func TestRootSkipMapsAreEnforced(t *testing.T) {
	if len(rootSegmentSkips) == 0 || len(rootFileSkips) == 0 {
		t.Fatal("skip maps unexpectedly empty")
	}
	for seg, reason := range rootSegmentSkips {
		if reason == "" {
			t.Errorf("rootSegmentSkips[%q] has empty rationale", seg)
		}
		if e := entryFor(seg); e != "" {
			t.Errorf("entryFor(%q) = %q, want \"\" (file form)", seg, e)
		}
		if e := entryFor(seg + "/child.md"); e != "" {
			t.Errorf("entryFor(%q/...) = %q, want \"\" (dir form)", seg, e)
		}
	}
	for name, reason := range rootFileSkips {
		if reason == "" {
			t.Errorf("rootFileSkips[%q] has empty rationale", name)
		}
		if e := entryFor(name); e != "" {
			t.Errorf("entryFor(%q) = %q, want \"\"", name, e)
		}
	}
	skipped := map[string]bool{}
	for seg := range rootSegmentSkips {
		skipped[seg] = true
		skipped[seg+"/"] = true
	}
	for name := range rootFileSkips {
		skipped[name] = true
	}
	for _, entry := range Compute(tools.All()) {
		if skipped[entry] {
			t.Errorf("Compute(tools.All()) emitted carve-out %q", entry)
		}
	}
}
