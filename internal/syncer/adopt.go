package syncer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/noah-hrbth/agentsync/internal/canonical"
)

// AdoptExternal reads the divergent file at <workspace>/<path>, maps it back to
// the matching canonical entity, and persists the canonical update.
// The caller must reload canonical from disk after this returns.
func AdoptExternal(workspace, path string) error {
	absPath := filepath.Join(workspace, path)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	content := string(data)

	switch {
	case path == "CLAUDE.md" ||
		path == ".claude/CLAUDE.md" ||
		path == "AGENTS.md" ||
		path == ".codex/AGENTS.md" ||
		path == ".opencode/AGENTS.md" ||
		path == ".config/opencode/AGENTS.md" ||
		path == "GEMINI.md" ||
		path == ".gemini/GEMINI.md":
		return canonical.SaveAgentsMD(workspace, content)

	case path == ".cursor/rules/general.mdc":
		// Strip the frontmatter wrapper added by the Cursor adapter.
		var discard map[string]interface{}
		rest, err := frontmatter.Parse(strings.NewReader(content), &discard)
		if err != nil {
			return fmt.Errorf("parse cursor frontmatter: %w", err)
		}
		return canonical.SaveAgentsMD(workspace, string(rest))

	case matchRulePath(path):
		var r canonical.Rule
		body, err := frontmatter.Parse(strings.NewReader(content), &r)
		if err != nil {
			return fmt.Errorf("parse rule frontmatter: %w", err)
		}
		r.Filename = ruleFilename(path)
		r.Body = string(body)
		return canonical.SaveRule(workspace, &r)

	case matchSkillPath(path):
		var s canonical.Skill
		body, err := frontmatter.Parse(strings.NewReader(content), &s)
		if err != nil {
			return fmt.Errorf("parse skill frontmatter: %w", err)
		}
		s.Dir = skillDir(path)
		s.Body = string(body)
		return canonical.SaveSkill(workspace, &s)

	case matchAgentPath(path):
		var a canonical.Agent
		body, err := frontmatter.Parse(strings.NewReader(content), &a)
		if err != nil {
			return fmt.Errorf("parse agent frontmatter: %w", err)
		}
		a.Filename = strings.TrimSuffix(filepath.Base(path), ".md")
		a.Body = string(body)
		return canonical.SaveAgent(workspace, &a)

	case matchCommandPath(path):
		var cmd canonical.Command
		body, err := frontmatter.Parse(strings.NewReader(content), &cmd)
		if err != nil {
			return fmt.Errorf("parse command frontmatter: %w", err)
		}
		cmd.Filename = strings.TrimSuffix(filepath.Base(path), ".md")
		cmd.Body = string(body)
		return canonical.SaveCommand(workspace, &cmd)

	default:
		return fmt.Errorf("adopt: no canonical mapping for path %q", path)
	}
}

// matchRulePath returns true for per-rule files in tools' rules directories,
// excluding Cursor's catch-all general.mdc (handled above as AGENTS.md).
func matchRulePath(path string) bool {
	if !strings.Contains(path, "/rules/") {
		return false
	}
	base := filepath.Base(path)
	// general.mdc is the rendered AGENTS.md catch-all; it is NOT a canonical rule.
	return base != "general.mdc"
}

// ruleFilename extracts the canonical rule filename (without extension) from a tool path.
func ruleFilename(path string) string {
	base := filepath.Base(path)
	// Strip either .md or .mdc extension.
	if strings.HasSuffix(base, ".mdc") {
		return strings.TrimSuffix(base, ".mdc")
	}
	return strings.TrimSuffix(base, ".md")
}

func matchSkillPath(path string) bool {
	return (strings.HasPrefix(path, ".claude/skills/") ||
		strings.HasPrefix(path, ".opencode/skills/") ||
		strings.HasPrefix(path, ".config/opencode/skills/")) &&
		strings.HasSuffix(path, "/SKILL.md")
}

func skillDir(path string) string {
	// .claude/skills/<dir>/SKILL.md           → parts[2]
	// .config/opencode/skills/<dir>/SKILL.md  → parts[3]
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "skills" && i+1 < len(parts)-1 {
			return parts[i+1]
		}
	}
	return ""
}

func matchAgentPath(path string) bool {
	return (strings.HasPrefix(path, ".claude/agents/") ||
		strings.HasPrefix(path, ".opencode/agents/") ||
		strings.HasPrefix(path, ".config/opencode/agents/")) &&
		strings.HasSuffix(path, ".md")
}

func matchCommandPath(path string) bool {
	return (strings.HasPrefix(path, ".claude/commands/") ||
		strings.HasPrefix(path, ".opencode/commands/") ||
		strings.HasPrefix(path, ".config/opencode/commands/")) &&
		strings.HasSuffix(path, ".md")
}
