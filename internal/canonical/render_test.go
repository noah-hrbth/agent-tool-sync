package canonical

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRulesRejectsReservedName(t *testing.T) {
	ws := t.TempDir()
	base := filepath.Join(ws, ".agentsync")
	if err := os.MkdirAll(filepath.Join(base, "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "rules", "general.md"), []byte("body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(ws)
	if err == nil {
		t.Fatal("expected error for reserved rule name 'general', got nil")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("expected error to mention 'reserved', got: %v", err)
	}
}

// TestRenderParseRuleRoundTrip verifies that RenderRule → ParseRule preserves all fields.
func TestRenderParseRuleRoundTrip(t *testing.T) {
	base := filepath.Join("..", "..", "examples", "sandbox-seed", ".agentsync")
	rules, err := loadRules(base)
	if err != nil {
		t.Fatalf("loadRules: %v", err)
	}
	if len(rules) == 0 {
		t.Skip("no seed rules found")
	}
	for _, r := range rules {
		rendered, err := RenderRule(r)
		if err != nil {
			t.Fatalf("RenderRule %s: %v", r.Filename, err)
		}
		parsed := &Rule{Filename: r.Filename}
		if err := ParseRule(rendered, parsed); err != nil {
			t.Fatalf("ParseRule %s: %v", r.Filename, err)
		}
		if parsed.Description != r.Description {
			t.Errorf("rule %s: Description mismatch: %q vs %q", r.Filename, parsed.Description, r.Description)
		}
		if parsed.Body != r.Body {
			t.Errorf("rule %s: Body mismatch\ngot:  %q\nwant: %q", r.Filename, parsed.Body, r.Body)
		}
		if parsed.Filename != r.Filename {
			t.Errorf("rule %s: Filename not preserved", r.Filename)
		}
		for i, p := range r.Paths {
			if i >= len(parsed.Paths) || parsed.Paths[i] != p {
				t.Errorf("rule %s: Paths mismatch", r.Filename)
				break
			}
		}
	}
}

// TestRenderParseSkillRoundTrip verifies that RenderSkill → ParseSkill preserves all fields.
func TestRenderParseSkillRoundTrip(t *testing.T) {
	base := filepath.Join("..", "..", "examples", "sandbox-seed", ".agentsync")
	skills, err := loadSkills(base)
	if err != nil {
		t.Fatalf("loadSkills: %v", err)
	}
	if len(skills) == 0 {
		t.Skip("no seed skills found")
	}
	for _, s := range skills {
		rendered, err := RenderSkill(s)
		if err != nil {
			t.Fatalf("RenderSkill %s: %v", s.Dir, err)
		}
		parsed := &Skill{Dir: s.Dir}
		if err := ParseSkill(rendered, parsed); err != nil {
			t.Fatalf("ParseSkill %s: %v", s.Dir, err)
		}
		if parsed.Name != s.Name {
			t.Errorf("skill %s: Name mismatch: %q vs %q", s.Dir, parsed.Name, s.Name)
		}
		if parsed.Description != s.Description {
			t.Errorf("skill %s: Description mismatch", s.Dir)
		}
		if parsed.Body != s.Body {
			t.Errorf("skill %s: Body mismatch\ngot:  %q\nwant: %q", s.Dir, parsed.Body, s.Body)
		}
		if parsed.Dir != s.Dir {
			t.Errorf("skill %s: Dir not preserved", s.Dir)
		}
	}
}

// TestRenderParseAgentRoundTrip verifies that RenderAgent → ParseAgent preserves all fields.
func TestRenderParseAgentRoundTrip(t *testing.T) {
	base := filepath.Join("..", "..", "examples", "sandbox-seed", ".agentsync")
	agents, err := loadAgents(base)
	if err != nil {
		t.Fatalf("loadAgents: %v", err)
	}
	if len(agents) == 0 {
		t.Skip("no seed agents found")
	}
	for _, a := range agents {
		rendered, err := RenderAgent(a)
		if err != nil {
			t.Fatalf("RenderAgent %s: %v", a.Filename, err)
		}
		parsed := &Agent{Filename: a.Filename}
		if err := ParseAgent(rendered, parsed); err != nil {
			t.Fatalf("ParseAgent %s: %v", a.Filename, err)
		}
		if parsed.Name != a.Name {
			t.Errorf("agent %s: Name mismatch", a.Filename)
		}
		if parsed.Description != a.Description {
			t.Errorf("agent %s: Description mismatch", a.Filename)
		}
		if parsed.Model != a.Model {
			t.Errorf("agent %s: Model mismatch: %q vs %q", a.Filename, parsed.Model, a.Model)
		}
		if parsed.Body != a.Body {
			t.Errorf("agent %s: Body mismatch", a.Filename)
		}
		if parsed.Filename != a.Filename {
			t.Errorf("agent %s: Filename not preserved", a.Filename)
		}
	}
}
