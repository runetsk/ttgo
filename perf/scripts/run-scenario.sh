#!/usr/bin/env bash
# Runs one k6 scenario against a freshly started TTGO server on a scratch DB.
#
# Usage:  scripts/run-scenario.sh k6/scenarios/smoke.js
# Env:    TIER=small PORT=8877 RESEED=1 K6_ARGS="--vus 1" plus any
#         scenario-specific vars (RESULTS_PER_RUN, STAGES, MODE, ...) which
#         k6 exposes to scripts via __ENV.
set -euo pipefail

PERF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_DIR="$(dirname "$PERF_DIR")"
SCENARIO="${1:?usage: run-scenario.sh <path relative to perf/, e.g. k6/scenarios/smoke.js>}"

TIER="${TIER:-small}"
PORT="${PORT:-8877}"
SCRATCH="$PERF_DIR/.scratch"
DB="$SCRATCH/perf-$TIER.db"
MANIFEST="$PERF_DIR/.seed-manifest.json"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT_DIR="$PERF_DIR/results/$STAMP-$(basename "$SCENARIO" .js)"
mkdir -p "$SCRATCH" "$OUT_DIR"

command -v k6 >/dev/null || { echo "k6 not found — install it first (see perf/README.md)"; exit 1; }

echo "==> building server"
(cd "$REPO_DIR/backend" && go build -tags sqlite_fts5 -o "$SCRATCH/ttgo-perf-server" ./cmd/server)

if [[ "${RESEED:-0}" == "1" || ! -f "$DB" || ! -f "$MANIFEST" ]]; then
  echo "==> seeding $DB (tier=$TIER)"
  (cd "$REPO_DIR/backend" && go run -tags sqlite_fts5 ./cmd/perfseed \
    -db "$DB" -tier "$TIER" -manifest "$MANIFEST" -wipe)
fi

echo "==> starting server on :$PORT"
(
  cd "$SCRATCH"
  DB_PATH="$DB" LISTEN_ADDR="127.0.0.1:$PORT" \
  ADMIN_EMAIL="perf-admin@perf.local" ADMIN_PASSWORD="perfseed-local-only" \
  CORS_ORIGIN="http://localhost:5173" \
  ./ttgo-perf-server >"$OUT_DIR/server.log" 2>&1 &
  echo $! >"$SCRATCH/server.pid"
)
SERVER_PID="$(cat "$SCRATCH/server.pid")"

SAMPLER_PID=""
cleanup() {
  [[ -n "$SAMPLER_PID" ]] && kill "$SAMPLER_PID" 2>/dev/null || true
  kill "$SERVER_PID" 2>/dev/null || true
  wait "$SERVER_PID" 2>/dev/null || true
}
trap cleanup EXIT

code="000"
for _ in $(seq 1 50); do
  code="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$PORT/api/categories" || true)"
  [[ "$code" != "000" ]] && break
  sleep 0.2
done
if [[ "$code" == "000" ]]; then
  echo "server did not come up; see $OUT_DIR/server.log"
  exit 1
fi
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
  echo "server process exited early — port $PORT already in use? see $OUT_DIR/server.log"
  exit 1
fi
echo "==> server up (probe returned HTTP $code)"

# Telemetry: server RSS (KB), %CPU, and WAL file size (bytes) every 2s.
(
  echo "ts,rss_kb,cpu_pct,wal_bytes" >"$OUT_DIR/telemetry.csv"
  while kill -0 "$SERVER_PID" 2>/dev/null; do
    rss_cpu="$(ps -o rss=,%cpu= -p "$SERVER_PID" | awk '{print $1","$2}')"
    wal="$(stat -f%z "$DB-wal" 2>/dev/null || stat -c%s "$DB-wal" 2>/dev/null || echo 0)"
    echo "$(date +%s),$rss_cpu,$wal" >>"$OUT_DIR/telemetry.csv"
    sleep 2
  done
) &
SAMPLER_PID=$!

echo "==> running $(basename "$SCENARIO")"
# --summary-export below is deprecated upstream in favor of handleSummary; works today, phase-4 cleanup candidate.
k6 run \
  -e TTGO_BASE_URL="http://localhost:$PORT" \
  -e TTGO_MANIFEST="$MANIFEST" \
  --summary-export "$OUT_DIR/summary.json" \
  ${K6_ARGS:-} \
  "$PERF_DIR/$SCENARIO"

echo "==> done. results in $OUT_DIR (summary.json, telemetry.csv, server.log)"
