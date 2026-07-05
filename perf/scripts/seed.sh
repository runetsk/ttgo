#!/usr/bin/env bash
# (Re)provisions the perf scratch DB and k6 seed manifest. Single owner of the
# perfseed invocation and of the TIER/TOKENS defaults — both `make -C perf seed`
# and run-scenario.sh's auto-reseed delegate here so the two paths can never
# drift apart.
set -euo pipefail

PERF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_DIR="$(dirname "$PERF_DIR")"
TIER="${TIER:-small}"
TOKENS="${TOKENS:-100}"
USERS="${USERS:-10}"
SCRATCH="$PERF_DIR/.scratch"

mkdir -p "$SCRATCH"
cd "$REPO_DIR/backend"
exec go run -tags sqlite_fts5 ./cmd/perfseed \
  -db "$SCRATCH/perf-$TIER.db" -tier "$TIER" -tokens "$TOKENS" -users "$USERS" \
  -manifest "$PERF_DIR/.seed-manifest.json" -wipe
