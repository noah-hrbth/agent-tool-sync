# agentsync Context

agentsync keeps one **Canonical** source of AI-tool configuration and one-way syncs it
into per-tool on-disk layouts. This file fixes the domain language; use these terms
exactly in code, comments, and discussion.

## Language

**Canonical**:
The single authored source of truth under `.agentsync/` (AGENTS.md, rules, skills, agents, commands).
_Avoid_: source, master, origin

**Concept**:
A category of canonical configuration — one of rules, skills, agents, commands.
_Avoid_: kind, type, category

**Scope**:
The directory tree the canonical maps onto: project (`<workspace>/`) or user (`$HOME`).
_Avoid_: level, target, environment

**Tool**:
A supported AI tool, represented as the value `Tool{ Meta ToolMeta; Render RenderFunc }`.
Pure data plus one function — there is no adapter interface.
_Avoid_: Adapter, plugin, integration, backend

**ToolMeta**:
The per-tool data descriptor: the single source of truth for everything about a Tool
except how it renders (Key, Name, Detect, Aliases, Concepts, Scopes, ConceptInfo).
_Avoid_: config, spec, descriptor (informal), metadata (bare)

**RenderFunc**:
The one deep, tool-specific behaviour: `(Canonical, Scope) → []FileWrite`. Implemented
per tool as `render<Tool>`.
_Avoid_: renderer, generator, emitter

**Registry**:
`registry.go::All()` — the ordered `[]Tool` slice; source of truth for ordering and `Names()`.
_Avoid_: list, catalog, table (when referring to the slice itself)

**Compatibility**:
Whether a Tool supports a Concept or Scope, with optional Reason / Deprecated / Replacement.
_Avoid_: support flag, capability

**Adopt**:
Reverse-mapping a divergent rendered file back into the Canonical (`AdoptExternal`).
_Avoid_: pull, import, reverse-sync

## Relationships

- A **Canonical** is rendered by each **Tool**'s **RenderFunc** into files, one per **Scope**.
- A **Tool** is its **ToolMeta** (data) plus its **RenderFunc** (behaviour); the **Registry** orders all Tools.
- **ToolMeta** declares a **Compatibility** for every **Concept** and every **Scope** explicitly.
- **Adopt** is the inverse of a **RenderFunc** for paths `adopt.go` can reverse-map.

## Example dialogue

> **Dev:** "Where does Cursor's user-scope unsupported reason live now — in the adapter?"
> **Maintainer:** "There is no adapter. It's data: `cursorMeta.Scopes[ScopeUser]` is a
> **Compatibility** with `Supported:false` and the Reason string. The **RenderFunc**
> never decides support — it just renders; the **ToolMeta** gates."

## Flagged ambiguities

- "adapter" historically meant the per-tool interface implementation. Resolved: the
  interface is gone; the concept is now **Tool** (data + RenderFunc). Do not reintroduce
  "Adapter" for the type, though prose may still say "the Cursor tool".
