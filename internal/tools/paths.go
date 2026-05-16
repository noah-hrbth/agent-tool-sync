package tools

import "strings"

// This file is the single owner of every tool's output-path vocabulary. Render
// funcs build paths from these constants; internal/syncer/adopt.go consumes the
// aggregate slices instead of re-typing literals; the render↔adopt contract
// test (internal/syncer/contract_test.go) consumes ExpectedAdoptOutcome so any
// drift between what a tool renders and what adopt.go reverses is a CI failure.

// Tool root directory prefixes (relative to the scope base dir).
const (
	claudeDir          = ".claude"
	opencodeDirProject = ".opencode"
	opencodeDirUser    = ".config/opencode"
	cursorDir          = ".cursor"
	geminiDir          = ".gemini"
	codexDir           = ".codex"
	codexSkillsProject = ".agents" // Codex auto-scans .agents/skills/ at project scope
	clineRulesProject  = ".clinerules"
	clineRulesUser     = "Documents/Cline/Rules"
	clineWorkflowsUser = "Documents/Cline/Workflows"
	clineSkillsDir     = ".cline"
	junieDir           = ".junie"
	vibeDir            = ".vibe"
	copilotDirProject  = ".github"
	copilotDirUser     = ".copilot"
	zedRulesFile       = ".rules"
)

// Concept sub-directory and skill manifest filename, shared by all tools.
const (
	skillsSub      = "skills"
	agentsSub      = "agents"
	commandsSub    = "commands"
	skillFile      = "SKILL.md"
	cursorCatchAll = ".cursor/rules/general.mdc"
)

// rootMemoryFiles are the exact paths adopt.go maps to canonical AGENTS.md
// (root memory). Mirrors the case-1 list in AdoptExternal.
func rootMemoryFiles() []string {
	return []string{
		"CLAUDE.md",
		".claude/CLAUDE.md",
		"AGENTS.md",
		".codex/AGENTS.md",
		".opencode/AGENTS.md",
		".config/opencode/AGENTS.md",
		"GEMINI.md",
		".gemini/GEMINI.md",
		".vibe/AGENTS.md",
		".github/copilot-instructions.md",
		".copilot/copilot-instructions.md",
	}
}

// SkillDirPrefixes returns every "<dir>/skills/" prefix adopt.go reverses to a
// canonical Skill. Single source for matchSkillPath.
func SkillDirPrefixes() []string {
	return []string{
		claudeDir + "/skills/",
		opencodeDirProject + "/skills/",
		opencodeDirUser + "/skills/",
		clineSkillsDir + "/skills/",
		junieDir + "/skills/",
		vibeDir + "/skills/",
		copilotDirProject + "/skills/",
		copilotDirUser + "/skills/",
	}
}

// AgentDirPrefixes returns every generic "<dir>/agents/" prefix adopt.go
// reverses to a canonical Agent (Copilot agents are matched separately).
func AgentDirPrefixes() []string {
	return []string{
		claudeDir + "/agents/",
		opencodeDirProject + "/agents/",
		opencodeDirUser + "/agents/",
		junieDir + "/agents/",
	}
}

// CommandDirPrefixes returns every generic "<dir>/commands/" prefix adopt.go
// reverses to a canonical Command (Cline workflows / Copilot prompts separate).
func CommandDirPrefixes() []string {
	return []string{
		claudeDir + "/commands/",
		opencodeDirProject + "/commands/",
		opencodeDirUser + "/commands/",
		junieDir + "/commands/",
	}
}

// RootMemoryFiles exposes rootMemoryFiles for adopt.go.
func RootMemoryFiles() []string { return rootMemoryFiles() }

// CursorCatchAll is Cursor's flattened AGENTS.md catch-all path.
const CursorCatchAll = cursorCatchAll

// OutcomeKind is what AdoptExternal is expected to do with a rendered path.
type OutcomeKind int

const (
	// OutcomeReversible: adopts back to a canonical entity of its own Concept.
	OutcomeReversible OutcomeKind = iota
	// OutcomeRootMemory: adopts to canonical AGENTS.md (root memory / catch-all).
	OutcomeRootMemory
	// OutcomeCrossMapped: adopts to a canonical entity of a different Concept.
	OutcomeCrossMapped
	// OutcomeNonReversible: AdoptExternal returns "no canonical mapping".
	OutcomeNonReversible
)

// AdoptOutcome is the declared expectation for one rendered path family.
type AdoptOutcome struct {
	Kind    OutcomeKind
	CrossTo Concept // set when Kind == OutcomeCrossMapped
	Reason  string  // documents why, when Kind == OutcomeNonReversible
}

// ExpectedAdoptOutcome declares, per (tool, concept, rendered path), what
// AdoptExternal does today. The default is OutcomeReversible — anything not
// explicitly excepted MUST reverse to its own Concept, so adding an
// unhandled render path makes the contract test fail (drift caught).
func ExpectedAdoptOutcome(toolKey string, concept Concept, path string) AdoptOutcome {
	for _, rm := range rootMemoryFiles() {
		if path == rm {
			return AdoptOutcome{Kind: OutcomeRootMemory}
		}
	}
	if path == cursorCatchAll {
		return AdoptOutcome{Kind: OutcomeRootMemory}
	}

	// Vibe renders commands as user-invocable skills; they reverse as Skills.
	if toolKey == "vibe" && concept == ConceptCommands {
		return AdoptOutcome{Kind: OutcomeCrossMapped, CrossTo: ConceptSkills}
	}

	const noMatcher = "no reverse matcher in adopt.go (future bidirectional work)"
	switch toolKey {
	case "zed":
		// Only output is .rules — not in adopt's root-memory list.
		return AdoptOutcome{Kind: OutcomeNonReversible, Reason: noMatcher}
	case "gemini":
		if concept == ConceptSkills || concept == ConceptAgents || concept == ConceptCommands {
			return AdoptOutcome{Kind: OutcomeNonReversible, Reason: noMatcher}
		}
	case "cursor":
		if strings.HasPrefix(path, cursorDir+"/skills/") ||
			strings.HasPrefix(path, cursorDir+"/agents/") ||
			strings.HasPrefix(path, cursorDir+"/commands/") {
			return AdoptOutcome{Kind: OutcomeNonReversible, Reason: noMatcher}
		}
	case "codex":
		if concept == ConceptSkills || concept == ConceptAgents {
			return AdoptOutcome{Kind: OutcomeNonReversible, Reason: noMatcher}
		}
	case "vibe":
		if concept == ConceptAgents {
			// .vibe/agents/*.toml + .vibe/prompts/*.md — TOML-agent gap.
			return AdoptOutcome{Kind: OutcomeNonReversible, Reason: "Vibe TOML agents are deferred (parity with Codex)"}
		}
	}

	return AdoptOutcome{Kind: OutcomeReversible}
}
