
# Core Project Guidance

This file provides guidance to AI Coding Tools when working with code in this repository.

## Project

`agentsync` is a Go CLI + Bubble Tea TUI that keeps a single canonical source of AI tool configs in `.agentsync/` and one-way syncs it to per-tool layouts (`.claude/`, `.cursor/`, `.opencode/`, `.gemini/`, `.codex/` + `.agents/`, `.rules`). It supports two scopes — **project** (default, `<workspace>/.agentsync/`) and **user** (`--global`, `~/.agentsync/`) — that stack at runtime in each target tool.

## Commands

- `make build` — build `./agentsync` with version embedded via `-X main.version=...`.
- `make test` — `go test ./...`.
- `make smoke` — TUI smoke tests only (`go test -run TestSmoke ./internal/tui/...`).
- `make lint` — `go vet ./...` (CI runs vet + tests + `goreleaser check`).
- `make dev` — run the TUI against `examples/sandbox` (a throwaway workspace).
- `make sandbox-reset` — wipe `examples/sandbox/` and recopy `examples/sandbox-seed/.agentsync/`. Always run this before iterating on TUI flows.
- `make release-snapshot` — build a local goreleaser snapshot (skip publish).
- Single test: `go test ./internal/syncer/ -run TestRunSync -v`.

## Architecture

The pipeline is **canonical → adapters → syncer → disk**, with a snapshot for divergence detection.

### Packages

- `cmd/agentsync` — Cobra entrypoint. `resolveBase()` decides scope (`--global` vs cwd vs `--workspace`) and returns `(baseDir, tools.Scope)`. `loadState()` loads canonical + config in one call.
- `internal/canonical` — parses `.agentsync/` (`AGENTS.md`, `rules/`, `skills/<dir>/SKILL.md`, `agents/`, `commands/`) into typed structs (`types.go`). The filename `general` is reserved at load time because Cursor's catch-all is `general.mdc`. `save.go` is used by adopt-flow to write canonical edits back.
- `internal/config` — YAML config (`.agentsync/config.yaml`) for per-tool enable/disable. `Default()` enables all tool names.
- `internal/tools` — one adapter per tool implementing `Adapter` (`adapter.go`):
  - `Render(c, scope) []FileWrite` produces files with **paths relative to the scope's base dir** (workspace root or `$HOME`). The syncer joins with `base` later — never absolute paths from adapters.
  - `Supports(concept)` and `SupportsScope(scope)` gate concepts/scopes; the syncer skips adapters where `SupportsScope(scope).Supported == false` (e.g. Cursor user, Zed user).
  - `Detect()` only reports presence (`~/.<tool>` or `~/.config/<tool>`); it does **not** gate sync — disabled-but-detected and enabled-but-undetected are both valid.
  - Frontmatter is built via `buildMDFrontmatter` / `buildTOML` in `frontmatter.go`; both skip zero values so adapters can pass every field unconditionally.
  - Concept-to-output translation lives **inside each adapter** (e.g. Claude's skill `paths:` → Cursor's `globs:`, Codex project skills land in `.agents/skills/` not `.codex/skills/`). Don't push translation up the stack.
  - `registry.go::All()` is the source of truth for adapter ordering and `Names()`.
- `internal/syncer` — orchestrates writes:
  - `Status()` renders all enabled adapters (without writing) and classifies each output as `StatusSynced/Divergent/Missing/New` by comparing **disk SHA-256** vs **snapshot SHA-256** at `.agentsync/.state/snapshot.json`.
  - `RunSync()` writes files, updates the snapshot, and performs **orphan cleanup** using `allRenderedPaths` computed from *all* scope-compatible adapters (enabled or not) so disabling a tool does **not** auto-delete its previously-synced files. Orphan files only get deleted when their disk hash still matches the last-synced snapshot hash; user-modified orphans are preserved with a warning.
  - `AdoptExternal()` reverse-maps a divergent output path back to a canonical entity using path heuristics in `adopt.go` (e.g. `.cursor/rules/general.mdc` ↔ `AGENTS.md`, `*/skills/<dir>/SKILL.md` ↔ canonical skill). After adopting, callers must reload canonical from disk.
- `internal/tui` — Bubble Tea model with three screens (`screenFiles`, `screenTools`, `screenSync`). Holds two `scopeSnapshot`s (project + user) and mirrors the active one into flat fields when toggling with `g`.

### Adding a new tool adapter

1. Implement `tools.Adapter` (see `claude.go` as the reference).
2. Use `detectGlobalDir("foo")` for `~/.foo` or `detectConfigDir("foo")` for `~/.config/foo/`.
3. Return `SupportsScope(ScopeUser).Supported = false` when the tool has no on-disk user config (Cursor, Zed).
4. Register in `registry.go::All()`.
5. If the path mapping is reversible, extend `internal/syncer/adopt.go` so external edits can flow back to canonical.

### Things to keep in mind

- Sync is **one-way** (canonical → tools). Bidirectional pull is on the roadmap (`TODO.md`); don't assume divergence-resolution code is symmetric.
- Snapshot path **must be workspace-relative** — both `Status` and `RunSync` key the snapshot map with `FileWrite.Path`. Never store absolute paths in the snapshot.
- The same path can be emitted by multiple adapters across scopes (e.g. OpenCode emits `.opencode/AGENTS.md` at project scope, `.config/opencode/AGENTS.md` at user scope). `adopt.go::matchSkillPath` etc. enumerate both prefixes.
- `examples/sandbox/` is a **build/test artifact** — anything under it (except `.agentsync/`) is regeneratable; don't hand-edit and expect it to survive `sandbox-reset`.
