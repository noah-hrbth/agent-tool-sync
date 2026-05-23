package tools_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

func scopeName(s tools.Scope) string {
	if s == tools.ScopeUser {
		return "user"
	}
	return "project"
}

func skillDocProbe() *canonical.Canonical {
	return &canonical.Canonical{
		Skills: []*canonical.Skill{{
			Dir:  "pdf-tools",
			Name: "pdf-tools",
			Body: "# manifest\n",
			Docs: []canonical.SkillDoc{
				{RelPath: "reference.md", Content: "# reference\n"},
				{RelPath: "examples/invoice.md", Content: "# invoice\n"},
			},
		}},
	}
}

func findWrite(writes []tools.FileWrite, path string) *tools.FileWrite {
	for i := range writes {
		if writes[i].Path == path {
			return &writes[i]
		}
	}
	return nil
}

// manifestSkillDir returns the dir holding the pdf-tools SKILL.md, derived from
// the rendered manifest path so the test never hardcodes per-tool bases.
func manifestSkillDir(writes []tools.FileWrite) string {
	for _, w := range writes {
		if w.Concept == tools.ConceptSkills &&
			filepath.Base(w.Path) == "SKILL.md" &&
			filepath.Base(filepath.Dir(w.Path)) == "pdf-tools" {
			return filepath.Dir(w.Path)
		}
	}
	return ""
}

func TestRenderEmitsSkillDocsBesideManifest(t *testing.T) {
	for _, tool := range tools.All() {
		if !tool.Meta.Supports(tools.ConceptSkills).Supported {
			continue
		}
		for _, scope := range []tools.Scope{tools.ScopeProject, tools.ScopeUser} {
			if !tool.Meta.SupportsScope(scope).Supported {
				continue
			}
			tool, scope := tool, scope
			t.Run(tool.Meta.Key+"/"+scopeName(scope), func(t *testing.T) {
				// Act
				writes, err := tool.Render(skillDocProbe(), scope)
				if err != nil {
					t.Fatalf("render: %v", err)
				}

				// Assert
				dir := manifestSkillDir(writes)
				if dir == "" {
					t.Fatalf("no pdf-tools SKILL.md manifest emitted")
				}
				for _, want := range []struct{ rel, content string }{
					{"reference.md", "# reference\n"},
					{filepath.Join("examples", "invoice.md"), "# invoice\n"},
				} {
					p := filepath.Join(dir, want.rel)
					w := findWrite(writes, p)
					if w == nil {
						t.Fatalf("skill doc not emitted at %s; writes=%v", p, writes)
					}
					if string(w.Content) != want.content {
						t.Errorf("%s content = %q, want %q", p, w.Content, want.content)
					}
					if w.Concept != tools.ConceptSkills {
						t.Errorf("%s concept = %v, want skills", p, w.Concept)
					}
				}
			})
		}
	}
}

func TestRenderEmitsOnlyManifestWhenSkillHasNoDocs(t *testing.T) {
	c := &canonical.Canonical{Skills: []*canonical.Skill{{Dir: "pdf-tools", Name: "pdf-tools", Body: "# m\n"}}}
	for _, tool := range tools.All() {
		if !tool.Meta.Supports(tools.ConceptSkills).Supported || !tool.Meta.SupportsScope(tools.ScopeProject).Supported {
			continue
		}
		tool := tool
		t.Run(tool.Meta.Key, func(t *testing.T) {
			writes, err := tool.Render(c, tools.ScopeProject)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			dir := manifestSkillDir(writes)
			if dir == "" {
				t.Fatalf("no pdf-tools SKILL.md manifest emitted")
			}
			prefix := dir + string(filepath.Separator)
			count := 0
			for _, w := range writes {
				if strings.HasPrefix(w.Path, prefix) {
					count++
				}
			}
			if count != 1 {
				t.Errorf("want exactly 1 file under %s (manifest only), got %d", dir, count)
			}
		})
	}
}

func TestZedEmitsNoSkillDocs(t *testing.T) {
	var zed tools.Tool
	for _, tool := range tools.All() {
		if tool.Meta.Key == "zed" {
			zed = tool
		}
	}
	if zed.Meta.Key == "" {
		t.Skip("zed tool not registered")
	}
	writes, err := zed.Render(skillDocProbe(), tools.ScopeProject)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, w := range writes {
		if w.Concept == tools.ConceptSkills {
			t.Errorf("zed must not emit skill files, got %s", w.Path)
		}
	}
}
