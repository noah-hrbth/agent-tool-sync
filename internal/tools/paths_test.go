package tools

import (
	"strings"
	"testing"
)

// TestAggregatePrefixSlices guards the shared path vocabulary: every aggregate
// slice adopt.go consumes must be non-empty and well-formed, so a typo in a
// constant surfaces here rather than as silent adopt drift.
func TestAggregatePrefixSlices(t *testing.T) {
	cases := []struct {
		name    string
		slice   []string
		wantSub string
	}{
		{"SkillDirPrefixes", SkillDirPrefixes(), "/skills/"},
		{"AgentDirPrefixes", AgentDirPrefixes(), "/agents/"},
		{"CommandDirPrefixes", CommandDirPrefixes(), "/commands/"},
	}
	for _, c := range cases {
		if len(c.slice) == 0 {
			t.Errorf("%s is empty", c.name)
		}
		for _, p := range c.slice {
			if !strings.HasSuffix(p, c.wantSub) {
				t.Errorf("%s entry %q does not end with %q", c.name, p, c.wantSub)
			}
		}
	}
	if len(RootMemoryFiles()) == 0 {
		t.Error("RootMemoryFiles is empty")
	}
	if CursorCatchAll == "" {
		t.Error("CursorCatchAll is empty")
	}
}

// TestExpectedAdoptOutcomeDefaults locks the safety default: an unknown path
// for a tool with no exception is OutcomeReversible, so a newly added render
// path that adopt.go cannot handle fails the contract test (not silently
// declared non-reversible).
func TestExpectedAdoptOutcomeDefaults(t *testing.T) {
	got := ExpectedAdoptOutcome("claude", ConceptSkills, ".claude/skills/x/SKILL.md")
	if got.Kind != OutcomeReversible {
		t.Errorf("claude skill: want OutcomeReversible, got %v", got.Kind)
	}
	got = ExpectedAdoptOutcome("gemini", ConceptSkills, ".gemini/skills/x/SKILL.md")
	if got.Kind != OutcomeNonReversible {
		t.Errorf("gemini skill: want OutcomeNonReversible, got %v", got.Kind)
	}
	got = ExpectedAdoptOutcome("vibe", ConceptCommands, ".vibe/skills/x/SKILL.md")
	if got.Kind != OutcomeCrossMapped || got.CrossTo != ConceptSkills {
		t.Errorf("vibe command: want CrossMapped→Skills, got %v/%v", got.Kind, got.CrossTo)
	}
	got = ExpectedAdoptOutcome("claude", ConceptRules, "CLAUDE.md")
	if got.Kind != OutcomeRootMemory {
		t.Errorf("CLAUDE.md: want OutcomeRootMemory, got %v", got.Kind)
	}
}
