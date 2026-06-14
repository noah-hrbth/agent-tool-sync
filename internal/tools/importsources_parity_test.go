package tools

import (
	"strings"
	"testing"
)

// pathReachable reports whether p is fetchable by the import engine: either an
// exact root-memory candidate or a file under one of the walkable dirs.
// Mirrors the coverage ImportFromSources actually walks (RootFiles + Dirs);
// DetectFiles are deliberately excluded — they are detect-only.
func pathReachable(src ImportSources, p string) bool {
	for _, rf := range src.RootFiles {
		if p == rf {
			return true
		}
	}
	for _, dir := range src.Dirs {
		if strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// TestDeriveImportSourcesCoversEveryReversiblePath guards the implicit coupling
// in DeriveImportSources: any rendered path that adopt.go can reverse
// (ExpectedAdoptOutcome != OutcomeNonReversible) MUST be reachable from the
// import sources, never parked in detect-only DetectFiles. A future adapter
// that renders a reversible concept to a fixed filename (no probe segment,
// not a root-memory path) would silently drop from import — this fails first.
func TestDeriveImportSourcesCoversEveryReversiblePath(t *testing.T) {
	for _, tool := range All() {
		for _, scope := range []Scope{ScopeProject, ScopeUser} {
			if !tool.Meta.SupportsScope(scope).Supported {
				continue
			}
			writes, err := tool.Render(probeCanonical(), scope)
			if err != nil {
				t.Fatalf("%s render %s: %v", tool.Meta.Key, scope, err)
			}
			src, err := DeriveImportSources(tool, scope)
			if err != nil {
				t.Fatalf("%s derive %s: %v", tool.Meta.Key, scope, err)
			}
			for _, fw := range writes {
				if ExpectedAdoptOutcome(tool.Meta.Key, fw.Concept, fw.Path).Kind == OutcomeNonReversible {
					continue
				}
				if !pathReachable(src, fw.Path) {
					t.Errorf("%s/%s: reversible render path %q not reachable from import sources (Dirs=%v RootFiles=%v)",
						tool.Meta.Key, scope, fw.Path, src.Dirs, src.RootFiles)
				}
			}
		}
	}
}
