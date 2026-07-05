#!/usr/bin/env bash
# Runs one k6 scenario against a freshly started TTGO server on a scratch DB.
#
# Usage:  scripts/run-scenario.sh k6/scenarios/smoke.js
# Env:    TIER=small PORT=8877 RESEED=1 TOKENS=100 K6_ARGS="--vus 1" plus any
#         scenario-specific vars (RESULTS_PER_RUN, STAGES, MODE, ...) which
#         k6 exposes to scripts via __ENV.
#         EXPECT_THRESHOLD_BREACH=1 treats k6's threshold-failure exit (99) as
#         success — set by the capacity Make targets, whose runs are *meant*
#         to ramp past the thresholds and map the ceiling.
set -euo pipefail

PERF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_DIR="$(dirname "$PERF_DIR")"
SCENARIO="${1:?usage: run-scenario.sh <path relative to perf/, e.g. k6/scenarios/smoke.js>}"

TIER="${TIER:-small}"
PORT="${PORT:-8877}"
TOKENS="${TOKENS:-100}"
SCRATCH="$PERF_DIR/.scratch"
DB="$SCRATCH/perf-$TIER.db"
MANIFEST="$PERF_DIR/.seed-manifest.json"
STAMP="$(date +%Y%m%d-%H%M%S)"
# MODE lands in the directory name so closed-loop (vus) and open-loop (rate)
# runs of the same scenario file stay distinguishable in results/ history.
OUT_DIR="$PERF_DIR/results/$STAMP-$(basename "$SCENARIO" .js)${MODE:+-$MODE}"
mkdir -p "$SCRATCH" "$OUT_DIR" "$PERF_DIR/baselines"

command -v k6 >/dev/null || { echo "k6 not found — install it first (see perf/README.md)"; exit 1; }

# Refuse a busy port up front: the readiness probe below cannot tell our
# server from a stale/foreign listener, so starting on a busy port means k6
# would measure the wrong process.
if (exec 3<>"/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
  echo "port $PORT is already in use — kill the stale server (or set PORT) and retry"
  exit 1
fi

echo "==> building server"
(cd "$REPO_DIR/backend" && go build -tags sqlite_fts5 -o "$SCRATCH/ttgo-perf-server" ./cmd/server)

reseeded=false
if [[ "${RESEED:-0}" == "1" || ! -f "$DB" || ! -f "$MANIFEST" ]]; then
  echo "==> seeding $DB (tier=$TIER, tokens=$TOKENS)"
  "$PERF_DIR/scripts/seed.sh"
  reseeded=true
else
  # The manifest is shared across tiers but its tokens exist only in the DB it
  # was seeded against. Reusing it against another tier's DB would 401 every
  # request, so a mismatch forces a reseed of the requested tier.
  manifest_db="$(awk -F'"' '/"db":/ {print $4; exit}' "$MANIFEST")"
  if [[ "$manifest_db" != "$DB" ]]; then
    echo "==> manifest belongs to ${manifest_db:-unknown} but this run targets $DB — reseeding"
    "$PERF_DIR/scripts/seed.sh"
    reseeded=true
  fi
fi
manifest_tokens="$(awk '/"tokens": \[/{f=1;next} f&&/\]/{exit} f{n++} END{print n+0}' "$MANIFEST")"
if [[ "$reseeded" == "false" && "$manifest_tokens" -ne "$TOKENS" ]]; then
  # Seed-time knobs only apply at seed time; warn when the reused manifest
  # disagrees with the requested pool so a stale seed can't silently
  # reintroduce token-row contention.
  echo "==> WARNING: reusing existing seed with $manifest_tokens tokens (TOKENS=$TOKENS has no effect without RESEED=1)"
fi

# Record everything that shaped this run next to its outputs, so kept results
# stay interpretable later without relying on the operator's notes. Knob
# values are the raw environment strings; null means unset, i.e. the scenario
# default at the recorded git SHA.
json_escape() {
  local s=${1//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\n'/\\n}
  s=${s//$'\r'/\\r}
  s=${s//$'\t'/\\t}
  printf '%s' "$s"
}
jstr() { if [[ -z "${1:-}" ]]; then printf 'null'; else printf '"%s"' "$(json_escape "$1")"; fi; }

git_sha="$(git -C "$REPO_DIR" rev-parse HEAD 2>/dev/null || echo unknown)"
git_dirty=false
[[ -n "$(git -C "$REPO_DIR" status --porcelain 2>/dev/null)" ]] && git_dirty=true
cpu_model="$(sysctl -n machdep.cpu.brand_string 2>/dev/null || awk -F': ' '/model name/{print $2; exit}' /proc/cpuinfo 2>/dev/null || echo unknown)"
mem_bytes="$(sysctl -n hw.memsize 2>/dev/null || awk '/MemTotal/{print $2*1024; exit}' /proc/meminfo 2>/dev/null || echo 0)"
cat >"$OUT_DIR/run-config.json" <<EOF
{
  "scenario": "$SCENARIO",
  "started_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "git_sha": "$git_sha",
  "git_dirty": $git_dirty,
  "k6_version": $(jstr "$(k6 version 2>/dev/null | head -1)"),
  "go_version": $(jstr "$(go version 2>/dev/null)"),
  "machine": {
    "os": $(jstr "$(uname -srm)"),
    "cpu": $(jstr "$cpu_model"),
    "cores": $(getconf _NPROCESSORS_ONLN 2>/dev/null || echo 0),
    "mem_bytes": ${mem_bytes:-0}
  },
  "knobs": {
    "tier": $(jstr "$TIER"),
    "port": $(jstr "$PORT"),
    "tokens_requested": $(jstr "$TOKENS"),
    "tokens_in_manifest": ${manifest_tokens:-0},
    "reseeded": $reseeded,
    "mode": $(jstr "${MODE:-}"),
    "results_per_run": $(jstr "${RESULTS_PER_RUN:-}"),
    "stages": $(jstr "${STAGES:-}"),
    "pipelines_per_min": $(jstr "${PIPELINES_PER_MIN:-}"),
    "rate_duration": $(jstr "${RATE_DURATION:-}"),
    "max_vus": $(jstr "${MAX_VUS:-}"),
    "read_vus": $(jstr "${READ_VUS:-}"),
    "iterations": $(jstr "${ITERATIONS:-}"),
    "phase_minutes": $(jstr "${PHASE_MINUTES:-}"),
    "write_baseline": $(jstr "${WRITE_BASELINE:-}"),
    "gate_ignore_machine": $(jstr "${GATE_IGNORE_MACHINE:-}"),
    "k6_args": $(jstr "${K6_ARGS:-}"),
    "expect_threshold_breach": $(jstr "${EXPECT_THRESHOLD_BREACH:-}")
  }
}
EOF

SERVER_PID=""
SAMPLER_PID=""
cleanup() {
  [[ -n "$SAMPLER_PID" ]] && kill "$SAMPLER_PID" 2>/dev/null || true
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" 2>/dev/null || true
    # The server is a direct child (exec'd in a backgrounded subshell), so
    # this genuinely waits for its graceful shutdown / WAL flush.
    wait "$SERVER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

echo "==> starting server on :$PORT"
(
  cd "$SCRATCH" &&
    DB_PATH="$DB" LISTEN_ADDR="127.0.0.1:$PORT" \
    ADMIN_EMAIL="perf-admin@perf.local" ADMIN_PASSWORD="perfseed-local-only" \
    exec ./ttgo-perf-server
) >"$OUT_DIR/server.log" 2>&1 &
SERVER_PID=$!

code="000"
for _ in $(seq 1 50); do
  kill -0 "$SERVER_PID" 2>/dev/null || break # server died — stop probing
  code="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$PORT/api/categories" || true)"
  [[ "$code" != "000" ]] && break
  sleep 0.2
done
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
  echo "server process exited early; see $OUT_DIR/server.log"
  exit 1
fi
if [[ "$code" == "000" ]]; then
  echo "server did not come up; see $OUT_DIR/server.log"
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
k6_status=0
k6 run \
  -e TTGO_BASE_URL="http://127.0.0.1:$PORT" \
  -e TTGO_MANIFEST="$MANIFEST" \
  -e TTGO_SUMMARY_PATH="$OUT_DIR/summary.json" \
  -e TTGO_BASELINE_PATH="$PERF_DIR/baselines/gate-$TIER.json" \
  -e TTGO_MACHINE="$cpu_model" \
  -e TTGO_TIER="$TIER" \
  ${K6_ARGS:-} \
  "$PERF_DIR/$SCENARIO" || k6_status=$?

# k6 exits 99 when thresholds fail at the end of a run. For capacity runs that
# is the expected outcome — the ramp is supposed to cross the breaking point —
# so the capacity targets set EXPECT_THRESHOLD_BREACH=1. Anything else nonzero
# is a real failure.
if [[ "$k6_status" -eq 99 && "${EXPECT_THRESHOLD_BREACH:-0}" == "1" ]]; then
  echo "==> thresholds breached — expected for a capacity mapping run (k6 exit 99)"
elif [[ "$k6_status" -ne 0 ]]; then
  echo "==> k6 failed (exit $k6_status); partial results in $OUT_DIR"
  exit "$k6_status"
fi

echo "==> done. results in $OUT_DIR (summary.json, run-config.json, telemetry.csv, server.log)"
