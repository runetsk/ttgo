#!/usr/bin/env bash
# Windowed per-operation latency/error table from a run's server.log.
#
# Usage:  scripts/analyze.sh results/<run-dir> [window-seconds]
#
# Why server-side: k6's end-of-run summary blends the whole run into one
# aggregate, which hides where inside a ramp the latency crossed a threshold
# (both 2026-07-04 capacity runs passed aggregate thresholds their peaks
# violated). The server logs every request with duration_ms; bucketing those
# by wall-clock window gives p95-over-time. Timestamps are server-side and
# second-granular; the first request line anchors t=0.
set -euo pipefail

DIR="${1:?usage: analyze.sh results/<run-dir> [window-seconds]}"
WIN="${2:-30}"
LOG="$DIR/server.log"
[[ -f "$LOG" ]] || { echo "no server.log in $DIR" >&2; exit 1; }

awk -v win="$WIN" '
/INFO http request/ {
  split($2, t, ":"); sec = t[1]*3600 + t[2]*60 + t[3];
  if (!t0) t0 = sec;
  # Midnight rollover: time-of-day seconds wrap once for a run crossing
  # 00:00 (runs are <24h, so one wrap is the only case).
  if (sec < t0) sec += 86400;
  w = int((sec - t0) / win);
  method = ""; path = ""; dur = -1; st = 0;
  for (i = 1; i <= NF; i++) {
    if ($i ~ /^method=/) { sub("method=", "", $i); method = $i }
    else if ($i ~ /^path=/) { sub("path=", "", $i); path = $i }
    else if ($i ~ /^status=/) { sub("status=", "", $i); st = $i + 0 }
    else if ($i ~ /^duration_ms=/) { sub("duration_ms=", "", $i); dur = $i + 0 }
  }
  op = "other";
  if (method == "POST" && path ~ /^\/api\/runs\/[^\/]+\/results$/) op = "add_result";
  else if (method == "POST" && path ~ /^\/api\/runs\/[^\/]+\/complete$/) op = "complete_run";
  else if (method == "POST" && path == "/api/runs") op = "create_run";
  else if (method == "GET" && path == "/api/runs") op = "runs_list";
  else if (method == "GET" && path ~ /^\/api\/runs\/[^\/]+$/) op = "run_detail";
  else if (method == "GET" && path ~ /^\/api\/analytics\/summary/) op = "analytics_summary";
  else if (method == "GET" && path ~ /^\/api\/analytics\/trend/) op = "analytics_trend";
  else if (method == "GET" && path ~ /^\/api\/analytics\/flaky/) op = "analytics_flaky";
  else if (method == "GET" && path ~ /^\/api\/search/) op = "search";
  if (dur >= 0) print w, op, dur, (st >= 500 ? 1 : 0);
}' "$LOG" |
  sort -k1,1n -k2,2 -k3,3n |
  awk -v win="$WIN" '
{
  key = $1 SUBSEP $2;
  if (key != cur) { if (NR > 1) flush(); cur = key; w = $1; op = $2; n = 0; e = 0 }
  v[n++] = $3; e += $4
}
function flush(  i) {
  # Nearest-rank p95: ceil(0.95*n)-th smallest, 0-based index ceil(0.95n)-1
  # (v[int(n*0.95)] was one rank high, e.g. sample 20 of 20 instead of 19).
  i = int(n * 0.95); if (i * 100 < n * 95) i++; i--; if (i < 0) i = 0;
  printf "%6ds  %-18s %7d reqs  p95=%6dms  5xx=%d\n", w * win, op, n, v[i], e
}
END { if (NR) flush() }'
