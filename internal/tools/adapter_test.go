package tools_test

import (
	"testing"

	"github.com/noah-hrbth/agentsync/internal/tools"
)

func TestAlias(t *testing.T) {
	tests := []struct {
		name    string
		adapter tools.Adapter
		cases   []struct {
			concept tools.Concept
			want    string
		}
	}{
		{
			name:    "Claude Code",
			adapter: tools.All()[0],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "CLAUDE.md"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "OpenCode",
			adapter: tools.All()[1],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Cursor",
			adapter: tools.All()[2],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "general.mdc"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Gemini CLI",
			adapter: tools.All()[3],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, "GEMINI.md"},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
		{
			name:    "Codex CLI",
			adapter: tools.All()[4],
			cases: []struct {
				concept tools.Concept
				want    string
			}{
				{tools.ConceptRules, ""},
				{tools.ConceptSkills, ""},
				{tools.ConceptAgents, ""},
				{tools.ConceptCommands, ""},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tc := range tt.cases {
				got := tt.adapter.Alias(tc.concept)
				if got != tc.want {
					t.Errorf("Alias(%q): got %q, want %q", tc.concept, got, tc.want)
				}
			}
		})
	}
}

func TestCursorSupportsAllConcepts(t *testing.T) {
	cursor := tools.All()[2]
	for _, concept := range []tools.Concept{
		tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands,
	} {
		if got := cursor.Supports(concept); !got.Supported {
			t.Errorf("Cursor.Supports(%v): want supported=true, got false (%s)", concept, got.Reason)
		}
	}
}

func TestClaudeCommandsDeprecated(t *testing.T) {
	claude := tools.All()[0]
	compat := claude.Supports(tools.ConceptCommands)
	if !compat.Supported {
		t.Error("Claude commands should still be Supported=true (backward-compat)")
	}
	if !compat.Deprecated {
		t.Error("Claude commands should be Deprecated=true")
	}
	if compat.Reason == "" {
		t.Error("Claude commands should have a non-empty Reason")
	}
}

func TestGeminiSupportsAllConcepts(t *testing.T) {
	gemini := tools.All()[3]
	for _, concept := range []tools.Concept{
		tools.ConceptRules, tools.ConceptSkills, tools.ConceptAgents, tools.ConceptCommands,
	} {
		compat := gemini.Supports(concept)
		if !compat.Supported {
			t.Errorf("Gemini.Supports(%v): want supported=true, got false (%s)", concept, compat.Reason)
		}
		if compat.Deprecated {
			t.Errorf("Gemini.Supports(%v): want Deprecated=false, got true", concept)
		}
	}
}

func TestCodexCommandsDeprecated(t *testing.T) {
	codex := tools.All()[4]

	// Skills and agents should be fully supported
	for _, concept := range []tools.Concept{tools.ConceptSkills, tools.ConceptAgents} {
		compat := codex.Supports(concept)
		if !compat.Supported {
			t.Errorf("Codex.Supports(%v): want supported=true", concept)
		}
		if compat.Deprecated {
			t.Errorf("Codex.Supports(%v): want Deprecated=false", concept)
		}
	}

	// Commands should be deprecated
	cmdCompat := codex.Supports(tools.ConceptCommands)
	if !cmdCompat.Supported {
		t.Error("Codex commands should be Supported=true (backward-compat)")
	}
	if !cmdCompat.Deprecated {
		t.Error("Codex commands should be Deprecated=true")
	}
	if cmdCompat.Reason == "" {
		t.Error("Codex commands should have a non-empty Reason")
	}
}
