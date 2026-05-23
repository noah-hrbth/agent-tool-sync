# Skills sync as multi-file, but markdown only

A canonical skill is a folder: the `SKILL.md` manifest plus any number of
additional `.md` docs (`reference.md`, `examples/x.md`, …) at any depth, all
synced to every skill-supporting tool with subfolders preserved. We deliberately
sync **only `.md` files** — non-`.md` files in a skill folder (Python/TS scripts,
binaries, tool-specific code) are ignored at load, never rendered, and never
deleted.

## Considered options

- **Sync every file in a skill folder.** Rejected: the maintenance and safety
  overhead is high and the semantics are murky — scripts are language- and
  tool-specific (one tool's skill ships `.py`, another `.ts`), agentsync would
  have to copy executable code verbatim across tools where it may not run, and
  treating arbitrary binaries as canonical source invites accidental clobbering.
- **Markdown-only, folders preserved** (chosen): `.md` is the portable,
  tool-agnostic substrate skills are actually authored in; limiting sync to it
  keeps the pipeline simple and safe while still carrying a skill's full prose
  (manifest + reference docs + examples). The TUI surfaces the limit on the skill
  dir node so the exclusion is visible, not silent.

## Consequences

- A skill whose behaviour depends on a non-`.md` asset (e.g. `scripts/run.py`)
  will not be fully reproduced in target tools — the asset must be managed
  outside agentsync. This is an accepted, documented limitation.
- A subfolder containing only non-`.md` files is not recreated on the target side
  (nothing `.md` carries it). Empty folders are not modelled.
- Reversibility (adopt) and the render↔adopt contract extend to skill docs for
  the tools whose skill paths are already reversible; see
  `internal/tools/paths.go` (`SkillDirPrefixes`, `ExpectedAdoptOutcome`).
