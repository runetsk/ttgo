#!/usr/bin/env bash
# Stop the TTGO dev stack started by scripts/dev-start.sh.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dev_dir="$root/.dev"

for name in frontend backend; do
  pid_file="$dev_dir/$name.pid"
  if [[ ! -f "$pid_file" ]]; then
    echo "$name - not running (no pid file)"
    continue
  fi
  pid="$(cat "$pid_file")"
  if kill -0 "$pid" 2>/dev/null; then
    kill "$pid"
    for _ in $(seq 20); do
      kill -0 "$pid" 2>/dev/null || break
      sleep 0.25
    done
    kill -0 "$pid" 2>/dev/null && kill -9 "$pid"
    echo "$name - stopped (pid $pid)"
  else
    echo "$name - not running (stale pid $pid)"
  fi
  rm -f "$pid_file"
done
