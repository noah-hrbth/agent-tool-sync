
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
- `make wizard-reset` — stage `examples/sandbox/` in a first-run state for the init wizard: each registered tool gets its OWN tool-labelled config (materialized by an isolated per-tool sync), then `.agentsync/` is removed so the next run launches the wizard with import sources you can tell apart. (The shared bare `AGENTS.md` is neutral — it's read by all AGENTS.md-standard tools and can't name one; dedicated memory files like `CLAUDE.md`/`GEMINI.md` keep their per-tool label.)
- `make dev-wizard` — `wizard-reset` then launch the TUI against `examples/sandbox` in one step, so the init wizard runs against distinguishable import sources.
- `make release-snapshot` — build a local goreleaser snapshot (skip publish).
- Single test: `go test ./internal/syncer/ -run TestRunSync -v`.

## Architecture

The pipeline is **canonical → adapters → syncer → disk**, with a snapshot for divergence detection.

### Packages

- `cmd/agentsync` — Cobra entrypoint. `resolveBase()` decides scope (`--global` vs cwd vs `--workspace`) and returns `(baseDir, tools.Scope)`. `loadState()` loads canonical + config in one call. Init flow: `runInitFlow` runs the wizard (TTY) or headless scaffold/import (`init --from <tool>`, `--force` for non-interactive re-init); `requireInitialized` guards `sync`/`status`; root command on an uninitialized scope launches the wizard (TTY) or exits 1.
- `internal/canonical` — parses `.agentsync/` (`AGENTS.md`, `rules/`, `skills/<dir>/SKILL.md`, `agents/`, `commands/`) into typed structs (`types.go`). The filename `general` is reserved at load time because Cursor's catch-all is `general.mdc`. `save.go` is used by adopt-flow to write canonical edits back.
- `internal/config` — YAML config (`.agentsync/config.yaml`) for per-tool enable/disable. `Default()` enables all tool names.
- `internal/tools` — one `Tool` per AI tool. `Tool` (`tool.go`) is a pure value: `Tool{ Meta ToolMeta; Render RenderFunc }`. There is **no `Adapter` interface** — metadata is data, rendering is a function.
  - `ToolMeta` (`tool.go`) is the per-tool data descriptor and single source of truth for everything except rendering: `Key`, `Name`, `Detect DetectFunc`, `Aliases`, `Concepts map[Concept]Compatibility`, `Scopes map[Scope]Compatibility`, `ConceptInfo`. Read it via `t.Meta.Name`, `t.Meta.Supports(c)`, `t.Meta.SupportsScope(s)`, `t.Meta.Alias(c)`, `t.Meta.Info(c)` — these are data lookups, not behaviour.
  - `RenderFunc` (`renderClaude`, `renderCursor`, …) produces files with **paths relative to the scope's base dir** (workspace root or `$HOME`). The syncer joins with `base` later — never absolute paths. This is the only deep, tool-specific code per tool.
  - `Meta.Concepts`/`Meta.Scopes` gate concepts/scopes; the syncer skips tools where `Meta.SupportsScope(scope).Supported == false` (e.g. Cursor user, Zed user). Every tool lists all 4 concepts and both scopes explicitly so the support matrix is visible in one literal.
  - `Meta.Detect` only reports presence (`detectGlobalDir`/`detectConfigDir` closures, or a custom one e.g. OpenCode's dual fallback); it does **not** gate sync — disabled-but-detected and enabled-but-undetected are both valid.
  - Frontmatter is built via `buildMDFrontmatter` / `buildTOML` in `frontmatter.go`; both skip zero values so render funcs can pass every field unconditionally.
  - Concept-to-output translation lives **inside each render func** (e.g. Claude's skill `paths:` → Cursor's `globs:`, Codex project skills land in `.agents/skills/` not `.codex/skills/`). Don't push translation up the stack.
  - `paths.go` is the single owner of every tool's output-path vocabulary (dir prefixes, root-memory files); render funcs and `adopt.go` both consume it. `ExpectedAdoptOutcome` declares per `(tool, concept, path)` whether a rendered path is reversible, root-memory, cross-mapped, or non-reversible — the render↔adopt contract test (`internal/syncer/contract_test.go`) fails on drift.
  - `registry.go::All()` assembles the ordered `[]Tool` and is the source of truth for ordering and `Names()`. Each tool's `ToolMeta` literal and `renderX` func are co-located in `internal/tools/<tool>.go`. `internal/tools/testdata/metadata_golden.json` + `TestMetadataParity` pin every tool's observable metadata; `TestSupportMatrix` enforces structural invariants.
- `internal/syncer` — orchestrates writes:
  - `Status()` renders all enabled tools (without writing) and classifies each output as `StatusSynced/Divergent/Missing/New` by comparing **disk SHA-256** vs **snapshot SHA-256** at `.agentsync/.state/snapshot.json`.
  - `RunSync()` writes files, updates the snapshot, and performs **orphan cleanup** using `allRenderedPaths` computed from *all* scope-compatible tools (enabled or not) so disabling a tool does **not** auto-delete its previously-synced files. Orphan files only get deleted when their disk hash still matches the last-synced snapshot hash; user-modified orphans are preserved with a warning.
  - `AdoptExternal()` reverse-maps a single divergent output path back to a canonical entity using path heuristics in `adopt.go` (e.g. `.cursor/rules/general.mdc` ↔ `AGENTS.md`, `*/skills/<dir>/SKILL.md` ↔ canonical skill). After adopting, callers must reload canonical from disk.
  - `Scaffold` (`scaffold.go`) writes the `.agentsync/` skeleton + starter AGENTS.md + config. `ImportFromTool` / `ImportFromSources` (`import.go`) bootstrap a fresh canonical by running adopt in bulk over a tool's render-derived on-disk sources; per-file failures become `Skipped` entries, never aborts.
- `internal/tui` — Bubble Tea model with three screens (`screenFiles`, `screenTools`, `screenSync`). Holds two `scopeSnapshot`s (project + user) and mirrors the active one into flat fields when toggling with `g`.
- `internal/wizard` — standalone Bubble Tea init wizard: method/tool selection (Start fresh vs Import, Claude Code pinned first), spinner, runs `syncer.Scaffold` + `ImportFromTool`. `BuildOptions`/`DetectedNames` probe each tool at scope via `tools.DetectAtScope`.

### Adding a new tool adapter

1. Add `internal/tools/<tool>.go` with a `var <tool>Meta = ToolMeta{...}` literal and a `func render<Tool>(c *canonical.Canonical, scope Scope) ([]FileWrite, error)` (see `claude.go` as the reference). List all 4 concepts and both scopes explicitly in `Meta`.
2. `Detect: detectGlobalDir("foo")` for `~/.foo` or `detectConfigDir("foo")` for `~/.config/foo/`; pass a custom `DetectFunc` closure for irregular probes.
3. Set `Scopes[ScopeUser] = {Supported: false, Reason: "..."}` when the tool has no on-disk user config (Cursor, Zed).
4. Register in `registry.go::All()` as `{Meta: <tool>Meta, Render: render<Tool>}` (order is intentional, not alphabetical — `TestRegistryTailOrder` pins the tail).
5. Regenerate the metadata golden: `UPDATE_GOLDEN=1 go test ./internal/tools/ -run TestMetadataParity`, and review the diff.
6. If the path mapping is reversible, extend `internal/syncer/adopt.go` so external edits can flow back to canonical, and keep `paths.go` + `ExpectedAdoptOutcome` in sync so the contract test passes.

### Things to keep in mind

- Sync is **one-way** (canonical → tools). Bidirectional pull is on the roadmap (`TODO.md`); don't assume divergence-resolution code is symmetric. **Import** is the related one-time bootstrap, not ongoing two-way sync.
- Snapshot path **must be workspace-relative** — both `Status` and `RunSync` key the snapshot map with `FileWrite.Path`. Never store absolute paths in the snapshot.
- The same path can be emitted by multiple tools across scopes (e.g. OpenCode emits bare `AGENTS.md` at project scope, `.config/opencode/AGENTS.md` at user scope). `adopt.go::matchSkillPath` etc. enumerate both prefixes.
- `.agentsync/` dir presence is the **only** init marker; `sync`/`status` fail fast (exit 1) without it.
- `examples/sandbox/` is a **build/test artifact** — anything under it (except `.agentsync/`) is regeneratable; don't hand-edit and expect it to survive `sandbox-reset`.
