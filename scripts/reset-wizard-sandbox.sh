#!/usr/bin/env bash
set -euo pipefail

# Stages examples/sandbox/ into a first-run state for exercising the init
# wizard, with each tool's on-disk config carrying its OWN name. Importing from
# Codex vs Claude Code then yields visibly different canonical content, so you
# can confirm the wizard pulled from the source you picked.
#
# How it works: for every registered tool we sync a tool-labelled canonical
# (only that tool enabled) inside an isolated temp workspace, then merge its
# rendered outputs into the sandbox. Isolating each sync avoids snapshot/orphan
# cross-talk between tools; the merge layers every tool's dirs side by side.
# Finally .agentsync/ is removed so the next `agentsync` run launches the wizard.
#
# Caveat: tools that read the shared bare AGENTS.md for root memory (OpenCode,
# Codex, Cline, Junie, Vibe, Pi, Copilot) cannot differ in that one file — last
# writer wins. Their agents/rules/commands/skills dirs still carry the tool
# name, so the import source stays obvious there.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SANDBOX="$ROOT/examples/sandbox"
GOLDEN="$ROOT/internal/tools/testdata/metadata_golden.json"

command -v jq >/dev/null || { echo "error: jq is required (tool list is read from $GOLDEN)" >&2; exit 1; }

echo "Staging wizard first-run sandbox at $SANDBOX …"

# clean slate: wipe every generated output, keep nothing (.agentsync is dropped
# at the end anyway). avoids stale leftovers from earlier syncs.
rm -rf "$SANDBOX"
mkdir -p "$SANDBOX"

# build once, reuse across per-tool syncs
BIN="$(mktemp -d)/agentsync"
( cd "$ROOT" && go build -o "$BIN" ./cmd/agentsync )

# write_canonical DEST TOOL — lay down a tool-labelled .agentsync/ under DEST,
# with config enabling only TOOL. The tool name is woven into AGENTS.md plus
# every entity body and frontmatter description, so it survives the round-trip
# into whichever per-tool layout the adapter renders.
write_canonical() {
  local dest="$1" tool="$2"
  mkdir -p "$dest/rules" "$dest/agents" "$dest/commands" "$dest/skills/code-review/docs"

  cat >"$dest/AGENTS.md" <<EOF
# Sandbox memory — sourced from ${tool}

This canonical was rendered into the **${tool}** layout. If you ran the wizard
and chose ${tool} as the import source, seeing "${tool}" here (and in the rule,
agent, command, and skill below) confirms the import pulled from ${tool}.

## Testing
- Follow AAA pattern (Arrange, Act, Assert)
- Run \`npm test\` before committing
EOF

  cat >"$dest/rules/style-guide.md" <<EOF
---
description: Style guide sourced from ${tool}
paths: [src/**/*.ts]
---
## Style Guide (${tool})

- This rule was imported from the ${tool} setup.
- Prefer named exports; avoid default exports.
EOF

  cat >"$dest/agents/explorer.md" <<EOF
---
name: explorer
description: Codebase explorer sourced from ${tool}. Maps structure and lists files.
tools: [Read, Glob, Grep]
model: haiku
---

You are the ${tool} explorer agent. When asked to explore, list structure and
return one line per significant file. Origin: ${tool}.
EOF

  cat >"$dest/commands/commit.md" <<EOF
---
description: Conventional commit helper sourced from ${tool}
argument-hint: "[scope]"
allowed-tools: [Bash]
---

1. Inspect staged changes (origin: ${tool})
2. Write a conventional commit message: \`<type>(<scope>): <description>\`
3. Commit — never include secrets or build artifacts
EOF

  cat >"$dest/skills/code-review/SKILL.md" <<EOF
---
name: code-review
description: Code reviewer sourced from ${tool}. Use to review or audit changes.
allowed-tools: [Read, Glob, Grep]
---

Review the specified diff (this skill came from the ${tool} setup):

1. Correctness, style, test coverage, security
2. Output a structured report with a one-line verdict
EOF

  cat >"$dest/skills/code-review/docs/test.md" <<EOF
# Reference (${tool})

Nested skill doc sourced from ${tool}.
EOF

  cat >"$dest/config.yaml" <<EOF
tools:
  ${tool}:
    enabled: true
EOF
}

# merge_outputs SRC — copy every rendered output under SRC into the sandbox,
# skipping the .agentsync/ source. dir contents are layered (cp -R "$e/.") so
# tools sharing a parent merge instead of nesting.
merge_outputs() {
  local src="$1" e
  ( cd "$src" && for e in * .[!.]*; do
      [ -e "$e" ] || continue
      [ "$e" = ".agentsync" ] && continue
      if [ -d "$e" ]; then
        mkdir -p "$SANDBOX/$e"
        cp -R "$e/." "$SANDBOX/$e/"
      else
        cp "$e" "$SANDBOX/"
      fi
    done )
}

# one isolated sync per registered tool
while IFS= read -r tool; do
  [ -n "$tool" ] || continue
  ws="$(mktemp -d)"
  write_canonical "$ws/.agentsync" "$tool"
  if "$BIN" sync --workspace "$ws" </dev/null >/dev/null 2>&1; then
    merge_outputs "$ws"
    echo "  • materialized: $tool"
  else
    echo "  • skipped (no project output): $tool"
  fi
  rm -rf "$ws"
done < <(jq -r '.[].Name' "$GOLDEN")

# The bare root AGENTS.md is the shared root-memory file every AGENTS.md-standard
# tool reads (Codex, OpenCode, Cline, Junie, Vibe, Pi, Copilot); it cannot differ
# per tool, so the merge above leaves it holding whichever tool wrote it last.
# Replace that misleading single-tool copy with a neutral note. Dedicated memory
# files (CLAUDE.md, GEMINI.md, .rules, .cursor/rules/general.mdc) are separate
# and keep their per-tool labels.
cat >"$SANDBOX/AGENTS.md" <<'EOF'
# Sandbox memory — shared AGENTS.md

This bare AGENTS.md is the shared root-memory file read by every
AGENTS.md-standard tool (Codex, OpenCode, Cline, Junie, Vibe, Pi, Copilot), so it
cannot name a single source. To tell those imports apart, read the tool-labelled
rule / agent / command / skill the import pulls in. Tools with their own root
memory (Claude → CLAUDE.md, Gemini → GEMINI.md, Cursor, Zed) carry the tool name
in that file directly.
EOF

echo
echo "Wizard sandbox ready: per-tool configs present, .agentsync/ removed."
echo "Run 'make dev' to launch the first-run wizard, pick a tool, and confirm the"
echo "imported canonical names that tool. 'make sandbox-reset' restores normal mode."
