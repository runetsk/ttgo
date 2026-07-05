// S4 — WebSocket fan-out (spec §5). CLIENTS session-authenticated sockets
// subscribe to runs:* while a single probe VU posts results at
// RESULTS_PER_SEC to one run. Measured per received result_updated event:
// receive lag (client clock − server "timestamp"; same-machine ⇒ no skew).
// Loss = 1 − ws_events_received / (ws_probe_results_posted × connected
// clients), computed offline — events carry no sequence numbers. Check
// server.log to attribute loss: hub-ingress drops (broadcast channel full)
// vs per-client egress drops (slow client disconnected) are different
// failure modes with different fixes.
//
//   make -C perf ws                          # 200 clients (default 10-user seed)
//   USERS=50 RESEED=1 CLIENTS=1000 make -C perf ws   # cap-probing run
//
// Thresholds are observational (the whole point of a cap run is to cross
// the caps); the ws Make target treats k6 exit 99 as expected.
import ws from 'k6/ws';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { Trend, Counter } from 'k6/metrics';
import { usersForClients, loginAll, wsParams } from '../lib/ws.js';
import { createRun, addResult, completeRun, tokenForVU, resultsPerRun, resultBodiesAt, intKnob, BASE_URL } from '../lib/api.js';
import { WS_DEFAULTS } from '../config/workloads.js';
export { handleSummary } from '../lib/summary.js';

const CLIENTS = intKnob('CLIENTS', WS_DEFAULTS.clients);
const HOLD_MINUTES = intKnob('HOLD_MINUTES', WS_DEFAULTS.holdMinutes);
const RESULTS_PER_SEC = intKnob('RESULTS_PER_SEC', WS_DEFAULTS.resultsPerSec);
const RESULTS_PER_RUN = resultsPerRun(200); // probe pool bound; posts loop over the ingest pool
const RAMP_MINUTES = 1;

const users = usersForClients(CLIENTS); // init-context cap guard

const wsLag = new Trend('ws_lag_ms');
const wsEvents = new Counter('ws_events_received');
const wsConnectErrors = new Counter('ws_connect_errors');
const probePosted = new Counter('ws_probe_results_posted');

export const options = {
  setupTimeout: '240s', // 50 paced logins ≈ 90s; headroom for more
  scenarios: {
    clients: {
      executor: 'ramping-vus',
      exec: 'wsClient',
      startVUs: 0,
      stages: [
        { duration: `${RAMP_MINUTES}m`, target: CLIENTS },
        { duration: `${HOLD_MINUTES}m`, target: CLIENTS },
        { duration: '30s', target: 0 },
      ],
      gracefulStop: '30s',
    },
    probe: {
      executor: 'constant-vus',
      exec: 'probe',
      vus: 1,
      // Probe runs only during the hold: every client still connected
      // through the hold should see every probe event, making
      // expected-count loss math clean.
      startTime: `${RAMP_MINUTES}m`,
      duration: `${HOLD_MINUTES}m`,
      gracefulStop: '60s',
    },
  },
  thresholds: {
    // Observational: mark, never abort — cap runs are mapping runs.
    ws_lag_ms: ['p(95)<500'],
    ws_connect_errors: ['count==0'],
    http_req_failed: ['rate<0.01'],
  },
  summaryTrendStats: ['avg', 'p(50)', 'p(90)', 'p(95)', 'p(99)', 'max'],
};

export function setup() {
  return { cookies: loginAll(users.emails, users.password) };
}

export function wsClient(data) {
  // Stable VU→user spread keeps every user ≤ 20 connections at full ramp.
  const cookie = data.cookies[(__VU - 1) % data.cookies.length];
  const url = `${BASE_URL.replace('http', 'ws')}/api/ws`;
  // Close 5s after the hold phase ends, SCENARIO-relative (not per-VU): a VU
  // that connects late in the ramp must not schedule its close past the
  // scenario's hard end, or the executor force-interrupts it and the clean
  // close never runs.
  const scenarioElapsedMs = Date.now() - exec.scenario.startTime;
  const holdEndMs = (RAMP_MINUTES + HOLD_MINUTES) * 60 * 1000 + 5000;
  // Ramp-down guard: after the synchronized close instant, ramping-vus can
  // still start fresh iterations while the VU target falls — each would
  // reconnect for the 1s floor and spam connect/disconnect noise into
  // server.log, which operators read for loss attribution. Idle instead.
  if (scenarioElapsedMs > holdEndMs) {
    sleep(5);
    return;
  }
  const holdMs = Math.max(1000, holdEndMs - scenarioElapsedMs);

  const res = ws.connect(url, wsParams(cookie), (socket) => {
    socket.on('open', () => {
      socket.send(JSON.stringify({ action: 'subscribe', topic: 'runs:*' }));
    });
    socket.on('message', (raw) => {
      let msg;
      try {
        msg = JSON.parse(raw);
      } catch (_) {
        return;
      }
      if (msg.type === 'result_updated' || msg.type === 'result_bulk_updated') {
        wsEvents.add(1);
        const serverTs = Date.parse(msg.timestamp);
        if (!Number.isNaN(serverTs)) {
          wsLag.add(Date.now() - serverTs);
        }
      }
    });
    socket.on('error', () => {
      wsConnectErrors.add(1);
    });
    socket.setTimeout(() => socket.close(), holdMs);
  });
  const ok = check(res, { 'ws upgrade 101': (r) => r && r.status === 101 });
  if (!ok) {
    // Rejections at the caps land here (401/immediate close) — count them;
    // back off so a capped VU doesn't hot-loop reconnects.
    wsConnectErrors.add(1);
    sleep(5);
  }
}

export function probe() {
  const token = tokenForVU();
  const run = createRun(token, `perf-ws-probe VU${__VU} it${__ITER}`);
  if (!run || !run.id) {
    sleep(1);
    return;
  }
  // Post RESULTS_PER_RUN results at the paced rate (200 at 20/s ≈ 10 s per
  // run), then complete and start a fresh run next iteration. Result events
  // carry O(1) delta payloads (affected row + run summary), so this bound is
  // run hygiene rather than frame-size control.
  for (let i = 0; i < RESULTS_PER_RUN; i++) {
    addResult(token, run.id, resultBodiesAt(i));
    probePosted.add(1);
    sleep(1 / RESULTS_PER_SEC);
  }
  completeRun(token, run.id);
}
