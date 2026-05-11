package gitignore

import (
	"reflect"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// fakeAdapter is a minimal Adapter implementation used to drive Compute without
// touching real adapters. Only Render is exercised; the other methods return
// inert values.
type fakeAdapter struct {
	name  string
	files []tools.FileWrite
}

func (a *fakeAdapter) Name() string                                { return a.name }
func (a *fakeAdapter) Detect(string) tools.Installation            { return tools.Installation{} }
func (a *fakeAdapter) Supports(tools.Concept) tools.Compatibility  { return tools.Compatibility{Supported: true} }
func (a *fakeAdapter) SupportsScope(tools.Scope) tools.Compatibility {
	return tools.Compatibility{Supported: true}
}
func (a *fakeAdapter) Alias(tools.Concept) string                  { return "" }
func (a *fakeAdapter) ConceptInfo(tools.Concept) string            { return "" }
func (a *fakeAdapter) Render(_ *canonical.Canonical, _ tools.Scope) ([]tools.FileWrite, error) {
	return a.files, nil
}

func newFake(name string, paths ...string) tools.Adapter {
	fa := &fakeAdapter{name: name}
	for _, p := range paths {
		fa.files = append(fa.files, tools.FileWrite{Path: p})
	}
	return fa
}

func TestComputeExtractsFirstSegmentForDirPaths(t *testing.T) {
	got := Compute([]tools.Adapter{newFake("x", ".foo/bar/baz.md")})
	want := []string{".foo/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeIncludesTopLevelFiles(t *testing.T) {
	got := Compute([]tools.Adapter{newFake("x", "WIDGET.md")})
	want := []string{"WIDGET.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeReturnsSortedUnique(t *testing.T) {
	got := Compute([]tools.Adapter{
		newFake("a", ".zeta/x", ".alpha/y", ".zeta/z"),
		newFake("b", ".alpha/q", ".beta/r"),
	})
	want := []string{".alpha/", ".beta/", ".zeta/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeExcludesAgentsyncDir(t *testing.T) {
	got := Compute([]tools.Adapter{newFake("x", ".agentsync/foo.md", ".claude/skills/s.md")})
	want := []string{".claude/"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestComputeExcludesRootAGENTSmd(t *testing.T) {
	// Multiple adapters emit AGENTS.md at workspace root; it must not appear.
	got := Compute([]tools.Adapter{
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
		".rules",
		".vibe/",
		"CLAUDE.md",
		"GEMINI.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}
