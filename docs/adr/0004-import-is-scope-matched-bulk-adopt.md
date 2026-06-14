# Import is scope-matched bulk Adopt over render-derived sources

Import is the one-time bootstrap of a fresh Canonical from an existing tool's
on-disk layout, offered by the init wizard (`agentsync` on an uninitialized
scope, `agentsync init`, or headless `agentsync init --from <tool>`). It is
implemented as bulk Adopt: `tools.DeriveImportSources` renders a probe
canonical and collects the emitted directories and root files, with a union
rule adding adopt-reversible-but-no-longer-rendered dirs (e.g.
`.claude/commands`) and root-memory alternates (e.g. `.claude/CLAUDE.md`).
Import is **scope-matched only**: project init imports from workspace-level
tool dirs (`tools.DetectAtScope`), `--global` from `~`-level dirs
(`Meta.Detect`). The walk takes `.md`/`.mdc` files only; every skip is
recorded with a reason (no canonical mapping, not markdown, symlink, reserved
rule name `general`) and per-file errors never abort the import.

## Considered options

- **Cross-scope import** (importing `~/.claude` into a project canonical).
  Rejected: it duplicates user config into repos, so the same content stacks
  twice at runtime when the tools read both layers.
- **Per-tool import-roots data** (a new `ToolMeta` field listing each tool's
  importable dirs). Rejected: it duplicates knowledge the render funcs already
  have; deriving sources from render output stays auto-consistent as adapters
  change.
- **Widening Adopt's glossary meaning to cover bootstrap.** Rejected: Adopt is
  a sharp term for per-file drift repair of rendered outputs; blurring it into
  "anything flowing back to canonical" would erode the domain language.
- **Scope-matched bulk Adopt over render-derived sources** (chosen): reuses
  the existing, contract-tested reverse mappings in `adopt.go`, needs no new
  per-tool data, and keeps Import and Adopt sharply distinguished (bulk
  bootstrap vs per-file repair).

## Consequences

- A bare `AGENTS.md` at project scope is **neutral**: it is the shared root
  memory of several AGENTS.md-standard tools (Codex, OpenCode, Vibe, Pi, Cline,
  Junie), so it cannot identify any one of them and is excluded from per-tool
  project detection (`neutralRootMemoryFiles` in `detect_scope.go`). Without
  this, every AGENTS.md tool reads as "installed" whenever any `AGENTS.md`
  exists — a false positive. A tool is project-detected only via a
  tool-specific marker (its own concept dir or dedicated memory file). Tradeoff:
  a repo whose only config is a lone `AGENTS.md` offers no import source in the
  wizard (the six fan-out options were redundant — all imported the same file);
  the user starts fresh instead.
- Tool-root-only installs (e.g. only `.claude/settings.json`, no concept dirs)
  are not project-detected.
- There is no auto-sync after import — the user reviews the canonical and runs
  sync explicitly.
- A partial `.agentsync/` left by a hard I/O abort counts as initialized,
  since dir presence is the only init marker; the user can re-init.
- `agentsync pull` (roadmap) remains a separate, ongoing bidirectional concept
  distinct from one-time Import.
