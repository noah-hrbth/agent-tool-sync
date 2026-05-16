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
		path == ".gemini/GEMINI.md" ||
		path == ".vibe/AGENTS.md" ||
		path == ".github/copilot-instructions.md" ||
		path == ".copilot/copilot-instructions.md":
		return canonical.SaveAgentsMD(workspace, content)

	case path == ".cursor/rules/general.mdc":
		// Strip the frontmatter wrapper added by the Cursor adapter.
		var discard map[string]interface{}
		rest, err := frontmatter.Parse(strings.NewReader(content), &discard)
		if err != nil {
			return fmt.Errorf("parse cursor frontmatter: %w", err)
		}
		return canonical.SaveAgentsMD(workspace, string(rest))

	case matchClineWorkflowPath(path):
		// Cline workflows have no frontmatter; the body is the prompt.
		var cmd canonical.Command
		cmd.Filename = strings.TrimSuffix(filepath.Base(path), ".md")
		cmd.Body = content
		return canonical.SaveCommand(workspace, &cmd)

	case matchClineRulePath(path):
		var r canonical.Rule
		body, err := frontmatter.Parse(strings.NewReader(content), &r)
		if err != nil {
			return fmt.Errorf("parse cline rule frontmatter: %w", err)
		}
		r.Filename = strings.TrimSuffix(filepath.Base(path), ".md")
		r.Body = string(body)
		return canonical.SaveRule(workspace, &r)

	case matchCopilotInstructionPath(path):
		// Copilot uses `applyTo:` (single glob string) instead of `paths:` array.
		var fm struct {
			ApplyTo     string `yaml:"applyTo"`
			Description string `yaml:"description"`
		}
		body, err := frontmatter.Parse(strings.NewReader(content), &fm)
		if err != nil {
			return fmt.Errorf("parse copilot instruction frontmatter: %w", err)
		}
		var r canonical.Rule
		r.Filename = strings.TrimSuffix(filepath.Base(path), ".instructions.md")
		r.Description = fm.Description
		if fm.ApplyTo != "" {
			r.Paths = []string{fm.ApplyTo}
		}
		r.Body = string(body)
		return canonical.SaveRule(workspace, &r)

	case matchCopilotAgentPath(path):
		var a canonical.Agent
		body, err := frontmatter.Parse(strings.NewReader(content), &a)
		if err != nil {
			return fmt.Errorf("parse copilot agent frontmatter: %w", err)
		}
		a.Filename = strings.TrimSuffix(filepath.Base(path), ".agent.md")
		a.Body = string(body)
		return canonical.SaveAgent(workspace, &a)

	case matchCopilotPromptPath(path):
		// Copilot prompts use `tools:` (not `allowed-tools:`).
		var fm struct {
			Description  string   `yaml:"description"`
			ArgumentHint string   `yaml:"argument-hint"`
			Tools        []string `yaml:"tools"`
			Model        string   `yaml:"model"`
		}
		body, err := frontmatter.Parse(strings.NewReader(content), &fm)
		if err != nil {
			return fmt.Errorf("parse copilot prompt frontmatter: %w", err)
		}
		cmd := canonical.Command{
			Filename:     strings.TrimSuffix(filepath.Base(path), ".prompt.md"),
			Description:  fm.Description,
			ArgumentHint: fm.ArgumentHint,
			AllowedTools: fm.Tools,
			Model:        fm.Model,
			Body:         string(body),
		}
		return canonical.SaveCommand(workspace, &cmd)

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
		strings.HasPrefix(path, ".config/opencode/skills/") ||
		strings.HasPrefix(path, ".cline/skills/") ||
		strings.HasPrefix(path, ".junie/skills/") ||
		strings.HasPrefix(path, ".vibe/skills/") ||
		strings.HasPrefix(path, ".github/skills/") ||
		strings.HasPrefix(path, ".copilot/skills/")) &&
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
		strings.HasPrefix(path, ".config/opencode/agents/") ||
		strings.HasPrefix(path, ".junie/agents/")) &&
		strings.HasSuffix(path, ".md")
}

func matchCommandPath(path string) bool {
	return (strings.HasPrefix(path, ".claude/commands/") ||
		strings.HasPrefix(path, ".opencode/commands/") ||
		strings.HasPrefix(path, ".config/opencode/commands/") ||
		strings.HasPrefix(path, ".junie/commands/")) &&
		strings.HasSuffix(path, ".md")
}

// matchClineRulePath returns true for Cline per-rule files at either scope.
// Project: .clinerules/<name>.md (excluding the workflows/ subdirectory).
// User: Documents/Cline/Rules/<name>.md.
func matchClineRulePath(path string) bool {
	if !strings.HasSuffix(path, ".md") {
		return false
	}
	if strings.Contains(path, "/workflows/") {
		return false
	}
	if strings.HasPrefix(path, ".clinerules/") {
		// Reject deeper-than-one-level paths under .clinerules/ that aren't workflows
		// (workflows already filtered above). e.g. .clinerules/foo.md ✓, .clinerules/sub/foo.md ✗.
		rest := strings.TrimPrefix(path, ".clinerules/")
		return !strings.Contains(rest, "/")
	}
	if strings.HasPrefix(path, "Documents/Cline/Rules/") {
		rest := strings.TrimPrefix(path, "Documents/Cline/Rules/")
		return !strings.Contains(rest, "/")
	}
	return false
}

// matchClineWorkflowPath returns true for Cline workflows (commands concept) at either scope.
// Project: .clinerules/workflows/<name>.md. User: Documents/Cline/Workflows/<name>.md.
func matchClineWorkflowPath(path string) bool {
	if !strings.HasSuffix(path, ".md") {
		return false
	}
	return strings.HasPrefix(path, ".clinerules/workflows/") ||
		strings.HasPrefix(path, "Documents/Cline/Workflows/")
}

// matchCopilotInstructionPath returns true for Copilot per-file rules at either
// scope: .github/instructions/<name>.instructions.md or .copilot/instructions/<name>.instructions.md.
// Single-level only — nested subdirectories under instructions/ are not adopted.
func matchCopilotInstructionPath(path string) bool {
	if !strings.HasSuffix(path, ".instructions.md") {
		return false
	}
	var rest string
	switch {
	case strings.HasPrefix(path, ".github/instructions/"):
		rest = strings.TrimPrefix(path, ".github/instructions/")
	case strings.HasPrefix(path, ".copilot/instructions/"):
		rest = strings.TrimPrefix(path, ".copilot/instructions/")
	default:
		return false
	}
	return !strings.Contains(rest, "/")
}

// matchCopilotAgentPath returns true for Copilot custom agents at either scope:
// .github/agents/<name>.agent.md or .copilot/agents/<name>.agent.md.
func matchCopilotAgentPath(path string) bool {
	if !strings.HasSuffix(path, ".agent.md") {
		return false
	}
	var rest string
	switch {
	case strings.HasPrefix(path, ".github/agents/"):
		rest = strings.TrimPrefix(path, ".github/agents/")
	case strings.HasPrefix(path, ".copilot/agents/"):
		rest = strings.TrimPrefix(path, ".copilot/agents/")
	default:
		return false
	}
	return !strings.Contains(rest, "/")
}

// matchCopilotPromptPath returns true for Copilot prompt files (commands concept).
// Project scope only — Copilot has no documented user-level prompts directory.
func matchCopilotPromptPath(path string) bool {
	if !strings.HasSuffix(path, ".prompt.md") {
		return false
	}
	if !strings.HasPrefix(path, ".github/prompts/") {
		return false
	}
	rest := strings.TrimPrefix(path, ".github/prompts/")
	return !strings.Contains(rest, "/")
}
