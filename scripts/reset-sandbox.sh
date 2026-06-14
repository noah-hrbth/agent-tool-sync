#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SANDBOX="$ROOT/examples/sandbox"
SEED="$ROOT/examples/sandbox-seed"

echo "Resetting sandbox at $SANDBOX …"

# wipe the whole sandbox (a build artifact) so no stale per-tool output from an
# earlier sync survives, then re-seed only the canonical .agentsync/
rm -rf "$SANDBOX"
mkdir -p "$SANDBOX"
cp -R "$SEED/.agentsync" "$SANDBOX/.agentsync"

echo "Sandbox reset. Run: make dev"
