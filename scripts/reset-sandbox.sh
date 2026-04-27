#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SANDBOX="$ROOT/examples/sandbox"
SEED="$ROOT/examples/sandbox-seed"

echo "Resetting sandbox at $SANDBOX …"

rm -rf \
  "$SANDBOX/.agentsync" \
  "$SANDBOX/.claude" \
  "$SANDBOX/.opencode" \
  "$SANDBOX/.cursor" \
  "$SANDBOX/.gemini" \
  "$SANDBOX/AGENTS.md" \
  "$SANDBOX/GEMINI.md"

mkdir -p "$SANDBOX"
cp -R "$SEED/.agentsync" "$SANDBOX/.agentsync"
mkdir -p "$SANDBOX/.claude" "$SANDBOX/.opencode" "$SANDBOX/.cursor"

echo "Sandbox reset. Run: make dev"
