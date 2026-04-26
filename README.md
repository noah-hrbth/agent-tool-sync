# agentsync

Maintain AI tool configs in one place and sync them to Claude Code, OpenCode, Cursor, Gemini CLI, and Codex CLI.

## Install

```bash
# macOS / Homebrew
brew install noah-hrbth/agentsync/agentsync

# Any OS — install script
curl -sSL https://raw.githubusercontent.com/noah-hrbth/agent-tool-sync/main/scripts/install.sh | sh

# Manual download
# GitHub Releases: https://github.com/noah-hrbth/agent-tool-sync/releases
```

## Quickstart

1. Scaffold the canonical source:
   ```bash
   agentsync init
   ```
2. Edit `.agentsync/AGENTS.md` with your rules, then add skills, agents, and commands under the corresponding subdirectories.
3. Sync to all enabled tools:
   ```bash
   agentsync sync      # headless
   agentsync           # TUI
   ```

## Supported AI tools

| Tool | Rules | Skills | Agents | Commands | Detection |
|---|---|---|---|---|---|
| Claude Code | `.claude/CLAUDE.md` | `.claude/skills/<dir>/SKILL.md` | `.claude/agents/<name>.md` | `.claude/commands/<name>.md` | `.claude/` folder |
| OpenCode | `AGENTS.md` | `.opencode/skills/<dir>/SKILL.md` | `.opencode/agents/<name>.md` | `.opencode/commands/<name>.md` | `.opencode/` folder |
| Cursor | `.cursor/rules/general.mdc` | `.cursor/skills/<dir>/SKILL.md` | `.cursor/agents/<name>.md` | `.cursor/commands/<name>.md` | `.cursor/` folder |
| Gemini CLI | `GEMINI.md` | — | — | — | `.gemini/` folder or `gemini` in `$PATH` |
| Codex CLI | `AGENTS.md` | — | — | — | `.codex/` folder or `codex` in `$PATH` |

`AGENTS.md` at the workspace root is shared by OpenCode and Codex CLI — both tools read it natively.

`—` means the tool doesn't support that concept. `agentsync` skips those files and shows a compatibility badge in the TUI.

## The `.agentsync/` folder

```
.agentsync/
├── config.yaml             # per-tool enable/disable
├── AGENTS.md               # canonical rules (synced to all tools)
├── skills/
│   └── <name>/
│       └── SKILL.md        # frontmatter + instructions
├── agents/
│   └── <name>.md           # frontmatter + system prompt
├── commands/
│   └── <name>.md           # frontmatter + prompt body
└── .state/                 # internal sync state (gitignored)
    └── snapshot.json
```

### Frontmatter schemas

**Skills** (`skills/<name>/SKILL.md`):

```yaml
---
name: skill-name              # required, ≤64 chars, lowercase + hyphens
description: ...              # required, ≤1024 chars — what it does + when to use it
allowed-tools: [Read, Bash]   # optional
disable-model-invocation: false  # optional
---
Skill instructions in markdown.
```

**Agents** (`agents/<name>.md`):

```yaml
---
name: agent-name
description: ...              # routing trigger — describe when to invoke
tools: [Read, Glob, Grep]     # optional; omit to inherit thread tools
model: sonnet                 # optional: sonnet | opus | haiku | inherit
---
Agent system prompt in markdown.
```

**Commands** (`commands/<name>.md`):

```yaml
---
description: ...
argument-hint: "[issue-number]"   # optional — shown as hint in TUI
allowed-tools: [Read, Bash]       # optional
model: sonnet                     # optional
---
Command prompt body in markdown.
```

## Sync controls

**Enable/disable tools:** edit `.agentsync/config.yaml` or use the Tools screen in the TUI.

**Sync direction:** always canonical → tools (one-way). To adopt external changes, use the TUI's divergence resolution.

**Divergence detection:** `agentsync` tracks a SHA-256 hash of every written file in `.agentsync/.state/snapshot.json`. When a tool's file changes externally, `agentsync` detects it and marks it `▲ divergent`.

**Divergence resolution (TUI only):** per-file choice of:
- **Adopt** — pull external changes into canonical
- **Overwrite** — discard the external edit
- **Defer** — leave it, sync the rest

**Status icons in TUI:** `●` synced, `▲` divergent, `○` missing, `+` not yet synced

## Concept compatibility

| Concept | Claude Code | OpenCode | Cursor | Gemini CLI | Codex CLI |
|---|---|---|---|---|---|
| Rules | ✓ | ✓ | ✓ | ✓ | ✓ |
| Skills | ✓ | ✓ | ✓ | — | — |
| Agents | ✓ | ✓ | ✓ | — | — |
| Commands | ✓ | ✓ | ✓ | — | — |

When editing a skill, agent, or command in the TUI, tools that don't support that concept are shown with `✗` and a reason, and are skipped during sync.

## CLI reference

```
agentsync                         Launch TUI (default)
agentsync init                    Scaffold .agentsync/ with sample AGENTS.md + config.yaml
agentsync sync                    Headless one-way sync; exits non-zero on unresolved divergences
agentsync status                  Print sync status for all files (●/▲/○/+)
agentsync version                 Print version

Flags:
  --workspace <path>           Target directory (default: current directory)
```

## Roadmap

- More tools: Windsurf, Continue.dev, Cline, Roo Code, GitHub Copilot, JetBrains Junie, Crush, Goose, Amazon Q, Kilo Code, Aider, Zed, Cody
- Bidirectional sync with `agentsync pull`
- File watcher / `--watch` flag
- AGENTS.md standard frontmatter extensions for skills/agents/commands

## Contributing — adding a new tool

The adapter interface is defined in [`internal/tools/adapter.go`](internal/tools/adapter.go) and has four methods:

- `Name()` — returns the tool's display name
- `Detect(workspace string) bool` — reports whether the tool is present in the workspace
- `Supports(concept string) bool` — reports whether the tool supports a given concept (rules/skills/agents/commands)
- `Render(canonical *canonical.Source, workspace string) error` — writes the tool's native files from the canonical source

See [`internal/tools/claude.go`](internal/tools/claude.go) for a reference implementation.

To add a new tool: implement the interface, then register it in [`internal/tools/registry.go`](internal/tools/registry.go) by adding an instance to the slice returned by `All()`.
