package syncer

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adrg/frontmatter"
	"github.com/noah-hrbth/agentsync/internal/canonical"
	"github.com/noah-hrbth/agentsync/internal/safepath"
	"github.com/noah-hrbth/agentsync/internal/tools"
)

// AdoptExternal reads the divergent file at <workspace>/<path>, maps it back to
// the matching canonical entity, and persists the canonical update.
// The caller must reload canonical from disk after this returns.
//
// INVARIANT: the switch case order is load-bearing. Tool-specific matchers
// (Cursor general.mdc, Cline rules/workflows, Copilot instructions/agents/
// prompts) MUST precede the generic matchers — their paths or suffixes would
// otherwise be mis-claimed by matchRulePath/matchAgentPath/matchCommandPath.
// matchSkillPath also MUST precede matchRulePath: a skill doc under a
// "<skill>/rules/" subfolder would otherwise be claimed by the generic /rules/
// matcher. internal/syncer/contract_test.go fails if this ordering (or the
// shared path vocabulary in internal/tools/paths.go) drifts from what render emits.
func AdoptExternal(workspace, path string) error {
	data, err := safepath.ReadFile(workspace, path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	content := string(data)

	switch {
	case isRootMemoryFile(path):
		return canonical.SaveAgentsMD(workspace, content)

	case path == tools.CursorCatchAll:
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

	case matchPiPromptPath(path):
		// Pi prompts (slash commands) support description and argument-hint only.
		var fm struct {
			Description  string `yaml:"description"`
			ArgumentHint string `yaml:"argument-hint"`
		}
		body, err := frontmatter.Parse(strings.NewReader(content), &fm)
		if err != nil {
			return fmt.Errorf("parse pi prompt frontmatter: %w", err)
		}
		cmd := canonical.Command{
			Filename:     strings.TrimSuffix(filepath.Base(path), ".md"),
			Description:  fm.Description,
			ArgumentHint: fm.ArgumentHint,
			Body:         string(body),
		}
		return canonical.SaveCommand(workspace, &cmd)

	case matchSkillPath(path):
		// Must precede matchRulePath: a skill doc may legally live under a
		// "<skill>/rules/" subfolder, which the generic /rules/ matcher would
		// otherwise claim. matchSkillPath only matches explicit /skills/ prefixes,
		// so real rule files are never stolen here.
		// SKILL.md is the manifest (typed frontmatter); any other .md is a plain
		// skill doc persisted verbatim under the same skill dir.
		if filepath.Base(path) != "SKILL.md" {
			return canonical.SaveSkillDoc(workspace, skillDir(path), skillDocRelPath(path), content)
		}
		var s canonical.Skill
		body, err := frontmatter.Parse(strings.NewReader(content), &s)
		if err != nil {
			return fmt.Errorf("parse skill frontmatter: %w", err)
		}
		s.Dir = skillDir(path)
		s.Body = string(body)
		return canonical.SaveSkill(workspace, &s)

	case matchRulePath(path):
		var r canonical.Rule
		body, err := frontmatter.Parse(strings.NewReader(content), &r)
		if err != nil {
			return fmt.Errorf("parse rule frontmatter: %w", err)
		}
		r.Filename = ruleFilename(path)
		r.Body = string(body)
		return canonical.SaveRule(workspace, &r)

	case matchOpenCodeAgentPath(path):
		// OpenCode renders agent `tools` as an object (allowlist), so the shared
		// canonical.Agent parser (Tools []string) cannot read it. Parse into a local
		// shape and reverse the object back to a canonical allowlist.
		var fm struct {
			Name        string          `yaml:"name"`
			Description string          `yaml:"description"`
			Tools       map[string]bool `yaml:"tools"`
			Model       string          `yaml:"model"`
		}
		body, err := frontmatter.Parse(strings.NewReader(content), &fm)
		if err != nil {
			return fmt.Errorf("parse opencode agent frontmatter: %w", err)
		}
		a := canonical.Agent{
			Name:        fm.Name,
			Description: fm.Description,
			Model:       fm.Model,
			Tools:       openCodeToolsToAllowlist(fm.Tools),
			Filename:    strings.TrimSuffix(filepath.Base(path), ".md"),
			Body:        string(body),
		}
		return canonical.SaveAgent(workspace, &a)

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

// isRootMemoryFile reports whether path is one of the tool root-memory files
// that reverse to canonical AGENTS.md. The set is owned by internal/tools.
func isRootMemoryFile(path string) bool {
	for _, f := range tools.RootMemoryFiles() {
		if path == f {
			return true
		}
	}
	return false
}

// matchSkillPath reports whether path is a skill file (the SKILL.md manifest or
// any other .md skill doc) under one of the tools' skill dirs. Requires at least
// "<prefix><dir>/<file>.md" so a stray .md directly under skills/ is not claimed.
func matchSkillPath(path string) bool {
	for _, prefix := range tools.SkillDirPrefixes() {
		if !strings.HasPrefix(path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(path, prefix)
		if strings.Contains(rest, "/") && strings.HasSuffix(rest, ".md") {
			return true
		}
	}
	return false
}

// skillDocRelPath returns the skill-doc path relative to its skill dir, e.g.
// ".claude/skills/foo/examples/x.md" → "examples/x.md".
func skillDocRelPath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if p == "skills" && i+2 < len(parts) {
			return strings.Join(parts[i+2:], "/")
		}
	}
	return ""
}

// hasAnyPrefix reports whether path begins with any of the prefixes.
func hasAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
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
	return strings.HasSuffix(path, ".md") && hasAnyPrefix(path, tools.AgentDirPrefixes())
}

// matchOpenCodeAgentPath matches OpenCode agent files at either scope. These need
// a dedicated reverse parse (OpenCode renders `tools` as an object) and so are
// handled before the generic matchAgentPath in the AdoptExternal switch.
func matchOpenCodeAgentPath(path string) bool {
	return strings.HasSuffix(path, ".md") && hasAnyPrefix(path, tools.OpenCodeAgentDirPrefixes())
}

// openCodeToolsToAllowlist reverses an OpenCode agent `tools` object back to a
// canonical allowlist: keys enabled (true), minus the deny-all sentinel "*",
// mapped to canonical tool names and sorted for deterministic output.
//
// Limitation: a hand-authored deny-all-only object (`{"*": false}`, nothing
// re-enabled) reverses to an empty allowlist, which canonical renders as "all
// tools" — the inverse of intent. canonical.Agent.Tools ([]string, omitempty) has
// no representation for "zero tools", so adopt cannot distinguish the two. render
// never emits the deny-all-only form, so this only bites manual edits.
func openCodeToolsToAllowlist(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	var out []string
	for k, enabled := range m {
		if k == "*" || !enabled {
			continue
		}
		out = append(out, tools.CanonicalToolName(k))
	}
	sort.Strings(out)
	return out
}

func matchCommandPath(path string) bool {
	return strings.HasSuffix(path, ".md") && hasAnyPrefix(path, tools.CommandDirPrefixes())
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

// matchPiPromptPath returns true for Pi prompt files (commands concept).
// Pi prompts are at .pi/prompts/<name>.md (project) or .pi/agent/prompts/<name>.md (user).
// Single-level only — nested subdirectories under prompts/ are not adopted.
func matchPiPromptPath(path string) bool {
	if !strings.HasSuffix(path, ".md") {
		return false
	}
	var rest string
	switch {
	case strings.HasPrefix(path, ".pi/prompts/"):
		rest = strings.TrimPrefix(path, ".pi/prompts/")
	case strings.HasPrefix(path, ".pi/agent/prompts/"):
		rest = strings.TrimPrefix(path, ".pi/agent/prompts/")
	default:
		return false
	}
	return !strings.Contains(rest, "/")
}
