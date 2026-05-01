---
name: add-adapter
description: Add a new tool adapter to agentsync — scaffolds internal/tools/<tool>.go, registers it, extends adopt.go for reverse mapping, adds a roundtrip scenario, and updates README compatibility tables. Use when implementing one of the tools listed in TODO.md (Windsurf, Continue.dev, Cline, Roo Code, Copilot, Junie, Crush, Goose, Amazon Q, Kilo Code, Aider, Cody) or any new AI tool.
allowed-tools: [Read, Edit, Write, Bash, Glob, Grep, WebFetch, WebSearch]
---

# Add a New Tool Adapter

Goal: implement a new `tools.Adapter` end-to-end, mirroring the conventions in `internal/tools/claude.go` (the reference implementation).

## 0. Research the target tool — do not skip

Find primary-source docs for the tool's config layout. Confirm:

- Config dir: `~/.<tool>/` or `~/.config/<tool>/`? (`detectGlobalDir` vs `detectConfigDir`)
- Project layout: `.<tool>/...` at workspace root? Anything special?
- Root memory file (name + format): does it live at workspace root or under the tool dir?
- Per-rule files supported? Subdir layout? File extension (`.md`, `.mdc`, `.toml`)?
- Skills, agents, slash commands: which concepts exist? Frontmatter schema for each?
- User scope: is there an on-disk user config layer? If not, return `SupportsScope(ScopeUser).Supported = false`.
- Deprecations: any concept the tool flags as legacy? (set `Compatibility.Deprecated = true` with a `Replacement`).

Cite the source URL in the adapter file as a one-line comment above the struct. If docs are ambiguous, prefer WebFetch over guessing.

## 1. Create `internal/tools/<tool>.go`

Use `claude.go` as the template. Required:

- Type: `type <tool>Adapter struct{}`
- `Name()` — display name (Title Case, e.g. `"Windsurf"`).
- `Detect()` — `detectGlobalDir("<tool>")` or `detectConfigDir("<tool>")`.
- `Supports(concept)` — return `Compatibility{Supported: true}` per concept the tool natively supports; `Supported: false` with a `Reason` for unsupported; mark deprecated concepts with `Deprecated: true, Replacement: "<successor>"`.
- `SupportsScope(scope)` — `false` for scopes with no on-disk layer (see `cursor.go` user scope, `zed.go` user scope).
- `Alias(concept)` — only for tools where the displayed filename differs from the canonical name (e.g. Claude `CLAUDE.md` for the rules root). Empty string otherwise.
- `Notice()` — short note for the TUI Tools screen when the path layout is non-obvious (see `codex.go` for an example). Empty string otherwise.
- `Render(c, scope)` — return `[]FileWrite` with **paths relative to the scope's base dir** (workspace root or `$HOME`). Never emit absolute paths. Use `filepath.Join` consistently.

Build all frontmatter via `buildMDFrontmatter` / `buildTOML` from `frontmatter.go` — they skip zero values, so pass every field unconditionally. Do not hand-roll YAML/TOML.

If the tool has no per-rule directory, append rule bodies as `##`-headed sections to the root memory file via `buildRootMemoryContent` (see `gemini.go`, `opencode.go`, `codex.go`).

If a canonical field maps to a different key in the target (e.g. Claude `paths:` → Cursor `globs:`), translate **inside this adapter only** — don't push translation up the stack.

## 2. Register the adapter

Edit `internal/tools/registry.go::All()` and add `&<tool>Adapter{}` to the slice in alphabetical-by-display-name order (matching the existing TUI ordering).

## 3. Extend `internal/syncer/adopt.go`

For each path the adapter writes that maps reversibly back to a canonical entity, extend the relevant matcher:

- Root memory file → add the path to the `case path == ...` switch arm that calls `canonical.SaveAgentsMD`.
- Per-rule files → ensure `matchRulePath` returns true for the new path prefix; `ruleFilename` already strips `.md`/`.mdc`.
- Skills → add the new prefix to `matchSkillPath`; update `skillDir` if the path depth differs.
- Agents → add the new prefix to `matchAgentPath`.
- Commands → add the new prefix to `matchCommandPath`.

If the tool's output format isn't reversible (e.g. concatenated rule bodies in the root memory), document why in a comment and skip the matcher — adopt-flow will refuse to map it, which is the correct behaviour.

## 4. Add a roundtrip scenario

Create `testdata/scenarios/<tool>/` with `input/.agentsync/` (canonical source) and `expected/` (rendered output tree). Mirror an existing scenario like `testdata/scenarios/pristine/`. The scenario will be picked up by the existing test harness in `internal/canonical/render_test.go` and `internal/tools/adapter_test.go` — check those tests' table-driven inputs and add a row for the new tool.

## 5. Update the README

Edit `README.md`:

- "Supported AI tools" table — new row with paths and detection dir.
- "Concept compatibility" table — ✓ / ⚠ / ✗ per concept.
- "Field translation across tools" table — only if the adapter renames any canonical field.
- "Tools that don't support user scope" / "user-scope output path differs" lists — add the tool if applicable.

## 6. Verify

Run, in order:

```bash
go vet ./...
go test ./...
make smoke
make sandbox-reset && make build && ./agentsync sync --workspace ./examples/sandbox
```

Then diff `examples/sandbox/.<tool>/` against your expectations.

## 7. Final review

Invoke the `adapter-reviewer` subagent on the new adapter file before considering the task done. Apply its findings.
