# agentsync

Maintain AI tool configs in one place and sync them to Claude Code, OpenCode, Cursor, Gemini CLI, Codex CLI, Cline, JetBrains Junie, and Zed.

## Install

### Homebrew (macOS)

```bash
brew tap noah-hrbth/agentsync
brew install --cask agentsync
```

Homebrew 4.x falls through `brew install agentsync` to the cask automatically; older versions need the explicit `--cask` flag.

Upgrade with `brew update && brew upgrade --cask agentsync`.

### Manual download (macOS, Linux, Windows)

Prebuilt binaries for macOS, Linux, and Windows (amd64 + arm64) are attached to each [GitHub release](https://github.com/noah-hrbth/agent-tool-sync/releases). Extract and place `agentsync` on your `PATH`. Recommended for Linux and Windows.

### From source

```bash
go install github.com/noah-hrbth/agentsync/cmd/agentsync@latest
```

## Quickstart

1. Scaffold the canonical source:
   ```bash
   agentsync init              # project scope: ./.agentsync/
   agentsync init --global     # user scope:    ~/.agentsync/
   ```
2. Edit `.agentsync/AGENTS.md` with your rules, then add skills, agents, and commands under the corresponding subdirectories.
3. Sync to all enabled tools:
   ```bash
   agentsync sync              # project: headless
   agentsync sync --global     # user:    headless
   agentsync                   # TUI (toggle scope with [g])
   ```

## Scopes

`agentsync` operates in one of two scopes:

| Scope | Canonical source | Output target | When to use |
|---|---|---|---|
| **Project** (default) | `<repo>/.agentsync/` | `<repo>/.<tool>/...` | Per-project rules — committed alongside the codebase |
| **User** (`--global` / `-g`) | `~/.agentsync/` | `~/.<tool>/...` (varies per tool) | Personal defaults applied across every project |

The two layers stack: each tool reads its user-level config plus any project-level overrides. Run `agentsync init --global` once to set up your personal canonical, then `agentsync init` per project for repo-specific overrides.

**Tools that don't support user scope** are skipped with a notice:
- **Cursor** — user rules are managed in the Settings UI, not on disk
- **Zed** — has no global rules file (project-root `.rules` only)

**Tools whose user-scope output path differs from project scope:**
- **Claude Code** — root memory writes to `<workspace>/CLAUDE.md` at project scope (auto-discovered up the tree) and `~/.claude/CLAUDE.md` at user scope
- **OpenCode** — root memory at `<workspace>/AGENTS.md` (project) vs `~/.config/opencode/AGENTS.md` (user); skills/agents/commands at `.opencode/` vs `~/.config/opencode/`
- **Gemini CLI** — root memory at `<workspace>/GEMINI.md` (project) vs `~/.gemini/GEMINI.md` (user)
- **Codex CLI** — root memory at `<workspace>/AGENTS.md` (project) vs `~/.codex/AGENTS.md` (user); user-scope skills go to `~/.codex/skills/` (vs. project-scope `.agents/skills/` for cross-tool sharing)
- **Cline** — project rules at `.clinerules/<name>.md` and root `AGENTS.md`; user rules at `~/Documents/Cline/Rules/<name>.md`. Workflows: `.clinerules/workflows/` (project) vs `~/Documents/Cline/Workflows/` (user). Skills always at `.cline/skills/`. No user-level `AGENTS.md`.
- **JetBrains Junie** — root `AGENTS.md` is **project-only** (Junie has no user-scope guidelines path); skills/agents/commands at `.junie/` are honoured at both scopes.

## Supported AI tools

| Tool | Root memory | Per-rule files | Skills | Agents | Commands | Detection |
|---|---|---|---|---|---|---|
| Claude Code | `CLAUDE.md` (workspace root; `~/.claude/CLAUDE.md` at user scope) | `.claude/rules/<name>.md` | `.claude/skills/<dir>/SKILL.md` | `.claude/agents/<name>.md` | `.claude/commands/<name>.md ⚠` | `~/.claude/` |
| OpenCode | `AGENTS.md` (workspace root; `~/.config/opencode/AGENTS.md` at user scope) | appended to root | `.opencode/skills/<dir>/SKILL.md` | `.opencode/agents/<name>.md` | `.opencode/commands/<name>.md` | `~/.opencode/` |
| Cursor | `.cursor/rules/general.mdc` | `.cursor/rules/<name>.mdc` | `.cursor/skills/<dir>/SKILL.md` | `.cursor/agents/<name>.md` | `.cursor/commands/<name>.md ⚠` | `~/.cursor/` |
| Gemini CLI | `GEMINI.md` (workspace root; `~/.gemini/GEMINI.md` at user scope) | appended to root | `.gemini/skills/<dir>/SKILL.md` | `.gemini/agents/<name>.md` | `.gemini/commands/<name>.toml` | `~/.gemini/` |
| Codex CLI | `AGENTS.md` (workspace root; `~/.codex/AGENTS.md` at user scope) | appended to root | `.agents/skills/<dir>/SKILL.md` | `.codex/agents/<name>.toml` | `⚠ deprecated → skills` | `~/.codex/` |
| Cline | `AGENTS.md` (workspace root; project-only) | `.clinerules/<name>.md` (project); `~/Documents/Cline/Rules/<name>.md` (user) | `.cline/skills/<dir>/SKILL.md` | — | `.clinerules/workflows/<name>.md` (project); `~/Documents/Cline/Workflows/` (user) | `~/.cline/` |
| JetBrains Junie | `AGENTS.md` (workspace root; project-only) | appended to root | `.junie/skills/<dir>/SKILL.md` | `.junie/agents/<name>.md` | `.junie/commands/<name>.md` | `~/.junie/` |
| Zed | `.rules` (workspace root) | appended to root | — | — | — | `~/.config/zed/` |

`AGENTS.md` at the workspace root is shared by OpenCode and Codex CLI — both tools read it natively.

Zed reads its rules file from the workspace root, so `.rules` lands there instead of in a tool-namespaced folder. Zed has no native concept for skills, file-defined agents, or user-defined slash commands; agentsync skips those for Zed.

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

| Concept | Claude Code | OpenCode | Cursor | Gemini CLI | Codex CLI | Cline | JetBrains Junie | Zed |
|---|---|---|---|---|---|---|---|---|
| Rules | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Skills | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ |
| Agents | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ✗ |
| Commands | ⚠ deprecated | ✓ | ⚠ deprecated | ✓ | ⚠ deprecated | ✓ | ✓ | ✗ |

When editing a skill, agent, or command in the TUI, tools that don't support that concept are shown with `✗` and a reason, and are skipped during sync.

### Field translation across tools

| Canonical field | Claude Code | Cursor | OpenCode | Gemini CLI | Codex CLI | Cline | JetBrains Junie | Zed |
|---|---|---|---|---|---|---|---|---|
| `paths` (skill) | `paths:` | `globs:` | — | — | — | — | — | — |
| `paths` (rule) | `paths:` | `globs:` | — | — | — | `paths:` | — | — |
| `allowed-tools` | `allowed-tools:` | `allowed-tools:` | `allowed-tools:` | — | — | — | — | — |
| `disable-model-invocation` | `disable-model-invocation:` | `disable-model-invocation:` | `disable-model-invocation:` | — | — | — | — | — |
| `tools` (agent) | `tools:` | — | `tools:` | `tools:` | — | — | `tools:` | — |
| `model` (agent) | `model:` | `model:` | `model:` | `model:` | `model:` | — | `model:` | — |

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
  -g, --global                 Operate at user scope (canonical at ~/.agentsync/, syncs to user-level tool dirs)
```

In the TUI, press `g` to toggle between project and user scope. The active scope is shown in the tab bar.
