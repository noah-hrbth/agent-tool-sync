package tools_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/tools"
)

// goldenPath holds the byte-for-byte snapshot of every adapter's observable
// metadata. The ToolMeta/Tool refactor must not change this output.
const goldenPath = "testdata/metadata_golden.json"

type conceptDump struct {
	Concept     string
	Supports    tools.Compatibility
	Alias       string
	ConceptInfo string
}

type scopeDump struct {
	Scope         string
	SupportsScope tools.Compatibility
}

type toolDump struct {
	Name     string
	Concepts []conceptDump
	Scopes   []scopeDump
}

var dumpConcepts = []tools.Concept{
	tools.ConceptRules,
	tools.ConceptSkills,
	tools.ConceptAgents,
	tools.ConceptCommands,
}

var dumpScopes = []tools.Scope{tools.ScopeProject, tools.ScopeUser}

// dumpMetadata builds a deterministic snapshot of all adapters' metadata in
// registry order. Detect is intentionally skipped (filesystem-dependent).
func dumpMetadata() []toolDump {
	all := tools.All()
	out := make([]toolDump, 0, len(all))
	for _, a := range all {
		td := toolDump{Name: a.Meta.Name}
		for _, concept := range dumpConcepts {
			td.Concepts = append(td.Concepts, conceptDump{
				Concept:     string(concept),
				Supports:    a.Meta.Supports(concept),
				Alias:       a.Meta.Alias(concept),
				ConceptInfo: a.Meta.Info(concept),
			})
		}
		for _, scope := range dumpScopes {
			td.Scopes = append(td.Scopes, scopeDump{
				Scope:         scope.String(),
				SupportsScope: a.Meta.SupportsScope(scope),
			})
		}
		out = append(out, td)
	}
	return out
}

// TestMetadataParity asserts the metadata snapshot matches the committed
// golden. Regenerate with UPDATE_GOLDEN=1 go test ./internal/tools/.
func TestMetadataParity(t *testing.T) {
	got, err := json.MarshalIndent(dumpMetadata(), "", "  ")
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	got = append(got, '\n')

	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden regenerated: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("metadata drift vs %s — refactor changed observable behaviour", goldenPath)
	}
}
