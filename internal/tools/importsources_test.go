package tools_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/tools"
)

// toolByKey finds a registered tool by its stable key or fails the test.
func toolByKey(t *testing.T, key string) tools.Tool {
	t.Helper()
	for _, tool := range tools.All() {
		if tool.Meta.Key == key {
			return tool
		}
	}
	t.Fatalf("tool %q not in registry", key)
	return tools.Tool{}
}

func deriveSources(t *testing.T, key string, scope tools.Scope) tools.ImportSources {
	t.Helper()
	sources, err := tools.DeriveImportSources(toolByKey(t, key), scope)
	if err != nil {
		t.Fatalf("DeriveImportSources(%s, %s): %v", key, scope, err)
	}
	return sources
}

func TestDeriveImportSourcesClaudeProject(t *testing.T) {
	sources := deriveSources(t, "claude", tools.ScopeProject)

	for _, want := range []string{".claude/rules", ".claude/skills", ".claude/agents"} {
		if !slices.Contains(sources.Dirs, want) {
			t.Errorf("Dirs missing %q: %v", want, sources.Dirs)
		}
	}
	for _, paths := range [][]string{sources.Dirs, sources.RootFiles, sources.DetectFiles} {
		for _, p := range paths {
			if strings.Contains(p, "agentsync-probe") {
				t.Errorf("probe name leaked into derived path %q", p)
			}
		}
	}
	// membership + first position; exact alternates list pinned in
	// TestDeriveImportSourcesClaudeRootFileAlternates
	if len(sources.RootFiles) == 0 || sources.RootFiles[0] != "CLAUDE.md" {
		t.Errorf("RootFiles: want CLAUDE.md first, got %v", sources.RootFiles)
	}
}

func TestDeriveImportSourcesOpenCodeUser(t *testing.T) {
	sources := deriveSources(t, "opencode", tools.ScopeUser)

	for _, want := range []string{".config/opencode/skills", ".config/opencode/agents", ".config/opencode/commands"} {
		if !slices.Contains(sources.Dirs, want) {
			t.Errorf("Dirs missing %q: %v", want, sources.Dirs)
		}
	}
	wantRoot := []string{".config/opencode/AGENTS.md"}
	if !slices.Equal(sources.RootFiles, wantRoot) {
		t.Errorf("RootFiles: want %v, got %v", wantRoot, sources.RootFiles)
	}
}

func TestDeriveImportSourcesCodexProjectSkillsLandInDotAgents(t *testing.T) {
	sources := deriveSources(t, "codex", tools.ScopeProject)

	if !slices.Contains(sources.Dirs, ".agents/skills") {
		t.Errorf("Dirs missing .agents/skills: %v", sources.Dirs)
	}
}

func TestDeriveImportSourcesVibeDedupesSkillsAndCommands(t *testing.T) {
	// Vibe renders both skills and commands under .vibe/skills/
	sources := deriveSources(t, "vibe", tools.ScopeProject)

	count := 0
	for _, dir := range sources.Dirs {
		if dir == ".vibe/skills" {
			count++
		}
	}
	if count != 1 {
		t.Errorf(".vibe/skills: want exactly 1 occurrence, got %d in %v", count, sources.Dirs)
	}
}

func TestDeriveImportSourcesClaudeIncludesDeprecatedCommandsDir(t *testing.T) {
	// renderClaude no longer emits commands (deprecated) but adopt.go still
	// reverses .claude/commands/ — an existing installation may have them
	sources := deriveSources(t, "claude", tools.ScopeProject)

	if !slices.Contains(sources.Dirs, ".claude/commands") {
		t.Errorf("Dirs missing .claude/commands: %v", sources.Dirs)
	}
}

func TestDeriveImportSourcesClaudeRootFileAlternates(t *testing.T) {
	sources := deriveSources(t, "claude", tools.ScopeProject)

	want := []string{"CLAUDE.md", ".claude/CLAUDE.md"}
	if !slices.Equal(sources.RootFiles, want) {
		t.Errorf("RootFiles: want %v (rootMemoryFiles order), got %v", want, sources.RootFiles)
	}
}

func TestDeriveImportSourcesPiProjectExcludesUserScopePaths(t *testing.T) {
	// full-prefix matching: ".pi/agent/skills" parent ".pi/agent" != derived root ".pi"
	sources := deriveSources(t, "pi", tools.ScopeProject)

	if slices.Contains(sources.Dirs, ".pi/agent/skills") {
		t.Errorf("Dirs leaked user-scope .pi/agent/skills: %v", sources.Dirs)
	}
	if slices.Contains(sources.RootFiles, ".pi/agent/AGENTS.md") {
		t.Errorf("RootFiles leaked user-scope .pi/agent/AGENTS.md: %v", sources.RootFiles)
	}
}

func TestDeriveImportSourcesUnionIsNoopForOpenCode(t *testing.T) {
	// every adopt prefix for OpenCode project scope is already render-derived
	sources := deriveSources(t, "opencode", tools.ScopeProject)

	want := []string{".opencode/agents", ".opencode/commands", ".opencode/skills"}
	if !slices.Equal(sources.Dirs, want) {
		t.Errorf("Dirs: want %v, got %v", want, sources.Dirs)
	}
}

func TestDeriveImportSourcesUnsupportedScopeEmpty(t *testing.T) {
	sources := deriveSources(t, "cursor", tools.ScopeUser)

	if len(sources.Dirs) != 0 || len(sources.RootFiles) != 0 || len(sources.DetectFiles) != 0 {
		t.Errorf("cursor user scope: want all-empty sources, got %+v", sources)
	}
}

func TestDeriveImportSourcesZedRulesIsDetectOnly(t *testing.T) {
	sources := deriveSources(t, "zed", tools.ScopeProject)

	if !slices.Equal(sources.DetectFiles, []string{".rules"}) {
		t.Errorf("DetectFiles: want [.rules], got %v", sources.DetectFiles)
	}
	if len(sources.RootFiles) != 0 {
		t.Errorf("RootFiles: want empty, got %v", sources.RootFiles)
	}
	if len(sources.Dirs) != 0 {
		t.Errorf("Dirs: want empty, got %v", sources.Dirs)
	}
}
