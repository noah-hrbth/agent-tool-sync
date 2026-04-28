# agentsync

Maintain AI tool configs in one place and sync them to Claude Code, OpenCode, Cursor, Gemini CLI, and Codex CLI.

## Install

```bash
# Manual download
# GitHub Releases: https://github.com/noah-hrbth/agent-tool-sync/releases
```

> Homebrew tap and install script are planned but not yet available.

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

| Tool | Root memory | Per-rule files | Skills | Agents | Commands | Detection |
|---|---|---|---|---|---|---|
| Claude Code | `.claude/CLAUDE.md` | `.claude/rules/<name>.md` | `.claude/skills/<dir>/SKILL.md` | `.claude/agents/<name>.md` | `.claude/commands/<name>.md ⚠` | `~/.claude/` |
| OpenCode | `.opencode/AGENTS.md` | appended to root | `.opencode/skills/<dir>/SKILL.md` | `.opencode/agents/<name>.md` | `.opencode/commands/<name>.md` | `~/.opencode/` |
| Cursor | `.cursor/rules/general.mdc` | `.cursor/rules/<name>.mdc` | `.cursor/skills/<dir>/SKILL.md` | `.cursor/agents/<name>.md` | `.cursor/commands/<name>.md ⚠` | `~/.cursor/` |
| Gemini CLI | `.gemini/GEMINI.md` | appended to root | `.gemini/skills/<dir>/SKILL.md` | `.gemini/agents/<name>.md` | `.gemini/commands/<name>.toml` | `~/.gemini/` |
| Codex CLI | `.codex/AGENTS.md` | appended to root | `.agents/skills/<dir>/SKILL.md` | `.codex/agents/<name>.toml` | `⚠ deprecated → skills` | `~/.codex/` |

`AGENTS.md` at the workspace root is shared by OpenCode and Codex CLI — both tools read it natively.

`—` means the tool doesn't support that concept. `agentsync` skips those files and shows a compatibility badge in the TUI.

## The `.agentsync/` folder

```
.agentsync/
├── config.yaml             # per-tool enable/disable
├── AGENTS.md               # root memory file (synced to all tools)
├── rules/
│   └── <name>.md           # frontmatter + rule body (per-file where supported)
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

Rules in `.agentsync/rules/` are synced per-file to tools that support a rules directory (Claude Code → `.claude/rules/<name>.md`, Cursor → `.cursor/rules/<name>.mdc`). Tools without a per-rule directory (Gemini CLI, OpenCode, Codex CLI) receive rule bodies appended as `##`-headed sections to their root memory file.

The filename `general` is reserved — it maps to Cursor's `general.mdc` catch-all and cannot be used as a canonical rule name.

### Frontmatter schemas

**Rules** (`rules/<name>.md`):

```yaml
---
description: ...              # optional — what the rule enforces
paths: [src/**/*.ts]          # optional — Cursor: auto-activate via globs; Claude Code: paths
---
Rule body in markdown.
```

**Skills** (`skills/<name>/SKILL.md`):

```yaml
---
name: skill-name              # required, ≤64 chars, lowercase + hyphens
description: ...              # required, ≤1024 chars — what it does + when to use it
allowed-tools: [Read, Bash]   # optional
disable-model-invocation: false  # optional
paths: [src/**/*.ts]          # optional — auto-activates when matching files are in context
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
| Skills | ✓ | ✓ | ✓ | ✓ | ✓ |
| Agents | ✓ | ✓ | ✓ | ✓ | ✓ |
| Commands | ⚠ deprecated | ✓ | ⚠ deprecated | ✓ | ⚠ deprecated |

When editing a skill, agent, or command in the TUI, tools that don't support that concept are shown with `✗` and a reason, and are skipped during sync.

### Field translation across tools

| Canonical field | Claude Code | Cursor | OpenCode | Gemini CLI | Codex CLI |
|---|---|---|---|---|---|
| `paths` (skill) | `paths:` | `globs:` | — | — | — |
| `allowed-tools` | `allowed-tools:` | `allowed-tools:` | `allowed-tools:` | — | — |
| `disable-model-invocation` | `disable-model-invocation:` | `disable-model-invocation:` | `disable-model-invocation:` | — | — |
| `tools` (agent) | `tools:` | — | `tools:` | `tools:` | — |
| `model` (agent) | `model:` | `model:` | `model:` | `model:` | `model:` |

`—` means the field is not emitted for that tool (unknown fields are silently ignored by most tools; omitting keeps output minimal).

> **Claude ↔ Cursor `paths`**: Claude Code's skill `paths:` and Cursor's rule `globs:` serve the same purpose — auto-activate on matching files. agentsync emits `paths:` to Claude Code and translates to `globs:` for Cursor. Per-rule `globs:` on the `general.mdc` rules file is a separate roadmap item.

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

## Contributing — adding a new tool

The adapter interface is defined in [`internal/tools/adapter.go`](internal/tools/adapter.go) and has six methods:

- `Name() string` — returns the tool's display name
- `Detect(workspace string) Installation` — reports whether the tool is installed (via `~/.<tool>`)
- `Supports(concept Concept) Compatibility` — reports whether the tool supports a given concept, with deprecation and reason metadata
- `Render(c *canonical.Canonical) ([]FileWrite, error)` — produces workspace-relative files to write from the canonical source
- `Alias(concept Concept) string` — returns a display filename when it differs from the canonical name (empty string otherwise)
- `Notice() string` — returns an optional informational note shown in the TUI tools screen (empty string otherwise)

See [`internal/tools/claude.go`](internal/tools/claude.go) for a reference implementation.

To add a new tool: implement the interface, then register it in [`internal/tools/registry.go`](internal/tools/registry.go) by adding an instance to the slice returned by `All()`.
