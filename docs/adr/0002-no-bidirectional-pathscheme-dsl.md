# No bidirectional PathScheme DSL — drift handled by a contract test + shared constants

Each tool's output-path scheme was independently re-encoded in ~13 places (notably
`adopt.go` re-typed 41 path literals with no import of `internal/tools`), and no test
verified that every rendered path is reachable by an adopt matcher — so path changes
drifted silently. We fixed the drift class with a render↔adopt **contract test**
(`internal/syncer/contract_test.go`) plus a single owner of the path vocabulary
(`internal/tools/paths.go`: dir constants, aggregate prefix slices, and the
`ExpectedAdoptOutcome` reversibility manifest). We deliberately did **not** introduce a
declarative bidirectional `PathScheme` that both render and adopt derive from.

## Considered options

- **Full bidirectional PathScheme**: each tool declares a structured spec (dir + filename
  template + ext per concept per scope, multi-file, non-reversible flags); render builds
  paths from it and adopt derives the reverse generically. Rejected: the conventions are
  intrinsically irregular (Cursor `general.mdc`↔`AGENTS.md`; Vibe two-file agents; Vibe
  commands rendered as skills; 7 tools concatenate rules into root memory; Codex skill
  base differs by scope; Cline user rules → `Documents/Cline/Rules/`). A spec covering
  all of it is an expressive mini-DSL — speculative generality harder to follow than the
  10 straightforward render funcs — and it cannot absorb the content logic
  (frontmatter/TOML/brace-expansion) anyway. It would also scatter adopt's deliberately
  ordered dispatch.
- **Contract test + shared path constants** (chosen): drift becomes a CI failure without
  forcing a forward DSL; adopt's ordered switch stays centralized; the irregular cases
  stay as readable bespoke code, pinned by the contract.

## Consequences

A future architecture sweep will likely re-suggest "derive both directions from one
scheme." This ADR records that the irregularity makes that speculative generality; the
contract test is the durable guard instead. The reversibility manifest also pins
currently non-reversible paths (gemini/cursor/codex skills+agents+commands, zed `.rules`,
vibe TOML agents) — making any future "make these reversible" work bounded and visible.
`internal/tui/tui.go::matchesFileItem` remains a separate, un-unified drift surface by
the same reasoning.
