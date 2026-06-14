package syncer

import (
	"os"
	"path/filepath"
	"testing"
)

// seedAdoptFile writes content at <ws>/<relPath>, creating parent dirs.
func seedAdoptFile(t *testing.T, ws, relPath, content string) {
	t.Helper()
	full := filepath.Join(ws, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestAdoptExternalResultKinds pins the adoptedKind reported for every switch
// case in adoptExternal, so the bulk-import engine can trust what each
// adoption wrote. One case per path class, in switch order.
func TestAdoptExternalResultKinds(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		content string
		want    adoptedKind
		wantErr bool
	}{
		{
			name:    "root memory file",
			path:    "CLAUDE.md",
			content: "# root memory\n",
			want:    adoptedRootMemory,
		},
		{
			name:    "cursor catch-all general.mdc",
			path:    ".cursor/rules/general.mdc",
			content: "---\nalwaysApply: true\ndescription: Synced by agentsync\n---\n# Rules body\n",
			want:    adoptedRootMemory,
		},
		{
			name:    "cline workflow",
			path:    ".clinerules/workflows/deploy.md",
			content: "do the deploy steps\n",
			want:    adoptedCommand,
		},
		{
			name:    "cline rule",
			path:    ".clinerules/style.md",
			content: "---\npaths: [src/**]\n---\nRule body.\n",
			want:    adoptedRule,
		},
		{
			name:    "copilot instruction",
			path:    ".github/instructions/style.instructions.md",
			content: "---\napplyTo: \"src/**\"\ndescription: Style rule\n---\nRule body.\n",
			want:    adoptedRule,
		},
		{
			name:    "copilot agent",
			path:    ".github/agents/reviewer.agent.md",
			content: "---\nname: reviewer\ndescription: Reviews code\n---\nReview.\n",
			want:    adoptedAgent,
		},
		{
			name:    "copilot prompt",
			path:    ".github/prompts/commit.prompt.md",
			content: "---\ndescription: Stage and commit\n---\nCommit.\n",
			want:    adoptedCommand,
		},
		{
			name:    "pi prompt",
			path:    ".pi/prompts/style.md",
			content: "---\ndescription: Apply style\n---\nFormat.\n",
			want:    adoptedCommand,
		},
		{
			name:    "claude skill manifest",
			path:    ".claude/skills/s/SKILL.md",
			content: "---\nname: s\ndescription: A skill\n---\nSkill body.\n",
			want:    adoptedSkill,
		},
		{
			name:    "claude skill doc",
			path:    ".claude/skills/s/ref.md",
			content: "# reference doc\n",
			want:    adoptedSkillDoc,
		},
		{
			name:    "vibe skill manifest",
			path:    ".vibe/skills/cmd/SKILL.md",
			content: "---\nname: cmd\ndescription: Vibe skill\n---\nVibe body.\n",
			want:    adoptedSkill,
		},
		{
			name:    "claude rule",
			path:    ".claude/rules/x.md",
			content: "---\npaths: [src/**]\n---\nRule body.\n",
			want:    adoptedRule,
		},
		{
			name:    "opencode agent",
			path:    ".opencode/agents/oa.md",
			content: "---\nname: oa\ndescription: OC agent\ntools:\n  \"*\": false\n  read: true\n---\nAgent body.\n",
			want:    adoptedAgent,
		},
		{
			name:    "claude agent",
			path:    ".claude/agents/a.md",
			content: "---\nname: a\ndescription: An agent\n---\nAgent body.\n",
			want:    adoptedAgent,
		},
		{
			name:    "opencode command",
			path:    ".opencode/commands/c.md",
			content: "---\ndescription: A command\n---\nCommand body.\n",
			want:    adoptedCommand,
		},
		{
			name:    "unmapped path",
			path:    "some/unknown/path.md",
			content: "no mapping\n",
			want:    adoptedNone,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			seedAdoptFile(t, ws, tc.path, tc.content)

			got, err := adoptExternal(ws, tc.path)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("adoptExternal(%q): expected error, got nil", tc.path)
				}
			} else if err != nil {
				t.Fatalf("adoptExternal(%q): %v", tc.path, err)
			}
			if got != tc.want {
				t.Errorf("adoptExternal(%q) kind = %d, want %d", tc.path, got, tc.want)
			}
		})
	}
}
