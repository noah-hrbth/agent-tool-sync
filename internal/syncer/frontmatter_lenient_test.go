package syncer

import (
	"strings"
	"testing"
)

func TestSanitizeGlobFrontmatter(t *testing.T) {
	cases := []struct {
		name        string
		in          string
		wantChanged bool
		wantContain string // substring expected in the (possibly) rewritten frontmatter
	}{
		{
			name:        "flow sequence unquoted globs",
			in:          "---\npaths: [**/*.ts, **/*.tsx]\n---\nbody\n",
			wantChanged: true,
			wantContain: "paths: ['**/*.ts', '**/*.tsx']",
		},
		{
			name:        "brace group internal comma preserved",
			in:          "---\nglobs: [**/*.{ts,tsx}]\n---\n",
			wantChanged: true,
			wantContain: "globs: ['**/*.{ts,tsx}']",
		},
		{
			name:        "bare scalar glob",
			in:          "---\napplyTo: **/*.ts\n---\n",
			wantChanged: true,
			wantContain: "applyTo: '**/*.ts'",
		},
		{
			name:        "block sequence items",
			in:          "---\npaths:\n  - **/*.ts\n  - src/**/*.go\n---\n",
			wantChanged: true,
			wantContain: "  - '**/*.ts'",
		},
		{
			name:        "already-quoted untouched",
			in:          "---\npaths: ['**/*.ts']\n---\n",
			wantChanged: false,
		},
		{
			name:        "letter-leading globs untouched",
			in:          "---\npaths: [src/**/*.ts, test/**/*.ts]\n---\n",
			wantChanged: false,
		},
		{
			name:        "non-glob key untouched",
			in:          "---\nname: thing\ndescription: x\n---\n",
			wantChanged: false,
		},
		{
			name:        "no frontmatter fence",
			in:          "# just a body\npaths: [**/*.ts]\n",
			wantChanged: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, changed := sanitizeGlobFrontmatter(c.in)
			if changed != c.wantChanged {
				t.Fatalf("changed: got %v, want %v\noutput:\n%s", changed, c.wantChanged, got)
			}
			if !c.wantChanged && got != c.in {
				t.Errorf("unchanged input mutated:\ngot:\n%s", got)
			}
			if c.wantContain != "" && !strings.Contains(got, c.wantContain) {
				t.Errorf("want substring %q in:\n%s", c.wantContain, got)
			}
		})
	}
}

func TestSplitFlowItemsBraceAware(t *testing.T) {
	got := splitFlowItems("**/*.{ts,tsx}, **/*.css")
	if len(got) != 2 {
		t.Fatalf("want 2 items, got %d: %v", len(got), got)
	}
	if strings.TrimSpace(got[0]) != "**/*.{ts,tsx}" {
		t.Errorf("brace comma split: got %q", got[0])
	}
}
