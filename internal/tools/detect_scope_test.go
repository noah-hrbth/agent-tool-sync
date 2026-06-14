package tools_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/tools"
)

// writeWorkspaceFile creates rel (and parent dirs) under ws with stub content.
func writeWorkspaceFile(t *testing.T, ws, rel string) {
	t.Helper()
	abs := filepath.Join(ws, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte("stub\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectAtScopeProjectDirHit(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".claude/skills/foo/SKILL.md")

	inst := tools.DetectAtScope(ws, toolByKey(t, "claude"), tools.ScopeProject)

	if !inst.Found {
		t.Fatal("claude: want Found via .claude/skills dir")
	}
	if want := filepath.Join(ws, ".claude", "skills"); inst.Path != want {
		t.Errorf("Path: want %q, got %q", want, inst.Path)
	}
}

func TestDetectAtScopeProjectRootMemoryHit(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "CLAUDE.md")

	inst := tools.DetectAtScope(ws, toolByKey(t, "claude"), tools.ScopeProject)

	if !inst.Found {
		t.Fatal("claude: want Found via root CLAUDE.md")
	}
	if want := filepath.Join(ws, "CLAUDE.md"); inst.Path != want {
		t.Errorf("Path: want %q, got %q", want, inst.Path)
	}
}

func TestDetectAtScopeProjectAgentsMDFansOut(t *testing.T) {
	// a bare AGENTS.md is the shared project root memory of six tools
	// (ADR-0004); each must report itself as an import candidate
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, "AGENTS.md")

	for _, key := range []string{"codex", "opencode", "vibe", "pi", "cline", "junie"} {
		inst := tools.DetectAtScope(ws, toolByKey(t, key), tools.ScopeProject)
		if !inst.Found {
			t.Errorf("%s: want Found via bare AGENTS.md", key)
		}
	}
}

func TestDetectAtScopeProjectZedRulesFile(t *testing.T) {
	ws := t.TempDir()
	writeWorkspaceFile(t, ws, ".rules")

	inst := tools.DetectAtScope(ws, toolByKey(t, "zed"), tools.ScopeProject)

	if !inst.Found {
		t.Fatal("zed: want Found via .rules detect file")
	}
}

func TestDetectAtScopeProjectEmptyWorkspaceDetectsNothing(t *testing.T) {
	ws := t.TempDir()

	for _, tool := range tools.All() {
		inst := tools.DetectAtScope(ws, tool, tools.ScopeProject)
		if inst.Found {
			t.Errorf("%s: want not Found in empty workspace, got Path %q", tool.Meta.Key, inst.Path)
		}
	}
}

func TestDetectAtScopeUserDelegatesToMetaDetect(t *testing.T) {
	want := tools.Installation{Found: true, Path: "/stub/install"}
	var gotWorkspace string
	stub := tools.Tool{Meta: tools.ToolMeta{
		Key: "stub",
		Detect: func(ws string) tools.Installation {
			gotWorkspace = ws
			return want
		},
	}}

	inst := tools.DetectAtScope("/fake/ws", stub, tools.ScopeUser)

	if inst != want {
		t.Errorf("Installation: want %+v, got %+v", want, inst)
	}
	if gotWorkspace != "/fake/ws" {
		t.Errorf("workspace passthrough: want /fake/ws, got %q", gotWorkspace)
	}
}

func TestImportEligible(t *testing.T) {
	cases := []struct {
		key   string
		scope tools.Scope
		want  bool
	}{
		{"zed", tools.ScopeProject, false}, // only output .rules is non-reversible
		{"claude", tools.ScopeProject, true},
		{"cursor", tools.ScopeProject, true}, // catch-all reverses as root memory
		{"opencode", tools.ScopeProject, true},
		{"cursor", tools.ScopeUser, false}, // user scope unsupported
	}
	for _, c := range cases {
		got := tools.ImportEligible(toolByKey(t, c.key), c.scope)
		if got != c.want {
			t.Errorf("ImportEligible(%s, %s): want %v, got %v", c.key, c.scope, c.want, got)
		}
	}
}
