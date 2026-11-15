#!/usr/bin/env bash
# Start the TTGO dev stack (Go backend + Vite frontend) in the background.
# PIDs and logs go to .dev/ at the repo root; stop with scripts/dev-stop.sh.
#
# Overridable via environment:
#   TTGO_BACKEND_PORT   (default 8090 - 8080 is often taken)
#   TTGO_FRONTEND_PORT  (default 5173)
#   DB_PATH             (default backend/tracker.db)
#   ADMIN_EMAIL / ADMIN_PASSWORD (default admin@example.com / changeme123)
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
dev_dir="$root/.dev"
backend_port="${TTGO_BACKEND_PORT:-8090}"
frontend_port="${TTGO_FRONTEND_PORT:-5173}"

port_open() { (echo >"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }

running() {
  local pid_file="$dev_dir/$1.pid"
  [[ -f "$pid_file" ]] && kill -0 "$(cat "$pid_file")" 2>/dev/null
}

wait_port() { # port label pid log
  for _ in $(seq 60); do
    port_open "$1" && return 0
    if ! kill -0 "$3" 2>/dev/null; then
      echo "$2 exited early. Last log lines ($4):" >&2
      tail -n 20 "$4" >&2 || true
      exit 1
    fi
    sleep 0.5
  done
  echo "$2 did not open port $1 within 30s - see $4" >&2
  exit 1
}

for svc in backend frontend; do
  if running "$svc"; then
    echo "$svc is already running (see .dev/$svc.pid). Run scripts/dev-stop.sh first." >&2
    exit 1
  fi
done
for port in "$backend_port" "$frontend_port"; do
  if port_open "$port"; then
    echo "Port $port is already in use by another process." >&2
    exit 1
  fi
done
if [[ ! -f "$root/frontend/node_modules/vite/bin/vite.js" ]]; then
  echo "frontend/node_modules is missing - run 'npm ci' in frontend/ first." >&2
  exit 1
fi

mkdir -p "$dev_dir/bin"

echo "Building backend..."
(cd "$root/backend" && go build -tags sqlite_fts5 -o "$dev_dir/bin/ttgo-server" ./cmd/server)

(
  cd "$root/backend"
  LISTEN_ADDR=":$backend_port" \
  CORS_ORIGIN="http://localhost:$frontend_port" \
  ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}" \
  ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme123}" \
    nohup "$dev_dir/bin/ttgo-server" >"$dev_dir/backend.log" 2>&1 &
  echo $! >"$dev_dir/backend.pid"
)

# Run vite through node directly (not npm) so the recorded PID is the server
# itself and stopping it leaves no orphans.
(
  cd "$root/frontend"
  TTGO_DEV_API_TARGET="http://localhost:$backend_port" \
    nohup node node_modules/vite/bin/vite.js --port "$frontend_port" --strictPort \
    >"$dev_dir/frontend.log" 2>&1 &
  echo $! >"$dev_dir/frontend.pid"
)

wait_port "$backend_port" Backend "$(cat "$dev_dir/backend.pid")" "$dev_dir/backend.log"
wait_port "$frontend_port" Frontend "$(cat "$dev_dir/frontend.pid")" "$dev_dir/frontend.log"

echo
echo "Backend   http://localhost:$backend_port   (pid $(cat "$dev_dir/backend.pid"))"
echo "Frontend  http://localhost:$frontend_port   (pid $(cat "$dev_dir/frontend.pid"))"
echo "Logs in .dev/   Stop with: scripts/dev-stop.sh"
