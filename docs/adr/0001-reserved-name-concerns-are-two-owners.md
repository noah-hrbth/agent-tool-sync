# Reserved-name carve-outs are two package-owned predicates, not one shared registry

agentsync has two unrelated "reserved name" concerns: the reserved canonical **rule
slug** (`general`, collides with Cursor's catch-all `.cursor/rules/general.mdc`) and the
**gitignore root carve-outs** (`.agentsync`, `.github`, root `AGENTS.md`). We
deliberately keep each as its own predicate in its own package —
`canonical.IsReservedRuleName` / `ReservedRuleReason` in `internal/canonical`, and
`rootSegmentSkips` / `rootFileSkips` in `internal/gitignore` — rather than unifying them
behind one cross-package "reserved name" module.

## Considered options

- **One shared reserved-name registry/predicate** (the architecture-review suggestion).
  Rejected: it conflates two semantically different domains (a rule-authoring constraint
  vs. a gitignore-tracking carve-out) under one name that would mean two things, and adds
  a cross-package coupling for no behavioural gain.
- **Two package-owned predicates with names+rationale as data, each pinned by a test**
  (chosen). Each concern's owner is obvious; the rationale is written once (the map
  value); adding a reserved name is one map entry.

## Consequences

A future architecture sweep will likely re-suggest "centralize reserved names." This ADR
records that the split is intentional and load-bearing — do not unify without revisiting
the domain distinction above.
