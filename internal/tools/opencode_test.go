package tools_test

import (
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// TestOpenCodeRendersAgents_ToolsAllowlistObject pins the conversion of a Claude
// `tools` allowlist array into OpenCode's `tools` object. OpenCode's config loader
// rejects an array ("Expected object | undefined, got [...]"); it requires an
// object keyed by lowercase tool names. An allowlist is expressed as the deny-all
// sentinel ("*": false) plus the enabled tools.
func TestOpenCodeRendersAgents_ToolsAllowlistObject(t *testing.T) {
	opencode := adapterByName(t, "OpenCode")
	c := &canonical.Canonical{
		Agents: []*canonical.Agent{{
			Filename:    "adapter-reviewer",
			Name:        "adapter-reviewer",
			Description: "Read-only adapter review",
			Tools:       []string{"Read", "Glob", "Grep", "Bash"},
			Model:       "sonnet",
			Body:        "System prompt.\n",
		}},
	}
	for _, tc := range []struct {
		scope    tools.Scope
		wantPath string
	}{
		{tools.ScopeProject, ".opencode/agents/adapter-reviewer.md"},
		{tools.ScopeUser, ".config/opencode/agents/adapter-reviewer.md"},
	} {
		writes, err := opencode.Render(c, tc.scope)
		if err != nil {
			t.Fatalf("scope=%v: Render: %v", tc.scope, err)
		}
		var agent *tools.FileWrite
		for i := range writes {
			if writes[i].Path == tc.wantPath {
				agent = &writes[i]
				break
			}
		}
		if agent == nil {
			t.Fatalf("scope=%v: expected %s, got %v", tc.scope, tc.wantPath, pathsOf(writes))
		}
		content := string(agent.Content)
		// must be the object form, never the array that crashes OpenCode's loader
		if strings.Contains(content, "tools: [") {
			t.Errorf("scope=%v: tools must be an object, not an array:\n%s", tc.scope, content)
		}
		for _, want := range []string{
			"name: adapter-reviewer",
			"description: Read-only adapter review",
			"model: sonnet",
			"tools:",
			": false", // deny-all sentinel "*": false
			"read: true",
			"glob: true",
			"grep: true",
			"bash: true",
			"System prompt.",
		} {
			if !strings.Contains(content, want) {
				t.Errorf("scope=%v: missing %q in:\n%s", tc.scope, want, content)
			}
		}
		if agent.Concept != tools.ConceptAgents {
			t.Errorf("scope=%v: Concept: want Agents, got %v", tc.scope, agent.Concept)
		}
	}
}

// TestOpenCodeRendersAgents_NoTools verifies that an agent without a tools
// allowlist emits no tools block (OpenCode then leaves every tool enabled, which
// matches a Claude agent that declares no `tools`).
func TestOpenCodeRendersAgents_NoTools(t *testing.T) {
	opencode := adapterByName(t, "OpenCode")
	c := &canonical.Canonical{
		Agents: []*canonical.Agent{{
			Filename: "free", Name: "free", Description: "no restriction", Body: "Body.\n",
		}},
	}
	writes, _ := opencode.Render(c, tools.ScopeProject)
	var agent *tools.FileWrite
	for i := range writes {
		if writes[i].Path == ".opencode/agents/free.md" {
			agent = &writes[i]
			break
		}
	}
	if agent == nil {
		t.Fatal("expected .opencode/agents/free.md")
	}
	if strings.Contains(string(agent.Content), "tools:") {
		t.Errorf("agent without allowlist must emit no tools block:\n%s", string(agent.Content))
	}
}

// TestOpenCodeToolNameRoundTrip locks the canonical↔OpenCode tool-name mapping,
// including the non-lowercase rename (LS↔list) and the case-preservation pins for
// Claude tools whose OpenCode key is just the lowercase form (WebSearch↔websearch,
// NotebookEdit↔notebookedit). Those pins guard the render↔adopt round-trip: without
// them the lowercase fallback would drop casing and the name could not reverse.
func TestOpenCodeToolNameRoundTrip(t *testing.T) {
	cases := []struct{ canonical, opencode string }{
		{"Read", "read"},
		{"Bash", "bash"},
		{"LS", "list"},
		{"WebFetch", "webfetch"},
		{"TodoWrite", "todowrite"},
		{"Skill", "skill"},
		{"WebSearch", "websearch"},
		{"NotebookEdit", "notebookedit"},
		{"BashOutput", "bashoutput"},
		{"SlashCommand", "slashcommand"},
		{"ExitPlanMode", "exitplanmode"},
	}
	for _, tc := range cases {
		if got := tools.OpenCodeToolName(tc.canonical); got != tc.opencode {
			t.Errorf("OpenCodeToolName(%q) = %q, want %q", tc.canonical, got, tc.opencode)
		}
		if got := tools.CanonicalToolName(tc.opencode); got != tc.canonical {
			t.Errorf("CanonicalToolName(%q) = %q, want %q", tc.opencode, got, tc.canonical)
		}
	}
	// unmapped name: lowercase on the way out, unchanged on the way back
	if got := tools.OpenCodeToolName("CustomMcp"); got != "custommcp" {
		t.Errorf("OpenCodeToolName fallback = %q, want %q", got, "custommcp")
	}
	if got := tools.CanonicalToolName("custommcp"); got != "custommcp" {
		t.Errorf("CanonicalToolName fallback = %q, want %q", got, "custommcp")
	}
}
